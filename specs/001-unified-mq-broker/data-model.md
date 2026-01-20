# Data Model: Unified MQ Broker

## Entities

### Message
The core data structure representing a unit of asynchronous communication.

| Field | Type | Description |
|-------|------|-------------|
| Header | `map[string]string` | Metadata and protocol-specific parameters (mapped via extensions). |
| Body | `[]byte` | The raw payload. |

### Event
A wrapper for a received message, providing control over its lifecycle.

| Field | Type | Description |
|-------|------|-------------|
| Topic | `string` | The topic/queue where the message originated. |
| Message | `*Message` | The actual message content. |
| Ack | `func() error` | Acknowledge successful processing. |
| Nack | `func() error` | Signal processing failure (triggering retry if supported). |

## State Transitions: Broker Lifecycle

1. **Uninitialized**: `broker.NewBroker()` called with options.
2. **Configured**: Internal state set, but no client SDK initialized.
3. **Connecting**: `Connect()` called; SDK client created and dial started.
4. **Connected**: TCP/Socket established; ready for I/O.
5. **Disconnected**: `Disconnect()` called; resources cleaned up.

## Validation Rules
- **Topic Name**: Must not be empty.
- **Body**: If nil, treated as empty `[]byte{}`.
- **Header Keys**: Should be unique; core system reserves `SHARDING_KEY`, `KEYS`, `TRACE_ID`.
