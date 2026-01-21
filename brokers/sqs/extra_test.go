package sqs

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/qvcloud/broker"
	"github.com/stretchr/testify/assert"
)

func TestSQS_RunLoop(t *testing.T) {
	mock := &mockSQSAPI{}
	b := &sqsBroker{
		client: mock,
		opts:   broker.Options{},
	}

	ctx, cancel := context.WithCancel(context.Background())

	msgChan := make(chan string, 1)
	mock.receiveMessage = func(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			return &sqs.ReceiveMessageOutput{
				Messages: []types.Message{
					{
						Body:          aws.String("hello"),
						ReceiptHandle: aws.String("handle1"),
						MessageAttributes: map[string]types.MessageAttributeValue{
							"key": {StringValue: aws.String("val"), DataType: aws.String("String")},
						},
					},
				},
			}, nil
		}
	}

	handler := func(ctx context.Context, event broker.Event) error {
		msgChan <- string(event.Message().Body)
		cancel() // Stop the loop after one message
		return nil
	}

	// Run in background
	go b.run(ctx, "http://sqs.test", handler, broker.SubscribeOptions{AutoAck: true})

	select {
	case msg := <-msgChan:
		assert.Equal(t, "hello", msg)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for message")
	}
}

func TestSQS_RunLoop_Error(t *testing.T) {
	mock := &mockSQSAPI{}
	logger := &mockLogger{}
	b := &sqsBroker{
		client: mock,
		opts: broker.Options{
			Logger: logger,
		},
	}

	ctx, cancel := context.WithCancel(context.Background())

	count := 0
	mock.receiveMessage = func(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
		count++
		if count == 1 {
			return nil, fmt.Errorf("sqs error")
		}
		cancel()
		return nil, ctx.Err()
	}

	b.run(ctx, "http://sqs.test", func(ctx context.Context, event broker.Event) error { return nil }, broker.SubscribeOptions{})

	assert.True(t, logger.called)
}

type mockLogger struct {
	called bool
}

func (l *mockLogger) Log(v ...any)                 {}
func (l *mockLogger) Logf(format string, v ...any) { l.called = true }

func TestSQS_NewBroker_Extra(t *testing.T) {
	b := NewBroker()
	assert.NotNil(t, b)
	assert.Equal(t, "sqs", b.String())
}

func TestSQS_Subscriber_Methods(t *testing.T) {
	sub := &sqsSubscriber{
		topic: "test",
		opts:  broker.SubscribeOptions{Queue: "q"},
	}
	assert.Equal(t, "test", sub.Topic())
	assert.Equal(t, "q", sub.Options().Queue)
}

func TestSQS_Broker_Options(t *testing.T) {
	b := &sqsBroker{opts: broker.Options{Addrs: []string{"addr"}}}
	assert.Equal(t, "addr", b.Options().Addrs[0])
	assert.Equal(t, "addr", b.Address())

	b2 := &sqsBroker{}
	assert.Equal(t, "", b2.Address())
}

func TestSQS_Nack_False(t *testing.T) {
	mock := &mockSQSAPI{}
	deleted := false
	mock.deleteMessage = func(ctx context.Context, params *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error) {
		deleted = true
		return &sqs.DeleteMessageOutput{}, nil
	}

	e := &sqsEvent{
		client: mock,
		topic:  "topic",
	}

	err := e.Nack(false)
	assert.NoError(t, err)
	assert.True(t, deleted)
}

func TestSQS_Connect_Running(t *testing.T) {
	b := &sqsBroker{running: true}
	assert.NoError(t, b.Connect())
}

func TestSQS_Disconnect_NotRunning(t *testing.T) {
	b := &sqsBroker{running: false}
	assert.NoError(t, b.Disconnect())
}

func TestSQS_Publish_ResolveError(t *testing.T) {
	mock := &mockSQSAPI{}
	mock.getQueueUrl = func(ctx context.Context, params *sqs.GetQueueUrlInput, optFns ...func(*sqs.Options)) (*sqs.GetQueueUrlOutput, error) {
		return nil, fmt.Errorf("resolve error")
	}

	b := &sqsBroker{client: mock, running: true}
	err := b.Publish(context.Background(), "topic", &broker.Message{})
	assert.Error(t, err)
}

func TestSQS_Subscribe_ResolveError(t *testing.T) {
	mock := &mockSQSAPI{}
	mock.getQueueUrl = func(ctx context.Context, params *sqs.GetQueueUrlInput, optFns ...func(*sqs.Options)) (*sqs.GetQueueUrlOutput, error) {
		return nil, fmt.Errorf("resolve error")
	}

	b := &sqsBroker{client: mock, running: true, ctx: context.Background()}
	_, err := b.Subscribe("topic", nil)
	assert.Error(t, err)
}

func TestSQS_Run_WaitOptions(t *testing.T) {
	b := NewBroker(
		WithMaxNumberOfMessages(5),
		WithWaitTimeSeconds(10),
		WithVisibilityTimeout(30),
	).(*sqsBroker)

	mock := &mockSQSAPI{}
	b.client = mock

	ctx, cancel := context.WithCancel(context.Background())
	mock.receiveMessage = func(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
		assert.Equal(t, int32(5), params.MaxNumberOfMessages)
		assert.Equal(t, int32(10), params.WaitTimeSeconds)
		assert.Equal(t, int32(30), params.VisibilityTimeout)
		cancel()
		return nil, ctx.Err()
	}

	// Ensure Connect is called to init ctx
	b.newClient = func(ctx context.Context) (sqsAPI, error) { return mock, nil }
	b.Connect()

	b.run(ctx, "http://sqs.test", func(ctx context.Context, p broker.Event) error { return nil }, broker.SubscribeOptions{})
}

func TestSQS_Options_NilContext(t *testing.T) {
	o := &broker.Options{}
	WithMaxNumberOfMessages(5)(o)
	assert.NotNil(t, o.Context)

	o = &broker.Options{}
	WithWaitTimeSeconds(10)(o)
	assert.NotNil(t, o.Context)

	o = &broker.Options{}
	WithVisibilityTimeout(30)(o)
	assert.NotNil(t, o.Context)

	po := &broker.PublishOptions{}
	WithDeduplicationId("id")(po)
	assert.NotNil(t, po.Context)
}
