# OAuth2 Authentication - Quick Start Guide

Get up and running with OAuth2 authentication in 5 minutes.

## Prerequisites

- Running Frappe/ERPNext instance
- MCP Server installed
- Basic understanding of OAuth2

## Quick Setup (5 Minutes)

### Step 1: Create OAuth2 Client in Frappe (2 min)

```bash
# Navigate to OAuth Client in Frappe
http://localhost:8000/app/oauth-client
```

Click **New** and enter:
- **App Name**: `MCP Backend Integration`
- **Scopes**: `openid profile email all`
- **Grant Type**: `Client Credentials`

Save and copy the **Client ID** and **Client Secret**.

### Step 2: Configure MCP Server (1 min)

Create or update `config.yaml`:

```yaml
auth:
  enabled: true
  require_auth: false  # Start with optional auth
  oauth2:
    token_info_url: "http://localhost:8000/api/method/frappe.integrations.oauth2.openid.userinfo"
    issuer_url: "http://localhost:8000"
    trusted_clients:
      - "YOUR_CLIENT_ID_HERE"  # Replace with actual client ID
    validate_remote: true
    timeout: "30s"
  token_cache:
    ttl: "5m"
    cleanup_interval: "10m"
```

Or use environment variables:

```bash
export AUTH_ENABLED=true
export AUTH_REQUIRE_AUTH=false
export OAUTH_TOKEN_INFO_URL=http://localhost:8000/api/method/frappe.integrations.oauth2.openid.userinfo
export OAUTH_ISSUER_URL=http://localhost:8000
```

### Step 3: Test Authentication (2 min)

```bash
# Get OAuth2 token
TOKEN=$(curl -s -X POST http://localhost:8000/api/method/frappe.integrations.oauth2.get_token \
  -d "grant_type=client_credentials" \
  -d "client_id=YOUR_CLIENT_ID" \
  -d "client_secret=YOUR_CLIENT_SECRET" \
  | jq -r '.access_token')

# Test authenticated request
curl -X POST http://localhost:8080/api/v1/chat \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"message": "List all projects"}'
```

✅ **Success!** You now have OAuth2 authentication enabled.

## Configuration Modes

### Mode 1: Development (Optional Auth)

Best for: Local development, testing, backward compatibility

```yaml
auth:
  enabled: true
  require_auth: false  # Requests work with or without auth
```

**Pros**: Easy to test, backward compatible  
**Cons**: Less secure

### Mode 2: Production (Required Auth)

Best for: Production deployments, sensitive data

```yaml
auth:
  enabled: true
  require_auth: true  # All requests must have valid auth
```

**Pros**: Secure, enforces authentication  
**Cons**: All clients must implement OAuth2

### Mode 3: Disabled (No Auth)

Best for: Internal networks, legacy systems

```yaml
auth:
  enabled: false  # No authentication required
```

**Pros**: Simplest setup  
**Cons**: No security

## Common Scenarios

### Scenario 1: Frappe Backend Integration

**Use Case**: Frappe app calling MCP server on behalf of users

**Configuration**:
```yaml
auth:
  enabled: true
  require_auth: false
  oauth2:
    trusted_clients:
      - "frappe-mcp-backend"  # Your backend client ID
```

**Python Code**:
```python
import frappe
import requests

def call_mcp(message):
    # Get OAuth2 token (cached)
    token = get_access_token()
    
    # Get current user
    user = frappe.session.user
    
    # Call MCP with user context
    response = requests.post(
        "http://localhost:8080/api/v1/chat",
        headers={
            "Authorization": f"Bearer {token}",
            "X-MCP-User-ID": user,
            "X-MCP-User-Email": frappe.db.get_value("User", user, "email"),
            "Content-Type": "application/json"
        },
        json={"message": message}
    )
    
    return response.json()
```

### Scenario 2: Mobile App

**Use Case**: Mobile app with user login

**Configuration**:
```yaml
auth:
  enabled: true
  require_auth: true  # All users must authenticate
```

**Mobile App Flow**:
1. User logs in via Frappe OAuth2 (Authorization Code flow)
2. App receives access token
3. App includes token in all API requests

```javascript
// JavaScript/React Native example
const token = await getOAuth2Token();

const response = await fetch('http://api.example.com/api/v1/chat', {
  method: 'POST',
  headers: {
    'Authorization': `Bearer ${token}`,
    'Content-Type': 'application/json',
  },
  body: JSON.stringify({ message: 'Show projects' }),
});
```

### Scenario 3: Service-to-Service

**Use Case**: Microservices calling MCP server

**Configuration**:
```yaml
auth:
  enabled: true
  require_auth: true
  oauth2:
    trusted_clients:
      - "service-a"
      - "service-b"
```

**Service Code**:
```go
func callMCP(message string) error {
    token, err := getServiceToken()
    if err != nil {
        return err
    }
    
    req, _ := http.NewRequest("POST", "http://mcp:8080/api/v1/chat", 
        bytes.NewBuffer([]byte(`{"message": "` + message + `"}`)))
    
    req.Header.Set("Authorization", "Bearer " + token)
    req.Header.Set("Content-Type", "application/json")
    
    resp, err := client.Do(req)
    // ... handle response
}
```

## Migration Path

### Phase 1: Enable Optional Auth (Week 1)

```yaml
auth:
  enabled: true
  require_auth: false
```

**Action**: Deploy and monitor. Requests work with or without auth.

### Phase 2: Update Clients (Week 2-3)

- Update all clients to send OAuth2 tokens
- Monitor logs for authentication success/failure
- Fix any client authentication issues

### Phase 3: Enforce Required Auth (Week 4)

```yaml
auth:
  enabled: true
  require_auth: true  # All requests now require auth
```

**Action**: Deploy and verify all clients are working.

## Troubleshooting

### Error: "Unauthorized"

**Check**:
1. Is `AUTH_ENABLED=true`?
2. Is the token valid? (not expired?)
3. Is the token validation URL correct?

**Fix**:
```bash
# Test token validation manually
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8000/api/method/frappe.integrations.oauth2.openid.userinfo
```

### Error: "Token validation timeout"

**Check**:
1. Can MCP reach Frappe? `curl http://frappe:8000/health`
2. Is Frappe responding? Check Frappe logs
3. Is timeout too low?

**Fix**:
```yaml
auth:
  oauth2:
    timeout: "60s"  # Increase timeout
```

### Error: "No user in context"

**Check**:
1. Is client in `trusted_clients` list?
2. Are `X-MCP-User-*` headers being sent?

**Fix**:
```yaml
auth:
  oauth2:
    trusted_clients:
      - "your-client-id"  # Add your client ID
```

## Testing

### Test 1: Health Check (No Auth)

```bash
curl http://localhost:8080/api/v1/health
# Should return: {"status":"healthy"}
```

### Test 2: Authenticated Request

```bash
# Get token
TOKEN=$(curl -s -X POST http://localhost:8000/api/method/frappe.integrations.oauth2.get_token \
  -d "grant_type=client_credentials" \
  -d "client_id=$CLIENT_ID" \
  -d "client_secret=$CLIENT_SECRET" \
  | jq -r '.access_token')

# Use token
curl -X POST http://localhost:8080/api/v1/chat \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"message": "test"}'
```

### Test 3: Invalid Token

```bash
curl -X POST http://localhost:8080/api/v1/chat \
  -H "Authorization: Bearer invalid-token" \
  -d '{"message": "test"}'
# Should return: 401 Unauthorized (if require_auth: true)
# Should succeed (if require_auth: false)
```

## Performance Tips

1. **Cache tokens**: Don't fetch a new token for every request
2. **Increase cache TTL**: Reduce validation overhead
   ```yaml
   token_cache:
     ttl: "10m"  # Default: 5m
   ```
3. **Use connection pooling**: Reuse HTTP connections to Frappe
4. **Monitor cache hit rate**: Log cache hits vs. misses

## Security Checklist

- [ ] Use HTTPS in production
- [ ] Store client secrets securely (environment variables, secrets manager)
- [ ] Enable `require_auth: true` in production
- [ ] Set strong client secrets (32+ characters)
- [ ] Regularly rotate client secrets
- [ ] Monitor authentication failures
- [ ] Implement rate limiting
- [ ] Log security events

## Next Steps

- Read the [full authentication documentation](./authentication.md)
- Implement [RBAC with Frappe permissions](./rbac.md) (coming soon)
- Set up [monitoring and metrics](./monitoring.md) (coming soon)
- Review [security best practices](./security.md) (coming soon)

## Support

- **Issues**: [GitHub Issues](https://github.com/your-repo/issues)
- **Documentation**: [Full Docs](./authentication.md)
- **Community**: [Frappe Forum](https://discuss.frappe.io)






