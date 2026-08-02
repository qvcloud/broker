package broker

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type noopSubscriber struct {
	id      string
	topic   string
	exit    chan bool
	handler Handler
	opts    SubscribeOptions
}

func (s *noopSubscriber) Options() SubscribeOptions {
	return s.opts
}

func (s *noopSubscriber) Topic() string {
	return s.topic
}

func (s *noopSubscriber) Unsubscribe() error {
	s.exit <- true
	return nil
}

type noopEvent struct {
	opts    *Options
	topic   string
	err     error
	message any
}

func (e *noopEvent) Topic() string {
	return e.topic
}

func (e *noopEvent) Message() *Message {
	switch v := e.message.(type) {
	case *Message:
		return v
	case []byte:
		msg := &Message{}
		if e.opts.Codec != nil {
			if err := e.opts.Codec.Unmarshal(v, msg); err != nil {
				return nil
			}
		}
		return msg
	}
	return nil
}

func (e *noopEvent) Ack() error {
	return nil
}

func (e *noopEvent) Nack(requeue bool) error {
	return nil
}

func (e *noopEvent) Error() error {
	return e.err
}

type noopBroker struct {
	opts   *Options
	obs    *Runtime
	obsErr error
	sync.RWMutex
	Subscribers map[string][]*noopSubscriber
	connected   bool
}

func (b *noopBroker) Options() Options {
	return *b.opts
}

func (b *noopBroker) Address() string {
	return ""
}

func (b *noopBroker) Connect() error {
	if b.obsErr != nil {
		return b.obsErr
	}
	b.obs.Start()
	b.obs.Transition(StateConnecting, false, false, nil)
	b.Lock()
	defer b.Unlock()
	b.connected = true
	b.obs.Transition(StateReady, true, true, nil)
	WarnUnconsumed(b.opts.Context, b.opts.Logger)
	return nil
}

func (b *noopBroker) Disconnect() error {
	b.Lock()
	defer b.Unlock()
	b.connected = false
	b.obs.Transition(StateStopped, false, false, nil)
	b.obs.Close()
	return nil
}

func (b *noopBroker) Init(opts ...Option) error {
	for _, opt := range opts {
		opt(b.opts)
	}
	b.obs, b.obsErr = NewRuntime("noop", b.opts.Observability)
	return b.obsErr
}

func (b *noopBroker) Observability() Observability { return b.obs }

func (b *noopBroker) String() string {
	return "noop"
}

func (b *noopBroker) Publish(ctx context.Context, topic string, msg *Message, opts ...PublishOption) error {
	instrument := b.obs.OperationsEnabled()
	var started time.Time
	if instrument {
		started = b.obs.StartOperation(OperationPublish)
	}
	var resultErr error
	if instrument {
		defer func() { b.obs.Observe(OperationPublish, started, resultErr) }()
	}
	options := PublishOptions{
		Context: ctx,
	}
	for _, o := range opts {
		o(&options)
	}
	WarnUnconsumed(options.Context, b.opts.Logger)

	if b.opts.Tracer != nil {
		var span trace.Span
		ctx, span = b.opts.Tracer.Start(ctx, "broker.publish",
			trace.WithSpanKind(trace.SpanKindProducer),
			trace.WithAttributes(
				attribute.String("messaging.system", "noop"),
				attribute.String("messaging.destination", topic),
			),
		)
		defer span.End()
	}

	b.RLock()
	subs, ok := b.Subscribers[topic]
	b.RUnlock()
	if !ok || len(subs) == 0 {
		return nil
	}

	var v any
	message := msg
	if msg != nil && b.obs.Enabled(CategoryCorrelation) {
		copy := *msg
		copy.Header = b.obs.InjectContext(ctx, msg.Header)
		message = &copy
	}
	if b.opts.Codec != nil {
		buf, err := b.opts.Codec.Marshal(message)
		if err != nil {
			resultErr = err
			return err
		}
		v = buf
	} else {
		v = message
	}

	var wg sync.WaitGroup
	for _, sub := range subs {
		wg.Add(1)
		go func(sub *noopSubscriber, wg *sync.WaitGroup) {
			defer wg.Done()
			handlerInstrument := b.obs.OperationsEnabled()
			var handlerStarted time.Time
			if handlerInstrument {
				handlerStarted = b.obs.StartOperation(OperationHandler)
			}
			p := &noopEvent{
				topic:   topic,
				message: v,
				opts:    b.opts,
			}
			handlerCtx := ctx
			if message := p.Message(); message != nil {
				handlerCtx = b.obs.ExtractContext(ctx, message.Header)
			}
			if err := sub.handler(handlerCtx, p); err != nil {
				if handlerInstrument {
					b.obs.Observe(OperationHandler, handlerStarted, err)
				}
				p.err = err
				if eh := b.opts.ErrorHandler; eh != nil {
					_ = eh(ctx, p)
				}
			} else {
				if handlerInstrument {
					b.obs.Observe(OperationHandler, handlerStarted, nil)
				}
			}
		}(sub, &wg)
	}
	wg.Wait()
	return nil
}

func (b *noopBroker) Subscribe(topic string, handler Handler, opts ...SubscribeOption) (Subscriber, error) {
	started := b.obs.StartOperation(OperationSubscribe)
	defer b.obs.Observe(OperationSubscribe, started, nil)
	options := NewSubscribeOptions(opts...)

	sub := &noopSubscriber{
		exit:    make(chan bool, 1),
		id:      uuid.New().String(),
		topic:   topic,
		handler: handler,
		opts:    options,
	}

	b.Lock()
	if b.Subscribers == nil {
		b.Subscribers = make(map[string][]*noopSubscriber)
	}
	b.Subscribers[topic] = append(b.Subscribers[topic], sub)
	b.Unlock()
	b.obs.AddSubscription(1)

	go func() {
		<-sub.exit
		b.Lock()
		var newSubscribers []*noopSubscriber
		for _, sb := range b.Subscribers[topic] {
			if sb.id == sub.id {
				continue
			}
			newSubscribers = append(newSubscribers, sb)
		}
		b.Subscribers[topic] = newSubscribers
		b.Unlock()
		b.obs.AddSubscription(-1)
	}()

	return sub, nil
}

func NewNoopBroker(opts ...Option) Broker {
	options := NewOptions(opts...)
	observability, err := NewRuntime("noop", options.Observability)

	return &noopBroker{
		opts:        options,
		obs:         observability,
		obsErr:      err,
		Subscribers: make(map[string][]*noopSubscriber),
	}
}
