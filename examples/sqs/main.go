package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/qvcloud/broker"
	"github.com/qvcloud/broker/brokers/sqs"
)

func main() {
	// SQS uses default AWS credentials from environment/config
	b := sqs.NewBroker()

	if err := b.Init(); err != nil {
		log.Fatalf("Broker Init error: %v", err)
	}

	if err := b.Connect(); err != nil {
		log.Fatalf("Broker Connect error: %v", err)
	}
	defer b.Disconnect()

	// queueURL := "your-sqs-queue-url"
	queueURL := "your-sqs-queue-url" 

	// Subscribe
	_, err := b.Subscribe(queueURL, func(ctx context.Context, p broker.Event) error {
		fmt.Printf("[Received] Message: %s, Header: %v\n", string(p.Message().Body), p.Message().Header)
		return nil
	})
	if err != nil {
		log.Fatalf("Subscribe error: %v", err)
	}

	// Publish with Delay
	t := time.NewTicker(2 * time.Second)
	for i := 0; i < 3; i++ {
		<-t.C
		msg := &broker.Message{
			Header: map[string]string{
				"Id": fmt.Sprintf("%d", i),
			},
			Body: []byte(fmt.Sprintf("sqs message %d", i)),
		}

		if err := b.Publish(context.Background(), queueURL, msg, broker.WithDelay(5*time.Second)); err != nil {
			log.Printf("[Publish] Error: %v", err)
		} else {
			log.Printf("[Publish] Sent message %d with 5s delay", i)
		}
	}

	// Wait for messages
	time.Sleep(15 * time.Second)
}
