# Project Memory

This file records durable decisions that every new task must read and follow.

## Repository baseline

- Canonical repository: `hvritual/yunka.io`
- Repository URL: `https://github.com/hvritual/yunka.io`
- Git remote name: `origin`
- Git remote URL: `https://github.com/hvritual/yunka.io.git`
- Default branch: `main`
- Repository visibility: private
- GitHub access verified on 2026-08-17 with admin, push, and pull permissions.

## Working decisions

### 2026-08-17 — Canonical task repository

All subsequent project tasks must use this repository as their source of truth and working scope. Project code, documentation, plans, and durable decisions belong in this repository unless the user explicitly changes that scope.

### 2026-08-17 — Local Git commits only

All work that requires a Git commit must be committed with the local `git` command-line tool. GitHub connector or API operations must not be used to construct commits or modify Git objects and refs.

The GitHub connector remains appropriate for reading repository metadata and collaborating through issues, pull requests, reviews, and permissions.

### 2026-08-17 — Mandatory memory read

Every new task must read `AGENTS.md` and this file before planning, analysis, edits, or task-specific commands. The task must also verify the current branch, working-tree status, and configured remotes before making changes.

## Known environment constraint

The GitHub connector authorization does not provide credentials to local Git. Local commits work normally, but HTTPS push requires a separately configured local Git credential, PAT, SSH key, or authenticated GitHub CLI.
