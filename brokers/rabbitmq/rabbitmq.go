package rabbitmq

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/qvcloud/broker"
	amqp "github.com/rabbitmq/amqp091-go"
)

type rmqBroker struct {
	opts broker.Options

	conn    *amqp.Connection
	channel *amqp.Channel

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
	return "amqp://guest:guest@localhost:5672/"
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

	addr := r.Address()
	config := amqp.Config{
		TLSClientConfig: r.opts.TLSConfig,
	}
	if r.opts.ClientID != "" {
		config.Properties = amqp.Table{
			"connection_name": r.opts.ClientID,
		}
	}

	conn, err := amqp.DialConfig(addr, config)
	if err != nil {
		return err
	}
	r.conn = conn

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return err
	}
	r.channel = ch

	r.ctx, r.cancel = context.WithCancel(context.Background())
	r.running = true

	// Handle reconnection
	go func() {
		for {
			r.RLock()
			ctx := r.ctx
			if !r.running {
				r.RUnlock()
				return
			}
			conn := r.conn
			r.RUnlock()

			if conn == nil || conn.IsClosed() {
				if r.opts.Logger != nil {
					r.opts.Logger.Log("RabbitMQ connection lost, reconnecting...")
				}
				newConn, err := amqp.Dial(r.Address())
				if err == nil {
					r.Lock()
					r.conn = newConn
					if ch, err := newConn.Channel(); err == nil {
						r.channel = ch
					}
					r.Unlock()
				} else {
					if r.opts.Logger != nil {
						r.opts.Logger.Logf("RabbitMQ reconnection failed: %v", err)
					}
				}
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
		}
	}()

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

	if r.channel != nil {
		r.channel.Close()
	}
	if r.conn != nil {
		r.conn.Close()
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

	r.RLock()
	ch := r.channel
	r.RUnlock()

	if ch == nil {
		return fmt.Errorf("not connected")
	}

	headers := stringMapToTable(msg.Header)
	if options.Delay > 0 {
		headers["x-delay"] = int64(options.Delay.Milliseconds())
	}

	exchange := ""
	if r.opts.Context != nil {
		if v, ok := r.opts.Context.Value(exchangeKey{}).(string); ok {
			exchange = v
		}
	}

	err := ch.PublishWithContext(ctx,
		exchange, // exchange
		topic,    // routing key
		false,    // mandatory
		false,    // immediate
		amqp.Publishing{
			Headers:     amqp.Table(headers),
			ContentType: "application/octet-stream",
			Body:        msg.Body,
		})

	return err
}

func (r *rmqBroker) Subscribe(topic string, handler broker.Handler, opts ...broker.SubscribeOption) (broker.Subscriber, error) {
	options := broker.NewSubscribeOptions(opts...)

	r.RLock()
	brokerCtx := r.ctx
	r.RUnlock()

	if brokerCtx == nil {
		brokerCtx = context.Background()
	}

	ctx, cancel := context.WithCancel(brokerCtx)

	go r.runSubscriber(ctx, topic, handler, options)

	return &rmqSubscriber{
		topic:  topic,
		opts:   options,
		cancel: cancel,
	}, nil
}

func (r *rmqBroker) runSubscriber(ctx context.Context, topic string, handler broker.Handler, options broker.SubscribeOptions) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			r.RLock()
			conn := r.conn
			r.RUnlock()

			if conn == nil || conn.IsClosed() {
				time.Sleep(time.Second)
				continue
			}

			ch, err := conn.Channel()
			if err != nil {
				time.Sleep(time.Second)
				continue
			}

			// Extract options
			durable := true
			autoDelete := false
			prefetchCount := 0
			exchange := ""
			exchangeType := "direct"

			if r.opts.Context != nil {
				if v, ok := r.opts.Context.Value(durableKey{}).(bool); ok {
					durable = v
				}
				if v, ok := r.opts.Context.Value(autoDeleteKey{}).(bool); ok {
					autoDelete = v
				}
				if v, ok := r.opts.Context.Value(prefetchCountKey{}).(int); ok {
					prefetchCount = v
				}
				if v, ok := r.opts.Context.Value(exchangeKey{}).(string); ok {
					exchange = v
				}
				if v, ok := r.opts.Context.Value(exchangeTypeKey{}).(string); ok {
					exchangeType = v
				}
			}

			if prefetchCount > 0 {
				ch.Qos(prefetchCount, 0, false)
			}

			args := amqp.Table{}
			if options.DeadLetterQueue != "" {
				args["x-dead-letter-exchange"] = ""
				args["x-dead-letter-routing-key"] = options.DeadLetterQueue
			}

			q, err := ch.QueueDeclare(
				options.Queue, // name
				durable,       // durable
				autoDelete,    // delete when unused
				false,         // exclusive
				false,         // no-wait
				args,          // arguments
			)
			if err != nil {
				ch.Close()
				time.Sleep(time.Second)
				continue
			}

			if exchange != "" {
				ch.ExchangeDeclare(exchange, exchangeType, true, false, false, false, nil)
			}

			if topic != "" && (topic != options.Queue || exchange != "") {
				ch.QueueBind(q.Name, topic, exchange, false, nil)
			}

			msgs, err := ch.Consume(
				q.Name, // queue
				"",     // consumer
				false,  // always manual ack for framework-level control
				false,  // exclusive
				false,  // no-local
				false,  // no-wait
				nil,    // args
			)
			if err != nil {
				ch.Close()
				time.Sleep(time.Second)
				continue
			}

			for d := range msgs {
				header := make(map[string]string)
				for k, v := range d.Headers {
					header[k] = fmt.Sprint(v)
				}

				msg := &broker.Message{
					Header: header,
					Body:   d.Body,
				}

				event := &rmqEvent{
					topic:    d.RoutingKey,
					message:  msg,
					delivery: d,
				}

				if err := handler(ctx, event); err != nil {
					if !options.AutoAck {
						d.Nack(false, true)
					}
				} else {
					if !options.AutoAck {
						d.Ack(false)
					}
				}
			}
			ch.Close()
		}
	}
}

func (r *rmqBroker) String() string {
	return "rabbitmq"
}

type rmqSubscriber struct {
	topic  string
	opts   broker.SubscribeOptions
	cancel context.CancelFunc
}

func (s *rmqSubscriber) Options() broker.SubscribeOptions { return s.opts }
func (s *rmqSubscriber) Topic() string                    { return s.topic }
func (s *rmqSubscriber) Unsubscribe() error {
	if s.cancel != nil {
		s.cancel()
	}
	return nil
}

type rmqEvent struct {
	topic    string
	message  *broker.Message
	delivery amqp.Delivery
}

func (e *rmqEvent) Topic() string            { return e.topic }
func (e *rmqEvent) Message() *broker.Message { return e.message }
func (e *rmqEvent) Ack() error               { return e.delivery.Ack(false) }
func (e *rmqEvent) Nack(requeue bool) error  { return e.delivery.Nack(false, requeue) }
func (e *rmqEvent) Error() error             { return nil }

func NewBroker(opts ...broker.Option) broker.Broker {
	options := broker.NewOptions(opts...)
	return &rmqBroker{
		opts: *options,
	}
}

type exchangeKey struct{}
type exchangeTypeKey struct{}
type prefetchCountKey struct{}
type durableKey struct{}
type autoDeleteKey struct{}

func WithExchange(name string) broker.Option {
	return func(o *broker.Options) {
		if o.Context == nil {
			o.Context = context.Background()
		}
		o.Context = context.WithValue(o.Context, exchangeKey{}, name)
	}
}

func WithExchangeType(kind string) broker.Option {
	return func(o *broker.Options) {
		if o.Context == nil {
			o.Context = context.Background()
		}
		o.Context = context.WithValue(o.Context, exchangeTypeKey{}, kind)
	}
}

func WithPrefetchCount(count int) broker.Option {
	return func(o *broker.Options) {
		if o.Context == nil {
			o.Context = context.Background()
		}
		o.Context = context.WithValue(o.Context, prefetchCountKey{}, count)
	}
}

func WithDurable(durable bool) broker.Option {
	return func(o *broker.Options) {
		if o.Context == nil {
			o.Context = context.Background()
		}
		o.Context = context.WithValue(o.Context, durableKey{}, durable)
	}
}

func WithAutoDelete(autoDelete bool) broker.Option {
	return func(o *broker.Options) {
		if o.Context == nil {
			o.Context = context.Background()
		}
		o.Context = context.WithValue(o.Context, autoDeleteKey{}, autoDelete)
	}
}

func stringMapToTable(m map[string]string) map[string]interface{} {
	res := make(map[string]interface{})
	for k, v := range m {
		res[k] = v
	}
	return res
}
