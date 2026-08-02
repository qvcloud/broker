# Implementation Plan: Opt-in Observability and Diagnostics

**Branch**: `006-observability-diagnostics` | **Date**: 2026-08-02 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/006-observability-diagnostics/spec.md`

## Summary

Add independently selectable logging, local health, diagnostic snapshots, measurements,
correlation, and state events without changing the existing `Broker` interface or default runtime
behavior. The root package will provide vendor-neutral contracts and a concurrency-safe per-broker
diagnostics runtime. Adapters will report lifecycle and operation outcomes into that runtime and
expose it through optional capability interfaces. Structured events use a bounded, panic-isolated
dispatcher; OpenTelemetry remains an opt-in integration through the existing tracer and meter.

## Technical Context

**Language/Version**: Go 1.26.5 (minimum patched toolchain and dependency requirement)
**Primary Dependencies**: Go standard library; existing OpenTelemetry trace/metric APIs; existing
adapter SDKs (no new production dependency planned)
**Storage**: In-memory bounded counters, state, and recent safe failure only; no persistence
**Testing**: `go test`, table-driven adapter tests, `go test -race`, benchmarks, fuzz/redaction
tests, and Docker-tagged Kafka/RabbitMQ/NATS/Redis integration tests
**Target Platform**: Go library on Linux, macOS, and Windows; broker services may be local,
containerized, cloud-hosted, or externally managed
**Project Type**: Single Go module/library with adapter subpackages
**Performance Goals**: Less than 1% throughput change and no workers when disabled; less than 5%
throughput change for the standard enabled package; diagnostic failure visibility within one second
**Constraints**: Existing `Broker` implementations continue compiling; no message bodies or secrets
in signals; bounded dimensions and buffers; observer failures cannot change delivery semantics;
passive health performs no network I/O
**Scale/Scope**: Root contracts plus noop and seven adapters (Kafka, RabbitMQ, NATS, Redis,
RocketMQ, SQS, GCP Pub/Sub); validate at least 100,000 unique message IDs/destinations for bounded
cardinality

## Constitution Check

*GATE: Passed before Phase 0 and re-checked after Phase 1 design.*

- [x] **Interface-Driven**: The existing `Broker` interface is unchanged. `Observable`,
  `HealthChecker`, and `DiagnosticsProvider` are optional vendor-neutral capability interfaces.
- [x] **Reliability**: This is not a new adapter. Instrumentation surrounds operations without
  changing handler, Ack, Nack, retry, or redelivery decisions.
- [x] **Binary Efficiency**: Contracts and runtime live in the root package and import no adapter;
  adapter packages depend inward on the root package only.
- [x] **Licensing**: No new production dependency is planned; existing OpenTelemetry integration is
  reused. Any implementation-time addition requires MIT compatibility and vulnerability review.
- [x] **Options Pattern**: `WithObservability` and nested typed functional options extend `Options`;
  unsupported adapter behavior is reported rather than ignored.
- [x] **Behavioral Consistency**: Event and measurement emission observes final outcomes and cannot
  select or alter Ack/Nack behavior.
- [x] **Doc/Ex**: The plan includes a runnable example plus semantically aligned Chinese and English
  README and adapter limitation updates.

### Post-Design Re-check

Phase 1 contracts keep vendor SDK types out of the root package, preserve the existing interface,
bound asynchronous work, propagate cancellation for probes, and explicitly document unsupported
capabilities. No constitution violation or exception is required.

## Design Approach

### Public capability discovery

Callers retain a `Broker` and use type assertions against small optional interfaces. Supported
adapters expose a shared diagnostics handle; third-party broker implementations require no source
change. Disabled or unsupported features return explicit status or `ErrUnsupported`, never panic.

### Shared runtime and adapter instrumentation

The root package owns configuration validation, immutable snapshots, lifecycle transitions,
counters, redaction, correlation helpers, and bounded event dispatch. Each adapter owns the truth
about its native client and updates the shared runtime at connect/disconnect/reconnect,
publish/receive/handler/Ack/Nack/unsubscribe boundaries. This prevents eight divergent
implementations of security and concurrency policy.

### Signal delivery and isolation

Structured diagnostic/state records enter a fixed-capacity queue only when their category is
enabled. A single optional worker invokes the user sink with panic recovery. Overflow follows the
configured `drop-newest` or `drop-oldest` policy and increments a dropped-record counter. Shutdown
is bounded and never waits indefinitely for a user sink. Metrics are recorded directly through the
configured meter because OpenTelemetry instruments are non-blocking by contract; sink callbacks
never execute on broker operation goroutines.

### Health and probing

`Health()` reads synchronized local state only. `Probe(ctx)` is separate, respects cancellation,
and delegates to an adapter-specific minimal operation. Adapters without a meaningful probe return
`ErrUnsupported` while still providing local health. Readiness is role-aware and distinct from a
raw connection flag.

### Security and cardinality

Diagnostic records use an allowlist and error categorization rather than serializing configuration,
headers, vendor objects, or raw connection errors. Default secret names are always redacted and
user additions extend rather than replace them. Topic/destination, message ID, arbitrary header,
client ID, and raw error text are excluded from metric attributes. Correlation uses reserved
W3C-compatible header names, preserves an existing valid upstream context, and applies a documented
conflict policy.

## Project Structure

### Documentation (this feature)

```text
specs/006-observability-diagnostics/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── public-api.md
│   ├── signal-catalog.md
│   └── example.md
├── checklists/
│   └── requirements.md
└── tasks.md                 # generated by speckit-tasks, not this phase
```

### Source Code (repository root)

```text
broker.go                    # optional capability interfaces and sentinel errors
options.go                   # observability functional options
observability.go             # state, health, snapshot, event, and measurement contracts
observability_runtime.go     # synchronized state and bounded dispatcher
observability_redact.go      # allowlist and redaction helpers
observability_context.go     # correlation inject/extract helpers
observability_test.go
observability_benchmark_test.go

brokers/
├── kafka/kafka.go
├── rabbitmq/rabbitmq.go
├── nats/nats.go
├── redis/redis.go
├── rocketmq/rocketmq.go
├── sqs/sqs.go
└── pubsub/pubsub.go          # adapter lifecycle/operation instrumentation and probes

middleware/
├── otel.go                  # retained tracing middleware; aligned semantic names
└── otel_test.go

integration/docker_test.go   # enabled/disabled observability and recovery checks
examples/observability/main.go
examples/observability/README.md
README.md
README_EN.md
ADAPTER_EXTENSIONS.md
```

**Structure Decision**: Extend the existing single Go module. Vendor-neutral contracts and runtime
remain in the root package, while native probes and lifecycle reporting remain in adapter packages.
No service, persistence, dashboard, or second module is introduced.

## Verification Strategy

1. Contract tests exercise disabled defaults, independent category enablement, capability
   discovery, immutable snapshots, lifecycle transitions, probe cancellation, sink panic/latency,
   queue overflow, redaction, and bounded attributes.
2. Every adapter uses the same behavior table for lifecycle and operation outcome reporting;
   vendor-specific mock tests cover native error mapping and unsupported probes.
3. Docker tests verify Kafka, RabbitMQ, NATS, and Redis publish/consume signals and at least one
   disconnect/recovery path supported reliably by the container setup.
4. RocketMQ, SQS, and Pub/Sub use strongest available mocked contract tests and document the
   real-service validation procedure.
5. Benchmarks compare disabled, logging-only, and standard-enabled publish/handle paths; CI runs
   formatting, vet/lint, full unit tests, race detector, coverage, vulnerability scan, and required
   Docker integration.
6. CI compiles and smoke-runs the observability example in disabled mode, then runs its switch
   matrix through tests so every documented category and failure scenario stays executable.

## Complexity Tracking

No constitution violations require justification.
