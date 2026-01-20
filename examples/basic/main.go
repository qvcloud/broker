package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/qvcloud/broker"
)

func main() {
	// Initialize a no-op broker for demonstration
	b := broker.NewNoopBroker()

	// Connect to the broker
	if err := b.Connect(); err != nil {
		log.Fatalf("failed to connect: %v", err)
	}
	defer b.Disconnect()

	// Subscribe to a topic
	topic := "example.topic"
	sub, err := b.Subscribe(topic, func(ctx context.Context, event broker.Event) error {
		msg := event.Message()
		fmt.Printf("Received message: %s\n", string(msg.Body))
		return nil
	})
	if err != nil {
		log.Fatalf("failed to subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	// Publish a message
	msg := &broker.Message{
		Body: []byte("hello world"),
	}
	if err := b.Publish(context.Background(), topic, msg); err != nil {
		log.Fatalf("failed to publish: %v", err)
	}

	// Wait a bit for the async handler to process the message
	time.Sleep(100 * time.Millisecond)
}
