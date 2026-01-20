# Quickstart: Using Platform-Specific Extensions

This guide demonstrates how to use the refined Option pattern to leverage advanced MQ features while maintaining a unified interface.

## 1. Initializing with Mismatched Option Detection

```go
import (
    "github.com/qvcloud/broker"
    "github.com/qvcloud/broker/brokers/kafka"
)

// Initialize
b := kafka.NewBroker(
    broker.Addrs("127.0.0.1:9092"),
    // If you pass an invalid option here, a warning will be logged during Connect()
    // rocketmq.WithGroupName("some-group"), 
)

// MUST call Connect to establish I/O
if err := b.Connect(); err != nil {
    log.Fatal(err)
}
```

## 2. Publishing with Precedence

When `broker.WithShardingKey` and `rocketmq.WithShardingKey` are both used, the RocketMQ-specific one takes precedence.

```go
err := b.Publish(ctx, "orders", &broker.Message{
    Body: []byte(`{"id": 123}`), // []byte is passed as-is (Zero Copy)
}, 
    broker.WithShardingKey("global-key"),
    rocketmq.WithShardingKey("rq-specific-key"), // This wins
)
```

## 3. Smart Serialization

```go
type Order struct { ID int }

// System will use JsonMarshaler because input is NOT []byte
err := b.Publish(ctx, "topic", &broker.Message{
    Body: Order{ID: 1}, 
})
```
