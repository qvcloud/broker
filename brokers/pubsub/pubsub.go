package pubsub

import (
	"context"
	"fmt"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/qvcloud/broker"
)

type pubsubProvider interface {
	Publish(ctx context.Context, topic string, msg *pubsub.Message) *pubsub.PublishResult
	Receive(ctx context.Context, sub string, opts broker.Options, f func(context.Context, *pubsub.Message)) error
	Close() error
}

type realPubSubProvider struct {
	client *pubsub.Client
}

func (r *realPubSubProvider) Publish(ctx context.Context, topic string, msg *pubsub.Message) *pubsub.PublishResult {
	return r.client.Topic(topic).Publish(ctx, msg)
}

func (r *realPubSubProvider) Receive(ctx context.Context, sub string, opts broker.Options, f func(context.Context, *pubsub.Message)) error {
	s := r.client.Subscription(sub)
	if opts.Context != nil {
		if v, ok := broker.GetTrackedValue(opts.Context, maxOutstandingMessagesKey{}).(int); ok {
			s.ReceiveSettings.MaxOutstandingMessages = v
		}
		if v, ok := broker.GetTrackedValue(opts.Context, maxOutstandingBytesKey{}).(int); ok {
			s.ReceiveSettings.MaxOutstandingBytes = v
		}
		if v, ok := broker.GetTrackedValue(opts.Context, maxExtensionKey{}).(time.Duration); ok {
			s.ReceiveSettings.MaxExtension = v
		}
	}
	return s.Receive(ctx, f)
}

func (r *realPubSubProvider) Close() error {
	return r.client.Close()
}

type pubsubBroker struct {
	opts     broker.Options
	provider pubsubProvider

	sync.RWMutex
	running bool
	ctx     context.Context
	cancel  context.CancelFunc
}

func (p *pubsubBroker) Options() broker.Options { return p.opts }

func (p *pubsubBroker) Address() string {
	if len(p.opts.Addrs) > 0 {
		return p.opts.Addrs[0]
	}
	return ""
}

func (p *pubsubBroker) Init(opts ...broker.Option) error {
	for _, o := range opts {
		o(&p.opts)
	}
	return nil
}

func (p *pubsubBroker) Connect() error {
	p.Lock()
	defer p.Unlock()

	if p.running {
		return nil
	}

	projectID := p.Address()
	if projectID == "" {
		return fmt.Errorf("project ID must be provided in Addrs")
	}

	client, err := pubsub.NewClient(context.Background(), projectID)
	if err != nil {
		return err
	}

	p.provider = &realPubSubProvider{client: client}
	p.ctx, p.cancel = context.WithCancel(context.Background())
	p.running = true

	// Warn about unconsumed options
	broker.WarnUnconsumed(p.opts.Context, p.opts.Logger)

	return nil
}

func (p *pubsubBroker) Disconnect() error {
	p.Lock()
	defer p.Unlock()

	if !p.running {
		return nil
	}

	if p.cancel != nil {
		p.cancel()
	}

	if p.provider != nil {
		p.provider.Close()
	}

	p.running = false
	return nil
}

func (p *pubsubBroker) Publish(ctx context.Context, topic string, msg *broker.Message, opts ...broker.PublishOption) error {
	options := broker.PublishOptions{
		Context: ctx,
	}
	for _, o := range opts {
		o(&options)
	}

	p.RLock()
	provider := p.provider
	p.RUnlock()

	if provider == nil {
		return fmt.Errorf("not connected")
	}

	attributes := make(map[string]string)
	for k, v := range msg.Header {
		attributes[k] = v
	}

	res := provider.Publish(ctx, topic, &pubsub.Message{
		Data:       msg.Body,
		Attributes: attributes,
	})

	_, err := res.Get(ctx)
	if err == nil {
		broker.WarnUnconsumed(options.Context, p.opts.Logger)
	}
	return err
}

func (p *pubsubBroker) Subscribe(topic string, handler broker.Handler, opts ...broker.SubscribeOption) (broker.Subscriber, error) {
	options := broker.NewSubscribeOptions(opts...)

	p.RLock()
	provider := p.provider
	brokerCtx := p.ctx
	p.RUnlock()

	if provider == nil {
		return nil, fmt.Errorf("not connected")
	}

	if brokerCtx == nil {
		brokerCtx = context.Background()
	}

	if options.Queue == "" {
		return nil, fmt.Errorf("subscription ID must be provided in Queue option")
	}

	ctx, cancel := context.WithCancel(brokerCtx)

	go func() {
		err := provider.Receive(ctx, options.Queue, p.opts, func(ctx context.Context, pm *pubsub.Message) {
			header := make(map[string]string)
			for k, v := range pm.Attributes {
				header[k] = v
			}

			msg := &broker.Message{
				Header: header,
				Body:   pm.Data,
			}

			event := &pubsubEvent{
				topic:   topic,
				message: msg,
				pm:      pm,
			}

			if err := handler(ctx, event); err == nil && options.AutoAck {
				event.Ack()
			}
		})
		if err != nil && p.opts.Logger != nil {
			p.opts.Logger.Logf("Pub/Sub receive error: %v", err)
		}
	}()

	return &pubsubSubscriber{
		topic:  topic,
		opts:   options,
		cancel: cancel,
	}, nil
}

func (p *pubsubBroker) String() string {
	return "pubsub"
}

type pubsubSubscriber struct {
	topic  string
	opts   broker.SubscribeOptions
	cancel context.CancelFunc
}

func (s *pubsubSubscriber) Options() broker.SubscribeOptions { return s.opts }
func (s *pubsubSubscriber) Topic() string                    { return s.topic }
func (s *pubsubSubscriber) Unsubscribe() error {
	s.cancel()
	return nil
}

type pubsubEvent struct {
	topic   string
	message *broker.Message
	pm      *pubsub.Message
}

func (e *pubsubEvent) Topic() string            { return e.topic }
func (e *pubsubEvent) Message() *broker.Message { return e.message }
func (e *pubsubEvent) Ack() error {
	e.pm.Ack()
	return nil
}
func (e *pubsubEvent) Nack(requeue bool) error {
	if !requeue {
		e.pm.Ack() // Drop
		return nil
	}
	e.pm.Nack() // Requeue
	return nil
}
func (e *pubsubEvent) Error() error { return nil }

func NewBroker(opts ...broker.Option) broker.Broker {
	options := broker.NewOptions(opts...)
	return &pubsubBroker{
		opts: *options,
	}
}

type maxOutstandingMessagesKey struct{}
type maxOutstandingBytesKey struct{}
type maxExtensionKey struct{}

func WithMaxOutstandingMessages(n int) broker.Option {
	return func(o *broker.Options) {
		if o.Context == nil {
			o.Context = context.Background()
		}
		o.Context = broker.WithTrackedValue(o.Context, maxOutstandingMessagesKey{}, n, "pubsub.WithMaxOutstandingMessages")
	}
}

func WithMaxOutstandingBytes(n int) broker.Option {
	return func(o *broker.Options) {
		if o.Context == nil {
			o.Context = context.Background()
		}
		o.Context = broker.WithTrackedValue(o.Context, maxOutstandingBytesKey{}, n, "pubsub.WithMaxOutstandingBytes")
	}
}

func WithMaxExtension(d time.Duration) broker.Option {
	return func(o *broker.Options) {
		if o.Context == nil {
			o.Context = context.Background()
		}
		o.Context = broker.WithTrackedValue(o.Context, maxExtensionKey{}, d, "pubsub.WithMaxExtension")
	}
}
