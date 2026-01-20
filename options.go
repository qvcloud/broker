package broker

import (
	"context"
	"crypto/tls"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Options contains the broker configuration.
type Options struct {
	// Addrs is a list of broker addresses.
	Addrs []string
	// Secure specifies whether to use a secure connection.
	Secure bool
	// Codec is the marshaler used for encoding/decoding messages.
	Codec Marshaler

	// ErrorHandler is called when an error occurs during message handling.
	ErrorHandler Handler

	// TLSConfig is the TLS configuration for secure connections.
	TLSConfig *tls.Config

	// Tracer is the OpenTelemetry tracer for observability.
	Tracer trace.Tracer
	// Meter is the OpenTelemetry meter for observability.
	Meter metric.Meter

	// Context is the underlying context for custom options.
	Context context.Context
}

// PublishOptions contains options for publishing a message.
type PublishOptions struct {
	// Context is the context for the publish operation.
	Context context.Context
	// ShardingKey is the key used for sharding/partitioning.
	ShardingKey string
}

// SubscribeOptions contains options for subscribing to a topic.
type SubscribeOptions struct {
	// AutoAck specifies whether to automatically acknowledge messages.
	AutoAck bool
	// Queue is the consumer group name or queue name.
	Queue string

	// Context is the context for the subscribe operation.
	Context context.Context
}

type Option func(*Options)

type PublishOption func(*PublishOptions)

type SubscribeOption func(*SubscribeOptions)

func NewOptions(opts ...Option) *Options {
	options := Options{
		Context: context.Background(),
	}

	for _, o := range opts {
		o(&options)
	}

	return &options
}

func NewSubscribeOptions(opts ...SubscribeOption) SubscribeOptions {
	opt := SubscribeOptions{
		AutoAck: true,
		Context: context.Background(),
	}

	for _, o := range opts {
		o(&opt)
	}

	return opt
}

// Addrs sets the host addresses to be used by the broker.
func Addrs(addrs ...string) Option {
	return func(o *Options) {
		o.Addrs = addrs
	}
}

// Codec sets the codec used for encoding/decoding used where
// a broker does not support headers.
func Codec(c Marshaler) Option {
	return func(o *Options) {
		o.Codec = c
	}
}

// DisableAutoAck will disable auto acking of messages
// after they have been handled.
func DisableAutoAck() SubscribeOption {
	return func(o *SubscribeOptions) {
		o.AutoAck = false
	}
}

// ErrorHandler will catch all broker errors that cant be handled
// in normal way, for example Codec errors.
func ErrorHandler(h Handler) Option {
	return func(o *Options) {
		o.ErrorHandler = h
	}
}

// Queue sets the name of the queue to share messages on.
func Queue(name string) SubscribeOption {
	return func(o *SubscribeOptions) {
		o.Queue = name
	}
}

// Secure communication with the broker.
func Secure(b bool) Option {
	return func(o *Options) {
		o.Secure = b
	}
}

// Tracer sets the tracer used for observability.
func Tracer(t trace.Tracer) Option {
	return func(o *Options) {
		o.Tracer = t
	}
}

// Meter sets the meter used for observability.
func Meter(m metric.Meter) Option {
	return func(o *Options) {
		o.Meter = m
	}
}

// Specify TLS Config.
func TLSConfig(t *tls.Config) Option {
	return func(o *Options) {
		o.TLSConfig = t
	}
}

// PublishContext set context.
func PublishContext(ctx context.Context) PublishOption {
	return func(o *PublishOptions) {
		o.Context = ctx
	}
}

func WithShardingKey(v string) PublishOption {
	return func(o *PublishOptions) {
		o.ShardingKey = v
	}
}

// SubscribeContext set context.
func SubscribeContext(ctx context.Context) SubscribeOption {
	return func(o *SubscribeOptions) {
		o.Context = ctx
	}
}
