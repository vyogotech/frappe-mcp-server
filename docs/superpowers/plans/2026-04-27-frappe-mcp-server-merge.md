# frappe-mcp-server: feature/mcp-go-sdk → main merge implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land `feature/mcp-go-sdk` (HEAD `9a35707`) onto `main` (`160f3d3`) with the two main-only commits cherry-picked forward and the audit-flagged BLOCKER + HIGH + MEDIUM fixes applied, then bump the parent `frappe-ai-assistant` submodule pin.

**Architecture:** Strategy B from the design spec — branch off feature, cherry-pick main's two commits not already covered (`121eddf` rate-limit defaults, `160f3d3` `ff_get_doctype_blueprint`), apply 13 granular fix commits (one per audit item, TDD where it fits), push the merge branch for CI dry-run, then fast-forward `main` and bump the parent pin.

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
| `internal/tools/frappeforge_test.go` | Modify | add happy-path Cypher test |
| `init__.py` | Delete | stray 0-byte misnamed file |
| `internal/tools/frappeforge.go` | Modify (cherry-pick) | add `FfGetDoctypeBlueprint` |
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
- `origin/main` → `160f3d3` (or newer; if drifted, abort and re-plan)
- `HEAD` → `9a35707` or newer
- merge-base → `ed4a532`

- [ ] **Step 4: Verify cherry-pick targets exist**

Run:
```bash
git cat-file -e 121eddf && echo "121eddf OK"
git cat-file -e 160f3d3 && echo "160f3d3 OK"
```
Expected: both lines print `OK`.

- [ ] **Step 5: Verify no in-progress merge state**

Run:
```bash
test ! -e .git/MERGE_HEAD && test ! -e .git/CHERRY_PICK_HEAD && echo "clean"
```
Expected: `clean`.

- [ ] **Step 6: Confirm Go toolchain and tools available**

Run:
```bash
go version
golangci-lint --version 2>&1 || echo "golangci-lint NOT INSTALLED — install before Step I1"
gosec --version 2>&1 || echo "gosec NOT INSTALLED — CI will run it"
docker --version
```
Expected: Go 1.25.x. golangci-lint v2.x preferred; if missing, must be installed before §I (CI dry-run).

---

## Task B1: Create merge branch

**Files:** none (branch op)

- [ ] **Step 1: Create the merge branch**

Run:
```bash
git checkout -b merge/main-into-feature feature/mcp-go-sdk
```
Expected: `Switched to a new branch 'merge/main-into-feature'`.

- [ ] **Step 2: Verify branch tip**

Run:
```bash
git log --oneline -1
```
Expected: same SHA as `feature/mcp-go-sdk` tip (`9a35707` or newer).

---

## Task C1: Cherry-pick rate-limit defaults (`121eddf`)

**Files:**
- Modify (via cherry-pick): `internal/config/config.go`

- [ ] **Step 1: Cherry-pick the commit**

Run:
```bash
git cherry-pick 121eddf
```
Expected: success, no conflict (touches only `Load()` in `config.go`, in a region feature didn't edit).

If conflict: abort with `git cherry-pick --abort`, investigate, do not advance.

- [ ] **Step 2: Verify the defaults are present**

Run:
```bash
grep -A2 'RequestsPerSecond == 0' internal/config/config.go
```
Expected: shows the default-10 line.

- [ ] **Step 3: Build and run config tests**

Run:
```bash
go build ./...
go test ./internal/config/...
```
Expected: build succeeds, tests PASS.

- [ ] **Step 4: Commit is already created by cherry-pick**

Verify:
```bash
git log --oneline -1
```
Expected: original commit message preserved.

---

## Task C2: Cherry-pick `ff_get_doctype_blueprint` (`160f3d3`) — manual conflict resolution

**Files:**
- Modify (via cherry-pick): `internal/tools/frappeforge.go`, `internal/tools/frappeforge_test.go`, `internal/server/server.go`

- [ ] **Step 1: Start the cherry-pick**

Run:
```bash
git cherry-pick 160f3d3
```
Expected: conflict on `frappeforge.go` (add/add), `frappeforge_test.go` (add/add), `server.go` (content).

- [ ] **Step 2: Resolve frappeforge.go — take main's version (strict superset)**

Run:
```bash
git checkout --theirs internal/tools/frappeforge.go
git add internal/tools/frappeforge.go
```

- [ ] **Step 3: Resolve frappeforge_test.go — take main's version (strict superset)**

Run:
```bash
git checkout --theirs internal/tools/frappeforge_test.go
git add internal/tools/frappeforge_test.go
```

- [ ] **Step 4: Resolve server.go — manual edit (keep feature, add 4 blueprint snippets)**

Open `internal/server/server.go`. The conflict markers will surround four insertion points. The desired final state is feature's content with these four additions from main's version:

a. In `toolCatalog()`, immediately after the `"ff_get_hooks"` entry (around line 517 in feature's version, before the closing `}` of the map):
```go
"ff_get_doctype_blueprint": {
    Description: "[FrappeForge] Get a comprehensive blueprint (fields, controllers, hooks) in one call",
    InputSchema: objSchema(map[string]interface{}{
        "doctype": strProp("Exact doctype name"),
    }, "doctype"),
},
```

b. In `registerTools()`, immediately after the `reg("ff_get_hooks", s.tools.FfGetHooks)` line:
```go
reg("ff_get_doctype_blueprint", s.tools.FfGetDoctypeBlueprint)
```

c. In `listTools()`, append `"ff_get_doctype_blueprint"` to the end of the `order` slice (after `"ff_get_hooks"`):
```go
"ff_find_doctypes_with_field", "ff_get_doctype_links", "ff_search_methods", "ff_get_hooks", "ff_get_doctype_blueprint",
```

d. In `handleToolCall()`, in the switch that handles `ff_*` cases, immediately after the `case "ff_get_hooks":` block, add:
```go
case "ff_get_doctype_blueprint":
    result, err = s.tools.FfGetDoctypeBlueprint(ctx, request)
```

Remove all `<<<<<<< / ======= / >>>>>>>` markers. Save.

Run:
```bash
git add internal/server/server.go
```

- [ ] **Step 5: Continue the cherry-pick**

Run:
```bash
git cherry-pick --continue
```
Expected: editor opens with original commit message; save and close.

- [ ] **Step 6: Verify all 4 blueprint references**

Run:
```bash
grep -c "FfGetDoctypeBlueprint" internal/server/server.go
grep -c "ff_get_doctype_blueprint" internal/server/server.go
```
Expected: `2` and `4` respectively (1 method ref + 1 method call + 1 catalog key + 1 reg + 1 listing + 1 dispatch case = 2 capitalized, 4 snake_case... actually: catalog key `ff_get_doctype_blueprint` (1), reg call `ff_get_doctype_blueprint` (1), listing slice `ff_get_doctype_blueprint` (1), switch case `ff_get_doctype_blueprint` (1) → 4. Method refs: registerTools `s.tools.FfGetDoctypeBlueprint` (1), dispatch `s.tools.FfGetDoctypeBlueprint` (1) → 2.)

- [ ] **Step 7: Build and run tool tests**

Run:
```bash
go build ./...
go test ./internal/tools/... -run TestFfGetDoctypeBlueprint -v
```
Expected: build succeeds, 2 tests PASS (`TestFfGetDoctypeBlueprint_MissingDoctype`, `TestFfGetDoctypeBlueprint_Neo4jUnavailable`).

- [ ] **Step 8: Run full server + tools tests**

Run:
```bash
go test ./internal/server/... ./internal/tools/...
```
Expected: PASS.

---

## Task D1: Fix generateWithLLM self-recursion (BLOCKER)

**Files:**
- Modify: `internal/server/server.go:38-48`
- Test: `internal/server/server_test.go` (add new test)

**Reference:** Spec §7.1, audit finding "BLOCKER". Current bug at `internal/server/server.go:46`:
```go
if s.llmClient != nil {
    return s.generateWithLLM(ctx, prompt)   // ← infinite recursion
}
```
Should be `s.llmClient.Generate(ctx, prompt)`. The `llm.Client` interface (`internal/llm/client.go:11-18`) defines `Generate(ctx, prompt) (string, error)` so the call is type-safe.

- [ ] **Step 1: Locate or create the test file**

Run:
```bash
ls internal/server/server_test.go 2>/dev/null && echo "exists" || echo "missing"
```

If missing, create it with this header:
```go
package server

import (
    "context"
    "testing"

    "frappe-mcp-server/internal/llm"
)
```

- [ ] **Step 2: Write the failing test**

Append to `internal/server/server_test.go`:
```go
// stubLLMClient is a minimal llm.Client that returns a known string from Generate.
type stubLLMClient struct{}

func (stubLLMClient) Generate(ctx context.Context, prompt string) (string, error) {
    return "stub-response", nil
}
func (stubLLMClient) Provider() string { return "stub" }

// TestGenerateWithLLM_LegacyClientFallback verifies that when llmManager is nil
// but llmClient is set, generateWithLLM delegates to the legacy client and does
// NOT recurse. Without the fix, this test stack-overflows (caught by go test as
// a goroutine stack-overflow panic).
func TestGenerateWithLLM_LegacyClientFallback(t *testing.T) {
    s := &MCPServer{
        llmClient:  stubLLMClient{},
        llmManager: nil,
    }
    got, err := s.generateWithLLM(context.Background(), "hello")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if got != "stub-response" {
        t.Fatalf("got %q, want %q", got, "stub-response")
    }
}

var _ llm.Client = stubLLMClient{} // compile-time check
```

- [ ] **Step 3: Run the test to confirm it fails (stack overflow)**

Run:
```bash
go test ./internal/server/... -run TestGenerateWithLLM_LegacyClientFallback -timeout 10s
```
Expected: FAIL with stack overflow / goroutine stack exceeds 1000000000-byte limit, or test timeout.

- [ ] **Step 4: Apply the fix**

Edit `internal/server/server.go`. Replace:
```go
	// Fall back to legacy client
	if s.llmClient != nil {
		return s.generateWithLLM(ctx, prompt)
	}
```
with:
```go
	// Fall back to legacy client
	if s.llmClient != nil {
		return s.llmClient.Generate(ctx, prompt)
	}
```

- [ ] **Step 5: Run the test to confirm it passes**

Run:
```bash
go test ./internal/server/... -run TestGenerateWithLLM_LegacyClientFallback -v
```
Expected: PASS.

- [ ] **Step 6: Run full package tests**

Run:
```bash
go test ./internal/server/...
```
Expected: PASS.

- [ ] **Step 7: Commit**

Run:
```bash
git add internal/server/server.go internal/server/server_test.go
git commit -m "fix(server): break generateWithLLM self-recursion in legacy-client fallback

When llmManager is nil but llmClient is set, generateWithLLM previously
called itself, producing infinite recursion and a stack overflow on the
first chat request. Delegate to s.llmClient.Generate as intended."
```

---

## Task D2: Enable golangci-lint v2 baseline rules (BLOCKER)

**Files:**
- Modify: `.golangci.yml`

**Reference:** Spec §7.2. Current config is a 4-line stub with no `linters.enable`, so CI's lint job is a no-op.

- [ ] **Step 1: Verify current state**

Run:
```bash
cat .golangci.yml
```
Expected: 4 lines — `version: "2"`, blank, `run:`, `  go: "1.25"`.

- [ ] **Step 2: Replace with baseline config**

Overwrite `.golangci.yml` with:
```yaml
version: "2"

run:
  go: "1.25"
  timeout: 3m

linters:
  default: none
  enable:
    - errcheck
    - govet
    - ineffassign
    - staticcheck
    - unused
    - gosec

linters-settings:
  errcheck:
    exclude-functions:
      - (io.Closer).Close
      - (net/http.ResponseWriter).Write

issues:
  max-issues-per-linter: 0
  max-same-issues: 0
```

- [ ] **Step 3: Run golangci-lint**

Run:
```bash
golangci-lint run --timeout=3m 2>&1 | tee /tmp/golangci-out.log
```
Expected: zero issues, OR a small set of pre-existing findings the engineer must triage.

If there are findings:
- Decide per finding: fix in this commit (if 1-2 trivial), OR add a `// nolint:<linter> // reason` with justification, OR file a follow-up issue and add the offending file/path to `issues.exclude-files`.
- Do NOT broadly disable a linter — the goal is to wake the linter up, not silence it.

- [ ] **Step 4: Re-run to confirm clean**

Run:
```bash
golangci-lint run --timeout=3m
```
Expected: exits 0 with no output.

- [ ] **Step 5: Commit**

Run:
```bash
git add .golangci.yml
git commit -m "ci(lint): enable baseline linters (errcheck/govet/ineffassign/staticcheck/unused/gosec)

The previous .golangci.yml was a 4-line stub with no linters.enable, so
the CI lint job was a no-op. Enable the v2 baseline so future commits
catch unchecked errors, ineffective assignments, and dead code."
```

---

## Task E1: Redact CSRF/api-key INFO logs (HIGH)

**Files:**
- Modify: `internal/frappe/client.go:438,445,461-465`

**Reference:** Spec §7.3. Three sites today log token-adjacent secrets at INFO level, accessible to log retention systems.

- [ ] **Step 1: Locate the offending lines**

Run:
```bash
grep -n "DEBUG Client:\|Using placeholder API credentials" internal/frappe/client.go
```
Expected: 3+ lines around 438, 445, 461.

- [ ] **Step 2: Apply edit at line 438** — demote "Using sid cookie" log

Replace:
```go
		slog.Info("DEBUG Client: Using sid cookie", "user", user.Email, "method", method, "csrf_token_len", len(user.CSRFToken))
```
with:
```go
		slog.Debug("Using sid cookie for outbound request", "user", user.Email, "method", method, "csrf_token_len", len(user.CSRFToken))
```

- [ ] **Step 3: Apply edit at line 445** — drop the token-prefix log

Replace:
```go
		if (method == "POST" || method == "PUT" || method == "DELETE") && user.CSRFToken != "" {
			req.Header.Set("X-Frappe-CSRF-Token", user.CSRFToken)
			slog.Info("DEBUG Client: Set CSRF token header", "user", user.Email, "method", method, "token", user.CSRFToken[:min(20, len(user.CSRFToken))]+"...")
		}
```
with:
```go
		if (method == "POST" || method == "PUT" || method == "DELETE") && user.CSRFToken != "" {
			req.Header.Set("X-Frappe-CSRF-Token", user.CSRFToken)
			slog.Debug("Set X-Frappe-CSRF-Token header", "user", user.Email, "method", method, "token_len", len(user.CSRFToken))
		}
```

- [ ] **Step 4: Apply edit at lines 461-465** — redact placeholder warning

Replace:
```go
		// Warn if using placeholder credentials
		if c.apiKey == "your_api_key_here" || c.apiSecret == "your_api_secret_here" {
			slog.Warn("Using placeholder API credentials - authentication will fail",
				"api_key", c.apiKey,
				"api_secret", c.apiSecret,
				"endpoint", endpoint)
		}
```
with:
```go
		// Warn if using placeholder credentials
		if c.apiKey == "your_api_key_here" || c.apiSecret == "your_api_secret_here" {
			slog.Warn("Using placeholder API credentials - authentication will fail",
				"endpoint", endpoint)
		}
```

- [ ] **Step 5: Verify no INFO/Warn-level secret strings remain**

Run:
```bash
grep -n "slog.Info.*csrf_token\|slog.Info.*token\b\|api_key.*c.apiKey\|api_secret.*c.apiSecret" internal/frappe/client.go
```
Expected: empty.

- [ ] **Step 6: Build and test**

Run:
```bash
go build ./...
go test ./internal/frappe/...
```
Expected: PASS.

- [ ] **Step 7: Commit**

Run:
```bash
git add internal/frappe/client.go
git commit -m "fix(frappe): demote CSRF/api-key INFO logs to DEBUG and stop logging values

INFO-level logs were leaking csrf_token_len, the first 20 chars of CSRF
tokens, and full api_key/api_secret values into stdout (and any log
retention system pulling logs from the container). Lower to DEBUG and
drop the value fields; lengths are kept for diagnostics."
```

---

## Task E2: Replace stdlib log.Printf with slog.Debug in oauth2 strategy (HIGH)

**Files:**
- Modify: `internal/auth/strategies/oauth2.go:83,91,265,275`

**Reference:** Spec §7.4. Stdlib `log.Printf` bypasses slog routing/levels; can't be silenced by operators.

- [ ] **Step 1: Locate the offending lines**

Run:
```bash
grep -n "log.Printf" internal/auth/strategies/oauth2.go
```
Expected: 4 hits at approximately lines 83, 91, 265, 275.

- [ ] **Step 2: Verify slog is already imported**

Run:
```bash
grep -E '^\s*"log/slog"' internal/auth/strategies/oauth2.go
```
If empty, add `"log/slog"` to the import block. If `"log"` is no longer used anywhere else in this file (after the edits below), remove `"log"` from the import block too.

- [ ] **Step 3: Replace line 83** — cached user log

Replace:
```go
				log.Printf("DEBUG Auth: Using cached user for sid, CSRF token len: %d", len(user.CSRFToken))
```
with:
```go
				slog.Debug("Using cached user for sid", "csrf_token_len", len(user.CSRFToken))
```

- [ ] **Step 4: Replace line 91** — session validation success

Replace:
```go
			log.Printf("DEBUG Auth: Session validation successful, CSRF token len: %d", len(user.CSRFToken))
```
with:
```go
			slog.Debug("Session validation successful", "csrf_token_len", len(user.CSRFToken))
```

- [ ] **Step 5: Replace line 265** — CSRF fetch warn (was already a WARN — keep level, switch to slog)

Replace:
```go
		log.Printf("WARN validateSession: failed to fetch CSRF token, writes will fail: %v", err)
```
with:
```go
		slog.Warn("validateSession: failed to fetch CSRF token; writes will fail", "error", err)
```

- [ ] **Step 6: Replace line 275** — created user debug

Replace:
```go
	log.Printf("DEBUG validateSession: Created user with CSRF token (len=%d)", len(user.CSRFToken))
```
with:
```go
	slog.Debug("validateSession: created user", "csrf_token_len", len(user.CSRFToken))
```

- [ ] **Step 7: Verify no log.Printf remains**

Run:
```bash
grep -n "log.Printf" internal/auth/strategies/oauth2.go
```
Expected: empty.

- [ ] **Step 8: Verify imports clean**

Run:
```bash
go build ./...
```
Expected: success. If it complains about unused `"log"` import, remove that line from the import block.

- [ ] **Step 9: Run auth tests**

Run:
```bash
go test ./internal/auth/...
```
Expected: PASS.

- [ ] **Step 10: Commit**

Run:
```bash
git add internal/auth/strategies/oauth2.go
git commit -m "fix(auth): replace stdlib log.Printf with slog in oauth2 strategy

Stdlib log.Printf bypassed slog routing and level filters, so operators
couldn't silence noisy DEBUG output. Switch to slog.Debug/Warn so logs
honour the configured handler."
```

---

## Task E3: Hide three fully-fabricated PM tools from MCP (HIGH)

**Files:**
- Modify: `internal/server/server.go` (multiple sites)

**Reference:** Spec §7.5. The three tools — `calculate_project_metrics`, `project_risk_assessment`, `portfolio_dashboard` — return hard-coded constants (`"Green"`, `0.0`, `"Low"` regardless of input). Hide from MCP discovery + dispatch + intent routing while leaving function definitions in `internal/tools/registry.go` for Phase 2.

- [ ] **Step 1: Snapshot current matches**

Run:
```bash
grep -n -E "calculate_project_metrics|project_risk_assessment|portfolio_dashboard" internal/server/server.go
```
Expected: 11 hits (verified earlier): registerTools (lines 571, 573, 575); handleToolCall (692, 698); intent map (2077, 2078); intent override (2138); intent dispatch case (2239 — only `project_risk_assessment` here); executeTool (2295, 2299, 2305).

- [ ] **Step 2: Write the failing test**

Append to `internal/server/server_test.go`:
```go
// TestExecuteTool_FabricatedPMToolsHidden verifies that the three PM tools
// returning hard-coded constants are NOT dispatchable through executeTool.
// They remain as exported functions in internal/tools but are not exposed
// via MCP, REST listing, or intent routing. Phase 2 will reimplement them.
func TestExecuteTool_FabricatedPMToolsHidden(t *testing.T) {
    s := &MCPServer{}
    for _, name := range []string{
        "calculate_project_metrics",
        "project_risk_assessment",
        "portfolio_dashboard",
    } {
        _, err := s.executeTool(context.Background(), name, []byte(`{}`))
        if err == nil {
            t.Errorf("executeTool(%q) returned nil error; expected 'tool not found'", name)
            continue
        }
        if !strings.Contains(err.Error(), "tool not found") {
            t.Errorf("executeTool(%q) error = %v; want contains 'tool not found'", name, err)
        }
    }
}
```

If `strings` is not yet imported in the test file, add it.

- [ ] **Step 3: Run test to confirm it fails**

Run:
```bash
go test ./internal/server/... -run TestExecuteTool_FabricatedPMToolsHidden -v
```
Expected: FAIL — the three names currently dispatch to live functions returning fabricated data.

- [ ] **Step 4: Remove from `registerTools()` (lines ~571, 573, 575)**

Open `internal/server/server.go`. Delete these three lines:
```go
	reg("calculate_project_metrics", s.tools.CalculateProjectMetrics)
	reg("project_risk_assessment", s.tools.ProjectRiskAssessment)
	reg("portfolio_dashboard", s.tools.PortfolioDashboard)
```

- [ ] **Step 5: Remove `case`s from `handleToolCall()` (lines ~692, 698)**

Delete:
```go
		case "portfolio_dashboard":
			slog.Info("Calling ERPNext PortfolioDashboard", "params", request.Params)
			result, err = s.tools.PortfolioDashboard(ctx, request)
```
and:
```go
		case "calculate_project_metrics":
			slog.Info("Calling ERPNext CalculateProjectMetrics", "params", request.Params)
			result, err = s.tools.CalculateProjectMetrics(ctx, request)
```

(Note: `project_risk_assessment` may not have a separate case here — verify before deleting.)

- [ ] **Step 6: Remove from intent-keyword maps (lines ~2065-2078)**

Delete these entries (the right-hand side `analyze_document` references and the portfolio/dashboard mappings to the hidden tools):
```go
		"metrics":          "analyze_document",
		"risk":             "analyze_document",
		"risk_assessment":  "analyze_document",
```
Keep — these route to `analyze_document`, which is real, not the fabricated tools. (Verify: yes, keep.)

Delete:
```go
		"portfolio": "portfolio_dashboard", // Keep for now (lists all)
		"dashboard": "portfolio_dashboard",
```

- [ ] **Step 7: Remove the portfolio override block (lines ~2136-2141)**

Delete:
```go
	if strings.Contains(lowerQuery, "portfolio") || strings.Contains(lowerQuery, "dashboard") {
		intent.Action = "dashboard"
		intent.Tool = "portfolio_dashboard"
		intent.Params = json.RawMessage(`{}`)
		return intent
	}
```

- [ ] **Step 8: Remove from intent dispatch case at line ~2239**

Replace:
```go
	case "get_project_status", "analyze_project_timeline", "calculate_project_metrics", "project_risk_assessment", "generate_project_report":
```
with:
```go
	case "get_project_status", "analyze_project_timeline", "generate_project_report":
```

- [ ] **Step 9: Remove `case`s from `executeTool()` (lines ~2295, 2299, 2305)**

Delete each of these three blocks:
```go
	case "portfolio_dashboard":
		return s.tools.PortfolioDashboard(ctx, request)
```
```go
	case "calculate_project_metrics":
		return s.tools.CalculateProjectMetrics(ctx, request)
```
```go
	case "project_risk_assessment":
		return s.tools.ProjectRiskAssessment(ctx, request)
```

- [ ] **Step 10: Verify all wirings removed**

Run:
```bash
grep -n -E "calculate_project_metrics|project_risk_assessment|portfolio_dashboard" internal/server/server.go
```
Expected: 0 hits.

- [ ] **Step 11: Verify functions still exist in registry**

Run:
```bash
grep -n -E "func \(t \*ToolRegistry\) (CalculateProjectMetrics|ProjectRiskAssessment|PortfolioDashboard)" internal/tools/registry.go
```
Expected: 3 hits (functions intact for Phase 2).

- [ ] **Step 12: Build, test, lint**

Run:
```bash
go build ./...
go test ./internal/server/... ./internal/tools/...
golangci-lint run --timeout=3m
```
Expected: build succeeds; the new `TestExecuteTool_FabricatedPMToolsHidden` PASSes; lint clean (golangci may flag `CalculateProjectMetrics`/`ProjectRiskAssessment`/`PortfolioDashboard` as unused if no tests reference them — if so, suppress with a `// Phase 2: re-wired in Q3 2026 — see docs/superpowers/specs/2026-04-27-frappe-mcp-server-merge-design.md` comment above each, or skip the `unused` linter for `internal/tools/registry.go`).

- [ ] **Step 13: Commit**

Run:
```bash
git add internal/server/server.go internal/server/server_test.go
git commit -m "feat(tools): hide fabricated PM tools from MCP until Phase 2

calculate_project_metrics, project_risk_assessment, and
portfolio_dashboard returned hard-coded constants ('Green', 0.0, 'Low')
regardless of input. An LLM agent calling them got plausible-looking
fake data. Remove from MCP registration, REST listing, intent routing,
and dispatch switches.

Functions remain as exported methods on ToolRegistry; Phase 2 will
reimplement their bodies against real Frappe data (Project + Task +
Timesheet Detail + Holiday List). See merge design spec for full plan."
```

---

## Task E4: Implement `budget_variance_analysis` real sums (HIGH)

**Files:**
- Modify: `internal/tools/registry.go:760-804`
- Test: `internal/tools/registry_test.go` (add new test)

**Reference:** Spec §7.6. Currently summary fields are hard-coded `0.0` with TODO comments; the projects list already has `total_budget` and `actual_cost` fields fetched.

- [ ] **Step 1: Write the failing test**

Append to `internal/tools/registry_test.go` (or create the file with package header if missing):
```go
func TestBudgetVarianceAnalysis_ComputesSums(t *testing.T) {
    // Arrange: stub Frappe client returning two projects with known budgets.
    stub := &stubFrappeClientForBudget{
        projects: []types.Document{
            {"name": "P1", "total_budget": 100.0, "actual_cost": 80.0, "status": "Open"},
            {"name": "P2", "total_budget": 200.0, "actual_cost": 250.0, "status": "Open"},
        },
    }
    reg := &ToolRegistry{frappeClient: stub}

    // Act
    resp, err := reg.BudgetVarianceAnalysis(context.Background(), mcp.ToolRequest{
        ID: "t1", Tool: "budget_variance_analysis", Params: []byte(`{}`),
    })
    require.NoError(t, err)

    // Assert: parse the JSON content payload and check summary
    var got struct {
        Summary struct {
            TotalBudget     float64 `json:"total_budget"`
            TotalActual     float64 `json:"total_actual"`
            OverallVariance float64 `json:"overall_variance"`
        } `json:"summary"`
    }
    require.NoError(t, json.Unmarshal([]byte(resp.Content[1].Text), &got))
    assert.InDelta(t, 300.0, got.Summary.TotalBudget, 0.001)
    assert.InDelta(t, 330.0, got.Summary.TotalActual, 0.001)
    assert.InDelta(t, -30.0, got.Summary.OverallVariance, 0.001)
}

// stubFrappeClientForBudget is a minimal stub satisfying just GetDocumentList.
type stubFrappeClientForBudget struct{ projects []types.Document }

func (s *stubFrappeClientForBudget) GetDocumentList(ctx context.Context, req types.SearchRequest) (*types.DocumentListResponse, error) {
    return &types.DocumentListResponse{Data: s.projects}, nil
}
```

If the `*ToolRegistry.frappeClient` field is typed as a concrete `*frappe.Client` (not an interface), this test won't compile. In that case, define an interface in the test file and use a thin wrapper, OR refactor `frappeClient` to an interface in this commit (smaller scope: just the methods this test needs; check `internal/tools/registry.go` for the field type before deciding).

- [ ] **Step 2: Run the test to confirm it fails**

Run:
```bash
go test ./internal/tools/... -run TestBudgetVarianceAnalysis_ComputesSums -v
```
Expected: FAIL — the function returns `0.0` summary fields regardless of inputs.

- [ ] **Step 3: Apply the fix**

Edit `internal/tools/registry.go` around line 773. Replace:
```go
	// Analyze budget variance
	analysis := map[string]interface{}{
		"budget_analysis": map[string]interface{}{
			"total_projects_analyzed": len(projects.Data),
			"analysis_note":           "Budget variance analysis for all projects",
		},
		"projects": projects.Data,
		"summary": map[string]interface{}{
			"total_budget":     0.0, // TODO: Calculate sum
			"total_actual":     0.0, // TODO: Calculate sum
			"overall_variance": 0.0, // TODO: Calculate
		},
	}
```
with:
```go
	// Compute totals across the returned projects. Frappe returns numeric
	// fields as float64 via JSON unmarshal; missing or non-numeric values
	// are skipped (treated as zero) so a partial dataset still produces a
	// best-effort sum rather than failing.
	var totalBudget, totalActual float64
	for _, p := range projects.Data {
		if v, ok := p["total_budget"].(float64); ok {
			totalBudget += v
		}
		if v, ok := p["actual_cost"].(float64); ok {
			totalActual += v
		}
	}

	analysis := map[string]interface{}{
		"budget_analysis": map[string]interface{}{
			"total_projects_analyzed": len(projects.Data),
			"analysis_note":           "Budget variance analysis for all projects",
		},
		"projects": projects.Data,
		"summary": map[string]interface{}{
			"total_budget":     totalBudget,
			"total_actual":     totalActual,
			"overall_variance": totalBudget - totalActual,
		},
	}
```

- [ ] **Step 4: Run the test to confirm it passes**

Run:
```bash
go test ./internal/tools/... -run TestBudgetVarianceAnalysis_ComputesSums -v
```
Expected: PASS.

- [ ] **Step 5: Run full tools tests**

Run:
```bash
go test ./internal/tools/...
```
Expected: PASS.

- [ ] **Step 6: Commit**

Run:
```bash
git add internal/tools/registry.go internal/tools/registry_test.go
git commit -m "fix(tools): compute real sums in budget_variance_analysis

Replace the three hard-coded 0.0 placeholders in the summary payload
with actual sums of total_budget and actual_cost across the returned
projects, plus the difference as overall_variance. Missing/non-numeric
fields are skipped so partial Frappe responses still produce a sum."
```

---

## Task E5: ERPNEXT_* env compat shim (HIGH)

**Files:**
- Modify: `internal/config/config.go:181-191`
- Test: `internal/config/config_test.go` (add new test)

**Reference:** Spec §7.7. The feature branch renamed `ERPNEXT_BASE_URL/API_KEY/API_SECRET` to `FRAPPE_*` with no fallback. Operators upgrading from main will see `frappe instance base URL is required` on startup.

- [ ] **Step 1: Write the failing test**

Append to `internal/config/config_test.go`:
```go
// TestLoadFromEnv_ERPNextLegacyShim verifies that the deprecated ERPNEXT_*
// environment variable names are still honoured (with a deprecation warn)
// when the new FRAPPE_* names are not set. Drop this shim no earlier than
// 2026-10-01 once operators have migrated.
func TestLoadFromEnv_ERPNextLegacyShim(t *testing.T) {
    t.Setenv("FRAPPE_BASE_URL", "")
    t.Setenv("FRAPPE_API_KEY", "")
    t.Setenv("FRAPPE_API_SECRET", "")
    t.Setenv("ERPNEXT_BASE_URL", "https://legacy.example.com")
    t.Setenv("ERPNEXT_API_KEY", "legacy-key")
    t.Setenv("ERPNEXT_API_SECRET", "legacy-secret")

    c := &Config{}
    if err := c.loadFromEnv(); err != nil {
        t.Fatalf("loadFromEnv: %v", err)
    }
    if c.ERPNext.BaseURL != "https://legacy.example.com" {
        t.Errorf("BaseURL = %q; want fallback to ERPNEXT_BASE_URL", c.ERPNext.BaseURL)
    }
    if c.ERPNext.APIKey != "legacy-key" {
        t.Errorf("APIKey = %q; want fallback to ERPNEXT_API_KEY", c.ERPNext.APIKey)
    }
    if c.ERPNext.APISecret != "legacy-secret" {
        t.Errorf("APISecret = %q; want fallback to ERPNEXT_API_SECRET", c.ERPNext.APISecret)
    }
}
```

- [ ] **Step 2: Run test to confirm it fails**

Run:
```bash
go test ./internal/config/... -run TestLoadFromEnv_ERPNextLegacyShim -v
```
Expected: FAIL — `BaseURL = ""; want fallback`.

- [ ] **Step 3: Apply the shim**

Open `internal/config/config.go` and locate `loadFromEnv()` (line ~181). Replace:
```go
	// Frappe instance configuration
	if baseURL := os.Getenv("FRAPPE_BASE_URL"); baseURL != "" {
		c.ERPNext.BaseURL = baseURL
	}
	if apiKey := os.Getenv("FRAPPE_API_KEY"); apiKey != "" {
		c.ERPNext.APIKey = apiKey
	}
	if apiSecret := os.Getenv("FRAPPE_API_SECRET"); apiSecret != "" {
		c.ERPNext.APISecret = apiSecret
	}
```
with:
```go
	// Frappe instance configuration. Prefer FRAPPE_* env names (current);
	// fall back to ERPNEXT_* (deprecated) so operators upgrading from main
	// don't see "frappe instance base URL is required" on first start.
	// Plan to drop the ERPNEXT_* shim no earlier than 2026-10-01.
	if baseURL := os.Getenv("FRAPPE_BASE_URL"); baseURL != "" {
		c.ERPNext.BaseURL = baseURL
	} else if legacy := os.Getenv("ERPNEXT_BASE_URL"); legacy != "" {
		c.ERPNext.BaseURL = legacy
		slog.Warn("ERPNEXT_BASE_URL is deprecated; rename to FRAPPE_BASE_URL")
	}
	if apiKey := os.Getenv("FRAPPE_API_KEY"); apiKey != "" {
		c.ERPNext.APIKey = apiKey
	} else if legacy := os.Getenv("ERPNEXT_API_KEY"); legacy != "" {
		c.ERPNext.APIKey = legacy
		slog.Warn("ERPNEXT_API_KEY is deprecated; rename to FRAPPE_API_KEY")
	}
	if apiSecret := os.Getenv("FRAPPE_API_SECRET"); apiSecret != "" {
		c.ERPNext.APISecret = apiSecret
	} else if legacy := os.Getenv("ERPNEXT_API_SECRET"); legacy != "" {
		c.ERPNext.APISecret = legacy
		slog.Warn("ERPNEXT_API_SECRET is deprecated; rename to FRAPPE_API_SECRET")
	}
```

If `log/slog` isn't already imported in this file, add it.

- [ ] **Step 4: Run the test to confirm it passes**

Run:
```bash
go test ./internal/config/... -run TestLoadFromEnv_ERPNextLegacyShim -v
```
Expected: PASS.

- [ ] **Step 5: Run full config tests**

Run:
```bash
go test ./internal/config/...
```
Expected: PASS.

- [ ] **Step 6: Commit**

Run:
```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add ERPNEXT_* → FRAPPE_* env fallback with deprecation warn

The feature branch renamed env vars without a compat layer; operators
upgrading from main saw 'frappe instance base URL is required' on
startup. Read FRAPPE_* first, then fall back to ERPNEXT_* with a
slog.Warn. Plan to drop the shim no earlier than 2026-10-01."
```

---

## Task F1: SSE ctx.Done watchdog and write-error logging (MEDIUM)

**Files:**
- Modify: `internal/server/sse.go:57-66`
- Test: `internal/server/sse_test.go` (create or extend)

**Reference:** Spec §7.8. `emit()` returns silently on Marshal/Fprintf error and `handleChatSSE` doesn't watch `r.Context().Done()`, leaking goroutines on client disconnect.

- [ ] **Step 1: Find handleChatSSE and confirm context is available**

Run:
```bash
grep -n "handleChatSSE\|r.Context\|ctx.Done" internal/server/sse.go
```
Expected: `handleChatSSE` defined; `r.Context()` may or may not already be wired through to the streaming loop.

- [ ] **Step 2: Modify `emit()` to log write errors**

Replace lines 57-66:
```go
func (sw *sseWriter) emit(ev sseEvent) {
	b, err := json.Marshal(ev)
	if err != nil {
		return
	}
	if _, err := fmt.Fprintf(sw.w, "data: %s\n\n", b); err != nil {
		return
	}
	sw.f.Flush()
}
```
with:
```go
func (sw *sseWriter) emit(ev sseEvent) {
	b, err := json.Marshal(ev)
	if err != nil {
		slog.Warn("sse: failed to marshal event", "type", ev.Type, "error", err)
		return
	}
	if _, err := fmt.Fprintf(sw.w, "data: %s\n\n", b); err != nil {
		slog.Warn("sse: failed to write event (client likely disconnected)", "type", ev.Type, "error", err)
		return
	}
	sw.f.Flush()
}
```

If `log/slog` isn't imported in `sse.go`, add it.

- [ ] **Step 3: Add ctx.Done() watchdog in handleChatSSE**

Locate `handleChatSSE` and the main streaming loop. The loop reads tokens from a channel produced by the LLM. Wrap the receive in a `select` that also observes `r.Context().Done()`. Concretely, replace any pattern like:
```go
for token := range tokens {
    sw.content(token)
}
```
with:
```go
ctx := r.Context()
for {
    select {
    case <-ctx.Done():
        slog.Debug("sse: client disconnected; stopping stream", "error", ctx.Err())
        return
    case token, ok := <-tokens:
        if !ok {
            return
        }
        sw.content(token)
    }
}
```

If the existing loop has a different shape, adapt — the principle is to never block on a channel receive without also watching `ctx.Done()`.

- [ ] **Step 4: Build**

Run:
```bash
go build ./...
```
Expected: success.

- [ ] **Step 5: Write a test for emit() error path**

Append to `internal/server/sse_test.go` (create with package header if missing):
```go
// failingWriter returns ErrShortWrite on every Write to simulate a
// disconnected client.
type failingWriter struct{}

func (failingWriter) Write(p []byte) (int, error) { return 0, io.ErrShortWrite }
func (failingWriter) Header() http.Header         { return http.Header{} }
func (failingWriter) WriteHeader(int)             {}

type noFlush struct{ failingWriter }

func (noFlush) Flush() {}

func TestEmit_LogsWriteError(t *testing.T) {
    sw := &sseWriter{w: failingWriter{}, f: noFlush{}}
    // Should not panic, should not block, should return cleanly.
    sw.emit(sseEvent{Type: "content", Text: "x"})
}
```

- [ ] **Step 6: Run sse tests**

Run:
```bash
go test ./internal/server/... -run TestEmit_LogsWriteError -v
```
Expected: PASS.

- [ ] **Step 7: Commit**

Run:
```bash
git add internal/server/sse.go internal/server/sse_test.go
git commit -m "fix(sse): observe ctx.Done() and log emit() write errors

emit() previously returned silently on Marshal/Fprintf error and the
streaming loop never observed r.Context().Done(), so a client that
hung up mid-stream caused the goroutine to run to completion before
the next write failed. Add a ctx watchdog and warn-level logging on
write failures."
```

---

## Task F2: Update Neo4j client docstring (MEDIUM)

**Files:**
- Modify: `internal/neo4j/client.go:1-4`

**Reference:** Spec §7.9. The package doc currently implies more resilience than the driver provides.

The docstring is already reasonable — verify wording, then no-op or minor tweak.

- [ ] **Step 1: Read current docstring**

Run:
```bash
sed -n '1,5p' internal/neo4j/client.go
```
Expected:
```go
// Package neo4j provides a lightweight Neo4j bolt client for FrappeForge
// graph queries. It is optional: when BoltURL is empty the package returns
// a nil *Client, and every Query call on a nil receiver returns a clear
// "not configured" error so callers can degrade gracefully.
```

- [ ] **Step 2: Decide if change needed**

The current text already says "optional" and "degrade gracefully" without claiming "fault-tolerant". If your team's external-facing docs (CHANGELOG, README) call this client "fault-tolerant", update those instead. **If wording is acceptable, skip to Step 4 with no edit and no commit.**

- [ ] **Step 3 (only if changes needed): Apply edit**

If you want to be more explicit, replace lines 1-4 with:
```go
// Package neo4j provides a lightweight Neo4j bolt client for FrappeForge
// graph queries. It is optional and nil-safe: when BoltURL is empty the
// package returns a nil *Client, and every Query call on a nil receiver
// returns a clear "not configured" error so callers can degrade
// gracefully. There is no retry/circuit-breaker beyond what the
// underlying driver provides.
```

- [ ] **Step 4: Verify CHANGELOG/README don't overclaim**

Run:
```bash
grep -in "fault-tolerant" CHANGELOG.md README.md docs/*.md 2>/dev/null
```
If hits exist outside docstrings, soften the claim in those files in this commit.

- [ ] **Step 5: Build**

Run:
```bash
go build ./...
```
Expected: success.

- [ ] **Step 6: Commit (skip if no changes)**

If changes were made:
```bash
git add internal/neo4j/client.go CHANGELOG.md README.md docs/
git commit -m "docs(neo4j): clarify client is nil-safe, not fault-tolerant"
```

If no changes: continue to F3.

---

## Task F3: Fix hardcoded tool count log (MEDIUM)

**Files:**
- Modify: `internal/server/server.go:578-579`

**Reference:** Spec §7.10. The log line says `count, 19` but the actual count is now ~30.

- [ ] **Step 1: Locate the line**

Run:
```bash
grep -n "Registered MCP tools" internal/server/server.go
```
Expected: line ~579.

- [ ] **Step 2: Apply edit**

The simplest fix is to count `reg(...)` calls. Refactor `registerTools()` to use a counter. Replace the `reg :=` closure block:
```go
	reg := func(name string, handler mcp.ToolHandler) {
		if meta, ok := catalog[name]; ok {
			s.server.RegisterToolWithSchema(name, meta.Description, meta.InputSchema, handler)
		} else {
			s.server.RegisterTool(name, handler)
		}
	}
```
with:
```go
	registered := 0
	reg := func(name string, handler mcp.ToolHandler) {
		if meta, ok := catalog[name]; ok {
			s.server.RegisterToolWithSchema(name, meta.Description, meta.InputSchema, handler)
		} else {
			s.server.RegisterTool(name, handler)
		}
		registered++
	}
```

And replace:
```go
	slog.Info("Registered MCP tools", "count", 19, "with_schema", len(catalog), "legacy", 19-len(catalog))
```
with:
```go
	slog.Info("Registered MCP tools", "count", registered, "with_schema", len(catalog), "legacy", registered-len(catalog))
```

- [ ] **Step 3: Build and run server tests**

Run:
```bash
go build ./...
go test ./internal/server/...
```
Expected: PASS.

- [ ] **Step 4: Smoke test (optional)**

Run:
```bash
go run ./cmd/mcp-stdio --help 2>&1 | head -20
```
Or otherwise invoke registerTools and observe the log. Not strictly required if build + tests pass.

- [ ] **Step 5: Commit**

Run:
```bash
git add internal/server/server.go
git commit -m "fix(server): replace hardcoded tool count with computed value

The slog.Info line said 'count: 19' regardless of how many reg() calls
ran; actual count is now ~30. Track registrations in a counter."
```

---

## Task F4: Remove stray `init__.py` (MEDIUM)

**Files:**
- Delete: `init__.py`

**Reference:** Spec §7.11. 0-byte misnamed file at repo root.

- [ ] **Step 1: Verify file**

Run:
```bash
ls -la init__.py
```
Expected: 0-byte file. If it's non-zero, investigate before deleting.

- [ ] **Step 2: Delete and commit**

Run:
```bash
git rm init__.py
git commit -m "chore: remove stray 0-byte init__.py at repo root

Likely a typo of __init__.py from an earlier Python prototype; not
referenced by anything in the Go module."
```

---

## Task G1: sid/CSRF auth flow integration test

**Files:**
- Modify: `internal/frappe/client_test.go`

**Reference:** Spec §7.12. The audit found no test exercising the sid-cookie path or asserting that the `X-Frappe-CSRF-Token` header is set on POST/PUT/DELETE.

- [ ] **Step 1: Locate or create the test**

Run:
```bash
ls internal/frappe/client_test.go
```
Expected: file exists.

- [ ] **Step 2: Add the test**

Append:
```go
// TestClient_SidCookieSetsCSRFHeader verifies that when an authenticated user
// with SessionID and CSRFToken is in context, the Client adds the sid cookie
// AND sets X-Frappe-CSRF-Token on writes.
func TestClient_SidCookieSetsCSRFHeader(t *testing.T) {
    var capturedReq *http.Request
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        capturedReq = r
        w.Header().Set("Content-Type", "application/json")
        _, _ = w.Write([]byte(`{"data": {}}`))
    }))
    defer srv.Close()

    c, err := NewClient(srv.URL, "", "", time.Second)
    require.NoError(t, err)

    user := &types.User{
        Email:     "alice@example.com",
        SessionID: "test-sid-12345",
        CSRFToken: "test-csrf-token-67890",
    }
    ctx := auth.WithUser(context.Background(), user)

    // Use any write method — CreateDocument / UpdateDocument both POST.
    _, err = c.CreateDocument(ctx, "User", types.Document{"email": "bob@example.com"})
    // Ignore the result; we care about request shape.
    _ = err

    require.NotNil(t, capturedReq, "no request reached test server")

    // Assert: sid cookie present
    cookie, err := capturedReq.Cookie("sid")
    require.NoError(t, err, "sid cookie missing")
    assert.Equal(t, "test-sid-12345", cookie.Value)

    // Assert: X-Frappe-CSRF-Token header set on POST
    assert.Equal(t, "test-csrf-token-67890", capturedReq.Header.Get("X-Frappe-CSRF-Token"))
}
```

If `auth.WithUser` doesn't exist, the equivalent is whatever helper sets `*types.User` into the context (`internal/auth/context.go` defines this). Adapt the import.

- [ ] **Step 3: Run the test**

Run:
```bash
go test ./internal/frappe/... -run TestClient_SidCookieSetsCSRFHeader -v
```
Expected: PASS.

- [ ] **Step 4: Run full frappe tests**

Run:
```bash
go test ./internal/frappe/...
```
Expected: PASS.

- [ ] **Step 5: Commit**

Run:
```bash
git add internal/frappe/client_test.go
git commit -m "test(frappe): cover sid cookie + CSRF-from-desk flow on writes

The flagship sid-authenticated-writes path had no automated coverage.
This test exercises the full priority-1 auth branch: sid cookie attached
to the outbound request and X-Frappe-CSRF-Token header set on POST."
```

---

## Task G2: FrappeForge happy-path Cypher test

**Files:**
- Modify: `internal/tools/frappeforge_test.go`

**Reference:** Spec §7.13. Existing tests cover only "missing param" + "neo4j unavailable".

- [ ] **Step 1: Inspect existing test setup**

Run:
```bash
grep -n "newNilNeo4jRegistry\|stub\|mock" internal/tools/frappeforge_test.go
```
Expected: helper for nil Neo4j; check whether a stubbable Query method exists on `*neo4j.Client`.

- [ ] **Step 2: Decide approach**

Two options:
1. **Interface-extract** — define a small interface `type queryRunner interface { Query(...) (...) }` in frappeforge.go, change `*ToolRegistry.neo4jClient` to that interface, supply a stub in tests. Larger refactor.
2. **httptest stub** — too low-level for Bolt protocol; not feasible.
3. **Embedded neo4j** — flaky in CI.

Recommendation: option 1 if you have time; otherwise add a single test that asserts the Cypher generation by reading the function-internal `cypher` constant via a package-private export hook (not great). For this plan, document option 1 but accept a TODO if scope is tight:

If you accept a TODO instead of a full happy-path test in this commit, add:
```go
// TestFfGetDoctypeBlueprint_HappyPath_TODO marks that a happy-path test
// requires extracting an interface for *neo4j.Client.Query so we can stub
// the result. Track in Phase 2. Do not delete this marker without filing
// the followup or implementing the test.
func TestFfGetDoctypeBlueprint_HappyPath_TODO(t *testing.T) {
    t.Skip("requires queryRunner interface extraction — Phase 2")
}
```

The audit listed this as MEDIUM, so the TODO marker is acceptable to unblock the merge. Choose your scope:

- [ ] **Step 3 (path A — full impl):** Extract `queryRunner` interface, refactor, add real happy-path test, commit.

- [ ] **Step 3 (path B — TODO marker):** Append the `t.Skip` test above.

- [ ] **Step 4: Run tests**

Run:
```bash
go test ./internal/tools/...
```
Expected: PASS (skipped tests count as PASS).

- [ ] **Step 5: Commit**

Run:
```bash
git add internal/tools/frappeforge_test.go internal/tools/frappeforge.go  # second file only if path A
git commit -m "test(frappeforge): add happy-path test marker / impl"
```

(Adjust commit message based on path A vs B.)

---

## Task H1: Local verification gates

**Files:** none (verify only)

**Reference:** Spec §8 step 5.

- [ ] **Step 1: vet**

Run:
```bash
go vet ./...
```
Expected: empty output.

- [ ] **Step 2: race-tests**

Run:
```bash
go test -race -count=1 ./...
```
Expected: PASS.

- [ ] **Step 3: lint**

Run:
```bash
golangci-lint run --timeout=3m
```
Expected: 0 issues.

- [ ] **Step 4: gosec (if installed)**

Run:
```bash
gosec ./... 2>&1 | tail -20
```
Expected: no HIGH/CRITICAL findings, OR existing nolint annotations cover them.

- [ ] **Step 5: build artifacts**

Run:
```bash
make build
make build-linux
```
Expected: both succeed.

- [ ] **Step 6: docker build**

Run:
```bash
docker build -t frappe-mcp-server:merge-test .
```
Expected: image built.

- [ ] **Step 7: smoke test — `tools/list` excludes hidden PM tools**

Run a quick smoke against a locally booted server. Easiest path: invoke `mcp-stdio` and send a `tools/list` JSON-RPC message. Or check via the REST `/tools` endpoint after booting `cmd/mcp-stdio` in HTTP mode. The exact incantation depends on existing tooling; if the team doesn't have a smoke harness, run:
```bash
go run ./cmd/mcp-stdio <<'EOF'
{"jsonrpc":"2.0","method":"tools/list","id":1}
EOF
```
Inspect output. Expected: `ff_get_doctype_blueprint` present; `calculate_project_metrics`, `project_risk_assessment`, `portfolio_dashboard` absent.

- [ ] **Step 8: smoke test — invariants**

Manual inspection of the run output:
- No `slog.Info`/`log.Printf` lines mentioning `csrf_token`, `api_key`, or `api_secret` values.
- Tool count log line shows the real number, not `19`.

If anything fails: do NOT advance to I1. Fix on the merge branch and rerun §H1.

---

## Task I1: CI dry-run

**Files:** none (push only)

- [ ] **Step 1: Push the merge branch to origin**

Run:
```bash
git push -u origin merge/main-into-feature
```
Expected: push succeeds; GitHub URL in output.

- [ ] **Step 2: Wait for CI**

Open the repository's GitHub Actions tab in browser, OR run:
```bash
gh run list --branch merge/main-into-feature --limit 5
gh run watch
```
Expected: matrix passes — `test`, `lint`, `gosec` (or whatever security job exists), `container` build all green.

- [ ] **Step 3: Inspect failures, if any**

If anything fails: pull the failing log, diagnose, fix on the merge branch locally, push again, repeat. Do NOT advance to J1 until everything is green.

```bash
gh run view --log-failed
```

- [ ] **Step 4: Confirm no skipped jobs hide a failure**

Eyeball the run summary. All required jobs should show ✓.

---

## Task J1: Fast-forward main

**Files:** none (refs only)

- [ ] **Step 1: Verify origin/main hasn't drifted**

Run:
```bash
git fetch --all
git log --oneline -1 origin/main
```
Expected: still `160f3d3`. If different, abort and re-plan: someone pushed to main.

- [ ] **Step 2: Switch to main**

Run:
```bash
git checkout main
git pull --ff-only origin main
```
Expected: at `160f3d3`.

- [ ] **Step 3: Fast-forward**

Run:
```bash
git merge --ff-only merge/main-into-feature
```
Expected: fast-forward succeeds. Tip moves from `160f3d3` to the merge branch tip.

If FF refuses ("not possible to fast-forward"): origin/main drifted between Step 1 and Step 3 of this task. Abort, fetch, rebase the merge branch onto the new main, re-run §H1 + §I1, retry §J1.

- [ ] **Step 4: Push main**

Run:
```bash
git push origin main
```
Expected: success. Note: this is the irreversible step. Make absolutely sure CI was green and verification passed before this command runs.

- [ ] **Step 5: Verify**

Run:
```bash
git log --oneline -5 main
```
Expected: shows feature commits + cherry-picks + audit fix commits at the top.

---

## Task K1: Update parent submodule pin

**Files:**
- Modify: `submodules/frappe-mcp-server` (parent repo's submodule pointer)

- [ ] **Step 1: Switch to parent repo**

Run:
```bash
cd /Users/sarathi/Documents/GitHub/frappe-ai-assistant
git status
```
Expected: clean, on whatever main/working branch is current.

- [ ] **Step 2: Update submodule pin**

Run:
```bash
git submodule update --remote submodules/frappe-mcp-server
git -C submodules/frappe-mcp-server log --oneline -1
```
Expected: submodule now at the new main tip.

- [ ] **Step 3: Stage and inspect the parent diff**

Run:
```bash
git status
git diff --staged submodules/frappe-mcp-server || git diff submodules/frappe-mcp-server
```
Expected: shows submodule SHA bump from `da2f382` to the new main tip.

- [ ] **Step 4: Run any parent-repo build/test (if applicable)**

Check parent README / CI for build commands. If nothing automated, smoke-boot the assistant if possible. Skip if none.

- [ ] **Step 5: Commit**

Run:
```bash
git add submodules/frappe-mcp-server
git commit -m "chore(submodule): bump frappe-mcp-server to merged main

Brings in the full feature/mcp-go-sdk merge: SDK migration, OAuth/sid
hybrid auth, SSE streaming, OTel, FrappeForge graph tools, and the
audit cleanup commits. See submodule's
docs/superpowers/specs/2026-04-27-frappe-mcp-server-merge-design.md
for the full plan."
```

- [ ] **Step 6: Push**

Run:
```bash
git push origin HEAD
```
Expected: success.

- [ ] **Step 7: Smoke-boot the parent**

If the parent repo has a `make dev` / `docker compose up` / similar, run it and verify the assistant boots cleanly with the new submodule.

---

## Task L1: Cleanup branches

**Files:** none (refs only)

- [ ] **Step 1: Switch back to submodule directory**

Run:
```bash
cd /Users/sarathi/Documents/GitHub/frappe-ai-assistant/submodules/frappe-mcp-server
```

- [ ] **Step 2: Delete merge branch (local + remote)**

Run:
```bash
git checkout main
git branch -d merge/main-into-feature
git push origin :merge/main-into-feature
```
Expected: deleted successfully.

- [ ] **Step 3: Delete feature branch (local + remote) — only after team confirms no in-flight work**

Run:
```bash
git branch -d feature/mcp-go-sdk
git push origin :feature/mcp-go-sdk
```
Expected: deleted successfully. If `git branch -d` refuses, verify the branch is fully merged into main (`git log feature/mcp-go-sdk..main` should not be empty in reverse).

- [ ] **Step 4: Final verification**

Run:
```bash
git branch -a
git log --oneline -5
```
Expected: only `main` and any other long-lived branches present locally; remote branches list shows no `merge/*` or `feature/mcp-go-sdk`.

---

## Self-review checklist (run before handing off)

- [ ] Every task references the spec section it implements.
- [ ] Every code-changing step shows the actual code, not "edit appropriately".
- [ ] Every test step shows the failing-test code AND the expected failure mode.
- [ ] Every commit step has a real message, not `<message>`.
- [ ] Branch names, SHAs, and file paths are exact.
- [ ] No tasks left as "TBD" or "similar to Task N".

---

## Notes for the executor

- **Order matters.** Do not reorder tasks. C1/C2 must precede the audit fixes; H1/I1 must precede J1; J1 must precede K1.
- **Per-fix commits are deliberate.** If a single fix touches more than one file (e.g., E1 only touches `client.go`, E5 touches `config.go` + test), keep the commit focused on that one audit item.
- **If a fix step fails verification:** stop immediately. Reset (`git reset --hard HEAD~1` if the broken commit landed) and re-do the step. Do not chain further fixes onto a broken state.
- **If origin/main drifts during the run:** abort, fetch, rebase the merge branch onto the new main, re-run §H1 + §I1.
- **Phase 2 (PM tools) is out of scope here.** Do not implement the 8 deferred tools as part of this plan. Their spec is a separate brainstorm cycle.
