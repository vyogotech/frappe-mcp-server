# Authentication

This document describes how to set up and use authentication with the Frappe MCP Server.

## Overview

The Frappe MCP Server supports **three authentication methods** with automatic priority-based fallback:

| Priority | Method | Use Case | Permissions |
|----------|--------|----------|-------------|
| 1 | **Frappe `sid` cookie** | Frappe apps (Awesome Bar, desk widgets) | User-level — respects Frappe roles |
| 2 | **OAuth2 Bearer token** | External apps (Open WebUI, VS Code, mobile) | User or system-level |
| 3 | **API key/secret** | Server-to-server, fallback | System-level |

Authentication is **optional by default** — set `require_auth: true` in production.

## Architecture

### Authentication Priority

The server tries each method in order, using the first that succeeds:

```
Incoming Request
     │
     ├─1─► sid cookie present?  → Validate with Frappe → ✅ User-level permissions
     │
     ├─2─► Bearer token present? → Validate OAuth2 token → ✅ User/system permissions  
     │
     └─3─► API key configured?  → Use token key:secret  → ✅ System-level permissions
```

### Request Flow (sid cookie)

```
User (logged into ERPNext)
  │  1. Request with Cookie: sid=abc123
  ▼
MCP Server (auth middleware)
  │  2. Validate sid with Frappe /api/method/frappe.integrations.oauth2.openid.userinfo
  │  3. Extract CSRF token from response
  │  4. Store {SessionID, CSRFToken, Email} in request context
  ▼
Tool Handler
  │  5. Frappe client reads user from context
  │  6. Forwards Cookie: sid + X-Frappe-CSRF-Token on all ERPNext API calls
  ▼
ERPNext (validates sid per request, enforces user permissions)
```

## Configuration

### `config.yaml`

```yaml
auth:
  # Master switch — set false to disable all auth checks
  enabled: true

  # true = reject unauthenticated requests (production)
  # false = optional auth, anonymous requests still allowed (development)
  require_auth: false

  oauth2:
    # Frappe userinfo endpoint (used for both sid validation and Bearer token introspection)
    token_info_url: "http://localhost:8000/api/method/frappe.integrations.oauth2.openid.userinfo"
    issuer_url: "http://localhost:8000"

    # Backend clients that can pass user context via X-MCP-User-* headers
    trusted_clients:
      - "frappe-mcp-backend"

    # Set false to skip remote validation in local development
    validate_remote: true

    timeout: "30s"

  token_cache:
    ttl: "5m"
    cleanup_interval: "10m"

# ERPNext credentials — used as Priority 3 fallback
erpnext:
  base_url: "http://localhost:8000"
  api_key: "your_api_key"
  api_secret: "your_api_secret"
```

### Environment Variables

```bash
AUTH_ENABLED=true
AUTH_REQUIRE_AUTH=false
OAUTH_TOKEN_INFO_URL=http://localhost:8000/api/method/frappe.integrations.oauth2.openid.userinfo
OAUTH_ISSUER_URL=http://localhost:8000
OAUTH_TIMEOUT=30s
CACHE_TTL=5m
CACHE_CLEANUP_INTERVAL=10m
```

## Method 1: Frappe `sid` Cookie (Recommended for Frappe Apps)

The simplest integration for any Frappe app. No OAuth2 setup required — the user's existing Frappe login session is passed through.

### How it works

1. User is already logged into ERPNext
2. Your Frappe app reads `frappe.session.sid`
3. Sends it as a cookie to the MCP server
4. MCP validates the session and forwards it to all ERPNext API calls
5. All queries run with the **logged-in user's permissions**

### Frappe App Example

```python
import frappe
import requests

@frappe.whitelist()
def query_mcp(message):
    settings = frappe.get_single("MCP Server Settings")

    response = requests.post(
        f"{settings.mcp_server_url}/api/v1/chat",
        json={"message": message},
        cookies={"sid": frappe.session.sid},  # Pass user session
        timeout=30
    )
    return response.json()
```

### Testing with curl

```bash
# Get your sid from ERPNext browser DevTools → Application → Cookies
export SID='your-sid-value-here'

curl -X POST http://localhost:8080/api/v1/chat \
  -b "sid=$SID" \
  -H "Content-Type: application/json" \
  -d '{"message": "show me top 5 customers"}'
```

### CSRF Token Handling

For `POST`/`PUT`/`DELETE` operations, the MCP server automatically extracts and forwards the `X-Frappe-CSRF-Token` header. This is handled transparently — no action needed in your app.

## Method 2: OAuth2 Bearer Token

For external clients (Open WebUI, VS Code extensions, mobile apps) that cannot share a Frappe session cookie.

### Setup: Register OAuth2 Client in Frappe

1. Navigate to `/app/oauth-client` in your Frappe instance
2. Create a new OAuth Client:
   - **App Name**: `MCP Backend Integration`
   - **Client ID**: `frappe-mcp-backend` (or auto-generated)
   - **Scopes**: `openid profile email all`
   - **Grant Type**: `Client Credentials` or `Authorization Code`
3. Note the **Client ID** and **Client Secret**

### Client Credentials Flow (Backend-to-Backend)

```bash
# 1. Get token
TOKEN=$(curl -s -X POST http://localhost:8000/api/method/frappe.integrations.oauth2.get_token \
  -d "grant_type=client_credentials" \
  -d "client_id=frappe-mcp-backend" \
  -d "client_secret=YOUR_CLIENT_SECRET" \
  | jq -r '.access_token')

# 2. Call MCP
curl -X POST http://localhost:8080/api/v1/chat \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"message": "Show me all open projects"}'
```

### Authorization Code Flow (User Login)

For user-facing apps where individual users authenticate via Frappe's login page:

```bash
python3 scripts/get-oauth-token-authcode.py
```

This script opens a browser for the user to log in and returns a Bearer token.

## Method 3: API Key / Secret (System Fallback)

Used automatically when no `sid` or `Bearer` token is present. Configured via `config.yaml` or environment variables.

```yaml
erpnext:
  api_key: "your_api_key"
  api_secret: "your_api_secret"
```

All tool calls run under the **API user's permissions** in this mode. For write operations (`POST`/`PUT`/`DELETE`), the server sets `X-Frappe-CSRF-Token: bypass` automatically since API key auth bypasses Frappe's CSRF checks.

## Accessing User Context in Tools

```go
import "frappe-mcp-server/internal/auth"

func (r *ToolRegistry) MyTool(ctx context.Context, req mcp.ToolRequest) (*mcp.ToolResponse, error) {
    user, found := auth.GetUserFromContext(ctx)
    if found {
        // user.Email, user.FullName, user.Roles, user.SessionID, user.Token
        slog.Info("Tool called by", "user", user.Email)
    }
    // ...
}
```

The `User` struct:

```go
type User struct {
    ID        string
    Email     string
    FullName  string
    Roles     []string
    Token     string    // OAuth2 Bearer token
    SessionID string    // Frappe sid cookie value
    CSRFToken string    // Frappe CSRF token (extracted during sid validation)
    ClientID  string    // OAuth2 client ID
}
```

## Security Best Practices

1. **Enable HTTPS** in production — never pass `sid` or Bearer tokens over plain HTTP
2. Set `require_auth: true` in production
3. Use `validate_remote: true` to introspect tokens with Frappe
4. Keep API key/secret in environment variables, not in `config.yaml` committed to source control
5. Rotate API key/secret and OAuth2 client secrets periodically
6. Token cache TTL of 5 minutes balances performance and security

## Troubleshooting

### `401 Unauthorized` on all requests

- Check `auth.enabled` and `auth.require_auth` in `config.yaml`
- Verify `token_info_url` points to a reachable Frappe endpoint
- Enable debug logging: `logging.level: debug`

### `"invalid session: status 401"` with sid cookie

The `sid` has expired. The user needs to log in to ERPNext again. Sessions expire based on Frappe's session lifetime setting.

### `"CSRF token required"` on POST/PUT/DELETE with sid auth

MCP server couldn't extract the CSRF token during session validation. Check Frappe server logs and ensure the `token_info_url` endpoint returns the `X-Frappe-CSRF-Token` response header.

### User context is nil in tool handler

- Auth middleware is only applied to the HTTP interface, not stdio
- Verify the client is listed in `trusted_clients` if using `X-MCP-User-*` headers
- Set `require_auth: false` if running in optional auth mode

### Debug Logging

```yaml
logging:
  level: "debug"
```

This logs auth decisions, sid validation calls, token cache hits/misses, and CSRF token extraction.

## Testing

```bash
# Run auth unit tests
go test ./internal/auth/... -v

# Test with sid cookie
export SID='your-sid-value'
./test_sid_auth.sh

# Test OAuth2 flow
./test-oauth.sh
```

## Related

- [Auth Quick Start](auth-quickstart) — Set up auth in 5 minutes
- [Configuration](configuration) — Full config reference
- [OAuth2 RFC 6749](https://tools.ietf.org/html/rfc6749)
- [Frappe OAuth2 Docs](https://frappeframework.com/docs/user/en/guides/integration/oauth)


