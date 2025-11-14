# Hybrid Authentication: sid Cookie + OAuth2

## Overview

The MCP server now supports **both sid cookie (Frappe session) and OAuth2** authentication methods, with automatic fallback.

## Authentication Priority

```
1. sid cookie     → User-level permissions (best)
2. Bearer token   → OAuth2 (system or user level)
3. API key/secret → Fallback (system level)
```

## Implementation Summary

### 1. Frappe App (`frappe_ai`) ✅

**File**: `frappe_ai/api/ai_query.py`

- **Default behavior**: Uses `sid` cookie (user's Frappe session)
- **Fallback**: Uses OAuth2 if configured
- **Benefit**: Simple, secure, respects user permissions

```python
# By default, sends sid cookie
response = requests.post(
    f"{settings.mcp_server_url}/api/v1/chat",
    json=payload,
    cookies={"sid": frappe.session.sid},  # User's session
    timeout=30
)
```

### 2. MCP Server Auth Strategy ✅

**File**: `internal/auth/strategies/oauth2.go`

- Tries `sid` cookie first
- Falls back to `Bearer` token if no sid
- Validates session with Frappe
- Caches validated sessions

```go
func (s *OAuth2Strategy) Authenticate(ctx context.Context, r *http.Request) (*types.User, error) {
    // Try sid cookie first
    if sidCookie, err := r.Cookie("sid"); err == nil {
        user, err := s.validateSessionCookie(ctx, sidCookie)
        if err == nil {
            return user, nil
        }
    }
    
    // Fallback to Bearer token
    token := extractBearerToken(r)
    // ... validate token
}
```

### 3. MCP Server Frappe Client ✅

**File**: `internal/frappe/client.go`

- Forwards `sid` cookie to Frappe API calls
- Falls back to OAuth token or API key
- Maintains user context throughout

```go
if user != nil && user.SessionID != "" {
    // Forward sid cookie to Frappe
    req.AddCookie(&http.Cookie{
        Name:  "sid",
        Value: user.SessionID,
    })
} else if user != nil && user.Token != "" {
    // Use OAuth token
    req.Header.Set("Authorization", "Bearer " + user.Token)
} else if c.apiKey != "" {
    // Use API key
    req.Header.Set("Authorization", "token " + c.apiKey + ":" + c.apiSecret)
}
```

### 4. Types ✅

**File**: `internal/types/types.go`

Added `SessionID` field to User struct:

```go
type User struct {
    ID        string
    Email     string
    FullName  string
    Roles     []string
    Token     string    // OAuth2 token
    SessionID string    // Frappe session ID (sid)
    // ...
}
```

## How It Works

### Awesome Bar Flow (sid cookie)

```
1. User logs into ERPNext → Gets sid cookie
2. User types "@ai show customers" in Awesome Bar
3. Frappe app reads frappe.session.sid
4. Sends to MCP: Cookie: sid=xyz123
5. MCP validates sid with Frappe
6. MCP forwards sid to all Frappe API calls
7. Frappe validates sid per request
8. ✅ All queries run with user's permissions
```

### Open WebUI Flow (OAuth - future)

```
1. Open WebUI gets OAuth token
2. Sends to MCP: Authorization: Bearer <token>
3. MCP validates token
4. MCP uses token for Frappe API calls
5. ⚠️  System-level permissions (or API user's role)
```

## Testing

### Test 1: Basic Health Check

```bash
curl http://localhost:8080/health
```

### Test 2: With sid Cookie

1. **Get your sid from ERPNext:**
   - Log into ERPNext at http://localhost:8000
   - Open browser DevTools (F12)
   - Go to Application > Cookies
   - Copy the `sid` cookie value

2. **Test with curl:**

```bash
export SID='your-sid-value-here'

curl -v -b "sid=$SID" \
  -H "Content-Type: application/json" \
  -X POST http://localhost:8080/api/v1/chat \
  -d '{
    "message": "show me top 5 customers",
    "context": {
      "user_id": "test",
      "timestamp": "2025-11-14T00:00:00"
    }
  }'
```

3. **Or use the test script:**

```bash
export SID='your-sid-value-here'
./test_with_real_sid.sh
```

### Test 3: Via Awesome Bar

1. Log into ERPNext at http://localhost:8000
2. Press `/` or click Awesome Bar
3. Type: `@ai show me top 5 customers`
4. Press Enter
5. Check MCP server logs: `docker logs -f frappe-mcp-server-frappe-mcp-server-1`

## Configuration

### Current Settings (config.yaml)

```yaml
auth:
  enabled: true
  require_auth: false  # Optional for now
  oauth2:
    token_info_url: "http://erpnext:8000/api/method/frappe.integrations.oauth2.openid.userinfo"
    issuer_url: "http://erpnext:8000"
    trusted_clients:
      - "g79ghfpol3"
    validate_remote: true
```

### MCP Server Settings (ERPNext)

```json
{
  "enabled": 1,
  "mcp_server_url": "http://frappe-mcp-server:8080",
  "frappe_base_url": "http://erpnext:8000",
  "oauth_client_id": "",  // Optional, for OAuth mode
  "oauth_client_secret": ""  // Optional, for OAuth mode
}
```

**Note**: If OAuth client ID/secret are not set, the app automatically uses sid cookie mode.

## Benefits

| Feature | sid Cookie | OAuth2 |
|---------|-----------|---------|
| **User Permissions** | ✅ Yes | ⚠️ Depends |
| **Setup Complexity** | ✅ Simple | ⚠️ Complex |
| **External Apps** | ❌ No | ✅ Yes |
| **Works Now** | ✅ Yes | ❌ Frappe limitation |

## Troubleshooting

### Issue: "missing authentication: no sid cookie or Bearer token found"

**Solution**: Ensure the Frappe app is sending the sid cookie:

```python
# In frappe_ai/api/ai_query.py
sid = frappe.session.sid
response = requests.post(
    ...,
    cookies={"sid": sid}
)
```

### Issue: "invalid session: status 401"

**Solution**: The sid has expired. The user needs to log in again to ERPNext.

### Issue: MCP server can't reach ERPNext

**Solution**: Check Docker network connectivity:

```bash
docker exec frappe-mcp-server-frappe-mcp-server-1 curl http://erpnext:8000/api/method/ping
```

### View Logs

```bash
# MCP server logs
docker logs -f frappe-mcp-server-frappe-mcp-server-1

# ERPNext logs
docker exec 281a931ec5d5 tail -f /home/frappe/frappe-bench/sites/dev.localhost/logs/web.log
```

## Next Steps

1. ✅ **Test with Awesome Bar** - Primary use case
2. ⏭️ **Add API key support** - For Open WebUI (when OAuth doesn't work)
3. ⏭️ **Session refresh** - Auto-refresh expired sessions
4. ⏭️ **Production setup** - Enable `require_auth: true`

## Architecture Diagram

```
┌──────────────────────────────────────────────┐
│  User logged into ERPNext                    │
│  Has: sid cookie (abc123xyz)                │
└───────────────┬──────────────────────────────┘
                │
                │ @ai show me customers
                ▼
┌──────────────────────────────────────────────┐
│  Frappe App (frappe_ai)                      │
│  - Reads frappe.session.sid                  │
│  - Sends: Cookie: sid=abc123xyz              │
└───────────────┬──────────────────────────────┘
                │
                ▼
┌──────────────────────────────────────────────┐
│  MCP Server                                   │
│  1. Extracts sid from Cookie header          │
│  2. Validates sid with Frappe                │
│  3. Stores sid in User context               │
└───────────────┬──────────────────────────────┘
                │
                │ Multiple API calls
                │ All with: Cookie: sid=abc123xyz
                ▼
┌──────────────────────────────────────────────┐
│  Frappe/ERPNext                               │
│  - Validates sid per request                  │
│  - Returns data based on user's permissions  │
│  ✅ User-level security maintained            │
└──────────────────────────────────────────────┘
```

## Files Changed

1. ✅ `/Users/varkrish/personal/frappista_sne_apps/apps/frappe_ai/frappe_ai/api/ai_query.py`
2. ✅ `/Users/varkrish/personal/frappe-mcp-server/internal/types/types.go`
3. ✅ `/Users/varkrish/personal/frappe-mcp-server/internal/auth/strategies/oauth2.go`
4. ✅ `/Users/varkrish/personal/frappe-mcp-server/internal/frappe/client.go`

All changes are backward compatible. OAuth2 still works if configured.

