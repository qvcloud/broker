package broker

import (
	"context"
	"testing"
)

func BenchmarkNoopBrokerPublish(b *testing.B) {
	broker := NewNoopBroker()
	topic := "bench"
	msg := &Message{Body: []byte("test")}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = broker.Publish(ctx, topic, msg)
	}
}

func BenchmarkDirectInterface(b *testing.B) {
	var broker Broker = NewNoopBroker()
	topic := "bench"
	msg := &Message{Body: []byte("test")}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = broker.Publish(ctx, topic, msg)
	}
}
