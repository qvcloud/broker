package pubsub

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/qvcloud/broker"
)

type mockPubSubProvider struct {
	publishFunc func(ctx context.Context, topic string, msg *pubsub.Message) *pubsub.PublishResult
	receiveFunc func(ctx context.Context, sub string, opts broker.Options, f func(context.Context, *pubsub.Message)) error
	closeFunc   func() error
}

func (m *mockPubSubProvider) Publish(ctx context.Context, topic string, msg *pubsub.Message) *pubsub.PublishResult {
	return m.publishFunc(ctx, topic, msg)
}

func (m *mockPubSubProvider) Receive(ctx context.Context, sub string, opts broker.Options, f func(context.Context, *pubsub.Message)) error {
	return m.receiveFunc(ctx, sub, opts, f)
}

func (m *mockPubSubProvider) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

type testLogger struct{ messages chan string }

func (l *testLogger) Log(v ...any) { l.messages <- fmt.Sprint(v...) }
func (l *testLogger) Logf(format string, v ...any) {
	l.messages <- fmt.Sprintf(format, v...)
}

func TestPubSubBroker(t *testing.T) {
	b := NewBroker()
	p := b.(*pubsubBroker)

	mockProvider := &mockPubSubProvider{
		publishFunc: func(ctx context.Context, topic string, msg *pubsub.Message) *pubsub.PublishResult {
			return nil
		},
	}

	p.provider = mockProvider
	p.running = true

	received := make(chan bool, 1)
	mockProvider.receiveFunc = func(ctx context.Context, sub string, opts broker.Options, f func(context.Context, *pubsub.Message)) error {
		f(ctx, &pubsub.Message{
			Data: []byte("hello"),
		})
		return nil
	}

	_, err := b.Subscribe("test-topic", func(ctx context.Context, event broker.Event) error {
		if string(event.Message().Body) != "hello" {
			t.Errorf("expected hello, got %s", string(event.Message().Body))
		}
		received <- true
		return nil
	}, broker.WithQueue("test-sub"))

	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}

	select {
	case <-received:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("timed out")
	}
}

func TestPubSubLifecycleAndErrors(t *testing.T) {
	t.Run("missing project", func(t *testing.T) {
		if err := NewBroker().Connect(); err == nil {
			t.Fatal("expected missing project error")
		}
	})

	t.Run("connect and disconnect", func(t *testing.T) {
		provider := &mockPubSubProvider{}
		b := NewBroker(broker.Addrs("project-id")).(*pubsubBroker)
		b.newProvider = func(ctx context.Context, projectID string) (pubsubProvider, error) {
			if projectID != "project-id" {
				t.Fatalf("unexpected project ID %q", projectID)
			}
			return provider, nil
		}
		if err := b.Connect(); err != nil {
			t.Fatal(err)
		}
		if !b.running || b.String() != "pubsub" || b.Address() != "project-id" {
			t.Fatal("broker did not enter connected state")
		}
		if err := b.Connect(); err != nil {
			t.Fatal("second connect should be idempotent:", err)
		}
		if err := b.Disconnect(); err != nil {
			t.Fatal(err)
		}
		if err := b.Disconnect(); err != nil {
			t.Fatal("second disconnect should be idempotent:", err)
		}
	})

	t.Run("provider errors", func(t *testing.T) {
		b := NewBroker(broker.Addrs("project-id")).(*pubsubBroker)
		b.newProvider = func(context.Context, string) (pubsubProvider, error) {
			return nil, errors.New("credentials unavailable")
		}
		if err := b.Connect(); err == nil {
			t.Fatal("expected provider creation error")
		}

		b.provider = &mockPubSubProvider{closeFunc: func() error { return errors.New("close failed") }}
		b.running = true
		if err := b.Disconnect(); err == nil {
			t.Fatal("expected close error")
		}
	})

	t.Run("subscribe validation and receive logging", func(t *testing.T) {
		logger := &testLogger{messages: make(chan string, 1)}
		provider := &mockPubSubProvider{receiveFunc: func(context.Context, string, broker.Options, func(context.Context, *pubsub.Message)) error {
			return errors.New("receive failed")
		}}
		b := NewBroker(broker.WithLogger(logger)).(*pubsubBroker)
		if _, err := b.Subscribe("topic", func(context.Context, broker.Event) error { return nil }); err == nil {
			t.Fatal("expected not connected error")
		}
		b.provider = provider
		if _, err := b.Subscribe("topic", func(context.Context, broker.Event) error { return nil }); err == nil {
			t.Fatal("expected missing subscription ID error")
		}
		sub, err := b.Subscribe("topic", func(context.Context, broker.Event) error { return nil }, broker.WithQueue("sub"))
		if err != nil {
			t.Fatal(err)
		}
		defer sub.Unsubscribe()
		select {
		case message := <-logger.messages:
			if !strings.Contains(message, "receive failed") {
				t.Fatalf("unexpected log message %q", message)
			}
		case <-time.After(time.Second):
			t.Fatal("receive error was not logged")
		}
	})
}

func TestPubSubOptionsAndEvent(t *testing.T) {
	b := NewBroker(
		WithMaxOutstandingMessages(10),
		WithMaxOutstandingBytes(2048),
		WithMaxExtension(time.Minute),
	)
	ctx := b.Options().Context
	if got := broker.GetTrackedValue(ctx, maxOutstandingMessagesKey{}); got != 10 {
		t.Fatalf("unexpected max messages: %v", got)
	}
	if got := broker.GetTrackedValue(ctx, maxOutstandingBytesKey{}); got != 2048 {
		t.Fatalf("unexpected max bytes: %v", got)
	}
	if got := broker.GetTrackedValue(ctx, maxExtensionKey{}); got != time.Minute {
		t.Fatalf("unexpected max extension: %v", got)
	}

	event := &pubsubEvent{topic: "topic", message: &broker.Message{Body: []byte("body")}, pm: &pubsub.Message{}}
	if event.Topic() != "topic" || string(event.Message().Body) != "body" || event.Error() != nil {
		t.Fatal("unexpected event metadata")
	}
}
