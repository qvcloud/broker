# Validation Quickstart: Opt-in Observability and Diagnostics

This guide defines runnable acceptance checks for the implementation phase. It intentionally omits
implementation bodies; public behavior is defined in [public-api.md](contracts/public-api.md) and
signals in [signal-catalog.md](contracts/signal-catalog.md).

## Prerequisites

- Go 1.24 or later in the Go 1.24 release line
- Docker with Compose support for integration scenarios
- Repository checked out on `006-observability-diagnostics`

## 1. Run the observability example

Compile and view the switches described in [example.md](contracts/example.md):

```bash
go build ./examples/observability
go run ./examples/observability -help
```

Validate the master switch and representative independent categories:

```bash
go run ./examples/observability
go run ./examples/observability -observe -logs
go run ./examples/observability -observe -health -diagnostics
go run ./examples/observability -observe -metrics -correlation -events
go run ./examples/observability -observe -all -scenario=publish-failure
```

Expected results:

- Without `-observe`, normal publish/subscribe works with no new observability output.
- A category flag without `-observe` is rejected with a clear usage error.
- Enabled categories produce only their documented output.
- Failure scenarios print safe categorized diagnostics and never print the message body or secret
  fixture values.

## 2. Baseline and disabled-mode compatibility

```bash
go test -count=1 ./...
go test -race -count=1 ./...
```

Run the observability benchmarks for the unconfigured broker and compare against the stored same-
machine baseline. Expected results:

- Existing programs compile without implementing a new `Broker` method.
- No new log/event/metric output occurs.
- No dispatcher goroutine or active probe is started.
- Throughput delta is below 1% within the benchmark confidence policy.

## 3. Independent category checks

Run the table-driven root and noop contract tests:

```bash
go test -count=1 -run 'TestObservability|TestNoopObservability' ./...
```

The table must enable logging, health, diagnostics, measurements, correlation, and state events one
at a time. Expected results:

- Only the selected category emits or becomes queryable.
- A disabled category returns a safe empty/disabled result, never a panic.
- Runtime category changes are race-free while publish and health queries are in flight.

## 4. Failure diagnostics and redaction

```bash
go test -count=1 -run 'Test(Failure|.*Redact|.*Sensitive|MetricCardinality)' ./...
go test -fuzz=FuzzObservabilityRedaction -fuzztime=10s ./...
```

Use fixtures containing password-bearing URLs, tokens, signed query strings, private-key markers,
custom secret header names, 100,000 unique message IDs, and 100,000 destinations. Expected results:

- Records identify adapter, instance, operation, state, category, and time.
- No secret, message body, signed query, or raw vendor object appears.
- Metric attribute key/value sets remain bounded as specified in the signal catalog.

## 5. Health, lifecycle, and probe cancellation

```bash
go test -count=1 -run 'Test(Health|ConcurrentHealth|Probe|ObservabilityProbe)' ./...
```

Validate unknown → connecting → ready → degraded/reconnecting → ready → stopped. Query health
concurrently at every transition. Cancel an active probe and test an adapter with no active probe.
Expected results:

- Passive health performs no network call.
- Ready and connected are independently represented.
- Probe cancellation is prompt and unsupported probes match `ErrUnsupported`.
- Returned snapshots cannot mutate subsequent results.

## 6. Sink isolation and overflow

```bash
go test -race -count=1 -run 'Test(EventSink|EventOverflow|RuntimeOverflow|Concurrent)' ./...
```

Exercise a sink that blocks, returns errors, and panics under both overflow policies. Expected
results:

- Publish, handler, Ack, and Nack outcomes are unchanged.
- Queue memory remains within its configured bound.
- Drops are counted and sequence gaps are detectable.
- Disconnect completes within the documented shutdown budget.

## 7. Docker-backed adapter validation

```bash
make integration-test
```

The integration suite must cover Kafka, RabbitMQ, NATS, and Redis with diagnostics enabled, verify
publish/consume measurement outcomes and correlation where native metadata permits it, and exercise
a deterministic recovery scenario where supported by the compose environment. Expected results:

- State and operation semantics match the shared contracts across all four adapters.
- Message bodies remain absent from signals.
- Delivery and Ack/Nack behavior matches the pre-feature integration tests.

## 8. Cloud/RocketMQ contract validation

```bash
go test -count=1 ./brokers/rocketmq ./brokers/sqs ./brokers/pubsub
```

Mocked tests must cover native error mapping, lifecycle, metadata correlation, and unsupported
capability reporting. Before release, follow `ADAPTER_EXTENSIONS.md` to run the same scenarios
against a disposable RocketMQ service, AWS SQS queue, and GCP Pub/Sub project; attach sanitized
results to the release evidence.

## 9. Full repository gates

```bash
gofmt -l .
go vet ./...
make lint
go test -count=1 -coverprofile=coverage.out ./...
go test -race -count=1 ./...
go tool cover -func=coverage.out
```

Expected results: no formatting, vet, lint, unit, or race failures; repository statement coverage is
at least 75%; changed behavior is directly exercised. Also run the configured vulnerability scan
and verify Chinese/English README examples compile and remain semantically aligned.
