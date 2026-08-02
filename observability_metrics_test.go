package broker

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestMetricCatalogSnapshotAccounting(t *testing.T) {
	runtime, _ := NewRuntime("test", &ObservabilityConfig{Categories: Categories(CategoryDiagnostics, CategoryMeasurements), BufferCapacity: 1, OverflowPolicy: OverflowDropNewest, SinkTimeout: time.Second, ProbeTimeout: time.Second, CorrelationConflict: CorrelationPreserve})
	for _, operation := range []Operation{OperationPublish, OperationReceive, OperationHandler, OperationAck, OperationNack, OperationRetry, OperationReconnect} {
		started := runtime.StartOperation(operation)
		runtime.Observe(operation, started, nil)
		started = runtime.StartOperation(operation)
		runtime.Observe(operation, started, errors.New("failed"))
	}
	runtime.AddSubscription(1)
	runtime.AddSubscription(-1)
	snapshot := runtime.Snapshot()
	for operation, value := range snapshot.InFlight {
		if value != 0 {
			t.Fatalf("inflight %s=%d", operation, value)
		}
	}
	if len(snapshot.OperationTotals) != 14 {
		t.Fatalf("totals=%v", snapshot.OperationTotals)
	}
}

func TestMetricCardinalityRemainsBounded(t *testing.T) {
	runtime, _ := NewRuntime("test", &ObservabilityConfig{Categories: Categories(CategoryDiagnostics), BufferCapacity: 1, OverflowPolicy: OverflowDropNewest, SinkTimeout: time.Second, ProbeTimeout: time.Second, CorrelationConflict: CorrelationPreserve})
	for i := 0; i < 100_000; i++ {
		_ = fmt.Sprintf("destination-%d", i)
		started := runtime.StartOperation(OperationPublish)
		runtime.Observe(OperationPublish, started, nil)
	}
	snapshot := runtime.Snapshot()
	if len(snapshot.OperationTotals) != 1 || snapshot.OperationTotals["publish:success"] != 100_000 {
		t.Fatalf("cardinality expanded: %v", snapshot.OperationTotals)
	}
}
