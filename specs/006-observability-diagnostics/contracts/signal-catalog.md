# Signal Catalog Contract

Signal names and bounded attributes are consistent across adapters. Unsupported native detail is
omitted and declared in the diagnostic snapshot rather than represented with fabricated values.

## Operations

`connect`, `disconnect`, `publish`, `receive`, `handler`, `ack`, `nack`, `retry`, `reconnect`,
`subscribe`, `unsubscribe`, `probe`, `dispatch`.

## Metrics

| Name | Kind | Unit | Meaning |
|---|---|---|---|
| `broker.operations` | Counter | `{operation}` | Completed operations by outcome |
| `broker.operation.duration` | Histogram | `s` | Operation elapsed time using monotonic timing |
| `broker.inflight` | Up/down counter | `{operation}` | Currently executing publish/receive/handler/Ack/Nack work |
| `broker.retries` | Counter | `{retry}` | Adapter-observed retries |
| `broker.reconnects` | Counter | `{reconnect}` | Reconnect attempts/outcomes |
| `broker.subscriptions` | Up/down counter | `{subscription}` | Current active subscriptions |
| `broker.observability.dropped` | Counter | `{record}` | Records dropped due to overflow, timeout, sink error, or panic |

Allowed attributes:

- `messaging.system`: fixed adapter name.
- `messaging.operation`: one operation from this catalog.
- `outcome`: `success`, `failure`, `canceled`, or `unsupported`.
- `error.category`: one bounded safe-error category, only for failures.
- `observability.drop.reason`: `overflow`, `timeout`, `sink_error`, `panic`, or `shutdown` only on
  the dropped-record metric.

Forbidden metric attributes include topic/destination, queue, subscription/consumer ID, client ID,
message ID, correlation/trace ID, address, header values, message body, raw error text, and arbitrary
user labels.

## Event kinds

| Kind | Default severity | Required context |
|---|---|---|
| `connection.started` | Info | from/to state |
| `connection.ready` | Info | from/to state |
| `connection.failed` | Error | state, safe error |
| `connection.stopped` | Info | from/to state |
| `reconnect.started` | Warn | from/to state, attempt count |
| `reconnect.succeeded` | Info | from/to state, attempt count |
| `reconnect.failed` | Error | state, attempt count, safe error |
| `operation.failed` | Error | operation, state, safe error |
| `observability.dropped` | Warn | aggregate drop reason/count |
| `capability.unsupported` | Warn | requested capability |

Successful high-frequency publish/receive/handler operations are measurements, not structured
events by default. This prevents log floods. Failure events must become available to the dispatcher
within one second of detection when configured capacity and sink budget are not exhausted.

## Correlation fields

Reserved logical fields are W3C `traceparent` and `tracestate`. Each adapter maps them to its native
header/attribute representation while preserving case and syntax rules required by that protocol.
Invalid inbound values are ignored and recorded only as a bounded `internal`/`invalid_context`
diagnostic; they never become parent context.

Default conflict policy is `preserve`: if the outgoing message already contains a reserved field,
the library leaves it untouched and skips injection for that field. Explicit `replace` permits
replacement with the current valid context. The library never injects baggage or payload content by
default.
