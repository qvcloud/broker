package obstest_test

import (
	"context"

	"github.com/qvcloud/broker"
)

type externalBroker struct{}

func (externalBroker) Init(...broker.Option) error { return nil }
func (externalBroker) Options() broker.Options     { return broker.Options{} }
func (externalBroker) Address() string             { return "external" }
func (externalBroker) Connect() error              { return nil }
func (externalBroker) Disconnect() error           { return nil }
func (externalBroker) Publish(context.Context, string, *broker.Message, ...broker.PublishOption) error {
	return nil
}
func (externalBroker) Subscribe(string, broker.Handler, ...broker.SubscribeOption) (broker.Subscriber, error) {
	return nil, nil
}
func (externalBroker) String() string { return "external" }

var _ broker.Broker = externalBroker{}
