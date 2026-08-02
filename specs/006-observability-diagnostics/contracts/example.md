# Observability Example Contract

The implementation must add a runnable `examples/observability` program. It uses the in-memory
noop broker by default so users can explore the feature without Docker or credentials. The example
is instructional production-style code: it checks errors, uses bounded contexts, cleans up the
subscriber and broker, and never embeds secrets.

## Command-line switches

| Switch | Default | Behavior |
|---|---:|---|
| `-observe` | `false` | Master switch; without it all new observability remains disabled |
| `-all` | `false` | Enables every category; requires `-observe` |
| `-logs` | `false` | Enables structured failure logging |
| `-health` | `false` | Prints passive health before connect, after connect, and after disconnect |
| `-diagnostics` | `false` | Prints a safe diagnostic snapshot at scenario completion |
| `-metrics` | `false` | Collects and prints bounded operation measurements through an example exporter |
| `-correlation` | `false` | Injects and verifies correlation context without printing trace identifiers by default |
| `-events` | `false` | Prints lifecycle and failure state events through a bounded sink |
| `-probe` | `false` | Runs an explicitly timed active probe; requires `-health` |
| `-scenario` | `normal` | Selects `normal`, `connect-failure`, `publish-failure`, or `handler-failure` |
| `-timeout` | `5s` | Bounds connect, publish, handler completion, and active probe waiting |

Category switches supplied without `-observe`, `-probe` without `-health`, unknown scenarios, and
non-positive timeouts produce a concise usage error and non-zero exit. `-all` may be combined with
individual switches idempotently.

## Scenarios

### Normal

Connect, subscribe, publish one non-sensitive demonstration message, await handler completion,
unsubscribe, and disconnect. With observation disabled it prints only the example's normal status.
With categories enabled it demonstrates their respective output.

### Connection failure

Use an example-local broker test double or supported deterministic noop failure option to return a
categorized connection error containing a secret fixture. The visible log/event/snapshot must show
the adapter, operation, state, and safe category, while the secret fixture is absent.

### Publish failure

Connect successfully, then trigger a deterministic publication error. Display the categorized
failure and unchanged broker readiness. No retry is introduced by the example.

### Handler failure

Publish successfully and return a deterministic handler error. Demonstrate handler failure
measurement/event reporting and verify that acknowledgment behavior follows the configured broker
semantics rather than an observability decision.

## Output rules

- Use stable prefixes (`health`, `event`, `metric`, `snapshot`, `scenario`) so smoke tests can parse
  output without depending on timestamps.
- Never print message bodies, raw connection strings, reserved correlation values, or raw vendor
  objects as diagnostic output.
- Sort maps before printing so examples and tests are deterministic.
- Clearly label unsupported capabilities and errors matched with `errors.Is`.
- Human-readable output is for teaching and testing only; the library contract remains typed Go
  values rather than this text format.

## Required tests

- `go build ./examples/observability` succeeds.
- A table-driven test covers master switch off, each category alone, `-all`, invalid combinations,
  every scenario, and timeout cancellation.
- Output assertions verify category isolation and absence of known secret/body fixtures.
- The disabled invocation terminates with no dispatcher worker left running.
- README commands are executed or compile-checked in CI to prevent drift.
