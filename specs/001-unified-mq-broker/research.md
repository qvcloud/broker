# Research Findings: Unified MQ Broker Refinement

## Decision 1: Option Consumption Detection (FR-006)
- **Decision**: Implement the **Option Auditor Pattern**.
- **Rationale**: 
    - Decouples core `broker` package from adapter implementations.
    - Lightweight: Uses `context.Context` to carry a tracking map without type dependency.
    - Allows `WarnUnconsumed(ctx)` to be called at the end of `Connect()` or `Publish()`.
- **Alternatives considered**: 
    - **Global Registry**: Rejected as it complicates concurrent tests and dynamic adapter loading.
    - **Generic Map**: Rejected as it doesn't solve the "how to know if it was used" problem without a key-by-key acknowledgement.

## Decision 2: Driver Initialization & Connection (FR-009)
- **Decision**: Strictly enforce **Late Binding via Connect()**.
- **Rationale**: 
    - Industry standard (Go-Micro, Watermill).
    - `NewBroker()` remains synchronous and I/O free, making it safe for container startup and configuration injection.
    - Configuration errors (e.g., missing credentials) caught at `NewBroker`/`Init`; Network errors (e.g., dial timeout) caught at `Connect`.
- **Alternatives considered**: 
    - **Lazy Connect**: Rejected because it pushes connectivity issues into the `Publish` path, making the first message highly latent and harder to monitor during startup.

## Decision 3: Smart Serialization for Body (FR-010)
- **Decision**: **Type Switch for Zero-Copy** + **sync.Pool in Marshaler**.
- **Rationale**: 
    - `case []byte` in a type switch is zero-allocation and extremely fast (~nanoseconds).
    - Using a `sync.Pool` for buffers in the JSON Marshaler significantly reduces GC pressure for high-throughput struct-to-json workloads.
- **Alternatives considered**: 
    - **Reflection**: Rejected due to high CPU overhead in the hot path.
    - **Always Marshal**: Rejected as it would wrap `[]byte` in unnecessary JSON quotes/encoding.

## Integration Plan
1. Update `broker.go` with specialized `OptionTracker` primitives in the context.
2. Update all 6 adapters to use `GetTrackedValue` instead of direct `ctx.Value`.
3. Add `WarnUnconsumed` calls in adapter `Connect` and `Publish` methods.
4. Refactor `NewBroker` implementations to defer client creation to `Connect`.
