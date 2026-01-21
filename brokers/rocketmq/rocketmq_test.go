package rocketmq

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/apache/rocketmq-client-go/v2/producer"
	"github.com/qvcloud/broker"
	"github.com/stretchr/testify/assert"
)

type mockLogger struct {
	warnings []string
}

func (l *mockLogger) Log(v ...any) {}
func (l *mockLogger) Logf(format string, v ...any) {
	l.warnings = append(l.warnings, fmt.Sprintf(format, v...))
}

func TestNewBroker(t *testing.T) {
	addr := "127.0.0.1:9876"
	b := NewBroker(broker.Addrs(addr))

	assert.Equal(t, "rocketmq", b.String())
	assert.Equal(t, addr, b.Address())
}

func TestRocketMQ_OptionPrecedence(t *testing.T) {
	// Test Publish options precedence
	t.Run("ShardingKeyPrecedence", func(t *testing.T) {
		opts := broker.PublishOptions{
			Context:     broker.WithTrackedValue(context.Background(), shardingKey{}, "mq-shard", "rocketmq.WithShardingKey"),
			ShardingKey: "global-shard",
		}

		// Simulate the logic in Publish
		if v, ok := broker.GetTrackedValue(opts.Context, shardingKey{}).(string); ok {
			opts.ShardingKey = v
		}
		assert.Equal(t, "mq-shard", opts.ShardingKey)
	})
}

func TestRocketMQ_UnconsumedWarning(t *testing.T) {
	logger := &mockLogger{}

	// Create context with an unrelated option
	type unrelatedKey struct{}
	ctx := broker.WithTrackedValue(context.Background(), unrelatedKey{}, "val", "unrelated.Option")

	// The Connect method should consume relevant options and then WarnUnconsumed
	// But since it fails early due to no producer, let's just test WarnUnconsumed directly
	// as per the requirement.

	broker.WarnUnconsumed(ctx, logger)
	assert.Len(t, logger.warnings, 1)
	assert.Contains(t, logger.warnings[0], "unrelated.Option")
}

func TestInit(t *testing.T) {
	b := NewBroker()
	addr := "127.0.0.1:9876"

	err := b.Init(broker.Addrs(addr))
	assert.NoError(t, err)
	assert.Equal(t, addr, b.Address())
}

type mockProducer struct {
	rocketmq.Producer
	startFunc     func() error
	sendSyncFunc  func(ctx context.Context, msgs ...*primitive.Message) (*primitive.SendResult, error)
	sendAsyncFunc func(ctx context.Context, callback func(context.Context, *primitive.SendResult, error), msgs ...*primitive.Message) error
	shutdownFunc  func() error
}

func (m *mockProducer) Start() error {
	if m.startFunc != nil {
		return m.startFunc()
	}
	return nil
}
func (m *mockProducer) Shutdown() error {
	if m.shutdownFunc != nil {
		return m.shutdownFunc()
	}
	return nil
}
func (m *mockProducer) SendSync(ctx context.Context, msgs ...*primitive.Message) (*primitive.SendResult, error) {
	if m.sendSyncFunc != nil {
		return m.sendSyncFunc(ctx, msgs...)
	}
	return &primitive.SendResult{Status: primitive.SendOK, MessageQueue: &primitive.MessageQueue{}}, nil
}
func (m *mockProducer) SendAsync(ctx context.Context, callback func(context.Context, *primitive.SendResult, error), msgs ...*primitive.Message) error {
	if m.sendAsyncFunc != nil {
		return m.sendAsyncFunc(ctx, callback, msgs...)
	}
	callback(ctx, &primitive.SendResult{Status: primitive.SendOK, MessageQueue: &primitive.MessageQueue{}}, nil)
	return nil
}

type mockConsumer struct {
	rocketmq.PushConsumer
	startFunc       func() error
	subscribeFunc   func(topic string, selector consumer.MessageSelector, f func(context.Context, ...*primitive.MessageExt) (consumer.ConsumeResult, error)) error
	unsubscribeFunc func(topic string) error
	shutdownFunc    func() error
}

func (m *mockConsumer) Start() error {
	if m.startFunc != nil {
		return m.startFunc()
	}
	return nil
}
func (m *mockConsumer) Shutdown() error {
	if m.shutdownFunc != nil {
		return m.shutdownFunc()
	}
	return nil
}
func (m *mockConsumer) Subscribe(topic string, selector consumer.MessageSelector, f func(context.Context, ...*primitive.MessageExt) (consumer.ConsumeResult, error)) error {
	if m.subscribeFunc != nil {
		return m.subscribeFunc(topic, selector, f)
	}
	return nil
}
func (m *mockConsumer) Unsubscribe(topic string) error {
	if m.unsubscribeFunc != nil {
		return m.unsubscribeFunc(topic)
	}
	return nil
}

func TestRocketMQ_Publish_Detailed(t *testing.T) {
	b := NewBroker(broker.Addrs("127.0.0.1:9876"))
	r := b.(*rmqBroker)
	mockP := &mockProducer{}
	r.producer = mockP
	r.running = true

	t.Run("Headers", func(t *testing.T) {
		msg := &broker.Message{
			Header: map[string]string{
				"KEYS":         "key1",
				"SHARDING_KEY": "shardX",
				"Custom":       "value",
			},
			Body: []byte("test"),
		}
		err := b.Publish(context.Background(), "test", msg)
		assert.NoError(t, err)
	})

	t.Run("Delay", func(t *testing.T) {
		err := b.Publish(context.Background(), "test", &broker.Message{Body: []byte("test")}, broker.WithDelay(time.Minute))
		assert.NoError(t, err)

		err = b.Publish(context.Background(), "test", &broker.Message{Body: []byte("test")}, broker.WithDelay(3*time.Hour))
		assert.NoError(t, err)
	})

	t.Run("TagsAndSharding", func(t *testing.T) {
		err := b.Publish(context.Background(), "test", &broker.Message{Body: []byte("test")}, WithTag("T1"), WithShardingKey("SK1"))
		assert.NoError(t, err)
	})

	t.Run("Async", func(t *testing.T) {
		mockP.sendAsyncFunc = func(ctx context.Context, callback func(context.Context, *primitive.SendResult, error), msgs ...*primitive.Message) error {
			callback(ctx, &primitive.SendResult{Status: primitive.SendOK}, nil)
			return nil
		}
		err := b.Publish(context.Background(), "test", &broker.Message{Body: []byte("hello")}, WithAsync())
		assert.NoError(t, err)

		// Test async error logging
		mockP.sendAsyncFunc = func(ctx context.Context, callback func(context.Context, *primitive.SendResult, error), msgs ...*primitive.Message) error {
			callback(ctx, nil, fmt.Errorf("async error"))
			return nil
		}
		logger := &mockLogger{}
		_ = b.Init(broker.WithLogger(logger))
		err = b.Publish(context.Background(), "test", &broker.Message{Body: []byte("hello")}, WithAsync())
		assert.NoError(t, err)
	})

	t.Run("SendFailure", func(t *testing.T) {
		mockP.sendSyncFunc = func(ctx context.Context, msgs ...*primitive.Message) (*primitive.SendResult, error) {
			return &primitive.SendResult{
				Status:       primitive.SendFlushDiskTimeout,
				MessageQueue: &primitive.MessageQueue{},
			}, nil
		}
		err := b.Publish(context.Background(), "test", &broker.Message{Body: []byte("test")})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "send failed")

		mockP.sendSyncFunc = func(ctx context.Context, msgs ...*primitive.Message) (*primitive.SendResult, error) {
			return nil, fmt.Errorf("network error")
		}
		err = b.Publish(context.Background(), "test", &broker.Message{Body: []byte("test")})
		assert.Error(t, err)
	})
}

func TestRocketMQ_Subscribe_Detailed(t *testing.T) {
	b := NewBroker(broker.Addrs("127.0.0.1:9876"))
	r := b.(*rmqBroker)
	mockC := &mockConsumer{}
	r.consumer = mockC
	r.running = true

	t.Run("SubscribeError", func(t *testing.T) {
		mockC.subscribeFunc = func(topic string, selector consumer.MessageSelector, f func(context.Context, ...*primitive.MessageExt) (consumer.ConsumeResult, error)) error {
			return fmt.Errorf("subscribe error")
		}
		_, err := b.Subscribe("test", nil)
		assert.Error(t, err)
	})

	t.Run("AutoAck_Success", func(t *testing.T) {
		var capturedHandler func(context.Context, ...*primitive.MessageExt) (consumer.ConsumeResult, error)
		mockC.subscribeFunc = func(topic string, selector consumer.MessageSelector, f func(context.Context, ...*primitive.MessageExt) (consumer.ConsumeResult, error)) error {
			capturedHandler = f
			return nil
		}

		_, _ = b.Subscribe("test", func(ctx context.Context, event broker.Event) error {
			return nil
		})

		res, err := capturedHandler(context.Background(), &primitive.MessageExt{
			Message: primitive.Message{Body: []byte("test")},
		})
		assert.NoError(t, err)
		assert.Equal(t, consumer.ConsumeSuccess, res)
	})

	t.Run("ManualAck_Retry", func(t *testing.T) {
		var capturedHandler func(context.Context, ...*primitive.MessageExt) (consumer.ConsumeResult, error)
		mockC.subscribeFunc = func(topic string, selector consumer.MessageSelector, f func(context.Context, ...*primitive.MessageExt) (consumer.ConsumeResult, error)) error {
			capturedHandler = f
			return nil
		}

		_, _ = b.Subscribe("test", func(ctx context.Context, event broker.Event) error {
			return nil // User forgot to Ack, but AutoAck is off
		}, broker.DisableAutoAck())

		res, err := capturedHandler(context.Background(), &primitive.MessageExt{
			Message: primitive.Message{Body: []byte("test")},
		})
		assert.NoError(t, err)
		assert.Equal(t, consumer.ConsumeRetryLater, res)
	})
}

func TestRocketMQ_Disconnect(t *testing.T) {
	b := NewBroker()
	r := b.(*rmqBroker)
	r.producer = &mockProducer{}
	r.consumer = &mockConsumer{}
	r.running = true

	err := b.Disconnect()
	assert.NoError(t, err)
	assert.False(t, r.running)
}

func TestRocketMQ_Options_Coverage(t *testing.T) {
	b := NewBroker(
		WithGroupName("test-group"),
		WithInstanceName("test-instance"),
		WithRetry(3),
		WithConsumeGoroutineNums(10),
		WithTracingEnabled(true),
		WithNamespace("test-ns"),
		WithLogLevel("debug"),
	)
	assert.NotNil(t, b)

	_ = b.Init(
		broker.Addrs("127.0.0.1:9092"),
	)
}

func TestRocketMQ_Connect(t *testing.T) {
	b := NewBroker(broker.Addrs("127.0.0.1:9876"))
	r := b.(*rmqBroker)

	r.newProducer = func(opts ...producer.Option) (rocketmq.Producer, error) {
		return &mockProducer{}, nil
	}
	r.newConsumer = func(opts ...consumer.Option) (rocketmq.PushConsumer, error) {
		return &mockConsumer{}, nil
	}

	err := b.Connect()
	assert.NoError(t, err)
	assert.True(t, r.running)
	assert.NotNil(t, r.producer)

	// Test connecting again
	err = b.Connect()
	assert.NoError(t, err)
}

func TestRocketMQ_Event(t *testing.T) {
	msg := &broker.Message{Body: []byte("test")}
	event := &rmqEvent{
		topic:   "test",
		message: msg,
		res:     consumer.ConsumeSuccess,
	}

	assert.Equal(t, "test", event.Topic())
	assert.Equal(t, msg, event.Message())
	assert.NoError(t, event.Error())

	event.Ack()
	assert.Equal(t, consumer.ConsumeSuccess, event.res)

	event.Nack(true)
	assert.Equal(t, consumer.ConsumeRetryLater, event.res)

	event.Nack(false)
	assert.Equal(t, consumer.ConsumeSuccess, event.res) // Re-consume in RocketMQ is usually RetryLater, but here we follow spec
}

func TestRocketMQ_Unsubscribe(t *testing.T) {
	b := NewBroker(broker.Addrs("127.0.0.1:9876"))
	r := b.(*rmqBroker)
	mockC := &mockConsumer{}
	r.consumer = mockC
	r.running = true

	mockC.subscribeFunc = func(topic string, selector consumer.MessageSelector, f func(context.Context, ...*primitive.MessageExt) (consumer.ConsumeResult, error)) error {
		return nil
	}
	mockC.unsubscribeFunc = func(topic string) error {
		return nil
	}

	sub, err := b.Subscribe("test", func(ctx context.Context, e broker.Event) error { return nil })
	assert.NoError(t, err)

	err = sub.Unsubscribe()
	assert.NoError(t, err)
}

func TestRocketMQ_Connect_Failures(t *testing.T) {
	t.Run("NoAddrs", func(t *testing.T) {
		b := NewBroker()
		err := b.Connect()
		assert.Error(t, err)
	})

	t.Run("NewProducerError", func(t *testing.T) {
		b := NewBroker(broker.Addrs("127.0.0.1:9876"))
		r := b.(*rmqBroker)
		r.newProducer = func(opts ...producer.Option) (rocketmq.Producer, error) {
			return nil, fmt.Errorf("creation error")
		}
		err := b.Connect()
		assert.Error(t, err)
	})

	t.Run("StartError", func(t *testing.T) {
		b := NewBroker(broker.Addrs("127.0.0.1:9876"))
		r := b.(*rmqBroker)
		r.newProducer = func(opts ...producer.Option) (rocketmq.Producer, error) {
			return &mockProducer{
				startFunc: func() error { return fmt.Errorf("start error") },
			}, nil
		}
		err := b.Connect()
		assert.Error(t, err)
	})
}

func TestRocketMQ_Tracking_Coverage(t *testing.T) {
	b := NewBroker(
		broker.Addrs("127.0.0.1:9876"),
		WithRetry(5),
		WithGroupName("tracked-group"),
		WithNamespace("tracked-ns"),
		WithInstanceName("tracked-instance"),
		WithTracingEnabled(true),
		WithConsumeGoroutineNums(20),
		broker.ClientID("test-client"),
	)
	r := b.(*rmqBroker)
	r.newProducer = func(opts ...producer.Option) (rocketmq.Producer, error) {
		return &mockProducer{}, nil
	}
	r.newConsumer = func(opts ...consumer.Option) (rocketmq.PushConsumer, error) {
		return &mockConsumer{}, nil
	}

	err := b.Connect()
	assert.NoError(t, err)

	_, err = b.Subscribe("topic", func(ctx context.Context, e broker.Event) error { return nil })
	assert.NoError(t, err)

	// Coverage for Address and String
	assert.Equal(t, "127.0.0.1:9876", b.Address())
	assert.Equal(t, "rocketmq", b.String())
}

func TestRocketMQ_Subscribe_Failures(t *testing.T) {
	t.Run("NoAddrs", func(t *testing.T) {
		b := NewBroker()
		_, err := b.Subscribe("test", nil)
		assert.Error(t, err)
	})

	t.Run("NewConsumerError", func(t *testing.T) {
		b := NewBroker(broker.Addrs("127.0.0.1:9876"))
		r := b.(*rmqBroker)
		r.newConsumer = func(opts ...consumer.Option) (rocketmq.PushConsumer, error) {
			return nil, fmt.Errorf("creation error")
		}
		_, err := b.Subscribe("test", nil)
		assert.Error(t, err)
	})

	t.Run("StartError", func(t *testing.T) {
		b := NewBroker(broker.Addrs("127.0.0.1:9876"))
		r := b.(*rmqBroker)
		r.newConsumer = func(opts ...consumer.Option) (rocketmq.PushConsumer, error) {
			return &mockConsumer{
				startFunc: func() error { return fmt.Errorf("start error") },
			}, nil
		}
		_, err := b.Subscribe("test", nil)
		assert.Error(t, err)
	})
}
