package pubsub

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/qvcloud/broker"
)

type mockPubSubProvider struct {
	publishFunc func(ctx context.Context, topic string, msg *pubsub.Message) *pubsub.PublishResult
	receiveFunc func(ctx context.Context, sub string, opts broker.Options, f func(context.Context, *pubsub.Message)) error
}

func (m *mockPubSubProvider) Publish(ctx context.Context, topic string, msg *pubsub.Message) *pubsub.PublishResult {
	return m.publishFunc(ctx, topic, msg)
}

func (m *mockPubSubProvider) Receive(ctx context.Context, sub string, opts broker.Options, f func(context.Context, *pubsub.Message)) error {
	return m.receiveFunc(ctx, sub, opts, f)
}

func (m *mockPubSubProvider) Close() error {
	return nil
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
