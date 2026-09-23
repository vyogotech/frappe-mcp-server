# Quick Start Guide

Get Frappe MCP Server running in 5 minutes.

## Prerequisites

- **Frappe/ERPNext Instance** - Running and accessible with API credentials
- **MCP Client** - Cursor IDE or Claude Desktop

## Step 1: Install

### Option A: Automated Install (Recommended)

```bash
# One-command installation
curl -fsSL https://raw.githubusercontent.com/vyogotech/frappe-mcp-server/main/install.sh | bash
```

This will:
- Detect your platform (Linux/Mac/Windows)
- Download the latest release
- Install to `~/.local/bin/frappe-mcp-server-stdio`
- Create configuration directory

### Option B: Manual Download

1. Go to [Releases](https://github.com/vyogotech/frappe-mcp-server/releases/latest)
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
git clone https://github.com/vyogotech/frappe-mcp-server
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

# Call a tool
curl -X POST http://localhost:8080/api/v1/tools/list_documents \
  -H "Content-Type: application/json" \
  -d '{"params": {"doctype": "Project"}}'
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
curl -X POST http://localhost:8080/api/v1/tools/aggregate_documents \
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
curl -X POST http://localhost:8080/api/v1/tools/run_report \
  -H "Content-Type: application/json" \
  -d '{
    "report_name": "Sales Analytics",
    "filters": {"company": "My Company"}
  }'
```

## Troubleshooting

### Connection Refused
- Ensure ERPNext is running and accessible
- Check `base_url` in `config.yaml`
- Verify API credentials

### Cursor Not Detecting Server
- Use absolute paths in `mcp.json`
- Completely restart Cursor (Cmd+Q)
- Check Cursor's MCP logs

## Next Steps

- [Configuration Guide](configuration.md) - Detailed configuration options
- [API Reference](api-reference.md) - Complete API documentation

