package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/qvcloud/broker"
	"github.com/qvcloud/broker/brokers/redis"
)

func main() {
	b := redis.NewBroker(
		broker.Addrs("127.0.0.1:6379"),
		redis.WithPassword(""),
		redis.WithDB(0),
	)

	if err := b.Connect(); err != nil {
		log.Fatalf("Cant connect to redis: %v", err)
	}
	defer b.Disconnect()

	topic := "test_topic"

	// Subscribe
	_, err := b.Subscribe(topic, func(ctx context.Context, event broker.Event) error {
		fmt.Printf("Received message: %s\n", string(event.Message().Body))
		return nil
	}, broker.Queue("my_consumer_group"))

	if err != nil {
		log.Fatal(err)
	}

	// Publish
	for i := 0; i < 5000; i++ {
		msg := &broker.Message{
			Body: []byte(fmt.Sprintf("Hello Redis Streams %d", i)),
		}
		if err := b.Publish(context.Background(), topic, msg, redis.WithMaxLen(1000)); err != nil {
			log.Printf("Publish error: %v", err)
		}
		time.Sleep(time.Second)
	}

	time.Sleep(time.Second * 5)
}
