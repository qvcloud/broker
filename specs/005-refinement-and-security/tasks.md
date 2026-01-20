# Tasks: Refinement & Security

## Phase 1: Lifetime Management & Graceful Shutdown

- [X] T001 Implement parent context in `sqsBroker` and cancel on `Disconnect`
- [X] T002 Implement parent context in `rmqBroker` and update `Disconnect`
- [X] T003 Implement parent context in `kafkaBroker` and update `Disconnect`
- [X] T004 Implement parent context in `natsBroker`, `rocketmqBroker`, and `pubsubBroker`
- [X] T005 Update `Subscribe` loops to ensure they finish processing current messages before exiting

## Phase 2: Unified AutoAck (RabbitMQ focus)

- [X] T006 Update RabbitMQ `ch.Consume` to always use `autoAck: false`
- [X] T007 Ensure `rmqBroker` handles manual Ack/Nack correctly in all scenarios

## Phase 3: Security (TLS Implementation)

- [X] T008 Implement TLS support in `brokers/rabbitmq/rabbitmq.go`
- [X] T009 Implement TLS support in `brokers/kafka/kafka.go`
- [X] T010 Implement TLS support in `brokers/nats/nats.go`

## Phase 4: Final Validation

- [ ] T011 Verify all tests pass
- [ ] T012 Update README with TLS configuration details
