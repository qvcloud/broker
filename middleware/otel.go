package middleware

import (
	"context"

	"github.com/qvcloud/broker"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// OtelHandler wraps a broker handler with OpenTelemetry tracing.
func OtelHandler(h broker.Handler, opts ...Option) broker.Handler {
	options := options{
		tracer: otel.Tracer("github.com/qvcloud/broker"),
	}
	for _, o := range opts {
		o(&options)
	}

	return func(ctx context.Context, event broker.Event) error {
		ctx, span := options.tracer.Start(ctx, "broker.handle",
			trace.WithSpanKind(trace.SpanKindConsumer),
			trace.WithAttributes(
				attribute.String("messaging.system", "broker"),
				attribute.String("messaging.destination", event.Topic()),
				attribute.String("messaging.operation", "process"),
			),
		)
		defer span.End()

		err := h(ctx, event)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		return err
	}
}

type options struct {
	tracer trace.Tracer
}

type Option func(*options)

func WithTracer(t trace.Tracer) Option {
	return func(o *options) {
		o.tracer = t
	}
}
