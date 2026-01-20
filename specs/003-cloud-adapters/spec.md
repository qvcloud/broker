# Spec: Cloud Adapters (Phase 3)

AWS SQS and GCP Pub/Sub integration.

## Goals
- Allow seamless transition between local/on-prem MQ (like RabbitMQ) and Cloud-managed services (like SQS).
- Handle the delivery-receipt acknowledgment patterns (SQS Delete, PubSub Ack).

## Non-Goals
- Full management of Cloud IAM roles (assume environment is configured).
- Supporting every SQS/PubSub edge case (focus on standard queueing/pubsub behavior).
