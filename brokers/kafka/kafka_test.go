package kafka

import (
	"testing"

	"github.com/qvcloud/broker"
	"github.com/stretchr/testify/assert"
)

func TestNewBroker(t *testing.T) {
	addr := "127.0.0.1:9092"
	b := NewBroker(broker.Addrs(addr))

	assert.Equal(t, "kafka", b.String())
	assert.Equal(t, addr, b.Address())
}
