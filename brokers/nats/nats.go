package nats

import (
	"context"
	"fmt"
	"sync"

	"github.com/nats-io/nats.go"
	"github.com/qvcloud/broker"
)

type natsBroker struct {
	opts broker.Options
	conn *nats.Conn

	sync.RWMutex
	running bool
	ctx     context.Context
	cancel  context.CancelFunc
}

func (n *natsBroker) Options() broker.Options { return n.opts }

func (n *natsBroker) Address() string {
	if len(n.opts.Addrs) > 0 {
		return n.opts.Addrs[0]
	}
	return nats.DefaultURL
}

func (n *natsBroker) Init(opts ...broker.Option) error {
	for _, o := range opts {
		o(&n.opts)
	}
	return nil
}

func (n *natsBroker) Connect() error {
	n.Lock()
	defer n.Unlock()

	if n.running {
		return nil
	}

	addr := n.Address()
	var (
		conn *nats.Conn
		err  error
	)

	opts := []nats.Option{}
	if n.opts.TLSConfig != nil {
		opts = append(opts, nats.Secure(n.opts.TLSConfig))
	}
	if n.opts.ClientID != "" {
		opts = append(opts, nats.Name(n.opts.ClientID))
	}

	conn, err = nats.Connect(addr, opts...)
	if err != nil {
		if n.opts.Logger != nil {
			n.opts.Logger.Logf("NATS connect error to %s: %v", addr, err)
		}
		return err
	}
	n.conn = conn

	n.ctx, n.cancel = context.WithCancel(context.Background())
	n.running = true
	return nil
}

func (n *natsBroker) Disconnect() error {
	n.Lock()
	defer n.Unlock()

	if !n.running {
		return nil
	}

	if n.cancel != nil {
		n.cancel()
	}

	if n.conn != nil {
		n.conn.Close()
	}

	n.running = false
	return nil
}

func (n *natsBroker) Publish(ctx context.Context, topic string, msg *broker.Message, opts ...broker.PublishOption) error {
	n.RLock()
	conn := n.conn
	n.RUnlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	nm := &nats.Msg{
		Subject: topic,
		Header:  make(nats.Header),
		Data:    msg.Body,
	}

	for k, v := range msg.Header {
		nm.Header.Set(k, v)
	}

	return conn.PublishMsg(nm)
}

func (n *natsBroker) Subscribe(topic string, handler broker.Handler, opts ...broker.SubscribeOption) (broker.Subscriber, error) {
	options := broker.NewSubscribeOptions(opts...)

	n.Lock()
	conn := n.conn
	brokerCtx := n.ctx
	n.Unlock()

	if conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	if brokerCtx == nil {
		brokerCtx = context.Background()
	}

	ctx, cancel := context.WithCancel(brokerCtx)

	var sub *nats.Subscription
	var err error

	h := func(nm *nats.Msg) {
		header := make(map[string]string)
		for k, v := range nm.Header {
			if len(v) > 0 {
				header[k] = v[0]
			}
		}

		msg := &broker.Message{
			Header: header,
			Body:   nm.Data,
		}

		event := &natsEvent{
			topic:   nm.Subject,
			message: msg,
			nm:      nm,
		}

		if err := handler(ctx, event); err == nil && options.AutoAck {
			event.Ack()
		}
	}

	if options.Queue != "" {
		sub, err = conn.QueueSubscribe(topic, options.Queue, h)
	} else {
		sub, err = conn.Subscribe(topic, h)
	}

	if err != nil {
		return nil, err
	}

	return &natsSubscriber{
		topic:  topic,
		opts:   options,
		sub:    sub,
		cancel: cancel,
	}, nil
}

func (n *natsBroker) String() string {
	return "nats"
}

type natsSubscriber struct {
	topic  string
	opts   broker.SubscribeOptions
	sub    *nats.Subscription
	cancel context.CancelFunc
}

func (s *natsSubscriber) Options() broker.SubscribeOptions { return s.opts }
func (s *natsSubscriber) Topic() string                    { return s.topic }
func (s *natsSubscriber) Unsubscribe() error {
	if s.cancel != nil {
		s.cancel()
	}
	if s.sub != nil {
		return s.sub.Unsubscribe()
	}
	return nil
}

type natsEvent struct {
	topic   string
	message *broker.Message
	nm      *nats.Msg
}

func (e *natsEvent) Topic() string            { return e.topic }
func (e *natsEvent) Message() *broker.Message { return e.message }
func (e *natsEvent) Ack() error               { return e.nm.Ack() }
func (e *natsEvent) Nack(requeue bool) error  { return e.nm.Nak() }
func (e *natsEvent) Error() error             { return nil }

func NewBroker(opts ...broker.Option) broker.Broker {
	options := broker.NewOptions(opts...)
	return &natsBroker{
		opts: *options,
	}
}
