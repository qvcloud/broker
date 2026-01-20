# Tasks: Stability & Audit Fixes

## Phase 1: Cloud Adapters Polish (SQS & Pub/Sub)

- [X] T001 Implement backoff in SQS receive loop `brokers/sqs/sqs.go`
- [X] T002 Ensure SQS `Ack`/`Nack` uses proper context
- [X] T003 Ensure GCP Pub/Sub logs errors using the configured Logger `brokers/pubsub/pubsub.go`

## Phase 2: RabbitMQ & Kafka Robustness

- [X] T004 Implement active reconnection loop for RabbitMQ `brokers/rabbitmq/rabbitmq.go`
- [X] T005 Verify and improve Kafka consumer stability `brokers/kafka/kafka.go`
- [X] T006 Ensure Kafka uses the configured Logger

## Phase 3: Global Logger & Context Audit

- [X] T007 Integrate Logger into NATS adapter `brokers/nats/nats.go`
- [X] T008 Integrate Logger into RocketMQ adapter `brokers/rocketmq/rocketmq.go`
- [X] T009 Final verification of `Ack`/`Nack` context propagation across all adapters
