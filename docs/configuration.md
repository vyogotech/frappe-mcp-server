# Configuration Guide

Complete reference for configuring ERPNext MCP Server.

## Configuration File

The server reads from `config.yaml` by default. You can specify a different path using the `CONFIG_FILE` environment variable.

```yaml
server:
  host: "0.0.0.0"          # Server bind address
  port: 8080               # HTTP server port
  log_level: "info"        # Logging level: debug, info, warn, error

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

ollama:
  url: "http://localhost:11434"      # Ollama API endpoint
  model: "llama3.2:1b"               # Model to use for AI features
  timeout: "60s"                     # LLM request timeout
```

## Environment Variables

Configuration can be overridden using environment variables:

| Variable | Description | Example |
|----------|-------------|---------|
| `CONFIG_FILE` | Path to config file | `/etc/erpnext-mcp/config.yaml` |
| `FRAPPE_BASE_URL` | Frappe URL | `https://erp.company.com` |
| `FRAPPE_API_KEY` | API key | `abc123...` |
| `FRAPPE_API_SECRET` | API secret | `xyz789...` |
| `OLLAMA_URL` | Ollama endpoint | `http://localhost:11434` |
| `OLLAMA_MODEL` | Ollama model name | `llama3.2:1b` |
| `SERVER_PORT` | HTTP port | `8080` |

Environment variables take precedence over config file values.

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

## Ollama Setup

### Installation

```bash
# macOS/Linux
curl https://ollama.ai/install.sh | sh

# Windows - download from https://ollama.ai
```

### Pull Models

```bash
# Recommended: Fast and efficient (1.2B parameters)
ollama pull llama3.2:1b

# Alternative: More capable (8B parameters, slower)
ollama pull llama3.1

# List downloaded models
ollama list
```

### Model Selection

| Model | Size | Speed | Accuracy | Use Case |
|-------|------|-------|----------|----------|
| `llama3.2:1b` | 1.3GB | ⚡ Fast | Good | Quick queries, entity extraction |
| `llama3.1` | 4.9GB | 🐢 Slower | Better | Complex analysis, detailed queries |

Configure in `config.yaml`:

```yaml
ollama:
  model: "llama3.2:1b"  # or "llama3.1"
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

ollama:
  timeout: "120s"            # Longer for complex AI processing
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

### 4. Network Isolation

Run Ollama on the same machine to avoid sending queries over network.

## Multiple Environments

### Development

```yaml
# config.dev.yaml
server:
  log_level: "debug"
  
erpnext:
  base_url: "http://localhost:8000"
```

### Production

```yaml
# config.prod.yaml
server:
  log_level: "warn"
  host: "127.0.0.1"

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
server:
  log_level: "debug"  # debug, info, warn, error
```

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
# INFO Ollama available at http://localhost:11434 with model llama3.2:1b
```

Next: [AI Features](ai-features.md)

