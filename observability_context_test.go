package broker

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func TestCorrelationInjectExtractAndPreserve(t *testing.T) {
	previous := otel.GetTextMapPropagator()
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() { otel.SetTextMapPropagator(previous) })
	runtime, err := NewRuntime("test", &ObservabilityConfig{Categories: Categories(CategoryCorrelation), BufferCapacity: 1, OverflowPolicy: OverflowDropNewest, SinkTimeout: 1, ProbeTimeout: 1, CorrelationConflict: CorrelationPreserve})
	if err != nil {
		t.Fatal(err)
	}
	spanContext := trace.NewSpanContext(trace.SpanContextConfig{TraceID: trace.TraceID{1}, SpanID: trace.SpanID{2}, TraceFlags: trace.FlagsSampled})
	ctx := trace.ContextWithSpanContext(context.Background(), spanContext)
	original := map[string]string{TraceParentHeader: "user-value", "business": "safe"}
	injected := runtime.InjectContext(ctx, original)
	if injected[TraceParentHeader] != "user-value" {
		t.Fatal("preserve policy overwrote user metadata")
	}
	if original["business"] != "safe" || len(original) != 2 {
		t.Fatal("injection mutated caller headers")
	}
	runtime.config.CorrelationConflict = CorrelationReplace
	replaced := runtime.InjectContext(ctx, original)
	if replaced[TraceParentHeader] == "user-value" {
		t.Fatal("replace policy did not inject context")
	}
	extracted := runtime.ExtractContext(context.Background(), replaced)
	if trace.SpanContextFromContext(extracted).TraceID() != spanContext.TraceID() {
		t.Fatal("trace ID did not round trip")
	}
}

func TestInjectCorrelationHelper(t *testing.T) {
	original := map[string]string{TraceParentHeader: "existing"}
	if got := InjectCorrelation(original, "new", "state", CorrelationPreserve); got[TraceParentHeader] != "existing" || got[TraceStateHeader] != "state" {
		t.Fatalf("unexpected preserve result: %v", got)
	}
	if got := InjectCorrelation(original, "new", "", CorrelationReplace); got[TraceParentHeader] != "new" {
		t.Fatalf("unexpected replace result: %v", got)
	}
}
