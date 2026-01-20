package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/qvcloud/broker"
	"github.com/qvcloud/broker/brokers/rabbitmq"
)

func main() {
	b := rabbitmq.NewBroker(
		broker.Addrs("amqp://guest:guest@localhost:5672/"),
	)

	if err := b.Connect(); err != nil {
		log.Fatalf("Connect error: %v", err)
	}
	defer b.Disconnect()

	// Subscribe
	_, err := b.Subscribe("test_topic", func(ctx context.Context, event broker.Event) error {
		fmt.Printf("Received message: %s\n", string(event.Message().Body))
		return nil
	}, broker.Queue("test_queue"))
	if err != nil {
		log.Fatalf("Subscribe error: %v", err)
	}

	// Publish
	for i := 0; i < 5; i++ {
		msg := &broker.Message{
			Body: []byte(fmt.Sprintf("hello rabbitmq %d", i)),
		}
		if err := b.Publish(context.Background(), "test_topic", msg); err != nil {
			log.Printf("Publish error: %v", err)
		}
		time.Sleep(time.Second)
	}
}
