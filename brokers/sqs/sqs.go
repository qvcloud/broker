package sqs

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/qvcloud/broker"
)

type sqsAPI interface {
	SendMessage(ctx context.Context, params *sqs.SendMessageInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
	ReceiveMessage(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	DeleteMessage(ctx context.Context, params *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
	ChangeMessageVisibility(ctx context.Context, params *sqs.ChangeMessageVisibilityInput, optFns ...func(*sqs.Options)) (*sqs.ChangeMessageVisibilityOutput, error)
}

type sqsBroker struct {
	opts   broker.Options
	client sqsAPI

	sync.RWMutex
	running bool
	ctx     context.Context
	cancel  context.CancelFunc
}

func (s *sqsBroker) Options() broker.Options { return s.opts }

func (s *sqsBroker) Address() string {
	if len(s.opts.Addrs) > 0 {
		return s.opts.Addrs[0]
	}
	return ""
}

func (s *sqsBroker) Init(opts ...broker.Option) error {
	for _, o := range opts {
		o(&s.opts)
	}
	return nil
}

func (s *sqsBroker) Connect() error {
	s.Lock()
	defer s.Unlock()

	if s.running {
		return nil
	}

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return err
	}

	s.client = sqs.NewFromConfig(cfg)
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.running = true
	return nil
}

func (s *sqsBroker) Disconnect() error {
	s.Lock()
	defer s.Unlock()

	if !s.running {
		return nil
	}

	if s.cancel != nil {
		s.cancel()
	}

	s.client = nil
	s.running = false
	return nil
}

func (s *sqsBroker) Publish(ctx context.Context, topic string, msg *broker.Message, opts ...broker.PublishOption) error {
	options := broker.PublishOptions{
		Context: ctx,
	}
	for _, o := range opts {
		o(&options)
	}

	s.RLock()
	client := s.client
	s.RUnlock()

	if client == nil {
		return fmt.Errorf("not connected")
	}

	queueUrl := topic

	input := &sqs.SendMessageInput{
		QueueUrl:          aws.String(queueUrl),
		MessageBody:       aws.String(string(msg.Body)),
		MessageAttributes: make(map[string]types.MessageAttributeValue),
	}

	if options.Delay > 0 {
		input.DelaySeconds = int32(options.Delay.Seconds())
	}

	for k, v := range msg.Header {
		input.MessageAttributes[k] = types.MessageAttributeValue{
			DataType:    aws.String("String"),
			StringValue: aws.String(v),
		}
	}

	_, err := client.SendMessage(ctx, input)
	return err
}

func (s *sqsBroker) Subscribe(topic string, handler broker.Handler, opts ...broker.SubscribeOption) (broker.Subscriber, error) {
	options := broker.NewSubscribeOptions(opts...)

	s.RLock()
	client := s.client
	brokerCtx := s.ctx
	s.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("not connected")
	}

	if brokerCtx == nil {
		brokerCtx = context.Background()
	}

	ctx, cancel := context.WithCancel(brokerCtx)

	sub := &sqsSubscriber{
		topic:  topic,
		opts:   options,
		cancel: cancel,
	}

	go s.run(ctx, topic, handler, options)

	return sub, nil
}

func (s *sqsBroker) run(ctx context.Context, queueUrl string, handler broker.Handler, options broker.SubscribeOptions) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			s.RLock()
			client := s.client
			s.RUnlock()

			if client == nil {
				time.Sleep(time.Second)
				continue
			}

			// Extract options
			maxMessages := int32(10)
			waitTime := int32(20)
			visibilityTimeout := int32(0) // 0 means use queue default

			if s.opts.Context != nil {
				if v, ok := s.opts.Context.Value(maxNumberOfMessagesKey{}).(int32); ok {
					maxMessages = v
				}
				if v, ok := s.opts.Context.Value(waitTimeSecondsKey{}).(int32); ok {
					waitTime = v
				}
				if v, ok := s.opts.Context.Value(visibilityTimeoutKey{}).(int32); ok {
					visibilityTimeout = v
				}
			}

			input := &sqs.ReceiveMessageInput{
				QueueUrl:              aws.String(queueUrl),
				MaxNumberOfMessages:   maxMessages,
				WaitTimeSeconds:       waitTime, // Long polling
				MessageAttributeNames: []string{"All"},
			}
			if visibilityTimeout > 0 {
				input.VisibilityTimeout = visibilityTimeout
			}

			output, err := s.client.ReceiveMessage(ctx, input)
			if err != nil {
				time.Sleep(time.Second)
				continue
			}

			for _, sm := range output.Messages {
				header := make(map[string]string)
				for k, v := range sm.MessageAttributes {
					if v.StringValue != nil {
						header[k] = *v.StringValue
					}
				}

				msg := &broker.Message{
					Header: header,
					Body:   []byte(*sm.Body),
				}

				event := &sqsEvent{
					topic:   queueUrl,
					message: msg,
					sm:      sm,
					client:  s.client,
					ctx:     ctx,
				}

				if err := handler(ctx, event); err == nil && options.AutoAck {
					event.Ack()
				}
			}
		}
	}
}

func (s *sqsBroker) String() string {
	return "sqs"
}

type sqsSubscriber struct {
	topic  string
	opts   broker.SubscribeOptions
	cancel context.CancelFunc
}

func (s *sqsSubscriber) Options() broker.SubscribeOptions { return s.opts }
func (s *sqsSubscriber) Topic() string                    { return s.topic }
func (s *sqsSubscriber) Unsubscribe() error {
	s.cancel()
	return nil
}

type sqsEvent struct {
	topic   string
	message *broker.Message
	sm      types.Message
	client  sqsAPI
	ctx     context.Context
}

func (e *sqsEvent) Topic() string            { return e.topic }
func (e *sqsEvent) Message() *broker.Message { return e.message }
func (e *sqsEvent) Ack() error {
	_, err := e.client.DeleteMessage(e.ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(e.topic),
		ReceiptHandle: e.sm.ReceiptHandle,
	})
	return err
}
func (e *sqsEvent) Nack(requeue bool) error {
	if !requeue {
		return e.Ack()
	}
	// Make message available immediately by setting visibility timeout to 0
	_, err := e.client.ChangeMessageVisibility(e.ctx, &sqs.ChangeMessageVisibilityInput{
		QueueUrl:          aws.String(e.topic),
		ReceiptHandle:     e.sm.ReceiptHandle,
		VisibilityTimeout: 0,
	})
	return err
}
func (e *sqsEvent) Error() error { return nil }

func NewBroker(opts ...broker.Option) broker.Broker {
	options := broker.NewOptions(opts...)
	return &sqsBroker{
		opts: *options,
	}
}

type waitTimeSecondsKey struct{}
type visibilityTimeoutKey struct{}
type maxNumberOfMessagesKey struct{}

func WithWaitTimeSeconds(seconds int32) broker.Option {
	return func(o *broker.Options) {
		if o.Context == nil {
			o.Context = context.Background()
		}
		o.Context = context.WithValue(o.Context, waitTimeSecondsKey{}, seconds)
	}
}

func WithVisibilityTimeout(seconds int32) broker.Option {
	return func(o *broker.Options) {
		if o.Context == nil {
			o.Context = context.Background()
		}
		o.Context = context.WithValue(o.Context, visibilityTimeoutKey{}, seconds)
	}
}

func WithMaxNumberOfMessages(num int32) broker.Option {
	return func(o *broker.Options) {
		if o.Context == nil {
			o.Context = context.Background()
		}
		o.Context = context.WithValue(o.Context, maxNumberOfMessagesKey{}, num)
	}
}
