# Implementation Plan: Unified MQ Broker for Go

**Branch**: `main` | **Date**: 2026-01-20 | **Spec**: [spec.md](spec.md)

## Summary
Develop a vendor-neutral messaging abstraction for Go. The project follows an interface-driven design, allowing multiple MQ implementations (drivers) to be plugged in without changing higher-level application code.

## Technical Context

**Language/Version**: Go 1.24+  
**Primary Dependencies**: `github.com/google/uuid`, `github.com/stretchr/testify`  
**Storage**: N/A (Abstraction layer)  
**Testing**: `go test` with `testify/assert`  
**Project Type**: Library / Framework  
**Performance Goals**: < 5% overhead vs native SDKs  

## Constitution Check

- **I. Interface-Driven**: ✅ Core interface defined in `broker.go`.
- **II. Test-First**: ✅ `noop_broker_test.go` implemented first.
- **III. High Performance**: ✅ Minimal logic in abstraction layer.
- **IV. Observability**: ⚠️ Planned for User Story 4.

## Project Structure

```text
/
├── broker.go          # Core Interface
├── options.go         # Common Config
├── json.go            # Default Codec
├── noop_broker.go     # Reference Implementation
├── brokers/           # Adapters
│   ├── rocketmq/
│   ├── kafka/
│   ├── nats/
│   └── ...
└── examples/          # Usage Examples
```

## Structure Decision
Single project repository with subpackages for each broker implementation to keep dependencies isolated where possible.
