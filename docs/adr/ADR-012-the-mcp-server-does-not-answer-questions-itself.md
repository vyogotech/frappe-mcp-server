# ADR-012: The MCP server serves tools; it does not answer questions itself

Status: accepted (2026-09-23, production-readiness audit, ledger item X03)

## Context

This repository held two things: an MCP server, and a second AI assistant built on top of it.

The second assistant was `POST /api/v1/chat`. It took a natural-language message, called its own LLM to extract an
intent (`extractQueryIntent`), called more LLMs to fill in each tool's parameters (`extractAggregationParams`,
`extractReportParams`, `extractCreateParams`, `extractUpdateParams`), dispatched the tool itself, then called the LLM
again to format the result (`formatResponseWithLLM`), and streamed the whole thing back as SSE. It came to 1,596 lines
in `internal/server/server.go`, plus `internal/server/sse.go`, `internal/server/formatter.go`,
`internal/server/report_schema.go`, the 1,148-line `internal/llm` package with its two parallel LLM abstractions
(`llm.Client`, "Legacy", and `llm.Manager`, "New"), and `open_webui_functions/*.py`.

That is the same job `frappe-ai-agent` does. `frappe-ai-agent` is the FastAPI service the stack actually runs: it takes
the question, runs the tool-use loop over this server's MCP tools, and streams SSE from its own
`POST /api/v1/chat`. Every consumer in the three projects points at this server's `/mcp` endpoint and at nothing else:

```text
ragbot/compose.demo.yaml:180        AI_AGENT_MCP_SERVER_URL: http://mcp:8080/mcp
ragbot/compose.test.yaml:82,115     AI_AGENT_MCP_SERVER_URL: http://test-mcp:8080/mcp
frappe-ai-agent .github/workflows/ci.yml:243  AI_AGENT_INTEGRATION_MCP_URL: http://localhost:8080/mcp
```

The embedded assistant was also dormant wherever it ran. `ragbot/mcp/config.yaml` carries no `llm:` block, so
`llm.NewClient` fails at startup with `LLM model is required`, `llm.NewManager` leaves no usable client, and
`/api/v1/chat` answers every question with a canned string (`evidence/X03/repro-2.txt`). `internal/llm` has no test
files. `handleChatJSON` is the module's only function over the complexity budget, at 55. `config.yaml.example`
recommended Groq, a third-party hosted API, as the provider for it, which would have sent the user's question and the
raw Frappe rows it fetched off the machine.

The REST tool routes are a separate question. `/api/v1/tools`, `/api/v1/tools/`, `/tools` and `/tool/` publish and
dispatch the same tool catalogue over plain REST rather than MCP. They are labelled "for Open WebUI integration" and
"for backward compatibility" in the code's own comments, and no consumer in the three projects calls them either —
with one exception, found by grep: `ragbot/tests/system/confirmation.py:73` drives
`POST http://test-mcp:8080/api/v1/tools/delete_document` with a real user's sid and no grant header, to assert that the
write confirmation gate of ADR-006 refuses it. That is the only test of that trust boundary from outside the module.

## Options considered

1. **Delete the chat pipeline and `internal/llm`; keep the REST tool routes.** The strongest case against: it leaves a
   second, non-MCP way into the same tools, which is exactly what the security lead `mcp-security-09` asked to close,
   and it leaves `/api/v1/tools` publishing a catalogue that differs from `/mcp`'s (11 names against 17, measured in
   `evidence/X03/repro-2.txt`) because REST lists only tools with a description.
2. **Delete the chat pipeline and the REST tool routes together.** Against: `ragbot/tests/system/confirmation.py` would
   stop exercising the confirmation gate from outside, and section 6 of the audit's own rules forbids removing the only
   test on a trust boundary. Deleting a route in this repository to silently blind a check in another is the worst of
   both. The route can go once that check is moved to `/mcp`, which is a ragbot-side change.
3. **Keep the pipeline behind an off-by-default flag.** Against: the code still compiles, still ships, still carries
   4,400 lines and an untested LLM package, and the flag is one more configuration key that nothing reads in the
   deployed stack — the same defect C18 was opened for.
4. **Move it to its own Go module.** Against: nothing consumes it. A second module to version, pin, build and release
   for zero users is strictly more work than deleting it, and git history keeps the code either way.

The strongest case against deleting at all is an operator outside this stack who points Open WebUI at
`/api/v1/chat`: they lose the endpoint with no deprecation window, and `open_webui_functions/*.py` goes with it. Against
that: the repository has no git tags and no released version, so nothing ever promised the endpoint as supported; the
pipeline is inert without an `llm:` block that the shipped configuration does not have; and an Open WebUI user is better
served by `frappe-ai-agent`, which does the same job with tests, a leak filter and a confirmation flow.

## Decision

Option 1.

- `POST /api/v1/chat` and `GET /api/v1/openapi.json` are removed, with `handleChat`, `handleChatJSON`, `handleChatSSE`,
  `handleOpenAPI`, `generateWithLLM`, `detectProvider`, every `extract*` intent and parameter function,
  `formatResponseWithLLM`, `searchForEntity`, `executeTool`, `executeToolWithEntity`, `mapActionToTool`,
  `fallbackQueryRouting`, `cleanJSONResponse`, the `QueryIntent` and `SearchResult` types, and the files
  `internal/server/sse.go`, `internal/server/formatter.go` and `internal/server/report_schema.go`.
- `internal/llm` is deleted whole, and with it the `llm:` section of the configuration, the `LLM_*` environment
  variables, and `docs/llm-providers.md`, `docs/llm-implementation.md`, `docs/generic-llm-config.md` and
  `docs/ai-features.md`, which describe only the removed pipeline.
- `open_webui_functions/` is deleted; it exists only to call the removed endpoint.
- `/mcp`, `/health`, `/api/v1/health`, `/api/v1/tools`, `/api/v1/tools/`, `/tools` and `/tool/` stay. This server's job
  is to publish and run Frappe tools; deciding what to call is the agent's.

## Consequences

- About 4,400 lines leave the module. The `gocyclo` finding on `handleChatJSON`, the only one in the repository, goes
  with them, and so does an LLM package with no tests. Three things the pipeline alone used go in the same commit:
  `frappe.Client.GetReportFilters` and the `types.ReportFilter` it returned, which only `report_schema.go` called, and
  `MCPServer.tools`, the registry handle no code reads once the chat handlers are gone.
- `frappe-ai-agent` is the only assistant. Nothing in the three projects changes, because nothing called the removed
  routes.
- A caller of `/api/v1/chat` or `/api/v1/openapi.json` now gets `404`. The `/mcp` endpoint, the tool catalogue it
  publishes and the REST tool routes are unchanged; `evidence/X03/verify-2.txt` shows the same `tools/list` before and
  after.
- Configuration is smaller: an `llm:` block, which strict decoding now rejects as an unknown key, has to be deleted
  from any file that still has one. `config.yaml.example` and the LLM documents go in the same commit.
- Six pre-existing test files change, each because the behaviour under it was removed: `server_test.go` (the legacy-LLM
  fallback test and the `/api/v1/chat` middleware row), `sse_test.go` and `session_expired_format_test.go` (the SSE
  writer and the chat error formatter), `ipv6_host_test.go` (it read the listener's address out of the OpenAPI
  document), `confirmation_test.go` (`executeTool` was one of its three entry points) and ADR-018's named confirmation
  `TestExecuteTool_FabricatedPMToolsHidden`, which moves to the surviving REST dispatcher and keeps asserting the same
  thing.
- The divergence between the REST catalogue and the MCP one, and the deprecated tools that the agent filters at its own
  boundary but REST does not, stay open. They are reasons to retire the REST routes next, once
  `ragbot/tests/system/confirmation.py` probes `/mcp` instead.

## Confirmation

`internal/server/probes_test.go`'s `TestServedPathsAndProbes` fails if `/api/v1/chat` or `/api/v1/openapi.json` answers
anything but `404`, and passes only while `/mcp`, `/health` and the REST tool routes still answer. `deadcode ./...`
fails the audit's `mcp:maintainability` check if an unreachable function comes back, and
`golangci-lint run --default=none --enable=gocyclo,dupl ./...` fails if a function over the complexity budget returns.
`internal/server/confirmation_test.go` and `internal/server/rest_dispatch_test.go` keep the surviving REST tool path
under test, and `ragbot/tests/system/confirmation.py` keeps probing it from outside.
