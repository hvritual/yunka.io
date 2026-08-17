# Repository Instructions

These instructions apply to the entire repository and every task performed in it.

## Mandatory task bootstrap

Before planning, analyzing, editing, or running task-specific commands:

1. Read this `AGENTS.md` completely.
2. Read `PROJECT_MEMORY.md` completely.
3. Run `git status --short --branch`.
4. Run `git branch --show-current` and `git remote -v`.
5. Preserve all existing user changes and inspect any relevant project documentation before editing.

Do not skip this bootstrap for small, read-only, or follow-up tasks.

## Repository and Git policy

- The canonical repository is `https://github.com/hvritual/yunka.io`.
- `origin` must point to `https://github.com/hvritual/yunka.io.git`.
- The default branch is `main`.
- All project work must be performed inside this repository.
- Any operation that creates a Git commit must use the local `git` command-line tool.
- Never create commits, trees, blobs, files, or refs through the GitHub API or connector.
- The GitHub connector may be used for repository metadata, issues, pull requests, reviews, permissions, and other non-commit collaboration operations.
- Never overwrite, discard, reset, or clean existing user changes without explicit authorization.

## Durable memory maintenance

- Treat `PROJECT_MEMORY.md` as the repository's durable decision memory.
- Update it when the user makes a lasting decision that changes repository scope, workflow, governance, architecture, or delivery policy.
- Do not add transient task details or speculation.
- When a memory update needs a commit, stage and commit it with local `git`.
