package contracts
// Package broker defines the unified messaging contracts.
package broker






































}	Unsubscribe() error	Topic() string	Options() SubscribeOptionstype Subscriber interface {// Subscriber represents a subscription handle.}	Error() error	Ack() error	Message() *Message	Topic() stringtype Event interface {// Event is the interface for received messages.type Handler func(context.Context, Event) error// Handler is the function called when a message is received.}	String() string	// String returns the name of the broker implementation.	Subscribe(topic string, handler Handler, opts ...SubscribeOption) (Subscriber, error)	// Subscribe registers a handler for a topic.	Publish(ctx context.Context, topic string, msg *Message, opts ...PublishOption) error	// Publish sends a message to a topic.	Disconnect() error	// Disconnect closes the connection and cleans up resources.	Connect() error	// Connect establishes the physical connection to the MQ server.	Options() Options	// Options returns the current broker options.	Init(opts ...Option) error	// Init initializes the broker with options.type Broker interface {// Broker is the core interface for MQ interactions.import "context"