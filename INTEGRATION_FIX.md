# MCP Integration Fix Guide

## What Was Fixed

### 1. Network Configuration ✅
- **config.yaml**: Updated OAuth URLs from `localhost:8000` → `erpnext:8000`
- **Trusted client**: Added your OAuth client ID `g79ghfpol3`

### 2. What You Need to Fix in ERPNext UI

#### Step 1: Update MCP Server Settings
Go to: **MCP Server Settings** DocType in ERPNext

Change:
- **Frappe Base URL**: `http://localhost:8000` → `http://erpnext:8000`
- **MCP Server URL**: Should already be `http://frappe-mcp-server:8080` ✓

**IMPORTANT**: After changing, click **Save**.

#### Step 2: Verify OAuth Client Configuration
Go to: **OAuth Client** list → Open client ID `g79ghfpol3`

Verify these settings:
```
App Name: MCP Integration
Client ID: g79ghfpol3
Grant Type: Client Credentials
Skip Authorization: ✓ (checked)
Scopes:
  - openid
  - all
```

**If missing**, add the scopes:
1. In "Scopes" table, click "Add Row"
2. Add scope: `openid`
3. Add another row with scope: `all`
4. Click **Save**

#### Step 3: Test the Integration

##### From ERPNext (Awesome Bar):
1. Press `/` or click Awesome Bar
2. Type: `@ai show me top 5 customers`
3. Press Enter

##### From Open WebUI:
1. Open: http://localhost:9080
2. In chat, type: `Show me top 10 sales orders`

## Troubleshooting

### If you get "unauthorized_client" error:
```bash
# Check OAuth client exists and has correct grant type
docker exec 281a931ec5d5 bench --site dev.localhost mariadb <<SQL
SELECT name, app_name, grant_type, skip_authorization, client_id 
FROM \`tabOAuth Client\` 
WHERE client_id = 'g79ghfpol3';
SQL
```

### To view MCP server logs:
```bash
docker logs -f frappe-mcp-server-frappe-mcp-server-1
```

### To test OAuth token directly:
```bash
# Replace YOUR_SECRET with actual client secret
docker exec 329a478fb26f curl -X POST http://erpnext:8000/api/method/frappe.integrations.oauth2.get_token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=client_credentials&client_id=g79ghfpol3&client_secret=YOUR_SECRET"
```

Expected response:
```json
{
  "access_token": "...",
  "token_type": "Bearer",
  "expires_in": 3600
}
```

## Network Architecture

```
┌─────────────────┐
│   Open WebUI    │ :9080
│  (Frontend UI)  │
└────────┬────────┘
         │
         ▼
┌─────────────────┐         ┌─────────────────┐
│  MCP Server     │◄────────┤    Ollama       │ :11434
│  :8080          │         │  (LLM Provider) │
└────────┬────────┘         └─────────────────┘
         │
         │ OAuth2 + API calls
         ▼
┌─────────────────┐
│    ERPNext      │ :8000
│  (Data Source)  │
└─────────────────┘

Internal Docker Network:
- erpnext:8000
- frappe-mcp-server:8080  
- ollama:11434
- open-webui:8080 (mapped to host 9080)
```

## Next Steps

After fixing the above:
1. Restart the stack: `docker compose restart`
2. Wait for all containers to be healthy: `docker compose ps`
3. Test from Awesome Bar or Open WebUI
4. Check MCP server logs for any errors

## Configuration Summary

**config.yaml** (MCP Server):
```yaml
auth:
  oauth2:
    token_info_url: "http://erpnext:8000/api/method/frappe.integrations.oauth2.openid.userinfo"
    issuer_url: "http://erpnext:8000"
    trusted_clients:
      - "g79ghfpol3"
```

**MCP Server Settings** (ERPNext):
```json
{
  "mcp_server_url": "http://frappe-mcp-server:8080",
  "frappe_base_url": "http://erpnext:8000",
  "oauth_client_id": "g79ghfpol3"
}
```

