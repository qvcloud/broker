package broker

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNoopBroker(t *testing.T) {
	b := NewNoopBroker()

	topic := "test"
	msgReceived := make(chan string, 1)

	sub, err := b.Subscribe(topic, func(ctx context.Context, event Event) error {
		msgReceived <- string(event.Message().Body)
		return nil
	})
	assert.NoError(t, err)
	defer sub.Unsubscribe()

	err = b.Publish(context.Background(), topic, &Message{
		Body: []byte("hello"),
	})
	assert.NoError(t, err)

	select {
	case msg := <-msgReceived:
		assert.Equal(t, "hello", msg)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for message")
	}
}
