# Frappe OAuth2 Setup Guide

## Important: Frappe's OAuth2 Grant Types

Frappe's OAuth2 UI **only shows 2 grant types**:
- ✅ **Authorization Code** 
- ✅ **Implicit**

❌ **Client Credentials is NOT available in the UI**

## ✅ Solution: Use Authorization Code with Skip Authorization

For backend services (like your MCP server), use **Authorization Code** with **"Skip Authorization"** enabled. This makes it work like Client Credentials for trusted backend applications.

## 🔧 Step-by-Step Setup

### Step 1: Create OAuth2 Client in Frappe

1. **Navigate to OAuth Client:**
   ```
   http://localhost:8000/app/oauth-client
   ```

2. **Click "New" and fill in:**

   ```
   ┌─────────────────────────────────────────────┐
   │                                             │
   │  App Name: MCP Backend Integration          │
   │                                             │
   │  Scopes:                                    │
   │    ☑ openid                                │
   │    ☑ profile                               │
   │    ☑ email                                 │
   │    ☑ all                                   │
   │                                             │
   │  Grant Type:                                │
   │    ☑ Authorization Code  ← Select this     │
   │    ☐ Implicit            ← Don't use       │
   │                                             │
   │  Redirect URIs:                             │
   │    http://localhost                         │
   │                                             │
   │  Default Redirect URI:                      │
   │    http://localhost                         │
   │                                             │
   │  ⚠️  CRITICAL SETTING:                      │
   │  Skip Authorization: ☑  ← CHECK THIS!      │
   │                                             │
   │  (Makes it work for backend services)       │
   └─────────────────────────────────────────────┘
   ```

3. **Save** and copy:
   - **Client ID** (e.g., `abc123xyz`)
   - **Client Secret** (e.g., `secret456def`)

### Step 2: Update MCP Server Config

Add your client ID to the trusted clients list:

```yaml
# /Users/varkrish/personal/frappista_sne_apps/erpnext-mcp-server/config.yaml

auth:
  enabled: true
  require_auth: false  # Start with optional for testing
  oauth2:
    token_info_url: "http://localhost:8000/api/method/frappe.integrations.oauth2.openid.userinfo"
    issuer_url: "http://localhost:8000"
    trusted_clients:
      - "YOUR_CLIENT_ID_HERE"  # ← Add your actual client ID
    validate_remote: true
    timeout: "30s"
```

### Step 3: Get OAuth2 Token

Since Frappe uses Authorization Code grant, we have a helper script:

```bash
cd /Users/varkrish/personal/frappe-mcp-server

# Run the Authorization Code token generator
python3 scripts/get-oauth-token-authcode.py
```

It will prompt for:
- Frappe URL (default: http://localhost:8000)
- OAuth Client ID
- OAuth Client Secret
- Frappe username (e.g., Administrator)
- Frappe password

The script will:
1. ✅ Log in to Frappe with your credentials
2. ✅ Get authorization code
3. ✅ Exchange code for access token
4. ✅ Display the token for testing

## 🧪 Testing OAuth2

### Test 1: Manual Token Request

```bash
# Variables
FRAPPE_URL="http://localhost:8000"
CLIENT_ID="your-client-id"
CLIENT_SECRET="your-client-secret"
USERNAME="Administrator"
PASSWORD="admin"

# Step 1: Login and get authorization code (requires browser or automated script)
# Use the script above, or:

# Step 2: Exchange code for token
curl -X POST "${FRAPPE_URL}/api/method/frappe.integrations.oauth2.get_token" \
  -d "grant_type=authorization_code" \
  -d "code=YOUR_AUTH_CODE" \
  -d "redirect_uri=http://localhost" \
  -d "client_id=${CLIENT_ID}" \
  -d "client_secret=${CLIENT_SECRET}"
```

### Test 2: Use Token with MCP Server

```bash
# Get token using the script
TOKEN=$(python3 scripts/get-oauth-token-authcode.py | grep "Access Token:" | awk '{print $3}')

# Test MCP API
curl -X POST http://localhost:8080/api/v1/chat \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"message": "List all projects"}' | jq '.'
```

### Test 3: Run Comprehensive Tests

```bash
# Update and run the test script
export OAUTH_CLIENT_ID='your-client-id'
export OAUTH_CLIENT_SECRET='your-client-secret'
export FRAPPE_USERNAME='Administrator'
export FRAPPE_PASSWORD='admin'

./test-oauth.sh
```

## 🔄 Comparison: Authorization Code vs Client Credentials

| Feature | Client Credentials | Authorization Code (Skip Auth) |
|---------|-------------------|--------------------------------|
| **User Login** | ❌ No | ✅ Yes (one-time) |
| **Token Lifetime** | Long | Shorter (can refresh) |
| **User Context** | ❌ No | ✅ Yes (optional) |
| **Setup Complexity** | Simple | Moderate |
| **Frappe Support** | ❌ Not in UI | ✅ Available |

## 💡 Practical Recommendations

### For Your Current Setup (STDIO Mode - Cursor)

**Continue using API keys** - it's simpler and works perfectly for STDIO mode:

```yaml
erpnext:
  base_url: "http://localhost:8000"
  api_key: "0d9f1b19563768b"
  api_secret: "9c2d83ff0906fd6"

auth:
  enabled: false  # Or enabled: true, require_auth: false for optional OAuth
```

### For HTTP Mode (Web Clients like Open WebUI)

**Use OAuth2 with Authorization Code grant:**

1. Create OAuth client (as shown above)
2. Enable auth in config
3. Web clients get tokens via OAuth flow
4. User-level permissions enforced

### Hybrid Approach (Both)

```yaml
erpnext:
  base_url: "http://localhost:8000"
  api_key: "0d9f1b19563768b"      # For STDIO and fallback
  api_secret: "9c2d83ff0906fd6"   # For STDIO and fallback

auth:
  enabled: true
  require_auth: false              # HTTP can use OAuth OR fallback to API keys
  oauth2:
    # ... OAuth2 config
```

**Benefits:**
- ✅ STDIO clients (Cursor) use API keys
- ✅ HTTP clients can use OAuth2
- ✅ Gradual migration path
- ✅ Backward compatible

## ❓ Troubleshooting

### Issue: "Skip Authorization" option not visible

**Solution**: Update Frappe to latest version or manually set in database:

```sql
UPDATE `tabOAuth Client` 
SET skip_authorization = 1 
WHERE name = 'your-client-id';
```

### Issue: Token request fails

**Check:**
1. ✅ OAuth client exists in Frappe
2. ✅ Client ID and secret are correct
3. ✅ User credentials are valid
4. ✅ Redirect URI matches exactly
5. ✅ Skip Authorization is enabled

### Issue: MCP returns 401 Unauthorized

**Check:**
1. ✅ Token is still valid (not expired)
2. ✅ Client ID is in `trusted_clients` list in config.yaml
3. ✅ `auth.enabled` is true in config
4. ✅ Token validation URL is correct

## 📚 Related Documentation

- [OAuth2 Testing Guide](./TESTING_OAUTH2.md)
- [Quick Reference](./OAUTH_TESTING_QUICKREF.md)
- [Full Authentication Docs](./docs/authentication.md)
- [OAuth2 Implementation Details](./docs/oauth2-implementation.md)

## 🎯 Summary

**For Frappe OAuth2:**
- ❌ Client Credentials grant NOT available in UI
- ✅ Use Authorization Code grant instead
- ✅ Enable "Skip Authorization" for backend services
- ✅ Works just like Client Credentials for trusted apps

**Your Setup:**
- STDIO mode (Cursor) → Use API keys (simpler, works great!)
- HTTP mode (web clients) → Use OAuth2 (user-level permissions)
- Both → Hybrid config (best of both worlds)

---

**Questions? Run the helper script:**
```bash
python3 scripts/get-oauth-token-authcode.py
```






