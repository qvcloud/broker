# Implementation Baseline

**Captured**: 2026-08-02
**Environment**: Go 1.24.0, darwin/arm64, Apple M4

## Pre-feature gates

- `go test -count=1 ./...`: pass
- `go test -race -count=1 ./...`: pass
- Existing `BenchmarkNoopBrokerPublish-10`: 26.90 ns/op, 80 B/op, 2 allocs/op

Commands use an isolated writable `GOCACHE` in the Codex sandbox. Results are a same-machine
reference only; performance success criteria require repeated statistical comparison after the
implementation is complete.

## Intentional capability limits

All adapters provide passive local health. Redis implements `PING`, RabbitMQ checks its connection
state, Kafka performs a cancelable TCP dial, and NATS uses `FlushWithContext`. RocketMQ, SQS, and
GCP Pub/Sub return `broker.ErrUnsupported` because their adapter abstractions do not expose a
low-impact cancelable probe. These limits are also listed in `ADAPTER_EXTENSIONS.md`.
