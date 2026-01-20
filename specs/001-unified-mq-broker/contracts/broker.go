package contracts

import "context"

// Broker is the core interface for MQ interactions.
type Broker interface {
	// Init initializes the broker with options.
	Init(opts ...any) error
	// Options returns the current broker options.
	Options() any
	// Connect establishes the physical connection to the MQ server.
	Connect() error
	// Disconnect closes the connection and cleans up resources.
	Disconnect() error
	// Publish sends a message to a topic.
	Publish(ctx context.Context, topic string, msg any, opts ...any) error
	// Subscribe registers a handler for a topic.
	Subscribe(topic string, handler any, opts ...any) (Subscriber, error)
	// String returns the name of the broker implementation.
	String() string
}

type Subscriber interface {
	Options() any
	Topic() string
	Unsubscribe() error
}
