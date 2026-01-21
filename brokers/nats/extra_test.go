package nats

import (
	"context"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/qvcloud/broker"
	"github.com/stretchr/testify/assert"
)

func TestNATS_Event_Nack(t *testing.T) {
	mockMsg := &nats.Msg{}
	e := &natsEvent{
		topic:   "test",
		message: &broker.Message{Body: []byte("test")},
		nm:      mockMsg,
	}

	// Should return error for non-JetStream message
	assert.Error(t, e.Nack(true))
	assert.Error(t, e.Nack(false))
	assert.Equal(t, "test", e.Topic())
}

func TestNATS_Options_NilContext(t *testing.T) {
	o := &broker.Options{}
	WithMaxReconnect(5)(o)
	assert.NotNil(t, o.Context)

	o = &broker.Options{}
	WithReconnectWait(time.Second)(o)
	assert.NotNil(t, o.Context)

	po := &broker.PublishOptions{}
	WithReplyTo("reply")(po)
	assert.NotNil(t, po.Context)
}

func TestNATS_Subscriber_Unsubscribe(t *testing.T) {
	sub := &natsSubscriber{
		topic:  "test",
		cancel: func() {},
	}

	// Test Unsubscribe with nil sub
	err := sub.Unsubscribe()
	assert.NoError(t, err)

	// Test with non-nil sub (but nil conn)
	sub.sub = &nats.Subscription{}
	err = sub.Unsubscribe()
	assert.Error(t, err)

	// Test with mock sub? nats.Subscription is a struct, hard to mock.
	// But we can check s.cancel is called.
	called := false
	sub.cancel = func() { called = true }
	sub.sub = nil
	sub.Unsubscribe()
	assert.True(t, called)
}

func TestNATS_Subscriber_Methods(t *testing.T) {
	sub := &natsSubscriber{
		topic: "test",
		opts:  broker.SubscribeOptions{Queue: "test-queue"},
	}
	assert.Equal(t, "test", sub.Topic())
	assert.Equal(t, "test-queue", sub.Options().Queue)
}

func TestNATS_NewBroker_WithOptions(t *testing.T) {
	b := NewBroker(broker.Addrs("localhost:4222"))
	assert.NotNil(t, b)
	assert.Equal(t, "localhost:4222", b.Address())
}

func TestNATS_Connect_TLS(t *testing.T) {
	// Simple path check for TLS
	b := NewBroker(
		broker.Addrs("localhost:4222"),
		broker.TLSConfig(nil), // Just to trigger the if branch
	).(*natsBroker)

	b.newConn = func(addr string, opts ...nats.Option) (natsConn, error) {
		return &mockNatsConn{}, nil
	}

	err := b.Connect()
	assert.NoError(t, err)
}

func TestNATS_Subscribe_AutoAck(t *testing.T) {
	b := NewBroker().(*natsBroker)
	mock := &mockNatsConn{}
	b.conn = mock
	b.running = true

	var capturedHandler nats.MsgHandler
	mock.subscribeFunc = func(subj string, cb nats.MsgHandler) (*nats.Subscription, error) {
		capturedHandler = cb
		return &nats.Subscription{}, nil
	}

	handlerCalled := false
	handler := func(ctx context.Context, p broker.Event) error {
		handlerCalled = true
		return nil
	}

	sub, err := b.Subscribe("test", handler, broker.DisableAutoAck())
	assert.NoError(t, err)
	assert.NotNil(t, sub)

	capturedHandler(&nats.Msg{Data: []byte("data")})
	assert.True(t, handlerCalled)
}
