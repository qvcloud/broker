//go:build !race

package broker

import (
	"context"
	"crypto/sha256"
	"os"
	"testing"
	"time"
)

var representativeDigest [32]byte

func BenchmarkObservabilityDisabledPublish(b *testing.B) {
	broker := NewNoopBroker()
	if err := broker.Connect(); err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = broker.Disconnect() })
	message := &Message{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := broker.Publish(context.Background(), "unused", message); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkObservabilityDiagnosticsPublish(b *testing.B) {
	broker := NewNoopBroker(WithObservability(EnableCategories(CategoryDiagnostics)))
	if err := broker.Connect(); err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = broker.Disconnect() })
	message := &Message{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := broker.Publish(context.Background(), "unused", message); err != nil {
			b.Fatal(err)
		}
	}
}

func runRepresentativePublish(iterations int, enabled bool, payload []byte) time.Duration {
	var options []Option
	if enabled {
		options = append(options, WithObservability(EnableCategories(CategoryDiagnostics, CategoryMeasurements, CategoryCorrelation)))
	}
	candidate := NewNoopBroker(options...)
	_ = candidate.Connect()
	defer candidate.Disconnect()
	message := &Message{Body: payload}
	started := time.Now()
	for index := 0; index < iterations; index++ {
		representativeDigest = sha256.Sum256(payload)
		_ = candidate.Publish(context.Background(), "representative", message)
	}
	return time.Since(started)
}

func TestObservabilityStandardOverheadBudget(t *testing.T) {
	if os.Getenv("BROKER_ENFORCE_PERFORMANCE") != "1" {
		t.Skip("performance budget is opt-in; coverage and race instrumentation distort timing")
	}
	const iterations = 200
	payload := make([]byte, 1024*1024)
	// Warm caches before comparing alternating runs.
	runRepresentativePublish(10, false, payload)
	runRepresentativePublish(10, true, payload)
	var disabled, enabled time.Duration
	for trial := 0; trial < 6; trial++ {
		// Alternate order to cancel CPU frequency and shared-runner drift.
		if trial%2 == 0 {
			disabled += runRepresentativePublish(iterations, false, payload)
			enabled += runRepresentativePublish(iterations, true, payload)
		} else {
			enabled += runRepresentativePublish(iterations, true, payload)
			disabled += runRepresentativePublish(iterations, false, payload)
		}
	}
	overhead := float64(enabled-disabled) / float64(disabled)
	if overhead >= 0.05 {
		t.Fatalf("standard observability overhead %.2f%% exceeds 5%% (disabled=%s enabled=%s)", overhead*100, disabled, enabled)
	}
	t.Logf("standard observability overhead %.2f%% (disabled=%s enabled=%s)", overhead*100, disabled, enabled)
}

func BenchmarkObservabilityStandardOverhead(b *testing.B) {
	for _, enabled := range []bool{false, true} {
		name := "disabled"
		if enabled {
			name = "standard-enabled"
		}
		b.Run(name, func(b *testing.B) {
			payload := make([]byte, 1024*1024)
			for index := 0; index < b.N; index++ {
				runRepresentativePublish(1, enabled, payload)
			}
		})
	}
}
