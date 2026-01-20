# Spec: Advanced Adapters & Advanced Features (Phase 2)

## Overview
Phase 2 focuses on expanding the ecosystem of supported message brokers and introducing advanced messaging patterns like delayed delivery and dead-letter handling.

## Scope
- **RabbitMQ Adapter**: Full support for AMQP 0-9-1.
- **NATS Adapter**: Support for NATS Core and potentially JetStream.
- **Delayed Messaging**: Universal interface for scheduling messages.
- **Dead Letter Queue (DLQ)**: Pattern for handling poison messages consistently.

## Functional Requirements
- `RabbitMQ`: Support for exchanges, queues, and routing keys.
- `NATS`: Support for subjects and queue groups.
- `Delayed Messages`: A standard `PublishOption` to specify delay.
- `DLQ`: Subscriber options to redirect failed messages after retries.

## Technical Requirements
- Interface compatibility: Must not break the `Broker` interface defined in Phase 1.
- Dependency Isolation: RabbitMQ and NATS clients should be optional or isolated in their respective packages.
- Performance: RabbitMQ and NATS overhead should remain < 5% compared to native SDKs.
