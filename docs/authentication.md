# OAuth2 Authentication

This document describes how to set up and use OAuth2 authentication with the Frappe MCP Server.

## Overview

The Frappe MCP Server supports **standard OAuth2 authentication** with the following features:

- **Client Credentials Grant**: For backend-to-backend authentication (Frappe backend → MCP)
- **Authorization Code Grant**: For external clients like mobile apps, VS Code extensions
- **JWT Validation**: Token validation with Frappe OAuth2 provider
- **Token Caching**: In-memory caching to reduce validation overhead
- **Optional Authentication**: Backward compatible mode for gradual migration
- **Trusted Clients**: Special handling for trusted backend clients that can provide user context

## Architecture

### Authentication Flow

```
┌─────────────┐                  ┌──────────────┐                 ┌────────────┐
│   Client    │                  │  MCP Server  │                 │   Frappe   │
│ (Frappe App)│                  │              │                 │  OAuth2    │
└──────┬──────┘                  └──────┬───────┘                 └─────┬──────┘
       │                                │                               │
       │  1. Get OAuth2 Token          │                               │
       │───────────────────────────────────────────────────────────────>│
       │                                │                               │
       │  2. Access Token               │                               │
       │<───────────────────────────────────────────────────────────────│
       │                                │                               │
       │  3. API Request + Bearer Token │                               │
       │     + User Context Headers     │                               │
       │───────────────────────────────>│                               │
       │                                │                               │
       │                                │  4. Validate Token            │
       │                                │──────────────────────────────>│
       │                                │                               │
       │                                │  5. Token Info + User Data    │
       │                                │<──────────────────────────────│
       │                                │                               │
       │  6. API Response               │                               │
       │<───────────────────────────────│                               │
```

## Configuration

### MCP Server Configuration

Add the following to your `config.yaml`:

```yaml
auth:
  # Enable/disable authentication
  enabled: true
  
  # Require authentication for all endpoints (false = optional auth)
  require_auth: false
  
  # OAuth2 Configuration
  oauth2:
    # Frappe OAuth2 endpoints
    token_info_url: "http://localhost:8000/api/method/frappe.integrations.oauth2.openid.userinfo"
    issuer_url: "http://localhost:8000"
    
    # Trusted backend clients (can provide user context via headers)
    trusted_clients:
      - "frappe-mcp-backend"
    
    # Validate tokens with remote OAuth2 provider
    validate_remote: true
    
    # HTTP client timeout for token validation
    timeout: "30s"
  
  # Token cache configuration
  token_cache:
    ttl: "5m"              # How long to cache validated tokens
    cleanup_interval: "10m" # How often to clean up expired tokens
```

### Environment Variables

You can also configure authentication using environment variables:

```bash
# Enable authentication
AUTH_ENABLED=true
AUTH_REQUIRE_AUTH=false

# OAuth2 configuration
OAUTH_TOKEN_INFO_URL=http://localhost:8000/api/method/frappe.integrations.oauth2.openid.userinfo
OAUTH_ISSUER_URL=http://localhost:8000
OAUTH_TIMEOUT=30s

# Cache configuration
CACHE_TTL=5m
CACHE_CLEANUP_INTERVAL=10m
```

## Setup Guide

### Step 1: Register OAuth2 Client in Frappe

1. Navigate to your Frappe instance: `/app/oauth-client`
2. Create a new OAuth Client:
   - **App Name**: MCP Backend Integration
   - **Client ID**: `frappe-mcp-backend` (or auto-generated)
   - **Scopes**: `openid`, `profile`, `email`, `all`
   - **Grant Type**: `Client Credentials`
3. Save and note the **Client Secret**

### Step 2: Configure MCP Server

Update your `config.yaml` with the OAuth2 configuration (see above).

### Step 3: Configure Frappe Backend Integration

If you're building a Frappe app to integrate with the MCP server:

```python
import frappe
import requests
from datetime import datetime, timedelta

# Token cache (use Redis in production)
_token_cache = {}

def get_access_token():
    """Get OAuth2 access token using client credentials grant"""
    settings = frappe.get_single("MCP Server Settings")
    
    # Check cache
    cache_key = "mcp_access_token"
    if cache_key in _token_cache:
        token_data = _token_cache[cache_key]
        if datetime.now() < token_data["expires_at"]:
            return token_data["access_token"]
    
    # Get new token
    token_url = f"{settings.frappe_base_url}/api/method/frappe.integrations.oauth2.get_token"
    
    response = requests.post(
        token_url,
        data={
            "grant_type": "client_credentials",
            "client_id": settings.oauth_client_id,
            "client_secret": settings.get_password("oauth_client_secret"),
        },
        headers={"Content-Type": "application/x-www-form-urlencoded"}
    )
    
    token_data = response.json()
    
    # Cache token
    _token_cache[cache_key] = {
        "access_token": token_data["access_token"],
        "expires_at": datetime.now() + timedelta(seconds=token_data.get("expires_in", 3600))
    }
    
    return token_data["access_token"]

@frappe.whitelist()
def query_mcp(message):
    """Call MCP server with OAuth2 authentication"""
    user = frappe.session.user
    user_email = frappe.db.get_value("User", user, "email")
    full_name = frappe.db.get_value("User", user, "full_name")
    
    settings = frappe.get_single("MCP Server Settings")
    access_token = get_access_token()
    
    headers = {
        "Content-Type": "application/json",
        "Authorization": f"Bearer {access_token}",
        # User context (trusted because token is from known backend client)
        "X-MCP-User-ID": user,
        "X-MCP-User-Email": user_email,
        "X-MCP-User-Name": full_name,
    }
    
    response = requests.post(
        f"{settings.mcp_server_url}/api/v1/chat",
        json={"message": message},
        headers=headers,
        timeout=30
    )
    
    return response.json()
```

## Usage

### Making Authenticated Requests

#### From Frappe Backend (Trusted Client)

```bash
# 1. Get OAuth2 token
TOKEN=$(curl -X POST http://localhost:8000/api/method/frappe.integrations.oauth2.get_token \
  -d "grant_type=client_credentials" \
  -d "client_id=frappe-mcp-backend" \
  -d "client_secret=YOUR_CLIENT_SECRET" \
  | jq -r '.access_token')

# 2. Call MCP with token + user context
curl -X POST http://localhost:8080/api/v1/chat \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-MCP-User-ID: user@example.com" \
  -H "X-MCP-User-Email: user@example.com" \
  -H "X-MCP-User-Name: John Doe" \
  -H "Content-Type: application/json" \
  -d '{"message": "Show me all projects"}'
```

#### From External Client

```bash
# Get token via Authorization Code flow (user login)
# Then use the token:

curl -X POST http://localhost:8080/api/v1/chat \
  -H "Authorization: Bearer $USER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"message": "Show me all projects"}'
```

### Accessing User Context in Tools

Within your MCP tools, you can access the authenticated user:

```go
import (
    "frappe-mcp-server/internal/auth"
)

func (r *ToolRegistry) MyTool(ctx context.Context, req mcp.ToolRequest) (*mcp.ToolResponse, error) {
    // Get authenticated user from context
    user, found := auth.GetUserFromContext(ctx)
    if found {
        // User is authenticated
        log.Printf("Request from user: %s (%s)", user.FullName, user.Email)
        
        // Access user properties
        userID := user.ID
        roles := user.Roles
        clientID := user.ClientID
        
        // Use user context for authorization, logging, etc.
    } else {
        // No authenticated user (optional auth mode)
        log.Printf("Anonymous request")
    }
    
    // ... rest of tool implementation
}
```

## Security Best Practices

### Production Deployment

1. **Always use HTTPS** in production
2. **Use strong client secrets** (32+ characters)
3. **Enable required auth** (`require_auth: true`)
4. **Validate tokens remotely** (`validate_remote: true`)
5. **Use secure token storage** (e.g., environment variables, secrets manager)
6. **Implement rate limiting** for token validation
7. **Monitor authentication failures**
8. **Rotate client secrets** periodically

### Token Security

- Tokens are cached in memory (5 minutes default)
- Cache is cleared on server restart
- Invalid tokens are rejected immediately
- Token validation timeout: 30 seconds default

## Backward Compatibility

The authentication system is **backward compatible**:

- Set `enabled: false` to disable authentication entirely
- Set `require_auth: false` for optional authentication
- Existing clients without tokens will still work in optional mode
- Gradually migrate by enabling optional auth first, then required auth

## Troubleshooting

### Common Issues

#### 1. "Unauthorized" errors

**Problem**: Requests are rejected with 401 Unauthorized

**Solutions**:
- Check that `AUTH_ENABLED=true` in your config
- Verify the Bearer token is valid and not expired
- Ensure the token validation URL is correct
- Check network connectivity to Frappe OAuth2 server

#### 2. Token validation timeout

**Problem**: Requests are slow or timeout during token validation

**Solutions**:
- Increase `timeout` in OAuth2 config (default: 30s)
- Check network latency to Frappe server
- Ensure Frappe OAuth2 endpoint is responsive
- Consider increasing cache TTL to reduce validation calls

#### 3. User context not available

**Problem**: `GetUserFromContext` returns nil even with valid token

**Solutions**:
- Verify the client is in `trusted_clients` list
- Check that `X-MCP-User-*` headers are being sent
- Ensure token belongs to a trusted client
- Verify middleware is properly configured

#### 4. Cache not working

**Problem**: Every request validates token remotely

**Solutions**:
- Check `token_cache.ttl` is set (default: 5m)
- Verify tokens are identical across requests
- Ensure cache cleanup interval is reasonable
- Check for memory constraints

### Debug Mode

Enable debug logging to troubleshoot authentication issues:

```yaml
logging:
  level: "debug"  # Enable detailed auth logs
```

## Testing

### Unit Tests

Run authentication tests:

```bash
go test ./internal/auth/... -v
```

### Integration Testing

Test with a mock OAuth2 server:

```bash
# Start mock OAuth2 server
go run test/mock_oauth_server.go

# Test authentication flow
curl -X POST http://localhost:8080/api/v1/chat \
  -H "Authorization: Bearer test-token" \
  -d '{"message": "test"}'
```

### Development Mode

For local development, you can skip remote validation:

```yaml
auth:
  enabled: true
  require_auth: false
  oauth2:
    validate_remote: false  # Skip remote validation
```

## API Reference

### User Context

The `User` type provides authenticated user information:

```go
type User struct {
    ID       string                 // User ID (email or username)
    Email    string                 // User email
    FullName string                 // Full name
    Roles    []string               // User roles
    ClientID string                 // OAuth2 client that issued token
    Metadata map[string]interface{} // Additional metadata
}
```

### Context Functions

```go
// Add user to context
ctx := auth.WithUser(ctx, user)

// Get user from context
user := auth.UserFromContext(ctx)

// Get user with boolean check
user, found := auth.GetUserFromContext(ctx)
```

## Future Enhancements

Planned improvements for future releases:

- **Redis-based caching** for distributed deployments
- **Rate limiting** with configurable thresholds
- **Token refresh** support
- **JWKS validation** for JWT tokens
- **RBAC integration** with Frappe permissions
- **Audit logging** for authentication events
- **Metrics** for token validation performance

## Related Documentation

- [OAuth2 RFC 6749](https://tools.ietf.org/html/rfc6749)
- [Frappe OAuth2 Documentation](https://frappeframework.com/docs/user/en/guides/integration/oauth)
- [OpenID Connect](https://openid.net/connect/)






