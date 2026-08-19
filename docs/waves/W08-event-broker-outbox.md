# W08 — Event Broker + Transactional Outbox

## Goal

Provide a reliable business-event boundary without replacing the existing in-process EventBus or
binding yunka to one external broker. W08 establishes the event contract, delivery semantics,
transactional-outbox seam, dispatcher state machine, and operational snapshots.

## Architecture

```text
business transaction
  ├─ domain/business state
  └─ outbox row (same DB transaction)
             ↓ commit
       durable outbox
             ↓ lease claim
         dispatcher
             ↓
        event.Broker
       ├─ LocalBroker (dev/process-local)
       └─ future external adapters
             ↓
        consumer handler
             ↓
      idempotency by Event.ID
```

SLS is not part of the business-message path. W4 observes the dispatcher and broker through
standard traces, metrics, and runtime events.

## Event envelope

Every message uses `event.Envelope` with:

- schema version;
- stable event ID;
- topic and type;
- source and subject;
- correlation and causation IDs;
- UTC occurrence time;
- content type;
- immutable payload/metadata copies;
- bounded routing identifiers and metadata count/key/value sizes;
- delivery attempt added only to the delivered clone.

`Normalize` generates the ID and timestamp once. Retrying the same stored envelope never generates
a new ID.

## Trust and tracing

The broker may propagate W3C trace context through envelope metadata using an optional
`event.Propagator`. Event handlers start from a value-stripped context: cancellation/deadline may
be retained for local delivery, but W02 Principal/authentication state is not inherited. This
keeps local and remote broker semantics aligned.

Recommended W4 wiring:

```go
broker := event.NewLocalBroker(existingBus,
    event.WithPropagator(observability.EventPropagator()),
    event.WithMiddleware(provider.Middleware()),
)
```

## Transactional outbox

Calling `Store.Enqueue` after a business commit is **not** a transactional outbox. The production
path must insert the outbox record with the exact same database transaction as the business
change. The existing request-scoped ORM exposes `TransactionDB()` only while its transaction is
active:

```go
err := rt.Transaction(nil, func() error {
    // business repository writes use orm.DB here
    tx, ok := orm.TransactionDB()
    if !ok {
        return errors.New("transaction is not active")
    }
    return outbox.EnqueueTx(ctx, outboxStore, tx, envelope)
})
```

`GORMStore.EnqueueTx` accepts that active `*gorm.DB`. The event ID is the outbox primary key, so
re-enqueueing the same event returns `ErrDuplicate` rather than creating two rows.

`MemoryStore` is intentionally not a `TransactionalStore` and is only for tests/dev.

## Dispatcher state machine

```text
pending
  ↓ claim(owner, lease), attempts++
in-flight
  ├─ publish success → published
  ├─ failure + attempts left → pending(nextAttemptAt)
  ├─ failure + attempts exhausted → dead-letter
  └─ worker dies → lease expiry → reclaim
```

Claims use database row locks and owner-scoped state updates. `MarkPublished`, `Retry`, and
`DeadLetter` fail with `ErrLeaseLost` when a stale worker tries to update a record it no longer
owns.

Dispatcher configuration rejects a lease shorter than the worst-case batch publish window
(`ceil(batch/concurrency) × publishTimeout`), preventing records from expiring while waiting for a
local worker slot.

## Delivery guarantee

W08 is at-least-once, not exactly-once. There is an unavoidable failure window:

```text
broker accepts event
        ↓
process/database failure before MarkPublished
        ↓
lease expires
        ↓
same Event.ID is published again
```

Consumers must use `Envelope.ID` for deduplication when repeating the side effect would be unsafe.

## Retention and dead letter

Published records are not purged automatically. Stores may implement `RetentionStore` and an
operator-controlled maintenance job can delete published rows older than a chosen retention
window in bounded batches.

Dead-letter records are never automatically replayed. Replay requires an explicit operational
workflow so poison events cannot silently re-enter the business path.

## Diagnostics and observability

`diagnostics.Outbox` exposes only:

- pending;
- in-flight;
- published;
- dead-letter;
- oldest pending timestamp/age.

It never exposes event payload, metadata, event ID, or last-error body.

`observability.OutboxObserver` emits the bounded runtime-event vocabulary:

- `outbox.published`;
- `outbox.retry`;
- `outbox.deadletter`.

Payload is never logged by the adapter.

## Acceptance

- Event envelope normalization is stable and clone-safe.
- LocalBroker applies the unified middleware chain and converts consumer panics into publish
  errors without leaking Principal context.
- Memory outbox validates duplicate IDs, leases, retry, dead-letter, reclaim, and retention.
- Dispatcher retry preserves event ID and delivery attempt, isolates observer/broker panics, and
  validates lease safety.
- GORM adapter compiles against the existing GORM version, uses primary-key idempotency, row-lock
  claims, and owner-checked state changes.
- Core event/outbox packages pass unit, race, and vet harnesses.
- Diagnostics expose aggregate outbox state only.

## Explicitly deferred

- Kafka/RabbitMQ/Pulsar/NATS adapters;
- automatic dead-letter replay;
- consumer-inbox/dedup persistence abstraction;
- cross-service Application Graph edges;
- exactly-once claims.
