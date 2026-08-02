# QvCloud Broker

[![CI](https://github.com/qvcloud/broker/actions/workflows/ci.yml/badge.svg)](https://github.com/qvcloud/broker/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/qvcloud/broker)](https://goreportcard.com/report/github.com/qvcloud/broker)
[![License](https://img.shields.io/github/license/qvcloud/broker)](LICENSE)

[English](README_EN.md) | [中文](README.md)

QvCloud Broker 是一个面向生产场景的 Go 语言消息中间件抽象层。它通过统一的 API 屏蔽 Kafka、RabbitMQ、RocketMQ、NATS、Redis、AWS SQS 和 GCP Pub/Sub 等底层差异，并内置 OpenTelemetry 链路追踪。生产采用前仍应针对选定的消息系统完成真实环境集成测试和容量验证。

## 核心特性

- **接口驱动**: 统一的 `Broker`, `Publisher`, `Subscriber` 接口。
- **多驱动支持**:
    | 驱动 (Driver) | 状态 (Status) | 测试覆盖率 (Coverage) | 说明 (Description) |
    | :--- | :--- | :--- | :--- |
    | **Core Framework** | ✅ 单元测试验证 | **89.4%** | 库核心逻辑与通用 Options |
    | **AWS SQS** | ✅ 单元测试验证 | **94.6%** | 亚马逊云队列服务 |
    | **NATS** | ✅ 单元测试验证 | **91.7%** | 高性能消息系统 |
    | **Redis** | ✅ 单元测试验证 | **91.7%** | 基于 **Streams** (Consumer Group) |
    | **RocketMQ** | ✅ 单元测试验证 | **83.6%** | 阿里云/原生 RocketMQ |
    | **RabbitMQ** | ✅ 单元测试验证 | **79.9%** | 标准 AMQP 协议 |
    | **Kafka** | ✅ 单元测试验证 | **81.0%** | 基于 `segmentio/kafka-go` |
    | **GCP Pub/Sub** | 🧪 持续完善 | **62.0%** | 谷歌云发布订阅 |
- **可扩展性**: 插件化架构，轻松接入新的 MQ 实现。
- **统一模型**: 厂商无关的消息模型。
- **按需可观测**: 日志、健康、诊断快照、指标、上下文关联和状态事件默认关闭并可独立开启；现有 `Broker` 接口保持兼容。

## 可观测性与故障排查

可观测能力是可选功能，默认全部关闭，不会改变现有 `broker.Broker` 接口。应用可以按需独立启用健康状态、诊断快照、结构化故障、指标、Trace Context 和状态事件。

### 快速体验与效果

示例使用内存 No-op Broker，无需 Docker、MQ 服务或账号：

```bash
# 默认关闭：只有业务执行结果，没有可观测输出
go run ./examples/observability

# 查询被动健康状态和诊断快照
go run ./examples/observability -observe -health -diagnostics

# 开启全部信号并模拟发布失败
go run ./examples/observability -observe -all -scenario=publish-failure

# 主动探测；-probe 必须与 -health 一起使用
go run ./examples/observability -observe -health -probe
```

正常场景的典型输出：

```text
health state=unknown ready=false
snapshot broker=noop state=ready subscriptions=1 dropped=0 error_category=
scenario name=normal status=complete
```

发布失败时的典型输出：

```text
event kind=operation.failed operation=publish outcome=failure state=ready
snapshot broker=noop state=ready subscriptions=1 dropped=0 error_category=publish
scenario error: demonstration publish failure
```

这些输出可以直接回答“是否已连接”“是否可接收流量”“哪个操作失败”“当前订阅数是多少”“事件是否因缓冲区满而丢弃”等排障问题。输出不会包含消息正文、连接凭据、签名查询参数或原始 Trace ID。

### 在实际 Broker 中启用

下面以 Kafka 为例；同一个 `broker.WithObservability(...)` 可用于 RabbitMQ、NATS、Redis、RocketMQ、SQS、Pub/Sub 和 No-op Broker：

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/qvcloud/broker"
    "github.com/qvcloud/broker/brokers/kafka"
)

func main() {
    sink := broker.EventSinkFunc(func(_ context.Context, event broker.BrokerStateEvent) error {
        // 可替换为 slog、日志平台或告警系统；不要在这里执行耗时操作。
        fmt.Printf("kind=%s operation=%s outcome=%s state=%s\n",
            event.Kind, event.Operation, event.Outcome, event.To)
        return nil
    })

    b := kafka.NewBroker(
        broker.Addrs("127.0.0.1:9092"),
        broker.WithObservability(
            broker.EnableCategories(
                broker.CategoryHealth,
                broker.CategoryDiagnostics,
                broker.CategoryLogging,
                broker.CategoryMeasurements,
                broker.CategoryCorrelation,
                broker.CategoryStateEvents,
            ),
            broker.WithEventSink(sink),
            broker.WithEventBuffer(256, broker.OverflowDropNewest),
            broker.WithSinkTimeout(time.Second),
            broker.WithProbeTimeout(3*time.Second),
            broker.WithRedactedFields("x-company-secret"),
            broker.WithInstanceID("order-consumer-01"),
        ),
    )

    if err := b.Connect(); err != nil {
        panic(err)
    }
    defer b.Disconnect()

    observable, ok := b.(broker.Observable)
    if !ok {
        // 第三方 Broker 可以不实现该可选接口。
        return
    }

    health := observable.Observability().Health()
    fmt.Printf("state=%s connected=%t ready=%t\n",
        health.State, health.Connected, health.Ready)

    snapshot := observable.Observability().Snapshot()
    fmt.Printf("subscriptions=%d publish_success=%d last_failure=%+v\n",
        snapshot.SubscriptionCount,
        snapshot.OperationTotals["publish:success"],
        snapshot.LastFailure)
}
```

`CategoryLogging` 或 `CategoryStateEvents` 开启时必须配置 `WithEventSink`，否则 `Connect()` 返回 `broker.ErrInvalidObservabilityConfig`。事件通过有界单 worker 异步派发；Sink 阻塞、报错或 panic 不会改变 Publish、Handler、Ack、Nack 的业务结果。队列满时按照指定策略丢弃，并通过 `Snapshot().DroppedRecords` 暴露数量。

### 分类开关

| 分类 | 作用 | 主要效果 |
| :--- | :--- | :--- |
| `CategoryLogging` | 结构化故障记录 | 向 Event Sink 发送已脱敏的操作失败事件 |
| `CategoryHealth` | 健康状态 | 查询 `unknown/connecting/ready/degraded/reconnecting/stopped` |
| `CategoryDiagnostics` | 诊断快照 | 查看操作结果、订阅数、进行中操作、最后错误和丢弃数 |
| `CategoryMeasurements` | OpenTelemetry 指标 | 记录操作次数、耗时、进行中数量、重试、重连和订阅数 |
| `CategoryCorrelation` | 调用链关联 | 使用 W3C `traceparent`/`tracestate` 随消息 Header 传播上下文 |
| `CategoryStateEvents` | 状态事件 | 异步发送连接状态变化和故障事件 |

运行期间也可以安全调整分类：

```go
obs := b.(broker.Observable).Observability()

// 临时只保留健康和诊断能力
err := obs.SetCategories(broker.Categories(
    broker.CategoryHealth,
    broker.CategoryDiagnostics,
))
```

### 健康检查与主动探测

`Health()` 是纯本地、无网络 I/O 的被动查询，适合高频状态页和排障接口。`Probe(ctx)` 才会执行主动检查，并受调用方取消和 `WithProbeTimeout` 限制：

```go
ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
defer cancel()

status, err := b.(broker.Observable).Observability().Probe(ctx)
switch {
case err == nil && status.Ready:
    // 主动探测成功
case errors.Is(err, broker.ErrUnsupported):
    // 当前适配器不支持主动探测，继续使用 Health() 的被动状态
default:
    // 超时、取消或连接异常
}
```

| 适配器 | 主动探测方式 |
| :--- | :--- |
| Kafka | 可取消 TCP Dial |
| RabbitMQ | AMQP 连接状态检查 |
| NATS | `FlushWithContext` |
| Redis | `PING` |
| RocketMQ、SQS、GCP Pub/Sub | 返回 `broker.ErrUnsupported` |

### 推荐排障顺序

1. 调用 `Health()` 判断当前是未连接、降级、重连还是已停止。
2. 必要时调用有超时的 `Probe(ctx)`，区分本地状态异常和网络端点异常。
3. 读取 `Snapshot().LastFailure` 的安全错误分类，以及 `OperationTotals` 中对应操作的成功/失败次数。
4. 检查 `SubscriptionCount`、`InFlight` 和 `DroppedRecords`，判断消费者、积压操作或事件 Sink 是否异常。
5. 使用 Trace Context 在生产者与 Handler 链路间关联问题，但不要把消息 ID、Topic、用户 ID 等高基数字段作为指标维度。

完整示例开关和场景说明见 [`examples/observability`](examples/observability/README.md)，各适配器差异见 [`ADAPTER_EXTENSIONS.md`](ADAPTER_EXTENSIONS.md)。禁用模式不启动事件 worker，实测目标热路径开销低于 1%；标准启用组合的代表性吞吐开销门禁为低于 5%。

## 项目结构

```
.
├── broker.go          // 核心接口定义
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

## 质量验证

```bash
make test      # 单元测试
make race      # Go 竞态检测
make coverage  # 合并覆盖率报告
make lint      # golangci-lint 或 go vet
make integration-test # Docker 端到端测试
```

`make integration-test` 会启动固定版本的 Kafka 3.9、RabbitMQ 3.13、NATS 2.10 和 Redis 7.4，验证真实的连接、发布、消费、消息正文及消息头传递，随后自动删除容器和卷。可通过 `BROKER_KAFKA_ADDR`、`BROKER_RABBITMQ_ADDR`、`BROKER_NATS_ADDR`、`BROKER_REDIS_ADDR` 覆盖默认地址。

CI 还会执行 Docker 集成测试、依赖漏洞扫描，并要求全仓语句覆盖率不低于 75%。生产上线前仍应使用目标 Broker 版本验证 TLS、断线重连、DLQ、重复投递、容量和优雅停机；RocketMQ、SQS 与 GCP Pub/Sub 不包含在本地 Docker 套件中。

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
