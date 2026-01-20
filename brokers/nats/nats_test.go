package nats

import (
	"testing"

	"github.com/qvcloud/broker"
	"github.com/stretchr/testify/assert"
)

func TestNewBroker(t *testing.T) {
	addr := "nats://localhost:4222"
	b := NewBroker(broker.Addrs(addr))

	assert.Equal(t, "nats", b.String())
	assert.Equal(t, addr, b.Address())
}

func TestInit(t *testing.T) {
	b := NewBroker()
	addr := "nats://user:pass@localhost:4222"
	err := b.Init(broker.Addrs(addr))
	assert.NoError(t, err)
	assert.Equal(t, addr, b.Address())
}
