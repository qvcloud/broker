package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/qvcloud/broker"
	"github.com/qvcloud/broker/brokers/rocketmq"
)

func main() {
	b := rocketmq.NewBroker(
		broker.Addrs("127.0.0.1:9876"),
		broker.ClientID("test-client-id"),
		rocketmq.WithGroupName("example_producer_group"),
		rocketmq.WithInstanceName("example_instance"),
		rocketmq.WithRetry(3),
		rocketmq.WithLogLevel("error"), // debug, info, warn, error, fatal
	)

	if err := b.Connect(); err != nil {
		log.Fatalf("Connect error: %v", err)
	}
	defer b.Disconnect()

	_, err := b.Subscribe("test_topic", func(ctx context.Context, event broker.Event) error {
		fmt.Printf("Received message: %s\n", string(event.Message().Body))
		return nil
	}, broker.Queue("example_consumer_group"))

	if err != nil {
		log.Fatalf("Subscribe error: %v", err)
	}

	fmt.Println("Broker connected, starting to produce messages...")

	for i := 0; i < 5000; i++ {
		msg := &broker.Message{
			Header: map[string]string{
				"index": fmt.Sprintf("%d", i),
			},
			Body: []byte(fmt.Sprintf("hello rocketmq %d", i)),
		}

		err := b.Publish(context.Background(), "test_topic", msg,
			rocketmq.WithTag("example_tag"),
		)

		if err != nil {
			log.Printf("Publish error: %v", err)
		} else {
			fmt.Printf("Published message %d\n", i)
		}

		time.Sleep(time.Second)
	}

	time.Sleep(2 * time.Second)
	fmt.Println("Example finished.")
}
