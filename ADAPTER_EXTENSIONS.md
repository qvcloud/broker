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

为了保证跨驱动的一致性，所有适配器均遵循以下语义：

| 操作 | `AutoAck: true` (默认) | `AutoAck: false` | 语义说明 |
| :--- | :--- | :--- | :--- |
| **Handler 成功 (nil)** | 自动触发 `Ack()` | 不触发 `Ack()` | 业务处理成功 |
| **Handler 失败 (error)** | 自动重试 | 自动重试 | 业务异常，需重试 |
| **显式 \`event.Ack()\`** | 无需手动调用 | **必须手动调用** | 确认消息已处理 |
| **\`event.Nack(true)\`** | 手动触发重试 | 手动触发重试 | 拒绝并重新入队 |
| **\`event.Nack(false)\`** | 手动丢弃 | 手动丢弃 | 拒绝并直接丢弃 (跳过) |

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
