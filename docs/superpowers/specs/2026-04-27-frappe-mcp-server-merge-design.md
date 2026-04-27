# frappe-mcp-server: `feature/mcp-go-sdk` → `main` merge design

**Date:** 2026-04-27
**Author:** Sarathi
**Status:** Draft, pending user review
**Repos:** `submodules/frappe-mcp-server` (work executed here); `frappe-ai-assistant` (parent submodule pointer)

---

## 1. Context

`feature/mcp-go-sdk` (HEAD `da2f382`) is 34 commits ahead of `origin/main` (`160f3d3`); main has 6 commits not in feature. Common ancestor `ed4a532`. Feature branch carries the SDK migration, OAuth/sid hybrid auth, SSE streaming, OpenTelemetry, FrappeForge graph intelligence + Neo4j, gosec fixes, Go 1.25 + golangci-lint v2, and a number of LLM/chat quality improvements. Main's six commits are surgical: rate-limit defaults, an `ff_get_doctype_blueprint` tool, optional API key validation, docker CI, and PR #5 (streamable HTTP/OAuth/global_search).

The parent repo `frappe-ai-assistant` pins this submodule at `da2f382`. Strategy choices that rewrite SHAs would invalidate that pin.

## 2. Goals

- Land all feature-branch work on `main` without losing any feature from either branch.
- Resolve the 5 predicted merge conflicts deliberately.
- Carry the two main-only commits that are not already covered by feature (`121eddf` rate-limit defaults; `160f3d3` `ff_get_doctype_blueprint`).
- Fix the BLOCKER bugs and HIGH-severity issues found by the audit before main moves.
- Preserve `da2f382` SHA so the parent submodule pin remains valid until intentionally bumped.

## 3. Non-goals (Phase 2)

- Implementing the 8 stub/partial Project Management tools (`analyze_project_timeline`, `calculate_project_metrics`, `get_resource_allocation`, `project_risk_assessment`, `generate_project_report`, `portfolio_dashboard`, `resource_utilization_analysis`, plus a polish pass on `budget_variance_analysis` beyond the trivial sum). These are pre-existing baseline code from `6a21a6e` (2025-11-12), unchanged on both branches; the merge does not introduce or worsen them. They will be addressed in a separate brainstorm/spec/plan cycle.
- Changes to the LLM Manager `persist` semantics (TODO at `internal/llm/manager.go:308`).
- Replacing the manual flag parser in `cmd/ollama-client/main.go:789-859`.
- Improvements to the Neo4j client beyond doc clarification.

## 4. Branch comparison summary

**No deletions in either direction.** `git diff --name-status origin/main...HEAD | grep ^D` is empty. Every change is `A` (added) or `M` (modified). Feature branch is functionally a strict superset of main except for the two main-only commits below.

### Main-only commits — disposition
| Commit | Subject | Disposition |
|---|---|---|
| `121eddf` | Default rate limits in config loader | **Cherry-pick** — not present on feature; without it `rate.NewLimiter(0,0)` blocks all requests. |
| `160f3d3` | `ff_get_doctype_blueprint` composite tool | **Cherry-pick** — net-new tool absent from feature. |
| `f429e36` | Test config validation for optional API keys | Skip — feature's variant subsumes; `config_test.go` auto-merges. |
| `ab0db47` | Optional API key/secret in config validation | Skip — feature's hybrid auth validation subsumes. |
| `613a5a8` | Docker build/push CI | Skip — byte-identical to feature's `baacaad`. |
| `deb203d` | Streamable HTTP / OAuth / global_search (PR #5) | Skip — equivalent functionality on feature via `4ad36c3`, `675ddab`, `internal/auth/`. |

### Tool catalog effect on main after merge
Net **+11 tools**: 10 FrappeForge tools from feature (`ff_graph_stats`, `ff_list_ingested_projects`, `ff_search_doctype`, `ff_get_doctype_detail`, `ff_get_doctype_controllers`, `ff_get_doctype_client_scripts`, `ff_find_doctypes_with_field`, `ff_get_doctype_links`, `ff_search_methods`, `ff_get_hooks`) plus `ff_get_doctype_blueprint` cherry-picked from main. All real, all Neo4j-backed with nil-safe degradation.

## 5. Merge strategy: B (cherry-pick onto feature, fast-forward main)

**Why B:**
- Preserves all 34 feature SHAs including `da2f382`. The parent repo's submodule pin stays valid until step 7 deliberately bumps it.
- Conflicts resolved on the feature branch where tests run before main moves at all.
- Linear history on main with no merge commit noise.
- A and C considered and rejected: A produces a merge commit but resolves conflicts on main itself (riskier); C squashes 34 commits into one and loses granular intent (gosec fix, sid auth, SSE, etc.); D rebases and rewrites SHAs, breaking the parent pin.

## 6. Conflict resolution playbook

| File | Type | Resolution |
|---|---|---|
| `.github/workflows/ci.yml` | content | KEEP-MAIN — drop `feature/mcp-go-sdk` from container-push allowlist (branch will no longer exist). |
| `internal/config/config.go` | content | UNION-BOTH — keep feature's hybrid validation; insert main's rate-limit defaults inside `Load()` before validation. |
| `internal/server/server.go` | content | MANUAL — keep feature as base; insert main's 4 blueprint snippets at: `toolCatalog()` map, `registerTools()` `reg(...)`, `listTools()` `order` slice, `handleToolCall()` `case` branch. |
| `internal/tools/frappeforge.go` | add/add | KEEP-MAIN — strict superset (adds `FfGetDoctypeBlueprint` method to identical 285-line base). |
| `internal/tools/frappeforge_test.go` | add/add | KEEP-MAIN — adds 2 tests for blueprint to identical 177-line base. |
| `internal/config/config_test.go` | auto-merge | No action — git auto-resolves. |

**Verification per file:**
- `ci.yml`: `grep -c 'refs/heads/feature/mcp-go-sdk' .github/workflows/ci.yml` → 0.
- `config.go`: `grep -A2 'RequestsPerSecond == 0' internal/config/config.go` shows the default-10 line; `go test ./internal/config/...` passes.
- `server.go`: `grep -c FfGetDoctypeBlueprint internal/server/server.go` → 2; `grep -c ff_get_doctype_blueprint internal/server/server.go` → 4 (catalog + reg + listing + dispatch).
- `frappeforge.go` / `frappeforge_test.go`: `diff <(git show origin/main:<path>) <path>` → empty; `go test ./internal/tools/... -run TestFfGetDoctypeBlueprint` passes 2 tests.

## 7. Pre-merge fixes (Tier 3 scope)

One commit per fix on `merge/main-into-feature` after the cherry-picks land — granular revertability. Each commit must build and (where applicable) pass `go test ./<package>` before the next is started. Severity per the audit.

### BLOCKER (must fix)
1. **`internal/server/server.go:46` — `generateWithLLM` self-recursion.** When `llmManager == nil` and `llmClient != nil`, the function recursively calls itself instead of `s.llmClient.Generate(ctx, prompt)`. Stack overflow on first chat request in legacy-client configuration. One-line fix.
2. **`.golangci.yml:1-4` — empty linter config.** 4-line stub with no `linters.enable`. CI's lint job is currently a no-op. Enable baseline ruleset: `errcheck, govet, ineffassign, staticcheck, unused, gosec`.

### HIGH (must fix)
3. **`internal/frappe/client.go:438,445,461-465` — INFO-level CSRF/api-key logging.** Drop to `slog.Debug`; redact values to `len()` or last-4. Add a `make audit-logs` grep gate in CI for `api_key|api_secret|csrf_token` strings outside test files.
4. **`internal/auth/strategies/oauth2.go:83,91,275` — `log.Printf` debug leaks.** Replace with `slog.Debug`. Bypasses operator log routing today.
5. **PM tool stubs disabled in production.** For the three fully-fabricated tools — `calculate_project_metrics`, `project_risk_assessment`, `portfolio_dashboard` — remove them from every wiring site in `internal/server/server.go`:
   - `toolCatalog()` map (so `tools/list` no longer advertises schemas)
   - `listTools()` `order` slice (REST listing skips them)
   - `registerTools()` `reg(...)` call (MCP server doesn't expose them)
   - intent-routing case (`server.go:2239` area — natural-language queries no longer normalize to these names)
   - intent-dispatch switches (`server.go:689-698`, `server.go:2293-2305`)

   **Keep** the function definitions in `internal/tools/registry.go` — Phase 2 will rewrite their bodies, not their signatures. The remaining 5 partial PM tools (`analyze_project_timeline`, `get_resource_allocation`, `generate_project_report`, `resource_utilization_analysis`, plus the polished `budget_variance_analysis`) stay visible — they at least return real underlying Frappe data alongside flagged-as-incomplete computed metrics. `get_project_status` and `analyze_document` are real and stay.

   Verification: `grep -E "calculate_project_metrics|project_risk_assessment|portfolio_dashboard" internal/server/server.go` returns 0 matches.
6. **Quick-fix `budget_variance_analysis` (`internal/tools/registry.go:760+`)** — replace the three `0.0` TODOs with real sums:
   ```go
   for _, p := range projects.Data {
       if b, ok := p["total_budget"].(float64); ok { totalBudget += b }
       if a, ok := p["actual_cost"].(float64); ok { totalActual += a }
   }
   summary["total_budget"] = totalBudget
   summary["total_actual"] = totalActual
   summary["overall_variance"] = totalBudget - totalActual
   ```
7. **ERPNEXT_* → FRAPPE_* env compat shim** in `internal/config/config.go:182-200`. For each renamed var, fall back to the old name and emit a `slog.Warn` once: `os.Getenv("FRAPPE_BASE_URL"); if "" { fallback to ERPNEXT_BASE_URL with deprecation warn }`. Avoids breaking operators upgrading from main.

### MEDIUM (cleanup)
8. **`internal/server/sse.go:62-66` — `emit()` swallows write errors.** Add `select { case <-ctx.Done(): return ... }` watchdog and `slog.Warn` on Marshal/Fprintf error. Prevents goroutine leak on client disconnect.
9. **`internal/neo4j/client.go:1-4`** — replace "fault-tolerant" docstring with "optional / nil-safe". Don't claim resilience the driver doesn't provide.
10. **`internal/server/server.go:578` hardcoded `count, 19`** — replace with actual count, e.g. `len(catalog) + legacyCount` or a counter. Currently misreports tool count by 10.
11. **`init__.py`** — remove the 0-byte misnamed file at repo root.

### Tests added
12. Sid/CSRF auth flow integration test — exercise `/api/v1/chat` POST with a `Cookie: sid=…` header, assert `X-Frappe-CSRF-Token` is set on outbound Frappe HTTP. Goes in `internal/frappe/client_test.go` (or an auth integration test file).
13. FrappeForge happy-path test — at minimum stub the `Query` method on `*neo4j.Client` so we can assert the Cypher generation for one tool. Goes in `internal/tools/frappeforge_test.go`.

## 8. Merge sequence (concrete commands)

### Step 1 — Pre-flight
```bash
cd /Users/sarathi/Documents/GitHub/frappe-ai-assistant/submodules/frappe-mcp-server
git status                                  # must be clean
git fetch --all
git log --oneline ed4a532..origin/main      # 6 commits expected
git log --oneline ed4a532..HEAD             # 34 commits expected
```

### Step 2 — Branch off feature (preserves SHAs)
```bash
git checkout -b merge/main-into-feature feature/mcp-go-sdk
```

### Step 3 — Cherry-pick the two commits not already covered
```bash
# A. Rate-limit defaults — touches only config.go; expect no conflict
git cherry-pick 121eddf
go build ./... && go test ./internal/config/...

# B. Blueprint tool — touches frappeforge.go, frappeforge_test.go, server.go
git cherry-pick 160f3d3
# Conflicts resolved per §6:
git checkout --theirs internal/tools/frappeforge.go
git checkout --theirs internal/tools/frappeforge_test.go
# server.go: manual edit per §6 — keep feature, add 4 blueprint snippets
git add internal/tools/frappeforge.go internal/tools/frappeforge_test.go internal/server/server.go
git cherry-pick --continue
go build ./... && go test ./internal/tools/...
```

### Step 4 — Apply pre-merge fixes (one commit per fix, scope §7)

Order BLOCKER → HIGH → MEDIUM → Tests. After each commit, run `go build ./...` and the targeted package test.

```bash
# 4.1 — BLOCKER §7.1 generateWithLLM recursion
$EDITOR internal/server/server.go
go build ./... && go test ./internal/server/...
git add internal/server/server.go
git commit -m "fix(server): break generateWithLLM self-recursion in legacy-client fallback"

# 4.2 — BLOCKER §7.2 golangci-lint config
$EDITOR .golangci.yml
golangci-lint run --timeout=3m
git add .golangci.yml
git commit -m "ci(lint): enable baseline linters (errcheck/govet/ineffassign/staticcheck/unused/gosec)"

# 4.3 — HIGH §7.3 redact CSRF/api-key logs
$EDITOR internal/frappe/client.go
go test ./internal/frappe/...
git add internal/frappe/client.go
git commit -m "fix(frappe): demote CSRF/api-key INFO logs to DEBUG and redact values"

# 4.4 — HIGH §7.4 oauth2 log.Printf → slog
$EDITOR internal/auth/strategies/oauth2.go
go test ./internal/auth/...
git add internal/auth/strategies/oauth2.go
git commit -m "fix(auth): replace stdlib log.Printf with slog.Debug in oauth2 strategy"

# 4.5 — HIGH §7.5 disable fabricated PM tools
$EDITOR internal/server/server.go
go build ./... && go test ./internal/server/... ./internal/tools/...
git add internal/server/server.go
git commit -m "feat(tools): hide fabricated PM tools (calculate_project_metrics, project_risk_assessment, portfolio_dashboard) until Phase 2"

# 4.6 — HIGH §7.6 budget_variance_analysis sum fix
$EDITOR internal/tools/registry.go
go test ./internal/tools/...
git add internal/tools/registry.go
git commit -m "fix(tools): compute real total_budget/total_actual/overall_variance in budget_variance_analysis"

# 4.7 — HIGH §7.7 ERPNEXT_* env compat shim
$EDITOR internal/config/config.go
go test ./internal/config/...
git add internal/config/config.go
git commit -m "feat(config): add ERPNEXT_* → FRAPPE_* env fallback with deprecation warn"

# 4.8 — MEDIUM §7.8 SSE ctx.Done watchdog
$EDITOR internal/server/sse.go
go test ./internal/server/...
git add internal/server/sse.go
git commit -m "fix(sse): observe ctx.Done() and log emit() write errors"

# 4.9 — MEDIUM §7.9 Neo4j docstring
$EDITOR internal/neo4j/client.go
git add internal/neo4j/client.go
git commit -m "docs(neo4j): clarify client is nil-safe, not fault-tolerant"

# 4.10 — MEDIUM §7.10 server.go hardcoded count
$EDITOR internal/server/server.go
go test ./internal/server/...
git add internal/server/server.go
git commit -m "fix(server): replace hardcoded tool count with computed value"

# 4.11 — MEDIUM §7.11 remove init__.py
git rm init__.py
git commit -m "chore: remove stray 0-byte init__.py at repo root"

# 4.12 — Tests §7.12 sid/CSRF auth flow integration test
$EDITOR internal/frappe/client_test.go
go test ./internal/frappe/...
git add internal/frappe/client_test.go
git commit -m "test(frappe): cover sid cookie + CSRF-from-desk flow on writes"

# 4.13 — Tests §7.13 frappeforge happy-path test
$EDITOR internal/tools/frappeforge_test.go
go test ./internal/tools/...
git add internal/tools/frappeforge_test.go
git commit -m "test(frappeforge): cover happy-path Cypher generation with stub Query"
```

### Step 5 — Local verification gates
```bash
go vet ./...
go test -race -count=1 ./...
golangci-lint run --timeout=3m
gosec ./... 2>/dev/null || echo "gosec not installed; CI will run it"
make build && make build-linux
docker build -t frappe-mcp-server:test .
# Boot smoke: serve, hit GET /api/v1/tools, expect ff_get_doctype_blueprint visible
# Boot smoke: tools/list MUST NOT contain calculate_project_metrics, project_risk_assessment, portfolio_dashboard
```

### Step 6 — Push merge branch for CI dry-run
```bash
git push -u origin merge/main-into-feature
# Wait for GitHub Actions: lint, test, gosec, container build all green.
# Inspect: gh pr view (or check repo Actions tab) — confirm matrix passes.
# Do NOT advance to step 7 until CI is green.
```

If CI flags issues local verification missed (golangci-lint v2 differences, race conditions, container build): fix on the merge branch, push, re-run CI, repeat. Do not delete the merge branch from origin until step 8 succeeds.

### Step 7 — Fast-forward main
```bash
git checkout main
git pull --ff-only origin main              # confirm at 160f3d3, no drift
git merge --ff-only merge/main-into-feature
git push origin main
```

If FF refuses: origin/main has drifted. Rebase `merge/main-into-feature` onto the new origin/main, re-run §5 + §6, retry. Do not force-push.

### Step 8 — Update parent submodule pin
```bash
cd /Users/sarathi/Documents/GitHub/frappe-ai-assistant
git submodule update --remote submodules/frappe-mcp-server
git add submodules/frappe-mcp-server
git commit -m "chore(submodule): bump frappe-mcp-server to merged main"
git push origin main   # or current parent branch
```

### Step 9 — Cleanup
```bash
# Once parent points at the new submodule SHA and CI is green:
cd /Users/sarathi/Documents/GitHub/frappe-ai-assistant/submodules/frappe-mcp-server
git push origin :merge/main-into-feature      # delete remote merge branch
git branch -d merge/main-into-feature         # delete local
git branch -d feature/mcp-go-sdk              # if no longer needed
git push origin :feature/mcp-go-sdk           # delete remote feature branch
```

## 9. Rollback plan

| Failure point | Recovery |
|---|---|
| §3 cherry-pick conflict can't be resolved | `git cherry-pick --abort` returns to feature tip; investigate; retry. |
| §4 audit fix breaks tests | `git reset --hard HEAD~1` on the merge branch (granular per-fix commits make this surgical); re-apply fix narrower. |
| §5 local verification gate fails | Stay on `merge/main-into-feature`; do not push. Iterate. |
| §6 CI dry-run fails | Stay on the merge branch; fix on the branch; re-push; do not advance to §7 until green. |
| §7 fast-forward fails (drift) | Rebase merge branch onto new origin/main; rerun §5 + §6. |
| §8 parent build breaks after submodule bump | In parent: `git checkout HEAD~1 -- submodules/frappe-mcp-server` restores the old pin (`da2f382`) without touching submodule state. |
| Post-deploy regression | Parent: revert submodule pin commit; submodule state on remote untouched; investigate before re-bumping. |

## 10. Post-merge verification

### Build & test
```bash
go vet ./...
go test -race -coverprofile=coverage.out ./...
golangci-lint run --timeout=3m
gosec ./...
make build-linux && make docker-build
```

### Per-feature smoke
| Feature | Probe | Expected |
|---|---|---|
| `ff_get_doctype_blueprint` | `tools/call` with `{"doctype":"Sales Invoice"}` | If neo4j down: "FrappeForge graph database is unavailable". If up: JSON with fields/controllers/hooks. |
| Streamable HTTP `/mcp` | `POST /mcp` with `tools/list` | Returns array including `ff_get_doctype_blueprint`; PM stubs (`calculate_project_metrics` etc.) absent. |
| Sid auth | `/api/v1/chat` with `Cookie: sid=…` | 200; outbound POST has `X-Frappe-CSRF-Token`; no token bytes in stdout logs. |
| OAuth Bearer | `/api/v1/chat` with `Authorization: Bearer …` | 200; "Using user OAuth2 token" log at debug. |
| `global_search` | chat: "search for 'invoice 12345'" | Routes to `global_search`. |
| Rate-limit defaults | Start with empty `rate_limit:` block | First request not blocked; client logs `RequestsPerSecond=10`, `Burst=20`. |
| ERPNEXT_* env shim | Boot with `ERPNEXT_BASE_URL=...` only | Server boots; deprecation warn logged. |
| OTel | `OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318` | Spans `tool.*`, child HTTP span, `tool.success` attribute. |

### Monitoring
- 24h soak in staging.
- First 72h in prod with on-call alerted; goroutine count growth, 5xx rate, Frappe instance 401/403 rate, log retention size.

## 11. Risk register (post-fix)

| # | Risk | L × I | Mitigation status |
|---|---|---|---|
| 1 | `generateWithLLM` recursion crashes server | M × H | Fixed in §7.1 |
| 2 | CSRF/api-key logging leaks | H × H | Fixed in §7.3 + CI grep gate |
| 3 | `tools/list` fabricated PM data | H × H | Hidden in §7.5; full impl Phase 2 |
| 4 | golangci no-op CI | H × M | Fixed in §7.2 |
| 5 | ERPNEXT_* → FRAPPE_* breaks operators | H × M | Shim added in §7.7 |
| 6 | SSE goroutine leak | M × M | Fixed in §7.8 |
| 7 | Submodule pin drift after merge | M × M | Step 7 documented; consider parent CI gate |
| 8 | Neo4j default container memory | M × M | Future patch — make `profiles: ["graph"]` opt-in |
| 9 | Manager `persist` TODO | L × M | Accepted as known-debt |
| 10 | Manual flag parser in ollama-client | L × L | Accepted as known-debt |

## 12. Out of scope (Phase 2 — separate spec)

After this merge lands, open a fresh brainstorm cycle for the PM tools project. Spec at `docs/superpowers/specs/<date>-pm-tools-implementation-design.md`. Scope:

- Implement `analyze_project_timeline` (critical path, milestones, timeline health from `Task.depends_on` and date fields)
- Implement `calculate_project_metrics` (BurnRate, Velocity, Efficiency, RiskScore, Health from `Project` + `Task` + `Timesheet Detail`)
- Implement `get_resource_allocation` and `resource_utilization_analysis` (Timesheet-based hours-per-employee, utilization vs Holiday List/working schedule)
- Implement `project_risk_assessment` (4-dimension risk + recommendations)
- Implement `generate_project_report` (compose outputs from above)
- Implement `portfolio_dashboard` (status counts + KPIs across projects)
- Polish `budget_variance_analysis` beyond the trivial sum (per-project variance flags, currency normalization)
- Tests with happy paths for each
- Re-add tools to `toolCatalog()` and `listTools()` order as each ships

Estimated effort: ~18-22 hours focused work. Best done as 8 small PRs, one per tool, each unhiding its tool from `tools/list` only after its tests pass.

## 13. Decisions

- **Spec location:** `docs/superpowers/specs/` in the submodule. Lands on `main` with the merge.
- **Pre-merge fixes:** one commit per fix (§7 / §8 step 4), granular revertability.
- **CI dry-run before main fast-forward:** yes — push `merge/main-into-feature` to origin, wait for GitHub Actions green, then advance to §8 step 7.
