package middleware

import (
	"context"
	"testing"

	"github.com/qvcloud/broker"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

type mockEvent struct {
	topic string
}

func (m *mockEvent) Topic() string            { return m.topic }
func (m *mockEvent) Message() *broker.Message { return &broker.Message{} }
func (m *mockEvent) Ack() error               { return nil }
func (m *mockEvent) Nack(requeue bool) error  { return nil }
func (m *mockEvent) Error() error             { return nil }

func TestOtelHandler(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := trace.NewTracerProvider(trace.WithSpanProcessor(sr))
	tracer := tp.Tracer("test")

	h := OtelHandler(func(ctx context.Context, event broker.Event) error {
		return nil
	}, WithTracer(tracer))

	event := &mockEvent{topic: "test-topic"}
	err := h(context.Background(), event)
	assert.NoError(t, err)

	spans := sr.Ended()
	assert.Len(t, spans, 1)
	assert.Equal(t, "broker.handle", spans[0].Name())

	attrs := spans[0].Attributes()
	foundTopic := false
	for _, attr := range attrs {
		if attr.Key == attribute.Key("messaging.destination") {
			assert.Equal(t, "test-topic", attr.Value.AsString())
			foundTopic = true
		}
	}
	assert.True(t, foundTopic)
}
