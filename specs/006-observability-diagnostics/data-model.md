# Data Model: Opt-in Observability and Diagnostics

All public values are immutable snapshots: callers receive copied maps/slices and cannot mutate
runtime state. Times are UTC wall-clock values; durations use a monotonic source internally.

## Observability Configuration

| Field | Type | Rules |
|---|---|---|
| Enabled categories | Bit set | Logging, health, diagnostics, measurements, correlation, and state events are independent; empty by default |
| Instance ID | String | Optional caller value or generated process-local opaque value; never derived from credentials/address |
| Event sink | Interface | Required only for structured logging/state records; invoked asynchronously |
| Buffer capacity | Integer | Non-negative, validated upper bound; zero means documented default when sink enabled |
| Overflow policy | Enum | `drop-newest` default or `drop-oldest` |
| Sink time budget | Duration | Positive and bounded; controls dispatcher callback budget, never broker operations |
| Probe timeout | Duration | Optional default; caller deadline wins when earlier |
| Redacted fields | Set of strings | Case-insensitive additions merged with mandatory defaults |
| Correlation conflict policy | Enum | `preserve` default; optional explicit `replace` |

Configuration is validated during `Init`/construction. Runtime enablement changes only the atomic
category mask; buffer sizing, sink, redaction, and conflict policy are immutable until re-init.

## Broker Lifecycle State

Values: `unknown`, `connecting`, `ready`, `degraded`, `reconnecting`, `stopped`.

```text
unknown ──Connect──> connecting ──success──> ready
   │                    │                     │
   └──Disconnect──────> stopped <──Disconnect┘
                        ▲                     │
                        │              partial failure
                        │                     ▼
                        └────────────── degraded
                                              │
                                      connection lost
                                              ▼
                                         reconnecting
                                          │        │
                                      success    failure/retry
                                          ▼        └── reconnecting
                                         ready
```

Invalid or duplicate native notifications do not panic; they retain the last valid state and may
emit a categorized internal diagnostic. Repeated Connect/Disconnect remains idempotent.

## Health Status

| Field | Type | Meaning |
|---|---|---|
| CheckedAt | Time | When this passive snapshot was created |
| State | Lifecycle state | Current synchronized state |
| Connected | Boolean | Native transport/client connection is available |
| Ready | Boolean | Broker can perform its configured publish/consume role |
| Degraded | Boolean | Some but not all expected capability is available |
| LastConnectedAt | Optional time | Most recent successful connection |
| LastReadyAt | Optional time | Most recent transition to ready |
| Error | Optional safe error | Most recent relevant categorized failure |
| Capabilities | Bounded map | Known support flags only; no vendor objects |

`Health()` performs no I/O. A `Probe(ctx)` result uses the same shape and updates `CheckedAt`; probe
failure does not overwrite lifecycle truth without an adapter-confirmed transition.

## Diagnostic Snapshot

| Field | Type | Validation |
|---|---|---|
| Broker system | Enum/string | Fixed adapter name |
| Instance ID | String | Safe opaque ID |
| Health | Health Status | Copy captured atomically with counters |
| Connection timestamps | Optional times | First/current/last success as available |
| Reconnect count | Unsigned integer | Monotonic |
| Subscription count | Integer | Non-negative current gauge |
| In-flight operations | Bounded map | Fixed operation keys, non-negative |
| Operation totals | Bounded map | Fixed operation/outcome keys, monotonic |
| Last failure | Optional safe error | Category and allowlisted summary only |
| Dropped records | Unsigned integer | Monotonic overflow/timeout/panic count |
| Unsupported capabilities | Set | Fixed capability names |

## Safe Error

| Field | Type | Rules |
|---|---|---|
| Category | Enum | `configuration`, `authentication`, `authorization`, `connection`, `timeout`, `canceled`, `publish`, `consume`, `handler`, `ack`, `nack`, `reconnect`, `shutdown`, `unsupported`, `internal`, `unknown` |
| Code | Optional string | Bounded adapter/provider code after allowlist validation |
| Summary | String | Length bounded and redacted; never a raw serialized vendor object |
| Retryable | Optional boolean | Present only when adapter can determine it safely |

## Broker State Event

| Field | Type | Rules |
|---|---|---|
| Sequence | Unsigned integer | Monotonic per broker instance |
| Timestamp | Time | UTC |
| Broker system / instance | Strings | Bounded safe identity |
| Kind | Enum | Connect, disconnect, reconnect start/success, operation failure, dropped-record notice |
| From / To | Lifecycle states | Required for transitions |
| Operation | Enum | Fixed signal catalog value |
| Severity | Enum | Debug, info, warn, error |
| Outcome | Enum | Success, failure, canceled, unsupported |
| Error | Optional safe error | Required for failure when known |
| Attributes | Map | Fixed allowlisted keys and bounded values only |

Ordering is the runtime enqueue order for one broker instance. Delivery may have gaps when overflow
occurs; gaps are detectable through sequence numbers and dropped-record count.

## Operational Measurement

| Field | Type | Rules |
|---|---|---|
| Name | Fixed string | Defined in `contracts/signal-catalog.md` |
| Kind | Counter, histogram, or up/down counter | Fixed per name |
| Value | Number/duration | Non-negative except decrement operations on gauges |
| Attributes | Fixed map | Broker system, operation, outcome, error category only |

No destination, message ID, client ID, queue, header, raw error, or body is a measurement
dimension.

## Correlation Context

| Field | Type | Rules |
|---|---|---|
| Trace parent | String | W3C-compatible syntax and size validation |
| Trace state | Optional string | W3C-compatible, size bounded |
| Baggage | Not propagated by default | Requires future explicit policy due to sensitive/high-cardinality risk |
| Conflict result | Enum | Injected, preserved, replaced, unsupported, invalid |

Correlation metadata is copied into broker-native message attributes only when enabled and
supported. Extraction creates a child processing context without mutating the received `Message`
visible to the user.
