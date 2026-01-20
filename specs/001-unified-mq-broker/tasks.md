# Tasks: Unified MQ Broker Refinement

**Feature**: Unified MQ Broker Refinement
**Story Completion Order**: Core Framework -> RocketMQ -> Kafka -> RabbitMQ -> NATS/SQS/PubSub -> Observability

## Phase 1: Setup

- [ ] T001 Define `OptionTracker` and `WithTrackedValue` in `broker.go`
- [ ] T002 Implement `GetTrackedValue` and `WarnUnconsumed` in `broker.go`

## Phase 2: Foundational (Core Framework Updates)

- [ ] T003 Implement `Late Binding` contract in `broker.go` (ensure `Connect()` is the only entry for I/O)
- [ ] T004 Implement `Smart Serialization` in `json.go` (Type Switch for `[]byte` zero-copy)
- [ ] T005 Update `Options` struct in `options.go` to include `Logger` interface correctly

## Phase 3: [US1] Core Framework Implementation

**Story Goal**: Standardize interfaces and establish the base for platform extensions.
**Independent Test Criteria**: Use `noop_broker` to verify that `Publish` and `Connect` behavior follow the new rules.

- [ ] T006 [P] [US1] Update `noop_broker.go` to support `Connect()` and new Option tracking
- [ ] T007 [US1] Create unit tests for `OptionTracker` in `broker_test.go`
- [ ] T008 [US1] Create unit tests for smart serialization in `json_test.go`

## Phase 4: [US2] RocketMQ Adapter Refinement

**Story Goal**: Apply new patterns to RocketMQ (Async, Tags, Late Binding, ShardingKey precedence).
**Independent Test Criteria**: Verify that RocketMQ-specific options take precedence and unconsumed options trigger warnings.

- [ ] T009 [P] [US2] Refactor `brokers/rocketmq/rocketmq.go` to use `Late Binding` (defer client creation to `Connect`)
- [ ] T010 [P] [US2] Update RocketMQ options to use `WithTrackedValue`
- [ ] T011 [US2] Implement precedence logic: `rocketmq.WithShardingKey` > `broker.WithShardingKey` in `brokers/rocketmq/rocketmq.go`
- [ ] T012 [US2] Add `WarnUnconsumed` calls in `rocketmq.Connect()` and `rocketmq.Publish()`

## Phase 5: [US3] Kafka Adapter Refinement

**Story Goal**: Apply new patterns to Kafka (Acks, Partition, Late Binding).

- [ ] T013 [P] [US3] Refactor `brokers/kafka/kafka.go` to use `Late Binding`
- [ ] T014 [P] [US3] Update Kafka options to use `WithTrackedValue`
- [ ] T015 [US3] Add `WarnUnconsumed` calls in `kafka.Connect()` and `kafka.Publish()`

## Phase 6: RabbitMQ, NATS, SQS & PubSub Refinement

**Story Goal**: Complete the refinement for all supported adapters.

- [ ] T016 [P] Refactor RabbitMQ adapter in `brokers/rabbitmq/rabbitmq.go`
- [ ] T017 [P] Refactor NATS adapter in `brokers/nats/nats.go`
- [ ] T018 [P] Refactor AWS SQS adapter in `brokers/sqs/sqs.go`
- [ ] T019 [P] Refactor GCP PubSub adapter in `brokers/pubsub/pubsub.go`

## Phase 7: [US4] Observability & Final Polish

**Story Goal**: Ensure OpenTelemetry is correctly integrated with the new patterns.

- [ ] T020 [US4] Update OTEL middleware in `middleware/otel/otel.go` to ensure context propagation with tracers
- [ ] T021 [US4] Verify traces for both synchronous and asynchronous publishing
- [ ] T022 Update `ADAPTER_EXTENSIONS.md` to document the new warning behavior and precedence rules

## Implementation Strategy
1. **MVP**: Complete Phases 1, 2, and 3 to stabilize the core.
2. **Incremental Delivery**: Refactor adapters one by one, starting with RocketMQ and Kafka.
3. **Validation**: Each Phase must pass its independent tests before proceeding.

## Dependency Graph
Phase 1 -> Phase 2 -> Phase 3 -> Phase 4/5/6 -> Phase 7

## Parallel Execution Examples
- Phase 4, Phase 5, and Phase 6 can run in parallel after Phase 2 is complete.
