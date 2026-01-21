package nats

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/qvcloud/broker"
	"github.com/stretchr/testify/assert"
)

// mockNatsConn implements natsConn interface
type mockNatsConn struct {
	publishFunc        func(m *nats.Msg) error
	subscribeFunc      func(subj string, cb nats.MsgHandler) (*nats.Subscription, error)
	queueSubscribeFunc func(subj, queue string, cb nats.MsgHandler) (*nats.Subscription, error)
	closeCalled        bool
}

func (m *mockNatsConn) PublishMsg(msg *nats.Msg) error {
	if m.publishFunc != nil {
		return m.publishFunc(msg)
	}
	return nil
}

func (m *mockNatsConn) Subscribe(subj string, cb nats.MsgHandler) (*nats.Subscription, error) {
	if m.subscribeFunc != nil {
		return m.subscribeFunc(subj, cb)
	}
	return &nats.Subscription{}, nil
}

func (m *mockNatsConn) QueueSubscribe(subj, queue string, cb nats.MsgHandler) (*nats.Subscription, error) {
	if m.queueSubscribeFunc != nil {
		return m.queueSubscribeFunc(subj, queue, cb)
	}
	return &nats.Subscription{}, nil
}

func (m *mockNatsConn) Close() {
	m.closeCalled = true
}

func TestNATS_Basic(t *testing.T) {
	b := NewBroker().(*natsBroker)
	mock := &mockNatsConn{}

	// Test Init/Options
	err := b.Init(broker.Addrs("nats://localhost:4222"), broker.ClientID("test-client"))
	assert.NoError(t, err)

	// Test ID/String
	assert.Equal(t, "nats", b.String())
	assert.Equal(t, "test-client", b.Options().ClientID)

	// Test Connect with mocked factory
	b.newConn = func(addr string, opts ...nats.Option) (natsConn, error) {
		return mock, nil
	}

	err = b.Connect()
	assert.NoError(t, err)
	assert.True(t, b.running)

	// Test Disconnect
	err = b.Disconnect()
	assert.NoError(t, err)
	assert.True(t, mock.closeCalled)
	assert.False(t, b.running)
}

func TestNATS_Publish(t *testing.T) {
	b := NewBroker().(*natsBroker)
	mock := &mockNatsConn{}
	b.conn = mock
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		mock.publishFunc = func(m *nats.Msg) error {
			assert.Equal(t, "test-topic", m.Subject)
			assert.Equal(t, []byte("hello"), m.Data)
			return nil
		}
		err := b.Publish(ctx, "test-topic", &broker.Message{Body: []byte("hello")})
		assert.NoError(t, err)
	})

	t.Run("Error", func(t *testing.T) {
		mock.publishFunc = func(m *nats.Msg) error {
			return errors.New("nats error")
		}
		err := b.Publish(ctx, "test-topic", &broker.Message{Body: []byte("fail")})
		assert.Error(t, err)
	})

	t.Run("Options", func(t *testing.T) {
		trackCtx := broker.TrackOptions(context.Background())
		mock.publishFunc = func(m *nats.Msg) error {
			assert.Equal(t, "val", m.Header.Get("Custom-Header"))
			return nil
		}
		err := b.Publish(trackCtx, "test-topic", &broker.Message{
			Body:   []byte("msg"),
			Header: map[string]string{"Custom-Header": "val"},
		}, broker.PublishContext(trackCtx))
		assert.NoError(t, err)
	})
}

func TestNATS_Subscribe(t *testing.T) {
	b := NewBroker().(*natsBroker)
	mock := &mockNatsConn{}
	b.conn = mock

	t.Run("Simple_Subscribe", func(t *testing.T) {
		var capturedHandler nats.MsgHandler
		mock.subscribeFunc = func(subj string, cb nats.MsgHandler) (*nats.Subscription, error) {
			assert.Equal(t, "test-topic", subj)
			capturedHandler = cb
			return &nats.Subscription{}, nil
		}

		msgReceived := make(chan struct{})
		handler := func(ctx context.Context, p broker.Event) error {
			assert.Equal(t, "test-topic", p.Topic())
			assert.Equal(t, []byte("data"), p.Message().Body)
			close(msgReceived)
			return nil
		}

		sub, err := b.Subscribe("test-topic", handler)
		assert.NoError(t, err)
		assert.NotNil(t, sub)

		// Simulate NATS message
		capturedHandler(&nats.Msg{
			Subject: "test-topic",
			Data:    []byte("data"),
		})

		select {
		case <-msgReceived:
		case <-time.After(time.Second):
			t.Fatal("Timeout waiting for message")
		}
	})

	t.Run("Queue_Subscribe", func(t *testing.T) {
		mock.queueSubscribeFunc = func(subj, queue string, cb nats.MsgHandler) (*nats.Subscription, error) {
			assert.Equal(t, "test-topic", subj)
			assert.Equal(t, "test-group", queue)
			return &nats.Subscription{}, nil
		}

		sub, err := b.Subscribe("test-topic", func(ctx context.Context, p broker.Event) error { return nil }, broker.WithQueue("test-group"))
		assert.NoError(t, err)
		assert.NotNil(t, sub)
	})

	t.Run("Subscribe_Error", func(t *testing.T) {
		mock.subscribeFunc = func(subj string, cb nats.MsgHandler) (*nats.Subscription, error) {
			return nil, errors.New("subscribe failed")
		}
		_, err := b.Subscribe("fail", func(ctx context.Context, p broker.Event) error { return nil })
		assert.Error(t, err)
	})
}

func TestNATS_Init_Failure(t *testing.T) {
	b := NewBroker()

	// Test Connect failure (real connection will fail if no NATS server)
	// We replace newConn to force failure
	originalNewConn := b.(*natsBroker).newConn
	defer func() { b.(*natsBroker).newConn = originalNewConn }()

	b.(*natsBroker).newConn = func(addr string, opts ...nats.Option) (natsConn, error) {
		return nil, errors.New("connection failed")
	}

	err := b.Connect()
	assert.Error(t, err)
}

func TestNATS_Event_Ack(t *testing.T) {
	e := &natsEvent{
		topic:   "test",
		message: &broker.Message{Body: []byte("test")},
		// We don't call Ack here because it might panic if nm is nil
	}
	assert.Equal(t, "test", e.Topic())
	assert.Equal(t, []byte("test"), e.Message().Body)
	assert.Nil(t, e.Error())
}

func TestNATS_More(t *testing.T) {
	t.Run("Address", func(t *testing.T) {
		b := NewBroker(broker.Addrs("addr1", "addr2"))
		assert.Equal(t, "addr1", b.Address())
		b2 := NewBroker()
		assert.Equal(t, "", b2.Address())
	})

	t.Run("Disconnect_NotRunning", func(t *testing.T) {
		b := NewBroker().(*natsBroker)
		assert.NoError(t, b.Disconnect())
	})

	t.Run("Connect_NoAddrs", func(t *testing.T) {
		b := NewBroker().(*natsBroker)
		assert.Error(t, b.Connect())
	})

	t.Run("Options_Context", func(t *testing.T) {
		b := NewBroker(
			broker.Addrs("localhost:4222"),
			WithMaxReconnect(10),
			WithReconnectWait(time.Second),
		).(*natsBroker)

		mock := &mockNatsConn{}
		b.newConn = func(addr string, opts ...nats.Option) (natsConn, error) {
			return mock, nil
		}

		err := b.Connect()
		assert.NoError(t, err)
	})

	t.Run("Publish_ReplyTo", func(t *testing.T) {
		b := NewBroker().(*natsBroker)
		mock := &mockNatsConn{}
		b.conn = mock
		b.running = true

		mock.publishFunc = func(m *nats.Msg) error {
			assert.Equal(t, "reply-topic", m.Reply)
			return nil
		}

		err := b.Publish(context.Background(), "test", &broker.Message{Body: []byte("hi")}, WithReplyTo("reply-topic"))
		assert.NoError(t, err)
	})
}
