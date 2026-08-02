# Tasks: Unified MQ Broker Refinement

**Feature**: Unified MQ Broker Refinement
**Story Completion Order**: Core Framework -> RocketMQ -> Kafka -> RabbitMQ -> NATS/SQS/PubSub -> Observability

## Phase 1: Setup

- [X] T001 Define `OptionTracker` and `WithTrackedValue` in `broker.go`
- [X] T002 Implement `GetTrackedValue` and `WarnUnconsumed` in `broker.go`

## Phase 2: Foundational (Core Framework Updates)

- [X] T003 Implement `Late Binding` contract in `broker.go` (ensure `Connect()` is the only entry for I/O)
- [X] T004 Implement `Smart Serialization` in `json.go` (Type Switch for `[]byte` zero-copy)
- [X] T005 Update `Options` struct in `options.go` to include `Logger` interface correctly

## Phase 3: [US1] Core Framework Implementation

**Story Goal**: Standardize interfaces and establish the base for platform extensions.
**Independent Test Criteria**: Use `noop_broker` to verify that `Publish` and `Connect` behavior follow the new rules.

- [X] T006 [P] [US1] Update `noop_broker.go` to support `Connect()` and new Option tracking
- [X] T007 [US1] Create unit tests for `OptionTracker` in `broker_test.go`
- [X] T008 [US1] Create unit tests for smart serialization and unmarshaling in `json_test.go`
- [X] T009 [P] [US1] Achieve 100% test coverage for all functions in `options.go` via `options_test.go`

## Phase 4: [US2] RocketMQ Adapter Refinement

**Story Goal**: Apply new patterns to RocketMQ (Async, Tags, Late Binding, ShardingKey precedence).
**Independent Test Criteria**: Verify that RocketMQ-specific options take precedence and unconsumed options trigger warnings.

- [X] T010 [P] [US2] Refactor `brokers/rocketmq/rocketmq.go` to use `Late Binding` (defer client creation to `Connect`)
- [X] T011 [P] [US2] Update RocketMQ options to use `WithTrackedValue`
- [X] T012 [US2] Implement precedence logic: `rocketmq.WithShardingKey` > `broker.WithShardingKey` in `brokers/rocketmq/rocketmq.go`
- [X] T013 [US2] Add `WarnUnconsumed` calls in `rocketmq.Connect()` and `rocketmq.Publish()`
- [X] T014 [US2] Implement comprehensive tests for RocketMQ (Late Binding, Async, Tags) in `brokers/rocketmq/rocketmq_test.go`

## Phase 5: [US3] Kafka Adapter Refinement

**Story Goal**: Apply new patterns to Kafka (Acks, Partition, Late Binding).

- [X] T015 [P] [US3] Refactor `brokers/kafka/kafka.go` to use `Late Binding`
- [X] T016 [P] [US3] Update Kafka options to use `WithTrackedValue`
- [X] T017 [US3] Add `WarnUnconsumed` calls in `kafka.Connect()` and `kafka.Publish()`
- [X] T018 [US3] Implement comprehensive tests for Kafka (Acks, Min/MaxBytes, Late Binding) in `brokers/kafka/kafka_test.go`

## Phase 6: RabbitMQ, NATS, SQS & PubSub Refinement

**Story Goal**: Complete the refinement for all supported adapters.

- [X] T019 [P] Refactor RabbitMQ adapter and add tests in `brokers/rabbitmq/rabbitmq.go`
- [X] T020 [P] Refactor NATS adapter and add tests in `brokers/nats/nats.go`
- [X] T021 [P] Refactor AWS SQS adapter and update tests in `brokers/sqs/sqs.go`
- [X] T022 [P] Refactor GCP PubSub adapter and add tests in `brokers/pubsub/pubsub.go`

## Phase 7: [US4] Observability & Final Polish

**Story Goal**: Ensure OpenTelemetry is correctly integrated with the new patterns.

- [X] T023 [US4] Update OTEL middleware in `middleware/otel/otel.go` to ensure context propagation with tracers
- [X] T024 [US4] Create unit tests for OpenTelemetry middleware in `middleware/otel_test.go`
- [X] T025 [US4] Verify traces for both synchronous and asynchronous publishing
- [X] T026 Update `ADAPTER_EXTENSIONS.md` to document the new warning behavior and precedence rules
- [X] T027 Final validation: Run `go test -cover` to confirm Core 100% and Adapters >80% coverage

## Implementation Strategy
1. **MVP**: Complete Phases 1, 2, and 3 to stabilize the core.
2. **Incremental Delivery**: Refactor adapters one by one, starting with RocketMQ and Kafka.
3. **Validation**: Each Phase must pass its independent tests before proceeding.

## Dependency Graph
Phase 1 -> Phase 2 -> Phase 3 -> Phase 4/5/6 -> Phase 7

## Parallel Execution Examples
- Phase 4, Phase 5, and Phase 6 can run in parallel after Phase 2 is complete.

## Phase 8: Convergence

- [ ] T028 CRITICAL Correct RabbitMQ framework-level AutoAck so handler success acknowledges and handler failure requeues under the default configuration in `brokers/rabbitmq/rabbitmq.go` and add unit/integration regression tests per Constitution II (contradicts)
- [ ] T029 CRITICAL Propagate the subscription/message context into Redis `Ack()` and `Nack()` instead of using `context.Background()` in `brokers/redis/redis.go` and cover cancellation behavior per Constitution III (contradicts)
- [ ] T030 CRITICAL Add synchronized broker-owned goroutine lifecycle tracking and bounded graceful shutdown for subscribers, handlers, and reconnect loops across `brokers/` so `Disconnect()` releases all resources before returning per Constitution III (partial)
- [ ] T031 Add connected-state and nil-message validation to every adapter, including preventing Kafka `Publish()` from dereferencing a nil writer and making Kafka `Connect()` verify broker availability in `brokers/kafka/kafka.go`, per FR-009 and Constitution III (partial)
- [ ] T032 Implement producer and consumer OpenTelemetry spans with trace-context injection/extraction for every real adapter in `brokers/` and verify publish-to-handler span linkage per US4 and Constitution IV (missing)
- [ ] T033 Implement `Options.Meter` metrics for publish, consume, Ack/Nack, errors, retries, reconnects, and latency across real adapters with deterministic tests in `brokers/` and `middleware/` per US4 and Constitution IV (missing)
- [ ] T034 Apply `Options.ErrorHandler`, Logger, ClientID, TLSConfig, tracer, meter, and context consistently where supported by every adapter, returning or warning on unsupported common options, per FR-003 and Constitution IV (partial)
- [ ] T035 Configure `JsonMarshaler` as the default codec and verify smart serialization/transparent byte passthrough is consistently applied or explicitly documented for all adapters in `options.go`, `json.go`, and `brokers/` per FR-004 and the serialization clarification (partial)
- [ ] T036 Track common publish and subscribe options such as Delay, Tags, DeadLetterQueue, Secure, and Codec so unsupported adapters emit Logger warnings and non-native async requests fall back synchronously with warnings per FR-006 and FR-008 (partial)
- [ ] T037 Add a reproducible RocketMQ real-service publish/subscribe integration test and CI or documented external execution path in `integration/` per US2 independent test and Constitution VI (missing)
- [ ] T038 Raise and enforce test coverage to 100% for core interfaces and at least 80% for every adapter, adding direct tests for Pub/Sub, RabbitMQ, and uncovered core behavior per SC-003 (partial)
- [ ] T039 Extend `integration/docker_test.go` with handler-failure redelivery, explicit Ack, `Nack(true)`, `Nack(false)`, broker restart/reconnect, and graceful-shutdown scenarios for Kafka, RabbitMQ, NATS, and Redis per Constitution II and Quality Gates (missing)
- [ ] T040 Preserve actual `SubscribeOptions` in Redis subscribers, remove closed Kafka readers from the broker registry, propagate all close errors, and make repeated Unsubscribe/Disconnect behavior deterministic in `brokers/redis/redis.go`, `brokers/kafka/kafka.go`, and other affected adapters per Constitution III (partial)
- [ ] T041 Replace unconditional retry sleeps with cancelable exponential backoff and jitter, classify permanent versus transient errors, and test cancellation in Kafka, RabbitMQ, Redis, and SQS loops per Constitution III (partial)
- [ ] T042 Align `README.md`, `README_EN.md`, and `ADAPTER_EXTENSIONS.md` with the implemented public interfaces and exact per-adapter AutoAck/Ack/Nack semantics, including removing the nonexistent Publisher interface claim, per Constitution II and VI (contradicts)
- [ ] T043 Add reproducible native-SDK comparison benchmarks and enforce or revise the documented API-overhead target in benchmark files and both README files per SC-001 and Constitution VI (missing)
- [ ] T044 Add real-service or emulator validation plans and strongest available contract tests for SQS and GCP Pub/Sub, documenting credentials, limitations, and pre-production verification in `integration/` and both README files per Constitution VI and Quality Gates (missing)
- [ ] T045 Update every example under `examples/` to handle Connect, Publish, Subscribe, Unsubscribe, and Disconnect errors and add an example compilation check to CI per Constitution VI Development Workflow (partial)
