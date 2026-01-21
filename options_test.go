package broker

import (
	"context"
	"crypto/tls"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
)

type testLogger struct{}

func (t *testLogger) Log(v ...any)                 {}
func (t *testLogger) Logf(format string, v ...any) {}

func TestOptions(t *testing.T) {
	opts := NewOptions(
		Addrs("localhost:9092"),
		Secure(true),
		ClientID("test-client"),
		WithLogger(&testLogger{}),
		ErrorHandler(func(context.Context, Event) error { return nil }),
		TLSConfig(&tls.Config{}),
		Tracer(trace.NewNoopTracerProvider().Tracer("test")),
		Meter(noop.NewMeterProvider().Meter("test")),
		Codec(JsonMarshaler{}),
	)

	assert.Equal(t, []string{"localhost:9092"}, opts.Addrs)
	assert.True(t, opts.Secure)
	assert.Equal(t, "test-client", opts.ClientID)
	assert.NotNil(t, opts.Logger)
	assert.NotNil(t, opts.ErrorHandler)
	assert.NotNil(t, opts.TLSConfig)
	assert.NotNil(t, opts.Tracer)
	assert.NotNil(t, opts.Meter)
	assert.NotNil(t, opts.Codec)
}

func TestSubscribeOptions(t *testing.T) {
	opts := NewSubscribeOptions(
		DisableAutoAck(),
		Queue("test-queue"),
		WithQueue("test-queue-2"),
		WithDeadLetterQueue("test-dlq"),
		SubscribeContext(context.WithValue(context.Background(), "key", "val")),
	)

	assert.False(t, opts.AutoAck)
	assert.Equal(t, "test-queue-2", opts.Queue)
	assert.Equal(t, "test-dlq", opts.DeadLetterQueue)
	assert.Equal(t, "val", opts.Context.Value("key"))
}

func TestPublishOptions(t *testing.T) {
	opts := PublishOptions{}

	PublishContext(context.WithValue(context.Background(), "key", "val"))(&opts)
	WithShardingKey("shard-1")(&opts)
	WithDelay(time.Second)(&opts)
	WithTags("tag1", "tag2")(&opts)

	assert.Equal(t, "val", opts.Context.Value("key"))
	assert.Equal(t, "shard-1", opts.ShardingKey)
	assert.Equal(t, time.Second, opts.Delay)
	assert.Equal(t, []string{"tag1", "tag2"}, opts.Tags)
}
