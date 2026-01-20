package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/qvcloud/broker"
	"github.com/qvcloud/broker/brokers/pubsub"
)

func main() {
	// PubSub requires Project ID in Addrs
	b := pubsub.NewBroker(
		broker.Addrs("your-gcp-project-id"),
	)

	if err := b.Init(); err != nil {
		log.Fatalf("Broker Init error: %v", err)
	}

	if err := b.Connect(); err != nil {
		log.Fatalf("Broker Connect error: %v", err)
	}
	defer b.Disconnect()

	topic := "your-topic-id"
	subscription := "your-subscription-id"

	// Subscribe
	_, err := b.Subscribe(topic, func(ctx context.Context, p broker.Event) error {
		fmt.Printf("[Received] Topic: %s, Message: %s\n", p.Topic(), string(p.Message().Body))
		return nil
	}, broker.WithQueue(subscription))

	if err != nil {
		log.Fatalf("Subscribe error: %v", err)
	}

	// Publish
	msg := &broker.Message{
		Header: map[string]string{
			"Version": "1.0",
		},
		Body: []byte("hello gcp pubsub"),
	}

	if err := b.Publish(context.Background(), topic, msg); err != nil {
		log.Printf("[Publish] Error: %v", err)
	} else {
		log.Println("[Publish] Success")
	}

	// Wait for processing
	time.Sleep(5 * time.Second)
}
