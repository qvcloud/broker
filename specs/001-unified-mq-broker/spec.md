# Feature Specification: Unified MQ Broker for Go

**Feature Branch**: `main`  
**Created**: 2026-01-20  
**Status**: Draft  
**Input**: Universal Go Message Middleware Adapter PRD

## User Scenarios & Testing

### User Story 1 - Core Framework & MVP (Priority: P1)

As a developer, I want a standard set of interfaces for messaging so that my application logic is decoupled from any specific MQ provider.

**Why this priority**: It is the foundation of the entire project. Without the core interface, no adapters can be built.

**Independent Test**: Use the `noop_broker` to verify that `Publish` calls result in the `Handler` being executed correctly.

**Acceptance Scenarios**:
1. **Given** a Noop Broker, **When** I subscribe to "test.topic", **Then** I should receive messages published to that topic.

---

### User Story 2 - RocketMQ Adapter (Priority: P1)

As a developer using RocketMQ, I want to use the unified API to send and receive messages so that I can switch to other MQs later if needed.

**Why this priority**: Core requirement to match original project functionality.

**Independent Test**: Run a local RocketMQ instance and verify end-to-end msg flow via the `rocketmq` adapter.

---

### User Story 3 - Kafka Adapter (Priority: P1)

As a developer using Kafka, I want to use the unified API so that I don't have to deal with the complexities of the Kafka native SDK directly.

**Why this priority**: High demand and proves the "Multi-MQ" core value proposition.

---

### User Story 4 - Observability (Priority: P2)

As an SRE, I want to see metrics and traces for every message sent/received so that I can monitor system health and latency.

**Why this priority**: Required for production-grade software according to NFRs.

**Independent Test**: Check if OpenTelemetry spans are created during `Publish` and `Subscribe` calls.

---

## Requirements

### Functional Requirements

- **FR-001**: System MUST provide a `Broker` interface with `Init`, `Connect`, `Publish`, `Subscribe`.
- **FR-002**: System MUST support message headers, body, and basic routing (topics/queues).
- **FR-003**: System MUST allow configuring brokers using Functional Options.
- **FR-004**: System MUST implement a `JsonMarshaler` as the default codec.
- **FR-005**: System MUST provide at least three production-ready adapters: RocketMQ, Kafka, and NATS.

### Key Entities

- **Broker**: The coordinator for connections and message flow.
- **Message**: The data envelope containing headers and payload.
- **Event**: The object delivered to subscribers providing access to the Message and Ack/Nack logic.

## Success Criteria

### Measurable Outcomes

- **SC-001**: API overhead < 5% compared to native SDKs.
- **SC-002**: Zero business logic changes required when switching between RocketMQ and Kafka via configuration.
- **SC-003**: 100% test coverage for core interfaces and at least 80% for each adapter.
