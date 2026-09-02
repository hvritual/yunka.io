# Repository Instructions

These instructions apply to the entire repository and every task performed in it.

## Mandatory task bootstrap

Before planning, analyzing, editing, or running task-specific commands:

1. Read this `AGENTS.md` completely.
2. Read `PROJECT_MEMORY.md` completely for durable governance and architecture invariants.
3. Read `docs/STATUS.md` completely for the current framework/wave/release and pressure state.
4. Run `git status --short --branch` when a local checkout is available.
5. Run `git branch --show-current` and `git remote -v` when a local checkout is available.
6. Preserve all existing user changes and inspect any relevant current, historical, and evidence documentation before editing.

Do not skip this bootstrap for small, read-only, or follow-up tasks. When work is performed through the GitHub Connector because no local checkout is available, verify the target repository, base branch/ref, and current head commit through the Connector before mutating repository state.

`AGENTS.md` is the current repository-governance authority. `PROJECT_MEMORY.md` is the durable architecture/governance decision authority. `docs/STATUS.md` is the current delivery/status authority. If an older historical entry conflicts with these current authorities, follow the authority that owns that fact and treat the older entry as historical or stale.

## Repository and Git policy

- The canonical repository is `https://github.com/hvritual/yunka.io`.
- The canonical remote name is `origin`, pointing to `https://github.com/hvritual/yunka.io.git` for local Git workflows.
- The default branch is `main`.
- All project work must be performed inside this repository.
- Local `git` remains the preferred path for fetch, branch, commit, merge, and push when a usable local checkout and repository authorization are available.
- GitHub Connector is also an explicitly authorized repository write path. It may create/update task branches, blobs, trees, commits, files, refs, pull requests, and push repository changes when needed to complete the user's requested work.
- Connector-based code delivery does not require a separate per-task authorization. This authorization remains in force until the user explicitly revokes or narrows it.
- Do not move or overwrite `main` directly unless the user explicitly authorizes the merge/ref movement. Normal implementation work must use an isolated task branch and then be reviewed/merged through the repository's normal flow.
- Do not force-update an existing branch unless the user explicitly authorizes it.
- Never overwrite, discard, reset, or clean existing user changes without explicit authorization.
- Before Connector writes, confirm the branch/base SHA to avoid writing against a stale baseline. Avoid concurrent writes to the same path/ref; sequence dependent writes using the latest returned SHA.

## Documentation truth policy

- Follow `docs/DOCUMENTATION_GOVERNANCE.md` for document classes and truth ownership.
- Current framework/wave/release/pressure state belongs only in `docs/STATUS.md`; do not reconstruct current status from historical roadmap headers or duplicate it into `PROJECT_MEMORY.md`.
- Durable architecture and governance decisions belong in `PROJECT_MEMORY.md`; do not store current HEADs, repository visibility, transient PR state, active task progress, or release-status tables there.
- `README.md` is current developer/product documentation and must describe mechanisms that exist on the current tree.
- Completed roadmaps under `docs/waves/**` may preserve their original planning prose, but must be explicitly classified as `HISTORICAL` when that prose can be mistaken for current status.
- Exact qualification evidence proves only the exact candidate/tree it names. Do not silently promote historical evidence into current status.
- A change that closes, opens, defers, supersedes, or activates a framework wave/pressure item must reconcile `docs/STATUS.md` in the same delivery flow.

## Durable memory maintenance

- Treat `PROJECT_MEMORY.md` as the repository's durable decision memory, not as a chronological task log.
- Update it when the user makes a lasting decision that changes repository scope, workflow, governance, architecture, security boundaries, execution ownership, or delivery policy.
- Do not add transient task details, current commit SHAs, repository metadata that can change independently, duplicated current wave status, or speculation.
- Durable memory updates may be committed through local Git or the authorized GitHub Connector according to the repository write policy above.
