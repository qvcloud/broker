package broker

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestHealthLifecycleAndReadiness(t *testing.T) {
	runtime, err := NewRuntime("test", nil)
	if err != nil {
		t.Fatal(err)
	}
	checks := []struct {
		state            LifecycleState
		connected, ready bool
	}{{StateConnecting, false, false}, {StateDegraded, true, false}, {StateReady, true, true}, {StateReconnecting, false, false}, {StateStopped, false, false}}
	for _, check := range checks {
		runtime.Transition(check.state, check.connected, check.ready, nil)
		health := runtime.Health()
		if health.State != check.state || health.Connected != check.connected || health.Ready != check.ready {
			t.Fatalf("health = %+v", health)
		}
	}
}

func TestConcurrentHealthAndTransitions(t *testing.T) {
	runtime, _ := NewRuntime("test", nil)
	var group sync.WaitGroup
	for i := 0; i < 16; i++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			for j := 0; j < 1000; j++ {
				if index%2 == 0 {
					runtime.Transition(StateReady, true, true, nil)
				} else {
					_ = runtime.Health()
					_ = runtime.Snapshot()
				}
			}
		}(i)
	}
	group.Wait()
}

func TestProbeTimeoutAndDegradedResult(t *testing.T) {
	runtime, _ := NewRuntime("test", &ObservabilityConfig{BufferCapacity: 1, OverflowPolicy: OverflowDropNewest, SinkTimeout: time.Second, ProbeTimeout: time.Millisecond, CorrelationConflict: CorrelationPreserve})
	runtime.Transition(StateDegraded, true, false, errors.New("partial"))
	runtime.SetProbe(func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() })
	status, err := runtime.Probe(context.Background())
	if !errors.Is(err, context.DeadlineExceeded) || !status.Degraded || status.Ready {
		t.Fatalf("status=%+v err=%v", status, err)
	}
}
