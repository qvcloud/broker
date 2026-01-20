package rabbitmq

import (
	"context"
	"fmt"
	"sync"

	"github.com/qvcloud/broker"
	amqp "github.com/rabbitmq/amqp091-go"
)

type rmqBroker struct {
	opts broker.Options

	conn    *amqp.Connection
	channel *amqp.Channel

	sync.RWMutex
	running bool
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
	conn, err := amqp.Dial(addr)
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

	r.running = true
	return nil
}

func (r *rmqBroker) Disconnect() error {
	r.Lock()
	defer r.Unlock()

	if !r.running {
		return nil
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

	err := ch.PublishWithContext(ctx,
		"",    // exchange
		topic, // routing key
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			Headers:     amqp.Table(headers),
			ContentType: "application/octet-stream",
			Body:        msg.Body,
		})

	return err
}

func (r *rmqBroker) Subscribe(topic string, handler broker.Handler, opts ...broker.SubscribeOption) (broker.Subscriber, error) {
	options := broker.NewSubscribeOptions(opts...)

	r.Lock()
	ch := r.channel
	r.Unlock()

	if ch == nil {
		return nil, fmt.Errorf("not connected")
	}

	args := amqp.Table{}
	if options.DeadLetterQueue != "" {
		args["x-dead-letter-exchange"] = ""
		args["x-dead-letter-routing-key"] = options.DeadLetterQueue
	}

	q, err := ch.QueueDeclare(
		options.Queue, // name
		true,          // durable
		false,         // delete when unused
		false,         // exclusive
		false,         // no-wait
		args,          // arguments
	)

	if err != nil {
		return nil, err
	}

	if topic != "" && topic != options.Queue {
		err = ch.QueueBind(
			q.Name, // queue name
			topic,  // routing key
			"",     // exchange (default direct)
			false,
			nil,
		)
		if err != nil {
			return nil, err
		}
	}

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		options.AutoAck,
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		return nil, err
	}

	go func() {
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

			if err := handler(context.Background(), event); err != nil {
				if !options.AutoAck {
					d.Nack(false, true)
				}
			} else {
				if !options.AutoAck {
					d.Ack(false)
				}
			}
		}
	}()

	return &rmqSubscriber{
		topic: topic,
		opts:  options,
	}, nil
}

func (r *rmqBroker) String() string {
	return "rabbitmq"
}

type rmqSubscriber struct {
	topic string
	opts  broker.SubscribeOptions
}

func (s *rmqSubscriber) Options() broker.SubscribeOptions { return s.opts }
func (s *rmqSubscriber) Topic() string                    { return s.topic }
func (s *rmqSubscriber) Unsubscribe() error               { return nil }

type rmqEvent struct {
	topic    string
	message  *broker.Message
	delivery amqp.Delivery
}

func (e *rmqEvent) Topic() string            { return e.topic }
func (e *rmqEvent) Message() *broker.Message { return e.message }
func (e *rmqEvent) Ack() error               { return e.delivery.Ack(false) }
func (e *rmqEvent) Error() error             { return nil }

func NewBroker(opts ...broker.Option) broker.Broker {
	options := broker.NewOptions(opts...)
	return &rmqBroker{
		opts: *options,
	}
}

func stringMapToTable(m map[string]string) map[string]interface{} {
	res := make(map[string]interface{})
	for k, v := range m {
		res[k] = v
	}
	return res
}
