# Quick Start Guide

Get Frappe MCP Server running in 5 minutes.

## Prerequisites

- **Frappe/ERPNext Instance** - Running and accessible with API credentials
- **Ollama** (optional, for AI features) - [Install](https://ollama.ai)
- **MCP Client** - Cursor IDE or Claude Desktop

## Step 1: Install

### Option A: Automated Install (Recommended)

```bash
# One-command installation
curl -fsSL https://raw.githubusercontent.com/varkrish/frappe-mcp-server/main/install.sh | bash
```

This will:
- Detect your platform (Linux/Mac/Windows)
- Download the latest release
- Install to `~/.local/bin/frappe-mcp-server-stdio`
- Create configuration directory

### Option B: Manual Download

1. Go to [Releases](https://github.com/varkrish/frappe-mcp-server/releases/latest)
2. Download the binary for your platform:
   - **Linux (Intel)**: `frappe-mcp-server-stdio-linux-amd64.tar.gz`
   - **Linux (ARM)**: `frappe-mcp-server-stdio-linux-arm64.tar.gz`
   - **macOS (Intel)**: `frappe-mcp-server-stdio-darwin-amd64.tar.gz`
   - **macOS (M1/M2)**: `frappe-mcp-server-stdio-darwin-arm64.tar.gz`
   - **Windows**: `frappe-mcp-server-stdio-windows-amd64.zip`
3. Extract and place in your PATH

### Option C: Build from Source

```bash
# Requires Go 1.24+
git clone https://github.com/varkrish/frappe-mcp-server
cd frappe-mcp-server

# Install dependencies
make deps

# Build the STDIO binary
make build-stdio
```

## Step 2: Configure

Create `config.yaml` with your ERPNext credentials:

```yaml
server:
  host: "0.0.0.0"
  port: 8080

erpnext:
  base_url: "http://localhost:8000"
  api_key: "your_api_key"
  api_secret: "your_api_secret"
  timeout: "30s"

ollama:
  url: "http://localhost:11434"
  model: "llama3.2:1b"
  timeout: "60s"
```

### Get ERPNext API Credentials

1. Log into ERPNext
2. Go to **User Settings** → **API Access**
3. Click **Generate Keys**
4. Copy API Key and API Secret

## Step 3: Run

### HTTP Server (for web integrations)

```bash
./bin/frappe-mcp-server
```

Server starts on `http://localhost:8080`

### STDIO Server (for Cursor/Claude Desktop)

Add to Cursor's MCP settings (`~/.cursor/mcp.json`):

```json
{
  "mcpServers": {
    "erpnext": {
      "command": "/absolute/path/to/bin/frappe-mcp-server-stdio",
      "args": ["--config", "/absolute/path/to/config.yaml"]
    }
  }
}
```

Restart Cursor and the MCP server will be available!

## Step 4: Test

### Test HTTP API

```bash
# Health check
curl http://localhost:8080/api/v1/health

# List available tools
curl http://localhost:8080/api/v1/tools

# Natural language query
curl -X POST http://localhost:8080/api/v1/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "List all projects"}'
```

### Test in Cursor

Open Cursor and type:
```
@erpnext List all ERPNext projects
```

## Try Analytics Queries 🆕

The server now supports powerful analytics and reporting:

### Aggregation Queries

```bash
# Top customers by revenue
curl -X POST http://localhost:8080/api/v1/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "show me top 5 customers by revenue in table format"}'

# Total sales by item
curl -X POST http://localhost:8080/api/v1/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "what are total sales by item this month?"}'

# Direct aggregation tool call
curl -X POST http://localhost:8080/api/v1/tool/aggregate_documents \
  -H "Content-Type: application/json" \
  -d '{
    "doctype": "Sales Invoice",
    "fields": ["customer", "SUM(grand_total) as revenue"],
    "group_by": "customer",
    "order_by": "revenue desc",
    "limit": 10
  }'
```

### Run Reports

```bash
# Execute ERPNext report via natural language
curl -X POST http://localhost:8080/api/v1/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "run Sales Analytics report"}'

# Direct report tool call
curl -X POST http://localhost:8080/api/v1/tool/run_report \
  -H "Content-Type: application/json" \
  -d '{
    "report_name": "Sales Analytics",
    "filters": {"company": "My Company"}
  }'
```

## Optional: Setup Ollama (AI Features)

```bash
# Install Ollama
curl https://ollama.ai/install.sh | sh

# Pull the model
ollama pull llama3.2:1b

# Verify
ollama list
```

Ollama enables natural language query understanding and intelligent entity extraction.

## Troubleshooting

### Connection Refused
- Ensure ERPNext is running and accessible
- Check `base_url` in `config.yaml`
- Verify API credentials

### Ollama Not Found
- AI features require Ollama running locally
- Install and start Ollama service
- Verify with `curl http://localhost:11434/api/tags`

### Cursor Not Detecting Server
- Use absolute paths in `mcp.json`
- Completely restart Cursor (Cmd+Q)
- Check Cursor's MCP logs

## Next Steps

- [Configuration Guide](configuration.md) - Detailed configuration options
- [AI Features](ai-features.md) - Learn about natural language queries
- [API Reference](api-reference.md) - Complete API documentation

