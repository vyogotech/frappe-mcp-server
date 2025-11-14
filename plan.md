# Production Authentication with Standard OAuth2 Only

## Architecture: Standard OAuth2 Throughout

**No custom authentication schemes. Only standard OAuth2.**

### OAuth2 Flows Used

1. **Client Credentials Grant** (Frappe backend → MCP)

      - Frappe backend is registered OAuth2 client
      - Gets access token with client_id + client_secret
      - Sends token + user context headers to MCP
      - MCP validates token, trusts user context from known backend client

2. **Authorization Code Grant** (External clients → MCP)

      - VS Code, mobile apps, etc.
      - User authorizes via Frappe login
      - Client gets access token
      - User context extracted from token claims

3. **JWT Validation** (Optional, for pre-issued tokens)

      - Validate JWT with JWKS
      - User context from JWT claims

## Core Dependencies (MVP)

```bash
go get github.com/shaj13/go-guardian/v2       # Basic auth framework
go get golang.org/x/oauth2                     # OAuth2 client
go get github.com/patrickmn/go-cache           # Simple in-memory caching
```

**Optional (can be added later):**
```bash
go get github.com/coreos/go-oidc/v3/oidc      # Advanced OIDC features
go get github.com/golang-jwt/jwt/v5            # JWT parsing (if needed)
```

## Phase 1: MCP Server OAuth2 Authentication

### 1.1 Auth Package Structure

```
internal/auth/
├── auth.go           # Main authenticator using go-guardian
├── middleware.go     # HTTP auth middleware
├── context.go        # User context management
├── cache.go          # Token caching
└── strategies/
    ├── oauth2.go     # OAuth2 token validation
    └── jwt.go        # JWT validation (optional)
```

### 1.2 OAuth2 Strategy Implementation (MVP)

**File: `internal/auth/strategies/oauth2.go`**

```go
type OAuth2Strategy struct {
    tokenInfoURL   string
    trustedClients map[string]bool  // client_ids that can provide user context
    cache          *sync.Map        // Simple in-memory cache
    httpClient     *http.Client
}

func NewOAuth2Strategy(tokenInfoURL string, trustedClients []string, timeout time.Duration) *OAuth2Strategy {
    trustedMap := make(map[string]bool)
    for _, client := range trustedClients {
        trustedMap[client] = true
    }

    return &OAuth2Strategy{
        tokenInfoURL:   tokenInfoURL,
        trustedClients: trustedMap,
        cache:          &sync.Map{},
        httpClient: &http.Client{
            Timeout: timeout,
        },
    }
}

func (s *OAuth2Strategy) Authenticate(ctx context.Context, r *http.Request) (auth.Info, error) {
    // Extract Bearer token
    token := extractBearerToken(r)
    if token == "" {
        return nil, errors.New("missing or invalid Bearer token")
    }

    // Check cache first
    if cached, ok := s.cache.Load(token); ok {
        if userInfo, ok := cached.(UserInfo); ok {
            // Simple TTL check - remove if expired
            if time.Now().After(userInfo.ExpiresAt) {
                s.cache.Delete(token)
            } else {
                return userInfo.User, nil
            }
        }
    }

    // Validate token with OAuth2 provider (Frappe)
    userInfo, clientID, err := s.validateToken(ctx, token)
    if err != nil {
        return nil, err
    }

    // If from trusted backend client, check for user context headers
    if s.trustedClients[clientID] {
        if userID := r.Header.Get("X-MCP-User-ID"); userID != "" {
            userInfo = s.extractUserFromHeaders(r)
        }
    }

    // Cache with simple TTL (5 minutes default)
    s.cache.Store(token, UserInfo{
        User:      userInfo,
        ExpiresAt: time.Now().Add(5 * time.Minute),
    })

    return userInfo, nil
}

type UserInfo struct {
    User      auth.Info
    ExpiresAt time.Time
}

func extractBearerToken(r *http.Request) string {
    auth := r.Header.Get("Authorization")
    if strings.HasPrefix(auth, "Bearer ") {
        return strings.TrimPrefix(auth, "Bearer ")
    }
    return ""
}

func (s *OAuth2Strategy) validateToken(ctx context.Context, token string) (auth.Info, string, error) {
    req, err := http.NewRequestWithContext(ctx, "GET", s.tokenInfoURL, nil)
    if err != nil {
        return nil, "", err
    }

    req.Header.Set("Authorization", "Bearer "+token)

    resp, err := s.httpClient.Do(req)
    if err != nil {
        return nil, "", err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, "", errors.New("invalid token")
    }

    var tokenInfo struct {
        Sub      string `json:"sub"`
        Email    string `json:"email"`
        Name     string `json:"name"`
        ClientID string `json:"client_id"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&tokenInfo); err != nil {
        return nil, "", err
    }

    user := &types.User{
        ID:       tokenInfo.Sub,
        Email:    tokenInfo.Email,
        FullName: tokenInfo.Name,
        ClientID: tokenInfo.ClientID,
    }

    return user, tokenInfo.ClientID, nil
}

func (s *OAuth2Strategy) extractUserFromHeaders(r *http.Request) auth.Info {
    return &types.User{
        ID:       r.Header.Get("X-MCP-User-ID"),
        Email:    r.Header.Get("X-MCP-User-Email"),
        FullName: r.Header.Get("X-MCP-User-Name"),
    }
}
```

### 1.3 Configuration (MVP)

**File: `internal/config/config.go`**

```go
type AuthConfig struct {
    Enabled     bool
    RequireAuth bool  // false = optional auth (backward compat)
    OAuth2      OAuth2Config
    Cache       CacheConfig
}

type OAuth2Config struct {
    // Frappe OAuth endpoints
    TokenInfoURL string   // /api/method/frappe.integrations.oauth2.openid.userinfo
    IssuerURL    string   // Frappe base URL

    // Trusted backend clients (can provide user context headers)
    TrustedClients []string  // List of client_ids (e.g., "frappe-mcp-backend")

    // Token validation
    ValidateRemote bool  // true = call Frappe to validate, false = JWT only

    // Basic timeouts
    Timeout string `yaml:"timeout"` // "30s" - HTTP client timeout
}

type CacheConfig struct {
    TTL             string `yaml:"ttl"`              // "5m" - token cache duration
    CleanupInterval string `yaml:"cleanup_interval"` // "10m" - cleanup frequency
}
```

### 1.4 User Types

**File: `internal/types/types.go`**

```go
type User struct {
    ID          string
    Email       string
    FullName    string
    Roles       []string
    ClientID    string  // OAuth2 client that issued token
    Metadata    map[string]interface{}
}
```

### 1.5 Middleware Integration (MVP)

**File: `internal/server/server.go`**

```go
func (s *MCPServer) setupAuth() error {
    if !s.config.Auth.Enabled {
        return nil  // Auth disabled
    }

    // Parse timeout
    timeout, err := time.ParseDuration(s.config.Auth.OAuth2.Timeout)
    if err != nil {
        timeout = 30 * time.Second  // default
    }

    // Create OAuth2 strategy
    s.authStrategy = strategies.NewOAuth2Strategy(
        s.config.Auth.OAuth2.TokenInfoURL,
        s.config.Auth.OAuth2.TrustedClients,
        timeout,
    )

    return nil
}

func (s *MCPServer) authMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Skip auth if disabled
        if !s.config.Auth.Enabled {
            next.ServeHTTP(w, r)
            return
        }

        // Try to authenticate
        user, err := s.authStrategy.Authenticate(r.Context(), r)

        if !s.config.Auth.RequireAuth {
            // Optional auth - continue even if auth fails
            if user != nil {
                r = r.WithContext(auth.WithUser(r.Context(), user))
            }
            next.ServeHTTP(w, r)
            return
        }

        // Required auth - fail if no valid auth
        if err != nil {
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusUnauthorized)
            json.NewEncoder(w).Encode(map[string]string{
                "error": "Unauthorized",
                "message": "Valid authentication required",
            })
            return
        }

        // Add user to context
        r = r.WithContext(auth.WithUser(r.Context(), user))
        next.ServeHTTP(w, r)
    })
}

// Helper function to get user from context
func GetUserFromContext(ctx context.Context) (*types.User, bool) {
    user := auth.UserFromContext(ctx)
    if user == nil {
        return nil, false
    }
    if mcpUser, ok := user.(*types.User); ok {
        return mcpUser, true
    }
    return nil, false
}
```

## Phase 2: Frappe Custom App (OAuth2 Client)

### 2.1 App Structure

```
frappe_mcp_integration/
├── frappe_mcp_integration/
│   ├── __init__.py
│   ├── hooks.py
│   ├── api/
│   │   ├── __init__.py
│   │   └── ai_query.py
│   ├── mcp_integration/
│   │   └── doctype/
│   │       └── mcp_server_settings/
│   │           ├── mcp_server_settings.json
│   │           ├── mcp_server_settings.py
│   │           └── mcp_server_settings.js
│   ├── public/
│   │   └── js/
│   │       └── awesome_bar.js
│   └── config/
│       └── desktop.py
├── setup.py
└── requirements.txt
```

### 2.2 OAuth2 Client Implementation

**File: `api/ai_query.py`**

```python
import frappe
import requests
from datetime import datetime, timedelta
from frappe import _

# Token cache (in-memory for simplicity, use Redis in production)
_token_cache = {}

def get_access_token():
    """
    Get OAuth2 access token using client credentials grant
    Standard OAuth2 - no custom auth
    """
    settings = frappe.get_single("MCP Server Settings")
    
    # Check cache
    cache_key = "mcp_access_token"
    if cache_key in _token_cache:
        token_data = _token_cache[cache_key]
        if datetime.now() < token_data["expires_at"]:
            return token_data["access_token"]
    
    # Get new token using client credentials grant
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
    
    if response.status_code != 200:
        frappe.throw(_("Failed to get access token from OAuth server"))
    
    token_data = response.json()
    
    # Cache token
    _token_cache[cache_key] = {
        "access_token": token_data["access_token"],
        "expires_at": datetime.now() + timedelta(seconds=token_data.get("expires_in", 3600))
    }
    
    return token_data["access_token"]

@frappe.whitelist()
def query(message):
    """
    AI query endpoint
    User already authenticated via Frappe session cookies
    Backend gets OAuth2 token and calls MCP server
    """
    # Get authenticated user from Frappe session
    user = frappe.session.user
    user_email = frappe.db.get_value("User", user, "email")
    full_name = frappe.db.get_value("User", user, "full_name")
    
    settings = frappe.get_single("MCP Server Settings")
    
    if not settings.enabled:
        frappe.throw(_("MCP integration is not enabled"))
    
    # Get OAuth2 access token (client credentials grant)
    access_token = get_access_token()
    
    # Call MCP server with standard OAuth2 Bearer token
    headers = {
        "Content-Type": "application/json",
        "Authorization": f"Bearer {access_token}",
        # User context (trusted because token is from known backend client)
        "X-MCP-User-ID": user,
        "X-MCP-User-Email": user_email,
        "X-MCP-User-Name": full_name,
    }
    
    payload = {"message": message}
    
    try:
        response = requests.post(
            f"{settings.mcp_server_url}/api/v1/chat",
            json=payload,
            headers=headers,
            timeout=settings.timeout or 30
        )
        response.raise_for_status()
        return response.json()
    except Exception as e:
        frappe.log_error(f"MCP query failed: {str(e)}")
        frappe.throw(_("Failed to query AI assistant"))
```

### 2.3 MCP Server Settings DocType

**File: `mcp_server_settings.json`**

```json
{
  "doctype": "DocType",
  "name": "MCP Server Settings",
  "issingle": 1,
  "fields": [
    {
      "fieldname": "enabled",
      "fieldtype": "Check",
      "label": "Enabled",
      "default": 1
    },
    {
      "fieldname": "mcp_server_url",
      "fieldtype": "Data",
      "label": "MCP Server URL",
      "reqd": 1,
      "description": "e.g., http://localhost:8080"
    },
    {
      "fieldname": "section_oauth",
      "fieldtype": "Section Break",
      "label": "OAuth2 Configuration"
    },
    {
      "fieldname": "frappe_base_url",
      "fieldtype": "Data",
      "label": "Frappe Base URL",
      "reqd": 1,
      "description": "OAuth2 server URL (this Frappe instance)"
    },
    {
      "fieldname": "oauth_client_id",
      "fieldtype": "Data",
      "label": "OAuth Client ID",
      "reqd": 1,
      "description": "Registered OAuth2 client ID"
    },
    {
      "fieldname": "oauth_client_secret",
      "fieldtype": "Password",
      "label": "OAuth Client Secret",
      "reqd": 1
    },
    {
      "fieldname": "section_settings",
      "fieldtype": "Section Break",
      "label": "Settings"
    },
    {
      "fieldname": "timeout",
      "fieldtype": "Int",
      "label": "Timeout (seconds)",
      "default": 30
    }
  ]
}
```

### 2.4 Awesome Bar Integration

**File: `public/js/awesome_bar.js`**

```javascript
frappe.provide('frappe.search');

frappe.search.utils.make_function('Ask AI', function(query) {
    if (!query) return [];
    
    return [{
        value: `Ask AI: ${query}`,
        description: 'Query AI assistant about ERPNext data',
        action: () => {
            let d = new frappe.ui.Dialog({
                title: 'AI Assistant',
                fields: [{
                    fieldname: 'result',
                    fieldtype: 'HTML',
                    options: '<div class="text-center"><i class="fa fa-spinner fa-spin"></i> Processing...</div>'
                }]
            });
            d.show();
            
            frappe.call({
                method: 'frappe_mcp_integration.api.ai_query.query',
                args: { message: query },
                callback: (r) => {
                    if (r.message) {
                        d.fields_dict.result.$wrapper.html(
                            `<div class="markdown-content">${frappe.markdown(r.message.response)}</div>`
                        );
                    }
                },
                error: () => {
                    d.hide();
                    frappe.msgprint('Failed to query AI assistant');
                }
            });
        }
    }];
}, 10);
```

## Phase 3: OAuth2 Setup in Frappe

### 3.1 Register OAuth2 Client

**Manual Steps (or via script):**

1. Navigate to: `/app/oauth-client`
2. Create new OAuth Client:
   ```
   App Name: MCP Backend Integration
   Client ID: frappe-mcp-backend (auto-generated or custom)
   Scopes: openid, profile, email, all
   Grant Type: Client Credentials
   ```

3. Save and note the client secret

### 3.2 Configure MCP Server Settings

1. Navigate to: `/app/mcp-server-settings`
2. Fill in:
   ```
   Enabled: ✓
   MCP Server URL: http://localhost:8080
   Frappe Base URL: http://localhost:8000
   OAuth Client ID: frappe-mcp-backend
   OAuth Client Secret: [paste secret]
   ```


### 3.3 Configure MCP Server (MVP)

**config.yaml:**

```yaml
auth:
  enabled: true
  require_auth: false  # Optional for backward compat

  oauth2:
    # Frappe OAuth endpoints
    token_info_url: "http://localhost:8000/api/method/frappe.integrations.oauth2.openid.userinfo"
    issuer_url: "http://localhost:8000"

    # Trusted clients that can provide user context headers
    trusted_clients:
      - "frappe-mcp-backend"

    validate_remote: true  # Call Frappe to validate tokens
    timeout: "30s"         # HTTP client timeout

  cache:
    ttl: "5m"              # Token cache duration
    cleanup_interval: "10m" # Cache cleanup frequency
```

**Environment variables:**

```bash
# Authentication
AUTH_ENABLED=true
AUTH_REQUIRE_AUTH=false

# OAuth2 Configuration
FRAPPE_BASE_URL=http://localhost:8000
OAUTH_TOKEN_INFO_URL=http://localhost:8000/api/method/frappe.integrations.oauth2.openid.userinfo
OAUTH_TIMEOUT=30s

# Cache settings (optional)
CACHE_TTL=5m
CACHE_CLEANUP_INTERVAL=10m
```

## Phase 4: Deployment

### 4.1 Docker Compose

```yaml
services:
  frappe-mcp-server:
    environment:
      AUTH_ENABLED: "true"
      FRAPPE_BASE_URL: "${FRAPPE_BASE_URL}"
      OAUTH_TOKEN_INFO_URL: "${FRAPPE_BASE_URL}/api/method/frappe.integrations.oauth2.openid.userinfo"
```

### 4.2 MVP Checklist

- [ ] HTTPS enabled on both Frappe and MCP Server
- [ ] Strong client secret (32+ characters)
- [ ] Token caching enabled (in-memory for MVP)
- [ ] Basic logging configured
- [ ] OAuth client credentials stored securely
- [ ] Error handling for token validation failures
- [ ] Health check endpoint available

## Phase 5: Testing

### 5.1 Test OAuth2 Flow

```bash
# 1. Get token as Frappe backend
curl -X POST http://localhost:8000/api/method/frappe.integrations.oauth2.get_token \
  -d "grant_type=client_credentials" \
  -d "client_id=frappe-mcp-backend" \
  -d "client_secret=SECRET"

# 2. Use token to call MCP
curl -X POST http://localhost:8080/api/v1/chat \
  -H "Authorization: Bearer TOKEN" \
  -H "X-MCP-User-ID: user@example.com" \
  -H "X-MCP-User-Email: user@example.com" \
  -d '{"message": "Show projects"}'
```

### 5.2 MVP Test Cases

- ✅ Token validation success
- ✅ Invalid token rejection
- ✅ Missing token handling
- ✅ Token caching functionality
- ✅ User context extraction from trusted client
- ✅ Optional vs required auth modes
- ✅ Basic error responses

## MVP Success Criteria

1. ✅ Only standard OAuth2 (no custom auth)
2. ✅ Frappe backend uses client credentials grant
3. ✅ MCP validates tokens with Frappe OAuth
4. ✅ User context from trusted backend client
5. ✅ Simple in-memory token caching
6. ✅ Backward compatible (optional auth)
7. ✅ Basic error handling and responses
8. ✅ Clear documentation

## MVP Benefits

- **100% Standard OAuth2** - No custom authentication
- **Simple & Fast**: Minimal dependencies, in-memory caching
- **Secure**: Frappe OAuth server controls all access
- **Backward Compatible**: Works with existing installations
- **Quick to Deploy**: Simple configuration, no external dependencies
- **Easy to Extend**: Can add Redis, monitoring later when needed

## Future Enhancements (Post-MVP)

### Phase 2 - Performance & Scale
- Redis-based token caching
- Rate limiting with Redis backend
- Connection pooling optimization
- Health check endpoints

### Phase 3 - Production Features
- Prometheus metrics
- Structured logging
- Circuit breakers
- Security headers middleware

### Phase 4 - Advanced Features
- Distributed tracing
- Load testing framework
- Automated security scanning
- Token rotation strategies

## MVP Implementation Order

### Step 1: Core Dependencies (5 min)
```bash
go get github.com/shaj13/go-guardian/v2       # Basic auth framework
go get golang.org/x/oauth2                     # OAuth2 client
go get github.com/patrickmn/go-cache           # Simple cache
```

### Step 2: Basic Auth Structure (30 min)
1. Create `internal/types/user.go` - User struct
2. Create `internal/config/config.go` - Config structs
3. Create `internal/auth/context.go` - User context helpers

### Step 3: OAuth2 Strategy (45 min)
1. Create `internal/auth/strategies/oauth2.go` - Core OAuth2 logic
2. Implement token validation with Frappe
3. Add simple in-memory caching

### Step 4: Middleware Integration (30 min)
1. Update `internal/server/server.go` - Auth middleware
2. Add auth setup in server initialization
3. Add user context helpers

### Step 5: Testing & Validation (45 min)
1. Test basic OAuth2 flow
2. Test optional vs required auth modes
3. Test error handling

**Total MVP Time: ~2.5 hours**

This MVP provides a solid foundation that can be enhanced incrementally without breaking changes.