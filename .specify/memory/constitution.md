<!--
Version change: 0.0.0 → 0.3.0
Modified principles:
  - [PRINCIPLE_1_NAME] → I. Interface-Driven & Vendor-Agnostic
  - [PRINCIPLE_2_NAME] → II. Reliability First (At-Least-Once Delivery)
  - [PRINCIPLE_3_NAME] → III. Minimal Dependencies & Loose Licensing
  - [PRINCIPLE_4_NAME] → IV. Standardized Configuration (Options Pattern)
  - [PRINCIPLE_5_NAME] → V. Comprehensive Documentation & Examples
Added sections: Technical Standards, Development Workflow
Templates requiring updates:
  - .specify/templates/plan-template.md (✅ aligned)
  - .specify/templates/spec-template.md (✅ aligned)
  - .specify/templates/tasks-template.md (✅ aligned)
Follow-up TODOs: None
-->

# Unified MQ Broker for Go Constitution

## Core Principles

### I. Interface-Driven & Vendor-Agnostic
The broker must provide a unified, concise API (`Broker`, `Publisher`, `Subscriber`) that decouples business logic from specific MQ implementations. Adding a new driver (e.g., Redis, Kafka) must not require changes to consumer code.

### II. Reliability First (At-Least-Once Delivery)
Implementations must prioritize message delivery reliability. For Redis, this means using Streams with PEL (Pending Entries List) and Consumer Groups instead of volatile Pub/Sub. All adapters must include address validation and error handling to prevent common pitfalls (like "no NamesrvAddrs" panics).

### III. Minimal Dependencies & Loose Licensing
The core library should have minimal external dependencies. All code and adapters are released under the MIT License to ensure maximum reuse and minimum friction for users.

### IV. Standardized Configuration (Options Pattern)
All brokers must follow the functional options pattern for configuration. Common options (Addrs, TLS, Logger, ClientID) must be uniformly respected. Custom options (e.g., `WithPassword`, `WithMaxLen`) should be implemented via `broker.Option` with tracked values to maintain interface consistency.

### V. Comprehensive Documentation & Examples
Every new broker implementation must be accompanied by a functional example in the `examples/` directory that demonstrates basic connection, publishing, and subscription. Documentation must be maintained in both Chinese (`README.md`) and English (`README_EN.md`).

## Technical Standards

- Use Go 1.21+ features and idiomatic patterns.
- Ensure thread-safety for all `Connect`, `Disconnect`, and internal state changes.
- Provide clear, wrapped error messages with context (e.g., `redis: address is required`).
- Support context propagation throughout all I/O operations.

## Development Workflow

- Semantic Versioning (SemVer) is strictly followed for all releases.
- All code changes must pass `go mod tidy` and local testing.
- New adapters must be added to both `README` files under "Supported Drivers".

## Governance

- This constitution supersedes all other general development practices for this project.
- Amendments to principles require a MINOR version bump.
- All PRs must verify compliance with the "Core Principles" during review.

**Version**: 0.3.0 | **Ratified**: 2025-05-18 | **Last Amended**: 2025-05-18
