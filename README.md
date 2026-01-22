# QvCloud Broker

[![CI](https://github.com/qvcloud/broker/actions/workflows/ci.yml/badge.svg)](https://github.com/qvcloud/broker/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/qvcloud/broker)](https://goreportcard.com/report/github.com/qvcloud/broker)
[![License](https://img.shields.io/github/license/qvcloud/broker)](LICENSE)

[English](README_EN.md) | [中文](README.md)

QvCloud Broker 是一个生产级的 Go 语言消息中间件抽象层。它通过统一的 API 屏蔽了 Kafka、RabbitMQ、RocketMQ、NATS、Redis 和 AWS SQS 等底层差异，内置 OpenTelemetry 链路追踪，并拥有严格的代码质量保障，旨在为分布式系统提供高可靠的解耦方案。

## 核心特性

- **接口驱动**: 统一的 `Broker`, `Publisher`, `Subscriber` 接口。
- **多驱动支持**:
    | 驱动 (Driver) | 状态 (Status) | 测试覆盖率 (Coverage) | 说明 (Description) |
    | :--- | :--- | :--- | :--- |
    | **Core Framework** | ✅ 生产就绪 | **89.4%** | 库核心逻辑与通用 Options |
    | **AWS SQS** | ✅ 生产就绪 | **94.6%** | 亚马逊云队列服务 |
    | **NATS** | ✅ 生产就绪 | **91.7%** | 高性能消息系统 |
    | **Redis** | ✅ 生产就绪 | **91.6%** | 基于 **Streams** (Consumer Group) |
    | **RocketMQ** | ✅ 生产就绪 | **83.6%** | 阿里云/原生 RocketMQ |
    | **RabbitMQ** | ✅ 生产就绪 | **82.7%** | 标准 AMQP 协议 |
    | **Kafka** | ✅ 生产就绪 | **82.4%** | 基于 sarama 的高并发实现 |
    | **GCP Pub/Sub** | ✅ 已支持 | **22.3%** | 谷歌云发布订阅 |
- **可扩展性**: 插件化架构，轻松接入新的 MQ 实现。
- **统一模型**: 厂商无关的消息模型。

## 项目结构

```
.
├── broker.go          // 核心接口定义
├── message.go         // 统一消息结构
├── options.go         // 统一配置项
├── json.go            // 默认 JSON 编解码器
├── noop_broker.go     // 空实现（用于测试）
├── middleware/        // 中间件（如 OTEL）
├── brokers/           // 各 MQ 适配器实现
│   ├── rocketmq/      // RocketMQ
│   ├── kafka/         // Kafka
│   ├── rabbitmq/      // RabbitMQ
│   ├── nats/          // NATS
│   ├── redis/         // Redis Streams
│   ├── sqs/           // AWS SQS
│   └── pubsub/        // GCP Pub/Sub
└── examples/          // 使用示例
```

## 快速开始

### 1. 使用 No-op Broker (用于本地开发/测试)

```go
import "github.com/qvcloud/broker"

// 初始化
b := broker.NewNoopBroker()
b.Connect()

// 订阅
b.Subscribe("topic", func(ctx context.Context, event broker.Event) error {
    fmt.Println("Received:", string(event.Message().Body))
    return nil
})

// 发布
b.Publish(context.Background(), "topic", &broker.Message{Body: []byte("hello")})
```

### 2. 使用 RocketMQ

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

### 3. 使用 Kafka

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

### 4. 使用 RabbitMQ

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

### 5. 使用 NATS

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

### 6. 使用 AWS SQS

```go
import (
    "github.com/qvcloud/broker"
    "github.com/qvcloud/broker/brokers/sqs"
)

// SQS 使用 AWS 默认配置加载凭证和区域
b := sqs.NewBroker()
b.Connect()

// 发布到指定 Queue URL
b.Publish(ctx, "https://sqs.us-east-1.amazonaws.com/123456789012/MyQueue", msg)
```

### 7. 使用 GCP Pub/Sub

```go
import (
    "github.com/qvcloud/broker"
    "github.com/qvcloud/broker/brokers/pubsub"
)

// Addrs 传入 GCP Project ID
b := pubsub.NewBroker(
    broker.Addrs("my-gcp-project-id"),
)
b.Connect()

// 订阅时需通过 WithQueue 指定 Subscription ID
b.Subscribe("my-topic", handler, broker.WithQueue("my-subscription"))
```

### 8. 使用 Redis Streams

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

// 订阅 (使用 Consumer Group)
b.Subscribe("topic", handler, broker.Queue("my-group"))
```

### 9. 集成 OpenTelemetry


```go
import (
    "github.com/qvcloud/broker/middleware"
)

b.Subscribe("topic", middleware.OtelHandler(func(ctx context.Context, event broker.Event) error {
    // 处理逻辑...
    return nil
}))
```

### 10. 安全连接 (TLS/SSL)

本库在 Kafka, RabbitMQ 和 NATS 适配器中支持标准 TLS 配置。

```go
import (
    "crypto/tls"
    "github.com/qvcloud/broker"
)

// 加载双向 TLS 配置 (可选)
tlsConfig := &tls.Config{
    InsecureSkipVerify: false,
    // 其他字段...
}

b := kafka.NewBroker(
    broker.Addrs("kafka:9093"),
    broker.Secure(true),
    broker.TLSConfig(tlsConfig),
)
b.Connect()
```

### 💡 消息处理回调 (Handler) 的返回值说明

在 `b.Subscribe` 中定义的 Handler 函数返回的 `error` 直接影响消息的确认机制：

- **返回 `nil`**：表示消息处理成功。Broker 适配器会自动确认消息（Ack），消息将不会再次派发。
- **返回 `error`**：表示处理失败。消息将不会被确认。根据底层 MQ 的实现，该消息通常会：
    - **重新入队 (Requeue)**：如 RabbitMQ，消息会回到队列等待下次消费。
    - **等待超时重发**：如 SQS 或 GCP Pub/Sub，消息在可见性超时后会重新派发。
    - **暂停提交位点**：如 Kafka，可能会导致该分区消息堆积。

**新手建议**：对于程序逻辑错误或无法通过重试解决的错误，建议捕获异常、记录日志并返回 `nil`，或者手动将其投递至死信队列（DLQ），以避免队列因无限重试而阻塞。

## 核心设计原则

1. **接口驱动**: 保证业务逻辑与具体的 MQ 实现解耦。
2. **高性能**: 适配层保持极简，最小化性能开销。
3. **可观测性**: 原生支持 OpenTelemetry。

## 性能表现

我们对 `broker` 框架的基础损耗进行了评估，测试环境为 Apple M2 Pro (Go 1.21)。

### 1. 序列化优化 (Smart Serialization)

通过对原始数据（Bytes/String）的智能路径优化，序列化性能提升了约 **5 倍**。

| 测试场景 | 耗时 (ns/op) | 内存分配 (B/op) | 分配次数 (allocs/op) | 结论 |
| :--- | :--- | :--- | :--- | :--- |
| 标准 `json.Marshal` (Bytes) | 83.52 | 88 | 2 | 基准 |
| **智能序列化 (Bytes)** | **15.91** | **24** | **1** | **提速 ~5.2x** |
| 标准 `json.Marshal` (String) | 86.18 | 64 | 2 | 基准 |
| **智能序列化 (String)** | **16.20** | **24** | **1** | **提速 ~5.3x** |

### 2. 框架基础开销

| 测试项目 | 耗时 (ns/op) | 内存分配 (B/op) | 分配次数 (allocs/op) |
| :--- | :--- | :--- | :--- |
| `NoopBroker` 发布 | 37.94 | 80 | 2 |
| `WithTrackedValue` (首次) | 154.9 | 504 | 6 |
| `GetTrackedValue` (读取) | 29.17 | 0 | 0 |

> **注**: 选项追踪 (Option Tracking) 引入的额外开销在网络 IO 面前（秒级/毫秒级）几乎可以忽略不计，但它能显著提升开发效率，防止因配置拼写错误或跨平台参数误用导致的“静默失败”。

## 开源协议

本项目采用 [MIT License](LICENSE) 协议。你可以自由地使用、修改和分发本项目，只需保留原始版权声明。

