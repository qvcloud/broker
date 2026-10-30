# 适配器特定参数扩展指南 (Platform-Specific Extensions)

本文档整理了 `broker` 包中各个消息中间件适配器支持的特定扩展参数。所有特定参数均通过各自适配器包下的 `Option` 函数实现，以保持与底层平台逻辑的一致性。

---

## 1. 通用参数 (Global Options)

这些参数定义在 `github.com/qvcloud/broker` 包下，适用于所有适配器。

### Broker 初始化选项 (Option)
| 函数 | 说明 |
| :--- | :--- |
| `broker.Addrs(...string)` | 设置服务器地址列表 |
| `broker.ClientID(string)` | 设置客户端唯一标识（在 RabbitMQ UI/Kafka/NATS 中显示） |
| `broker.TLSConfig(*tls.Config)` | 设置 TLS 加密配置 |
| `broker.Logger(Logger)` | 注入自定义日志接口 |

### 发布选项 (PublishOption)
| 函数 | 说明 |
| :--- | :--- |
| `broker.WithShardingKey(string)` | 分区/分片 Key（Kafka Key, RQ ShardingKey, SQS GroupId） |
| `broker.WithDelay(time.Duration)` | 延时消息投递时长 |
| `broker.WithTags(...string)` | 消息标签列表 (RocketMQ Tag) |

---

## 参数解析规则与冲突处理

为了确保不同中间件适配器的健壮性，本库遵循以下参数处理规则：

### 1. 优先级策略 (Precedence)
当同时提供了通用 Option 和适配器特定 Option 时，**适配器特定 Option 具有更高优先级**。
例如，在 RocketMQ 中：
* `rocketmq.WithTag("A")` 将覆盖 `broker.WithTags("B")`。
* `rocketmq.WithShardingKey("K1")` 将覆盖 `broker.WithShardingKey("K2")`。

### 2. 延迟绑定 (Late Binding)
所有的网络连接和客户端底层初始化均在 `Connect()` 调用时发生，而非 `NewBroker` 或 `Init`。这允许在连接前动态调整所有参数。

### 3. 未消费选项警告 (Unconsumed Option Warnings)
如果用户向适配器传递了该适配器不支持的选项（例如向 Kafka 传递了 `rocketmq.WithAsync`），系统会在调用 `Connect()` 或 `Publish()` 时通过注入的 `broker.Logger` 输出警告日志：
> `Warning: option "rocketmq.WithAsync" was ignored by the implementation`

---

## 2. RocketMQ (`brokers/rocketmq`)

### 初始化选项 (Option)
| 函数 | 说明 |
| :--- | :--- |
| `rocketmq.WithGroupName(string)` | 设置生产者/消费者的组名 |
| `rocketmq.WithRetry(int)` | 设置发送消息的重试次数 |
| `rocketmq.WithNamespace(string)` | 设置实例的 Namespace |
| `rocketmq.WithTracingEnabled(bool)` | 是否开启 RocketMQ 原生链路追踪 |
| `rocketmq.WithConsumeGoroutineNums(int)` | 消费者处理消息的最大协程数 |

### 发布选项 (PublishOption)
| 函数 | 说明 |
| :--- | :--- |
| `rocketmq.WithTag(string)` | 单个消息 Tag |
| `rocketmq.WithAsync()` | 开启异步发送模式（非阻塞） |

**注**：`Publish` 时会自动映射 Header 中的 `KEYS` 到 RocketMQ 的检索索引，映射 `SHARDING_KEY` 到分区选择。

---

## 3. RabbitMQ (`brokers/rabbitmq`)

### 初始化选项 (Option)
| 函数 | 说明 |
| :--- | :--- |
| `rabbitmq.WithExchange(string)` | 设置默认的 Exchange 名称 |
| `rabbitmq.WithExchangeType(string)` | 设置 Exchange 类型 (direct, topic, fanout, headers) |
| `rabbitmq.WithDurable(bool)` | 队列是否持久化 (默认 true) |
| `rabbitmq.WithAutoDelete(bool)` | 队列在无消费者时是否自动删除 |
| `rabbitmq.WithPrefetchCount(int)` | QoS 预取数量，限制单个消费者未确认的消息数 |

### 发布选项 (PublishOption)
| 函数 | 说明 |
| :--- | :--- |
| `rabbitmq.WithPersistent(bool)` | 消息是否持久化到磁盘 |
| `rabbitmq.WithPriority(int)` | 消息优先级 (0-9) |
| `rabbitmq.WithMandatory()` | 如果消息无法路由到队列，则退回给生产者 |

---

## 4. Kafka (`brokers/kafka`)

### 初始化选项 (Option)
| 函数 | 说明 |
| :--- | :--- |
| `kafka.WithBalancer(kafka.Balancer)` | 自定义分区平衡策略 |
| `kafka.WithBatchSize(int)` | 生产者批量发送的消息字节数限制 |
| `kafka.WithAcks(int)` | 确认等级 (0:不确认, 1:Leader, -1:All) |
| `kafka.WithMinBytes/MaxBytes(int)` | 消费者单词拉取的数据量限制 |
| `kafka.WithOffset(int64)` | 消费者启动时的起始位点 |

### 发布选项 (PublishOption)
| 函数 | 说明 |
| :--- | :--- |
| `kafka.WithPartition(int)` | 强制指定发送到特定的物理分区 |

---

## 5. AWS SQS (`brokers/sqs`)

### 初始化选项 (Option)
| 函数 | 说明 |
| :--- | :--- |
| `sqs.WithWaitTimeSeconds(int32)` | SQS 长轮询等待时间 |
| `sqs.WithVisibilityTimeout(int32)` | 消息不可见超时时间 |
| `sqs.WithMaxNumberOfMessages(int32)` | 单次 ReceiveMessage 请求获取的最大消息数 |

### 发布选项 (PublishOption)
| 函数 | 说明 |
| :--- | :--- |
| `sqs.WithDeduplicationId(string)` | FIFO 队列的消息去重标识 |

---

## 6. NATS (`brokers/nats`)

### 初始化选项 (Option)
| 函数 | 说明 |
| :--- | :--- |
| `nats.WithMaxReconnect(int)` | 最大重连尝试次数 |
| `nats.WithReconnectWait(time.Duration)` | 重连等待间隔时长 |

### 发布选项 (PublishOption)
| 函数 | 说明 |
| :--- | :--- |
| `nats.WithReplyTo(string)` | 设置响应主题 (Request/Reply 模式使用) |

---

## 7. GCP Pub/Sub (`brokers/pubsub`)

### 初始化选项 (Option)
| 函数 | 说明 |
| :--- | :--- |
| `pubsub.WithMaxOutstandingMessages(int)` | 最大待处理消息并发数限制 |
| `pubsub.WithMaxOutstandingBytes(int)` | 最大待处理消息字节数限制 |
| `pubsub.WithMaxExtension(time.Duration)` | 自动延长 Ack 截止时间的最大时长 |

---

## 8. 消息确认一致性矩阵 (Ack/Nack Consistency)

依据项目宪法（Constitution §II）要求，框架规范了 Handler 结果、`AutoAck`、显式 `Ack()` 与 `Nack(requeue)` 的语义映射。无法跨中间件完全归一化的底层协议差异必须如实公开，禁止隐藏在虚假的跨 Broker 保证之下。

### 核心语义总表

| 操作 | `AutoAck: true` (默认) | `AutoAck: false` | 语义说明 |
| :--- | :--- | :--- | :--- |
| **Handler 成功 (`nil`)** | 框架自动触发 `Ack()` | 不触发 `Ack()` | 业务处理成功，确认消息已消费 |
| **Handler 失败 (`error`)** | 框架不触发 `Ack()`，触发重试/保留 | 框架不触发 `Ack()`，触发重试/保留 | 业务异常，按中间件原生策略重试或保持未确认 |
| **显式 `event.Ack()`** | 无需手动调用 | **必须手动调用** | 显式向 Broker 提交确认，完成消息消费 |
| **显式 `event.Nack(true)`** | 手动触发重试/保留 | 手动触发重试/保留 | 拒绝并重新入队或在待处理队列/PEL中保留（见下方差异说明） |
| **显式 `event.Nack(false)`** | 手动丢弃 | 手动丢弃 | 拒绝并直接丢弃（跳过位点或从队列/PEL中移除） |

---

### 各适配器原生协议映射矩阵

| 适配器 (Broker) | `Ack()` 原生协议映射 | `Nack(false)` (丢弃/跳过) | `Nack(true)` (重试/入队/保留) | 重新投递机制与差异说明 |
| :--- | :--- | :--- | :--- | :--- |
| **RabbitMQ** | `delivery.Ack(false)` | `delivery.Nack(false, false)` | `delivery.Nack(false, true)` | 原生重新入队到队列头部；若配置 DLQ，丢弃消息转入死信交换机 |
| **NATS** (JetStream) | `msg.Ack()` | `msg.Term()` | `msg.Nak()` | JetStream 服务端控制重投递与消费者退避调度 |
| **AWS SQS** | `DeleteMessage` | `DeleteMessage` | `ChangeMessageVisibility(0)` | 设置不可见超时为 0 秒，使其立即对后续 ReceiveMessage 可见 |
| **GCP Pub/Sub** | `msg.Ack()` | `msg.Ack()` | `msg.Nack()` | 触发 Google Cloud Pub/Sub 订阅原生重试与死信策略 |
| **RocketMQ** | `ConsumeSuccess` | `ConsumeSuccess` | `ConsumeRetryLater` | 原生投递至 `%RETRY%` 重试主题进行阶梯式延时重投递 |
| **Kafka** | `CommitMessages` | `CommitMessages` (提交位点) | **未提交位点 (保留在原分区)** | **存在平台差异 (Caveat)**：Kafka 为追加日志分区架构，无原生单消息重排原语。详见下文。 |
| **Redis Streams** | `XACK` | `XACK` (从 PEL 移除) | **不执行 XACK (保留在 PEL)** | **存在平台差异 (Caveat)**：Redis Streams 无原生 Nack 命令。保留在消费组 PEL 中。详见下文。 |

---

### Kafka 确认与拒绝差异说明 (Caveat)

- **底层架构约束**：Kafka 是基于只读追加日志（Append-only Log）与单调递增 Partition Offset 的流式系统。Kafka Broker 协议层不存在针对单个已拉取消息的“拒绝并重新放回队尾”原语。
- **`Nack(false)` 行为**：视为业务主动跳过该消息，调用 `CommitMessages` 提交该消息位点，后续消费不会再拉取。
- **`Nack(true)` 行为**：不提交该消息位点（返回 `nil`），该位点在当前消费会话中保持未提交状态（符合宪法 "retain" 原则）。
- **生产注意事项**：
  1. **单调位点覆盖**：由于分区消费位点是线性向前推进的，如果同分区的后续消息被处理并执行了 `Ack()`，后续位点的提交将连带将此前未提交的位点一并提交覆盖。
  2. **进程内不重拉**：当前运行的消费者拉取循环按顺序向前拉取新消息，不会在活跃循环内自动倒回重拉已读消息。
  3. **最佳实践**：若业务需实现可靠的单条消息重试或死信重排，建议通过业务逻辑将消息发布至专门的重试/死信主题（Retry / Dead Letter Topic）。

---

### Redis Streams 确认与拒绝差异说明 (Caveat)

- **底层架构约束**：Redis 适配器基于 Redis Streams Consumer Group 实现，Redis 协议本身并未提供 `XNACK` 命令。
- **`Nack(false)` 行为**：视为业务丢弃，调用 `XACK` 将该消息从当前消费组的待处理条目列表（Pending Entries List, PEL）中彻底确认并移除。
- **`Nack(true)` 行为**：不调用 `XACK`（返回 `nil`），消息继续保留在当前消费组的 PEL 中，其空闲时间（idle time）继续累计（符合宪法 "retain" 原则）。
- **生产注意事项**：
  1. **活跃循环不重复派发**：当前消费者协程的主读取循环使用 `XREADGROUP ... >` 仅拉取分配给该消费者的最新未投递消息。因此被 `Nack(true)` 的消息不会在当前运行循环中立刻被重复派发。
  2. **故障恢复与重认领**：保留在 PEL 中的消息可在消费者重启时重新加载（初始化阶段首先拉取 ID `"0"` 的 pending 消息），或者通过外部巡检/后台守护进程调用 `XPENDING` / `XCLAIM` / `XAUTOCLAIM` 将超时未确认的消息重新认领或故障转移给其他健康的消费者。

---

## 9. Redis (`brokers/redis`)

### 初始化选项 (Option)

| 函数 | 说明 |
| :--- | :--- |
| `redis.WithPassword(string)` | 设置 Redis 连接密码 |
| `redis.WithDB(int)` | 设置 Redis 数据库索引 |

### 发布选项 (PublishOption)

| 函数 | 说明 |
| :--- | :--- |
| `redis.WithMaxLen(int64)` | 设置 Stream 的最大长度 (`MAXLEN`) |

**注**：Redis 适配器基于 **Redis Streams** 实现。订阅时必须通过 `broker.Queue(groupName)` 指定 Consumer Group 名称以保证消息可靠消费（PEL 支持）。

---

## 10. 可观测能力矩阵

所有内置适配器都通过可选 `broker.Observable` 接口提供本地生命周期状态、不可变诊断快照、发布/Handler/Ack/Nack 结果、固定维度指标和 W3C 上下文传递。该能力不会加入 `broker.Broker`，第三方实现无需修改。

| 适配器 | 被动健康 | 主动探测 | 诊断/指标 | 上下文传递 | 验证方式 |
| :--- | :--- | :--- | :--- | :--- | :--- |
| Kafka | 支持 | 可取消 TCP dial | 支持 | Header | Docker 集成 |
| RabbitMQ | 支持 | 连接状态检查 | 支持 | AMQP Header | Docker 集成 |
| NATS | 支持 | `FlushWithContext` | 支持 | Header | Docker 集成 |
| Redis Streams | 支持 | `PING` | 支持 | Stream 字段 | Docker 集成 |
| RocketMQ | 支持 | 暂不支持 | 支持 | Message Property | Mock；发布前需真实服务复验 |
| AWS SQS | 支持 | 暂不支持 | 支持 | Message Attribute | Mock；发布前需真实 AWS 队列复验 |
| GCP Pub/Sub | 支持 | 暂不支持 | 支持 | Message Attribute | Mock；发布前需真实 GCP 项目复验 |

被动 `Health()` 只读取本地同步状态，不发起网络请求。`Probe(ctx)` 仅在表中明确支持时执行，并遵守调用方取消和超时；其他适配器返回可用 `errors.Is` 判断的 `broker.ErrUnsupported`。所有能力默认关闭，启用方式和故障场景见 `examples/observability/`。

### 云适配器发布前验证

`go test -count=1 ./integration -run CloudAdapterObservabilityContracts` 会在无云凭证时验证 RocketMQ、SQS 与 Pub/Sub 的被动能力、固定信号集合、上下文复制和“不支持主动探测”契约。发布版本还应分别在隔离的真实 RocketMQ NameServer、AWS 测试队列和 GCP 测试项目中运行各适配器包测试，并人工确认：连接状态可恢复、消息正文和凭证未出现在输出中、Trace Context 能随消息往返、关闭后无后台消费者残留。真实服务凭证仅通过环境或工作负载身份注入，不写入命令、日志或仓库。
