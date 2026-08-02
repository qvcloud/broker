//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/qvcloud/broker"
	"github.com/qvcloud/broker/brokers/kafka"
	"github.com/qvcloud/broker/brokers/nats"
	"github.com/qvcloud/broker/brokers/rabbitmq"
	"github.com/qvcloud/broker/brokers/redis"
	kafkago "github.com/segmentio/kafka-go"
)

const integrationTimeout = 30 * time.Second

func TestDockerBrokersPublishSubscribe(t *testing.T) {
	tests := []struct {
		name         string
		newBroker    func() broker.Broker
		subOpts      []broker.SubscribeOption
		prepare      func(*testing.T, string)
		queueIsTopic bool
	}{
		{
			name: "kafka",
			newBroker: func() broker.Broker {
				return kafka.NewBroker(
					broker.Addrs(env("BROKER_KAFKA_ADDR", "127.0.0.1:19092")),
					kafka.WithAllowAutoTopicCreation(true),
					kafka.WithMinBytes(1),
				)
			},
			prepare: prepareKafkaTopic,
		},
		{
			name: "rabbitmq",
			newBroker: func() broker.Broker {
				return rabbitmq.NewBroker(
					broker.Addrs(env("BROKER_RABBITMQ_ADDR", "amqp://guest:guest@127.0.0.1:15672/")),
					rabbitmq.WithDurable(false),
					rabbitmq.WithAutoDelete(true),
				)
			},
			subOpts:      []broker.SubscribeOption{broker.DisableAutoAck()},
			queueIsTopic: true,
		},
		{
			name: "nats",
			newBroker: func() broker.Broker {
				return nats.NewBroker(broker.Addrs(env("BROKER_NATS_ADDR", "nats://127.0.0.1:14222")))
			},
		},
		{
			name: "redis",
			newBroker: func() broker.Broker {
				return redis.NewBroker(broker.Addrs(env("BROKER_REDIS_ADDR", "127.0.0.1:16379")))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runPublishSubscribe(t, tc.newBroker(), tc.prepare, tc.queueIsTopic, tc.subOpts...)
		})
	}
}

func runPublishSubscribe(t *testing.T, b broker.Broker, prepare func(*testing.T, string), queueIsTopic bool, subOpts ...broker.SubscribeOption) {
	t.Helper()
	if err := b.Connect(); err != nil {
		t.Fatalf("connect %s: %v", b.String(), err)
	}
	t.Cleanup(func() {
		if err := b.Disconnect(); err != nil {
			t.Errorf("disconnect %s: %v", b.String(), err)
		}
	})

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	topic := "broker-integration-" + b.String() + "-" + suffix
	queue := "broker-integration-group-" + suffix
	if queueIsTopic {
		queue = topic
	}
	if prepare != nil {
		prepare(t, topic)
	}
	opts := append([]broker.SubscribeOption{broker.WithQueue(queue)}, subOpts...)

	received := make(chan *broker.Message, 1)
	sub, err := b.Subscribe(topic, func(ctx context.Context, event broker.Event) error {
		select {
		case received <- event.Message():
		default:
		}
		return nil
	}, opts...)
	if err != nil {
		t.Fatalf("subscribe %s: %v", b.String(), err)
	}
	t.Cleanup(func() {
		if err := sub.Unsubscribe(); err != nil {
			t.Errorf("unsubscribe %s: %v", b.String(), err)
		}
	})

	// Give push-based consumers time to register before publishing. Kafka and
	// Redis tolerate publishing earlier, but NATS intentionally does not buffer.
	time.Sleep(500 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), integrationTimeout)
	defer cancel()
	want := &broker.Message{
		Header: map[string]string{"integration-id": suffix},
		Body:   []byte("docker-integration-" + b.String()),
	}
	if err := publishWithRetry(ctx, b, topic, want); err != nil {
		t.Fatalf("publish %s: %v", b.String(), err)
	}

	select {
	case got := <-received:
		if string(got.Body) != string(want.Body) {
			t.Fatalf("%s body: got %q, want %q", b.String(), got.Body, want.Body)
		}
		if got.Header["integration-id"] != suffix {
			t.Fatalf("%s header: got %q, want %q", b.String(), got.Header["integration-id"], suffix)
		}
	case <-ctx.Done():
		t.Fatalf("%s delivery timed out: %v", b.String(), ctx.Err())
	}
}

func prepareKafkaTopic(t *testing.T, topic string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), integrationTimeout)
	defer cancel()
	addr := env("BROKER_KAFKA_ADDR", "127.0.0.1:19092")
	conn, err := kafkago.DialContext(ctx, "tcp", addr)
	if err != nil {
		t.Fatalf("dial Kafka for topic creation: %v", err)
	}
	controller, err := conn.Controller()
	conn.Close()
	if err != nil {
		t.Fatalf("find Kafka controller: %v", err)
	}
	controllerAddr := fmt.Sprintf("%s:%d", controller.Host, controller.Port)
	controllerConn, err := kafkago.DialContext(ctx, "tcp", controllerAddr)
	if err != nil {
		t.Fatalf("dial Kafka controller: %v", err)
	}
	defer controllerConn.Close()
	if err := controllerConn.CreateTopics(kafkago.TopicConfig{Topic: topic, NumPartitions: 1, ReplicationFactor: 1}); err != nil {
		t.Fatalf("create Kafka topic: %v", err)
	}
}

func publishWithRetry(ctx context.Context, b broker.Broker, topic string, msg *broker.Message) error {
	var lastErr error
	for {
		lastErr = b.Publish(ctx, topic, msg)
		if lastErr == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w (last publish error: %v)", ctx.Err(), lastErr)
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
