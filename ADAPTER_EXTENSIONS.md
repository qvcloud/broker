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
