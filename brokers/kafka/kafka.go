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
	ctx     context.Context
	cancel  context.CancelFunc
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

	dialer := &kafka.Dialer{
		ClientID:  k.opts.ClientID,
		Timeout:   10 * time.Second,
		DualStack: true,
		TLS:       k.opts.TLSConfig,
	}

	balancer := kafka.Balancer(&kafka.LeastBytes{})
	batchSize := 0
	requiredAcks := kafka.RequiredAcks(0) // Default

	if k.opts.Context != nil {
		if v, ok := broker.GetTrackedValue(k.opts.Context, balancerKey{}).(kafka.Balancer); ok {
			balancer = v
		}
		if v, ok := broker.GetTrackedValue(k.opts.Context, batchSizeKey{}).(int); ok {
			batchSize = v
		}
		if v, ok := broker.GetTrackedValue(k.opts.Context, acksKey{}).(int); ok {
			requiredAcks = kafka.RequiredAcks(v)
		}
		// Acknowledge options used elsewhere
		broker.GetTrackedValue(k.opts.Context, minBytesKey{})
		broker.GetTrackedValue(k.opts.Context, maxBytesKey{})
		broker.GetTrackedValue(k.opts.Context, offsetKey{})
	}

	k.writer = &kafka.Writer{
		Addr:         kafka.TCP(k.opts.Addrs...),
		Balancer:     balancer,
		RequiredAcks: requiredAcks,
		Transport: &kafka.Transport{
			Dial: dialer.DialFunc,
			TLS:  k.opts.TLSConfig,
		},
		BatchSize: batchSize,
	}

	k.ctx, k.cancel = context.WithCancel(context.Background())
	k.running = true

	broker.WarnUnconsumed(k.opts.Context, k.opts.Logger)
	return nil
}

func (k *kafkaBroker) Disconnect() error {
	k.Lock()
	defer k.Unlock()

	if !k.running {
		return nil
	}

	if k.cancel != nil {
		k.cancel()
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

	partition := 0
	if options.Context != nil {
		if v, ok := broker.GetTrackedValue(options.Context, partitionKey{}).(int); ok {
			partition = v
		}
	}

	err := k.writer.WriteMessages(ctx, kafka.Message{
		Topic:     topic,
		Partition: partition,
		Key:       []byte(options.ShardingKey),
		Value:     msg.Body,
		Headers:   headers,
	})

	broker.WarnUnconsumed(options.Context, k.opts.Logger)
	return err
}

func (k *kafkaBroker) Subscribe(topic string, handler broker.Handler, opts ...broker.SubscribeOption) (broker.Subscriber, error) {
	options := broker.NewSubscribeOptions(opts...)

	dialer := &kafka.Dialer{
		ClientID:  k.opts.ClientID,
		Timeout:   10 * time.Second,
		DualStack: true,
		TLS:       k.opts.TLSConfig,
	}

	minBytes := 10e3
	maxBytes := 10e6
	var offset int64 = 0

	if k.opts.Context != nil {
		if v, ok := broker.GetTrackedValue(k.opts.Context, minBytesKey{}).(int); ok {
			minBytes = float64(v)
		}
		if v, ok := broker.GetTrackedValue(k.opts.Context, maxBytesKey{}).(int); ok {
			maxBytes = float64(v)
		}
		if v, ok := broker.GetTrackedValue(k.opts.Context, offsetKey{}).(int64); ok {
			offset = v
		}
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     k.opts.Addrs,
		GroupID:     options.Queue,
		Topic:       topic,
		MinBytes:    int(minBytes),
		MaxBytes:    int(maxBytes),
		Dialer:      dialer,
		StartOffset: offset,
	})

	k.Lock()
	brokerCtx := k.ctx
	if k.readers == nil {
		k.readers = make(map[string]*kafka.Reader)
	}
	subID := fmt.Sprintf("%s-%s-%d", topic, options.Queue, time.Now().UnixNano())
	k.readers[subID] = reader
	k.Unlock()

	if brokerCtx == nil {
		brokerCtx = context.Background()
	}

	ctx, cancel := context.WithCancel(brokerCtx)

	go func() {
		defer cancel()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				m, err := reader.FetchMessage(ctx)
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					if k.opts.Logger != nil {
						k.opts.Logger.Logf("Kafka fetch error: %v", err)
					}
					time.Sleep(time.Second)
					continue
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
					ctx:     ctx,
				}

				if err := handler(ctx, event); err == nil && options.AutoAck {
					event.Ack()
				}
			}
		}
	}()

	return &kafkaSubscriber{
		topic:  topic,
		opts:   options,
		reader: reader,
		cancel: cancel,
	}, nil
}

func (k *kafkaBroker) String() string {
	return "kafka"
}

type kafkaSubscriber struct {
	topic  string
	opts   broker.SubscribeOptions
	reader *kafka.Reader
	cancel context.CancelFunc
}

func (s *kafkaSubscriber) Options() broker.SubscribeOptions {
	return s.opts
}

func (s *kafkaSubscriber) Topic() string {
	return s.topic
}

func (s *kafkaSubscriber) Unsubscribe() error {
	if s.cancel != nil {
		s.cancel()
	}
	return s.reader.Close()
}

type kafkaEvent struct {
	topic   string
	message *broker.Message
	reader  *kafka.Reader
	rawMsg  kafka.Message
	ctx     context.Context
}

func (e *kafkaEvent) Topic() string {
	return e.topic
}

func (e *kafkaEvent) Message() *broker.Message {
	return e.message
}

func (e *kafkaEvent) Ack() error {
	return e.reader.CommitMessages(e.ctx, e.rawMsg)
}

func (e *kafkaEvent) Nack(requeue bool) error {
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

type batchSizeKey struct{}
type balancerKey struct{}
type minBytesKey struct{}
type maxBytesKey struct{}
type offsetKey struct{}
type acksKey struct{}

func WithBatchSize(size int) broker.Option {
	return func(o *broker.Options) {
		o.Context = broker.WithTrackedValue(o.Context, batchSizeKey{}, size, "kafka.WithBatchSize")
	}
}

func WithBalancer(balancer kafka.Balancer) broker.Option {
	return func(o *broker.Options) {
		o.Context = broker.WithTrackedValue(o.Context, balancerKey{}, balancer, "kafka.WithBalancer")
	}
}

func WithMinBytes(bytes int) broker.Option {
	return func(o *broker.Options) {
		o.Context = broker.WithTrackedValue(o.Context, minBytesKey{}, bytes, "kafka.WithMinBytes")
	}
}

func WithMaxBytes(bytes int) broker.Option {
	return func(o *broker.Options) {
		o.Context = broker.WithTrackedValue(o.Context, maxBytesKey{}, bytes, "kafka.WithMaxBytes")
	}
}

func WithOffset(offset int64) broker.Option {
	return func(o *broker.Options) {
		o.Context = broker.WithTrackedValue(o.Context, offsetKey{}, offset, "kafka.WithOffset")
	}
}

func WithAcks(acks int) broker.Option {
	return func(o *broker.Options) {
		o.Context = broker.WithTrackedValue(o.Context, acksKey{}, acks, "kafka.WithAcks")
	}
}

type partitionKey struct{}

func WithPartition(partition int) broker.PublishOption {
	return func(o *broker.PublishOptions) {
		o.Context = broker.WithTrackedValue(o.Context, partitionKey{}, partition, "kafka.WithPartition")
	}
}
