# Plan: Stability & Audit Fixes

## Problem Statement
A technical audit identified several weaknesses in the current implementation:
- SQS receive loop can spin on errors or nil client.
- RabbitMQ reconnection logic is reactive but not robust (needs active retry).
- Logger interface exists but is not used in all adapters.
- `Ack/Nack` operations sometimes use `context.Background()` instead of message context.

## Proposed Changes
1. **SQS Stability**:
   - Add backoff (1s) to the receive loop on errors.
   - Ensure the loop doesn't spin if the client is temporarily nil.
2. **RabbitMQ Reconnection**:
   - Use `NotifyClose` to detect disconnects.
   - Implement an active retry loop for dialing.
3. **Kafka Stability**:
   - Verify error handling in the consumption loop.
   - Add backoff where necessary.
4. **Logger Integration**:
   - Update all adapters (NATS, RocketMQ, etc.) to use the `Logger` from `broker.Options`.
5. **Context Propagation**:
   - Ensure `Ack` and `Nack` implementations use the context provided by the event or handler.

## File Structure
- `brokers/sqs/sqs.go`
- `brokers/rabbitmq/rabbitmq.go`
- `brokers/kafka/kafka.go`
- `brokers/nats/nats.go`
- `brokers/rocketmq/rocketmq.go`
- `brokers/pubsub/pubsub.go`
