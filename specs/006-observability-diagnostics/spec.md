# Feature Specification: Opt-in Observability and Diagnostics

**Feature Branch**: `006-observability-diagnostics`
**Created**: 2026-08-02
**Status**: Draft
**Input**: User description: "在当前项目上新增可观测、故障排查、日志等相关功能，供使用者按需开启"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Enable Only Needed Observability (Priority: P1)

As a broker library user, I want observability features to be disabled by default and independently
selectable so that I can gain operational insight without paying for or configuring features I do
not use.

**Why this priority**: Optional adoption preserves backward compatibility and prevents a monitoring
feature from becoming a mandatory runtime dependency.

**Independent Test**: Create brokers with no observability configuration, with logging only, and
with diagnostics only; verify each configuration exposes only the selected signals and preserves
normal message behavior.

**Acceptance Scenarios**:

1. **Given** an existing application with no observability configuration, **When** it upgrades the
   library, **Then** no new logs, diagnostics events, traces, or metrics are emitted.
2. **Given** a user enables one signal category, **When** broker operations occur, **Then** that
   category is available without requiring the other categories.
3. **Given** observability is enabled, **When** publishing and consuming messages, **Then** message
   delivery and acknowledgment outcomes remain unchanged.

---

### User Story 2 - Troubleshoot Broker Failures (Priority: P1)

As an operator, I want structured, correlated information about connection, publication,
consumption, acknowledgment, retry, and shutdown failures so that I can identify the affected
broker and operation without reproducing the incident locally.

**Why this priority**: Failure diagnosis is the primary user value and directly reduces recovery
time during production incidents.

**Independent Test**: Cause connection, publication, handler, acknowledgment, and reconnection
failures and verify that each produces a safe diagnostic record containing the broker, operation,
time, state, and causal error.

**Acceptance Scenarios**:

1. **Given** diagnostic logging is enabled, **When** a broker operation fails, **Then** the user
   receives a structured record identifying the broker type, operation, destination when safe,
   timestamp, state, and error category.
2. **Given** a connection repeatedly disconnects and recovers, **When** an operator inspects the
   diagnostics, **Then** the current state, last failure, last successful connection, and reconnect
   count are available.
3. **Given** configuration or message metadata contains credentials or secrets, **When** any
   diagnostic signal is emitted, **Then** sensitive values are absent or redacted.

---

### User Story 3 - Query Health and Runtime State (Priority: P2)

As a service owner, I want a consistent health and runtime-state view across adapters so that my
service can make readiness decisions and attach useful information to support reports.

**Why this priority**: A consistent state view makes the abstraction operationally useful without
forcing users to inspect vendor clients.

**Independent Test**: Query a broker before connection, after successful connection, during a
simulated failure, after recovery, and after shutdown; verify state transitions and timestamps.

**Acceptance Scenarios**:

1. **Given** a broker has not connected, **When** health is queried, **Then** it reports that it is
   not ready without initiating network activity.
2. **Given** a connected broker can perform its configured role, **When** health is queried, **Then**
   it reports ready and includes the most recent successful check time.
3. **Given** only part of a multi-address configuration is usable, **When** health is queried,
   **Then** it can report a degraded state rather than only healthy or failed.
4. **Given** diagnostics are disabled, **When** a caller queries unsupported diagnostic capability,
   **Then** the absence is explicit and does not cause a panic.

---

### User Story 4 - Monitor Performance and Message Flow (Priority: P2)

As an SRE, I want consistent counters, timings, and correlations for message operations so that I
can detect elevated failures, latency, retries, reconnects, and in-flight work across adapters.

**Why this priority**: Aggregate operational signals reveal degradation before individual errors
become an outage.

**Independent Test**: Run successful and failed publish/consume flows and verify that totals,
errors, durations, acknowledgments, retries, reconnects, and in-flight values reflect the observed
operations without exposing high-cardinality message data.

**Acceptance Scenarios**:

1. **Given** operational measurements are enabled, **When** messages are published and handled,
   **Then** users can observe operation totals, failures, durations, and in-flight work.
2. **Given** correlation is enabled, **When** a message flows from publication to handling, **Then**
   the related operations can be associated across the boundary where the broker preserves
   metadata.
3. **Given** a message has a unique identifier or arbitrary destination, **When** aggregate
   measurements are emitted, **Then** unbounded values are not used as aggregate dimensions.

---

### User Story 5 - Observe State Changes Programmatically (Priority: P3)

As an advanced user, I want optional notifications for important broker state changes so that I can
integrate the broker with custom incident handling and diagnostics workflows.

**Why this priority**: Programmatic observation enables custom integrations but is not required for
the core troubleshooting experience.

**Independent Test**: Register an observer, trigger connection and failure transitions, and verify
ordered notifications; then make the observer slow or faulty and verify broker operations continue.

**Acceptance Scenarios**:

1. **Given** an observer is enabled, **When** the broker connects, disconnects, begins reconnecting,
   recovers, or encounters an operation failure, **Then** the observer receives the corresponding
   event with safe context.
2. **Given** an observer is slow, fails, or panics, **When** an event is delivered, **Then** broker
   message processing and acknowledgment results are not changed.
3. **Given** no observer is configured, **When** state changes occur, **Then** no observer work is
   performed.

### Edge Cases

- Observability is enabled or disabled while broker operations are already in progress.
- Health is queried concurrently with Connect, Disconnect, Unsubscribe, or reconnection.
- A configured logger, exporter, or observer blocks, fails, or panics.
- An error wraps vendor-specific details containing credentials or signed addresses.
- A destination, client identifier, header, or error message has extremely high cardinality.
- The broker is connected but cannot publish, cannot consume, or has only some reachable addresses.
- Clock changes produce timestamps or durations that would otherwise appear inconsistent.
- Diagnostic buffers reach their configured capacity during a failure storm.
- An adapter cannot provide a requested signal or cannot propagate correlation metadata.
- Multiple brokers with the same adapter type run in one process and must remain distinguishable.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST keep all new observability categories disabled by default.
- **FR-002**: Users MUST be able to enable logging, health, diagnostic snapshots, operational
  measurements, correlation, and state-change observation independently.
- **FR-003**: Enabling or disabling an observability category MUST NOT change publishing,
  consumption, retry, acknowledgment, or shutdown semantics.
- **FR-004**: The system MUST expose a consistent broker state vocabulary covering at least unknown,
  connecting, ready, degraded, reconnecting, and stopped states.
- **FR-005**: Health information MUST distinguish connection state from readiness to perform the
  configured publish or consume role.
- **FR-006**: A health query MUST expose the check time, current state, readiness, last successful
  connection time, and most recent safe error summary when available.
- **FR-007**: A local state query MUST NOT initiate network activity unless the user explicitly
  requests an active health probe.
- **FR-008**: Active health probes MUST respect caller cancellation and a user-controlled timeout.
- **FR-009**: Diagnostic snapshots MUST include broker type, a non-secret instance identity,
  lifecycle state, connection timestamps, reconnect count, subscription count, in-flight work, and
  the most recent safe failure details when available.
- **FR-010**: Structured failure records MUST cover connection, publication, consumption, handler,
  acknowledgment, negative acknowledgment, retry, reconnection, unsubscription, and shutdown
  operations.
- **FR-011**: Every structured failure record MUST include a timestamp, broker instance, operation,
  severity, state, and error category; destination and consumer identity MAY be included only when
  safe and bounded.
- **FR-012**: The system MUST redact credentials, tokens, passwords, private keys, signed query
  parameters, and credential-bearing connection strings from every observability output.
- **FR-013**: Users MUST be able to configure additional header and field redaction without replacing
  the default protected-field set.
- **FR-014**: Operational measurements MUST cover totals, failures, and duration for publish,
  receive, handler, Ack, and Nack operations, plus retries, reconnects, and current in-flight work.
- **FR-015**: Aggregate measurements MUST NOT use message identifiers, arbitrary headers, full error
  text, or other unbounded values as dimensions.
- **FR-016**: Correlation MUST preserve an existing upstream context and associate publication with
  downstream handling when the selected broker preserves the required metadata.
- **FR-017**: Correlation metadata MUST use documented reserved fields and MUST NOT overwrite user
  metadata with the same name without an explicit conflict policy.
- **FR-018**: The system MUST expose unsupported observability capabilities explicitly rather than
  silently ignoring a requested category.
- **FR-019**: State-change observation MUST cover connect, disconnect, reconnect start, reconnect
  success, publish failure, consume failure, handler failure, and acknowledgment failure.
- **FR-020**: Slow, failing, or panicking observability consumers MUST be isolated from broker message
  processing and MUST NOT change acknowledgment outcomes.
- **FR-021**: Users MUST be able to bound memory used for buffered diagnostic events and select a
  documented overflow behavior.
- **FR-022**: Concurrent observability queries and emissions MUST return internally consistent data
  without races or exposing mutable internal state.
- **FR-023**: Disabled observability MUST avoid background workers, network calls, and externally
  visible side effects.
- **FR-024**: Existing applications and third-party broker implementations MUST continue to compile
  without implementing the new optional observability capabilities.
- **FR-025**: Chinese and English documentation MUST describe enablement, signal meanings, redaction,
  overhead expectations, unsupported capabilities, and troubleshooting examples consistently.
- **FR-026**: The repository MUST provide a runnable observability example with a master enable
  switch and independent logging, health, diagnostics, measurements, correlation, and state-event
  switches, including normal message flow and opt-in failure-troubleshooting scenarios.

### Key Entities

- **Observability Configuration**: The independently selected signal categories, probe behavior,
  redaction additions, buffer limits, and overflow preference for one broker instance.
- **Health Status**: A point-in-time view of lifecycle state, connection, readiness, check time,
  last successful connection, and safe error summary.
- **Diagnostic Snapshot**: A non-secret, immutable summary of broker identity, state, timestamps,
  reconnects, subscriptions, in-flight work, and recent failure context.
- **Operational Measurement**: A bounded-dimension count, duration, or current value associated with
  a broker operation and outcome.
- **Broker State Event**: A timestamped notification of a lifecycle transition or operation failure
  containing safe, bounded context.
- **Correlation Context**: Portable metadata that relates upstream publication to downstream
  processing without exposing business payloads.

## Scope

### In Scope

- Optional logging, local health and diagnostics, operational measurements, correlation, and broker
  state events across supported adapters.
- Passive state queries and explicitly requested active probes.
- Secure redaction and bounded-cardinality defaults.
- Consistent signal names and meanings with documented adapter limitations.
- Unit, race, failure-path, and applicable Docker-backed verification.

### Out of Scope

- A hosted monitoring service, dashboard, alert manager, log store, or trace store.
- Persisting diagnostic history after process exit.
- Automatically restarting applications or changing retry and delivery policies in response to
  observed failures.
- Broker administration such as creating topics, resizing partitions, or deleting queues.
- Reading or exposing message bodies for diagnostics.
- Guaranteeing correlation when a broker or external intermediary removes required metadata.

## Assumptions

- Existing logging, tracing, and measurement integrations remain usable and are consolidated rather
  than removed abruptly.
- Users provide any external collectors, exporters, storage, dashboards, or alerting systems.
- A safe local state snapshot is available synchronously; active probes are opt-in and cancelable.
- Default diagnostics never contain message bodies and use an allowlist for metadata exposure.
- Optional capability discovery preserves compatibility with third-party Broker implementations.
- When an adapter cannot provide a meaningful active health probe, it reports the limitation and
  still provides passive lifecycle state.

## Dependencies

- Existing broker lifecycle, Logger, tracing, measurement, Context, and option-tracking contracts.
- Adapter cooperation for accurate lifecycle state, reconnect counts, acknowledgment failures, and
  correlation transport.
- External observability systems only when users choose to export signals beyond the process.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Existing applications with no new configuration produce zero new observability output
  and complete the existing compatibility suite without source changes.
- **SC-002**: Users can enable any one supported signal category without configuring another signal
  category in 100% of documented examples.
- **SC-003**: For every supported adapter, users can determine current lifecycle state, readiness,
  last successful connection, and last safe failure reason through one consistent workflow.
- **SC-004**: Connection, publication, consumption, handler, acknowledgment, reconnection, and
  shutdown failures appear in enabled diagnostics within one second of being observed.
- **SC-005**: Automated security tests detect zero credential, token, private-key, signed-query, or
  message-body disclosures across all observability outputs.
- **SC-006**: With observability disabled, representative publish and consume throughput changes by
  less than 1% and introduces no persistent background activity.
- **SC-007**: With standard logging, diagnostics, measurements, and correlation enabled,
  representative publish and consume throughput changes by less than 5% at the same workload.
- **SC-008**: Concurrent health queries, state transitions, and signal emissions complete the full
  race and stress suites with zero detected data races or deadlocks.
- **SC-009**: A slow or failing observability consumer causes zero changed Ack/Nack outcomes and zero
  blocked broker operations beyond the configured observability time budget.
- **SC-010**: Aggregate measurement dimensions remain within the documented bounded set during a
  test containing at least 100,000 unique message identifiers and destinations.
- **SC-011**: At least 90% of troubleshooting test participants can identify the affected broker,
  failed operation, and most recent safe error from a diagnostic snapshot without vendor-client
  access.
- **SC-012**: The observability example compiles in CI and demonstrates disabled mode, each
  independently enabled category, a diagnostic snapshot, and at least connection, publish, and
  handler failure scenarios using documented command-line switches.
