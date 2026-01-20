package broker

import (
	"context"
)

// Broker is an interface used for asynchronous messaging.
// It provides a unified API to interact with different message brokers.
type Broker interface {
	// Init initializes the broker with options.
	Init(...Option) error
	// Options returns the broker options.
	Options() Options
	// Address returns the broker address.
	Address() string
	// Connect connects the broker to the message service.
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
