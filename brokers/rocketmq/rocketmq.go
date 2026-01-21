package rocketmq

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/apache/rocketmq-client-go/v2"

	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/apache/rocketmq-client-go/v2/producer"
	"github.com/apache/rocketmq-client-go/v2/rlog"
	"github.com/qvcloud/broker"
)

type rmqBroker struct {
	opts broker.Options

	producer rocketmq.Producer
	consumer rocketmq.PushConsumer

	sync.RWMutex
	running bool
	ctx     context.Context
	cancel  context.CancelFunc
}

func (r *rmqBroker) Options() broker.Options { return r.opts }

func (r *rmqBroker) Address() string {
	if len(r.opts.Addrs) > 0 {
		return r.opts.Addrs[0]
	}
	return ""
}

func (r *rmqBroker) Init(opts ...broker.Option) error {
	for _, o := range opts {
		o(&r.opts)
	}
	return nil
}

func (r *rmqBroker) Connect() error {
	r.Lock()
	defer r.Unlock()

	if r.running {
		return nil
	}

	if len(r.opts.Addrs) == 0 {
		return fmt.Errorf("rocketmq: namesrv addrs are required")
	}

	if r.producer == nil {
		opts := []producer.Option{
			producer.WithNameServer(r.opts.Addrs),
		}

		// Extract custom options from Context
		retry := 2
		if r.opts.Context != nil {
			if v, ok := broker.GetTrackedValue(r.opts.Context, retryKey{}).(int); ok {
				retry = v
			}
			if v, ok := broker.GetTrackedValue(r.opts.Context, groupNameKey{}).(string); ok {
				opts = append(opts, producer.WithGroupName(v))
			}
			if v, ok := broker.GetTrackedValue(r.opts.Context, namespaceKey{}).(string); ok {
				opts = append(opts, producer.WithNamespace(v))
			}
			if v, ok := broker.GetTrackedValue(r.opts.Context, instanceNameKey{}).(string); ok {
				opts = append(opts, producer.WithInstanceName(v))
			} else if r.opts.ClientID != "" {
				opts = append(opts, producer.WithInstanceName(r.opts.ClientID))
			}
			if v, ok := broker.GetTrackedValue(r.opts.Context, tracingKey{}).(bool); ok && v {
				opts = append(opts, producer.WithTrace(&primitive.TraceConfig{
					NamesrvAddrs: r.opts.Addrs,
				}))
			}
			// Acknowledge options used elsewhere
			broker.GetTrackedValue(r.opts.Context, concurrencyKey{})
		} else if r.opts.ClientID != "" {
			opts = append(opts, producer.WithInstanceName(r.opts.ClientID))
		}
		opts = append(opts, producer.WithRetry(retry))

		p, err := rocketmq.NewProducer(opts...)
		if err != nil {
			if r.opts.Logger != nil {
				r.opts.Logger.Logf("RocketMQ producer creation error: %v", err)
			}
			return err
		}
		if err := p.Start(); err != nil {
			if r.opts.Logger != nil {
				r.opts.Logger.Logf("RocketMQ producer start error: %v", err)
			}
			return err
		}
		r.producer = p
	}

	r.ctx, r.cancel = context.WithCancel(context.Background())
	r.running = true

	broker.WarnUnconsumed(r.opts.Context, r.opts.Logger)
	return nil
}

func (r *rmqBroker) Disconnect() error {
	r.Lock()
	defer r.Unlock()

	if !r.running {
		return nil
	}

	if r.cancel != nil {
		r.cancel()
	}

	if r.producer != nil {
		r.producer.Shutdown()
		r.producer = nil
	}

	if r.consumer != nil {
		r.consumer.Shutdown()
		r.consumer = nil
	}

	r.running = false
	return nil
}

func (r *rmqBroker) Publish(ctx context.Context, topic string, msg *broker.Message, opts ...broker.PublishOption) error {
	options := broker.PublishOptions{
		Context: ctx,
	}
	for _, o := range opts {
		o(&options)
	}

	rmqMsg := primitive.NewMessage(topic, msg.Body)
	for k, v := range msg.Header {
		switch k {
		case "KEYS":
			rmqMsg.WithKeys([]string{v})
		case "SHARDING_KEY":
			if options.ShardingKey == "" {
				options.ShardingKey = v
			}
		default:
			rmqMsg.WithProperty(k, v)
		}
	}

	if options.Delay > 0 {
		// RocketMQ delay levels: 1s 5s 10s 30s 1m 2m 3m 4m 5m 6m 7m 8m 9m 10m 20m 30m 1h 2h
		// We'll map to the closest level or just provide a helper.
		// For now, we'll use a simple mapping for common durations.
		level := 0
		d := options.Delay
		switch {
		case d <= 1*time.Second:
			level = 1
		case d <= 5*time.Second:
			level = 2
		case d <= 10*time.Second:
			level = 3
		case d <= 30*time.Second:
			level = 4
		case d <= 1*time.Minute:
			level = 5
		case d <= 2*time.Minute:
			level = 6
		case d <= 3*time.Minute:
			level = 7
		case d <= 4*time.Minute:
			level = 8
		case d <= 5*time.Minute:
			level = 9
		case d <= 6*time.Minute:
			level = 10
		case d <= 7*time.Minute:
			level = 11
		case d <= 8*time.Minute:
			level = 12
		case d <= 9*time.Minute:
			level = 13
		case d <= 10*time.Minute:
			level = 14
		case d <= 20*time.Minute:
			level = 15
		case d <= 30*time.Minute:
			level = 16
		case d <= 1*time.Hour:
			level = 17
		case d <= 2*time.Hour:
			level = 18
		}
		if level > 0 {
			rmqMsg.WithDelayTimeLevel(level)
		}
	}

	// Precedence: rocketmq.WithShardingKey > broker.WithShardingKey
	if options.Context != nil {
		if v, ok := broker.GetTrackedValue(options.Context, shardingKey{}).(string); ok {
			options.ShardingKey = v
		}
	}
	if options.ShardingKey != "" {
		rmqMsg.WithShardingKey(options.ShardingKey)
	}

	// Handle tags
	if len(options.Tags) > 0 {
		rmqMsg.WithTag(options.Tags[0])
	}
	if options.Context != nil {
		if v, ok := broker.GetTrackedValue(options.Context, tagKey{}).(string); ok {
			rmqMsg.WithTag(v)
		}
	}

	// Handle Async
	async := false
	if options.Context != nil {
		if v, ok := broker.GetTrackedValue(options.Context, asyncKey{}).(bool); ok {
			async = v
		}
	}

	if async {
		return r.producer.SendAsync(ctx, func(ctx context.Context, res *primitive.SendResult, err error) {
			if err != nil && r.opts.Logger != nil {
				r.opts.Logger.Logf("RocketMQ async send error: %v", err)
			}
		}, rmqMsg)
	}

	res, err := r.producer.SendSync(ctx, rmqMsg)
	if err != nil {
		return err
	}

	if res.Status != primitive.SendOK {
		return fmt.Errorf("send failed: %s", res.String())
	}

	broker.WarnUnconsumed(options.Context, r.opts.Logger)
	return nil
}

func (r *rmqBroker) Subscribe(topic string, handler broker.Handler, opts ...broker.SubscribeOption) (broker.Subscriber, error) {
	options := broker.NewSubscribeOptions(opts...)

	r.Lock()
	defer r.Unlock()

	if len(r.opts.Addrs) == 0 {
		return nil, fmt.Errorf("rocketmq: namesrv addrs are required")
	}

	groupID := options.Queue
	if groupID == "" {
		if v, ok := broker.GetTrackedValue(r.opts.Context, groupNameKey{}).(string); ok {
			groupID = v
		}
	}
	if groupID == "" {
		groupID = "GID_DEFAULT"
	}

	brokerCtx := r.ctx
	if r.consumer == nil {
		subOpts := []consumer.Option{
			consumer.WithNameServer(r.opts.Addrs),
			consumer.WithGroupName(groupID),
		}

		if r.opts.Context != nil {
			if v, ok := broker.GetTrackedValue(r.opts.Context, concurrencyKey{}).(int); ok {
				subOpts = append(subOpts, consumer.WithConsumeGoroutineNums(v))
			}
			if v, ok := broker.GetTrackedValue(r.opts.Context, namespaceKey{}).(string); ok {
				subOpts = append(subOpts, consumer.WithNamespace(v))
			}
			if v, ok := broker.GetTrackedValue(r.opts.Context, instanceNameKey{}).(string); ok {
				subOpts = append(subOpts, consumer.WithInstance(v))
			} else if r.opts.ClientID != "" {
				subOpts = append(subOpts, consumer.WithInstance(r.opts.ClientID))
			}
			if v, ok := broker.GetTrackedValue(r.opts.Context, tracingKey{}).(bool); ok && v {
				subOpts = append(subOpts, consumer.WithTrace(&primitive.TraceConfig{
					NamesrvAddrs: r.opts.Addrs,
				}))
			}
		} else if r.opts.ClientID != "" {
			subOpts = append(subOpts, consumer.WithInstance(r.opts.ClientID))
		}

		c, err := rocketmq.NewPushConsumer(subOpts...)
		if err != nil {
			if r.opts.Logger != nil {
				r.opts.Logger.Logf("RocketMQ consumer creation error: %v", err)
			}
			return nil, err
		}
		r.consumer = c
		if err := r.consumer.Start(); err != nil {
			if r.opts.Logger != nil {
				r.opts.Logger.Logf("RocketMQ consumer start error: %v", err)
			}
			return nil, err
		}
	}

	if brokerCtx == nil {
		brokerCtx = context.Background()
	}

	ctx, cancel := context.WithCancel(brokerCtx)

	err := r.consumer.Subscribe(topic, consumer.MessageSelector{}, func(consumeCtx context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
		for _, m := range msgs {
			msg := &broker.Message{
				Header: m.GetProperties(),
				Body:   m.Body,
			}

			event := &rmqEvent{
				topic:   topic,
				message: msg,
				res:     consumer.ConsumeSuccess,
			}

			// If AutoAck is off, default to retry unless user explicitly acks
			if !options.AutoAck {
				event.res = consumer.ConsumeRetryLater
			}

			if err := handler(ctx, event); err != nil {
				return consumer.ConsumeRetryLater, err
			}

			if !options.AutoAck {
				return event.res, nil
			}
		}
		return consumer.ConsumeSuccess, nil
	})

	if err != nil {
		cancel()
		return nil, err
	}

	return &rmqSubscriber{
		topic:  topic,
		opts:   options,
		broker: r,
		cancel: cancel,
	}, nil
}

func (r *rmqBroker) String() string {
	return "rocketmq"
}

type rmqSubscriber struct {
	topic  string
	opts   broker.SubscribeOptions
	broker *rmqBroker
	cancel context.CancelFunc
}

func (s *rmqSubscriber) Options() broker.SubscribeOptions {
	return s.opts
}

func (s *rmqSubscriber) Topic() string {
	return s.topic
}

func (s *rmqSubscriber) Unsubscribe() error {
	if s.cancel != nil {
		s.cancel()
	}
	s.broker.Lock()
	defer s.broker.Unlock()
	if s.broker.consumer != nil {
		return s.broker.consumer.Unsubscribe(s.topic)
	}
	return nil
}

type rmqEvent struct {
	topic   string
	message *broker.Message
	res     consumer.ConsumeResult
	mu      sync.Mutex
}

func (e *rmqEvent) Topic() string {
	return e.topic
}

func (e *rmqEvent) Message() *broker.Message {
	return e.message
}

func (e *rmqEvent) Ack() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.res = consumer.ConsumeSuccess
	return nil
}

func (e *rmqEvent) Nack(requeue bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if requeue {
		e.res = consumer.ConsumeRetryLater
	} else {
		// RocketMQ doesn't have a direct "drop" without acking in common push consumer,
		// so we just succeed to move offset forward.
		e.res = consumer.ConsumeSuccess
	}
	return nil
}

func (e *rmqEvent) Error() error {
	return nil
}

func NewBroker(opts ...broker.Option) broker.Broker {
	options := broker.NewOptions(opts...)

	return &rmqBroker{
		opts: *options,
	}
}

type groupNameKey struct{}
type retryKey struct{}
type concurrencyKey struct{}
type tracingKey struct{}
type namespaceKey struct{}
type instanceNameKey struct{}
type shardingKey struct{}

func WithGroupName(name string) broker.Option {
	return func(o *broker.Options) {
		o.Context = broker.WithTrackedValue(o.Context, groupNameKey{}, name, "rocketmq.WithGroupName")
	}
}

func WithInstanceName(name string) broker.Option {
	return func(o *broker.Options) {
		o.Context = broker.WithTrackedValue(o.Context, instanceNameKey{}, name, "rocketmq.WithInstanceName")
	}
}

func WithRetry(times int) broker.Option {
	return func(o *broker.Options) {
		o.Context = broker.WithTrackedValue(o.Context, retryKey{}, times, "rocketmq.WithRetry")
	}
}

func WithConsumeGoroutineNums(nums int) broker.Option {
	return func(o *broker.Options) {
		o.Context = broker.WithTrackedValue(o.Context, concurrencyKey{}, nums, "rocketmq.WithConsumeGoroutineNums")
	}
}

func WithTracingEnabled(enabled bool) broker.Option {
	return func(o *broker.Options) {
		o.Context = broker.WithTrackedValue(o.Context, tracingKey{}, enabled, "rocketmq.WithTracingEnabled")
	}
}

func WithNamespace(ns string) broker.Option {
	return func(o *broker.Options) {
		o.Context = broker.WithTrackedValue(o.Context, namespaceKey{}, ns, "rocketmq.WithNamespace")
	}
}

func WithLogLevel(level string) broker.Option {
	return func(o *broker.Options) {
		rlog.SetLogLevel(level)
	}
}

type asyncKey struct{}
type tagKey struct{}

func WithAsync() broker.PublishOption {
	return func(o *broker.PublishOptions) {
		o.Context = broker.WithTrackedValue(o.Context, asyncKey{}, true, "rocketmq.WithAsync")
	}
}

func WithTag(tag string) broker.PublishOption {
	return func(o *broker.PublishOptions) {
		o.Context = broker.WithTrackedValue(o.Context, tagKey{}, tag, "rocketmq.WithTag")
	}
}

func WithShardingKey(key string) broker.PublishOption {
	return func(o *broker.PublishOptions) {
		o.Context = broker.WithTrackedValue(o.Context, shardingKey{}, key, "rocketmq.WithShardingKey")
	}
}
