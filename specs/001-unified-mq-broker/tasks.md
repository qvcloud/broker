# Tasks: Unified MQ Broker for Go

**Input**: Design documents from `specs/unified-broker-go/`
**Prerequisites**: [plan.md](plan.md), [spec.md](spec.md)

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and basic structure

- [X] T001 Initialize Go module and project skeleton (Verify `go.mod` and base files)
- [X] T002 [P] Initialize directory structure for all planned brokers in `brokers/`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story implementation

- [X] T003 Finalize core `Broker` interface and `Message` struct in `broker.go`
- [X] T004 Implement functional options for `Options`, `PublishOptions`, and `SubscribeOptions` in `options.go`
- [X] T005 Implement default `JsonMarshaler` in `json.go`

---

## Phase 3: User Story 1 - Core Framework & Noop Driver (Priority: P1) 🎯 MVP

**Goal**: Provide a reference implementation and verify basic message flow abstraction.

**Independent Test**: Run `go test noop_broker_test.go` to verify publication-subscription loop.

### Implementation for User Story 1

- [X] T006 [P] [US1] Complete `noop_broker.go` with full implementation of `Broker` interface
- [X] T007 [P] [US1] Implement `noopSubscriber` and `noopEvent` logic in `noop_broker.go`
- [X] T008 [US1] Finalize basic usage example in `examples/basic/main.go`

**Checkpoint**: Core framework and Noop driver functional.

---

## Phase 4: User Story 2 - RocketMQ Adapter (Priority: P1)

**Goal**: Implement production-ready support for Apache RocketMQ.

**Independent Test**: Run integration tests in `brokers/rocketmq/rocketmq_test.go` against a RocketMQ instance.

### Implementation for User Story 2

- [X] T009 [P] [US2] Scaffold RocketMQ implementation in `brokers/rocketmq/rocketmq.go`
- [X] T010 [US2] Implement `Connect` and `Disconnect` using RocketMQ Go Client in `brokers/rocketmq/rocketmq.go`
- [X] T011 [US2] Implement `Publish` (supporting Sync/Async) in `brokers/rocketmq/rocketmq.go`
- [X] T012 [US2] Implement `Subscribe` and message handling logic in `brokers/rocketmq/rocketmq.go`
- [X] T013 [US2] Add comprehensive integration tests in `brokers/rocketmq/rocketmq_test.go`

---

## Phase 5: User Story 3 - Kafka Adapter (Priority: P1)

**Goal**: Implement production-ready support for Apache Kafka.

**Independent Test**: Verify message persistence and consumption in `brokers/kafka/kafka_test.go`.

### Implementation for User Story 3

- [X] T014 [P] [US3] Scaffold Kafka implementation in `brokers/kafka/kafka.go`
- [X] T015 [US3] Implement `Connect` and `Disconnect` using `segmentio/kafka-go` or similar in `brokers/kafka/kafka.go`
- [X] T016 [US3] Implement `Publish` with partition support in `brokers/kafka/kafka.go`
- [X] T017 [US3] Implement `Subscribe` and `Event` interface mapping in `brokers/kafka/kafka.go`
- [X] T018 [US3] Add integration tests for Kafka in `brokers/kafka/kafka_test.go`

---

## Phase 6: User Story 4 - Observability (Priority: P2)

**Goal**: Integrate Metrics and Tracing for all messaging operations.

**Independent Test**: Verify trace propagation and metric increments after a `Publish` call.

### Implementation for User Story 4

- [X] T019 [P] [US4] Add `Meter` and `Tracer` hooks to `Options` in `options.go`
- [X] T020 [US4] Implement OpenTelemetry middleware for span creation in `middleware/otel.go`
- [ ] T021 [US4] Update `noop_broker.go` to demonstrate observability integration

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Documentation and final alignment.

- [X] T022 [P] Add documentation comments to all exported symbols in all packages
- [X] T023 [P] Finalize project `README.md` with detailed driver configuration guide
- [X] T024 Perform final benchmark of `noop_broker` vs direct interface usage

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1-2**: Must complete first.
- **US1 (Phase 3)**: Independent starter.
- **US2, US3 (Phase 4-5)**: Can be implemented in parallel after Phase 2.
- **US4 (Phase 6)**: Depends on US1 completion.

### Parallel Opportunities

- All tasks marked **[P]** across different directories can run in parallel.
- RocketMQ and Kafka drivers can be developed by different people simultaneously after the core interfaces are locked.

---

## Implementation Strategy

### MVP First
Complete **US1** to establish the pattern, then move to **US2** (RocketMQ) as it's the primary requirement.

### Incremental Delivery
Each driver (RocketMQ, Kafka, etc.) represents a complete, testable increment that adds value to the ecosystem without affecting existing drivers.
