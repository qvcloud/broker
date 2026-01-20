package rocketmq

import (
	"context"
	"fmt"
	"sync"

	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/apache/rocketmq-client-go/v2/producer"
	"github.com/qvcloud/broker"
)

type rmqBroker struct {
	opts broker.Options

	producer rocketmq.Producer
	consumer rocketmq.PushConsumer

	sync.RWMutex
	running bool
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

	if r.producer == nil {
		p, err := rocketmq.NewProducer(
			producer.WithNameServer(r.opts.Addrs),
			producer.WithRetry(2),
		)
		if err != nil {
			return err
		}
		if err := p.Start(); err != nil {
			return err
		}
		r.producer = p
	}

	r.running = true
	return nil
}

func (r *rmqBroker) Disconnect() error {
	r.Lock()
	defer r.Unlock()

	if !r.running {
		return nil
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
		rmqMsg.WithProperty(k, v)
	}

	if options.ShardingKey != "" {
		rmqMsg.WithShardingKey(options.ShardingKey)
	}

	res, err := r.producer.SendSync(ctx, rmqMsg)
	if err != nil {
		return err
	}

	if res.Status != primitive.SendOK {
		return fmt.Errorf("send failed: %s", res.String())
	}

	return nil
}

func (r *rmqBroker) Subscribe(topic string, handler broker.Handler, opts ...broker.SubscribeOption) (broker.Subscriber, error) {
	options := broker.NewSubscribeOptions(opts...)

	groupID := options.Queue
	if groupID == "" {
		groupID = "GID_DEFAULT"
	}

	r.Lock()
	if r.consumer == nil {
		c, err := rocketmq.NewPushConsumer(
			consumer.WithNameServer(r.opts.Addrs),
			consumer.WithGroupName(groupID),
		)
		if err != nil {
			r.Unlock()
			return nil, err
		}
		r.consumer = c
		if err := r.consumer.Start(); err != nil {
			r.Unlock()
			return nil, err
		}
	}
	r.Unlock()

	err := r.consumer.Subscribe(topic, consumer.MessageSelector{}, func(ctx context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
		for _, m := range msgs {
			msg := &broker.Message{
				Header: m.GetProperties(),
				Body:   m.Body,
			}

			if err := handler(ctx, &rmqEvent{topic: topic, message: msg}); err != nil {
				return consumer.ConsumeRetryLater, err
			}
		}
		return consumer.ConsumeSuccess, nil
	})

	if err != nil {
		return nil, err
	}

	return &rmqSubscriber{
		topic: topic,
		opts:  options,
	}, nil
}

func (r *rmqBroker) String() string {
	return "rocketmq"
}

type rmqSubscriber struct {
	topic string
	opts  broker.SubscribeOptions
}

func (s *rmqSubscriber) Options() broker.SubscribeOptions {
	return s.opts
}

func (s *rmqSubscriber) Topic() string {
	return s.topic
}

func (s *rmqSubscriber) Unsubscribe() error {
	return nil
}

type rmqEvent struct {
	topic   string
	message *broker.Message
}

func (e *rmqEvent) Topic() string {
	return e.topic
}

func (e *rmqEvent) Message() *broker.Message {
	return e.message
}

func (e *rmqEvent) Ack() error {
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
