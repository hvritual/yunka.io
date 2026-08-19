# W05 — Selector 2.0

## Goal

Upgrade service-node selection without breaking the existing `selector.Selector` contract. W05 adds a stateful selection lifecycle that can use request feedback for P2C/EWMA/least-request routing, passive health, and outlier ejection.

## Scope

- Preserve `Selector`, `Next`, `Filter`, and legacy `Strategy` source compatibility.
- Keep `NewSelector()` on the historical Random behavior.
- Add `NewAdaptiveSelector()` with P2C as the opt-in default.
- Add `Picker.Pick()` / `Selection.Done()` so selection, latency, in-flight count, and outcome are one lifecycle.
- Add adaptive modes: P2C, EWMA, and LeastRequest.
- Add passive-health state and bounded outlier ejection.
- Add region/zone locality helpers while retaining existing version/label filters.
- Add diagnostics snapshots.
- Add an RPC client wrapper that closes the selection feedback loop without editing generated RPC files.
- Compose W3 resilience outside W5 selection so retries reselect nodes and local policy rejections do not poison node health.

## Non-goals

- No active network probing in the selector.
- No replacement of registry/discovery implementations.
- No generated gRPC file edits.
- No cross-service global breaker state in the selector.
- No user/request IDs as selector state keys or metric dimensions.

## Selection lifecycle

```text
RPCPolicy (timeout/retry/rate/load-shed/circuit)
  -> selector.WrapRPCClient
      -> Pick(service)
          -> P2C / EWMA / LeastRequest
          -> outlier filter
      -> transport.InvokeNode
      -> Selection.Done(error)
          -> EWMA latency
          -> in-flight decrement
          -> passive success/failure
          -> optional outlier ejection
```

Because W3 `Retry` wraps the final client, every retry attempt performs a fresh Pick. Circuit/rate/load-shed rejection happens before Pick, so a node is never punished for a request that was rejected locally.

## Safety defaults

- Adaptive selection is opt-in; existing `NewSelector()` behavior is unchanged.
- Outlier ejection requires consecutive failures.
- Maximum ejection percentage prevents all capacity from being removed in small clusters.
- A one-node service is never passively ejected.
- Zero-value outlier behavior is fail-open if every candidate is unavailable; `FailClosed` must be explicit.
- `context.Canceled` is ignored by the default feedback classifier.
- `Selection.Done` is idempotent.

## Acceptance

- P2C prefers the lower EWMA score when comparing two candidates.
- EWMA and LeastRequest modes are deterministic under controlled state.
- Pick/Done tracks in-flight requests and measured latency.
- Consecutive failures eject a node and it automatically becomes eligible after the ejection window.
- Ejection percentage limits preserve minimum capacity.
- Region/zone filters operate on node metadata.
- RPC wrapper invokes the selected node and records the result.
- Legacy selector implementations still work through the `Pick` fallback.
- `go test`, `go test -race`, and `go vet` pass for the selector implementation.
