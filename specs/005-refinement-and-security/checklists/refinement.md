# Refinement Checklist

- [X] All brokers cancel active subscriptions on `Disconnect()`
- [X] RabbitMQ `AutoAck` is framework-level (safe delivery)
- [X] TLS is implemented for Kafka, NATS, and RabbitMQ
- [X] No `context.Background()` is used in subscription loops
- [X] All unit tests pass
