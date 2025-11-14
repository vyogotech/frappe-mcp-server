# Open WebUI with Frappe OAuth2 Login

## The Right Way: User Login with Redirect Flow ✅

You asked the right question! Users **SHOULD** be redirected to Frappe's login page. This is the proper OAuth2 Authorization Code flow for web applications.

## 🎯 How It Should Work

### The User Experience:

```
1. User visits Open WebUI
   http://localhost:3000

2. Clicks "Login with Frappe" button

3. Gets redirected to Frappe login page
   http://localhost:8000/login?oauth_redirect=...

4. User enters Frappe credentials
   Username: john@company.com
   Password: •••••••••

5. Frappe asks: "Allow Open WebUI to access your data?"
   [Allow] [Deny]

6. User clicks "Allow"

7. Gets redirected back to Open WebUI
   http://localhost:3000/callback?code=abc123...

8. Open WebUI exchanges code for token (behind the scenes)

9. User is now logged in to Open WebUI with their Frappe identity!
   ✅ Can access ERPNext data with THEIR permissions
   ✅ No API keys needed!
   ✅ Secure OAuth2 flow
```

## 🔧 Setup Steps

### Step 1: Create OAuth2 Client in Frappe (For Web Users)

1. **Go to**: http://localhost:8000/app/oauth-client

2. **Click "New"** and configure:

```
┌──────────────────────────────────────────────┐
│  OAuth Client Configuration                  │
├──────────────────────────────────────────────┤
│                                              │
│  App Name: Open WebUI User Login            │
│                                              │
│  Scopes:                                     │
│    ☑ openid                                 │
│    ☑ profile                                │
│    ☑ email                                  │
│    ☑ all                                    │
│                                              │
│  Grant Type:                                 │
│    ☑ Authorization Code  ← Yes!             │
│    ☐ Implicit                               │
│                                              │
│  Redirect URIs: (Important!)                 │
│    http://localhost:3000/oauth/callback     │
│    http://localhost:3000/api/auth/callback  │
│                                              │
│  Default Redirect URI:                       │
│    http://localhost:3000/oauth/callback     │
│                                              │
│  Skip Authorization: ☐  ← LEAVE UNCHECKED!  │
│                                              │
│  Users WILL see:                             │
│    1. Frappe login page                     │
│    2. "Allow Open WebUI?" confirmation      │
│    3. Redirect back to Open WebUI           │
│                                              │
└──────────────────────────────────────────────┘
```

3. **Save** and copy:
   - Client ID
   - Client Secret

### Step 2: Configure Open WebUI

Add to your `.env` file:

```bash
# Open WebUI OAuth2 Configuration
FRAPPE_OAUTH_CLIENT_ID=your-client-id-from-step-1
FRAPPE_OAUTH_CLIENT_SECRET=your-client-secret
FRAPPE_BASE_URL=http://localhost:8000
```

### Step 3: Update compose.yml

Uncomment the OAuth2 section:

```yaml
# compose.yml
services:
  open-webui:
    environment:
      # OAuth2 Configuration - UNCOMMENT THESE:
      OAUTH_CLIENT_ID: ${FRAPPE_OAUTH_CLIENT_ID}
      OAUTH_CLIENT_SECRET: ${FRAPPE_OAUTH_CLIENT_SECRET}
      OAUTH_PROVIDER_NAME: "Frappe"
      OPENID_PROVIDER_URL: ${FRAPPE_BASE_URL}/.well-known/openid-configuration
```

### Step 4: Restart Open WebUI

```bash
docker-compose restart open-webui
```

## 🎬 Testing the Flow

### Test 1: Login Flow

1. Open Open WebUI: http://localhost:3000
2. Look for "Login with Frappe" button (may need to configure)
3. Click it
4. Should redirect to: http://localhost:8000/login
5. Enter Frappe credentials
6. Approve the authorization
7. Get redirected back to Open WebUI
8. You're logged in!

### Test 2: Verify User Context

After logging in, test that queries use YOUR permissions:

```
Query: "Show my assigned tasks"
Result: Shows tasks assigned to YOU (not all tasks)

Query: "What projects am I working on?"
Result: Shows YOUR projects (based on your Frappe roles)
```

## 📋 Two Types of OAuth2 Clients

You should create **BOTH**:

### Client 1: For MCP Server (Backend Service)
```
Name: MCP Backend Integration
Grant Type: Authorization Code
Skip Authorization: ✅ CHECKED
Redirect URI: http://localhost

Purpose: MCP server authenticates with Frappe
Use case: Server-to-server communication
```

### Client 2: For Web Users (What you asked about!)
```
Name: Open WebUI User Login
Grant Type: Authorization Code
Skip Authorization: ☐ UNCHECKED  ← Key difference!
Redirect URI: http://localhost:3000/oauth/callback

Purpose: Users log in to Open WebUI with Frappe credentials
Use case: Web application user authentication
```

## 🔄 Flow Comparison

### Backend Service Flow (Skip Auth = ✅):
```
MCP Server → Get Token → Use Token
(No human interaction)
```

### Web User Flow (Skip Auth = ☐):
```
User → Click Login → Redirect to Frappe → 
Login Page → Enter Credentials → Approve → 
Redirect Back → Logged In!
(Full user interaction)
```

## ✅ Benefits of User Login Flow

1. **User-Level Permissions**
   - Each user sees only THEIR data
   - Respects Frappe role permissions
   - Audit trail shows actual user

2. **No API Keys Needed**
   - Users don't need API keys
   - Secure OAuth2 tokens
   - Tokens expire automatically

3. **Single Sign-On (SSO)**
   - One login for Frappe + Open WebUI
   - Consistent user experience
   - Centralized authentication

4. **Better Security**
   - No shared credentials
   - Token-based authentication
   - Can revoke access per user

## 🛠️ Implementation for Open WebUI

Check if Open WebUI supports OIDC/OAuth2:

```bash
# Check Open WebUI documentation
docker exec -it open-webui env | grep OAUTH
docker exec -it open-webui env | grep OPENID
```

If Open WebUI supports OAuth2/OIDC, you'll configure:

```yaml
environment:
  # OpenID Connect Configuration
  OPENID_PROVIDER_URL: "http://localhost:8000"
  OAUTH_CLIENT_ID: "your-web-user-client-id"
  OAUTH_CLIENT_SECRET: "your-secret"
  OAUTH_SCOPES: "openid profile email"
  OAUTH_REDIRECT_URI: "http://localhost:3000/oauth/callback"
```

## 🔍 Checking Frappe's OAuth2 Endpoints

Frappe should provide these endpoints:

```bash
# Authorization endpoint
http://localhost:8000/api/method/frappe.integrations.oauth2.authorize

# Token endpoint
http://localhost:8000/api/method/frappe.integrations.oauth2.get_token

# UserInfo endpoint
http://localhost:8000/api/method/frappe.integrations.oauth2.openid.userinfo

# OpenID Configuration (if supported)
http://localhost:8000/.well-known/openid-configuration
```

Test them:

```bash
curl http://localhost:8000/.well-known/openid-configuration
```

## 📚 Summary

**Your Question**: Why not redirect users to login page?

**Answer**: You're absolutely right! For web applications:

- ✅ **DO redirect to login** (Skip Authorization = ☐)
- ✅ Users see Frappe login page
- ✅ Users approve the app
- ✅ Get redirected back
- ✅ User-level permissions

**Two different setups**:
1. **Backend service** (MCP ↔ Frappe): Skip Authorization = ✅
2. **Web users** (Open WebUI users): Skip Authorization = ☐

You asked the perfect question! The redirect flow is the right way for user authentication! 🎯

---

**Next Steps**:
1. Create OAuth client with Skip Authorization = ☐
2. Configure Open WebUI with OAuth2 settings
3. Test the login redirect flow
4. Users log in with their Frappe credentials
5. Enjoy user-level permissions!
