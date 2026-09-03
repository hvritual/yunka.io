# Distributed Trace and Execution Analysis

> Document class: **CURRENT**  
> Authority: current developer-facing distributed tracing, event causality, and TraceID analysis behavior  
> Current project status authority: [`STATUS.md`](STATUS.md)

## Purpose

Yunka uses one technical trace identity plus explicit business-event causality to analyze a request across multiple Yunka applications without introducing a second tracing runtime.

The two axes are intentionally distinct:

```text
technical execution       business causality
        |                         |
     trace_id              correlation_id
                                  +
                             causation_id
```

`trace_id` answers which synchronous or linked asynchronous execution participated in a technical call. `correlation_id` and `causation_id` preserve business-event lineage when work crosses durable asynchronous boundaries.

## Canonical RPC propagation

W3C Trace Context and Baggage are the canonical distributed propagation format.

App-owned gRPC connections created by `framework/platform.GRPCFactory` automatically install unary and streaming propagation interceptors. A consumer using the canonical Platform RPC capability therefore does not need to remember a separate tracing interceptor merely to preserve `traceparent`/Baggage across a Yunka-to-Yunka call.

`framework/observability.Provider.UnaryClientInterceptor` and `GovernedUnaryClientInterceptor` also inject propagation at the transport boundary after their logical client middleware has established the active span. This preserves the newly-created client span as the propagated parent.

Propagation does not establish remote caller identity. Authentication remains a separate trust-boundary concern.

## Event and Outbox causality

`framework/event.Envelope` remains the canonical event contract. Event IDs stay stable across retry and remain consumer idempotency keys.

`event.PrepareForPublish` is the canonical preparation boundary. Before an event enters a broker or durable Outbox it:

1. clones the envelope;
2. inherits parent event causality from `context.Context` when the child has only the default self-correlation;
3. sets `CausationID` to the parent event ID when no explicit causation is supplied;
4. injects the configured propagation metadata such as `traceparent`, `tracestate`, and Baggage;
5. normalizes and validates the final envelope.

For a parent event `E1` with correlation `C1`, a normal child event becomes:

```text
parent E1
  correlation_id = C1
        |
        v
child E2
  correlation_id = C1
  causation_id   = E1
```

Explicit non-default correlation/causation values remain application-owned.

Event handling starts a new identity trust boundary. The LocalBroker retains only supported propagation data and event causality; authenticated `Principal` state is not inherited implicitly.

## Durable trace preservation

The Outbox must preserve trace information at the original transaction boundary rather than recreate it in a later dispatcher worker.

`outbox.GORMStore` supports an `event.Propagator`. When configured, both `Enqueue` and `EnqueueTx` prepare the Envelope before JSON serialization. The canonical `framework/modules/outboxruntime` wiring supplies `observability.EventPropagator()` to both the durable store and the broker.

Therefore a committed Outbox row can retain the originating W3C trace metadata even when dispatch happens after delay, retry, process restart, or lease reclaim.

This does not change Outbox delivery semantics: delivery remains at-least-once and `Published` still means broker publication succeeded, not that a downstream business effect is semantically confirmed.

## Operation evidence in the active trace

`framework/observability.Middleware` installs a request-scoped Operation observer in the active `context.Context`. `framework/operation.Executor` consults both its explicitly configured observers and request-scoped observers.

Operation execution remains owned entirely by the existing Executor. The observer is read-only evidence and does not participate in security, idempotency, transaction, or outcome decisions.

Each observed phase/outcome is emitted as the bounded runtime event `operation.phase` with attributes including:

- `operation_id`
- invocation kind (`root` or `child`)
- phase
- outcome

Because the event is emitted through the active Observability Provider, structured logs and OpenTelemetry span events carry the current `trace_id`/`span_id`.

## Outbox lifecycle evidence

`observability.OutboxObserver` emits the existing bounded events:

- `outbox.published`
- `outbox.retry`
- `outbox.deadletter`

Before emission it restores W3C trace context from the persisted Envelope when available. Event evidence includes stable identifiers such as event ID, topic/type, correlation ID, causation ID, and delivery attempt.

This allows an Outbox worker lifecycle event to be correlated with the originating trace without depending on the worker's background context.

## TraceAnalyzer

`framework/diagnostics.TraceAnalyzer` is the vendor-neutral aggregation contract for a single TraceID.

A deployment supplies one or more `TraceSource` adapters:

```go
type TraceSource interface {
    Name() string
    LookupTrace(context.Context, string) ([]TraceEvidence, error)
}
```

Evidence is normalized into four kinds:

```text
span
log
operation
event
```

`TraceAnalyzer.Analyze(ctx, traceID)` returns one `TraceReport` containing deterministic evidence ordering plus per-source status. A failed source is isolated so healthy sources may still return partial evidence. Evidence for a different TraceID is rejected instead of being mixed into the requested chain.

The framework intentionally does not implement an SLS-specific query client. SLS, another OpenTelemetry backend, or a local evidence source can implement `TraceSource` without changing the framework execution model.

## Read-only HTTP analysis endpoint

`diagnostics.NewTraceHTTPHandler` exposes the TraceAnalyzer through an opt-in read-only HTTP handler. It follows the same diagnostics security posture as the existing diagnostics control plane:

- loopback-only by default;
- remote access requires explicit enablement and a Bearer token;
- no listener is started automatically.

A deployment may mount it, for example, as:

```text
GET /_yunka/trace?trace_id=<trace-id>
```

The exact route is application-owned because Yunka does not start a diagnostics server implicitly.

## What one TraceID can and cannot prove

With appropriate TraceSource adapters, one TraceID can correlate:

```text
HTTP/RPC entry
  -> Yunka Operation phases/outcome
  -> downstream RPC spans
  -> structured logs
  -> Outbox event publication/retry/dead-letter evidence
```

A TraceID does **not** by itself prove that an external side effect reached the desired authoritative state. External-effect receipt/readback/reconciliation remains a separate semantic closure problem.

Likewise, long-lived asynchronous business workflows may span multiple technical traces. Use `CorrelationID`/`CausationID` to follow that business lineage across trace boundaries rather than forcing one unbounded trace to represent the entire workflow.
