//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/qvcloud/broker"
	"github.com/qvcloud/broker/brokers/kafka"
	"github.com/qvcloud/broker/brokers/nats"
	"github.com/qvcloud/broker/brokers/rabbitmq"
	"github.com/qvcloud/broker/brokers/redis"
	kafkago "github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const integrationTimeout = 30 * time.Second

func TestDockerBrokersPublishSubscribe(t *testing.T) {
	previousPropagator := otel.GetTextMapPropagator()
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() { otel.SetTextMapPropagator(previousPropagator) })
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
					observabilityOptions(),
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
					observabilityOptions(),
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
				return nats.NewBroker(broker.Addrs(env("BROKER_NATS_ADDR", "nats://127.0.0.1:14222")), observabilityOptions())
			},
		},
		{
			name: "redis",
			newBroker: func() broker.Broker {
				return redis.NewBroker(broker.Addrs(env("BROKER_REDIS_ADDR", "127.0.0.1:16379")), observabilityOptions())
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
	observable, ok := b.(broker.Observable)
	if !ok {
		t.Fatalf("%s does not expose observability", b.String())
	}
	if health := observable.Observability().Health(); !health.Connected || !health.Ready || health.State != broker.StateReady {
		t.Fatalf("%s health after connect: %+v", b.String(), health)
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
	opts := append([]broker.SubscribeOption{broker.WithQueue(queue), broker.DisableAutoAck()}, subOpts...)

	received := make(chan *broker.Message, 1)
	ackResult := make(chan error, 1)
	correlated := make(chan bool, 1)
	sub, err := b.Subscribe(topic, func(ctx context.Context, event broker.Event) error {
		ackResult <- event.Ack()
		correlated <- trace.SpanContextFromContext(ctx).IsValid()
		select {
		case received <- event.Message():
		default:
		}
		return nil
	}, opts...)
	if err != nil {
		t.Fatalf("subscribe %s: %v", b.String(), err)
	}
	var unsubscribeOnce sync.Once
	unsubscribe := func() {
		unsubscribeOnce.Do(func() {
			if err := sub.Unsubscribe(); err != nil {
				t.Errorf("unsubscribe %s: %v", b.String(), err)
			}
		})
	}
	t.Cleanup(unsubscribe)

	// Give push-based consumers time to register before publishing. Kafka and
	// Redis tolerate publishing earlier, but NATS intentionally does not buffer.
	time.Sleep(500 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), integrationTimeout)
	defer cancel()
	want := &broker.Message{
		Header: map[string]string{"integration-id": suffix},
		Body:   []byte("docker-integration-" + b.String()),
	}
	spanContext := trace.NewSpanContext(trace.SpanContextConfig{TraceID: trace.TraceID{1}, SpanID: trace.SpanID{2}, TraceFlags: trace.FlagsSampled, Remote: true})
	ctx = trace.ContextWithSpanContext(ctx, spanContext)
	if err := publishWithRetry(ctx, b, topic, want); err != nil {
		t.Fatalf("publish %s: %v", b.String(), err)
	}
	if total := observable.Observability().Snapshot().OperationTotals["publish:success"]; total == 0 {
		t.Fatalf("%s did not record successful publish", b.String())
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
	select {
	case <-ackResult:
	case <-ctx.Done():
		t.Fatalf("%s ack result timed out", b.String())
	}
	if !<-correlated {
		t.Fatalf("%s did not restore trace correlation", b.String())
	}
	ackTotals := observable.Observability().Snapshot().OperationTotals["ack:success"] + observable.Observability().Snapshot().OperationTotals["ack:failure"]
	if ackTotals == 0 {
		t.Fatalf("%s did not observe native Ack outcome", b.String())
	}

	// A handler failure and native Nack must be observable without replacing the
	// adapter's original error or requeue decision.
	failureTopic := topic + "-failure"
	if prepare != nil {
		prepare(t, failureTopic)
	}
	failureSeen := make(chan struct{}, 1)
	failureQueue := queue + "-failure"
	if queueIsTopic {
		failureQueue = failureTopic
	}
	failureSub, err := b.Subscribe(failureTopic, func(_ context.Context, event broker.Event) error {
		_ = event.Nack(false)
		select {
		case failureSeen <- struct{}{}:
		default:
		}
		return fmt.Errorf("integration handler failure")
	}, append([]broker.SubscribeOption{broker.WithQueue(failureQueue), broker.DisableAutoAck()}, subOpts...)...)
	if err != nil {
		t.Fatalf("failure subscribe %s: %v", b.String(), err)
	}
	time.Sleep(500 * time.Millisecond)
	if err := publishWithRetry(ctx, b, failureTopic, &broker.Message{Body: []byte("not-for-observability")}); err != nil {
		t.Fatalf("failure publish %s: %v", b.String(), err)
	}
	select {
	case <-failureSeen:
	case <-ctx.Done():
		t.Fatalf("%s handler failure timed out", b.String())
	}
	_ = failureSub.Unsubscribe()
	snapshot := observable.Observability().Snapshot()
	if snapshot.OperationTotals["handler:failure"] == 0 || snapshot.OperationTotals["nack:success"]+snapshot.OperationTotals["nack:failure"] == 0 {
		t.Fatalf("%s missing handler/Nack outcomes: %+v", b.String(), snapshot.OperationTotals)
	}

	// Deterministic manual recovery exercises stopped -> connecting -> ready.
	unsubscribe()
	if err := b.Disconnect(); err != nil {
		t.Fatalf("recovery disconnect %s: %v", b.String(), err)
	}
	if err := b.Connect(); err != nil {
		t.Fatalf("recovery connect %s: %v", b.String(), err)
	}
	if health := observable.Observability().Health(); !health.Ready || health.State != broker.StateReady {
		t.Fatalf("%s health after recovery: %+v", b.String(), health)
	}
	beforeDisabled := observable.Observability().Snapshot().OperationTotals
	if err := observable.Observability().SetCategories(0); err != nil {
		t.Fatalf("disable observability %s: %v", b.String(), err)
	}
	if err := publishWithRetry(ctx, b, topic+"-disabled", &broker.Message{Body: []byte("disabled-body")}); err != nil {
		t.Fatalf("disabled publish %s: %v", b.String(), err)
	}
	if after := observable.Observability().Snapshot().OperationTotals; !reflect.DeepEqual(after, beforeDisabled) {
		t.Fatalf("%s emitted diagnostics while disabled: before=%v after=%v", b.String(), beforeDisabled, after)
	}
}

func observabilityOptions() broker.Option {
	return broker.WithObservability(broker.EnableCategories(broker.CategoryHealth, broker.CategoryDiagnostics, broker.CategoryCorrelation))
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
