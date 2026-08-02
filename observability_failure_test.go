package broker

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestFailureRecordsCoverRequiredOperations(t *testing.T) {
	operations := []Operation{OperationConnect, OperationPublish, OperationReceive, OperationHandler, OperationAck, OperationNack, OperationRetry, OperationReconnect, OperationUnsubscribe, OperationDisconnect}
	for _, operation := range operations {
		t.Run(string(operation), func(t *testing.T) {
			runtime, err := NewRuntime("test", &ObservabilityConfig{Categories: Categories(CategoryDiagnostics), BufferCapacity: 2, OverflowPolicy: OverflowDropNewest, SinkTimeout: time.Second, ProbeTimeout: time.Second, CorrelationConflict: CorrelationPreserve})
			if err != nil {
				t.Fatal(err)
			}
			started := runtime.StartOperation(operation)
			runtime.Observe(operation, started, errors.New("password=secret failure"))
			snapshot := runtime.Snapshot()
			if snapshot.OperationTotals[string(operation)+":failure"] != 1 {
				t.Fatalf("missing total: %v", snapshot.OperationTotals)
			}
			if snapshot.LastFailure == nil || snapshot.LastFailure.Category == "" {
				t.Fatal("missing safe failure")
			}
			if strings.Contains(snapshot.LastFailure.Summary, "secret") || len(snapshot.LastFailure.Summary) > 515 {
				t.Fatalf("unsafe summary: %q", snapshot.LastFailure.Summary)
			}
			snapshot.LastFailure.Summary = "mutated"
			if runtime.Snapshot().LastFailure.Summary == "mutated" {
				t.Fatal("snapshot failure was mutable")
			}
		})
	}
}

func TestFailureEventAvailablePromptly(t *testing.T) {
	sink := newRecordingSink()
	runtime, err := NewRuntime("test", &ObservabilityConfig{Categories: Categories(CategoryLogging), Sink: sink, BufferCapacity: 4, OverflowPolicy: OverflowDropNewest, SinkTimeout: time.Second, ProbeTimeout: time.Second, CorrelationConflict: CorrelationPreserve})
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	runtime.Observe(OperationPublish, time.Now(), errors.New("publish failed"))
	select {
	case <-sink.wake:
	case <-time.After(time.Second):
		t.Fatal("failure event exceeded one-second availability target")
	}
}
