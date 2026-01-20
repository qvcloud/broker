# Tasks: Advanced Adapters & Advanced Features

## Phase 1: RabbitMQ Adapter (Priority: P1)

- [X] T001 [P] Initialize RabbitMQ adapter directory `brokers/rabbitmq/`
- [X] T002 Implement `Connect` and `Disconnect` for RabbitMQ in `brokers/rabbitmq/rabbitmq.go`
- [X] T003 Implement `Publish` for RabbitMQ
- [X] T004 Implement `Subscribe` and exchange/queue binding in `brokers/rabbitmq/rabbitmq.go`
- [X] T005 Add unit tests for RabbitMQ in `brokers/rabbitmq/rabbitmq_test.go`

## Phase 2: NATS Adapter (Priority: P1)

- [X] T006 [P] Initialize NATS adapter directory `brokers/nats/`
- [X] T007 Implement `Connect` and `Disconnect` for NATS in `brokers/nats/nats.go`
- [X] T008 Implement `Publish` and Subject mapping in `brokers/nats/nats.go`
- [X] T009 Implement `Subscribe` and Queue Groups in `brokers/nats/nats.go`
- [X] T010 Add unit tests for NATS in `brokers/nats/nats_test.go`


## Phase 3: Advanced Features (Priority: P2)

- [X] T011 Extend `PublishOptions` with `Delay` (Duration) in `options.go`
- [X] T012 Implement Delayed message logic in RabbitMQ adapter
- [X] T013 Extend `SubscribeOptions` with `DeadLetterQueue` (String) in `options.go`
- [X] T014 Implement DLQ logic in RocketMQ adapter (matching existing functionality)

## Phase 4: Polish & Docs

- [X] T015 Update `README.md` with RabbitMQ and NATS configuration examples
- [X] T016 Add integration examples for RabbitMQ and NATS in `examples/`

