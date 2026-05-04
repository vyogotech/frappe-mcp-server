# frappe-mcp-server: feature/mcp-go-sdk \u2192 main merge implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land `feature/mcp-go-sdk` (HEAD `9a35707`) onto `main` with the one main-only commit cherry-picked forward and the audit-flagged BLOCKER + HIGH + MEDIUM fixes applied, then bump the parent `frappe-ai-assistant` submodule pin.

**Architecture:** Strategy B from the design spec \u2014 branch off feature, cherry-pick main's one commit not already covered (`121eddf` rate-limit defaults), apply 13 granular fix commits (one per audit item, TDD where it fits), push the merge branch for CI dry-run, then fast-forward `main` and bump the parent pin.

**Tech Stack:** Go 1.25, modelcontextprotocol/go-sdk v1.4.0, neo4j-go-driver v5, golangci-lint v2, gosec, OpenTelemetry, slog.

**Spec:** `docs/superpowers/specs/2026-04-27-frappe-mcp-server-merge-design.md`

**Working directory for all commands:** `/Users/sarathi/Documents/GitHub/frappe-ai-assistant/submodules/frappe-mcp-server` unless explicitly stated otherwise.

---

## File structure

| File | Action | Why |
|---|---|---|
| `internal/server/server.go` | Modify | recursion fix, hide 3 PM tools, hardcoded count |
| `.golangci.yml` | Modify | enable baseline linters |
| `internal/frappe/client.go` | Modify | redact CSRF/api-key logs |
| `internal/auth/strategies/oauth2.go` | Modify | replace `log.Printf` with `slog.Debug` |
| `internal/tools/registry.go` | Modify | budget_variance sum fix |
| `internal/config/config.go` | Modify | ERPNEXT_* env compat shim |
| `internal/server/sse.go` | Modify | ctx.Done watchdog |
| `internal/neo4j/client.go` | Modify | docstring clarification |
| `internal/frappe/client_test.go` | Modify | add sid/CSRF integration test |
| `init__.py` | Delete | stray 0-byte misnamed file |
| `internal/config/config.go` | Modify (cherry-pick) | rate-limit defaults |

---

## Task A1: Pre-flight verification

**Files:** none (verification only)

- [ ] **Step 1: Verify working directory and clean tree**

Run:
```bash
cd /Users/sarathi/Documents/GitHub/frappe-ai-assistant/submodules/frappe-mcp-server
git status
```
Expected: branch `feature/mcp-go-sdk`, working tree clean (the spec commit `9a35707` should be the local tip, possibly ahead of `origin/feature/mcp-go-sdk`).

- [ ] **Step 2: Fetch latest refs**

Run:
```bash
git fetch --all
```
Expected: success, may report new refs.

- [ ] **Step 3: Verify branch SHAs**

Run:
```bash
git log --oneline -1 origin/main
git log --oneline -1 HEAD
git merge-base origin/main HEAD
```
Expected:
- `origin/main` \u2192 `160f3d3` (or newer; if drifted, abort and re-plan)
- `HEAD` \u2192 `9a35707` or newer
- merge-base \u2192 `ed4a532`

- [ ] **Step 4: Verify cherry-pick targets exist**

Run:
```bash
git cat-file -e 121eddf && echo \