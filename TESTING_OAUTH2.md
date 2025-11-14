# Testing OAuth2 Authentication

This guide walks you through testing OAuth2 authentication in the Frappe MCP Server.

## Prerequisites

- ✅ Frappe/ERPNext instance running at `http://localhost:8000`
- ✅ MCP Server built and ready to run
- ✅ `jq` installed (`brew install jq` on macOS)
- ✅ `curl` available

## Quick Test (5 Minutes)

### Option 1: Automated Setup (Recommended)

Use the automated script to create an OAuth2 client and test:

```bash
# 1. Create OAuth2 client (requires your Frappe API credentials)
python3 scripts/create-oauth-client.py

# 2. Run the test script (it will prompt for client ID/secret)
./test-oauth.sh
```

### Option 2: Manual Setup

#### Step 1: Create OAuth2 Client in Frappe

1. **Navigate to OAuth Client**:
   ```
   http://localhost:8000/app/oauth-client
   ```

2. **Click "New"** and fill in:
   - **App Name**: `MCP Backend Integration`
   - **Scopes**: Check all or type: `openid profile email all`
   - **Grant Type**: Select `Client Credentials`
   - **Skip Authorization**: ✅ (check this)

3. **Save** and copy:
   - **Client ID** (e.g., `abc123xyz`)
   - **Client Secret** (e.g., `secret456def`)

#### Step 2: Update Your Config

Add the client ID to your `config.yaml` trusted clients:

```yaml
auth:
  enabled: true
  require_auth: false  # Start with optional auth for testing
  oauth2:
    token_info_url: "http://localhost:8000/api/method/frappe.integrations.oauth2.openid.userinfo"
    issuer_url: "http://localhost:8000"
    trusted_clients:
      - "YOUR_CLIENT_ID_HERE"  # ← Replace with actual client ID
    validate_remote: true
    timeout: "30s"
  token_cache:
    ttl: "5m"
    cleanup_interval: "10m"
```

#### Step 3: Start MCP Server

```bash
# With HTTP API (for testing)
cd /Users/varkrish/personal/frappe-mcp-server
go run main.go --config /Users/varkrish/personal/frappista_sne_apps/erpnext-mcp-server/config.yaml

# Or with the built binary
./bin/frappe-mcp-server-stdio --config /Users/varkrish/personal/frappista_sne_apps/erpnext-mcp-server/config.yaml
```

#### Step 4: Run Tests

```bash
# Export your credentials
export OAUTH_CLIENT_ID='your-client-id'
export OAUTH_CLIENT_SECRET='your-client-secret'

# Run the test script
./test-oauth.sh
```

## Manual Testing (Command Line)

### Test 1: Get OAuth2 Token

```bash
curl -X POST http://localhost:8000/api/method/frappe.integrations.oauth2.get_token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=client_credentials" \
  -d "client_id=YOUR_CLIENT_ID" \
  -d "client_secret=YOUR_CLIENT_SECRET" \
  | jq '.'
```

Expected output:
```json
{
  "access_token": "eyJ0eXAiOiJKV1QiLCJhbGc...",
  "token_type": "Bearer",
  "expires_in": 3600,
  "scope": "openid profile email all"
}
```

### Test 2: Validate Token with Frappe

```bash
# Save token from previous step
TOKEN="your-access-token"

curl -X GET http://localhost:8000/api/method/frappe.integrations.oauth2.openid.userinfo \
  -H "Authorization: Bearer $TOKEN" \
  | jq '.'
```

Expected output:
```json
{
  "sub": "Administrator",
  "email": "admin@example.com",
  "name": "Administrator"
}
```

### Test 3: Call MCP Server with Authentication

```bash
# Using the token from Test 1
curl -X POST http://localhost:8080/api/v1/chat \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "message": "List all projects"
  }' | jq '.'
```

### Test 4: Call MCP with User Context (Trusted Client)

```bash
curl -X POST http://localhost:8080/api/v1/chat \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-MCP-User-ID: admin@example.com" \
  -H "X-MCP-User-Email: admin@example.com" \
  -H "X-MCP-User-Name: Administrator" \
  -H "Content-Type: application/json" \
  -d '{
    "message": "Show my tasks"
  }' | jq '.'
```

### Test 5: Test Unauthenticated Request

```bash
# Without Authorization header
curl -X POST http://localhost:8080/api/v1/chat \
  -H "Content-Type: application/json" \
  -d '{
    "message": "Test without auth"
  }'
```

**Expected behavior:**
- If `require_auth: true` → Returns `401 Unauthorized`
- If `require_auth: false` → Returns `200 OK` (uses fallback API key)

### Test 6: Test Invalid Token

```bash
curl -X POST http://localhost:8080/api/v1/chat \
  -H "Authorization: Bearer invalid-token" \
  -H "Content-Type: application/json" \
  -d '{
    "message": "Test with invalid token"
  }'
```

**Expected:** Returns `401 Unauthorized`

## Testing Different Modes

### Mode 1: Optional Auth (Development)

**Config:**
```yaml
auth:
  enabled: true
  require_auth: false
```

**Behavior:**
- ✅ Requests with valid token → Use OAuth2 authentication
- ✅ Requests without token → Use fallback API key (if configured)
- ❌ Requests with invalid token → Reject with 401

**Use case:** Development, gradual migration

### Mode 2: Required Auth (Production)

**Config:**
```yaml
auth:
  enabled: true
  require_auth: true
```

**Behavior:**
- ✅ Requests with valid token → Use OAuth2 authentication
- ❌ Requests without token → Reject with 401
- ❌ Requests with invalid token → Reject with 401

**Use case:** Production, sensitive data

### Mode 3: Disabled (No Auth)

**Config:**
```yaml
auth:
  enabled: false
```

**Behavior:**
- ✅ All requests succeed
- Uses API key from config

**Use case:** Internal networks, backward compatibility

## Troubleshooting

### Issue: "Failed to get access token"

**Possible causes:**
1. Wrong client ID/secret
2. Frappe instance not accessible
3. OAuth2 client not properly configured

**Solution:**
```bash
# Check Frappe is accessible
curl http://localhost:8000

# Verify OAuth client exists
# Go to: http://localhost:8000/app/oauth-client
```

### Issue: "Token validation failed"

**Possible causes:**
1. Token expired
2. Token info URL incorrect
3. Network issue between MCP and Frappe

**Solution:**
```bash
# Test token validation manually
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8000/api/method/frappe.integrations.oauth2.openid.userinfo
```

### Issue: "401 Unauthorized" from MCP

**Possible causes:**
1. `require_auth: true` but no token provided
2. Invalid token
3. Auth not properly configured

**Solution:**
```bash
# Check MCP server logs for detailed error
# Verify config:
cat config.yaml | grep -A 20 "auth:"
```

### Issue: "Connection refused"

**Possible causes:**
1. MCP server not running
2. Wrong port

**Solution:**
```bash
# Check if MCP is running
curl http://localhost:8080/api/v1/health

# Start MCP server
go run main.go --config config.yaml
```

## Integration with Your App

### Python Example

```python
import requests
from datetime import datetime, timedelta

class MCPClient:
    def __init__(self, mcp_url, frappe_url, client_id, client_secret):
        self.mcp_url = mcp_url
        self.frappe_url = frappe_url
        self.client_id = client_id
        self.client_secret = client_secret
        self.token = None
        self.token_expires_at = None
    
    def get_token(self):
        """Get or refresh OAuth2 token."""
        if self.token and self.token_expires_at > datetime.now():
            return self.token
        
        response = requests.post(
            f"{self.frappe_url}/api/method/frappe.integrations.oauth2.get_token",
            data={
                "grant_type": "client_credentials",
                "client_id": self.client_id,
                "client_secret": self.client_secret,
            }
        )
        
        data = response.json()
        self.token = data["access_token"]
        self.token_expires_at = datetime.now() + timedelta(seconds=data["expires_in"] - 60)
        
        return self.token
    
    def chat(self, message, user_id=None):
        """Send a message to MCP server."""
        headers = {
            "Authorization": f"Bearer {self.get_token()}",
            "Content-Type": "application/json"
        }
        
        if user_id:
            headers["X-MCP-User-ID"] = user_id
        
        response = requests.post(
            f"{self.mcp_url}/api/v1/chat",
            headers=headers,
            json={"message": message}
        )
        
        return response.json()

# Usage
client = MCPClient(
    mcp_url="http://localhost:8080",
    frappe_url="http://localhost:8000",
    client_id="your-client-id",
    client_secret="your-client-secret"
)

result = client.chat("List all projects")
print(result)
```

### JavaScript/Node.js Example

```javascript
const axios = require('axios');

class MCPClient {
    constructor(mcpUrl, frappeUrl, clientId, clientSecret) {
        this.mcpUrl = mcpUrl;
        this.frappeUrl = frappeUrl;
        this.clientId = clientId;
        this.clientSecret = clientSecret;
        this.token = null;
        this.tokenExpiresAt = null;
    }
    
    async getToken() {
        if (this.token && this.tokenExpiresAt > Date.now()) {
            return this.token;
        }
        
        const params = new URLSearchParams();
        params.append('grant_type', 'client_credentials');
        params.append('client_id', this.clientId);
        params.append('client_secret', this.clientSecret);
        
        const response = await axios.post(
            `${this.frappeUrl}/api/method/frappe.integrations.oauth2.get_token`,
            params
        );
        
        this.token = response.data.access_token;
        this.tokenExpiresAt = Date.now() + (response.data.expires_in - 60) * 1000;
        
        return this.token;
    }
    
    async chat(message, userId = null) {
        const token = await this.getToken();
        
        const headers = {
            'Authorization': `Bearer ${token}`,
            'Content-Type': 'application/json'
        };
        
        if (userId) {
            headers['X-MCP-User-ID'] = userId;
        }
        
        const response = await axios.post(
            `${this.mcpUrl}/api/v1/chat`,
            { message },
            { headers }
        );
        
        return response.data;
    }
}

// Usage
const client = new MCPClient(
    'http://localhost:8080',
    'http://localhost:8000',
    'your-client-id',
    'your-client-secret'
);

client.chat('List all projects').then(result => {
    console.log(result);
});
```

## Performance Considerations

1. **Token Caching**: Tokens are valid for 3600 seconds (1 hour). Cache them!
2. **Token Validation Cache**: MCP caches validated tokens for 5 minutes (configurable)
3. **Connection Pooling**: Reuse HTTP connections to Frappe
4. **Rate Limiting**: Be mindful of Frappe API rate limits

## Security Checklist

- [ ] Use HTTPS in production
- [ ] Store client secrets in environment variables or secrets manager
- [ ] Enable `require_auth: true` in production
- [ ] Regularly rotate client secrets
- [ ] Monitor authentication failures
- [ ] Implement rate limiting
- [ ] Use strong client secrets (32+ characters)

## Next Steps

1. ✅ Test OAuth2 with automated script
2. ✅ Integrate OAuth2 in your client application
3. ✅ Deploy to staging with `require_auth: false`
4. ✅ Test all clients
5. ✅ Deploy to production with `require_auth: true`

## Support

- **Documentation**: [Full OAuth2 Docs](./docs/authentication.md)
- **Quick Start**: [OAuth2 Quick Start](./docs/auth-quickstart.md)
- **Issues**: [GitHub Issues](https://github.com/your-repo/issues)

---

**Happy Testing! 🚀**

