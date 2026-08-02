# Observability Example

This example uses the in-memory noop broker, so it needs no Docker service or credentials. All new
observability is off unless the master `-observe` switch is present.

```bash
go run ./examples/observability
go run ./examples/observability -observe -logs
go run ./examples/observability -observe -health -diagnostics
go run ./examples/observability -observe -metrics -correlation -events
go run ./examples/observability -observe -all -scenario=handler-failure
```

| Switch | Purpose |
|---|---|
| `-observe` | Master opt-in switch |
| `-all` | Enable every category |
| `-logs` | Structured failure records |
| `-health` | Passive local health |
| `-diagnostics` | Safe diagnostic snapshot |
| `-metrics` | Bounded operation measurements |
| `-correlation` | Publish-to-handler context propagation |
| `-events` | Lifecycle and failure events |
| `-probe` | Explicit active probe; requires `-health` |
| `-scenario` | `normal`, `connect-failure`, `publish-failure`, or `handler-failure` |
| `-timeout` | Bound scenario and probe waiting; default `5s` |

Category flags without `-observe` are rejected. Diagnostic output never prints message bodies, raw
connection strings, or correlation values. Synthetic failures are deterministic teaching aids and
do not retry or alter acknowledgment behavior.
