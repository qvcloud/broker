# Implementation Plan: Unified MQ Broker Refinement

**Branch**: `001-unified-mq-broker` | **Date**: 2026-01-20 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/001-unified-mq-broker/spec.md`

## Summary

This plan refines the Unified MQ Broker framework to address architectural ambiguities identified during the audit. It focuses on implementing platform-specific Option extensions using a standardized Context-driven pattern, enforcing parameter precedence, establishing "Late Binding" for connection management, and ensuring smart serialization for message bodies.

## Technical Context

**Language/Version**: Go 1.24.0  
**Primary Dependencies**: RocketMQ, Kafka (segmentio/kafka-go), RabbitMQ (amqp091-go), NATS, AWS SDK (SQS), Google Cloud SDK (PubSub)  
**Storage**: N/A (Stateless message routing)  
**Testing**: go test (standard library), testify  
**Target Platform**: Go-based microservices  
**Project Type**: Library (Single project)  
**Performance Goals**: API overhead < 5% compared to native SDKs; Zero allocation path for raw []byte bodies  
**Constraints**: Functional Options (FR-003), Context-based extension pattern (FR-007), Mandatory Late Binding (FR-009)  
**Scale/Scope**: Supports 6+ MQ providers; Production-grade observability via OpenTelemetry  

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Implementation Strategy |
|-----------|--------|-------------------------|
| I. Library-First | PASS | The project is a standalone library with clean exported interfaces. |
| II. CLI Interface | N/A | Primarily a programmatic library, though examples/noop act as demonstrators. |
| III. Test-First | PASS | Every new Option and Adapter state transition must have a corresponding test case. |
| IV. Integration | PASS | Requires integration tests with localized MQ instances (Docker) to verify specific Options. |
| V. Observability | PASS | OpenTelemetry is integrated as a first-class citizen (FR-004 logic). |

## Project Structure

### Documentation (this feature)

```text
specs/001-unified-mq-broker/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
└── spec.md              # Original specification
```

### Source Code (repository root)

```text
.
├── broker.go          # Core interfaces
├── options.go         # Global options (ClientID, etc.)
├── message.go         # Common message model
├── middleware/        # OTEL and other intercepts
└── brokers/           # Adapter implementations
    ├── kafka/
    ├── rabbitmq/
    ├── rocketmq/
    ├── nats/
    ├── sqs/
    └── pubsub/
```

**Structure Decision**: Single project architecture as defined in the initial workspace layout. This maintains simplicity and easy discovery.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| N/A | | |
