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
├── brokers/           // 各 MQ 适配器实现
│   ├── rocketmq/
│   ├── kafka/
│   ├── nats/
│   ├── rabbitmq/
│   └── ...
└── examples/          // 使用示例
```

## 快速开始

参考 [examples/basic/main.go](examples/basic/main.go) 中的简单示例。

```go
import "github.com/qvcloud/broker"

// 初始化
b := broker.NewNoopBroker()
b.Connect()

// 订阅
b.Subscribe("topic", func(ctx context.Context, event broker.Event) error {
    fmt.Println(string(event.Message().Body))
    return nil
})

// 发布
b.Publish(ctx, "topic", &broker.Message{Body: []byte("hello")})
```

## 贡献

欢迎提交 Issue 和 Pull Request。

## 许可证

MIT
