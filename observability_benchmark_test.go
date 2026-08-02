//go:build !race

package broker

import (
	"context"
	"crypto/sha256"
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

func runRepresentativePublish(iterations int, enabled bool) time.Duration {
	var options []Option
	if enabled {
		options = append(options, WithObservability(EnableCategories(CategoryDiagnostics, CategoryMeasurements, CategoryCorrelation)))
	}
	candidate := NewNoopBroker(options...)
	_ = candidate.Connect()
	defer candidate.Disconnect()
	payload := make([]byte, 64*1024)
	message := &Message{Body: payload}
	started := time.Now()
	for index := 0; index < iterations; index++ {
		representativeDigest = sha256.Sum256(payload)
		_ = candidate.Publish(context.Background(), "representative", message)
	}
	return time.Since(started)
}

func TestObservabilityStandardOverheadBudget(t *testing.T) {
	const iterations = 2000
	// Warm caches before comparing alternating runs.
	runRepresentativePublish(100, false)
	runRepresentativePublish(100, true)
	var disabled, enabled time.Duration
	for trial := 0; trial < 3; trial++ {
		disabled += runRepresentativePublish(iterations, false)
		enabled += runRepresentativePublish(iterations, true)
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
			for index := 0; index < b.N; index++ {
				runRepresentativePublish(1, enabled)
			}
		})
	}
}
