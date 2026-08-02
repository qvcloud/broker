package broker

import (
	"context"
	"fmt"
	"time"
)

// Category identifies an independently enabled observability signal.
type Category uint32

const (
	CategoryLogging Category = 1 << iota
	CategoryHealth
	CategoryDiagnostics
	CategoryMeasurements
	CategoryCorrelation
	CategoryStateEvents
)

// CategorySet is a bit set of Category values.
type CategorySet uint32

// Categories creates a set from individual categories.
func Categories(values ...Category) CategorySet {
	var set CategorySet
	for _, value := range values {
		set |= CategorySet(value)
	}
	return set
}

// Has reports whether a category is enabled.
func (s CategorySet) Has(category Category) bool { return s&CategorySet(category) != 0 }

const allCategories = CategorySet(CategoryLogging | CategoryHealth | CategoryDiagnostics | CategoryMeasurements | CategoryCorrelation | CategoryStateEvents)

// LifecycleState is the vendor-neutral lifecycle of a broker instance.
type LifecycleState string

const (
	StateUnknown      LifecycleState = "unknown"
	StateConnecting   LifecycleState = "connecting"
	StateReady        LifecycleState = "ready"
	StateDegraded     LifecycleState = "degraded"
	StateReconnecting LifecycleState = "reconnecting"
	StateStopped      LifecycleState = "stopped"
)

// Operation identifies a bounded broker operation.
type Operation string

const (
	OperationConnect     Operation = "connect"
	OperationDisconnect  Operation = "disconnect"
	OperationPublish     Operation = "publish"
	OperationReceive     Operation = "receive"
	OperationHandler     Operation = "handler"
	OperationAck         Operation = "ack"
	OperationNack        Operation = "nack"
	OperationRetry       Operation = "retry"
	OperationReconnect   Operation = "reconnect"
	OperationSubscribe   Operation = "subscribe"
	OperationUnsubscribe Operation = "unsubscribe"
	OperationProbe       Operation = "probe"
	OperationDispatch    Operation = "dispatch"
)

// Outcome is the bounded result of an operation.
type Outcome string

const (
	OutcomeSuccess     Outcome = "success"
	OutcomeFailure     Outcome = "failure"
	OutcomeCanceled    Outcome = "canceled"
	OutcomeUnsupported Outcome = "unsupported"
)

// ErrorCategory is a safe, bounded failure classification.
type ErrorCategory string

const (
	ErrorCategoryConfiguration  ErrorCategory = "configuration"
	ErrorCategoryAuthentication ErrorCategory = "authentication"
	ErrorCategoryAuthorization  ErrorCategory = "authorization"
	ErrorCategoryConnection     ErrorCategory = "connection"
	ErrorCategoryTimeout        ErrorCategory = "timeout"
	ErrorCategoryCanceled       ErrorCategory = "canceled"
	ErrorCategoryPublish        ErrorCategory = "publish"
	ErrorCategoryConsume        ErrorCategory = "consume"
	ErrorCategoryHandler        ErrorCategory = "handler"
	ErrorCategoryAck            ErrorCategory = "ack"
	ErrorCategoryNack           ErrorCategory = "nack"
	ErrorCategoryReconnect      ErrorCategory = "reconnect"
	ErrorCategoryShutdown       ErrorCategory = "shutdown"
	ErrorCategoryUnsupported    ErrorCategory = "unsupported"
	ErrorCategoryInternal       ErrorCategory = "internal"
	ErrorCategoryUnknown        ErrorCategory = "unknown"
)

// SafeError contains only sanitized diagnostic failure information.
type SafeError struct {
	Category  ErrorCategory
	Code      string
	Summary   string
	Retryable *bool
}

// HealthStatus is an immutable passive or active health result.
type HealthStatus struct {
	CheckedAt       time.Time
	State           LifecycleState
	Connected       bool
	Ready           bool
	Degraded        bool
	LastConnectedAt time.Time
	LastReadyAt     time.Time
	Error           *SafeError
	Capabilities    map[string]bool
}

// DiagnosticSnapshot is a bounded point-in-time broker diagnostic view.
type DiagnosticSnapshot struct {
	BrokerSystem            string
	InstanceID              string
	Health                  HealthStatus
	ReconnectCount          uint64
	SubscriptionCount       int64
	InFlight                map[Operation]int64
	OperationTotals         map[string]uint64
	LastFailure             *SafeError
	DroppedRecords          uint64
	UnsupportedCapabilities []string
}

// Severity identifies the importance of an event.
type Severity string

const (
	SeverityDebug Severity = "debug"
	SeverityInfo  Severity = "info"
	SeverityWarn  Severity = "warn"
	SeverityError Severity = "error"
)

// BrokerStateEvent is a safe, ordered lifecycle or failure notification.
type BrokerStateEvent struct {
	Sequence     uint64
	Timestamp    time.Time
	BrokerSystem string
	InstanceID   string
	Kind         string
	From         LifecycleState
	To           LifecycleState
	Operation    Operation
	Severity     Severity
	Outcome      Outcome
	Error        *SafeError
	Attributes   map[string]string
}

// EventSink consumes structured events outside broker operation goroutines.
type EventSink interface {
	HandleObservabilityEvent(context.Context, BrokerStateEvent) error
}

// EventSinkFunc adapts a function to EventSink.
type EventSinkFunc func(context.Context, BrokerStateEvent) error

func (f EventSinkFunc) HandleObservabilityEvent(ctx context.Context, event BrokerStateEvent) error {
	return f(ctx, event)
}

// OverflowPolicy determines which event is discarded when a buffer is full.
type OverflowPolicy string

const (
	OverflowDropNewest OverflowPolicy = "drop-newest"
	OverflowDropOldest OverflowPolicy = "drop-oldest"
)

// CorrelationConflictPolicy controls reserved metadata conflicts.
type CorrelationConflictPolicy string

const (
	CorrelationPreserve CorrelationConflictPolicy = "preserve"
	CorrelationReplace  CorrelationConflictPolicy = "replace"
)

// ObservabilityConfig is immutable after Runtime construction except for the
// category mask changed through SetCategories.
type ObservabilityConfig struct {
	Categories          CategorySet
	InstanceID          string
	Sink                EventSink
	BufferCapacity      int
	OverflowPolicy      OverflowPolicy
	SinkTimeout         time.Duration
	ProbeTimeout        time.Duration
	RedactedFields      []string
	CorrelationConflict CorrelationConflictPolicy
}

// ObservabilityOption configures optional observability.
type ObservabilityOption func(*ObservabilityConfig)

func defaultObservabilityConfig() ObservabilityConfig {
	return ObservabilityConfig{BufferCapacity: 256, OverflowPolicy: OverflowDropNewest, SinkTimeout: time.Second, ProbeTimeout: 5 * time.Second, CorrelationConflict: CorrelationPreserve}
}

// EnableCategories enables only the supplied categories in addition to any already enabled.
func EnableCategories(categories ...Category) ObservabilityOption {
	return func(config *ObservabilityConfig) { config.Categories |= Categories(categories...) }
}

func WithEventSink(sink EventSink) ObservabilityOption {
	return func(config *ObservabilityConfig) { config.Sink = sink }
}
func WithEventBuffer(capacity int, policy OverflowPolicy) ObservabilityOption {
	return func(config *ObservabilityConfig) { config.BufferCapacity, config.OverflowPolicy = capacity, policy }
}
func WithSinkTimeout(timeout time.Duration) ObservabilityOption {
	return func(config *ObservabilityConfig) { config.SinkTimeout = timeout }
}
func WithProbeTimeout(timeout time.Duration) ObservabilityOption {
	return func(config *ObservabilityConfig) { config.ProbeTimeout = timeout }
}
func WithRedactedFields(names ...string) ObservabilityOption {
	return func(config *ObservabilityConfig) { config.RedactedFields = append(config.RedactedFields, names...) }
}
func WithCorrelationConflictPolicy(policy CorrelationConflictPolicy) ObservabilityOption {
	return func(config *ObservabilityConfig) { config.CorrelationConflict = policy }
}
func WithInstanceID(id string) ObservabilityOption {
	return func(config *ObservabilityConfig) { config.InstanceID = id }
}

func validateObservabilityConfig(config ObservabilityConfig) error {
	if config.Categories&^allCategories != 0 {
		return fmt.Errorf("%w: unknown category bits", ErrInvalidObservabilityConfig)
	}
	if config.BufferCapacity < 0 || config.BufferCapacity > 1_000_000 {
		return fmt.Errorf("%w: buffer capacity out of range", ErrInvalidObservabilityConfig)
	}
	if config.SinkTimeout <= 0 || config.ProbeTimeout <= 0 {
		return fmt.Errorf("%w: timeouts must be positive", ErrInvalidObservabilityConfig)
	}
	if config.OverflowPolicy != OverflowDropNewest && config.OverflowPolicy != OverflowDropOldest {
		return fmt.Errorf("%w: unknown overflow policy", ErrInvalidObservabilityConfig)
	}
	if config.CorrelationConflict != CorrelationPreserve && config.CorrelationConflict != CorrelationReplace {
		return fmt.Errorf("%w: unknown correlation conflict policy", ErrInvalidObservabilityConfig)
	}
	if (config.Categories.Has(CategoryLogging) || config.Categories.Has(CategoryStateEvents)) && config.Sink == nil {
		return fmt.Errorf("%w: event sink required for logging or state events", ErrInvalidObservabilityConfig)
	}
	return nil
}
