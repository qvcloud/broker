package broker

import (
	"context"
	"crypto/tls"
	"time"

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

	// Logger for debug/info logging.
	Logger Logger

	// ClientID is a unique identifier for the client.
	ClientID string
}

// Logger is a simple logging interface.
type Logger interface {
	Log(v ...any)
	Logf(format string, v ...any)
}

// PublishOptions contains options for publishing a message.
type PublishOptions struct {
	// Context is the context for the publish operation.
	Context context.Context
	// ShardingKey is the key used for sharding/partitioning.
	ShardingKey string
	// Delay is the delay duration for the message.
	Delay time.Duration
	// Tags are labels for the message (e.g. for filtering).
	Tags []string
}

// SubscribeOptions contains options for subscribing to a topic.
type SubscribeOptions struct {
	// AutoAck specifies whether to automatically acknowledge messages.
	AutoAck bool
	// Queue is the consumer group name or queue name.
	Queue string
	// DeadLetterQueue is the name of the dead letter queue.
	DeadLetterQueue string

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

func NewPublishOptions(opts ...PublishOption) PublishOptions {
	opt := PublishOptions{
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

// ClientID sets the client identifier.
func ClientID(id string) Option {
	return func(o *Options) {
		o.ClientID = id
	}
}

// WithContext sets the context for the broker.
func WithContext(ctx context.Context) Option {
	return func(o *Options) {
		o.Context = ctx
	}
}

// WithLogger sets the logger for the broker.
func WithLogger(l Logger) Option {
	return func(o *Options) {
		o.Logger = l
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

// WithClientID sets the client identifier.
func WithClientID(id string) Option {
	return func(o *Options) {
		o.ClientID = id
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

// WithDelay sets the delay duration for a publish operation.
func WithDelay(d time.Duration) PublishOption {
	return func(o *PublishOptions) {
		o.Delay = d
	}
}

func WithTags(tags ...string) PublishOption {
	return func(o *PublishOptions) {
		o.Tags = tags
	}
}

// WithDeadLetterQueue sets the dead letter queue name.

func WithDeadLetterQueue(v string) SubscribeOption {
	return func(o *SubscribeOptions) {
		o.DeadLetterQueue = v
	}
}

// WithQueue sets the name of the queue or consumer group.
func WithQueue(name string) SubscribeOption {
	return func(o *SubscribeOptions) {
		o.Queue = name
	}
}

// WithAutoAck sets the auto acknowledgement for the subscription.
func WithAutoAck(ack bool) SubscribeOption {
	return func(o *SubscribeOptions) {
		o.AutoAck = ack
	}
}

// SubscribeContext set context.
func SubscribeContext(ctx context.Context) SubscribeOption {
	return func(o *SubscribeOptions) {
		o.Context = ctx
	}
}
