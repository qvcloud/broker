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

### 4. 集成 OpenTelemetry

```go
import (
    "github.com/qvcloud/broker/middleware"
)

b.Subscribe("topic", middleware.OtelHandler(func(ctx context.Context, event broker.Event) error {
    // 处理逻辑...
    return nil
}))
```

## 核心设计原则

1. **接口驱动**: 保证业务逻辑与具体的 MQ 实现解耦。
2. **高性能**: 适配层保持极简，最小化性能开销。
3. **可观测性**: 原生支持 OpenTelemetry。
