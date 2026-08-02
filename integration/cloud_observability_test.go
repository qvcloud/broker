package integration_test

import (
	"testing"

	"github.com/qvcloud/broker"
	"github.com/qvcloud/broker/brokers/pubsub"
	"github.com/qvcloud/broker/brokers/rocketmq"
	"github.com/qvcloud/broker/brokers/sqs"
	"github.com/qvcloud/broker/internal/obstest"
)

func TestCloudAdapterObservabilityContracts(t *testing.T) {
	tests := []struct {
		name   string
		system string
		new    func(...broker.Option) broker.Broker
	}{
		{name: "rocketmq", system: "rocketmq", new: rocketmq.NewBroker},
		{name: "sqs", system: "sqs", new: sqs.NewBroker},
		{name: "pubsub", system: "pubsub", new: pubsub.NewBroker},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := test.new()
			obstest.CheckPassiveCapability(t, candidate, test.system)
			obstest.CheckFailureAndSignalContract(t, candidate)
		})
	}
}
