package obstest_test

import (
	"testing"

	"github.com/qvcloud/broker"
	"github.com/qvcloud/broker/brokers/kafka"
	"github.com/qvcloud/broker/brokers/nats"
	"github.com/qvcloud/broker/brokers/pubsub"
	"github.com/qvcloud/broker/brokers/rabbitmq"
	"github.com/qvcloud/broker/brokers/redis"
	"github.com/qvcloud/broker/brokers/rocketmq"
	"github.com/qvcloud/broker/brokers/sqs"
	"github.com/qvcloud/broker/internal/obstest"
)

func TestAdapterPassiveObservability(t *testing.T) {
	tests := []struct {
		name   string
		create func(...broker.Option) broker.Broker
	}{
		{"kafka", kafka.NewBroker}, {"rabbitmq", rabbitmq.NewBroker}, {"nats", nats.NewBroker}, {"redis", redis.NewBroker}, {"rocketmq", rocketmq.NewBroker}, {"sqs", sqs.NewBroker}, {"pubsub", pubsub.NewBroker},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			candidate := tc.create()
			obstest.CheckPassiveCapability(t, candidate, tc.name)
			obstest.CheckFailureAndSignalContract(t, candidate)
			obstest.CheckProbeContract(t, candidate)
			obstest.CheckInvalidConfiguration(t, tc.create(broker.WithObservability(broker.EnableCategories(broker.CategoryLogging))))
		})
	}
}
