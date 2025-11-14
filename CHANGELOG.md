# Changelog

All notable changes to ERPNext MCP Server.

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

For detailed documentation, visit [GitHub Pages](https://varkrish.github.io/frappe-mcp-server/)

