package broker

import (
	"context"
	"sync"

	"github.com/google/uuid"
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

func (e *noopEvent) Error() error {
	return e.err
}

type noopBroker struct {
	opts *Options
	sync.RWMutex
	Subscribers map[string][]*noopSubscriber
}

func (b *noopBroker) Options() Options {
	return *b.opts
}

func (b *noopBroker) Address() string {
	return ""
}

func (b *noopBroker) Connect() error {
	return nil
}

func (b *noopBroker) Disconnect() error {
	return nil
}

func (b *noopBroker) Init(opts ...Option) error {
	for _, opt := range opts {
		opt(b.opts)
	}
	return nil
}

func (b *noopBroker) String() string {
	return "noop"
}

func (b *noopBroker) Publish(ctx context.Context, topic string, msg *Message, opts ...PublishOption) error {
	b.RLock()
	subs, ok := b.Subscribers[topic]
	b.RUnlock()
	if !ok || len(subs) == 0 {
		return nil
	}

	var v any
	if b.opts.Codec != nil {
		buf, err := b.opts.Codec.Marshal(msg)
		if err != nil {
			return err
		}
		v = buf
	} else {
		v = msg
	}

	var wg sync.WaitGroup
	for _, sub := range subs {
		wg.Add(1)
		go func(sub *noopSubscriber, wg *sync.WaitGroup) {
			defer wg.Done()
			p := &noopEvent{
				topic:   topic,
				message: v,
				opts:    b.opts,
			}
			if err := sub.handler(ctx, p); err != nil {
				p.err = err
				if eh := b.opts.ErrorHandler; eh != nil {
					eh(ctx, p)
				}
			}
		}(sub, &wg)
	}
	wg.Wait()
	return nil
}

func (b *noopBroker) Subscribe(topic string, handler Handler, opts ...SubscribeOption) (Subscriber, error) {
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
	}()

	return sub, nil
}

func NewNoopBroker(opts ...Option) Broker {
	options := NewOptions(opts...)

	return &noopBroker{
		opts:        options,
		Subscribers: make(map[string][]*noopSubscriber),
	}
}
