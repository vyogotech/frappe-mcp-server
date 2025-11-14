# OAuth2 Testing - Quick Reference

## 🚀 Quick Start (Choose One)

### Option A: Automated (Easiest)
```bash
# 1. Create OAuth2 client
python3 scripts/create-oauth-client.py

# 2. Run tests
./test-oauth.sh
```

### Option B: Manual
```bash
# 1. Create OAuth client in Frappe UI
open http://localhost:8000/app/oauth-client

# 2. Export credentials
export OAUTH_CLIENT_ID='your-id'
export OAUTH_CLIENT_SECRET='your-secret'

# 3. Run tests
./test-oauth.sh
```

## 📋 Testing Checklist

- [ ] Frappe running at `http://localhost:8000`
- [ ] MCP server running at `http://localhost:8080`
- [ ] OAuth2 client created in Frappe
- [ ] Client ID added to `config.yaml` trusted_clients
- [ ] `jq` installed (`brew install jq`)

## 🧪 Manual Tests

### Get Token
```bash
TOKEN=$(curl -s -X POST http://localhost:8000/api/method/frappe.integrations.oauth2.get_token \
  -d "grant_type=client_credentials" \
  -d "client_id=$OAUTH_CLIENT_ID" \
  -d "client_secret=$OAUTH_CLIENT_SECRET" \
  | jq -r '.access_token')

echo $TOKEN
```

### Test Authenticated Request
```bash
curl -X POST http://localhost:8080/api/v1/chat \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"message": "List all projects"}' | jq '.'
```

### Test with User Context
```bash
curl -X POST http://localhost:8080/api/v1/chat \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-MCP-User-ID: admin@example.com" \
  -H "Content-Type: application/json" \
  -d '{"message": "Show my tasks"}' | jq '.'
```

## ⚙️ Config Modes

### Development (Optional Auth)
```yaml
auth:
  enabled: true
  require_auth: false  # Requests work with or without token
```

### Production (Required Auth)
```yaml
auth:
  enabled: true
  require_auth: true   # All requests must have valid token
```

## 🔍 Troubleshooting

| Issue | Quick Fix |
|-------|-----------|
| "Failed to get token" | Check client ID/secret, verify Frappe is running |
| "401 Unauthorized" | Verify token is valid: `curl -H "Authorization: Bearer $TOKEN" http://localhost:8000/api/method/frappe.integrations.oauth2.openid.userinfo` |
| "Connection refused" | Start MCP: `go run main.go --config config.yaml` |
| "Token validation failed" | Check `token_info_url` in config matches Frappe URL |

## 📂 Files Created

- `test-oauth.sh` - Automated OAuth2 testing script
- `scripts/create-oauth-client.py` - Create OAuth2 client programmatically  
- `TESTING_OAUTH2.md` - Complete testing guide
- `docs/auth-quickstart.md` - OAuth2 quick start guide

## 🎯 Your Current Setup

**Config Location:**
```
/Users/varkrish/personal/frappista_sne_apps/erpnext-mcp-server/config.yaml
```

**Current Auth Settings:**
- ✅ Auth enabled: `true`
- ✅ Require auth: `true`
- ✅ Token validation: `true`
- ✅ Trusted client: `frappe-mcp-backend`

## 🎬 Quick Test Now

```bash
# Go to project directory
cd /Users/varkrish/personal/frappe-mcp-server

# Create OAuth client (first time only)
python3 scripts/create-oauth-client.py

# Run comprehensive tests
./test-oauth.sh
```

## 📚 Full Documentation

- Complete Guide: `TESTING_OAUTH2.md`
- Auth Quick Start: `docs/auth-quickstart.md`
- Full Auth Docs: `docs/authentication.md`
- OAuth2 Implementation: `docs/oauth2-implementation.md`

---

**Need help? Check `TESTING_OAUTH2.md` for detailed instructions!**

