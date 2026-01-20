# Unified MQ Broker for Go

Unified MQ Broker for Go 是一个通用的消息中间件适配包，旨在提供统一、简洁的 API 接口，消除底层 MQ 服务实现（如 RocketMQ, Kafka, RabbitMQ 等）与业务逻辑之间的耦合。

## 核心特性

- **接口驱动**: 统一的 `Broker`, `Publisher`, `Subscriber` 接口。
- **多驱动支持**: 计划支持 RocketMQ, Kafka, NATS, RabbitMQ, AWS SQS, GCP Pub/Sub 等。
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
│   ├── rocketmq/      // RocketMQ 适配器
│   ├── kafka/         // Kafka 适配器
│   └── ...
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

### 8. 集成 OpenTelemetry


```go
import (
    "github.com/qvcloud/broker/middleware"
)

b.Subscribe("topic", middleware.OtelHandler(func(ctx context.Context, event broker.Event) error {
    // 处理逻辑...
    return nil
}))
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
