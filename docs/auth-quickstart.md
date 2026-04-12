# Authentication - Quick Start

Get up and running with authentication in 5 minutes. Choose the method that fits your use case.

## Option A: sid Cookie (Fastest — Frappe Apps)

No OAuth2 setup needed. Uses the user's existing Frappe login session.

### Step 1: Get your sid from ERPNext

1. Log into ERPNext at `http://localhost:8000`
2. Open browser DevTools → **Application** tab → **Cookies**
3. Copy the `sid` cookie value

### Step 2: Test

```bash
export SID='paste-your-sid-here'

curl -X POST http://localhost:8080/api/v1/chat \
  -b "sid=$SID" \
  -H "Content-Type: application/json" \
  -d '{"message": "show me top 5 customers"}'
```

### Step 3: Configure MCP Server

```yaml
# config.yaml
auth:
  enabled: true
  require_auth: false   # Set true in production
  oauth2:
    token_info_url: "http://localhost:8000/api/method/frappe.integrations.oauth2.openid.userinfo"
    issuer_url: "http://localhost:8000"
    validate_remote: true
    timeout: "30s"
  token_cache:
    ttl: "5m"
    cleanup_interval: "10m"
```

That's it — the MCP server validates the sid with Frappe and forwards it to all ERPNext API calls.

---

## Option B: OAuth2 Bearer Token (External Apps)

For Open WebUI, VS Code extensions, or any app that can't share a Frappe session cookie.

### Step 1: Create OAuth2 Client in Frappe

Navigate to `http://localhost:8000/app/oauth-client` → **New**:

- **App Name**: `MCP Backend Integration`
- **Scopes**: `openid profile email all`
- **Grant Type**: `Client Credentials`

Save and note the **Client ID** and **Client Secret**.

### Step 2: Configure MCP Server

```yaml
auth:
  enabled: true
  require_auth: false
  oauth2:
    token_info_url: "http://localhost:8000/api/method/frappe.integrations.oauth2.openid.userinfo"
    issuer_url: "http://localhost:8000"
    trusted_clients:
      - "YOUR_CLIENT_ID"
    validate_remote: true
    timeout: "30s"
  token_cache:
    ttl: "5m"
    cleanup_interval: "10m"
```

### Step 3: Get a Token and Test

```bash
# Get token (Client Credentials)
TOKEN=$(curl -s -X POST http://localhost:8000/api/method/frappe.integrations.oauth2.get_token \
  -d "grant_type=client_credentials" \
  -d "client_id=YOUR_CLIENT_ID" \
  -d "client_secret=YOUR_CLIENT_SECRET" \
  | jq -r '.access_token')

# Test
curl -X POST http://localhost:8080/api/v1/chat \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"message": "show me all customers"}'
```

For user-level OAuth2 (Authorization Code flow):

```bash
python3 scripts/get-oauth-token-authcode.py
```

---

## Option C: API Key (No Auth Setup)

Simplest option — no Frappe OAuth config needed. All calls run under the API user's permissions.

```yaml
# config.yaml
auth:
  enabled: false   # Disable inbound auth checks

erpnext:
  base_url: "http://localhost:8000"
  api_key: "your_api_key"
  api_secret: "your_api_secret"
```

---

## Verify It's Working

```bash
# Health check (no auth required)
curl http://localhost:8080/health

# Test auth unit tests
go test ./internal/auth/... -v
```

## Production Checklist

- [ ] `require_auth: true` in `config.yaml`
- [ ] HTTPS enabled (never send `sid` or tokens over plain HTTP)
- [ ] API key/secret stored in environment variables, not in committed config
- [ ] `validate_remote: true`
- [ ] Rotate secrets periodically

## More

See [Authentication](authentication) for the full reference including CSRF handling, token caching, and troubleshooting.

