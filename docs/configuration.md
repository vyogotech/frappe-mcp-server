# Configuration Guide

Complete reference for configuring ERPNext MCP Server.

## Configuration File

The server reads from `config.yaml` by default. You can specify a different path using the `CONFIG_FILE` environment variable.

```yaml
server:
  host: "0.0.0.0"          # Server bind address
  port: 8080               # HTTP server port
  timeout: "30s"           # Read and write timeout for one request
  tls:                     # Native TLS / HTTPS (optional)
    enabled: true          # Enable HTTPS listener
    cert_file: "/path/to/cert.pem" # Path to certificate PEM
    key_file: "/path/to/key.pem"   # Path to private key PEM
    auto_generate: true    # Use ephemeral self-signed cert if cert/key omitted

logging:
  level: "info"            # debug, info, warn, error

erpnext:
  base_url: "http://localhost:8000"  # ERPNext instance URL
  api_key: "your_api_key"            # ERPNext API key
  api_secret: "your_api_secret"      # ERPNext API secret
  timeout: "30s"                     # Request timeout
  
  # Retry configuration for failed requests
  retry:
    max_attempts: 5                  # Maximum retry attempts
    initial_delay: "500ms"           # Initial delay between retries
    max_delay: "5s"                  # Maximum delay between retries
  
  # Rate limiting
  rate_limit:
    requests_per_second: 10          # Max requests per second
    burst: 20                        # Burst capacity
```

## Environment Variables

Configuration can be overridden using environment variables:

| Variable | Description | Example |
| ---------- | ------------- | --------- |
| `CONFIG_FILE` | Path to config file | `/etc/erpnext-mcp/config.yaml` |
| `FRAPPE_BASE_URL` | Frappe URL | `https://erp.company.com` |
| `FRAPPE_API_KEY` | API key | `abc123...` |
| `FRAPPE_API_SECRET` | API secret | `xyz789...` |
| `FRAPPE_SITE_ROUTES_FILE` | Site-to-bench routes file for multi-tenant deployments | `/etc/frappe/site_routes.yaml` |
| `FRAPPE_SITE_URL_TEMPLATE` | Bench URL pattern with a `{site}` placeholder | `http://{site}.bench.local:8000` |
| `SERVER_HOST` | Bind address | `0.0.0.0` |
| `SERVER_PORT` | HTTP port | `8080` |
| `SERVER_TLS_ENABLED` | Enable native HTTPS/TLS listener (`TLS_ENABLED` fallback) | `true` |
| `SERVER_TLS_CERT_FILE` | Path to TLS certificate PEM (`TLS_CERT_FILE` fallback) | `/etc/tls/cert.pem` |
| `SERVER_TLS_KEY_FILE` | Path to TLS private key PEM (`TLS_KEY_FILE` fallback) | `/etc/tls/key.pem` |
| `SERVER_TLS_AUTO_GENERATE` | Generate ephemeral self-signed cert if cert/key omitted | `true` |
| `LOG_LEVEL` | Logging level | `debug` |
| `AUTH_ENABLED` | Turn authentication on or off | `true` |
| `AUTH_REQUIRE_AUTH` | Refuse an unauthenticated request | `true` |
| `OAUTH_TOKEN_INFO_URL` | Bearer token introspection endpoint | `https://erp.company.com/api/method/frappe.integrations.oauth2.openid_profile` |
| `OAUTH_TIMEOUT` | Timeout for token validation | `30s` |
| `CACHE_TTL` | How long a validated token is cached | `5m` |
| `TOOLS_KNOWLEDGE_BASE` | Offer `search_knowledge_base` | `true` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP/HTTP collector; tracing is off when unset | `http://otel-collector:4318` |

A numeric or boolean variable whose value cannot be parsed stops the server at startup and names the variable;
booleans take the spellings `strconv.ParseBool` accepts (`true`, `True`, `1`, `t`, `false`, `0`, ...).

Environment variables take precedence over config file values.

## Multi-tenant Routing

One server can front many Frappe sites, on one bench or on several. With nothing configured here it behaves as a single-site server against `erpnext.base_url`.

A request names its site in the `X-Frappe-Site-Name` header. The server validates the caller's session against that site, and sends the same site name on every Frappe call it makes for the request, so a bench hosting several sites answers for the right one. Sessions, CSRF tokens and rate limits are kept per site, and session ids are never held in clear.

Where a site's bench lives is read from the server's own configuration, never from the request:

```yaml
erpnext:
  base_url: "http://localhost:8000"                    # the default, and the only URL a single-site install needs
  site_routes_file: "/etc/frappe/site_routes.yaml"    # explicit site → bench map; re-read when the file changes
  site_url_template: "http://{site}.bench.local:8000"  # or a pattern, when bench addresses follow one
```

```yaml
# site_routes.yaml
routes:
  tenant-a.example.com: http://bench-01:8000
  tenant-b.example.com: http://bench-02:8000
```

The routes file wins over the template, and the template over `base_url`. When a routes file is configured, a request naming an unlisted site, or no site at all, is refused with `400` before anything reaches Frappe. Bearer tokens are introspected on the site's own bench whenever the introspection endpoint lives on `base_url`; a separate identity provider is used as configured for every site.

## ERPNext Setup

### 1. Create API Credentials

In ERPNext:

1. Go to **User** list
2. Select your user
3. Scroll to **API Access** section
4. Click **Generate Keys**
5. Save the **API Key** and **API Secret**

### 2. Set Permissions

Ensure the user has appropriate roles:

- **System Manager** (for full access)
- Or specific DocType permissions for limited access

### 3. Configure CORS (if needed)

If ERPNext is on a different domain, add CORS headers in `site_config.json`:

```json
{
  "allow_cors": "*",
  "cors_allowed_origins": ["http://localhost:8080"]
}
```

## Performance Tuning

### Retry Configuration

For unreliable networks or resource-constrained ERPNext:

```yaml
erpnext:
  retry:
    max_attempts: 10         # Increase retries
    initial_delay: "1s"      # Longer initial delay
    max_delay: "10s"         # Longer max delay
```

### Rate Limiting

Prevent overwhelming ERPNext:

```yaml
erpnext:
  rate_limit:
    requests_per_second: 5   # Reduce for slower instances
    burst: 10                # Lower burst capacity
```

### Timeouts

Adjust for slow networks or large data:

```yaml
erpnext:
  timeout: "60s"             # Longer timeout for large queries
```

## Security Best Practices

### 1. Protect API Credentials

```bash
# Set restrictive file permissions
chmod 600 config.yaml

# Or use environment variables
export FRAPPE_API_KEY="..."
export FRAPPE_API_SECRET="..."
```

### 2. Use HTTPS

For production, use HTTPS for ERPNext:

```yaml
erpnext:
  base_url: "https://erp.company.com"
```

### 3. Limit Bind Address

For local-only access:

```yaml
server:
  host: "127.0.0.1"  # Only accessible locally
```

## Multiple Environments

### Development

```yaml
# config.dev.yaml
logging:
  level: "debug"

erpnext:
  base_url: "http://localhost:8000"
```

### Production

```yaml
# config.prod.yaml
server:
  host: "127.0.0.1"

logging:
  level: "warn"

erpnext:
  base_url: "https://erp.company.com"
  timeout: "60s"
  retry:
    max_attempts: 3
```

Run with:

```bash
CONFIG_FILE=config.prod.yaml ./bin/frappe-mcp-server
```

## Logging

Set log level:

```yaml
logging:
  level: "debug"  # debug, info, warn, error
```

Or set `LOG_LEVEL` in the environment.

Logs are written to:

- **STDOUT** for HTTP server
- **STDERR** for STDIO server (to not interfere with MCP protocol)

## Verification

Test your configuration:

```bash
# HTTP server
./bin/frappe-mcp-server

# Should see:
# INFO Starting ERPNext MCP Server on 0.0.0.0:8080
# INFO Connected to ERPNext at http://localhost:8000
```

Next: [Analytics & Reporting](analytics-features.md)
