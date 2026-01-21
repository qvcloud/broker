package broker

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestNoopBroker(t *testing.T) {
	b := NewNoopBroker()

	topic := "test"
	msgReceived := make(chan string, 1)

	sub, err := b.Subscribe(topic, func(ctx context.Context, event Event) error {
		msgReceived <- string(event.Message().Body)
		return nil
	})
	assert.NoError(t, err)
	defer sub.Unsubscribe()

	err = b.Publish(context.Background(), topic, &Message{
		Body: []byte("hello"),
	})
	assert.NoError(t, err)

	select {
	case msg := <-msgReceived:
		assert.Equal(t, "hello", msg)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for message")
	}
}

func TestNoopBroker_Tracing(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := trace.NewTracerProvider(trace.WithSpanProcessor(sr))
	tracer := tp.Tracer("test")

	b := NewNoopBroker(Tracer(tracer))

	err := b.Publish(context.Background(), "test-topic", &Message{Body: []byte("hello")})
	assert.NoError(t, err)

	spans := sr.Ended()
	assert.Len(t, spans, 1)
	assert.Equal(t, "broker.publish", spans[0].Name())
}

func TestNoopBroker_Methods(t *testing.T) {
	b := NewNoopBroker()
	assert.Equal(t, "", b.Address())
	assert.Equal(t, "noop", b.String())

	err := b.Init()
	assert.NoError(t, err)

	err = b.Connect()
	assert.NoError(t, err)

	err = b.Disconnect()
	assert.NoError(t, err)
}

func TestNoopEvent(t *testing.T) {
	opts := &Options{Codec: JsonMarshaler{}}

	t.Run("MessageByte", func(t *testing.T) {
		msg := &Message{Body: []byte("test")}
		data, _ := opts.Codec.Marshal(msg)
		event := &noopEvent{opts: opts, message: data}
		assert.Equal(t, msg.Body, event.Message().Body)
	})

	t.Run("MessageStruct", func(t *testing.T) {
		msg := &Message{Body: []byte("test")}
		event := &noopEvent{opts: opts, message: msg}
		assert.Equal(t, msg, event.Message())
	})

	t.Run("AckNackError", func(t *testing.T) {
		event := &noopEvent{err: context.DeadlineExceeded}
		assert.NoError(t, event.Ack())
		assert.NoError(t, event.Nack(true))
		assert.Equal(t, context.DeadlineExceeded, event.Error())
		assert.Equal(t, "", event.Topic())
	})

	t.Run("MessageOther", func(t *testing.T) {
		event := &noopEvent{message: 123}
		assert.Nil(t, event.Message())
	})
}

func TestNoopSubscriber_Methods(t *testing.T) {
	s := &noopSubscriber{
		topic: "test",
		exit:  make(chan bool, 1),
	}
	assert.Equal(t, "test", s.Topic())
	assert.NotNil(t, s.Options())

	err := s.Unsubscribe()
	assert.NoError(t, err)
	assert.True(t, <-s.exit)
}
