# Implementation Plan: Cloud Adapters (AWS SQS & GCP Pub/Sub)

**Branch**: `003-cloud-adapters` | **Date**: 2026-01-20

## Summary
In this phase, we add support for two major cloud-native messaging services: AWS SQS and GCP Pub/Sub. This expands the library's utility for serverless and cloud-heavy environments.

## Technical Context
- **AWS SQS**: Use `github.com/aws/aws-sdk-go-v2/service/sqs`.
- **GCP Pub/Sub**: Use `cloud.google.com/go/pubsub`.
- **Infrastructure**: These adapters usually require environment variables or standard cloud credential files.

## Project Structure
```text
brokers/
  ├── sqs/      (AWS Simple Queue Service)
  ├── pubsub/   (GCP Cloud Pub/Sub)
```

## Strategy
1. **AWS SQS**: Focus on the common pattern of long-polling and message deletion after successful handling.
2. **GCP Pub/Sub**: Implement topic/subscription management within the adapter or assume existing resources.
3. **Common Patterns**: Ensure these cloud adapters leverage the standard `broker.Message` and `broker.Event` interfaces, including `Ack()` mapping to cloud-specific deletion/acknowledgment.
