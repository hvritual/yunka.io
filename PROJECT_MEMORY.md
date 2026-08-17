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
- Local GitHub account: `hvritual`
- Local Git transport: HTTPS

## Working decisions

### 2026-08-17 — Canonical task repository

All subsequent project tasks must use this repository as their source of truth and working scope. Project code, documentation, plans, and durable decisions belong in this repository unless the user explicitly changes that scope.

### 2026-08-17 — Local Git commits only

All work that requires a Git commit must be committed with the local `git` command-line tool. GitHub connector or API operations must not be used to construct commits or modify Git objects and refs.

The GitHub connector remains appropriate for reading repository metadata and collaborating through issues, pull requests, reviews, and permissions.

### 2026-08-17 — Mandatory memory read

Every new task must read `AGENTS.md` and this file before planning, analysis, edits, or task-specific commands. The task must also verify the current branch, working-tree status, and configured remotes before making changes.

### 2026-08-17 — Local GitHub authorization and reuse

Local GitHub authorization is established for account `hvritual` with the minimum `repo` OAuth scope required to read and write the private repository. Local Git fetch and push must reuse the checkout's configured HTTPS credential helper.

Current workspace tooling:

- GitHub CLI version: `2.45.0`
- GitHub CLI binary: `/workspace/scratch/7cbedf9749a1/tools/gh/usr/bin/gh`
- GitHub CLI wrapper: `/workspace/scratch/7cbedf9749a1/tools/bin/gh`
- GitHub CLI config: `/workspace/scratch/7cbedf9749a1/tools/gh-config`
- Local Git credential file: `/workspace/scratch/7cbedf9749a1/tools/git-credentials`
- Credential file permission: `0600`
- Required network endpoints: `github.com:443` and `api.github.com:443`

The credential and OAuth token are environment-local secrets. Never print, inspect, copy into project files, include in logs, commit, or expose them through tool output. Repository memory records only non-secret metadata. If the environment-local paths no longer exist, reauthenticate instead of reconstructing or requesting the token in chat.

Use the configured local `git` commands for fetch, merge, commit, and push. The local Git credential helper is the source of truth for repository authentication. The `gh` wrapper path is available for CLI execution, but its authentication must be checked separately and its permissions must not be broadened automatically beyond the repository operation requested by the user.

## Connection boundary

GitHub connector authorization and local Git authorization are separate. The connector remains available for repository collaboration operations, while local Git uses the environment-local credential described above.
