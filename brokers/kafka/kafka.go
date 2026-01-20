package kafka

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/qvcloud/broker"
	"github.com/segmentio/kafka-go"
)

type kafkaBroker struct {
	opts broker.Options

	writer *kafka.Writer

	sync.RWMutex
	readers map[string]*kafka.Reader
	running bool
}

func (k *kafkaBroker) Options() broker.Options { return k.opts }

func (k *kafkaBroker) Address() string {
	if len(k.opts.Addrs) > 0 {
		return k.opts.Addrs[0]
	}
	return ""
}

func (k *kafkaBroker) Init(opts ...broker.Option) error {
	for _, o := range opts {
		o(&k.opts)
	}
	return nil
}

func (k *kafkaBroker) Connect() error {
	k.Lock()
	defer k.Unlock()

	if k.running {
		return nil
	}

	k.writer = &kafka.Writer{
		Addr:     kafka.TCP(k.opts.Addrs...),
		Balancer: &kafka.LeastBytes{},
	}

	k.running = true
	return nil
}

func (k *kafkaBroker) Disconnect() error {
	k.Lock()
	defer k.Unlock()

	if !k.running {
		return nil
	}

	if k.writer != nil {
		k.writer.Close()
		k.writer = nil
	}

	for _, r := range k.readers {
		r.Close()
	}
	k.readers = make(map[string]*kafka.Reader)

	k.running = false
	return nil
}

func (k *kafkaBroker) Publish(ctx context.Context, topic string, msg *broker.Message, opts ...broker.PublishOption) error {
	options := broker.PublishOptions{
		Context: ctx,
	}
	for _, o := range opts {
		o(&options)
	}

	headers := []kafka.Header{}
	for key, val := range msg.Header {
		headers = append(headers, kafka.Header{
			Key:   key,
			Value: []byte(val),
		})
	}

	err := k.writer.WriteMessages(ctx, kafka.Message{
		Topic:   topic,
		Key:     []byte(options.ShardingKey),
		Value:   msg.Body,
		Headers: headers,
	})

	return err
}

func (k *kafkaBroker) Subscribe(topic string, handler broker.Handler, opts ...broker.SubscribeOption) (broker.Subscriber, error) {
	options := broker.NewSubscribeOptions(opts...)

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  k.opts.Addrs,
		GroupID:  options.Queue,
		Topic:    topic,
		MinBytes: 10e3,
		MaxBytes: 10e6,
	})

	k.Lock()
	if k.readers == nil {
		k.readers = make(map[string]*kafka.Reader)
	}
	subID := fmt.Sprintf("%s-%s-%d", topic, options.Queue, time.Now().UnixNano())
	k.readers[subID] = reader
	k.Unlock()

	go func() {
		ctx := context.Background()
		for {
			m, err := reader.ReadMessage(ctx)
			if err != nil {
				return
			}

			header := make(map[string]string)
			for _, h := range m.Headers {
				header[h.Key] = string(h.Value)
			}

			msg := &broker.Message{
				Header: header,
				Body:   m.Value,
			}

			event := &kafkaEvent{
				topic:   m.Topic,
				message: msg,
				reader:  reader,
				rawMsg:  m,
			}

			if err := handler(ctx, event); err != nil {
			}
		}
	}()

	return &kafkaSubscriber{
		topic:  topic,
		opts:   options,
		reader: reader,
	}, nil
}

func (k *kafkaBroker) String() string {
	return "kafka"
}

type kafkaSubscriber struct {
	topic  string
	opts   broker.SubscribeOptions
	reader *kafka.Reader
}

func (s *kafkaSubscriber) Options() broker.SubscribeOptions {
	return s.opts
}

func (s *kafkaSubscriber) Topic() string {
	return s.topic
}

func (s *kafkaSubscriber) Unsubscribe() error {
	return s.reader.Close()
}

type kafkaEvent struct {
	topic   string
	message *broker.Message
	reader  *kafka.Reader
	rawMsg  kafka.Message
}

func (e *kafkaEvent) Topic() string {
	return e.topic
}

func (e *kafkaEvent) Message() *broker.Message {
	return e.message
}

func (e *kafkaEvent) Ack() error {
	return nil
}

func (e *kafkaEvent) Error() error {
	return nil
}

func NewBroker(opts ...broker.Option) broker.Broker {
	options := broker.NewOptions(opts...)

	return &kafkaBroker{
		opts:    *options,
		readers: make(map[string]*kafka.Reader),
	}
}
