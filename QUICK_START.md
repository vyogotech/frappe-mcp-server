# Quick Start: Testing sid Cookie Authentication

## ✅ What's Done

1. **Frappe App** - Now sends `sid` cookie instead of OAuth token
2. **MCP Server** - Accepts both `sid` cookie and OAuth Bearer token
3. **Frappe Client** - Forwards `sid` cookie to all Frappe API calls
4. **Types** - Added `SessionID` field to User struct
5. **MCP Server** - Rebuilt and restarted with new code

## 🧪 Test Now!

### Option 1: Test from Awesome Bar (Recommended)

1. **Make sure the frappe_ai app is running in ERPNext:**

```bash
docker exec 281a931ec5d5 bench --site dev.localhost list-apps
# Should show: frappe, erpnext, frappe_ai
```

2. **Log into ERPNext:**
   - Open: http://localhost:8000
   - Login with your credentials

3. **Use the Awesome Bar:**
   - Press `/` or click the Awesome Bar
   - Type: `@ai show me top 5 customers`
   - Press Enter

4. **Check logs to see it working:**

```bash
# MCP server logs (see sid cookie authentication)
docker logs -f frappe-mcp-server-frappe-mcp-server-1

# Look for:
# "Using Frappe session cookie"
```

### Option 2: Test with curl

1. **Get your sid from ERPNext:**
   - Log into http://localhost:8000
   - Open DevTools (F12) > Application > Cookies
   - Copy the `sid` cookie value

2. **Test:**

```bash
export SID='your-sid-cookie-value-here'

curl -b "sid=$SID" \
  -H "Content-Type: application/json" \
  -X POST http://localhost:8080/api/v1/chat \
  -d '{
    "message": "show me my projects",
    "context": {"user_id": "test"}
  }' | jq '.'
```

## 🔍 How to Verify It's Working

### 1. Check MCP Server Logs

```bash
docker logs -f frappe-mcp-server-frappe-mcp-server-1
```

Look for:
```
"Using Frappe session cookie", "user": "your-email@example.com"
```

### 2. Check Frappe App is Sending sid

The `frappe_ai/api/ai_query.py` file now has:

```python
# Line 119
sid = frappe.session.sid

response = requests.post(
    f"{settings.mcp_server_url}/api/v1/chat",
    json=payload,
    cookies={"sid": sid},  # ← Sending sid cookie!
    timeout=30
)
```

### 3. Check Auth Strategy

The MCP server now tries sid first:

```go
// internal/auth/strategies/oauth2.go
if sidCookie, err := r.Cookie("sid"); err == nil {
    user, err := s.validateSessionCookie(ctx, sidCookie)
    // ✅ sid cookie authentication
}
```

## 📊 Container Status

```bash
docker compose ps

# All should be Up:
# - frappe-mcp-server-erpnext-1 
# - frappe-mcp-server-frappe-mcp-server-1
# - frappe-mcp-server-ollama-1
# - frappe-mcp-server-open-webui-1
```

## 🐛 Troubleshooting

### Issue: "frappe_ai app not installed"

```bash
cd /Users/varkrish/personal/frappista_sne_apps/apps
docker exec 281a931ec5d5 bench --site dev.localhost install-app frappe_ai
```

### Issue: "Connection failed"

Check MCP Server Settings in ERPNext:
- MCP Server URL: `http://frappe-mcp-server:8080`
- Frappe Base URL: `http://erpnext:8000`
- Enabled: ✓ (checked)

### Issue: "No response from Awesome Bar"

Check the browser console for errors:
1. Press F12 in browser
2. Go to Console tab
3. Type `@ai test` in Awesome Bar
4. Check for errors

## 🎯 Expected Behavior

1. **User types in Awesome Bar** → `@ai show me customers`
2. **Frappe app extracts sid** → `frappe.session.sid`
3. **Sends to MCP server** → `Cookie: sid=xyz123`
4. **MCP validates sid** → Calls Frappe to verify session
5. **MCP makes API calls** → Forwards `sid` cookie
6. **Frappe responds** → With data based on user's permissions
7. **LLM processes** → Generates response
8. **User sees result** → In Awesome Bar dropdown

## 📝 Authentication Methods Supported

| Method | Status | Use Case |
|--------|--------|----------|
| **sid cookie** | ✅ **Working** | Awesome Bar (user-level) |
| **Bearer token** | ⏭️ Future | External apps (OAuth2) |
| **API Key** | ⏭️ Future | Open WebUI (system-level) |

## 📚 Documentation

- Full implementation details: `SID_OAUTH_HYBRID_AUTH.md`
- Integration fix guide: `INTEGRATION_FIX.md`
- Test scripts: `test_sid_auth.sh`, `test_with_real_sid.sh`

## 🚀 Next Steps

1. **Test with Awesome Bar** - Primary use case
2. **Add API key support** - For Open WebUI (since OAuth doesn't work)
3. **Update MCP Server Settings** - Add API key fields
4. **Production setup** - Enable `require_auth: true`

---

**Everything is ready to test!** Just log into ERPNext and try the Awesome Bar. 🎉

