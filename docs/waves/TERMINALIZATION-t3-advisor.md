# Framework Terminalization — T3 Advisor Evidence Contract

> Document class: **QUALIFICATION EVIDENCE**  
> State: **T3 complete / production-qualified / real-consumer pressure-qualified / merged**  
> Delivery PR: #143  
> Base: `main@c661abd8c7a4bbc14309d1dc712d55b8b4149672`  
> Final behavioral candidate: `5738ac71ef5e838fae9231e908ae1a87ee7e2b31`  
> Final PR head: `b1ff3515be70769f6a9aa7f1c6e95f0d8348467a`  
> Canonical merge: `63aa9881f84aa3f1a53fa1a1f46b999ddbbdde83`

## Goal

Allow an external AI/architecture reasoner to consume deterministic Yunka Audit evidence without turning model output into framework truth, mutation authority, or merge authority.

T3 remains outside the Yunka runtime. Yunka exports a deterministic request envelope, validates an externally produced response against the exact request/evidence digests, and emits an `advisory_only` attestation. Yunka does not invoke an LLM, does not persist advisor state, and does not apply code or contract changes.

## T3.1 — Evidence-bound advisor contract

The new control-plane entry points are:

```text
yunka advisor request [--base <git-ref>] --format agent-json

yunka advisor validate \
  --request <request.json> \
  --response <response.json> \
  --format agent-json
```

`advisor request` reuses the canonical `audit.Build` / `audit.BuildWithBase` path. It embeds the normalized `auditcore.Report` and binds it with SHA-256 `auditDigest` and `requestDigest` values.

The request policy is fixed:

```text
authority = advisory_only
mutationAuthorized = false
authorityExpansionRequiresHuman = true
```

Allowed recommendation kinds are limited to:

- `remediation_recommendation`
- `investigation_hypothesis`
- `design_question`

Every recommendation must reference one or more current Audit Finding IDs. A remediation recommendation may reference only `proven_violation` findings. Unknown findings, duplicate bindings, unsupported kinds, request/evidence digest drift, unknown JSON fields, and trailing JSON fail closed. A request with zero deterministic findings accepts only an empty recommendation set.

Successful validation emits an `advisory_only` attestation containing request/response digests and evidence bindings. It is not a Change Attestation and cannot represent `safe_to_merge`, mutation approval, architecture approval, or business authority.

Exact T3.1 candidate `01260e60b0043c726aecd5808bd352e41ef8fb94` passed:

- CI #512 / run `33780256579`, including full Verify and determinism
- production #270 / run `33780256560`, including MySQL 8.4 and clean-worktree

## T3.2 — Agent Context protocol v3

`yunka context --json` is explicitly versioned from schema v2 to schema v3 rather than silently adding machine-contract fields.

Schema v3 exposes the qualified advisor protocol:

```text
advisorRequest  = yunka advisor request --format agent-json
advisorValidate = yunka advisor validate --request <request.json> --response <response.json> --format agent-json
```

No Advisor semantics changed in T3.2; it only makes the already-qualified T3.1 protocol discoverable to AI/automation clients.

Exact T3.2 candidate `5738ac71ef5e838fae9231e908ae1a87ee7e2b31` passed:

- CI #515 / run `33815046944`, including full Verify and determinism
- production #273 / run `33815046909`, including MySQL 8.4 and clean-worktree

## T3.3 — Real Biz adversarial qualification

Consumer: `hvritual/biz`

- Biz B13 pressure tree: `506e9c117822855db318f8b4b6689d318a62ded1`
- Biz debt baseline: `main@7d5afbe9cb4b849be462d9a7aed65877ed227700`
- exact Yunka candidate: `5738ac71ef5e838fae9231e908ae1a87ee7e2b31`
- qualification branch commit: `3a74c14ca89b1d0f7eccfab40a9b23db8ec6d591`
- qualification run: `33815375770` — PASS
- evidence artifact: `9916336880`
- artifact SHA-256: `606d68ad001311d183ff0ff22a00cf8fd55fe0c93ea734b3ce91c39719432e2a`

The real consumer qualification proved:

1. `context --json` is byte-stable at schema v3 and exposes the exact Advisor request/validate commands.
2. Clean B13 Audit evidence contains no findings; an empty external response validates only as `advisory_only` with no bindings.
3. A temporary uncommitted Application fixture using the real Biz import forms produced exactly the three proven/new-debt rules:
   - `AUDIT-APP-001`
   - `AUDIT-AUTH-001`
   - `AUDIT-INFRA-001`
4. A valid `remediation_recommendation` bound to the exact three Finding IDs produced a valid `advisory_only` attestation.
5. The following adversarial responses all failed closed:
   - recommendation referencing an unknown Finding ID;
   - top-level `authority: safe_to_merge` injection;
   - mismatched `requestDigest`;
   - unsupported `apply_patch` recommendation kind;
   - non-empty advice against a zero-finding request.
6. The temporary pressure fixture was removed and Biz Git status reconciled to the pre-qualification state.

## Final delivery qualification and integration

Final PR head `b1ff3515be70769f6a9aa7f1c6e95f0d8348467a` contained the qualified T3 behavior plus delivery evidence and STATUS reconciliation. It passed:

- CI #517 / run `33815799648`, including full Verify and determinism
- production #275 / run `33815799615`, including MySQL 8.4 and clean-worktree

PR #143 then merged that exact head into canonical `main` as `63aa9881f84aa3f1a53fa1a1f46b999ddbbdde83`.

## Hard boundaries preserved

T3 does not:

- invoke or embed an LLM/provider/network SDK;
- create a second architecture or Audit Source of Truth;
- persist advisor state;
- generate or apply patches;
- approve Change Contract scope expansion;
- authorize security/transaction/capability/permission changes;
- produce `safe_to_merge` authority;
- infer that a recommendation is business/domain truth merely because it passed evidence binding;
- modify Runtime, Executor, ExecutionScope/UoW, authorization, persistence, transport, compiler, or protobuf business semantics.

## Next gate

T3 is closed. The next planned terminalization work is T4 ChangeSet/new-structure convergence, but it is not active until a new isolated delivery is explicitly started from the reconciled canonical `main`.
