# Changelog

All notable changes to ERPNext MCP Server.

## [Unreleased] - 2026-04-12

### Added
- **Global Search Tool** — new `global_search` MCP tool backed by `POST /api/method/frappe.utils.global_search.search`
  - Full-text search across all indexed Frappe/ERPNext doctypes in a single call
  - Optional `doctype` param to restrict results to one doctype
  - Optional `scope` param (string or `[]string`) for multi-doctype filtering
  - `limit` (default 20) and `start` for pagination
  - Returns `name`, `doctype`, `content`, and `route` per result
  - Exposed via Streamable HTTP (`POST /mcp`) and REST (`POST /tool/global_search`)
  - Full `inputSchema` published in `/tools` listing
- **Unit tests for Global Search**
  - `internal/frappe/client_test.go` — `TestGlobalSearch` (valid text, default limit, empty text error)
  - `internal/tools/registry_test.go` — `TestGlobalSearch` (valid text, optional doctype filter, missing text error, invalid JSON error)
  - `internal/testutils/testutils.go` — mock handler for `POST /api/method/frappe.utils.global_search.search`
- **Streamable HTTP Transport** (ported from PR #3)
  - `POST /mcp` endpoint — JSON-RPC 2.0 Streamable HTTP with SSE support
  - Registered on the main port-8080 mux; inherits full 3-tier auth middleware (fixes auth gap)
- **OpenTelemetry Tracing** (ported from PR #3)
  - `go.opentelemetry.io/otel v1.43.0` with stdout OTLP exporter
  - Spans on every tool call (`tool.name`, `tool.doctype`, `tool.success` attributes)
  - Outbound Frappe HTTP wrapped with `otelhttp.NewTransport`
  - `internal/telemetry` package with `Init` / `Shutdown` helpers
- **Tool Input Schemas** (ported from PR #3)
  - `inputSchema` JSON Schema objects published for all active tools in `/tools` listing
  - Deprecated legacy tools hidden from listing but remain callable for backward compatibility
- **go-sdk Migration**
  - Migrated from custom MCP implementation to `github.com/modelcontextprotocol/go-sdk v1.4.0`
  - Server wraps `gosdk.Server`; all tool handlers use the SDK's `ToolRequest`/`ToolResponse` types

### Fixed
- **Auth gap on Streamable HTTP** — `POST /mcp` placed on main mux instead of a separate port, ensuring OAuth2/SID/API-key middleware is always applied

### Changed
- Go toolchain bumped to **1.25** (required by OTEL v1.43.0 indirect dependency)
- `Dockerfile` base image updated to `golang:1.25-alpine`

### Removed
- Deleted unused `create_oauth_client.py` and `create_oauth_client_fixed.py`
- **Neo4j configuration** — the `neo4j:` block and `NEO4J_BOLT_URL` / `NEO4J_USERNAME` / `NEO4J_PASSWORD` are gone; nothing has read them since the FrappeForge graph tools left. Config files are read strictly, so a leftover `neo4j:` block now stops the server at startup: delete it.
- **Demo CLIs `cmd/ollama-client` and `cmd/test-client`**, `Dockerfile.ollama-client`, their `make build-ollama-client` / `build-test-client` / `build-clients` targets, the `tests/*.sh` scripts that drove `ollama-client`, and `internal/utils`, which `cmd/ollama-client` alone imported. Neither shipped binary linked them, and `github.com/ollama/ollama` leaves `go.mod` with them: 9 of the module's 11 reachable advisories go with it.
- **`mcp.Server.RegisterResource`** and the resource URIs it tracked; its only caller left in 474d594 and the server has published no resources since.
- **The server's own chat pipeline** — `POST /api/v1/chat`, `GET /api/v1/openapi.json`, `internal/llm`, `internal/server/sse.go`, `formatter.go`, `report_schema.go` and `open_webui_functions/`. It was a second AI assistant beside frappe-ai-agent, and no consumer called it; the `llm:` configuration section, the `LLM_*` environment variables and the LLM documents go with it. Config files are read strictly, so a leftover `llm:` block now stops the server at startup: delete it. `/mcp` and the REST tool routes are unchanged. See `docs/adr/ADR-012-the-mcp-server-does-not-answer-questions-itself.md`.
- **The `ollama` and `open-webui` services in `compose.yml`**, with `OLLAMA_*`, `WEBUI_*` and `ENABLE_SIGNUP` in `env.example`. They existed to drive the removed chat endpoint, and the MCP server waited on Ollama's healthcheck to start although it never called it.
- **Configuration nothing read** — `server.max_connections`, `logging.format`, the whole `cache:` and `performance:` sections and `auth.token_cache.cleanup_interval`, with the `CACHE_CLEANUP_INTERVAL` environment variable and their keys in `config.yaml.example` and the docs. Config files are read strictly, so any of these left in a file now stops the server at startup: delete them. `logging.level` and `auth.token_cache.ttl` are read and stay.
- **Exported functions with no caller** — `auth.GetUserFromContext` (use `auth.UserFromContext`, which the rest of the module already uses; a nil result means no user), `strategies.OAuth2Strategy.ClearCache`, `mcp.Server.SDKServer`, `mcp.Server.ToolMetadata`, and the `testutils` `CreateTest*` and `Assert*` helpers, which duplicated testify.

## [Unreleased] - 2025-11-13

### Added
- **OAuth2 Authentication** - Standard OAuth2 security implementation
  - Bearer token validation with Frappe OAuth2 server
  - In-memory token caching (configurable TTL, default 5 minutes)
  - Support for Client Credentials Grant (backend-to-backend)
  - Support for Authorization Code Grant (user authentication)
  - Trusted client support for user context delegation
  - Optional vs required authentication modes (backward compatible)
  - User context propagation via `context.Context`
  - Comprehensive test coverage (18 unit tests, all passing)
  - Full documentation with quick start guide
  - Environment variable configuration support
  - Production-ready with proper error handling
- **Authentication Documentation** - Complete guides for OAuth2 setup
  - Full authentication guide (`docs/authentication.md`)
  - 5-minute quick start guide (`docs/auth-quickstart.md`)
  - Configuration examples and best practices
  - Troubleshooting guide and common scenarios
  - Migration path for gradual adoption

## [Previous] - 2025-11-12

### Added
- **Generic LLM Configuration** - Simplified 3-field config for ANY provider
  - Single unified configuration: `base_url`, `api_key`, `model`
  - Works with ANY OpenAI-compatible API
  - Easy provider switching without code changes
  - Automatic provider detection from base_url
- **Multiple LLM Provider Support** - Choose your AI provider beyond Ollama
  - **OpenAI** (gpt-4o-mini, gpt-4, gpt-3.5-turbo)
  - **Anthropic Claude** (claude-3.5-sonnet, claude-3-opus, claude-3-haiku)
  - **Azure OpenAI** (enterprise deployments)
  - **Ollama** (local, privacy-focused - remains default)
  - Unified LLM client abstraction layer
  - Easy provider switching via config or environment variables
  - Support for OpenAI-compatible APIs (LocalAI, LM Studio, OpenRouter)
- **GitHub Pages Documentation** - Professional documentation site with comprehensive guides
  - Quick Start guide
  - Configuration reference
  - AI Features documentation
  - Complete API reference
  - Development guide
  - Docker deployment guide
- **Cursor IDE Integration** - Fixed STDIO server to work seamlessly with Cursor MCP extension
  - Added `-config` flag support for stdio server
  - Fixed JSON-RPC protocol compliance
  - Added support for `initialized` notification
  - Added ping/heartbeat handling
- **AI-Powered Query Processing** - Natural language understanding using Ollama
  - Intent extraction from user queries
  - Entity detection (project IDs, customer names, etc.)
  - Generic tool support for ANY ERPNext doctype
- **Generic Tools** - `analyze_document` works with any doctype without hardcoding
- **Environment Variable Configuration** - `env.example` for easy setup
- **Simplified Docker Compose** - Single `compose.yml` with clear profiles

### Changed
- **Reorganized Documentation** - Moved all docs to `docs/` folder for GitHub Pages
- **Updated README** - Concise overview pointing to comprehensive docs
- **Cleaned Docker Configuration**:
  - Removed hardcoded credentials (now uses environment variables)
  - Fixed port conflicts
  - Added Ollama configuration to MCP server
  - Proper health checks and dependencies
- **Updated Makefile** - Build targets for all components

### Fixed
- **STDIO Server Configuration Loading** - Now correctly loads config file in Cursor
- **JSON-RPC Protocol Errors** - Fixed `id` field in error responses
- **Notification Handling** - Properly handles notifications without responses
- **AI Entity Extraction** - Improved prompt for better entity detection
- **ERPNext Retry Logic** - Increased retries and delays for resource-constrained instances

### Removed
- Deleted 30+ unnecessary files:
  - Demo scripts (`demo-*.sh`)
  - Redundant setup scripts (`setup-*.sh`)
  - Duplicate test scripts
  - 13 old markdown files (consolidated into `docs/`)
  - Unused Python scripts
  - Redundant JSON configuration files
  - Old Docker Compose files with wrong extensions
- Cleaned up root directory for better organization

### Security
- Removed hardcoded API credentials from Docker Compose
- Added environment variable support for all sensitive data
- Created `env.example` for secure credential management

## [1.0.0] - 2024

### Added
- Initial release
- MCP protocol support (HTTP and STDIO)
- ERPNext integration
- Basic CRUD tools
- Project management tools
- OpenAPI specification

---

## Release Notes

### Upgrading from Previous Version

1. **Update Configuration**:
```bash
cp env.example .env
# Fill in your ERPNext credentials in .env
```

2. **Rebuild Binaries**:
```bash
make clean
make build build-stdio
```

3. **Update Cursor Configuration** (if using Cursor):
```json
{
  "mcpServers": {
    "erpnext": {
      "command": "/absolute/path/to/bin/frappe-mcp-server-stdio",
      "args": ["--config", "/absolute/path/to/config.yaml"]
    }
  }
}
```

4. **Restart Services**:
```bash
# If using Docker
docker compose down
docker compose up -d

# If running locally
./bin/frappe-mcp-server
```

### Breaking Changes

- Docker Compose now requires `.env` file (use `env.example` as template)
- STDIO server requires `-config` flag in Cursor configuration
- Removed legacy docker-compose.*.yml files (use main `compose.yml`)

### Migration Guide

**From Hardcoded Config to Environment Variables**:

Before:
```yaml
# compose.yml
FRAPPE_API_KEY: 0d9f1b19563768b
```

After:
```bash
# .env file
FRAPPE_API_KEY=0d9f1b19563768b
```

**From Multiple Compose Files to Single File**:

Before:
```bash
docker-compose -f docker-compose.openwebui.yml up
```

After:
```bash
docker compose up -d
```

---

## Documentation

For detailed documentation, visit [GitHub Pages](https://vyogotech.github.io/frappe-mcp-server/)

