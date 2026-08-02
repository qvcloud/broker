# Phase 0 Research: Opt-in Observability and Diagnostics

## Optional capability model

**Decision**: Preserve `Broker` and expose observability through small optional interfaces detected
with Go type assertions.

**Rationale**: Adding methods to `Broker` would break every third-party implementation. Optional
interfaces are idiomatic, preserve SemVer compatibility, and let adapters explicitly expose only
supported capabilities.

**Alternatives considered**:

- Add health and diagnostics methods to `Broker`: rejected because it is source-incompatible.
- Global registry keyed by broker: rejected because ownership, cleanup, and identity become
  ambiguous and global mutable state harms tests.
- Wrapper-only instrumentation: rejected because a wrapper cannot reliably observe native
  reconnects, receive failures, or Ack/Nack internals.

## Shared runtime placement

**Decision**: Implement the state machine, snapshot aggregation, redaction, correlation helpers,
and event dispatcher in the root package; adapters feed it native outcomes.

**Rationale**: Security policy, state semantics, and concurrency behavior must be consistent across
adapters. The root package remains vendor-neutral and adapters already import it.

**Alternatives considered**:

- Duplicate a runtime in every adapter: rejected due to drift and repeated race/security risk.
- Put runtime in `middleware`: rejected because middleware cannot see all adapter lifecycle events.
- New internal package imported by root and adapters: viable, but public contracts would still need
  root facades and the extra boundary adds little value at current size.

## Configuration and runtime enablement

**Decision**: Add a nested `ObservabilityConfig` configured by functional options, with immutable
validated settings and an atomic category mask for runtime enable/disable. Categories remain off
unless explicitly enabled; existing `Logger`, `Tracer`, and `Meter` continue to work compatibly.

**Rationale**: A single namespace avoids expanding `Options` with many unrelated fields, while the
atomic mask handles concurrent toggles without rebuilding clients. Existing integrations cannot be
removed in a backward-compatible feature.

**Alternatives considered**:

- Environment variables: rejected because a library should not own process-global configuration.
- Mutable exported config fields: rejected because concurrent mutation would race and bypass
  validation.
- Require restart to change categories: simpler, but does not cover the specified in-flight toggle
  edge case.

## Logging and structured event contract

**Decision**: Retain legacy `Logger`; add a structured `EventSink` fed through a bounded asynchronous
dispatcher. Records contain typed safe fields and categorized errors, not formatted vendor objects.

**Rationale**: The existing two-method logger cannot guarantee structured safe data or isolate slow
callbacks. A separate sink enables deterministic tests and custom integrations without requiring a
logging framework dependency.

**Alternatives considered**:

- Adopt `slog` as the sole logger: rejected because replacing `Logger` is a compatibility change;
  a future adapter can bridge records to `slog`.
- Call sinks synchronously: rejected because a slow or panicking sink could alter broker latency or
  delivery behavior.
- Unbounded goroutine per record: rejected because failure storms could exhaust memory.

## Overflow and shutdown policy

**Decision**: Use a fixed-capacity queue, default `drop-newest`, optional `drop-oldest`, and an
observable dropped-record counter. Dispatcher shutdown is bounded; pending records are best effort.

**Rationale**: Broker correctness is more important than diagnostic completeness. A bounded queue
makes memory behavior predictable, while both policies cover freshness versus chronology needs.

**Alternatives considered**:

- Block producer: rejected because it couples diagnostics backpressure to message processing.
- Spill to disk: rejected as persistence is outside scope and introduces I/O/security concerns.
- Unbounded queue: rejected because it fails safely only under light load.

## Health and active probes

**Decision**: Separate synchronous passive `Health()` from cancelable `Probe(ctx)`. Readiness is
derived from adapter role/capability and can be unknown or degraded independently of connection.

**Rationale**: Read-only health must be safe for frequent service health endpoints and must not
surprise users with network traffic. Explicit probes can have cost and adapter limitations.

**Alternatives considered**:

- Probe on every health query: rejected because it adds latency, traffic, and outage amplification.
- Boolean `IsConnected`: rejected because it cannot represent degraded or role-specific readiness.
- Background periodic probe: rejected because disabled mode must have no workers/network calls.

## Metrics and dimensions

**Decision**: Reuse the configured OpenTelemetry `metric.Meter`, define a fixed signal catalog, and
allow only bounded attributes: broker system, operation, outcome, error category, and optional
adapter capability. Destination, client/message IDs, headers, and raw errors are excluded.

**Rationale**: Existing dependencies already provide a vendor-neutral export path. Fixed attributes
protect collectors from cardinality explosions and satisfy the no-built-in-backend scope.

**Alternatives considered**:

- Custom metrics registry: rejected because it duplicates aggregation/export concerns.
- Destination as a default attribute: rejected because user-created destinations may be unbounded.
- Prometheus client dependency: rejected because it forces one exporter and increases binary size.

## Correlation transport

**Decision**: Use reserved, documented W3C-compatible context headers and OpenTelemetry propagation
when configured. Preserve valid upstream context; default conflict policy is preserve-user and skip
injection with a safe diagnostic event.

**Rationale**: W3C context is interoperable and already supported by OpenTelemetry. Never silently
overwriting user metadata protects application semantics.

**Alternatives considered**:

- Broker-specific IDs: rejected because they do not correlate across adapters/services.
- Always overwrite reserved headers: rejected because it mutates user data unexpectedly.
- Put correlation in the body: rejected because it changes payload contracts.

## Error safety and redaction

**Decision**: Produce safe error categories and allowlisted summaries; apply mandatory key/value
redaction to any optional text. User redaction additions extend the non-removable default set.

**Rationale**: Raw vendor errors can embed signed URLs, credentials, addresses, or tokens. An
allowlist is safer than attempting to enumerate every secret representation.

**Alternatives considered**:

- Emit `err.Error()` unchanged: rejected due to credential leakage risk.
- Regex-only scrubbing: retained as defense in depth, but insufficient as the primary policy.
- Let users disable default redaction: rejected because secure defaults are constitutional.

## Dependency and licensing impact

**Decision**: Add no production dependency. Use the standard library and existing OpenTelemetry
modules.

**Rationale**: This preserves module isolation, licensing posture, and downstream binary size.

**Alternatives considered**:

- Add a logging or metrics framework: rejected because consumers already choose their backend.

## Adapter verification scope

**Decision**: Run Docker-backed tests for Kafka, RabbitMQ, NATS, and Redis; use mock contracts plus
documented real-service validation for RocketMQ, SQS, and Pub/Sub.

**Rationale**: This matches the constitution and existing compose environment. Cloud credentials
and a RocketMQ service are not reliably available in CI.

**Alternatives considered**:

- Mock-only for all adapters: rejected as insufficient evidence for lifecycle and propagation.
- Add all services to default Docker CI: rejected due to cloud credentials and disproportionate
  RocketMQ setup cost; this can be revisited separately.
