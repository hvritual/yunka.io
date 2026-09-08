# AG-03 — IoT Delivery saved-view narrowing

> Document class: **EVIDENCE**
> Current status authority: [`../STATUS.md`](../STATUS.md)
> Scope: one existing use-case pair, not a complete application redesign

## Exact subjects

- Consumer base: `hvritual/iot-delivery-system@f190fe237fa1efb738b4866275804093a209de5b`.
- Qualified consumer: `bcd20632b666405c3f7a8fe8d53f591c78450087`.
- Consumer tree: `0c0d1a2da23de4b7315245faba2103ec7a64377e`.
- Consumer PR: [IoT Delivery #7](https://github.com/hvritual/iot-delivery-system/pull/7).
- Qualification: [run 34222448048](https://github.com/hvritual/iot-delivery-system/actions/runs/34222448048).
- Actual-main acceptance: [run 34224052206](https://github.com/hvritual/iot-delivery-system/actions/runs/34224052206).
- Runtime/generator gitlink: `057ebcf88a87303eb633eb6e604d306f633dfac0`, unchanged.

Product commits were created with local Git and recreated with the same metadata
and tree by Runner-local Git. Qualification binds product identity, not the
separate control-workflow commit. Temporary control files are not product files.
A malformed unreferenced initial transfer blob was not applied or executed;
verified fragments reconstructed the exact local patch and candidate tree.

## Delivered architecture

SaveView and ListSavedViews remain methods on the existing public handwritten
Service facade and the same canonical DeliveryService/management contract. Their
rules now reside in a hidden `application/savedview/internal/usecase` package.
The owner's resource-free Build returns only the two-method use-case interface.

The leaf `domain` package owns saved-view/filter values and normalization. Existing
Go names are aliases, retaining source use and JSON persistence shapes; reflective
package identity now follows the domain owner. The new Repository port exposes
only CreateSavedView and ListSavedViews. Crucially the object actually injected
into the use case is a separate non-embedded two-function adapter, rather than
an unchanged 22-method Repository assigned to a smaller interface.

Saved-view SQL is isolated in `infrastructure/persistence/savedview`. Its per-call
resolver delegates to the existing SQLite executor's root-transaction selection.
It opens no pool and begins/commits no transaction. The create function binds the
already-transactional repository wrapper, preserving Outbox-before-write and audit
atomicity at the existing canonical execution boundary.

Unrelated use cases still use the original broad Service/Repository. This pilot
is not evidence that the entire internal tree has been remediated. It does not
add a new canonical Application, RPC, generated contract, framework runtime or
architecture source of truth.

## Behavior preserved and checks added

The same behavior assertions run against the immutable old implementation and the
candidate: validation/identity ordering, trusted UserID, owner isolation, shared
sequence consumption, cross-owner ID collision, filter normalization, stable sort,
SQLite reopen/duplicate/error handling. The original ID-scan concurrency guarantee
is not widened into cross-process atomic allocation. Source review after the first
full qualification found an omitted legacy case: a wrapped ErrNotFound from the
all-owner collision query was formerly treated as an available ID. The second
commit normalizes only that query in the compatibility adapter; public owner-list
errors still propagate. The same new test fails against the initial extraction,
but passes on the immutable pre-refactor source and the corrected candidate.
Initial qualification is not silently promoted to evidence for the corrected tree.

Six explicit TestAG03 parent groups check these behaviors, actual method sets,
a truly minimal two-method Repository, new-role import policies and the bounded
write-guard adaptation. A real mutant replaces the attenuator with the full
Repository. It must fail for the actual 22-method object; restoring the source
must pass. A smaller interface declaration alone cannot satisfy this proof.

No existing whole-file exclusion is extended. Only two exact source locations
can invoke their declared canonical narrow contract in the existing parser guard;
wrong callers, fake imports and wide Repository declarations remain rejected.
This is a limited parser-based consumer rule, not the general AG-04 type analyzer.

Three real Go-overlay probes accept the owner import and reject direct/aliased
hidden imports with the precise compiler diagnostic. The probes modify no tracked
source and are permanently called by run-yu30-regression.sh. All new Go tests are
also included by the existing full/race package tests.

## AG-03R qualification correction: SQLite startup (consumer #6)

The final-sentinel candidate run 34221473016 failed the existing bootstrap
seed/restart test during full race execution: configure SQLite connection returned
SQLITE_BUSY. The failing constructor's pragma order was unchanged from the base;
it configured WAL before installing its intended 5000ms busy wait. A separate
corrective commit moves the SAME busy setting before the lock-sensitive pragma.
There is no timeout increase, retry-until-green, schema change, transaction-owner
change, driver update or Yunka runtime mutation.

The real-file regression holds an exclusive lock through another connection.
The immutable old constructor must fail for the exact startup SQLITE_BUSY reason;
the corrected constructor waits for release, then verifies WAL, the original
busy/foreign-key settings and preserved data. The original seed/restart test is
also repeated under race. The new untagged regression is covered by recurring
full package/race gates. The failure record remains a failure, not an inherited
qualification for the later corrected source.

## Verification and evidence

Final candidate qualification used Go 1.25.13, protoc 3.21.12, Node 22.16.0,
real disposable SQLite and the installed Chromium-compatible browser. It passed:

- seven AG03 parents plus nine subcases: 16 named test pass events;
- four original YU15 transaction/audit/Outbox tests unchanged;
- identical old-base behavioral assertions: two parents plus three subcases;
- actual 22-method substitution rejected, restored two-method object accepted;
- the original constructor rejected with exact SQLITE_BUSY in three held-lock
  race tests, corrected constructor passed ten, original bootstrap passed ten;
- the same wrapped-sentinel assertion failed on the initial extraction and passed
  the original baseline and fixed implementation.

No test-level failures or skips occur in the targeted positive streams. Expected
RED failures are asserted by exact name and error, not counted as positive tests.
Artifact `10054532123` has ZIP SHA256
`4fa1167983f488ae6a6bd753dfc89158534a74c9522b2cfee75ce1b368ad07c7`;
its CRC, all 38 manifest members, Git bundle, head/tree and named JSON events were
independently read back. First qualifier 34220398527 applies only to the initial
03db source. Failed run 34221473016 applies to 59b and remains a failure. Neither
is substituted for qualification of final bcd20632.

Actual-main run `34224052206` repeats the seven named AG03 groups,
four unchanged transactional tests, wide-object rejection/restoration and the full
YU32H/YU30/YU31 pipelines after verifying merged PR #7 and exact main SHA/tree.
Its final readback matches the same qualified product. Artifact `10055167536`
has ZIP SHA256 `7b66ceab8aa0031de13124cdb48422b2881c69eb3eca5aa104c6b92fc6fc2e79`;
CRC, all 32 manifest members, named test events and the unsigned
main receipt were independently verified. Candidate and main verification have
distinct receipts; neither is a self-issued approval.

Qualification includes the existing full YU-30 pipeline (canonical generate/check
repeat, Ownership/Audit/ChangeSet, module drift, Go tests/vet/race, frontend unit
/typecheck/build/security audit and real browser E2E), YU-31 actual processes and
transport/shutdown, and YU-32H behavioral RED/GREEN. The unchanged YU-15 transactional
commit/audit-failure/Outbox-failure tests remain executable; no gate is replaced by
an isolated helper test. Exact test inventories, tool versions, source hashes,
clean worktrees and final remote head/tree are retained in the delivery artifact.

Ordinary exact-head PR workflows YU30 `34223297120` (four matrix jobs), YU31
`34223297149`, and YU32 `34223297154` (two jobs) all passed. Independent automated
review comment `5584812723` reports no major issues for exact `bcd20632b6`.
The subsequent non-force main fast-forward retains that same commit and tree;
PR #7 merge SHA equals the qualified head. Automated review is not represented
as a human or self-issued APPROVE.

The fixed runtime/framework gitlink, PB/API/Operation identities, generated
contracts/modules/assembly, dependency locks, web and historical backend are
unchanged. Existing evidence proves only its named source. Main integration and
post-integration readback are separate from candidate qualification and review.

## Framework qualification: exact historical candidate

The first framework documentation candidate was separately qualified; it is not
part of the consumer runtime qualification above:

- Framework base: `5f6623ee3ddad297403727d293dec77bd3c64367`.
- Qualified framework commit: `f4b15698f586ea26ec1c110e8af9c66cad1b8d39`.
- Qualified framework tree: `2208250337540fd61e9ad9cabfe519e662042367`.
- Qualification: [run 34225085409](https://github.com/hvritual/yunka.io/actions/runs/34225085409), SUCCESS.
- Artifact: `10055639922`, `ag03-framework-qualification`.
- Artifact ZIP SHA256: `63c639968fb6d6413be25bbf10ad607f9bd1a0de43084191fff00617b0d31ef1`.

That run created the normal Git commit before locked Go 1.25.13 / protoc 3.21.12
`make verify-production` on real MySQL 8.4, then checked deterministic generation,
dependencies and a clean worktree before publishing the exact task branch. The
artifact's unsigned `QUALIFICATION-RECEIPT.json` records `QUALIFIED_NOT_YET_MERGED`
and binds that product SHA/tree; its control-workflow SHA is not the product SHA.
The ZIP/CRC, all nine manifest members and prerequisite-aware Git bundle were
independently verified during closeout.

This linkage was added following the review of that candidate in
[PR #172](https://github.com/hvritual/yunka.io/pull/172). It names historical evidence,
not qualification of the later amendment containing this paragraph. Every later
candidate requires fresh exact-head qualification and review. Final main
integration is recorded by PR #172 and its post-commit main receipt, binding the
actual framework SHA/tree, successful verification runs and consumer acceptance
run `34224052206`; no commit attempts to certify itself using its own unwritten SHA.

## Framework handoff and continuation

This framework change only reconciles STATUS, the AG plan and this evidence. No
Yunka compiler, Executor, authorization, UoW, dependency lock or normal CI is changed.
It is qualified separately before synchronization to main. No production/data
migration occurred. Rollback of the extraction keeps aliases, facade, SQL adapter, tests and
recurring script consistent. The independently proven startup correction should
be retained unless its lock risk is explicitly addressed; no database migration
rollback is implied. AG-04 should build precise generic checks from both proven consumer
pilots; it is not claimed delivered by AG-03.
