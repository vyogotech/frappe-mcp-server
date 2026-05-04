# Imported project — detected tech stack

## Job vision / description
Change this Frappe MCP Server : https://github.com/vyogotech/frappe-mcp-server to remove ff_* tools and have only other tools.

## Detected languages
Go

## Markers & frameworks
- Go (go.mod)

## Top-level layout

- **Subdirectories:** .github, cmd, configs, dashboards, docs, internal, open_webui_functions, scripts, tests
- **Files in root:** .gitignore, .golangci.yml, CHANGELOG.md, Dockerfile, Dockerfile.ollama-client, LICENSE, Makefile, README.md, compose.yml, config.yaml.example, crew_errors.log, delivery_mode_triage.json, env.example, go.mod, go.sum, install.sh, main.go, state_bb44072e-0c32-4e79-a6cd-4c8d9a3bcb6e.json, tasks_bb44072e-0c32-4e79-a6cd-4c8d9a3bcb6e.db, test-api.sh, test-oauth.sh, test_account_manager.sh, test_sid_auth.sh, test_with_real_sid.sh

## File listing sample
```
All files under .:
  .github/workflows/ci.yml (3625 bytes)
  .github/workflows/release.yml (4657 bytes)
  .gitignore (1713 bytes)
  .golangci.yml (348 bytes)
  CHANGELOG.md (8393 bytes)
  Dockerfile (807 bytes)
  Dockerfile.ollama-client (1421 bytes)
  LICENSE (1066 bytes)
  Makefile (8928 bytes)
  README.md (7298 bytes)
  cmd/mcp-stdio/main.go (4054 bytes)
  cmd/ollama-client/main.go (40295 bytes)
  cmd/test-client/main.go (11049 bytes)
  compose.yml (4326 bytes)
  config.yaml.example (6167 bytes)
  configs/claude-desktop-template.json (460 bytes)
  crew_errors.log (1374 bytes)
  dashboards/executive_template.md (1097 bytes)
  dashboards/project_health_template.md (685 bytes)
  delivery_mode_triage.json (314 bytes)
  docs/_config.yml (753 bytes)
  docs/ai-features.md (8850 bytes)
  docs/analytics-features.md (11193 bytes)
  docs/api-reference.md (10472 bytes)
  docs/auth-quickstart.md (3465 bytes)
  docs/authentication.md (8913 bytes)
  docs/configuration.md (5381 bytes)
  docs/development.md (8721 bytes)
  docs/distribution.md (7306 bytes)
  docs/docker.md (5794 bytes)
  docs/generic-llm-config.md (8707 bytes)
  docs/implementation-summary.md (9830 bytes)
  docs/index.md (4588 bytes)
  docs/installation.md (6721 bytes)
  docs/llm-implementation.md (7553 bytes)
  docs/llm-providers.md (10775 bytes)
  docs/oauth2-implementation.md (14125 bytes)
  docs/quick-start.md (4947 bytes)
  docs/releases.md (5976 bytes)
  docs/superpowers/plans/2026-04-27-frappe-mcp-server-merge.md (55329 bytes)
  docs/superpowers/specs/2026-04-27-frappe-mcp-server-merge-design.md (21842 bytes)
  env.example (2107 bytes)
  go.mod (1952 bytes)
  go.sum (9323 bytes)
  install.sh (7224 bytes)
  internal/auth/context.go (730 bytes)
  internal/auth/context_test.go (2214 bytes)
  internal/auth/middleware.go (1348 bytes)
  internal/auth/middleware_test.go (7628 bytes)
  internal/auth/strategies/oauth2.go (9566 bytes)
  internal/auth/strategies/oauth2_test.go (9099 bytes)
  internal/config/config.go (11610 bytes)
  internal/config/config_test.go (8610 bytes)
  internal/frappe/client.go (23067 bytes)
  internal/frappe/client_test.go (14199 bytes)
  internal/llm/anthropic.go (2306 bytes)
  internal/llm/azure.go (2581 bytes)
  internal/llm/client.go (2361 bytes)
  internal/llm/manager.go (16405 bytes)
  internal/llm/openai.go (5720 bytes)
  internal/llm/registry.go (3939 bytes)
  internal/mcp/jsonrpc.go (3332 bytes)
  internal/mcp/server.go (9963 bytes)
  internal/mcp/server_test.go (4994 bytes)
  internal/mcp/streamable_http.go (7044 bytes)
  internal/mcp/streamable_http_e2e_test.go (1569 bytes)
  internal/mcp/streamable_http_test.go (6123 bytes)
  internal/neo4j/client.go (2148 bytes)
  internal/neo4j/client_test.go (1520 bytes)
  internal/server/formatter.go (17995 bytes)
  internal/server/report_schema.go (6862 bytes)
  internal/server/server.go (94492 bytes)
  internal/server/server_test.go (3710 bytes)
  internal/server/sse.go (13996 bytes)
  internal/server/sse_test.go (789 bytes)
  internal/telemetry/tracing.go (2817 bytes)
  internal/telemetry/tracing_test.go (1706 bytes)
  internal/testutils/testutils.go (8473 bytes)
  internal/tools/frappeforge.go (12086 bytes)
  internal/tools/frappeforge_test.go (9228 bytes)
  internal/tools/registry.go (29152 bytes)
  internal/tools/registry_test.go (14963 bytes)
  internal/types/types.go (6514 bytes)
  internal/utils/entity_resolution.go (4866 bytes)
  internal/utils/entity_resolution_test.go (1281 bytes)
  internal/utils/utils.go (3591 bytes)
  internal/utils/utils_test.go (9808 bytes)
  main.go (1978 bytes)
  open_webui_functions/erpnext_integration.py (7686 bytes)
  open_webui_functions/erpnext_oauth_integration.py (12413 bytes)
  scripts/create-demo-data.py (11193 bytes)
  scripts/create-demo-via-chat.sh (2872 bytes)
  scripts/create-oauth-client.py (5954 bytes)
  scripts/get-oauth-token-authcode.py (7125 bytes)
  state_bb44072e-0c32-4e79-a6cd-4c8d9a3bcb6e.json (122 bytes)
  tasks_bb44072e-0c32-4e79-a6cd-4c8d9a3bcb6e.db (40960 bytes)
  test-api.sh (3898 bytes)
  test-oauth.sh (8968 bytes)
  test_account_manager.sh (12498 bytes)
  test_sid_auth.sh (997 bytes)
  test_with_real_sid.sh (742 bytes)
  tests/test-api-endpoints.sh (2631 bytes)
  tests/test-data-display.sh (1203 bytes)
  tests/test-data-driven.sh (3920 bytes)
  tests/test-data-passing.sh (1664 bytes)
  tests/test-enhanced-nl.sh (3577 bytes)
  tests/test-enhanced-preprocessing.sh (3369 bytes)
  tests/test-integration.sh (5104 bytes)
  tests/test-open-webui.sh (5284 bytes)
  tests/test-openwebui-integration.sh (13460 bytes)
```

## Project overview (LLM)
*   This project is a centralized server, named Frappe MCP Server, for managing multiple Frappe framework sites.
*   The primary technology is the Go programming language, managed with Go Modules (`go.mod`).
*   To run the development server, use the command `go run . serve`.
*   The project can be built using `make build` and tests can be run with `make test`.

## Iteration notes
- Use **Refine** in the Files UI to apply natural-language edits.
- Prefer targeted file scope when possible for speed and accuracy.
