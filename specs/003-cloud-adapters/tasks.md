# Tasks: Cloud Adapters (AWS SQS & GCP Pub/Sub)

## Phase 1: AWS SQS Adapter (Priority: P1)

- [X] T001 Initialize SQS adapter directory `brokers/sqs/`
- [X] T002 Implement `Connect` (client initialization) for SQS in `brokers/sqs/sqs.go`
- [X] T003 Implement `Publish` (SendMessage) for SQS
- [X] T004 Implement `Subscribe` (ReceiveMessage loop) for SQS
- [X] T005 Implement `Ack` (DeleteMessage) for SQS events
- [X] T006 Add unit tests with mocking for SQS in `brokers/sqs/sqs_test.go`

## Phase 2: GCP Pub/Sub Adapter (Priority: P1)

- [X] T007 Initialize GCP Pub/Sub adapter directory `brokers/pubsub/`
- [X] T008 Implement `Connect` (client initialization) for PubSub in `brokers/pubsub/pubsub.go`
- [X] T009 Implement `Publish` for GCP Pub/Sub
- [X] T010 Implement `Subscribe` for GCP Pub/Sub
- [X] T011 Add unit tests with mocking for GCP Pub/Sub in `brokers/pubsub/pubsub_test.go`


## Phase 3: Polish & Examples

- [X] T012 Update `README.md` with Cloud Adapter configuration examples
- [X] T013 Add integration examples for SQS and GCP Pub/Sub in `examples/`
