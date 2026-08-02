package broker

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

type controllableClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *controllableClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *controllableClock) Advance(delta time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(delta)
	c.mu.Unlock()
}

type fakeTelemetry struct {
	meter    metric.Meter
	tracer   trace.Tracer
	recorder *tracetest.SpanRecorder
}

func newFakeTelemetry() fakeTelemetry {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	return fakeTelemetry{meter: metricnoop.NewMeterProvider().Meter("broker-test"), tracer: provider.Tracer("broker-test"), recorder: recorder}
}

func assertNoGoroutineGrowth(t *testing.T, before, allowance int) {
	t.Helper()
	time.Sleep(20 * time.Millisecond)
	if after := runtime.NumGoroutine(); after > before+allowance {
		t.Fatalf("goroutines before=%d after=%d allowance=%d", before, after, allowance)
	}
}

type testEvent struct{ ackErr, nackErr error }

func (e *testEvent) Topic() string     { return "topic" }
func (e *testEvent) Message() *Message { return &Message{} }
func (e *testEvent) Ack() error        { return e.ackErr }
func (e *testEvent) Nack(bool) error   { return e.nackErr }
func (e *testEvent) Error() error      { return nil }

type recordingSink struct {
	mu     sync.Mutex
	events []BrokerStateEvent
	wake   chan struct{}
}

func newRecordingSink() *recordingSink { return &recordingSink{wake: make(chan struct{}, 32)} }

func (s *recordingSink) HandleObservabilityEvent(_ context.Context, event BrokerStateEvent) error {
	s.mu.Lock()
	s.events = append(s.events, event)
	s.mu.Unlock()
	select {
	case s.wake <- struct{}{}:
	default:
	}
	return nil
}

func (s *recordingSink) count() int { s.mu.Lock(); defer s.mu.Unlock(); return len(s.events) }

func TestReusableObservabilityTestHarness(t *testing.T) {
	before := runtime.NumGoroutine()
	clock := &controllableClock{now: time.Date(2026, 8, 2, 1, 2, 3, 0, time.UTC)}
	telemetry := newFakeTelemetry()
	_, span := telemetry.tracer.Start(context.Background(), "test")
	span.End()
	if len(telemetry.recorder.Ended()) != 1 {
		t.Fatal("fake tracer did not record span")
	}
	sink := newRecordingSink()
	observability := &ObservabilityConfig{Categories: Categories(CategoryMeasurements, CategoryStateEvents), Sink: sink, BufferCapacity: 4, OverflowPolicy: OverflowDropNewest, SinkTimeout: time.Second, ProbeTimeout: time.Second, CorrelationConflict: CorrelationPreserve}
	runtime := NewRuntimeForOptions("test", Options{Observability: observability, Meter: telemetry.meter})
	runtime.now = clock.Now
	runtime.Transition(StateReady, true, true, nil)
	if got := runtime.Health().CheckedAt; !got.Equal(clock.Now()) {
		t.Fatalf("checked at %s, want %s", got, clock.Now())
	}
	clock.Advance(time.Second)
	started := runtime.StartOperation(OperationPublish)
	clock.Advance(25 * time.Millisecond)
	runtime.Observe(OperationPublish, started, nil)
	runtime.Close()
	assertNoGoroutineGrowth(t, before, 1)
}

func TestObservabilityDisabledByDefault(t *testing.T) {
	b := NewNoopBroker()
	observable, ok := b.(Observable)
	if !ok {
		t.Fatal("noop broker must expose optional observability")
	}
	if got := observable.Observability().Snapshot(); got.BrokerSystem != "noop" || got.Health.State != StateUnknown {
		t.Fatalf("unexpected snapshot: %+v", got)
	}
	if err := b.Connect(); err != nil {
		t.Fatal(err)
	}
	if err := b.Publish(context.Background(), "topic", &Message{Body: []byte("secret body")}); err != nil {
		t.Fatal(err)
	}
	if got := observable.Observability().Snapshot(); len(got.OperationTotals) != 0 {
		t.Fatalf("disabled runtime recorded totals: %v", got.OperationTotals)
	}
	if err := b.Disconnect(); err != nil {
		t.Fatal(err)
	}
}

func TestObservabilityCategoryIsolationAndToggle(t *testing.T) {
	sink := newRecordingSink()
	b := NewNoopBroker(WithObservability(WithEventSink(sink), EnableCategories(CategoryDiagnostics)))
	obs := b.(Observable).Observability()
	if err := b.Connect(); err != nil {
		t.Fatal(err)
	}
	if err := b.Publish(context.Background(), "topic", &Message{}); err != nil {
		t.Fatal(err)
	}
	if got := obs.Snapshot().OperationTotals["publish:success"]; got != 1 {
		t.Fatalf("publish total = %d", got)
	}
	if sink.count() != 0 {
		t.Fatal("diagnostics-only configuration emitted events")
	}
	if err := obs.SetCategories(Categories(CategoryStateEvents)); err != nil {
		t.Fatal(err)
	}
	if err := b.Disconnect(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-sink.wake:
	case <-time.After(time.Second):
		t.Fatal("state event not delivered")
	}
}

func TestEveryCategoryCanBeEnabledIndependently(t *testing.T) {
	all := []Category{CategoryLogging, CategoryHealth, CategoryDiagnostics, CategoryMeasurements, CategoryCorrelation, CategoryStateEvents}
	for _, category := range all {
		t.Run(fmt.Sprint(category), func(t *testing.T) {
			opts := []ObservabilityOption{EnableCategories(category)}
			if category == CategoryLogging || category == CategoryStateEvents {
				opts = append(opts, WithEventSink(newRecordingSink()))
			}
			candidate := NewNoopBroker(WithObservability(opts...))
			if err := candidate.Connect(); err != nil {
				t.Fatal(err)
			}
			if err := candidate.Disconnect(); err != nil {
				t.Fatal(err)
			}
		})
	}
	sink := newRecordingSink()
	runtime, _ := NewRuntime("test", &ObservabilityConfig{Sink: sink, BufferCapacity: 8, OverflowPolicy: OverflowDropNewest, SinkTimeout: time.Second, ProbeTimeout: time.Second, CorrelationConflict: CorrelationPreserve})
	var group sync.WaitGroup
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			for j := 0; j < 500; j++ {
				if index%2 == 0 {
					_ = runtime.SetCategories(Categories(CategoryDiagnostics))
				} else {
					_ = runtime.SetCategories(Categories(CategoryStateEvents))
				}
				_ = runtime.Snapshot()
			}
		}(i)
	}
	group.Wait()
	runtime.Close()
}

func TestObservabilitySnapshotsAreImmutable(t *testing.T) {
	runtime, err := NewRuntime("test", nil)
	if err != nil {
		t.Fatal(err)
	}
	runtime.SetUnsupported("probe")
	first := runtime.Snapshot()
	first.Health.Capabilities["probe"] = true
	first.InFlight[OperationPublish] = 99
	first.UnsupportedCapabilities[0] = "changed"
	second := runtime.Snapshot()
	if second.Health.Capabilities["probe"] || second.InFlight[OperationPublish] == 99 || second.UnsupportedCapabilities[0] != "probe" {
		t.Fatal("snapshot exposed mutable runtime state")
	}
}

func TestObservabilityProbe(t *testing.T) {
	runtime, err := NewRuntime("test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.Probe(context.Background()); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("expected unsupported, got %v", err)
	}
	runtime.SetProbe(func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := runtime.Probe(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled, got %v", err)
	}
}

func TestObservabilityInvalidConfiguration(t *testing.T) {
	b := NewNoopBroker(WithObservability(EnableCategories(CategoryLogging)))
	if err := b.Connect(); !errors.Is(err, ErrInvalidObservabilityConfig) {
		t.Fatalf("expected invalid config, got %v", err)
	}
}

func TestObservabilityOptionValidation(t *testing.T) {
	sink := EventSinkFunc(func(context.Context, BrokerStateEvent) error { return nil })
	options := NewOptions(WithObservability(EnableCategories(CategoryLogging), WithEventSink(sink), WithEventBuffer(4, OverflowDropOldest), WithSinkTimeout(time.Second), WithProbeTimeout(time.Second), WithRedactedFields("custom"), WithCorrelationConflictPolicy(CorrelationReplace), WithInstanceID("instance")))
	runtime := NewRuntimeForOptions("test", *options)
	if runtime.ValidationError() != nil {
		t.Fatal(runtime.ValidationError())
	}
	if runtime.Snapshot().InstanceID != "instance" {
		t.Fatal("instance option not applied")
	}
	runtime.Close()
	for _, cfg := range []*ObservabilityConfig{
		{BufferCapacity: -1, OverflowPolicy: OverflowDropNewest, SinkTimeout: time.Second, ProbeTimeout: time.Second, CorrelationConflict: CorrelationPreserve},
		{BufferCapacity: 1, OverflowPolicy: "bad", SinkTimeout: time.Second, ProbeTimeout: time.Second, CorrelationConflict: CorrelationPreserve},
		{BufferCapacity: 1, OverflowPolicy: OverflowDropNewest, SinkTimeout: 0, ProbeTimeout: time.Second, CorrelationConflict: CorrelationPreserve},
		{BufferCapacity: 1, OverflowPolicy: OverflowDropNewest, SinkTimeout: time.Second, ProbeTimeout: time.Second, CorrelationConflict: "bad"},
	} {
		if _, err := NewRuntime("test", cfg); !errors.Is(err, ErrInvalidObservabilityConfig) {
			t.Fatalf("expected invalid config for %+v, got %v", cfg, err)
		}
	}
}

func TestRuntimeWrapHandlerAckNack(t *testing.T) {
	runtime := NewRuntimeForOptions("test", Options{Observability: &ObservabilityConfig{Categories: Categories(CategoryDiagnostics), BufferCapacity: 1, OverflowPolicy: OverflowDropNewest, SinkTimeout: time.Second, ProbeTimeout: time.Second, CorrelationConflict: CorrelationPreserve}, Meter: metricnoop.NewMeterProvider().Meter("test")})
	wantAck, wantNack, wantHandler := errors.New("ack"), errors.New("nack"), errors.New("handler")
	wrapped := runtime.WrapHandler(func(_ context.Context, event Event) error {
		if !errors.Is(event.Ack(), wantAck) || !errors.Is(event.Nack(true), wantNack) {
			t.Fatal("wrapped event changed errors")
		}
		return wantHandler
	})
	if err := wrapped(context.Background(), &testEvent{ackErr: wantAck, nackErr: wantNack}); !errors.Is(err, wantHandler) {
		t.Fatal(err)
	}
	snapshot := runtime.Snapshot()
	for _, key := range []string{"ack:failure", "nack:failure", "handler:failure"} {
		if snapshot.OperationTotals[key] != 1 {
			t.Fatalf("%s = %d", key, snapshot.OperationTotals[key])
		}
	}
}

func TestRuntimeOverflowAndSinkFailure(t *testing.T) {
	block := make(chan struct{})
	sink := EventSinkFunc(func(context.Context, BrokerStateEvent) error { <-block; return errors.New("sink") })
	runtime, err := NewRuntime("test", &ObservabilityConfig{Categories: Categories(CategoryStateEvents), Sink: sink, BufferCapacity: 1, OverflowPolicy: OverflowDropOldest, SinkTimeout: time.Millisecond, ProbeTimeout: time.Second, CorrelationConflict: CorrelationPreserve})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		runtime.Transition(StateReady, true, true, nil)
	}
	time.Sleep(10 * time.Millisecond)
	if runtime.Snapshot().DroppedRecords == 0 {
		t.Fatal("expected dropped records")
	}
	runtime.Close()
	close(block)
}

func TestSafeErrorRedaction(t *testing.T) {
	err := errors.New("connect https://user:password@example.test?token=abc password=hunter2")
	safe := safeError(OperationConnect, err)
	for _, secret := range []string{"user:password", "token=abc", "hunter2"} {
		if contains(safe.Summary, secret) {
			t.Fatalf("safe summary leaked %q: %s", secret, safe.Summary)
		}
	}
}

func contains(value, part string) bool {
	for i := 0; i+len(part) <= len(value); i++ {
		if value[i:i+len(part)] == part {
			return true
		}
	}
	return false
}

type legacyThirdPartyBroker struct{}

func (legacyThirdPartyBroker) Init(...Option) error { return nil }
func (legacyThirdPartyBroker) Options() Options     { return Options{} }
func (legacyThirdPartyBroker) Address() string      { return "" }
func (legacyThirdPartyBroker) Connect() error       { return nil }
func (legacyThirdPartyBroker) Disconnect() error    { return nil }
func (legacyThirdPartyBroker) Publish(context.Context, string, *Message, ...PublishOption) error {
	return nil
}
func (legacyThirdPartyBroker) Subscribe(string, Handler, ...SubscribeOption) (Subscriber, error) {
	return nil, nil
}
func (legacyThirdPartyBroker) String() string { return "third-party" }

var _ Broker = legacyThirdPartyBroker{}
