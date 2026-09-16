from pathlib import Path

path = Path("docs/STATUS.md")
text = path.read_text()
old_date = "> Reconciled date: 2026-09-10"
if old_date not in text:
    raise SystemExit("expected reconciled-date marker not found")
text = text.replace(old_date, "> Reconciled date: 2026-09-17", 1)

typed_row = "| Typed infrastructure capability export / binding | **Complete / production-qualified / merged** | issue #124 / PR #131; exact candidate `c3123fb4a3e7731f0edf5539c3d8003fc0e41bc7` passed CI #475 and production #233 after synchronization with AX6 main, merged as `d06a6330db9093e0bc586decb6bdc00122b4aa99`, and exact-main push CI #476 / production #234 also passed |"
quality_row = "| Engineering Quality / human-reviewable AI code | **Complete / globally qualified / merged** | issue #192 global acceptance passed on exact behavioral main `fe63375b84be89c068ad57b81cc7def578cf30fc` after #220 evidence-integrity repair through PR #222; exact-main CI `35162731142`, Production `35162731126`, source-policy `35162731168`, template `35162731173`, and global acceptance `35127746651` passed |"
if typed_row not in text:
    raise SystemExit("typed-infrastructure status row not found")
if quality_row not in text:
    text = text.replace(typed_row, typed_row + "\n" + quality_row, 1)

marker = "## Bounded engineering-quality migration — issue #200"
if marker not in text:
    raise SystemExit("engineering-quality migration marker not found")
section = """## Engineering Quality global acceptance — issues #192 / #220

**State: COMPLETE / GLOBALLY QUALIFIED / MERGED.**

The human-reviewable AI-code initiative is accepted on exact behavioral main
`fe63375b84be89c068ad57b81cc7def578cf30fc`, tree
`ca1b922106e15175b31c74c538fef348dfd0bcec`. PR #222 integrated the #220
control-plane evidence-integrity repair by non-force fast-forward, so the
accepted main commit and previously qualified repair candidate are identical.

The repair carries compact semantic evidence binding through validated semantic
review attestations and finding deltas, re-reads immutable baseline source from
Git objects and current source from the active project, re-derives request/source
identity, and requires the active Change `BaseSHA`, `HeadSHA`, and candidate
identity to match before advisory debt can enter `QualityDebtProof`. Human review
packet construction/checking revalidates the same candidate. Stale current-review
head, same-head stale source bytes, and review-baseline/Change-baseline mismatch
now fail closed. Valid semantic findings remain visible and advisory-only; this
change does not grant semantic advice mutation or merge authority.

Fresh global acceptance run `35127746651` rechecked the eight #192 clauses
against the exact accepted source. The contracts/consumer branch passed the
integrated quality matrix with 211 top-level tests, required semantic/debt/review/
migration tests, pinned Biz and IoT qualifications, zero required-test skips, and
unchanged source readback. Its Production branch passed the unchanged canonical
`make verify-production` gate with the locked toolchain and MySQL 8.4, followed by
clean source readback and evidence hashing.

Separate exact-main push gates also passed: CI `35162731142` including Verify and
determinism, Production `35162731126` including MySQL 8.4 and clean worktree,
AG-05 source-policy `35162731168`, and AG-06 template qualification
`35162731173`. Candidate qualification for PR #222 additionally passed semantic
review, quality-debt, review-packet and quality-migration gates before integration.
External review was not performed and is not required by current repository
policy; automated qualification is not represented as independent approval.

Acceptance remains bounded to the documented engineering-quality rules and named
consumer scopes. It does not claim universal business/DDD correctness, automatic
mutation/merge authority, repository-wide historical-debt removal, or completion
of unrelated #161, #177, #178/#188 or remaining AG work. Runtime, Executor,
Authz, UoW, protobuf business contracts, dependencies and consumer source are
unchanged by the #220 repair.

"""
if "## Engineering Quality global acceptance — issues #192 / #220" not in text:
    text = text.replace(marker, section + marker, 1)
path.write_text(text)
