package kafka

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/qvcloud/broker"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
)

type mockWriter struct {
	writeFunc func(ctx context.Context, msgs ...kafka.Message) error
	closeFunc func() error
}

func (m *mockWriter) WriteMessages(ctx context.Context, msgs ...kafka.Message) error {
	if m.writeFunc != nil {
		return m.writeFunc(ctx, msgs...)
	}
	return nil
}
func (m *mockWriter) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

type mockReader struct {
	fetchFunc  func(ctx context.Context) (kafka.Message, error)
	commitFunc func(ctx context.Context, msgs ...kafka.Message) error
	closeFunc  func() error
}

func (m *mockReader) FetchMessage(ctx context.Context) (kafka.Message, error) {
	if m.fetchFunc != nil {
		return m.fetchFunc(ctx)
	}
	return kafka.Message{}, nil
}
func (m *mockReader) CommitMessages(ctx context.Context, msgs ...kafka.Message) error {
	if m.commitFunc != nil {
		return m.commitFunc(ctx, msgs...)
	}
	return nil
}
func (m *mockReader) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func TestKafka_Basic(t *testing.T) {
	b := NewBroker(broker.Addrs("127.0.0.1:9092"))
	assert.Equal(t, "127.0.0.1:9092", b.Address())
	assert.Equal(t, "kafka", b.String())

	err := b.Init()
	assert.NoError(t, err)
}

func TestKafka_Connect_Disconnect(t *testing.T) {
	b := NewBroker(broker.Addrs("127.0.0.1:9092"))
	k := b.(*kafkaBroker)

	k.newWriter = func(w *kafka.Writer) kafkaWriter {
		return &mockWriter{}
	}

	err := b.Connect()
	assert.NoError(t, err)
	assert.True(t, k.running)

	err = b.Disconnect()
	assert.NoError(t, err)
	assert.False(t, k.running)
}

func TestKafka_Publish(t *testing.T) {
	b := NewBroker(broker.Addrs("127.0.0.1:9092"))
	k := b.(*kafkaBroker)
	mockW := &mockWriter{}
	k.writer = mockW
	k.running = true

	t.Run("Success", func(t *testing.T) {
		var capturedMsgs []kafka.Message
		mockW.writeFunc = func(ctx context.Context, msgs ...kafka.Message) error {
			capturedMsgs = msgs
			return nil
		}

		msg := &broker.Message{
			Header: map[string]string{"H1": "V1"},
			Body:   []byte("hello"),
		}
		err := b.Publish(context.Background(), "test-topic", msg)
		assert.NoError(t, err)
		assert.Len(t, capturedMsgs, 1)
		assert.Equal(t, "test-topic", capturedMsgs[0].Topic)
		assert.Equal(t, []byte("hello"), capturedMsgs[0].Value)
	})

	t.Run("Failure", func(t *testing.T) {
		mockW.writeFunc = func(ctx context.Context, msgs ...kafka.Message) error {
			return fmt.Errorf("kafka error")
		}
		err := b.Publish(context.Background(), "test-topic", &broker.Message{Body: []byte("err")})
		assert.Error(t, err)
	})
}

func TestKafka_Subscribe(t *testing.T) {
	b := NewBroker(broker.Addrs("127.0.0.1:9092"))
	k := b.(*kafkaBroker)
	k.running = true
	k.ctx, k.cancel = context.WithCancel(context.Background())

	mockR := &mockReader{}
	k.newReader = func(cfg kafka.ReaderConfig) kafkaReader {
		return mockR
	}

	t.Run("FetchAndHandle", func(t *testing.T) {
		msgChan := make(chan struct{})
		mockR.fetchFunc = func(ctx context.Context) (kafka.Message, error) {
			select {
			case <-msgChan:
				return kafka.Message{
					Topic: "test",
					Value: []byte("world"),
					Headers: []kafka.Header{
						{Key: "K1", Value: []byte("V1")},
					},
				}, nil
			case <-ctx.Done():
				return kafka.Message{}, ctx.Err()
			}
		}

		handlerCalled := make(chan struct{})
		sub, err := b.Subscribe("test", func(ctx context.Context, event broker.Event) error {
			assert.Equal(t, []byte("world"), event.Message().Body)
			assert.Equal(t, "V1", event.Message().Header["K1"])
			close(handlerCalled)
			return nil
		})
		assert.NoError(t, err)

		msgChan <- struct{}{}
		select {
		case <-handlerCalled:
		case <-time.After(time.Second):
			t.Fatal("handler not called")
		}

		err = sub.Unsubscribe()
		assert.NoError(t, err)
	})
}

func TestKafka_Options_Tracking(t *testing.T) {
	trackedCtx := broker.TrackOptions(context.Background())
	b := NewBroker(
		broker.Addrs("127.0.0.1:9092"),
		broker.WithContext(trackedCtx),
		WithBalancer(&kafka.LeastBytes{}),
		WithBatchSize(100),
		WithAcks(1),
		WithMinBytes(1024),
		WithMaxBytes(1000000),
		WithOffset(kafka.LastOffset),
	)
	k := b.(*kafkaBroker)
	k.newWriter = func(w *kafka.Writer) kafkaWriter { return &mockWriter{} }
	k.newReader = func(cfg kafka.ReaderConfig) kafkaReader { return &mockReader{} }

	err := b.Connect()
	assert.NoError(t, err)

	_, err = b.Subscribe("test", func(ctx context.Context, e broker.Event) error { return nil })
	assert.NoError(t, err)
}

func TestKafka_Event_Methods(t *testing.T) {
	mockR := &mockReader{}
	event := &kafkaEvent{
		topic:   "test",
		message: &broker.Message{Body: []byte("test")},
		reader:  mockR,
		rawMsg:  kafka.Message{},
		ctx:     context.Background(),
	}

	assert.Equal(t, "test", event.Topic())
	assert.Equal(t, []byte("test"), event.Message().Body)
	assert.NoError(t, event.Error())

	t.Run("Ack", func(t *testing.T) {
		committed := false
		mockR.commitFunc = func(ctx context.Context, msgs ...kafka.Message) error {
			committed = true
			return nil
		}
		err := event.Ack()
		assert.NoError(t, err)
		assert.True(t, committed)
	})

	t.Run("Nack", func(t *testing.T) {
		err := event.Nack(true)
		assert.NoError(t, err)
	})
}
