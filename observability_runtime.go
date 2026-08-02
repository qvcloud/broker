package broker

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type probeFunc func(context.Context) error

// Runtime is the shared concurrency-safe observability implementation used by adapters.
type Runtime struct {
	system     string
	instance   string
	categories atomic.Uint32
	config     ObservabilityConfig
	now        func() time.Time

	mu                sync.RWMutex
	state             LifecycleState
	connected         bool
	ready             bool
	lastConnected     time.Time
	lastReady         time.Time
	lastFailure       *SafeError
	reconnects        uint64
	subscriptions     int64
	inflight          map[Operation]int64
	totals            map[string]uint64
	unsupported       map[string]bool
	probe             probeFunc
	sequence          uint64
	dropped           atomic.Uint64
	validationErr     error
	operationCounter  metric.Int64Counter
	durationHistogram metric.Float64Histogram
	inflightCounter   metric.Int64UpDownCounter
	retryCounter      metric.Int64Counter
	reconnectCounter  metric.Int64Counter
	subscriptionGauge metric.Int64UpDownCounter

	dispatchMu sync.Mutex
	queue      chan BrokerStateEvent
	stop       chan struct{}
	done       chan struct{}
}

// NewRuntimeForOptions creates an always-non-nil runtime for constructors that
// cannot return errors. ValidationError must be checked by Init or Connect.
func NewRuntimeForOptions(system string, options Options) *Runtime {
	runtime, err := NewRuntime(system, options.Observability)
	if err == nil {
		runtime.setMeter(options.Meter)
		runtime.SetUnsupported("active_probe")
		return runtime
	}
	runtime, _ = NewRuntime(system, nil)
	runtime.validationErr = err
	return runtime
}

func (r *Runtime) setMeter(meter metric.Meter) {
	if meter == nil {
		return
	}
	r.operationCounter, _ = meter.Int64Counter("broker.operations")
	r.durationHistogram, _ = meter.Float64Histogram("broker.operation.duration", metric.WithUnit("s"))
	r.inflightCounter, _ = meter.Int64UpDownCounter("broker.inflight")
	r.retryCounter, _ = meter.Int64Counter("broker.retries")
	r.reconnectCounter, _ = meter.Int64Counter("broker.reconnects")
	r.subscriptionGauge, _ = meter.Int64UpDownCounter("broker.subscriptions")
}

// ValidationError returns deferred constructor-time configuration errors.
func (r *Runtime) ValidationError() error {
	if r == nil {
		return nil
	}
	return r.validationErr
}

// Observability returns this runtime as the optional capability handle.
func (r *Runtime) Observability() Observability { return r }

// NewRuntime creates a disabled runtime when config is nil.
func NewRuntime(system string, config *ObservabilityConfig) (*Runtime, error) {
	cfg := defaultObservabilityConfig()
	if config != nil {
		cfg = *config
	}
	if err := validateObservabilityConfig(cfg); err != nil {
		return nil, err
	}
	instance := cfg.InstanceID
	if instance == "" {
		instance = fmt.Sprintf("%s-%x", system, time.Now().UnixNano())
	}
	r := &Runtime{system: system, instance: instance, config: cfg, now: time.Now, state: StateUnknown, inflight: make(map[Operation]int64), totals: make(map[string]uint64), unsupported: make(map[string]bool)}
	r.categories.Store(uint32(cfg.Categories))
	if r.sinkEnabled(cfg.Categories) {
		r.startDispatcher()
	}
	return r, nil
}

func (r *Runtime) sinkEnabled(set CategorySet) bool {
	return r.config.Sink != nil && (set.Has(CategoryLogging) || set.Has(CategoryStateEvents))
}

// SetCategories changes signal enablement atomically.
func (r *Runtime) SetCategories(set CategorySet) error {
	if set&^allCategories != 0 {
		return fmt.Errorf("%w: unknown category bits", ErrInvalidObservabilityConfig)
	}
	if (set.Has(CategoryLogging) || set.Has(CategoryStateEvents)) && r.config.Sink == nil {
		return fmt.Errorf("%w: event sink required", ErrInvalidObservabilityConfig)
	}
	old := CategorySet(r.categories.Swap(uint32(set)))
	if !r.sinkEnabled(old) && r.sinkEnabled(set) {
		r.startDispatcher()
	}
	if r.sinkEnabled(old) && !r.sinkEnabled(set) {
		r.stopDispatcher()
	}
	return nil
}

func (r *Runtime) enabled(category Category) bool {
	return CategorySet(r.categories.Load()).Has(category)
}

// Enabled reports whether one category is currently enabled.
func (r *Runtime) Enabled(category Category) bool {
	return r != nil && r.enabled(category)
}

// OperationsEnabled is a low-cost hot-path guard for operation diagnostics.
func (r *Runtime) OperationsEnabled() bool {
	set := CategorySet(r.categories.Load())
	return set.Has(CategoryDiagnostics) || set.Has(CategoryMeasurements) || set.Has(CategoryLogging)
}

// Health returns local state without network activity.
func (r *Runtime) Health() HealthStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.healthLocked(r.now())
}

func (r *Runtime) healthLocked(now time.Time) HealthStatus {
	return HealthStatus{CheckedAt: now.UTC(), State: r.state, Connected: r.connected, Ready: r.ready, Degraded: r.state == StateDegraded, LastConnectedAt: r.lastConnected, LastReadyAt: r.lastReady, Error: cloneSafeError(r.lastFailure), Capabilities: r.capabilitiesLocked()}
}

func (r *Runtime) capabilitiesLocked() map[string]bool {
	result := make(map[string]bool, len(r.unsupported))
	for key, unsupported := range r.unsupported {
		result[key] = !unsupported
	}
	return result
}

// Probe executes an explicitly registered adapter probe.
func (r *Runtime) Probe(ctx context.Context) (HealthStatus, error) {
	r.mu.RLock()
	probe := r.probe
	r.mu.RUnlock()
	if probe == nil {
		return r.Health(), ErrUnsupported
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.config.ProbeTimeout)
		defer cancel()
	}
	err := probe(ctx)
	status := r.Health()
	status.CheckedAt = r.now().UTC()
	if err != nil {
		status.Error = safeError(OperationProbe, err)
	}
	return status, err
}

// SetProbe registers an adapter-specific active probe.
func (r *Runtime) SetProbe(probe func(context.Context) error) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.probe = probe
	delete(r.unsupported, "active_probe")
	r.mu.Unlock()
}

// SetUnsupported declares a native limitation.
func (r *Runtime) SetUnsupported(capability string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.unsupported[capability] = true
	r.mu.Unlock()
}

// Transition updates lifecycle state and emits an optional state event.
func (r *Runtime) Transition(to LifecycleState, connected, ready bool, err error) {
	if r == nil {
		return
	}
	r.mu.Lock()
	from := r.state
	r.state, r.connected, r.ready = to, connected, ready
	now := r.now().UTC()
	if connected {
		r.lastConnected = now
	}
	if to == StateReconnecting {
		r.reconnects++
	}
	if ready {
		r.lastReady = now
	}
	if err != nil {
		r.lastFailure = safeErrorWithFields(OperationConnect, err, r.config.RedactedFields)
	}
	r.sequence++
	event := BrokerStateEvent{Sequence: r.sequence, Timestamp: now, BrokerSystem: r.system, InstanceID: r.instance, Kind: "connection." + string(to), From: from, To: to, Operation: OperationConnect, Severity: SeverityInfo, Outcome: OutcomeSuccess, Error: cloneSafeError(r.lastFailure)}
	if err != nil {
		event.Severity, event.Outcome = SeverityError, OutcomeFailure
	}
	r.mu.Unlock()
	if r.enabled(CategoryStateEvents) {
		r.enqueue(event)
	}
}

// Observe records an operation completion and optional failure event.
func (r *Runtime) Observe(operation Operation, started time.Time, err error) {
	if r == nil {
		return
	}
	if !r.enabled(CategoryDiagnostics) && !r.enabled(CategoryMeasurements) && (err == nil || !r.enabled(CategoryLogging)) {
		return
	}
	outcome := OutcomeSuccess
	if err != nil {
		outcome = OutcomeFailure
	}
	r.mu.Lock()
	key := string(operation) + ":" + string(outcome)
	r.totals[key]++
	if r.inflight[operation] > 0 {
		r.inflight[operation]--
	}
	if err != nil {
		r.lastFailure = safeErrorWithFields(operation, err, r.config.RedactedFields)
		r.sequence++
	}
	event := BrokerStateEvent{Sequence: r.sequence, Timestamp: r.now().UTC(), BrokerSystem: r.system, InstanceID: r.instance, Kind: "operation.failed", To: r.state, Operation: operation, Severity: SeverityError, Outcome: outcome, Error: cloneSafeError(r.lastFailure)}
	r.mu.Unlock()
	if r.enabled(CategoryMeasurements) {
		values := []attribute.KeyValue{attribute.String("messaging.system", r.system), attribute.String("messaging.operation", string(operation)), attribute.String("outcome", string(outcome))}
		if err != nil && event.Error != nil {
			values = append(values, attribute.String("error.category", string(event.Error.Category)))
		}
		attrs := metric.WithAttributes(values...)
		if r.operationCounter != nil {
			r.operationCounter.Add(context.Background(), 1, attrs)
		}
		if r.durationHistogram != nil && !started.IsZero() {
			r.durationHistogram.Record(context.Background(), r.now().Sub(started).Seconds(), attrs)
		}
		if r.inflightCounter != nil {
			r.inflightCounter.Add(context.Background(), -1, metric.WithAttributes(attribute.String("messaging.system", r.system), attribute.String("messaging.operation", string(operation))))
		}
		if operation == OperationRetry && r.retryCounter != nil {
			r.retryCounter.Add(context.Background(), 1, attrs)
		}
		if operation == OperationReconnect && r.reconnectCounter != nil {
			r.reconnectCounter.Add(context.Background(), 1, attrs)
		}
	}
	if err != nil && r.enabled(CategoryLogging) {
		r.enqueue(event)
	}
}

// StartOperation increments an in-flight operation and returns its start time.
func (r *Runtime) StartOperation(operation Operation) time.Time {
	if r == nil {
		return time.Now()
	}
	if r.enabled(CategoryDiagnostics) || r.enabled(CategoryMeasurements) {
		r.mu.Lock()
		r.inflight[operation]++
		r.mu.Unlock()
	}
	if r.enabled(CategoryMeasurements) && r.inflightCounter != nil {
		r.inflightCounter.Add(context.Background(), 1, metric.WithAttributes(attribute.String("messaging.system", r.system), attribute.String("messaging.operation", string(operation))))
	}
	return r.now()
}

// AddSubscription adjusts the active subscription count.
func (r *Runtime) AddSubscription(delta int64) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.subscriptions += delta
	if r.subscriptions < 0 {
		r.subscriptions = 0
	}
	r.mu.Unlock()
	if r.enabled(CategoryMeasurements) && r.subscriptionGauge != nil {
		r.subscriptionGauge.Add(context.Background(), delta, metric.WithAttributes(attribute.String("messaging.system", r.system)))
	}
}

// WrapHandler observes receive, handler, Ack, and Nack outcomes without
// changing the wrapped event's delivery decisions.
func (r *Runtime) WrapHandler(handler Handler) Handler {
	if r == nil || handler == nil {
		return handler
	}
	return func(ctx context.Context, event Event) (err error) {
		instrument := r.OperationsEnabled()
		if instrument {
			received := r.StartOperation(OperationReceive)
			r.Observe(OperationReceive, received, nil)
		}
		var started time.Time
		if instrument {
			started = r.StartOperation(OperationHandler)
		}
		if event != nil {
			if message := event.Message(); message != nil {
				ctx = r.ExtractContext(ctx, message.Header)
			}
			event = &observedEvent{Event: event, runtime: r}
		}
		err = handler(ctx, event)
		if instrument {
			r.Observe(OperationHandler, started, err)
		}
		return err
	}
}

// WrapSubscriber observes subscription lifetime and Unsubscribe outcomes.
func (r *Runtime) WrapSubscriber(subscriber Subscriber) Subscriber {
	if r == nil || subscriber == nil {
		return subscriber
	}
	r.AddSubscription(1)
	return &observedSubscriber{Subscriber: subscriber, runtime: r}
}

type observedSubscriber struct {
	Subscriber
	runtime *Runtime
	once    sync.Once
}

func (s *observedSubscriber) Unsubscribe() (err error) {
	instrument := s.runtime.OperationsEnabled()
	var started time.Time
	if instrument {
		started = s.runtime.StartOperation(OperationUnsubscribe)
	}
	err = s.Subscriber.Unsubscribe()
	if instrument {
		s.runtime.Observe(OperationUnsubscribe, started, err)
	}
	if err == nil {
		s.once.Do(func() { s.runtime.AddSubscription(-1) })
	}
	return err
}

type observedEvent struct {
	Event
	runtime *Runtime
}

func (e *observedEvent) Ack() (err error) {
	instrument := e.runtime.OperationsEnabled()
	var started time.Time
	if instrument {
		started = e.runtime.StartOperation(OperationAck)
	}
	err = e.Event.Ack()
	if instrument {
		e.runtime.Observe(OperationAck, started, err)
	}
	return err
}

func (e *observedEvent) Nack(requeue bool) (err error) {
	instrument := e.runtime.OperationsEnabled()
	var started time.Time
	if instrument {
		started = e.runtime.StartOperation(OperationNack)
	}
	err = e.Event.Nack(requeue)
	if instrument {
		e.runtime.Observe(OperationNack, started, err)
	}
	return err
}

// Snapshot returns an immutable diagnostic copy.
func (r *Runtime) Snapshot() DiagnosticSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	unsupported := make([]string, 0, len(r.unsupported))
	for key, value := range r.unsupported {
		if value {
			unsupported = append(unsupported, key)
		}
	}
	sort.Strings(unsupported)
	return DiagnosticSnapshot{BrokerSystem: r.system, InstanceID: r.instance, Health: r.healthLocked(r.now()), ReconnectCount: r.reconnects, SubscriptionCount: r.subscriptions, InFlight: cloneOperationMap(r.inflight), OperationTotals: cloneStringMap(r.totals), LastFailure: cloneSafeError(r.lastFailure), DroppedRecords: r.dropped.Load(), UnsupportedCapabilities: unsupported}
}

func cloneSafeError(value *SafeError) *SafeError {
	if value == nil {
		return nil
	}
	copy := *value
	if value.Retryable != nil {
		retryable := *value.Retryable
		copy.Retryable = &retryable
	}
	return &copy
}
func cloneOperationMap(source map[Operation]int64) map[Operation]int64 {
	result := make(map[Operation]int64, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
func cloneStringMap(source map[string]uint64) map[string]uint64 {
	result := make(map[string]uint64, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func (r *Runtime) startDispatcher() {
	r.dispatchMu.Lock()
	defer r.dispatchMu.Unlock()
	if r.queue != nil {
		return
	}
	capacity := r.config.BufferCapacity
	if capacity == 0 {
		capacity = 256
	}
	r.queue, r.stop, r.done = make(chan BrokerStateEvent, capacity), make(chan struct{}), make(chan struct{})
	go r.dispatch(r.queue, r.stop, r.done)
}

// Start activates optional workers for the current category set. It is safe to
// call repeatedly when a broker reconnects.
func (r *Runtime) Start() {
	if r == nil {
		return
	}
	if r.sinkEnabled(CategorySet(r.categories.Load())) {
		r.startDispatcher()
	}
}

func (r *Runtime) stopDispatcher() {
	r.dispatchMu.Lock()
	if r.stop == nil {
		r.dispatchMu.Unlock()
		return
	}
	stop, done := r.stop, r.done
	r.stop, r.done, r.queue = nil, nil, nil
	close(stop)
	r.dispatchMu.Unlock()
	select {
	case <-done:
	case <-time.After(r.config.SinkTimeout):
	}
}

func (r *Runtime) enqueue(event BrokerStateEvent) {
	r.dispatchMu.Lock()
	queue := r.queue
	if queue == nil {
		r.dispatchMu.Unlock()
		return
	}
	select {
	case queue <- event:
		r.dispatchMu.Unlock()
		return
	default:
	}
	if r.config.OverflowPolicy == OverflowDropOldest {
		select {
		case <-queue:
		default:
		}
		select {
		case queue <- event:
		default:
		}
	}
	r.dropped.Add(1)
	r.dispatchMu.Unlock()
}

func (r *Runtime) dispatch(queue <-chan BrokerStateEvent, stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	for {
		select {
		case <-stop:
			for {
				select {
				case event := <-queue:
					r.callSink(event)
				default:
					return
				}
			}
		case event := <-queue:
			r.callSink(event)
		}
	}
}

func (r *Runtime) callSink(event BrokerStateEvent) {
	ctx, cancel := context.WithTimeout(context.Background(), r.config.SinkTimeout)
	defer cancel()
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		defer func() {
			if recover() != nil {
				r.dropped.Add(1)
			}
		}()
		if err := r.config.Sink.HandleObservabilityEvent(ctx, event); err != nil {
			r.dropped.Add(1)
		}
	}()
	select {
	case <-finished:
	case <-ctx.Done():
		r.dropped.Add(1)
	}
}

// Close stops optional observability work within the configured budget.
func (r *Runtime) Close() {
	if r == nil {
		return
	}
	r.stopDispatcher()
}
