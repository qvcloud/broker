package broker

import (
	"context"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

const (
	TraceParentHeader = "traceparent"
	TraceStateHeader  = "tracestate"
)

// InjectCorrelation copies reserved fields according to the configured policy.
func InjectCorrelation(headers map[string]string, traceParent, traceState string, policy CorrelationConflictPolicy) map[string]string {
	result := make(map[string]string, len(headers)+2)
	for key, value := range headers {
		result[key] = value
	}
	set := func(key, value string) {
		if value == "" {
			return
		}
		if _, exists := result[key]; !exists || policy == CorrelationReplace {
			result[key] = value
		}
	}
	set(TraceParentHeader, strings.TrimSpace(traceParent))
	set(TraceStateHeader, strings.TrimSpace(traceState))
	return result
}

// InjectContext returns copied headers with the configured OpenTelemetry
// propagation fields. The caller's map is never mutated.
func (r *Runtime) InjectContext(ctx context.Context, headers map[string]string) map[string]string {
	result := make(map[string]string, len(headers)+2)
	for key, value := range headers {
		result[key] = value
	}
	if r == nil || !r.Enabled(CategoryCorrelation) {
		return result
	}
	temporary := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, temporary)
	for key, value := range temporary {
		if _, exists := result[key]; !exists || r.config.CorrelationConflict == CorrelationReplace {
			result[key] = value
		}
	}
	return result
}

// ExtractContext returns a context linked to valid correlation headers.
func (r *Runtime) ExtractContext(ctx context.Context, headers map[string]string) context.Context {
	if r == nil || !r.Enabled(CategoryCorrelation) {
		return ctx
	}
	return otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(headers))
}
