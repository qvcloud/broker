# Tasks: Opt-in Observability and Diagnostics

**Input**: Design documents from `specs/006-observability-diagnostics/`
**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `contracts/`, `quickstart.md`
**Tests**: Required by the feature specification, constitution quality gates, and measurable success criteria.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel because it changes different files and has no dependency on an incomplete task in the same phase.
- **[Story]**: Maps work to one independently testable user story.
- Every task names the exact repository file or directory it changes.

## Phase 1: Setup and Baseline

**Purpose**: Establish reproducible pre-feature behavior and shared test organization.

- [X] T001 Record disabled-mode publish/handle benchmark baselines with Go version and environment metadata in `observability_benchmark_test.go`
- [X] T002 [P] Add reusable fake event sink, fake meter/tracer, controllable clock, and goroutine leak assertions in `observability_test.go`
- [X] T003 [P] Add an adapter-neutral observability conformance test harness skeleton in `internal/obstest/conformance.go`
- [X] T004 Run the pre-change unit, race, coverage, and example compilation gates and record reproducible baseline results in `specs/006-observability-diagnostics/baseline.md`

---

## Phase 2: Foundational Contracts and Runtime

**Purpose**: Implement the vendor-neutral types and safe runtime that block every user story.

**⚠️ CRITICAL**: No user-story implementation starts until this phase passes root-package tests and the race detector.

- [X] T005 [P] Write compile-time and behavior tests for optional capability discovery, sentinel errors, immutable returned values, and third-party `Broker` compatibility in `observability_test.go`
- [X] T006 [P] Write validation tests for category masks, nested functional options, buffer policy, timeouts, instance identity, redaction additions, and conflict policy in `options_test.go`
- [X] T007 [P] Write table and fuzz tests for mandatory secret removal, allowlisted safe errors, length bounds, signed URLs, private keys, and custom protected fields in `observability_redact_test.go`
- [X] T008 Define `Observable`, `Observability`, `HealthChecker`, and `DiagnosticsProvider` optional interfaces plus `ErrUnsupported` and `ErrInvalidObservabilityConfig` in `broker.go`
- [X] T009 Define lifecycle states, categories, operations, outcomes, safe errors, health status, diagnostic snapshots, state events, overflow policies, and sink contracts in `observability.go`
- [X] T010 Implement `ObservabilityConfig`, nested option types, validation, disabled defaults, and atomic runtime category configuration in `options.go`
- [X] T011 Implement mandatory default redaction, user redaction merging, safe error classification, allowlisting, and bounded summaries in `observability_redact.go`
- [X] T012 Implement the synchronized per-broker state machine, immutable snapshot copying, fixed operation counters, in-flight gauges, timestamps, and unsupported capability tracking in `observability_runtime.go`
- [X] T013 Implement runtime construction and cleanup helpers that start no worker or network activity when all categories are disabled in `observability_runtime.go`
- [X] T014 Run root contract, fuzz seed, and race tests for the foundation and document any intentional unsupported behavior in `specs/006-observability-diagnostics/baseline.md`

**Checkpoint**: Public contracts compile, third-party implementations remain compatible, defaults are inert, and the shared runtime is race-free.

---

## Phase 3: User Story 1 - Enable Only Needed Observability (Priority: P1) 🎯 MVP

**Goal**: Users independently enable only the required observability categories while existing applications remain behaviorally unchanged.

**Independent Test**: Construct unconfigured, logging-only, diagnostics-only, and runtime-toggled noop brokers; verify category isolation, zero disabled output/workers, and unchanged publish/subscribe/Ack/Nack outcomes.

### Tests for User Story 1

- [X] T015 [P] [US1] Write failing table-driven tests for absent configuration, each category alone, `-all` equivalent masks, and concurrent category toggles in `observability_test.go`
- [X] T016 [P] [US1] Write failing noop broker tests for optional capability discovery, disabled output, independent enablement, and unchanged delivery outcomes in `noop_broker_test.go`
- [X] T017 [P] [US1] Add a compile-only external third-party `Broker` implementation proving the existing interface remains unchanged in `internal/obstest/compatibility_test.go`

### Implementation for User Story 1

- [X] T018 [US1] Implement atomic `SetCategories` behavior, category dependency validation, and safe disabled-query responses in `observability_runtime.go`
- [X] T019 [US1] Attach an optional observability runtime to the noop broker and report its local lifecycle and message operation outcomes in `noop_broker.go`
- [X] T020 [US1] Implement the base `examples/observability/main.go` command with `-observe`, `-all`, independent category flags, validation, bounded timeout, normal scenario, and deterministic output prefixes
- [X] T021 [US1] Add table-driven CLI parsing and disabled/category-isolation smoke tests for the example in `examples/observability/main_test.go`
- [X] T022 [US1] Re-run disabled and enabled benchmarks and enforce the less-than-1% disabled-overhead target in `observability_benchmark_test.go`

**Checkpoint**: The MVP supports opt-in discovery and independent toggles on the noop broker, with a runnable switch example and no compatibility break.

---

## Phase 4: User Story 2 - Troubleshoot Broker Failures (Priority: P1)

**Goal**: Operators receive safe, structured, correlated failure context for lifecycle and message operations across adapters.

**Independent Test**: Inject connection, publish, consume, handler, Ack/Nack, reconnect, unsubscribe, and shutdown errors; verify safe records contain broker, instance, operation, time, state, severity, and category with no body or secret disclosure.

### Tests for User Story 2

- [X] T023 [P] [US2] Write failure-record contract tests covering every required operation, safe field bounds, one-second availability, and immutable snapshots in `observability_failure_test.go`
- [X] T024 [P] [US2] Add secret-bearing connection strings, tokens, signed queries, custom headers, raw vendor errors, and message-body non-disclosure cases in `observability_redact_test.go`
- [X] T025 [P] [US2] Add connection, publish, consume, handler, Ack/Nack, unsubscribe, and shutdown failure cases to the adapter harness in `internal/obstest/conformance.go`
- [X] T026 [US2] Extend the example tests for `connect-failure`, `publish-failure`, and `handler-failure`, including secret/body absence and unchanged delivery behavior, in `examples/observability/main_test.go`

### Implementation for User Story 2

- [X] T027 [US2] Implement structured failure recording, last-safe-failure retention, severity mapping, monotonic sequence assignment, and one-second enqueue guarantees in `observability_runtime.go`
- [X] T028 [P] [US2] Instrument Kafka lifecycle, publish, receive, handler, Ack/Nack, unsubscribe, and shutdown outcomes in `brokers/kafka/kafka.go` and cover mappings in `brokers/kafka/kafka_test.go`
- [X] T029 [P] [US2] Instrument RabbitMQ lifecycle, reconnect, publish, receive, handler, Ack/Nack, unsubscribe, and shutdown outcomes in `brokers/rabbitmq/rabbitmq.go` and cover mappings in `brokers/rabbitmq/rabbitmq_test.go`
- [X] T030 [P] [US2] Instrument NATS lifecycle, publish, receive, handler, Ack/Nack, unsubscribe, and shutdown outcomes in `brokers/nats/nats.go` and cover mappings in `brokers/nats/nats_test.go`
- [X] T031 [P] [US2] Instrument Redis lifecycle, publish, receive, handler, Ack/Nack, retry, unsubscribe, and shutdown outcomes in `brokers/redis/redis.go` and cover mappings in `brokers/redis/redis_test.go`
- [X] T032 [P] [US2] Instrument RocketMQ lifecycle, publish, consume, handler, retry, unsubscribe, and shutdown outcomes in `brokers/rocketmq/rocketmq.go` and cover mappings in `brokers/rocketmq/rocketmq_test.go`
- [X] T033 [P] [US2] Instrument SQS lifecycle, publish, receive, handler, Ack/Nack, retry, unsubscribe, and shutdown outcomes in `brokers/sqs/sqs.go` and cover mappings in `brokers/sqs/sqs_test.go`
- [X] T034 [P] [US2] Instrument GCP Pub/Sub lifecycle, publish, receive, handler, Ack/Nack, unsubscribe, and shutdown outcomes in `brokers/pubsub/pubsub.go` and cover mappings in `brokers/pubsub/pubsub_test.go`
- [X] T035 [US2] Implement deterministic failure scenarios and safe snapshot printing in `examples/observability/main.go`
- [X] T036 [US2] Run the adapter conformance suite and resolve operation-name, error-category, or redaction inconsistencies in `internal/obstest/conformance_test.go`

**Checkpoint**: Each adapter exposes actionable failure diagnostics without leaking secrets or changing broker delivery semantics.

---

## Phase 5: User Story 3 - Query Health and Runtime State (Priority: P2)

**Goal**: Service owners query consistent passive health and explicit cancelable active probes across adapters.

**Independent Test**: Query before connection, while connecting, when ready/degraded/reconnecting, after recovery, and after shutdown; verify passive queries perform no I/O and unsupported/canceled probes are explicit.

### Tests for User Story 3

- [X] T037 [P] [US3] Write lifecycle transition, readiness-versus-connection, immutable health, concurrent query, and passive-no-I/O tests in `observability_health_test.go`
- [X] T038 [P] [US3] Add probe deadline, caller cancellation, configured timeout, degraded result, and `ErrUnsupported` cases to `internal/obstest/conformance.go`
- [X] T039 [P] [US3] Add health/probe switch validation and before-connect/after-connect/after-disconnect output tests in `examples/observability/main_test.go`

### Implementation for User Story 3

- [X] T040 [US3] Implement passive `Health`, role-aware readiness, adapter probe registration, deadline composition, and unsupported probe handling in `observability_runtime.go`
- [X] T041 [P] [US3] Implement native passive-state reporting and a minimal cancelable active probe for Kafka in `brokers/kafka/kafka.go`
- [X] T042 [P] [US3] Implement native passive-state reporting and a minimal cancelable active probe for RabbitMQ in `brokers/rabbitmq/rabbitmq.go`
- [X] T043 [P] [US3] Implement native passive-state reporting and a minimal cancelable active probe for NATS in `brokers/nats/nats.go`
- [X] T044 [P] [US3] Implement native passive-state reporting and a minimal cancelable active probe for Redis in `brokers/redis/redis.go`
- [X] T045 [P] [US3] Implement best-effort passive state and explicitly supported or unsupported active probing for RocketMQ in `brokers/rocketmq/rocketmq.go`
- [X] T046 [P] [US3] Implement best-effort passive state and explicitly supported or unsupported active probing for SQS in `brokers/sqs/sqs.go`
- [X] T047 [P] [US3] Implement best-effort passive state and explicitly supported or unsupported active probing for GCP Pub/Sub in `brokers/pubsub/pubsub.go`
- [X] T048 [US3] Add `-health` and `-probe` execution, timeout handling, and deterministic state output to `examples/observability/main.go`

**Checkpoint**: All adapters provide the same passive health workflow and clearly declare active-probe limitations.

---

## Phase 6: User Story 4 - Monitor Performance and Message Flow (Priority: P2)

**Goal**: SREs observe bounded counters, timings, in-flight work, retries/reconnects, and portable publish-to-handler correlation.

**Independent Test**: Run successful and failing flows with 100,000 unique IDs and destinations; verify totals/durations/gauges, bounded attributes, and correlation continuity where metadata is preserved.

### Tests for User Story 4

- [X] T049 [P] [US4] Write metric catalog tests for operation totals, failures, durations, retries, reconnects, subscriptions, in-flight balance, and fixed attributes in `observability_metrics_test.go`
- [X] T050 [P] [US4] Write 100,000-identity/destination cardinality tests that reject forbidden metric dimensions in `observability_metrics_test.go`
- [X] T051 [P] [US4] Write W3C correlation inject/extract, invalid context, preserve/replace conflict, absent metadata, and immutable message-header tests in `observability_context_test.go`
- [X] T052 [P] [US4] Add cross-adapter metric and correlation cases to the shared harness in `internal/obstest/conformance.go`

### Implementation for User Story 4

- [X] T053 [US4] Implement fixed-name OpenTelemetry instruments, bounded attributes, monotonic durations, and balanced in-flight accounting in `observability_runtime.go`
- [X] T054 [US4] Implement W3C-compatible correlation injection/extraction and preserve/replace conflict policies without baggage propagation in `observability_context.go`
- [X] T055 [P] [US4] Align producer/consumer span semantics and avoid duplicate emission in `middleware/otel.go` with regression coverage in `middleware/otel_test.go`
- [X] T056 [P] [US4] Map correlation metadata and measurement outcomes for Kafka and RabbitMQ in `brokers/kafka/kafka.go` and `brokers/rabbitmq/rabbitmq.go`
- [X] T057 [P] [US4] Map correlation metadata and measurement outcomes for NATS and Redis in `brokers/nats/nats.go` and `brokers/redis/redis.go`
- [X] T058 [P] [US4] Map correlation metadata and measurement outcomes for RocketMQ, SQS, and Pub/Sub in `brokers/rocketmq/rocketmq.go`, `brokers/sqs/sqs.go`, and `brokers/pubsub/pubsub.go`
- [X] T059 [US4] Add `-metrics` and `-correlation` output and validation to `examples/observability/main.go`
- [X] T060 [US4] Benchmark the standard enabled package and enforce the less-than-5% throughput target in `observability_benchmark_test.go`

**Checkpoint**: Aggregate monitoring is bounded and consistent, while supported transports preserve correlation without mutating user metadata unexpectedly.

---

## Phase 7: User Story 5 - Observe State Changes Programmatically (Priority: P3)

**Goal**: Advanced users consume ordered state/failure events without allowing observers to affect broker operations.

**Independent Test**: Trigger lifecycle and operation events with normal, slow, erroring, panicking, and blocked sinks under both overflow policies; verify ordering/gaps, bounded memory, prompt shutdown, and unchanged Ack/Nack outcomes.

### Tests for User Story 5

- [X] T061 [P] [US5] Write event-kind, ordering, sequence-gap, disabled-worker, dynamic-toggle, and immutable-event tests in `observability_event_test.go`
- [X] T062 [P] [US5] Write slow/error/panic/blocked sink tests for timeout isolation, recursion prevention, bounded shutdown, and unchanged delivery in `observability_event_test.go`
- [X] T063 [P] [US5] Write drop-newest/drop-oldest capacity, drop-reason counter, and failure-storm memory-bound tests in `observability_event_test.go`

### Implementation for User Story 5

- [X] T064 [US5] Implement the single-worker bounded event dispatcher, timeout context, panic recovery, recursion guard, and bounded shutdown in `observability_runtime.go`
- [X] T065 [US5] Implement drop-newest/drop-oldest queue behavior, dropped-record measurements, and coalesced drop notices in `observability_runtime.go`
- [X] T066 [US5] Route lifecycle transitions and operation failures already reported by all adapters into the shared event dispatcher in `observability_runtime.go`
- [X] T067 [US5] Add `-events` and `-all` event-sink demonstrations with deterministic sorted output in `examples/observability/main.go`
- [X] T068 [US5] Complete the example switch/scenario matrix and goroutine-cleanup tests in `examples/observability/main_test.go`

**Checkpoint**: State observation is useful under normal load and fails safely under overload or hostile sink behavior.

---

## Phase 8: Integration, Documentation, and Cross-Cutting Quality

**Purpose**: Prove adapter behavior end-to-end and finish all constitutional quality gates.

- [X] T069 [P] Extend Docker integration coverage for enabled/disabled signals, correlation, handler failure, Ack/Nack invariance, and deterministic recovery across Kafka, RabbitMQ, NATS, and Redis in `integration/docker_test.go`
- [X] T070 [P] Add strongest available mocked observability contracts and real-service validation instructions for RocketMQ, SQS, and Pub/Sub in `integration/cloud_observability_test.go` and `ADAPTER_EXTENSIONS.md`
- [X] T071 [P] Document per-adapter health probes, correlation transport, reconnect visibility, and unsupported capabilities in `ADAPTER_EXTENSIONS.md`
- [X] T072 [P] Add the full switch table, normal/failure commands, safe output guidance, and troubleshooting walkthrough in `examples/observability/README.md`
- [X] T073 [P] Add semantically aligned Chinese observability enablement, signal, security, overhead, and example documentation in `README.md`
- [X] T074 [P] Add semantically aligned English observability enablement, signal, security, overhead, and example documentation in `README_EN.md`
- [X] T075 Add example build and disabled/category/failure smoke commands to pull-request checks in `.github/workflows/pull-request.yml`
- [X] T076 Add observability Docker scenarios and sanitized failure log capture to `.github/workflows/integration.yml`
- [X] T077 Run `gofmt`, dependency verification, vet/lint, unit tests without cache, race detector, at-least-75% coverage, vulnerability scan, and example compilation; record results in `specs/006-observability-diagnostics/validation.md`
- [X] T078 Execute every command and verify every expected outcome in `specs/006-observability-diagnostics/quickstart.md`, then record platform-specific limitations in `specs/006-observability-diagnostics/validation.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 Setup**: Starts immediately.
- **Phase 2 Foundation**: Depends on Phase 1 and blocks all user stories.
- **US1 / Phase 3**: Depends on Phase 2 and is the MVP.
- **US2 / Phase 4**: Depends on Phase 2; complete before final adapter conformance and before US5 routes failure events.
- **US3 / Phase 5**: Depends on Phase 2; may proceed alongside US2 once shared runtime contracts stabilize.
- **US4 / Phase 6**: Depends on Phase 2; correlation adapter edits should be sequenced after overlapping US2/US3 adapter edits.
- **US5 / Phase 7**: Depends on the Phase 2 dispatcher contracts and consumes lifecycle/failure reporting from US2.
- **Phase 8 Polish**: Depends on all selected stories; full validation requires US1-US5.

### User Story Completion Order

```text
Setup → Foundation → US1 (MVP)
                         ├──→ US2 ──→ US5
                         ├──→ US3
                         └──→ US4
US2 + US3 + US4 + US5 ──→ Integration and quality gates
```

### Within Each User Story

- Write the listed tests first and confirm they fail for the intended missing behavior.
- Implement root runtime behavior before adapter integration that calls it.
- Run the story's root, noop, adapter, example, and race tests at its checkpoint.
- Do not mark a story complete when a metric, event, or diagnostic path changes delivery decisions.

### Parallel Opportunities

- T002 and T003 can run in parallel after T001 begins; T005-T007 can run in parallel.
- After Phase 2, US2 and US3 test design can proceed in parallel with US1 example work.
- Adapter tasks T028-T034 are parallel after T027; T041-T047 are parallel after T040.
- Metric/correlation adapter groups T056-T058 are parallel after T053-T054.
- Event test tasks T061-T063 are parallel before T064-T065.
- Documentation and integration tasks T069-T074 are parallel after their underlying stories finish.

## Parallel Execution Examples

### User Story 1

```text
T015: Root category and toggle tests in observability_test.go
T016: Noop behavior tests in noop_broker_test.go
T017: Third-party compatibility compile test in internal/obstest/compatibility_test.go
```

### User Story 2

```text
After T027, implement T028-T034 concurrently, one adapter per worker.
Run T036 only after all selected adapter tasks pass their package tests.
```

### User Story 3

```text
After T040, implement T041-T047 concurrently, preserving explicit unsupported results where a
provider lacks a meaningful low-impact probe.
```

### User Story 4

```text
T049: Metric catalog tests
T050: High-cardinality stress tests
T051: Correlation contract tests
T052: Adapter conformance cases
```

### User Story 5

```text
T061: Ordering and enablement tests
T062: Hostile sink isolation tests
T063: Overflow and memory-bound tests
```

## Implementation Strategy

### MVP First

1. Complete Setup and Foundation.
2. Complete US1 through T022.
3. Stop and validate disabled compatibility, independent toggles, noop behavior, and the base example.
4. Demo the MVP without requiring Docker or external credentials.

### Incremental Delivery

1. Add US2 safe failure diagnostics and adapter mappings.
2. Add US3 passive health and explicit probes.
3. Add US4 bounded measurements and correlation.
4. Add US5 isolated state-event delivery.
5. Complete Docker/cloud evidence, documentation, CI, security, performance, and quickstart gates.

### Implementation Invocation

After reviewing this task list, run `$speckit-implement` to execute tasks in dependency order. Each
task or tightly related test/implementation pair should be committed separately so regressions can
be bisected.

## Notes

- No new production dependency is planned; any exception requires constitution review.
- Existing `Broker`, `Event`, `Subscriber`, and `Logger` interfaces must remain source compatible.
- Never use raw vendor errors, message bodies, addresses, identifiers, or arbitrary headers as
  observability output or metric dimensions.
- Docker-backed evidence is mandatory when common lifecycle, acknowledgment, or adapter behavior
  changes for Kafka, RabbitMQ, NATS, or Redis.
