# Public API Contract

This contract describes the intended vendor-neutral Go surface. Exact comments may be refined
during implementation, but compatibility and behavior requirements are normative.

## Capability discovery

The existing `broker.Broker` interface remains unchanged. An adapter may implement:

```go
type Observable interface {
	Observability() Observability
}

type Observability interface {
	HealthChecker
	DiagnosticsProvider
	SetCategories(CategorySet) error
}

type HealthChecker interface {
	Health() HealthStatus
	Probe(context.Context) (HealthStatus, error)
}

type DiagnosticsProvider interface {
	Snapshot() DiagnosticSnapshot
}
```

Callers use a checked type assertion. Failure to implement `Observable` means the broker does not
support the new capability; it is not an error for existing third-party brokers.

`Health`, `Snapshot`, and returned nested values are copies. They are safe for concurrent use with
all broker methods. `Probe` returns `ErrUnsupported` when no meaningful native probe exists and
must return promptly when its context is canceled.

## Configuration

```go
func WithObservability(opts ...ObservabilityOption) Option

func EnableCategories(categories ...Category) ObservabilityOption
func WithEventSink(EventSink) ObservabilityOption
func WithEventBuffer(capacity int, policy OverflowPolicy) ObservabilityOption
func WithSinkTimeout(time.Duration) ObservabilityOption
func WithProbeTimeout(time.Duration) ObservabilityOption
func WithRedactedFields(names ...string) ObservabilityOption
func WithCorrelationConflictPolicy(CorrelationConflictPolicy) ObservabilityOption
func WithInstanceID(string) ObservabilityOption
```

All categories are disabled when `WithObservability` is absent or no category is enabled. A sink
does not implicitly enable a category. Invalid capacity, duration, instance ID, or nil required
dependency returns a configuration error from `Init`/`Connect` according to existing constructor
behavior; it never silently falls back to an unsafe value.

`SetCategories` changes only enablement. It is atomic and safe during broker operations. Enabling a
sink-backed category starts at most one bounded dispatcher; disabling the final sink-backed
category stops it without blocking broker work. Configuration values other than categories require
normal re-initialization.

Existing `WithLogger`, `Tracer`, and `Meter` remain supported. They do not become globally enabled
by this feature. The implementation documents their mapping to the new category model and avoids
double emission.

## Structured sink

```go
type EventSink interface {
	HandleObservabilityEvent(context.Context, BrokerStateEvent) error
}
```

The sink is called outside broker operation and acknowledgment goroutines. A callback error, panic,
or timeout increments `DroppedRecords` and may generate a coalesced internal warning, but cannot be
recursively delivered without bound. Event delivery is at-most-once and best-effort. Per-instance
sequence numbers expose gaps.

## Errors and unsupported behavior

The package exports errors inspectable with `errors.Is`:

```go
var ErrUnsupported = errors.New("broker: observability capability unsupported")
var ErrInvalidObservabilityConfig = errors.New("broker: invalid observability configuration")
```

An adapter that exposes `Observable` must enumerate unsupported sub-capabilities in snapshots.
Requests for unsupported active behavior return `ErrUnsupported`; passive queries still return the
best safe local information. No observability method panics because a category is disabled.

## Compatibility guarantees

- Existing `Broker`, `Event`, `Subscriber`, `Logger`, and option signatures do not change.
- Disabled mode emits no new output, starts no worker, and performs no observability network call.
- Observability cannot acknowledge, negatively acknowledge, retry, drop, or mutate a user message.
- The root API contains no vendor SDK type.
- User-provided metadata is not overwritten unless `replace` conflict policy is explicitly chosen.
