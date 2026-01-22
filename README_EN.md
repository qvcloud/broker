# QvCloud Broker

[![CI](https://github.com/qvcloud/broker/actions/workflows/ci.yml/badge.svg)](https://github.com/qvcloud/broker/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/qvcloud/broker)](https://goreportcard.com/report/github.com/qvcloud/broker)
[![License](https://img.shields.io/github/license/qvcloud/broker)](LICENSE)

[English](README_EN.md) | [中文](README.md)

QvCloud Broker is a production-grade messaging abstraction for Go. It provides a unified API to decouple business logic from various brokers including Kafka, RabbitMQ, RocketMQ, NATS, Redis, and AWS SQS. Featuring built-in OpenTelemetry integration and engineered for high reliability.

## Key Features

- **Interface Driven**: Unified `Broker`, `Publisher`, and `Subscriber` interfaces.
- **Multi-Driver Support**:
    | Driver | Status | Coverage | Description |
    | :--- | :--- | :--- | :--- |
    | **Core Framework** | ✅ Ready | **89.4%** | Core logic & Global Options |
    | **AWS SQS** | ✅ Ready | **94.6%** | Amazon Simple Queue Service |
    | **NATS** | ✅ Ready | **91.7%** | High-performance messaging system |
    | **Redis** | ✅ Ready | **91.6%** | Based on **Streams** (Consumer Group) |
    | **RocketMQ** | ✅ Ready | **83.6%** | Alibaba / Native RocketMQ |
    | **RabbitMQ** | ✅ Ready | **82.7%** | Standard AMQP protocol |
    | **Kafka** | ✅ Ready | **82.4%** | High-performance implementation via sarama |
    | **GCP Pub/Sub** | ✅ Supported | **22.3%** | Google Cloud Pub/Sub |
- **Extensibility**: Plugin-based architecture for easy integration of new MQ implementations.
- **Universal Model**: A vendor-agnostic message structure.

## Project Structure

```
.
├── broker.go          // Core interface definitions
├── message.go         // Unified message structure
├── options.go         // Unified configuration options
├── json.go            // Default JSON codec
├── noop_broker.go     // Mock implementation (for testing)
├── middleware/        // Middlewares (e.g., OpenTelemetry)
├── brokers/           // MQ adapter implementations
│   ├── rocketmq/      // RocketMQ
│   ├── kafka/         // Kafka
│   ├── rabbitmq/      // RabbitMQ
│   ├── nats/          // NATS
│   ├── redis/         // Redis Streams
│   ├── sqs/           // AWS SQS
│   └── pubsub/        // GCP Pub/Sub
└── examples/          // Usage examples
```

## Quick Start

### 1. Using No-op Broker (For local development/testing)

```go
import "github.com/qvcloud/broker"

// Initialize
b := broker.NewNoopBroker()
b.Connect()

// Subscribe
b.Subscribe("topic", func(ctx context.Context, event broker.Event) error {
    fmt.Println("Received:", string(event.Message().Body))
    return nil
})

// Publish
b.Publish(context.Background(), "topic", &broker.Message{Body: []byte("hello")})
```

### 2. Using RocketMQ

```go
import (
    "github.com/qvcloud/broker"
    "github.com/qvcloud/broker/brokers/rocketmq"
)

b := rocketmq.NewBroker(
    broker.Addrs("127.0.0.1:9876"),
)
b.Connect()
```

### 3. Using Kafka

```go
import (
    "github.com/qvcloud/broker"
    "github.com/qvcloud/broker/brokers/kafka"
)

b := kafka.NewBroker(
    broker.Addrs("127.0.0.1:9092"),
)
b.Connect()
```

### 4. Using RabbitMQ

```go
import (
    "github.com/qvcloud/broker"
    "github.com/qvcloud/broker/brokers/rabbitmq"
)

b := rabbitmq.NewBroker(
    broker.Addrs("amqp://guest:guest@localhost:5672/"),
)
b.Connect()
```

### 5. Using NATS

```go
import (
    "github.com/qvcloud/broker"
    "github.com/qvcloud/broker/brokers/nats"
)

b := nats.NewBroker(
    broker.Addrs("nats://localhost:4222"),
)
b.Connect()
```

### 6. Using AWS SQS

```go
import (
    "github.com/qvcloud/broker"
    "github.com/qvcloud/broker/brokers/sqs"
)

// SQS loads credentials and region from default AWS config
b := sqs.NewBroker()
b.Connect()

// Publish to a specific Queue URL
b.Publish(ctx, "https://sqs.us-east-1.amazonaws.com/123456789012/MyQueue", msg)
```

### 7. Using GCP Pub/Sub

```go
import (
    "github.com/qvcloud/broker"
    "github.com/qvcloud/broker/brokers/pubsub"
)

// Pass GCP Project ID in Addrs
b := pubsub.NewBroker(
    broker.Addrs("my-gcp-project-id"),
)
b.Connect()

// Use WithQueue to specify the Subscription ID during subscription
b.Subscribe("my-topic", handler, broker.WithQueue("my-subscription"))
```

### 8. Using Redis Streams

```go
import (
    "github.com/qvcloud/broker"
    "github.com/qvcloud/broker/brokers/redis"
)

b := redis.NewBroker(
    broker.Addrs("127.0.0.1:6379"),
    redis.WithDB(0),
    redis.WithPassword("your-password"),
)
b.Connect()

// Subscribe (using Consumer Group)
b.Subscribe("topic", handler, broker.Queue("my-group"))
```

### 9. Integrating OpenTelemetry

```go
import (
    "github.com/qvcloud/broker/middleware"
)

b.Subscribe("topic", middleware.OtelHandler(func(ctx context.Context, event broker.Event) error {
    // Handling logic...
    return nil
}))
```

### 💡 Handling Callback (Handler) Return Values

The `error` returned by the handler function in `b.Subscribe` directly affects the message acknowledgment mechanism:

- **Return `nil`**: The message was processed successfully. The broker adapter will automatically acknowledge (Ack) the message, and it will not be redelivered.
- **Return `error`**: The processing failed. The message will not be acknowledged. Depending on the underlying MQ implementation, the message will typically:
    - **Requeue**: e.g., in RabbitMQ, it returns to the queue for another attempt.
    - **Wait for Timeout**: e.g., in SQS or GCP Pub/Sub, the message becomes visible again after the Visibility Timeout expires.
    - **Pause Commit**: e.g., in Kafka, it might delay the advancement of the consumer offset.

**Pro-tip**: For logical errors or errors that cannot be fixed by retrying, it is recommended to catch the exception, log it, and return `nil`, or manually move the message to a Dead Letter Queue (DLQ) to avoid blocking the queue with infinite retries.

## Core Design Principles

1. **Interface Driven**: Ensures business logic is decoupled from specific MQ implementations.
2. **High Performance**: The adaptation layer is kept minimal to minimize overhead.
3. **Observability**: Built-in support for OpenTelemetry.

## Performance

We evaluated the baseline overhead of the `broker` framework on an Apple M2 Pro (Go 1.21).

### 1. Smart Serialization

By implementing smart paths for raw data (`[]byte`/`string`), serialization performance has been improved by approximately **5x**.

| Test Case | Latency (ns/op) | Memory (B/op) | Allocations (allocs/op) | Conclusion |
| :--- | :--- | :--- | :--- | :--- |
| Standard `json.Marshal` (Bytes) | 83.52 | 88 | 2 | Baseline |
| **Smart Serialization (Bytes)** | **15.91** | **24** | **1** | **~5.2x Faster** |
| Standard `json.Marshal` (String) | 86.18 | 64 | 2 | Baseline |
| **Smart Serialization (String)** | **16.20** | **24** | **1** | **~5.3x Faster** |

### 2. Framework Overhead

| Item | Latency (ns/op) | Memory (B/op) | Allocations (allocs/op) |
| :--- | :--- | :--- | :--- |
| `NoopBroker` Publish | 37.94 | 80 | 2 |
| `WithTrackedValue` (Initial) | 154.9 | 504 | 6 |
| `GetTrackedValue` (Read) | 29.17 | 0 | 0 |

> **Note**: The overhead introduced by **Option Tracking** is negligible compared to network I/O latency (ms/s range). However, it significantly improves developer experience by preventing "silent failures" caused by typos or cross-platform parameter misuse.

## License

This project is licensed under the [MIT License](LICENSE). You are free to use, modify, and distribute this project as long as the original copyright notice is retained.


