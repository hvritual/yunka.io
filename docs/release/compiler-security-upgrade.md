# Compiler certificate-verification security upgrade

> Document class: **EVIDENCE**
> Current status authority: [STATUS](../STATUS.md)
> Candidate identity and actual results: the task branch Git/PR and original CI artifacts

WHY / Problem: cs-biz reproduced reachable GO-2026-6809 using fixed Go 1.26.9.
The official [Go advisory](https://pkg.go.dev/vuln/GO-2026-6809) identifies
Go 1.26.10 as the patch in the current language series. The user authorized the
toolchain/Consumer upgrade rather than suppressing the scanner.

WHAT / Ownership: tools/toolchain.env remains the tool identity owner. Workspace,
module and existing workflow selections plus the doctor fallback follow 1.26.10.
The original tools/qualify_consumer_release.py retains bundle, SDK source/checksum,
clean-tree, identical rebuild, independent installed-consumer generation/audit and
security ownership. No second distribution or qualification path is introduced.

BOUNDARY: language directives stay 1.26.0; published SDK versions/checksums,
protobuf tools, source/dependency policies, generated artifacts, public API,
runtime/authorization/UoW semantics and persistence remain unchanged. The current
doctor default selects the approved patch; explicit project/toolchain requirements
retain precedence. Historical security and distribution receipts retain their
original Go 1.26.9 identity. This record is not a new tagged/stable release.

PROOF: source changes alone are not acceptance. Original exact-candidate CI,
Production/MySQL, applicable consumer type/source/template and packaged-consumer
qualification must complete with clean source. Retain failures and report Windows
prechecks separately from Linux qualification. The downstream cs-biz lock is
updated only after actual producer commit/tree, manifest, SDK and archive digests
are verified against the qualified artifact. Candidate source uses the task-branch
Git identity; no future or invented qualification SHA is stored here.
