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
- **FR-006**: System SHOULD log a warning when an unsupported or mismatched platform-specific Option is provided to an adapter.
- **FR-007**: Platform-specific options MUST take precedence over generic (Global) options if both provide the same logical parameter.
- **FR-008**: Async publishing (`WithAsync`) MUST only be truly asynchronous if the underlying driver/SDK supports it natively; otherwise, it should fallback to synchronous execution with a warning.
- **FR-009**: Broker implementation SHOULD use "Late Binding"; `NewBroker` should only validate configuration, while actual resource initialization and connection verification MUST happen in `Connect()`.

### Key Entities

- **Broker**: The coordinator for connections and message flow.
- **Message**: The data envelope containing headers and payload.
- **Event**: The object delivered to subscribers providing access to the Message and Ack/Nack logic.

## Success Criteria

### Measurable Outcomes

- **SC-001**: API overhead < 5% compared to native SDKs.
- **SC-002**: Zero business logic changes required when switching between RocketMQ and Kafka via configuration.
- **SC-003**: 100% test coverage for core interfaces and at least 80% for each adapter.


## Clarifications

### Session 2026-01-20

- Q: 跨平台 Option 冲突处理策略 → A: Logger 警告 (如果注入了 broker.Logger，则在检测到不支持的 Option 时输出警告日志)。
- Q: 参数冲突优先级 → A: 特定覆盖通用 (适配器特定 Option 始终覆盖通用的 Global Option)。
- Q: 发布异步行为的一致性 → A: 仅限特定驱动 (只有原生支持异步的驱动，如 RocketMQ，才处理 WithAsync 参数，其他驱动应遵循 FR-006 逻辑进行警告并同步发送)。
- Q: 驱动初始化失败的回退策略 → A: Late Binding (NewBroker 仅校验参数，连接与初始化错误在 Connect() 时返回)。
- Q: 消息 Body 序列化的透明度 → A: 智能透传 ([]byte 类型默认不处理，非 []byte 类型且配置了序列化器时才执行转换)。
