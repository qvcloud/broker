package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/qvcloud/broker"
)

type config struct {
	observe, all, logs, health, diagnostics, metrics, correlation, events, probe bool
	scenario                                                                     string
	timeout                                                                      time.Duration
}

type printSink struct{ writer io.Writer }

type lockedWriter struct {
	mu     sync.Mutex
	writer io.Writer
}

func (w *lockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.writer.Write(p)
}

func (s printSink) HandleObservabilityEvent(_ context.Context, event broker.BrokerStateEvent) error {
	_, err := fmt.Fprintf(s.writer, "event kind=%s operation=%s outcome=%s state=%s\n", event.Kind, event.Operation, event.Outcome, event.To)
	return err
}

type scenarioBroker struct {
	broker.Broker
	runtime  *broker.Runtime
	scenario string
}

func (b *scenarioBroker) Observability() broker.Observability { return b.runtime }

func (b *scenarioBroker) Connect() error {
	if b.scenario == "connect-failure" {
		err := errors.New("connection rejected token=[REDACTED]")
		b.runtime.Transition(broker.StateDegraded, false, false, err)
		return err
	}
	return b.Broker.Connect()
}

func (b *scenarioBroker) Publish(ctx context.Context, topic string, message *broker.Message, opts ...broker.PublishOption) error {
	if b.scenario == "publish-failure" {
		err := errors.New("demonstration publish failure")
		started := b.runtime.StartOperation(broker.OperationPublish)
		b.runtime.Observe(broker.OperationPublish, started, err)
		return err
	}
	return b.Broker.Publish(ctx, topic, message, opts...)
}

func parse(args []string) (config, error) {
	var cfg config
	set := flag.NewFlagSet("observability", flag.ContinueOnError)
	set.BoolVar(&cfg.observe, "observe", false, "enable opt-in observability")
	set.BoolVar(&cfg.all, "all", false, "enable every observability category")
	set.BoolVar(&cfg.logs, "logs", false, "enable structured failure logs")
	set.BoolVar(&cfg.health, "health", false, "print passive health")
	set.BoolVar(&cfg.diagnostics, "diagnostics", false, "print diagnostic snapshot")
	set.BoolVar(&cfg.metrics, "metrics", false, "enable operation measurements")
	set.BoolVar(&cfg.correlation, "correlation", false, "enable correlation")
	set.BoolVar(&cfg.events, "events", false, "enable state events")
	set.BoolVar(&cfg.probe, "probe", false, "run an active health probe")
	set.StringVar(&cfg.scenario, "scenario", "normal", "normal, connect-failure, publish-failure, or handler-failure")
	set.DurationVar(&cfg.timeout, "timeout", 5*time.Second, "scenario timeout")
	if err := set.Parse(args); err != nil {
		return cfg, err
	}
	selected := cfg.all || cfg.logs || cfg.health || cfg.diagnostics || cfg.metrics || cfg.correlation || cfg.events || cfg.probe
	if selected && !cfg.observe {
		return cfg, errors.New("observability category flags require -observe")
	}
	if cfg.probe && !cfg.health {
		return cfg, errors.New("-probe requires -health")
	}
	if cfg.timeout <= 0 {
		return cfg, errors.New("-timeout must be positive")
	}
	switch cfg.scenario {
	case "normal", "connect-failure", "publish-failure", "handler-failure":
	default:
		return cfg, fmt.Errorf("unknown scenario %q", cfg.scenario)
	}
	return cfg, nil
}

func categories(cfg config) []broker.Category {
	if cfg.all {
		return []broker.Category{broker.CategoryLogging, broker.CategoryHealth, broker.CategoryDiagnostics, broker.CategoryMeasurements, broker.CategoryCorrelation, broker.CategoryStateEvents}
	}
	var result []broker.Category
	if cfg.logs {
		result = append(result, broker.CategoryLogging)
	}
	if cfg.health {
		result = append(result, broker.CategoryHealth)
	}
	if cfg.diagnostics {
		result = append(result, broker.CategoryDiagnostics)
	}
	if cfg.metrics {
		result = append(result, broker.CategoryMeasurements)
	}
	if cfg.correlation {
		result = append(result, broker.CategoryCorrelation)
	}
	if cfg.events {
		result = append(result, broker.CategoryStateEvents)
	}
	return result
}

func run(args []string) error {
	return runTo(args, os.Stdout)
}

func runTo(args []string, output io.Writer) error {
	cfg, err := parse(args)
	if err != nil {
		return err
	}
	output = &lockedWriter{writer: output}
	var opts []broker.Option
	if cfg.observe {
		opts = append(opts, broker.WithObservability(broker.WithEventSink(printSink{writer: output}), broker.EnableCategories(categories(cfg)...)))
	}
	base := broker.NewNoopBroker(opts...)
	runtime := base.(broker.Observable).Observability().(*broker.Runtime)
	defer runtime.Close()
	b := broker.Broker(&scenarioBroker{Broker: base, runtime: runtime, scenario: cfg.scenario})
	observable := b.(broker.Observable).Observability()
	if cfg.all || cfg.health {
		_, _ = fmt.Fprintf(output, "health state=%s ready=%t\n", observable.Health().State, observable.Health().Ready)
	}
	if err := b.Connect(); err != nil {
		if cfg.all || cfg.diagnostics {
			printSnapshot(output, observable.Snapshot())
		}
		return err
	}
	defer func() { _ = b.Disconnect() }()
	ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
	defer cancel()
	if cfg.probe {
		status, err := observable.Probe(ctx)
		_, _ = fmt.Fprintf(output, "health probe state=%s ready=%t unsupported=%t\n", status.State, status.Ready, errors.Is(err, broker.ErrUnsupported))
	}
	handler := func(context.Context, broker.Event) error {
		if cfg.scenario == "handler-failure" {
			return errors.New("demonstration handler failure")
		}
		return nil
	}
	sub, err := b.Subscribe("observability.example", handler)
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()
	if err := b.Publish(ctx, "observability.example", &broker.Message{Body: []byte("not printed")}); err != nil {
		if cfg.all || cfg.diagnostics {
			printSnapshot(output, observable.Snapshot())
		}
		return err
	}
	if cfg.all || cfg.metrics {
		total := observable.Snapshot().OperationTotals["publish:success"]
		_, _ = fmt.Fprintf(output, "metric name=broker.operations operation=publish outcome=success value=%d\n", total)
	}
	if cfg.all || cfg.correlation {
		_, _ = fmt.Fprintln(output, "scenario correlation=enabled propagated=true")
	}
	if cfg.all || cfg.diagnostics {
		printSnapshot(output, observable.Snapshot())
	}
	_, _ = fmt.Fprintf(output, "scenario name=%s status=complete\n", cfg.scenario)
	return nil
}

func printSnapshot(output io.Writer, snap broker.DiagnosticSnapshot) {
	category := broker.ErrorCategory("")
	if snap.LastFailure != nil {
		category = snap.LastFailure.Category
	}
	_, _ = fmt.Fprintf(output, "snapshot broker=%s state=%s subscriptions=%d dropped=%d error_category=%s\n", snap.BrokerSystem, snap.Health.State, snap.SubscriptionCount, snap.DroppedRecords, category)
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, "scenario error:", err)
		os.Exit(2)
	}
}
