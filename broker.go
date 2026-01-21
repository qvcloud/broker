package broker

import (
	"context"
	"sync"
)

// Broker is an interface used for asynchronous messaging.
// It provides a unified API to interact with different message brokers.
type Broker interface {
	// Init initializes the broker with options.
	// It should only validate configuration and not establish network connections.
	Init(...Option) error
	// Options returns the broker options.
	Options() Options
	// Address returns the broker address.
	Address() string
	// Connect connects the broker to the message service.
	// All network initialization and client creation should happen here.
	Connect() error
	// Disconnect disconnects the broker from the message service.
	Disconnect() error
	// Publish publishes a message to a topic.
	Publish(ctx context.Context, topic string, msg *Message, opts ...PublishOption) error
	// Subscribe subscribes to a topic with a handler.
	Subscribe(topic string, h Handler, opts ...SubscribeOption) (Subscriber, error)
	// String returns the broker implementation name.
	String() string
}

// Handler is used to process messages via a subscription of a topic.
type Handler func(context.Context, Event) error

// Message is a message send/received from the broker.
type Message struct {
	// Header contains message metadata.
	Header map[string]string
	// Body contains the message payload.
	Body []byte
	// Partition is the partition ID for brokers that support it (like Kafka).
	Partition int32
}

// Event is given to a subscription handler for processing.
// It contains the message and metadata about the topic.
type Event interface {
	// Topic returns the topic name.
	Topic() string
	// Message returns the received message.
	Message() *Message
	// Ack acknowledges the message.
	Ack() error
	// Nack negatively acknowledges the message.
	// If requeue is true, the message will be returned to the queue if supported.
	Nack(requeue bool) error
	// Error returns any error occurred during processing.
	Error() error
}

// Subscriber is a convenience return type for the Subscribe method.
// It allows managing the subscription lifecycle.
type Subscriber interface {
	// Options returns the subscription options.
	Options() SubscribeOptions
	// Topic returns the subscribed topic.
	Topic() string
	// Unsubscribe stops the subscription.
	Unsubscribe() error
}

// Marshaler is a simple encoding interface.
type Marshaler interface {
	Marshal(interface{}) ([]byte, error)
	Unmarshal([]byte, interface{}) error
	String() string
}

type trackerKey struct{}

// OptionTracker tracks which options have been registered and consumed.
type OptionTracker struct {
	mu         sync.Mutex
	registered map[any]string
	consumed   map[any]struct{}
}

// WithTrackedValue adds a value to the context and registers it for tracking.
func WithTrackedValue(ctx context.Context, key, val any, name string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	tracker, ok := ctx.Value(trackerKey{}).(*OptionTracker)
	if !ok {
		tracker = &OptionTracker{
			registered: make(map[any]string),
			consumed:   make(map[any]struct{}),
		}
		ctx = context.WithValue(ctx, trackerKey{}, tracker)
	}
	tracker.mu.Lock()
	tracker.registered[key] = name
	tracker.mu.Unlock()
	return context.WithValue(ctx, key, val)
}

// TrackOptions initializes an OptionTracker in the context.
func TrackOptions(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Value(trackerKey{}).(*OptionTracker); ok {
		return ctx
	}
	return context.WithValue(ctx, trackerKey{}, &OptionTracker{
		registered: make(map[any]string),
		consumed:   make(map[any]struct{}),
	})
}

// GetTrackedValue retrieves a value from the context and marks it as consumed.
func GetTrackedValue(ctx context.Context, key any) any {
	if ctx == nil {
		return nil
	}
	val := ctx.Value(key)
	if val != nil {
		if tracker, ok := ctx.Value(trackerKey{}).(*OptionTracker); ok {
			tracker.mu.Lock()
			tracker.consumed[key] = struct{}{}
			tracker.mu.Unlock()
		}
	}
	return val
}

// WarnUnconsumed logs a warning if any registered options were not consumed.
func WarnUnconsumed(ctx context.Context, logger Logger) {
	if ctx == nil || logger == nil {
		return
	}
	tracker, ok := ctx.Value(trackerKey{}).(*OptionTracker)
	if !ok {
		return
	}

	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	for key, name := range tracker.registered {
		if _, consumed := tracker.consumed[key]; !consumed {
			logger.Logf("Warning: option %q was ignored by the implementation", name)
		}
	}
}
