package rocketmq

import (
	"testing"

	"github.com/qvcloud/broker"
	"github.com/stretchr/testify/assert"
)

func TestNewBroker(t *testing.T) {
	addr := "127.0.0.1:9876"
	b := NewBroker(broker.Addrs(addr))

	assert.Equal(t, "rocketmq", b.String())
	assert.Equal(t, addr, b.Address())
}

func TestInit(t *testing.T) {
	b := NewBroker()
	addr := "127.0.0.1:9876"
	
	err := b.Init(broker.Addrs(addr))
	assert.NoError(t, err)
	assert.Equal(t, addr, b.Address())
}
