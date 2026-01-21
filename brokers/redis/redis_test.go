package redis

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/qvcloud/broker"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

type mockRedisClient struct {
	pingFunc                 func(ctx context.Context) *redis.StatusCmd
	closeFunc                func() error
	xAddFunc                 func(ctx context.Context, a *redis.XAddArgs) *redis.StringCmd
	xGroupCreateMkStreamFunc func(ctx context.Context, stream, group, start string) *redis.StatusCmd
	xReadGroupFunc           func(ctx context.Context, a *redis.XReadGroupArgs) *redis.XStreamSliceCmd
	xAckFunc                 func(ctx context.Context, stream, group string, ids ...string) *redis.IntCmd
}

func (m *mockRedisClient) Ping(ctx context.Context) *redis.StatusCmd {
	if m.pingFunc != nil {
		return m.pingFunc(ctx)
	}
	cmd := redis.NewStatusCmd(ctx)
	return cmd
}

func (m *mockRedisClient) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func (m *mockRedisClient) XAdd(ctx context.Context, a *redis.XAddArgs) *redis.StringCmd {
	if m.xAddFunc != nil {
		return m.xAddFunc(ctx, a)
	}
	return redis.NewStringCmd(ctx)
}

func (m *mockRedisClient) XGroupCreateMkStream(ctx context.Context, stream, group, start string) *redis.StatusCmd {
	if m.xGroupCreateMkStreamFunc != nil {
		return m.xGroupCreateMkStreamFunc(ctx, stream, group, start)
	}
	return redis.NewStatusCmd(ctx)
}

func (m *mockRedisClient) XReadGroup(ctx context.Context, a *redis.XReadGroupArgs) *redis.XStreamSliceCmd {
	if m.xReadGroupFunc != nil {
		return m.xReadGroupFunc(ctx, a)
	}
	return redis.NewXStreamSliceCmd(ctx)
}

func (m *mockRedisClient) XAck(ctx context.Context, stream, group string, ids ...string) *redis.IntCmd {
	if m.xAckFunc != nil {
		return m.xAckFunc(ctx, stream, group, ids...)
	}
	return redis.NewIntCmd(ctx)
}

func TestRedis_Basic(t *testing.T) {
	b := NewBroker(broker.Addrs("localhost:6379"))
	assert.Equal(t, "redis", b.String())
	assert.Equal(t, "localhost:6379", b.Address())
}

func TestRedis_Connect_Disconnect(t *testing.T) {
	b := &redisBroker{
		opts: broker.Options{Addrs: []string{"localhost:6379"}},
		newClient: func(opts *redis.Options) redisClient {
			return &mockRedisClient{}
		},
	}

	err := b.Connect()
	assert.NoError(t, err)
	assert.True(t, b.running)

	err = b.Disconnect()
	assert.NoError(t, err)
	assert.False(t, b.running)
}

func TestRedis_Connect_Errors(t *testing.T) {
	t.Run("NoAddr", func(t *testing.T) {
		b := &redisBroker{}
		err := b.Connect()
		assert.Error(t, err)
	})

	t.Run("PingError", func(t *testing.T) {
		b := &redisBroker{
			opts: broker.Options{Addrs: []string{"localhost:6379"}},
			newClient: func(opts *redis.Options) redisClient {
				m := &mockRedisClient{}
				m.pingFunc = func(ctx context.Context) *redis.StatusCmd {
					cmd := redis.NewStatusCmd(ctx)
					cmd.SetErr(fmt.Errorf("ping failed"))
					return cmd
				}
				return m
			},
		}
		err := b.Connect()
		assert.Error(t, err)
	})
}

func TestRedis_Publish(t *testing.T) {
	mockC := &mockRedisClient{}
	b := &redisBroker{
		client:  mockC,
		running: true,
	}

	t.Run("Success", func(t *testing.T) {
		var capturedStream string
		mockC.xAddFunc = func(ctx context.Context, a *redis.XAddArgs) *redis.StringCmd {
			capturedStream = a.Stream
			return redis.NewStringCmd(ctx)
		}

		err := b.Publish(context.Background(), "test-topic", &broker.Message{
			Header: map[string]string{"foo": "bar"},
			Body:   []byte("hello"),
		})
		assert.NoError(t, err)
		assert.Equal(t, "test-topic", capturedStream)
	})

	t.Run("NotConnected", func(t *testing.T) {
		b2 := &redisBroker{}
		err := b2.Publish(context.Background(), "t", &broker.Message{})
		assert.Error(t, err)
	})
}

func TestRedis_Subscribe(t *testing.T) {
	mockC := &mockRedisClient{}
	b := &redisBroker{
		client:    mockC,
		running:   true,
		ctx:       context.Background(),
		newClient: func(opts *redis.Options) redisClient { return mockC },
	}

	mockC.xGroupCreateMkStreamFunc = func(ctx context.Context, stream, group, start string) *redis.StatusCmd {
		return redis.NewStatusCmd(ctx)
	}

	mockC.xReadGroupFunc = func(ctx context.Context, a *redis.XReadGroupArgs) *redis.XStreamSliceCmd {
		cmd := redis.NewXStreamSliceCmd(ctx)
		if len(a.Streams) > 1 && a.Streams[1] == "0" {
			cmd.SetVal([]redis.XStream{
				{
					Stream: "test-topic",
					Messages: []redis.XMessage{
						{ID: "1-0", Values: map[string]interface{}{"body": "pending", "h:foo": "bar"}},
					},
				},
			})
		} else {
			time.Sleep(20 * time.Millisecond)
			if ctx.Err() != nil {
				return cmd
			}
			cmd.SetVal([]redis.XStream{
				{
					Stream: "test-topic",
					Messages: []redis.XMessage{
						{ID: "2-0", Values: map[string]interface{}{"body": "new"}},
					},
				},
			})
		}
		return cmd
	}

	results := make(chan string, 2)
	sub, err := b.Subscribe("test-topic", func(ctx context.Context, event broker.Event) error {
		results <- string(event.Message().Body)
		return nil
	}, broker.WithQueue("test-group"), broker.WithAutoAck(true))

	assert.NoError(t, err)
	assert.NotNil(t, sub)

	count := 0
	timer := time.After(2 * time.Second)
	for count < 2 {
		select {
		case msg := <-results:
			count++
			if count == 1 {
				assert.Equal(t, "pending", msg)
			} else {
				assert.Equal(t, "new", msg)
			}
		case <-timer:
			t.Fatal("Timeout waiting for messages")
		}
	}

	sub.Unsubscribe()
}

func TestRedis_Options_Check(t *testing.T) {
	b := &redisBroker{
		newClient: func(opts *redis.Options) redisClient {
			assert.Equal(t, "secret", opts.Password)
			assert.Equal(t, 2, opts.DB)
			return &mockRedisClient{}
		},
		opts: broker.Options{
			Addrs: []string{"localhost:6379"},
			Context: broker.WithTrackedValue(
				broker.WithTrackedValue(context.Background(), passwordKey{}, "secret", "WithPassword"),
				dbKey{}, 2, "WithDB",
			),
		},
	}
	b.Connect()
}

func TestRedis_Extra_Coverage(t *testing.T) {
	mockC := &mockRedisClient{}
	b := &redisBroker{
		client:  mockC,
		running: true,
	}

	t.Run("InitAndOptions", func(t *testing.T) {
		b.Init(broker.Addrs("localhost:9999"))
		assert.Equal(t, "localhost:9999", b.Address())
		assert.NotNil(t, b.Options())
	})

	t.Run("EventMethods", func(t *testing.T) {
		mockC.xAckFunc = func(ctx context.Context, stream, group string, ids ...string) *redis.IntCmd {
			return redis.NewIntCmd(ctx)
		}
		event := &redisEvent{
			topic:  "test",
			msg:    &broker.Message{},
			raw:    redis.XMessage{ID: "1-0"},
			group:  "group",
			client: mockC,
		}
		assert.Equal(t, "test", event.Topic())
		assert.NotNil(t, event.Message())
		assert.NoError(t, event.Ack())
		assert.NoError(t, event.Nack(false))
		assert.NoError(t, event.Nack(true))
		assert.NoError(t, event.Error())
	})

	t.Run("SubscriberInfo", func(t *testing.T) {
		sub := &redisSubscriber{topic: "test"}
		assert.Equal(t, "test", sub.Topic())
		assert.NotNil(t, sub.Options())
	})

	t.Run("OptionHelpers", func(t *testing.T) {
		opts := broker.Options{}
		WithPassword("pass")(&opts)
		WithDB(1)(&opts)

		pubOpts := broker.PublishOptions{}
		WithMaxLen(100)(&pubOpts)
	})

	t.Run("ProcessStreamErrors", func(t *testing.T) {
		mockC.xReadGroupFunc = func(ctx context.Context, a *redis.XReadGroupArgs) *redis.XStreamSliceCmd {
			cmd := redis.NewXStreamSliceCmd(ctx)
			cmd.SetErr(fmt.Errorf("read error"))
			return cmd
		}
		b.processStream(context.Background(), mockC, "t", "g", "c", ">", func(ctx context.Context, event broker.Event) error { return nil }, broker.SubscribeOptions{})
	})
}
