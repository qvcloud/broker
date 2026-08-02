// Package obstest provides reusable observability assertions for broker adapters.
package obstest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/qvcloud/broker"
)

// CheckPassiveCapability verifies safe optional discovery without network I/O.
func CheckPassiveCapability(t *testing.T, candidate broker.Broker, system string) {
	t.Helper()
	observable, ok := candidate.(broker.Observable)
	if !ok {
		t.Fatalf("%s does not implement broker.Observable", system)
	}
	snapshot := observable.Observability().Snapshot()
	if snapshot.BrokerSystem != system || snapshot.Health.State != broker.StateUnknown || snapshot.Health.Ready {
		t.Fatalf("%s initial snapshot: %+v", system, snapshot)
	}
	if _, err := observable.Observability().Probe(context.Background()); !errors.Is(err, broker.ErrUnsupported) {
		t.Fatalf("%s probe error = %v", system, err)
	}
}

// CheckFailureAndSignalContract verifies bounded operation names, failures,
// measurements, and correlation helpers shared by every built-in adapter.
func CheckFailureAndSignalContract(t *testing.T, candidate broker.Broker) {
	t.Helper()
	observable := candidate.(broker.Observable).Observability()
	if err := observable.SetCategories(broker.Categories(broker.CategoryDiagnostics, broker.CategoryMeasurements, broker.CategoryCorrelation)); err != nil {
		t.Fatal(err)
	}
	runtime, ok := observable.(*broker.Runtime)
	if !ok {
		t.Fatal("built-in adapter did not expose shared runtime")
	}
	for _, operation := range []broker.Operation{broker.OperationConnect, broker.OperationPublish, broker.OperationReceive, broker.OperationHandler, broker.OperationAck, broker.OperationNack, broker.OperationRetry, broker.OperationReconnect, broker.OperationUnsubscribe, broker.OperationDisconnect} {
		started := runtime.StartOperation(operation)
		runtime.Observe(operation, started, errors.New("failure"))
	}
	snapshot := runtime.Snapshot()
	if len(snapshot.OperationTotals) != 10 || snapshot.LastFailure == nil {
		t.Fatalf("failure contract: %+v", snapshot)
	}
	headers := map[string]string{"business": "value"}
	copy := runtime.InjectContext(context.Background(), headers)
	if headers["business"] != "value" || copy["business"] != "value" {
		t.Fatal("correlation mutated metadata")
	}
	if snapshot.Health.CheckedAt.After(time.Now().Add(time.Second)) {
		t.Fatal("invalid snapshot time")
	}
}

// CheckProbeContract verifies unsupported discovery and cancellation semantics.
func CheckProbeContract(t *testing.T, candidate broker.Broker) {
	t.Helper()
	observable := candidate.(broker.Observable).Observability()
	if _, err := observable.Probe(context.Background()); !errors.Is(err, broker.ErrUnsupported) {
		t.Fatalf("unsupported probe = %v", err)
	}
	runtime, _ := broker.NewRuntime("probe", nil)
	runtime.Transition(broker.StateDegraded, true, false, errors.New("partial"))
	runtime.SetProbe(func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() })
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	status, err := runtime.Probe(ctx)
	if !errors.Is(err, context.DeadlineExceeded) || !status.Degraded || status.Ready {
		t.Fatalf("probe status=%+v err=%v", status, err)
	}
}

// CheckInvalidConfiguration verifies validation happens before adapter I/O.
func CheckInvalidConfiguration(t *testing.T, candidate broker.Broker) {
	t.Helper()
	if err := candidate.Connect(); !errors.Is(err, broker.ErrInvalidObservabilityConfig) {
		t.Fatalf("Connect error = %v", err)
	}
}
