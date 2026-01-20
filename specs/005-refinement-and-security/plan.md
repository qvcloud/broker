# Plan: Refinement & Security Fixes

## Problem Statement
The current implementation has gaps in lifetime management (leaking subscriptions on disconnect), security (ignoring TLS), and inconsistent messaging semantics (RabbitMQ AutoAck).

## Proposed Changes
1. **Lifetime Management**:
   - Introduce a parent context in each `Broker` that is cancelled on `Disconnect()`.
   - Track all `Subscriber` instances within the Broker and ensure they are stopped when the Broker is disconnected.
2. **Unified AutoAck**:
   - Standardize `AutoAck` behavior across all adapters to be "framework-level" (Ack after handler success) rather than "protocol-level" (Ack on delivery).
   - Update RabbitMQ to always consume with `autoAck: false` and explicitly Ack/Nack based on handler result.
3. **Security (TLS Support)**:
   - Implement TLS handshake in `Connect()` for Kafka, RabbitMQ, and NATS using `options.TLSConfig`.
4. **Graceful Shutdown**:
   - Ensure the subscription loops wait for the handler to finish before exiting after a cancellation signal.

## File Structure
- `brokers/sqs/sqs.go`
- `brokers/rabbitmq/rabbitmq.go`
- `brokers/kafka/kafka.go`
- `brokers/nats/nats.go`
- `brokers/rocketmq/rocketmq.go`
- `brokers/pubsub/pubsub.go`
