package rabbitmq

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/qvcloud/broker"
	amqp "github.com/rabbitmq/amqp091-go"
)

type rabbitConn interface {
	Channel() (rabbitChannel, error)
	Close() error
	IsClosed() bool
}

type rabbitChannel interface {
	PublishWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error
	Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error)
	QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error)
	ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error
	QueueBind(name, key, exchange string, noWait bool, args amqp.Table) error
	Close() error
	Qos(prefetchCount, prefetchSize int, global bool) error
}

type connWrapper struct{ *amqp.Connection }

func (w *connWrapper) Channel() (rabbitChannel, error) {
	return w.Connection.Channel()
}

type rmqBroker struct {
	opts broker.Options

	conn    rabbitConn
	channel rabbitChannel

	sync.RWMutex
	running bool
	ctx     context.Context
	cancel  context.CancelFunc

	// Internal factories for testing
	newConn func(addr string, config amqp.Config) (rabbitConn, error)

	reconnectInterval time.Duration
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
		return fmt.Errorf("rabbitmq: server addresses are required")
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

	conn, err := r.newConn(addr, config)
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

	// Warn about unconsumed options at connection time
	broker.WarnUnconsumed(r.opts.Context, r.opts.Logger)

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
				newConn, err := r.newConn(r.Address(), amqp.Config{})
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
			case <-time.After(r.reconnectInterval):
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
		if v, ok := broker.GetTrackedValue(r.opts.Context, exchangeKey{}).(string); ok {
			exchange = v
		}
	}

	priority := uint8(0)
	deliveryMode := amqp.Transient
	mandatory := false

	if options.Context != nil {
		if v, ok := broker.GetTrackedValue(options.Context, priorityKey{}).(int); ok {
			priority = uint8(v)
		}
		if v, ok := broker.GetTrackedValue(options.Context, persistentKey{}).(bool); ok && v {
			deliveryMode = amqp.Persistent
		}
		if v, ok := broker.GetTrackedValue(options.Context, mandatoryKey{}).(bool); ok {
			mandatory = v
		}
	}

	err := ch.PublishWithContext(ctx,
		exchange,  // exchange
		topic,     // routing key
		mandatory, // mandatory
		false,     // immediate
		amqp.Publishing{
			Headers:      amqp.Table(headers),
			ContentType:  "application/octet-stream",
			Body:         msg.Body,
			Priority:     priority,
			DeliveryMode: deliveryMode,
		})

	if err == nil {
		broker.WarnUnconsumed(options.Context, r.opts.Logger)
	}

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
				time.Sleep(r.reconnectInterval)
				continue
			}

			ch, err := conn.Channel()
			if err != nil {
				time.Sleep(r.reconnectInterval)
				continue
			}

			// Extract options
			durable := true
			autoDelete := false
			prefetchCount := 0
			exchange := ""
			exchangeType := "direct"

			if r.opts.Context != nil {
				if v, ok := broker.GetTrackedValue(r.opts.Context, durableKey{}).(bool); ok {
					durable = v
				}
				if v, ok := broker.GetTrackedValue(r.opts.Context, autoDeleteKey{}).(bool); ok {
					autoDelete = v
				}
				if v, ok := broker.GetTrackedValue(r.opts.Context, prefetchCountKey{}).(int); ok {
					prefetchCount = v
				}
				if v, ok := broker.GetTrackedValue(r.opts.Context, exchangeKey{}).(string); ok {
					exchange = v
				}
				if v, ok := broker.GetTrackedValue(r.opts.Context, exchangeTypeKey{}).(string); ok {
					exchangeType = v
				}
			}

			if prefetchCount > 0 {
				if err := ch.Qos(prefetchCount, 0, false); err != nil {
					ch.Close()
					time.Sleep(r.reconnectInterval)
					continue
				}
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
				time.Sleep(r.reconnectInterval)
				continue
			}

			if exchange != "" {
				if err := ch.ExchangeDeclare(exchange, exchangeType, true, false, false, false, nil); err != nil {
					ch.Close()
					time.Sleep(r.reconnectInterval)
					continue
				}
			}

			if topic != "" && (topic != options.Queue || exchange != "") {
				if err := ch.QueueBind(q.Name, topic, exchange, false, nil); err != nil {
					ch.Close()
					time.Sleep(r.reconnectInterval)
					continue
				}
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
				time.Sleep(r.reconnectInterval)
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
		newConn: func(addr string, config amqp.Config) (rabbitConn, error) {
			conn, err := amqp.DialConfig(addr, config)
			if err != nil {
				return nil, err
			}
			return &connWrapper{conn}, nil
		}, reconnectInterval: 5 * time.Second}
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
		o.Context = broker.WithTrackedValue(o.Context, exchangeKey{}, name, "rabbitmq.WithExchange")
	}
}

func WithExchangeType(kind string) broker.Option {
	return func(o *broker.Options) {
		if o.Context == nil {
			o.Context = context.Background()
		}
		o.Context = broker.WithTrackedValue(o.Context, exchangeTypeKey{}, kind, "rabbitmq.WithExchangeType")
	}
}

func WithPrefetchCount(count int) broker.Option {
	return func(o *broker.Options) {
		if o.Context == nil {
			o.Context = context.Background()
		}
		o.Context = broker.WithTrackedValue(o.Context, prefetchCountKey{}, count, "rabbitmq.WithPrefetchCount")
	}
}

func WithDurable(durable bool) broker.Option {
	return func(o *broker.Options) {
		if o.Context == nil {
			o.Context = context.Background()
		}
		o.Context = broker.WithTrackedValue(o.Context, durableKey{}, durable, "rabbitmq.WithDurable")
	}
}

func WithAutoDelete(autoDelete bool) broker.Option {
	return func(o *broker.Options) {
		if o.Context == nil {
			o.Context = context.Background()
		}
		o.Context = broker.WithTrackedValue(o.Context, autoDeleteKey{}, autoDelete, "rabbitmq.WithAutoDelete")
	}
}

type priorityKey struct{}
type persistentKey struct{}
type mandatoryKey struct{}

func WithPriority(p int) broker.PublishOption {
	return func(o *broker.PublishOptions) {
		if o.Context == nil {
			o.Context = context.Background()
		}
		o.Context = broker.WithTrackedValue(o.Context, priorityKey{}, p, "rabbitmq.WithPriority")
	}
}

func WithPersistent(p bool) broker.PublishOption {
	return func(o *broker.PublishOptions) {
		if o.Context == nil {
			o.Context = context.Background()
		}
		o.Context = broker.WithTrackedValue(o.Context, persistentKey{}, p, "rabbitmq.WithPersistent")
	}
}

func WithMandatory() broker.PublishOption {
	return func(o *broker.PublishOptions) {
		if o.Context == nil {
			o.Context = context.Background()
		}
		o.Context = broker.WithTrackedValue(o.Context, mandatoryKey{}, true, "rabbitmq.WithMandatory")
	}
}

func stringMapToTable(m map[string]string) map[string]interface{} {
	if m == nil {
		return nil
	}
	res := make(map[string]interface{})
	for k, v := range m {
		res[k] = v
	}
	return res
}
