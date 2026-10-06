# ADR-018: A tool answers with what Frappe returned, or it does not exist

Status: accepted (2026-09-23, production-readiness audit, ledger item C10)

## Context

Four project tools answered with values no code had computed.

`calculate_project_metrics` returned `burn_rate`, `velocity`, `efficiency` and `risk_score` as `0.0` and `health` as
`"Green"`. `project_risk_assessment` returned `schedule_risk`, `budget_risk`, `resource_risk`, `scope_risk` and
`overall_risk` as `"Low"`, with three recommendations no calculation produced. `portfolio_dashboard` returned
`active_projects`, `completed_projects` and `overdue_projects` as `0` and every KPI as `0.0`, beside the project rows it
had just fetched. `analyze_project_timeline` returned `critical_path_tasks` and `milestones` as empty lists and
`timeline_health` as `"analyzing..."`. Each value was a literal in `internal/tools/registry.go`, followed by a `TODO`.

Driven against a project 200 days past its end date, 0% complete and four times over budget, the three stdio-only tools
returned Green, Low and zero for that project (`evidence/C10/repro-1.txt` in the audit workspace).

The caller is a language model. It cannot see the literal, and nothing in the payload marks these fields as not yet
implemented: a `TODO` in the source is invisible over JSON-RPC. The model reads `"health": "Green"` as a finding about
the project and says so to the user, who has no way to tell it apart from a number that was measured. This is OWASP's
LLM09, Misinformation (<https://genai.owasp.org/llmrisk/llm092025-misinformation/>): the system states as fact something
it never determined. A tool that returns nothing is safe, because the model says it does not know. A tool that returns a
confident constant is not.

The three stdio-only tools had already been taken off the HTTP server for this reason, but the stdio server
(`cmd/mcp-stdio`, which `cmd/ollama-client` spawns) still registered all three, and its system prompt told the model to
call `portfolio_dashboard` for any general question about the business.

## Options considered

1. **Implement the calculations.** The honest end state, and the earlier merge plan deferred it to a "Phase 2" that has
   not come. Against: critical path, milestones, velocity, burn rate and a four-dimension risk model are a feature, not a
   fix; nothing in this repository knows what the owner means by any of them, and the tools stay wrong until it ships.
2. **Keep the tools and mark the fields**, for example `"health": null` with a `"not_implemented": ["health"]` list.
   Against: it puts the burden on the model to notice and relay the marker on every turn, and a field that is always
   null is a field that should not be in the payload. It also keeps the tool in `tools/list`, so the model keeps choosing
   it over a tool that would have answered.
3. **Leave them; they are only on stdio.** Against: stdio is how `cmd/ollama-client` runs, and it is the transport an
   editor or desktop MCP client uses. "Only reachable by the users we ship to" is not a defence.
4. **Delete the three tools and the four stub fields, and keep whatever answers with real data.** The strongest case
   against: it is a breaking change to a published tool list, a client that calls `portfolio_dashboard` stops working,
   and the deleted code is the only written record of the intended metrics. Weighed against that, the tools have no
   correct callers to break (every answer they gave was wrong), and the intent is preserved in this ADR and in git
   history, which is where an unimplemented design belongs.

## Decision

Option 4.

- `calculate_project_metrics`, `project_risk_assessment` and `portfolio_dashboard` are deleted, with their registrations
  in `cmd/mcp-stdio/main.go` and the now-unused `types.ProjectMetrics`.
- `analyze_project_timeline` is kept. With `critical_path_tasks`, `milestones` and `timeline_health` removed, every field
  left is read from the project or counted from its tasks: the project document, the task rows ordered by start date,
  `total_tasks`, `project_start_date`, `project_end_date` and `project_progress`. That is a true answer about a project's
  timeline, so the tool stays.
- No tool in this server reports a field that no code computes. A value that cannot be calculated is left out of the
  payload, not defaulted.

## Consequences

- A client's `tools/list` over stdio no longer contains the three names; the HTTP server's list is unchanged, because it
  never offered them.
- A client that calls one of the three by name gets the go-sdk's JSON-RPC error `-32602`, `unknown tool "<name>"`. Over
  the REST tool route the same call answers `404 Tool not found`, as it already did.
- `analyze_project_timeline` answers the same shape minus the three fields. A client reading `timeline_health` gets
  nothing; there was nothing there to read.
- `cmd/ollama-client`'s system prompt now points general questions about the business at `list_documents` on `Project`,
  and `cmd/test-client` loses its portfolio command and its portfolio test.
- An operator who wants portfolio or risk numbers uses ERPNext for them: `run_report` for a query report the site
  already defines, `aggregate_documents` for counts and sums per status, or `list_documents` on `Project` and its tasks
  to read the rows. Those answer from the site's own data. If the metrics in option 1 are wanted as tools, they are a
  feature request with a definition attached, not a restoration of these functions.

## Confirmation

`internal/tools/no_fabricated_values_test.go`. `TestAnalyzeProjectTimelineReportsNoUncomputedField` fails if
`critical_path_tasks`, `milestones` or `timeline_health` comes back in `timeline_analysis` again, and
`TestFabricatedProjectToolsStayRemoved` fails if any of the three methods returns to `ToolRegistry`, which is the only
way a registration for those names can come back. `internal/server/server_test.go`'s
`TestExecuteTool_FabricatedPMToolsHidden` keeps the HTTP dispatcher refusing the three names.
