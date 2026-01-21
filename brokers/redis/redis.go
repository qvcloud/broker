package redis

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/qvcloud/broker"
	"github.com/redis/go-redis/v9"
)

type redisClient interface {
	Ping(ctx context.Context) *redis.StatusCmd
	Close() error
	XAdd(ctx context.Context, a *redis.XAddArgs) *redis.StringCmd
	XGroupCreateMkStream(ctx context.Context, stream, group, start string) *redis.StatusCmd
	XReadGroup(ctx context.Context, a *redis.XReadGroupArgs) *redis.XStreamSliceCmd
	XAck(ctx context.Context, stream, group string, ids ...string) *redis.IntCmd
}

type redisBroker struct {
	opts   broker.Options
	client redisClient

	sync.RWMutex
	running bool
	ctx     context.Context
	cancel  context.CancelFunc

	newClient func(opts *redis.Options) redisClient
}

func (r *redisBroker) Options() broker.Options { return r.opts }

func (r *redisBroker) Address() string {
	if len(r.opts.Addrs) > 0 {
		return r.opts.Addrs[0]
	}
	return ""
}

func (r *redisBroker) Init(opts ...broker.Option) error {
	for _, o := range opts {
		o(&r.opts)
	}
	return nil
}

func (r *redisBroker) Connect() error {
	r.Lock()
	defer r.Unlock()

	if r.running {
		return nil
	}

	addr := r.Address()
	if addr == "" {
		return fmt.Errorf("redis: address is required")
	}

	redisOpts := &redis.Options{
		Addr: addr,
	}

	if r.opts.TLSConfig != nil {
		redisOpts.TLSConfig = r.opts.TLSConfig
	}

	if r.opts.Context != nil {
		if v, ok := broker.GetTrackedValue(r.opts.Context, passwordKey{}).(string); ok {
			redisOpts.Password = v
		}
		if v, ok := broker.GetTrackedValue(r.opts.Context, dbKey{}).(int); ok {
			redisOpts.DB = v
		}
	}

	r.client = r.newClient(redisOpts)

	// Check connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := r.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis: connect error: %v", err)
	}

	r.ctx, r.cancel = context.WithCancel(context.Background())
	r.running = true

	broker.WarnUnconsumed(r.opts.Context, r.opts.Logger)
	return nil
}

func (r *redisBroker) Disconnect() error {
	r.Lock()
	defer r.Unlock()

	if !r.running {
		return nil
	}

	if r.cancel != nil {
		r.cancel()
	}

	if r.client != nil {
		r.client.Close()
	}

	r.running = false
	return nil
}

func (r *redisBroker) Publish(ctx context.Context, topic string, msg *broker.Message, opts ...broker.PublishOption) error {
	options := broker.PublishOptions{Context: ctx}
	for _, o := range opts {
		o(&options)
	}

	r.RLock()
	client := r.client
	r.RUnlock()

	if client == nil {
		return fmt.Errorf("not connected")
	}

	values := make(map[string]interface{})
	values["body"] = msg.Body
	for k, v := range msg.Header {
		values["h:"+k] = v
	}

	maxlen := int64(0)
	if options.Context != nil {
		if v, ok := broker.GetTrackedValue(options.Context, maxlenKey{}).(int64); ok {
			maxlen = v
		}
	}

	arg := &redis.XAddArgs{
		Stream: topic,
		Values: values,
		MaxLen: maxlen,
		Approx: true,
	}

	err := client.XAdd(ctx, arg).Err()
	if err == nil {
		broker.WarnUnconsumed(options.Context, r.opts.Logger)
	}
	return err
}

func (r *redisBroker) Subscribe(topic string, handler broker.Handler, opts ...broker.SubscribeOption) (broker.Subscriber, error) {
	options := broker.NewSubscribeOptions(opts...)

	group := options.Queue
	if group == "" {
		group = "broker_group"
	}

	consumerName := r.opts.ClientID
	if consumerName == "" {
		consumerName = fmt.Sprintf("consumer-%d", time.Now().UnixNano())
	}

	r.Lock()
	client := r.client
	brokerCtx := r.ctx
	r.Unlock()

	if client == nil {
		return nil, fmt.Errorf("not connected")
	}

	// Ensure group exists
	err := client.XGroupCreateMkStream(brokerCtx, topic, group, "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return nil, err
	}

	ctx, cancel := context.WithCancel(brokerCtx)

	go func() {
		defer cancel()

		// Read pending messages first
		r.processStream(ctx, client, topic, group, consumerName, "0", handler, options)

		// Then loop for new messages
		for {
			select {
			case <-ctx.Done():
				return
			default:
				if !r.processStream(ctx, client, topic, group, consumerName, ">", handler, options) {
					// If processStream fails or has no messages, we shouldn't spin too fast if it's a persistent error
					time.Sleep(time.Millisecond * 100)
				}
			}
		}
	}()

	return &redisSubscriber{
		topic:  topic,
		cancel: cancel,
	}, nil
}

func (r *redisBroker) processStream(ctx context.Context, client redisClient, topic, group, consumer, id string, handler broker.Handler, subOpts broker.SubscribeOptions) bool {
	streams, err := client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{topic, id},
		Count:    10,
		Block:    time.Second * 5,
	}).Result()

	if err != nil {
		if err != redis.Nil && ctx.Err() == nil {
			if r.opts.Logger != nil {
				r.opts.Logger.Logf("redis: read error: %v", err)
			}
			time.Sleep(time.Second)
		}
		return false
	}

	if len(streams) == 0 || len(streams[0].Messages) == 0 {
		return false
	}

	for _, xmsg := range streams[0].Messages {
		msg := &broker.Message{
			Header: make(map[string]string),
		}
		if body, ok := xmsg.Values["body"].([]byte); ok {
			msg.Body = body
		} else if body, ok := xmsg.Values["body"].(string); ok {
			msg.Body = []byte(body)
		}

		for k, v := range xmsg.Values {
			if strings.HasPrefix(k, "h:") {
				msg.Header[k[2:]] = fmt.Sprint(v)
			}
		}

		event := &redisEvent{
			topic:  streams[0].Stream,
			msg:    msg,
			raw:    xmsg,
			group:  group,
			client: client,
		}

		err := handler(ctx, event)
		if err == nil && subOpts.AutoAck {
			event.Ack()
		}
	}

	return true
}

func (r *redisBroker) String() string { return "redis" }

type redisSubscriber struct {
	topic  string
	cancel context.CancelFunc
}

func (s *redisSubscriber) Options() broker.SubscribeOptions { return broker.SubscribeOptions{} }
func (s *redisSubscriber) Topic() string                    { return s.topic }
func (s *redisSubscriber) Unsubscribe() error {
	if s.cancel != nil {
		s.cancel()
	}
	return nil
}

type redisEvent struct {
	topic  string
	msg    *broker.Message
	raw    redis.XMessage
	group  string
	client redisClient
}

func (e *redisEvent) Topic() string            { return e.topic }
func (e *redisEvent) Message() *broker.Message { return e.msg }
func (e *redisEvent) Ack() error {
	return e.client.XAck(context.Background(), e.topic, e.group, e.raw.ID).Err()
}
func (e *redisEvent) Nack(requeue bool) error {
	if !requeue {
		return e.Ack()
	}
	return nil
}
func (e *redisEvent) Error() error { return nil }

type passwordKey struct{}
type dbKey struct{}
type maxlenKey struct{}

func WithPassword(p string) broker.Option {
	return func(o *broker.Options) {
		o.Context = broker.WithTrackedValue(o.Context, passwordKey{}, p, "redis.WithPassword")
	}
}

func WithDB(db int) broker.Option {
	return func(o *broker.Options) {
		o.Context = broker.WithTrackedValue(o.Context, dbKey{}, db, "redis.WithDB")
	}
}

func WithMaxLen(l int64) broker.PublishOption {
	return func(o *broker.PublishOptions) {
		o.Context = broker.WithTrackedValue(o.Context, maxlenKey{}, l, "redis.WithMaxLen")
	}
}

func NewBroker(opts ...broker.Option) broker.Broker {
	return &redisBroker{
		opts: *broker.NewOptions(opts...),
		newClient: func(opts *redis.Options) redisClient {
			return redis.NewClient(opts)
		},
	}
}
