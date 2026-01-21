<!--
Version change: 0.3.2 → 0.3.3
Modified principles:
  - III. Minimal Dependencies & Loose Licensing → III. Minimal Dependencies & Binary Efficiency
Added sections: None
Removed sections: None
Templates requiring updates:
  - .specify/templates/plan-template.md (✅ updated)
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

### III. Minimal Dependencies & Binary Efficiency
The core library (`/`, `middleware/`) MUST NOT import any implementation packages from `brokers/`. This architecture ensures that users only pull in the specific dependencies and code for the adapters they explicitly import, preventing binary bloat. All code and adapters are released under the MIT License.

### IV. Standardized & Transparent Configuration
All brokers must follow the functional options pattern for configuration. Common options (Addrs, TLS, Logger, ClientID) must be uniformly respected and propagated to internal clients. Adapters MUST provide semantic transparency; for example, SQS MUST auto-resolve Queue URLs from names, and Kafka MUST propagate ClientIDs for observability.

### V. Behavioral Consistency (Ack/Nack/AutoAck)
All adapters must honor the `AutoAck` setting and provide consistent `Ack()` and `Nack(requeue)` behaviors. `Nack(false)` must ensure the message is not redelivered (e.g., by committing offset or deleting message), while `Nack(true)` must trigger a retry.

### VI. Comprehensive Documentation & Examples
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
- This PRs must verify compliance with the "Core Principles" during review.

**Version**: 0.3.3 | **Ratified**: 2025-05-18 | **Last Amended**: 2026-01-21
