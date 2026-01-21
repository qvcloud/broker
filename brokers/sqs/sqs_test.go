package sqs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/qvcloud/broker"
	"github.com/stretchr/testify/assert"
)

type mockSQSAPI struct {
	getQueueUrl             func(ctx context.Context, params *sqs.GetQueueUrlInput, optFns ...func(*sqs.Options)) (*sqs.GetQueueUrlOutput, error)
	sendMessage             func(ctx context.Context, params *sqs.SendMessageInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
	receiveMessage          func(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	deleteMessage           func(ctx context.Context, params *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
	changeMessageVisibility func(ctx context.Context, params *sqs.ChangeMessageVisibilityInput, optFns ...func(*sqs.Options)) (*sqs.ChangeMessageVisibilityOutput, error)
}

func (m *mockSQSAPI) GetQueueUrl(ctx context.Context, params *sqs.GetQueueUrlInput, optFns ...func(*sqs.Options)) (*sqs.GetQueueUrlOutput, error) {
	if m.getQueueUrl != nil {
		return m.getQueueUrl(ctx, params, optFns...)
	}
	return &sqs.GetQueueUrlOutput{QueueUrl: aws.String("http://sqs.test")}, nil
}

func (m *mockSQSAPI) SendMessage(ctx context.Context, params *sqs.SendMessageInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error) {
	if m.sendMessage != nil {
		return m.sendMessage(ctx, params, optFns...)
	}
	return &sqs.SendMessageOutput{}, nil
}

func (m *mockSQSAPI) ReceiveMessage(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
	if m.receiveMessage != nil {
		return m.receiveMessage(ctx, params, optFns...)
	}
	return &sqs.ReceiveMessageOutput{}, nil
}

func (m *mockSQSAPI) DeleteMessage(ctx context.Context, params *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error) {
	if m.deleteMessage != nil {
		return m.deleteMessage(ctx, params, optFns...)
	}
	return &sqs.DeleteMessageOutput{}, nil
}

func (m *mockSQSAPI) ChangeMessageVisibility(ctx context.Context, params *sqs.ChangeMessageVisibilityInput, optFns ...func(*sqs.Options)) (*sqs.ChangeMessageVisibilityOutput, error) {
	if m.changeMessageVisibility != nil {
		return m.changeMessageVisibility(ctx, params, optFns...)
	}
	return &sqs.ChangeMessageVisibilityOutput{}, nil
}

func TestSQS_Basic(t *testing.T) {
	b := NewBroker().(*sqsBroker)
	mock := &mockSQSAPI{}

	// Test Init
	err := b.Init(broker.Addrs("sqs://localhost"))
	assert.NoError(t, err)
	assert.Equal(t, "sqs://localhost", b.Address())

	// Test Connect with mock factory
	b.newClient = func(ctx context.Context) (sqsAPI, error) {
		return mock, nil
	}

	err = b.Connect()
	assert.NoError(t, err)
	assert.True(t, b.running)
	assert.Equal(t, "sqs", b.String())

	// Test Disconnect
	err = b.Disconnect()
	assert.NoError(t, err)
	assert.False(t, b.running)
}

func TestSQS_Publish(t *testing.T) {
	b := NewBroker().(*sqsBroker)
	mock := &mockSQSAPI{}
	b.client = mock
	b.running = true

	t.Run("Success", func(t *testing.T) {
		mock.sendMessage = func(ctx context.Context, params *sqs.SendMessageInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error) {
			assert.Equal(t, "http://sqs.test", *params.QueueUrl)
			assert.Equal(t, "hello", *params.MessageBody)
			return &sqs.SendMessageOutput{}, nil
		}
		err := b.Publish(context.Background(), "test-topic", &broker.Message{Body: []byte("hello")})
		assert.NoError(t, err)
	})

	t.Run("WithOptions", func(t *testing.T) {
		mock.sendMessage = func(ctx context.Context, params *sqs.SendMessageInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error) {
			assert.Equal(t, int32(5), params.DelaySeconds)
			assert.Equal(t, "group1", *params.MessageGroupId)
			assert.Equal(t, "dedup1", *params.MessageDeduplicationId)
			assert.Equal(t, "v1", *params.MessageAttributes["k1"].StringValue)
			return &sqs.SendMessageOutput{}, nil
		}
		trackCtx := broker.TrackOptions(context.Background())
		err := b.Publish(trackCtx, "test-topic",
			&broker.Message{Body: []byte("hello"), Header: map[string]string{"k1": "v1"}},
			broker.WithDelay(5*time.Second),
			broker.WithShardingKey("group1"),
			WithDeduplicationId("dedup1"),
		)
		assert.NoError(t, err)
	})

	t.Run("NotConnected", func(t *testing.T) {
		b2 := NewBroker().(*sqsBroker)
		err := b2.Publish(context.Background(), "topic", &broker.Message{})
		assert.Error(t, err)
	})
}

func TestSQS_Subscribe(t *testing.T) {
	b := NewBroker().(*sqsBroker)
	mock := &mockSQSAPI{}
	b.client = mock
	b.running = true
	b.ctx, b.cancel = context.WithCancel(context.Background())

	t.Run("Success", func(t *testing.T) {
		sub, err := b.Subscribe("test-topic", func(ctx context.Context, event broker.Event) error {
			return nil
		})
		assert.NoError(t, err)
		assert.NotNil(t, sub)
		assert.Equal(t, "http://sqs.test", sub.Topic())

		err = sub.Unsubscribe()
		assert.NoError(t, err)
	})
}

func TestSQS_Event(t *testing.T) {
	mock := &mockSQSAPI{}
	e := &sqsEvent{
		topic:   "test",
		message: &broker.Message{Body: []byte("hi")},
		sm: types.Message{
			ReceiptHandle: aws.String("handle"),
			Body:          aws.String("hi"),
		},
		client: mock,
		ctx:    context.Background(),
	}

	assert.Equal(t, "test", e.Topic())
	assert.Equal(t, []byte("hi"), e.Message().Body)

	t.Run("Ack", func(t *testing.T) {
		mock.deleteMessage = func(ctx context.Context, params *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error) {
			assert.Equal(t, "handle", *params.ReceiptHandle)
			return &sqs.DeleteMessageOutput{}, nil
		}
		assert.NoError(t, e.Ack())
	})

	t.Run("Nack", func(t *testing.T) {
		mock.changeMessageVisibility = func(ctx context.Context, params *sqs.ChangeMessageVisibilityInput, optFns ...func(*sqs.Options)) (*sqs.ChangeMessageVisibilityOutput, error) {
			assert.Equal(t, int32(0), params.VisibilityTimeout)
			return &sqs.ChangeMessageVisibilityOutput{}, nil
		}
		assert.NoError(t, e.Nack(true))
	})

	assert.Nil(t, e.Error())
}

func TestSQS_Connect_Error(t *testing.T) {
	b := NewBroker().(*sqsBroker)
	b.newClient = func(ctx context.Context) (sqsAPI, error) {
		return nil, errors.New("conf error")
	}
	err := b.Connect()
	assert.Error(t, err)
}

func TestSQS_Options(t *testing.T) {
	b := NewBroker(
		WithMaxNumberOfMessages(5),
		WithWaitTimeSeconds(10),
		WithVisibilityTimeout(30),
	).(*sqsBroker)

	assert.NotNil(t, b.opts.Context)
}
