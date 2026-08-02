package broker

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestEventOrderingAndDynamicToggle(t *testing.T) {
	sink := newRecordingSink()
	runtime, _ := NewRuntime("test", &ObservabilityConfig{Sink: sink, BufferCapacity: 8, OverflowPolicy: OverflowDropNewest, SinkTimeout: time.Second, ProbeTimeout: time.Second, CorrelationConflict: CorrelationPreserve})
	if sink.count() != 0 {
		t.Fatal("disabled runtime emitted")
	}
	if err := runtime.SetCategories(Categories(CategoryStateEvents)); err != nil {
		t.Fatal(err)
	}
	runtime.Transition(StateConnecting, false, false, nil)
	runtime.Transition(StateReady, true, true, nil)
	runtime.Close()
	sink.mu.Lock()
	events := append([]BrokerStateEvent(nil), sink.events...)
	sink.mu.Unlock()
	if len(events) != 2 || events[0].Sequence >= events[1].Sequence {
		t.Fatalf("events=%+v", events)
	}
	events[0].Attributes = map[string]string{"changed": "yes"}
	if runtime.Snapshot().Health.State != StateReady {
		t.Fatal("event mutation changed runtime")
	}
}

func TestEventSinkFailurePanicAndTimeout(t *testing.T) {
	tests := []struct {
		name string
		sink EventSink
	}{
		{"error", EventSinkFunc(func(context.Context, BrokerStateEvent) error { return errors.New("sink") })},
		{"panic", EventSinkFunc(func(context.Context, BrokerStateEvent) error { panic("sink") })},
		{"timeout", EventSinkFunc(func(ctx context.Context, _ BrokerStateEvent) error { <-ctx.Done(); return ctx.Err() })},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runtime, _ := NewRuntime("test", &ObservabilityConfig{Categories: Categories(CategoryStateEvents), Sink: tc.sink, BufferCapacity: 2, OverflowPolicy: OverflowDropNewest, SinkTimeout: time.Millisecond, ProbeTimeout: time.Second, CorrelationConflict: CorrelationPreserve})
			runtime.Transition(StateReady, true, true, nil)
			time.Sleep(5 * time.Millisecond)
			runtime.Close()
			if runtime.Snapshot().DroppedRecords == 0 {
				t.Fatal("sink failure was not counted")
			}
		})
	}
}

func TestEventOverflowPoliciesBoundCapacity(t *testing.T) {
	for _, policy := range []OverflowPolicy{OverflowDropNewest, OverflowDropOldest} {
		t.Run(string(policy), func(t *testing.T) {
			block := make(chan struct{})
			runtime, _ := NewRuntime("test", &ObservabilityConfig{Categories: Categories(CategoryStateEvents), Sink: EventSinkFunc(func(context.Context, BrokerStateEvent) error { <-block; return nil }), BufferCapacity: 2, OverflowPolicy: policy, SinkTimeout: time.Millisecond, ProbeTimeout: time.Second, CorrelationConflict: CorrelationPreserve})
			for i := 0; i < 1000; i++ {
				runtime.Transition(StateReady, true, true, nil)
			}
			if runtime.Snapshot().DroppedRecords == 0 {
				t.Fatal("failure storm did not overflow bounded queue")
			}
			runtime.Close()
			close(block)
		})
	}
}
