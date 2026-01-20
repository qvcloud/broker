package rabbitmq

import (
	"testing"

	"github.com/qvcloud/broker"
	"github.com/stretchr/testify/assert"
)

func TestNewBroker(t *testing.T) {
	addr := "amqp://guest:guest@localhost:5672/"
	b := NewBroker(broker.Addrs(addr))

	assert.Equal(t, "rabbitmq", b.String())
	assert.Equal(t, addr, b.Address())
}

func TestInit(t *testing.T) {
	b := NewBroker()
	addr := "amqp://guest:guest@user:pass@localhost:5672/"

	err := b.Init(broker.Addrs(addr))
	assert.NoError(t, err)
	assert.Equal(t, addr, b.Address())
}
