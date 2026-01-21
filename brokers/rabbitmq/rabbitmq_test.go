package rabbitmq

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/qvcloud/broker"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
)

type mockConn struct {
	channelFunc  func() (rabbitChannel, error)
	closeFunc    func() error
	isClosedFunc func() bool
}

func (m *mockConn) Channel() (rabbitChannel, error) {
	if m.channelFunc != nil {
		return m.channelFunc()
	}
	return nil, nil
}
func (m *mockConn) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}
func (m *mockConn) IsClosed() bool {
	if m.isClosedFunc != nil {
		return m.isClosedFunc()
	}
	return false
}

type mockChannel struct {
	publishFunc         func(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error
	consumeFunc         func(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error)
	queueDeclareFunc    func(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error)
	exchangeDeclareFunc func(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error
	queueBindFunc       func(name, key, exchange string, noWait bool, args amqp.Table) error
	closeFunc           func() error
	qosFunc             func(prefetchCount, prefetchSize int, global bool) error

	deliveries chan amqp.Delivery
}

func (m *mockChannel) PublishWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error {
	if m.publishFunc != nil {
		return m.publishFunc(ctx, exchange, key, mandatory, immediate, msg)
	}
	return nil
}

func (m *mockChannel) Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error) {
	if m.consumeFunc != nil {
		return m.consumeFunc(queue, consumer, autoAck, exclusive, noLocal, noWait, args)
	}
	return m.deliveries, nil
}

func (m *mockChannel) QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error) {
	if m.queueDeclareFunc != nil {
		return m.queueDeclareFunc(name, durable, autoDelete, exclusive, noWait, args)
	}
	return amqp.Queue{}, nil
}

func (m *mockChannel) ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error {
	if m.exchangeDeclareFunc != nil {
		return m.exchangeDeclareFunc(name, kind, durable, autoDelete, internal, noWait, args)
	}
	return nil
}

func (m *mockChannel) QueueBind(name, key, exchange string, noWait bool, args amqp.Table) error {
	if m.queueBindFunc != nil {
		return m.queueBindFunc(name, key, exchange, noWait, args)
	}
	return nil
}

func (m *mockChannel) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func (m *mockChannel) Qos(prefetchCount, prefetchSize int, global bool) error {
	if m.qosFunc != nil {
		return m.qosFunc(prefetchCount, prefetchSize, global)
	}
	return nil
}

func TestRabbitMQ_Basic(t *testing.T) {
	b := NewBroker(broker.Addrs("amqp://guest:guest@localhost:5672/"))
	assert.Equal(t, "amqp://guest:guest@localhost:5672/", b.Address())
	assert.Equal(t, "rabbitmq", b.String())

	err := b.Init()
	assert.NoError(t, err)
}

func TestRabbitMQ_Connect_Disconnect(t *testing.T) {
	b := NewBroker(broker.Addrs("amqp://localhost"))
	r := b.(*rmqBroker)

	mockC := &mockConn{}
	mockCh := &mockChannel{}
	r.newConn = func(addr string, config amqp.Config) (rabbitConn, error) {
		return mockC, nil
	}
	mockC.channelFunc = func() (rabbitChannel, error) {
		return mockCh, nil
	}

	err := b.Connect()
	assert.NoError(t, err)
	assert.True(t, r.running)

	err = b.Disconnect()
	assert.NoError(t, err)
	assert.False(t, r.running)
}

func TestRabbitMQ_Publish(t *testing.T) {
	b := NewBroker(broker.Addrs("amqp://localhost"))
	r := b.(*rmqBroker)
	mockCh := &mockChannel{}
	r.channel = mockCh
	r.running = true

	t.Run("Success", func(t *testing.T) {
		var capturedKey string
		mockCh.publishFunc = func(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error {
			capturedKey = key
			return nil
		}

		err := b.Publish(context.Background(), "test-topic", &broker.Message{Body: []byte("hello")})
		assert.NoError(t, err)
		assert.Equal(t, "test-topic", capturedKey)
	})

	t.Run("WithOptions", func(t *testing.T) {
		trackedCtx := broker.TrackOptions(context.Background())
		err := b.Publish(context.Background(), "test-topic", &broker.Message{Body: []byte("hello")},
			WithPriority(5),
			WithPersistent(true),
			WithMandatory(),
			broker.PublishContext(trackedCtx),
		)
		assert.NoError(t, err)
	})
}

func TestRabbitMQ_Subscribe(t *testing.T) {
	b := NewBroker(broker.Addrs("amqp://localhost"))
	r := b.(*rmqBroker)
	mockC := &mockConn{}
	mockCh := &mockChannel{}
	r.conn = mockC
	r.running = true
	r.ctx, r.cancel = context.WithCancel(context.Background())

	mockC.channelFunc = func() (rabbitChannel, error) {
		return mockCh, nil
	}

	t.Run("ConsumeAndAck", func(t *testing.T) {
		deliveryChan := make(chan amqp.Delivery, 1)
		mockCh.consumeFunc = func(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error) {
			return deliveryChan, nil
		}

		handlerCalled := make(chan struct{})
		_, err := b.Subscribe("test", func(ctx context.Context, event broker.Event) error {
			close(handlerCalled)
			return nil
		}, broker.WithQueue("test-queue"))
		assert.NoError(t, err)

		deliveryChan <- amqp.Delivery{
			RoutingKey: "test",
			Body:       []byte("world"),
		}

		select {
		case <-handlerCalled:
		case <-time.After(time.Second * 2):
			t.Fatal("handler not called")
		}
	})
}

func TestRabbitMQ_Options(t *testing.T) {
	trackedCtx := broker.TrackOptions(context.Background())
	b := NewBroker(
		broker.WithContext(trackedCtx),
		WithExchange("test-exchange"),
		WithExchangeType("direct"),
		WithPrefetchCount(10),
		WithDurable(true),
		WithAutoDelete(true),
	)
	err := b.Init()
	assert.NoError(t, err)

	optionsCtx := b.Options().Context
	assert.Equal(t, "test-exchange", broker.GetTrackedValue(optionsCtx, exchangeKey{}))
	assert.Equal(t, "direct", broker.GetTrackedValue(optionsCtx, exchangeTypeKey{}))
	assert.Equal(t, 10, broker.GetTrackedValue(optionsCtx, prefetchCountKey{}))
	assert.Equal(t, true, broker.GetTrackedValue(optionsCtx, durableKey{}))
	assert.Equal(t, true, broker.GetTrackedValue(optionsCtx, autoDeleteKey{}))
}

func TestRabbitMQ_Subscriber_Info(t *testing.T) {
	sub := &rmqSubscriber{
		topic: "test",
		opts:  broker.SubscribeOptions{Queue: "test-q"},
	}
	assert.Equal(t, "test", sub.Topic())
	assert.Equal(t, "test-q", sub.Options().Queue)

	sub.cancel = func() {}
	err := sub.Unsubscribe()
	assert.NoError(t, err)
}

func TestRabbitMQ_Event_Info(t *testing.T) {
	acked := false
	nacked := false
	event := &rmqEvent{
		topic:   "test-topic",
		message: &broker.Message{Body: []byte("test-body")},
		delivery: amqp.Delivery{
			Acknowledger: &mockAcknowledger{
				ackFunc: func(tag uint64, multiple bool) error {
					acked = true
					return nil
				},
				nackFunc: func(tag uint64, multiple, requeue bool) error {
					nacked = true
					return nil
				},
			},
		},
	}
	assert.Equal(t, "test-topic", event.Topic())
	assert.Equal(t, []byte("test-body"), event.Message().Body)
	assert.NoError(t, event.Error())

	err := event.Ack()
	assert.NoError(t, err)
	assert.True(t, acked)

	err = event.Nack(true)
	assert.NoError(t, err)
	assert.True(t, nacked)
}

func TestRabbitMQ_RealNewBroker(t *testing.T) {
	// This hits NewBroker but won't be able to connect
	b := NewBroker(broker.Addrs("amqp://invalid"))
	assert.NotNil(t, b)
}

func TestRabbitMQ_Connect_ClientID(t *testing.T) {
	b := NewBroker(broker.Addrs("amqp://localhost"), broker.WithClientID("test-client"))
	r := b.(*rmqBroker)

	var capturedConfig amqp.Config
	r.newConn = func(addr string, config amqp.Config) (rabbitConn, error) {
		capturedConfig = config
		return &mockConn{}, nil
	}

	err := b.Connect()
	assert.NoError(t, err)
	assert.Equal(t, "test-client", capturedConfig.Properties["connection_name"])
}

func TestRabbitMQ_Detailed_Subscribe_Options(t *testing.T) {
	b := NewBroker()
	r := b.(*rmqBroker)
	r.reconnectInterval = time.Millisecond * 10
	mockC := &mockConn{}
	mockCh := &mockChannel{}
	r.conn = mockC
	r.running = true
	r.ctx, r.cancel = context.WithCancel(context.Background())

	mockC.channelFunc = func() (rabbitChannel, error) {
		return mockCh, nil
	}

	t.Run("DeadLetterQueue", func(t *testing.T) {
		called := make(chan struct{})
		var capturedArgs amqp.Table
		mockCh.queueDeclareFunc = func(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error) {
			capturedArgs = args
			close(called)
			return amqp.Queue{Name: "test-queue"}, nil
		}
		mockCh.consumeFunc = func(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error) {
			return make(chan amqp.Delivery), nil
		}

		_, err := b.Subscribe("test", func(ctx context.Context, event broker.Event) error { return nil },
			broker.WithQueue("test-queue"),
			broker.WithDeadLetterQueue("dlq"))
		assert.NoError(t, err)

		select {
		case <-called:
		case <-time.After(time.Second * 2):
			t.Fatal("QueueDeclare not called")
		}
		assert.Equal(t, "dlq", capturedArgs["x-dead-letter-routing-key"])
	})
}

type mockAcknowledger struct {
	ackFunc    func(tag uint64, multiple bool) error
	nackFunc   func(tag uint64, multiple, requeue bool) error
	rejectFunc func(tag uint64, requeue bool) error
}

func (m *mockAcknowledger) Ack(tag uint64, multiple bool) error {
	return m.ackFunc(tag, multiple)
}
func (m *mockAcknowledger) Nack(tag uint64, multiple, requeue bool) error {
	return m.nackFunc(tag, multiple, requeue)
}
func (m *mockAcknowledger) Reject(tag uint64, requeue bool) error {
	return m.rejectFunc(tag, requeue)
}

func TestRabbitMQ_Detailed_Subscribe(t *testing.T) {
	b := NewBroker()
	r := b.(*rmqBroker)
	r.reconnectInterval = time.Millisecond * 10
	mockC := &mockConn{}
	mockCh := &mockChannel{}
	r.conn = mockC
	r.running = true
	r.ctx, r.cancel = context.WithCancel(context.Background())

	mockC.channelFunc = func() (rabbitChannel, error) {
		return mockCh, nil
	}

	t.Run("QueueDeclareCalled", func(t *testing.T) {
		called := make(chan struct{})
		mockCh.queueDeclareFunc = func(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error) {
			select {
			case <-called:
			default:
				close(called)
			}
			return amqp.Queue{Name: "test-queue"}, nil
		}
		mockCh.consumeFunc = func(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error) {
			return make(chan amqp.Delivery), nil
		}

		_, err := b.Subscribe("test", func(ctx context.Context, event broker.Event) error { return nil }, broker.WithQueue("test-queue"))
		assert.NoError(t, err)

		select {
		case <-called:
		case <-time.After(time.Second * 5):
			t.Fatal("QueueDeclare not called")
		}
	})

	t.Run("ExchangeDeclareCalled", func(t *testing.T) {
		called := make(chan struct{})
		mockCh.exchangeDeclareFunc = func(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error {
			select {
			case <-called:
			default:
				close(called)
			}
			return nil
		}

		b.Init(WithExchange("ex"))
		_, err := b.Subscribe("test2", func(ctx context.Context, event broker.Event) error { return nil })
		assert.NoError(t, err)

		select {
		case <-called:
		case <-time.After(time.Second * 5):
			t.Fatal("ExchangeDeclare not called")
		}
	})
}

func TestRabbitMQ_Reconnection(t *testing.T) {
	b := NewBroker(broker.Addrs("amqp://localhost"))
	r := b.(*rmqBroker)
	r.reconnectInterval = time.Millisecond * 10

	mockC := &mockConn{}
	mockCh := &mockChannel{}

	dialCount := 0
	r.newConn = func(addr string, config amqp.Config) (rabbitConn, error) {
		dialCount++
		return mockC, nil
	}
	mockC.channelFunc = func() (rabbitChannel, error) {
		return mockCh, nil
	}

	err := b.Connect()
	assert.NoError(t, err)

	// Simulate connection loss
	mockC.isClosedFunc = func() bool { return true }

	// Wait for reconnection
	time.Sleep(time.Millisecond * 50)

	r.RLock()
	assert.True(t, dialCount > 1)
	r.RUnlock()
}

func TestRabbitMQ_Errors(t *testing.T) {
	b := NewBroker(broker.Addrs("amqp://localhost"))
	r := b.(*rmqBroker)

	t.Run("ConnectError", func(t *testing.T) {
		r.newConn = func(addr string, config amqp.Config) (rabbitConn, error) {
			return nil, fmt.Errorf("dial failed")
		}
		err := b.Connect()
		assert.Error(t, err)
	})

	t.Run("ChannelError", func(t *testing.T) {
		mockC := &mockConn{}
		r.newConn = func(addr string, config amqp.Config) (rabbitConn, error) {
			return mockC, nil
		}
		mockC.channelFunc = func() (rabbitChannel, error) {
			return nil, fmt.Errorf("channel failed")
		}
		err := b.Connect()
		assert.Error(t, err)
	})

	t.Run("PublishNotConnected", func(t *testing.T) {
		r.channel = nil
		err := b.Publish(context.Background(), "t", &broker.Message{})
		assert.Error(t, err)
	})
}

func TestRabbitMQ_Detailed_Subscribe_Errors(t *testing.T) {
	mockC := &mockConn{}

	b := &rmqBroker{
		opts:              broker.Options{Addrs: []string{"amqp://localhost"}},
		newConn:           func(addr string, config amqp.Config) (rabbitConn, error) { return mockC, nil },
		reconnectInterval: 10 * time.Millisecond,
		conn:              mockC,
	}

	// Sub-test 1 & 2
	mockCh1 := &mockChannel{
		deliveries: make(chan amqp.Delivery, 10),
	}
	mockC.channelFunc = func() (rabbitChannel, error) { return mockCh1, nil }

	// 1. QueueDeclare Error
	mockCh1.queueDeclareFunc = func(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error) {
		return amqp.Queue{}, fmt.Errorf("queue fail")
	}

	sub, err := b.Subscribe("test", func(ctx context.Context, event broker.Event) error {
		return nil
	}, broker.WithQueue("fail-queue"))
	assert.NoError(t, err)

	time.Sleep(50 * time.Millisecond) // Wait for runSubscriber to fail and loop
	mockCh1.queueDeclareFunc = nil

	// 2. Consume Error
	mockCh1.consumeFunc = func(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error) {
		return nil, fmt.Errorf("consume fail")
	}
	time.Sleep(50 * time.Millisecond)
	mockCh1.consumeFunc = nil

	sub.Unsubscribe()
	time.Sleep(50 * time.Millisecond)

	// Sub-test 3
	mockCh2 := &mockChannel{
		deliveries: make(chan amqp.Delivery, 10),
	}
	consumeCalled := make(chan struct{})
	mockCh2.consumeFunc = func(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error) {
		close(consumeCalled)
		return mockCh2.deliveries, nil
	}
	mockC.channelFunc = func() (rabbitChannel, error) { return mockCh2, nil }

	// 3. Handler Error
	done := make(chan bool)
	sub2, _ := b.Subscribe("test-handler-err", func(ctx context.Context, event broker.Event) error {
		done <- true
		return fmt.Errorf("handler error")
	})

	<-consumeCalled

	mockCh2.deliveries <- amqp.Delivery{
		RoutingKey: "test-handler-err",
		Body:       []byte("hello"),
		Acknowledger: &mockAcknowledger{
			nackFunc: func(tag uint64, multiple, requeue bool) error {
				assert.True(t, requeue)
				return nil
			},
		},
	}

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("handler not called")
	}

	sub2.Unsubscribe()
}

func TestRabbitMQ_Options_Various(t *testing.T) {
	b := NewBroker().(*rmqBroker)

	// Test Address helper
	assert.Equal(t, "", b.Address())
	b.Init(broker.Addrs("localhost:5672", "other:5672"))
	assert.Equal(t, "localhost:5672", b.Address())

	// Test Options coverage
	b.Init(
		WithExchange("my-ex"),
		WithExchangeType("topic"),
		WithPrefetchCount(10),
		WithDurable(true),
		WithAutoDelete(true),
	)

	assert.NotNil(t, b)
}

func TestRabbitMQ_StringMapToTable(t *testing.T) {
	m := map[string]string{
		"key1": "val1",
		"key2": "val2",
	}
	table := stringMapToTable(m)
	assert.Equal(t, "val1", table["key1"])
	assert.Equal(t, "val2", table["key2"])

	assert.Nil(t, stringMapToTable(nil))
}
