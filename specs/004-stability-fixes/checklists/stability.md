# Stability Checklist

- [X] All adapters implement error backoff in subscription loops
- [X] RabbitMQ reconnection is verified
- [X] All adapters use the `Logger` interface if provided
- [X] `Ack/Nack` methods use message context instead of `context.Background()`
- [X] Unit tests pass for all modified adapters
