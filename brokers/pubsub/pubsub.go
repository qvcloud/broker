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
	*broker.Runtime
	opts        broker.Options
	provider    pubsubProvider
	newProvider func(context.Context, string) (pubsubProvider, error)

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

func (p *pubsubBroker) Connect() (err error) {
	defer func() {
		if err != nil {
			p.Transition(broker.StateDegraded, false, false, err)
		}
	}()
	if err := p.ValidationError(); err != nil {
		return err
	}
	p.Start()
	p.Transition(broker.StateConnecting, false, false, nil)
	p.Lock()
	defer p.Unlock()

	if p.running {
		return nil
	}

	projectID := p.Address()
	if projectID == "" {
		return fmt.Errorf("project ID must be provided in Addrs")
	}

	ctx := p.opts.Context
	if ctx == nil {
		ctx = context.Background()
	}
	client, err := p.newProvider(ctx, projectID)
	if err != nil {
		return err
	}

	p.provider = client
	p.ctx, p.cancel = context.WithCancel(context.Background())
	p.running = true
	p.Transition(broker.StateReady, true, true, nil)

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

	var closeErr error
	if p.provider != nil {
		closeErr = p.provider.Close()
	}
	p.provider = nil
	p.running = false
	p.Transition(broker.StateStopped, false, false, nil)
	p.Close()
	return closeErr
}

func (p *pubsubBroker) Publish(ctx context.Context, topic string, msg *broker.Message, opts ...broker.PublishOption) (err error) {
	if p.Runtime != nil && p.OperationsEnabled() {
		started := p.StartOperation(broker.OperationPublish)
		defer func() { p.Observe(broker.OperationPublish, started, err) }()
	}
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
	for k, v := range p.InjectContext(ctx, msg.Header) {
		attributes[k] = v
	}

	res := provider.Publish(ctx, topic, &pubsub.Message{
		Data:       msg.Body,
		Attributes: attributes,
	})

	_, err = res.Get(ctx)
	if err == nil {
		broker.WarnUnconsumed(options.Context, p.opts.Logger)
	}
	return err
}

func (p *pubsubBroker) Subscribe(topic string, handler broker.Handler, opts ...broker.SubscribeOption) (broker.Subscriber, error) {
	handler = p.WrapHandler(handler)
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
		if err != nil {
			p.Observe(broker.OperationReceive, time.Now(), err)
			if p.opts.Logger != nil {
				p.opts.Logger.Logf("Pub/Sub receive error: %v", err)
			}
		}
	}()

	return p.WrapSubscriber(&pubsubSubscriber{
		topic:  topic,
		opts:   options,
		cancel: cancel,
	}), nil
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
		Runtime: broker.NewRuntimeForOptions("pubsub", *options),
		opts:    *options,
		newProvider: func(ctx context.Context, projectID string) (pubsubProvider, error) {
			client, err := pubsub.NewClient(ctx, projectID)
			if err != nil {
				return nil, err
			}
			return &realPubSubProvider{client: client}, nil
		},
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
