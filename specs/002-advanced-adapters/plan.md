# Implementation Plan: Advanced Adapters & Advanced Features

**Branch**: `002-advanced-adapters-and-features` | **Date**: 2026-01-20

## Summary
In this phase, we add RabbitMQ and NATS support and abstract advanced features like delayed messages and DLQs.

## Technical Context
- **RabbitMQ**: Use `github.com/rabbitmq/amqp091-go`.
- **NATS**: Use `github.com/nats-io/nats.go`.
- **Delayed Messages**: Implement via provider-specific mechanisms (e.g., RabbitMQ TTL/DeadLetter, Kafka headers check in app logic if needed).

## Project Structure
```text
brokers/
  ├── rabbitmq/
  ├── nats/
```

## Strategy
1. **RabbitMQ First**: Common usage in legacy and enterprise systems.
2. **NATS Second**: High-performance cloud-native messaging.
3. **Advanced Features**: Extend `PublishOptions` and `SubscribeOptions` to support delay and DLQ.
