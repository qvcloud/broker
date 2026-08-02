<!--
Sync Impact Report
Version change: 0.3.3 → 0.4.0
Modified principles:
  - I. Interface-Driven & Vendor-Agnostic → I. Stable, Vendor-Agnostic Public API
  - II. Reliability First (At-Least-Once Delivery) → II. Explicit and Reliable Delivery Semantics
  - III. Minimal Dependencies & Binary Efficiency → V. Dependency Isolation and Compatibility
  - IV. Standardized & Transparent Configuration → IV. Transparent Configuration, Security, and Observability
  - V. Behavioral Consistency (Ack/Nack/AutoAck) → merged into II. Explicit and Reliable Delivery Semantics
  - VI. Comprehensive Documentation & Examples → VI. Evidence-Based Quality and Documentation
Added sections:
  - III. Lifecycle, Context, and Concurrency Safety
  - Quality Gates
Removed sections: None
Follow-up TODOs: None
-->

# QvCloud Broker Constitution

## Core Principles

### I. Stable, Vendor-Agnostic Public API

The root `broker` package MUST define the common message, lifecycle, publishing, subscription,
and acknowledgment contracts without importing any adapter package. Business code using those
contracts MUST NOT require changes solely because one supported broker is replaced by another.
Adapter-specific capabilities MUST remain in the adapter package and MUST NOT leak vendor SDK
types into the common API. Any incompatible public API change MUST include migration guidance and
a SemVer major release.

Rationale: the library exists to isolate applications from vendor choice while preserving a small,
stable Go API.

### II. Explicit and Reliable Delivery Semantics

Every adapter MUST document and test how handler success, handler failure, `AutoAck`, `Ack()`, and
`Nack(requeue)` map to its native protocol. Framework-level automatic acknowledgment MUST occur
only after the handler returns success. `Nack(false)` MUST prevent intentional redelivery where the
underlying broker permits it; `Nack(true)` MUST retain or requeue the message where supported.
Differences that cannot be normalized MUST be explicit in `ADAPTER_EXTENSIONS.md`; they MUST NOT be
hidden behind a false cross-broker guarantee.

Reliable adapters MUST use durable broker primitives appropriate to the provider, such as Redis
Streams consumer groups rather than volatile Redis Pub/Sub. Changes to delivery, retry, ordering,
offset, queue, or dead-letter behavior MUST include tests for success and failure paths.

Rationale: a uniform method name is insufficient unless users can predict message loss and
redelivery behavior.

### III. Lifecycle, Context, and Concurrency Safety

Constructors and `Init` MUST be free of network I/O; connection and client creation belong in
`Connect`. Repeated `Connect`, `Disconnect`, and `Unsubscribe` calls MUST be safe according to their
documented lifecycle. `Disconnect` MUST cancel broker-owned work and release clients, readers,
subscriptions, channels, and goroutines. Blocking I/O and acknowledgment operations MUST propagate
the applicable caller, subscription, or message context.

Shared state accessed by publishing, subscription, reconnection, or shutdown goroutines MUST be
synchronized. The full unit suite MUST pass the Go race detector before merge. Retry loops MUST be
cancelable and bounded by backoff; they MUST NOT busy-loop after persistent failures.

Rationale: messaging adapters are inherently concurrent and failures most often occur during
reconnection and shutdown rather than the happy path.

### IV. Transparent Configuration, Security, and Observability

Common configuration MUST use functional options and consistently honor `Addrs`, `TLSConfig`,
`Logger`, `ClientID`, tracer, meter, and context wherever the adapter supports them. Adapter-specific
options MUST use typed keys and option tracking. An option that cannot be consumed MUST produce a
clear warning or a returned error; silent ignoring is prohibited. When generic and adapter-specific
options represent the same behavior, documented adapter-specific precedence MUST apply.

TLS verification MUST remain enabled by default. Secrets, credentials, access tokens, and private
keys MUST NOT be committed, logged, placed in examples, or embedded in test fixtures. Errors MUST
include adapter and operation context without exposing sensitive values. Trace and metric context
MUST be propagated across publish and handler boundaries when observability is configured.

Rationale: portable configuration is only useful when it is applied predictably and safely.

### V. Dependency Isolation and Compatibility

The root package and `middleware` MUST NOT import implementations from `brokers/`. Consumers MUST
only link the SDKs of adapters they explicitly import. New dependencies MUST be necessary for the
adapter or cross-cutting capability, use a license compatible with MIT distribution, and pass the
project vulnerability scan. Dependency additions and upgrades MUST preserve a tidy, verifiable
module graph.

The supported Go baseline is the version declared in `go.mod`, currently Go 1.24. Public behavior
MUST follow SemVer: incompatible contracts require MAJOR, backward-compatible features require
MINOR, and compatible fixes require PATCH releases.

Rationale: isolation limits binary size, transitive risk, and unintended compatibility changes.

### VI. Evidence-Based Quality and Documentation

Claims such as "supported", "production-ready", coverage percentages, performance improvements,
and delivery guarantees MUST be backed by reproducible tests or benchmarks. New adapters MUST
include unit tests, a runnable example, an entry in `ADAPTER_EXTENSIONS.md`, and matching Chinese
and English README documentation. Changes to common behavior MUST update every affected adapter's
tests and both README files when user-facing guidance changes.

Mock tests prove local logic but MUST NOT be presented as proof of production readiness. Adapters
that can run locally without credentials SHOULD have Docker-backed publish/subscribe coverage.
Cloud or externally hosted adapters MUST document the missing integration coverage and the required
pre-production validation.

Rationale: documentation is part of the public contract, and confidence must come from evidence
rather than labels.

## Technical Standards

- All Go source MUST be formatted with `gofmt` and pass `go vet`; repository lint rules are
  mandatory when configured in CI.
- Public types and exported functions MUST have useful Go documentation. Errors MUST be returned
  with adapter and operation context and MUST remain inspectable with standard error wrapping.
- Network methods MUST accept or derive a cancelable `context.Context`; subscription loops MUST
  terminate when their broker or subscriber context is canceled.
- Adapter state transitions MUST be synchronized. Tests MUST use synchronization primitives rather
  than timing-only sleeps when coordinating goroutines.
- TLS, logging, acknowledgment, reconnect, and shutdown behavior MUST be tested in proportion to
  their risk. Unsafe TLS examples such as `InsecureSkipVerify: true` are prohibited unless clearly
  isolated as a negative test.
- Benchmarks MUST record the Go version and relevant environment. README performance numbers MUST
  be updated or removed when they can no longer be reproduced.

## Quality Gates

Every pull request that changes Go code or module metadata MUST pass all applicable checks:

1. dependency verification, `gofmt`, `go vet`, and configured lint;
2. the complete unit test suite with no cache-dependent result;
3. `go test -race ./...`;
4. repository-wide statement coverage of at least 75%, with new and changed behavior directly
   exercised rather than relying only on unrelated aggregate coverage;
5. vulnerability scanning for reachable known vulnerabilities;
6. Docker integration tests for Kafka, RabbitMQ, NATS, and Redis when their adapters, common
   message behavior, acknowledgment behavior, or lifecycle code changes.

Changes to RocketMQ, SQS, or GCP Pub/Sub MUST include the strongest available mocked contract tests
and MUST state how the change was or will be verified against a real service. A failing gate may be
waived only through a documented, time-bounded exception approved by a maintainer; the exception
MUST identify the risk, owner, expiry date, and follow-up issue.

## Development Workflow

- Material features and behavioral changes MUST have a specification, implementation plan, and
  actionable tasks before implementation. Bug fixes MAY use a reduced artifact set when the defect,
  invariant, and regression test are explicit.
- Pull requests MUST describe user-visible behavior, compatibility impact, test evidence, and any
  adapter-specific limitations. Reviewers MUST evaluate compliance with this constitution.
- New adapters MUST implement the complete `broker.Broker` lifecycle, publish/subscribe behavior,
  event acknowledgment contract, option tracking, logging, and context cancellation before being
  listed as supported.
- The Chinese and English README files MUST remain semantically aligned. Examples MUST compile and
  MUST handle returned connection, subscription, and publishing errors.
- Releases MUST use SemVer tags and MUST NOT claim production readiness for an adapter without
  real-service evidence covering the target deployment characteristics.

## Governance

This constitution supersedes conflicting general development practices in this repository.
Specifications, plans, task lists, code reviews, and release decisions MUST comply with it.

Amendments require a pull request that explains the motivation, affected principles, migration or
enforcement impact, and proposed version bump. At least one maintainer MUST approve the amendment.
Governance versions follow SemVer: principle removal or incompatible redefinition is MAJOR; a new
principle, section, or materially stronger obligation is MINOR; clarification without changed
obligation is PATCH.

Each material pull request MUST include a constitution compliance review. Maintainers MUST review
the constitution at least once per minor release and whenever a repeated exception reveals that a
rule is impractical or incomplete. Approved exceptions MUST follow the documented process in
Quality Gates and do not amend the constitution by precedent.

**Version**: 0.4.0 | **Ratified**: 2025-05-18 | **Last Amended**: 2026-08-02
