package nats

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/qvcloud/broker"
)

type natsConn interface {
	PublishMsg(m *nats.Msg) error
	Subscribe(subj string, cb nats.MsgHandler) (*nats.Subscription, error)
	QueueSubscribe(subj, queue string, cb nats.MsgHandler) (*nats.Subscription, error)
	Close()
}

type natsBroker struct {
	opts broker.Options
	conn natsConn

	sync.RWMutex
	running bool
	ctx     context.Context
	cancel  context.CancelFunc

	newConn func(addr string, opts ...nats.Option) (natsConn, error)
}

func (n *natsBroker) Options() broker.Options { return n.opts }

func (n *natsBroker) Address() string {
	if len(n.opts.Addrs) > 0 {
		return n.opts.Addrs[0]
	}
	return ""
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

	if len(n.opts.Addrs) == 0 {
		return fmt.Errorf("nats: server addresses are required")
	}

	addr := n.Address()
	var (
		conn natsConn
		err  error
	)

	opts := []nats.Option{}
	if n.opts.TLSConfig != nil {
		opts = append(opts, nats.Secure(n.opts.TLSConfig))
	}
	if n.opts.ClientID != "" {
		opts = append(opts, nats.Name(n.opts.ClientID))
	}

	if n.opts.Context != nil {
		if v, ok := broker.GetTrackedValue(n.opts.Context, maxReconnectKey{}).(int); ok {
			opts = append(opts, nats.MaxReconnects(v))
		}
		if v, ok := broker.GetTrackedValue(n.opts.Context, reconnectWaitKey{}).(time.Duration); ok {
			opts = append(opts, nats.ReconnectWait(v))
		}
	}

	conn, err = n.newConn(addr, opts...)
	if err != nil {
		if n.opts.Logger != nil {
			n.opts.Logger.Logf("NATS connect error to %s: %v", addr, err)
		}
		return err
	}
	n.conn = conn

	n.ctx, n.cancel = context.WithCancel(context.Background())
	n.running = true

	// Warn about unconsumed options
	broker.WarnUnconsumed(n.opts.Context, n.opts.Logger)

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
	options := broker.PublishOptions{
		Context: ctx,
	}
	for _, o := range opts {
		o(&options)
	}

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

	if options.Context != nil {
		if v, ok := broker.GetTrackedValue(options.Context, replyToKey{}).(string); ok {
			nm.Reply = v
		}
	}

	for k, v := range msg.Header {
		nm.Header.Set(k, v)
	}

	err := conn.PublishMsg(nm)
	if err == nil {
		broker.WarnUnconsumed(options.Context, n.opts.Logger)
	}
	return err
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
		cancel()
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
func (e *natsEvent) Nack(requeue bool) error {
	if !requeue {
		return e.nm.Term()
	}
	return e.nm.Nak()
}
func (e *natsEvent) Error() error { return nil }

func NewBroker(opts ...broker.Option) broker.Broker {
	options := broker.NewOptions(opts...)
	return &natsBroker{
		opts: *options,
		newConn: func(addr string, opts ...nats.Option) (natsConn, error) {
			return nats.Connect(addr, opts...)
		},
	}
}

type maxReconnectKey struct{}
type reconnectWaitKey struct{}

func WithMaxReconnect(max int) broker.Option {
	return func(o *broker.Options) {
		if o.Context == nil {
			o.Context = context.Background()
		}
		o.Context = broker.WithTrackedValue(o.Context, maxReconnectKey{}, max, "nats.WithMaxReconnect")
	}
}

func WithReconnectWait(wait time.Duration) broker.Option {
	return func(o *broker.Options) {
		if o.Context == nil {
			o.Context = context.Background()
		}
		o.Context = broker.WithTrackedValue(o.Context, reconnectWaitKey{}, wait, "nats.WithReconnectWait")
	}
}

type replyToKey struct{}

func WithReplyTo(reply string) broker.PublishOption {
	return func(o *broker.PublishOptions) {
		if o.Context == nil {
			o.Context = context.Background()
		}
		o.Context = broker.WithTrackedValue(o.Context, replyToKey{}, reply, "nats.WithReplyTo")
	}
}
