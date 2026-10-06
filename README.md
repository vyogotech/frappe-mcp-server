# ERPNext MCP Server

> **AI-powered Model Context Protocol server for ERPNext and Frappe applications**

Publish ERPNext and other Frappe-based apps to an AI assistant as MCP tools. Use with Cursor IDE, Claude Desktop, and any other MCP client.

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## ✨ Features

- 🔌 **MCP Protocol** - Standard protocol for AI tool integration
- 📊 **Generic Tools** - Works with ANY ERPNext doctype (standard or custom)
- 📈 **Advanced Analytics** 🆕 - Aggregations (SUM, COUNT, AVG, TOP N) and ERPNext reports
- 🔐 **OAuth2 Authentication** - Standard OAuth2 security with token caching
- 🚀 **Production Ready** - Built with Go for performance

## 🚀 Quick Start

### Option 1: Automated Install (Recommended for MCP)

```bash
# Install the STDIO binary for MCP clients (Cursor, Claude Desktop)
curl -fsSL https://raw.githubusercontent.com/vyogotech/frappe-mcp-server/main/install.sh | bash
```

This installs the MCP server binary to `~/.local/bin/frappe-mcp-server-stdio`

### Option 2: Manual Install

```bash
# 1. Download pre-built binary from releases
# Visit: https://github.com/vyogotech/frappe-mcp-server/releases/latest

# 2. Or build from source
git clone https://github.com/vyogotech/frappe-mcp-server
cd frappe-mcp-server
make build-stdio

# 3. Configure
cp config.yaml.example config.yaml
# Edit config.yaml with your Frappe/ERPNext credentials
```

## 📖 Documentation

**Complete documentation:** [https://vyogotech.github.io/frappe-mcp-server/](https://vyogotech.github.io/frappe-mcp-server/)

**Key guides:**

- [Quick Start](https://vyogotech.github.io/frappe-mcp-server/quick-start) - Get running in 5 minutes
- [Authentication](https://vyogotech.github.io/frappe-mcp-server/authentication) - sid cookie, OAuth2, and API key auth
- [Auth Quick Start](https://vyogotech.github.io/frappe-mcp-server/auth-quickstart) - Set up auth in 5 minutes
- [Docker Deployment](https://vyogotech.github.io/frappe-mcp-server/docker) - Deploy with Docker Compose
- [API Reference](https://vyogotech.github.io/frappe-mcp-server/api-reference) - Complete API docs

## 💡 Usage Examples

### Cursor IDE

```text
@erpnext List all open projects
@erpnext Show me customer ABC-CORP
@erpnext What are the pending tasks?
@erpnext Show me top 5 customers by revenue  🆕
@erpnext Run Sales Analytics report  🆕
```

### HTTP API

Every path except the health probes is authenticated when `auth.require_auth` is on, as
`config.yaml.example` ships it: send the caller's OAuth2 bearer token or their Frappe `sid` cookie.
Without one the server answers `401 Unauthorized`.

```bash
# List the tools this server publishes
curl http://localhost:8080/api/v1/tools

# Call one of them
curl -X POST http://localhost:8080/api/v1/tools/get_document \
  -H "Content-Type: application/json" \
  -d '{"params": {"doctype": "Project", "name": "PROJ-0001"}}'
```

### Claude Desktop

Add to `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "erpnext": {
      "command": "/path/to/bin/frappe-mcp-server-stdio",
      "args": ["--config", "/path/to/config.yaml"]
    }
  }
}
```

## 🏗️ Architecture

```text
┌─────────────────┐
│   AI Clients    │
│ Cursor, Claude  │
└────────┬────────┘
         │ MCP Protocol
         ↓
┌─────────────────┐
│  ERPNext MCP    │
│     Server      │
└────────┬────────┘
         │ REST API
         ↓
┌─────────────────┐
│    ERPNext      │
│   (Frappe API)  │
└─────────────────┘
```

## 🛠️ Prerequisites

- Go 1.25+ — `go.mod` names the toolchain the build uses
- ERPNext instance (local or remote)

## 📋 Available Tools

Both binaries register the same catalogue, so `tools/list` answers the same over stdio and over
HTTP. `search_knowledge_base` is registered only where the `rag` app it reads is installed
(`tools.knowledge_base`). The six legacy names are still callable and carry no description, so a
model is never told to reach for them. `go test ./internal/tools -run Readme` fails when the table
below and the catalogue drift apart, and prints the table to paste back in.

<!-- tools:start -->
| Tool | What the model is told |
| --- | --- |
| `get_document` | Retrieve a single ERPNext document by doctype and name |
| `list_documents` | Fetch document rows to read their contents. Returns at most page_length rows (default 20), so it CANNOT be used to count records - use aggregate_documents for counts. |
| `create_document` | Create a new ERPNext document. `data` is a flat object of fieldname→value pairs (NOT spread into top-level args). |
| `update_document` | Update an existing ERPNext document. `data` is a flat object of fieldname→value pairs for fields to change. |
| `delete_document` | Delete an ERPNext document |
| `search_documents` | Search ERPNext documents of a given doctype using full-text search |
| `aggregate_documents` | Count, sum or average ERPNext records. Use this for any "how many" question: with metric="count" it returns the exact total of all matching records, not just one page. |
| `run_report` | Execute a Frappe/ERPNext report (Sales Analytics, Purchase Register, etc.). Returns at most 100 rows; the result says how many the report had. |
| `global_search` | Full-text search across all indexed Frappe/ERPNext doctypes |
| `search_knowledge_base` | Search the user's uploaded documents (HR, expense, travel, security and vehicle policies) for a passage answering a question. Use for any policy, entitlement, limit or deadline question. |
| `analyze_document` | Analyze any ERPNext document with optional related data |
| `get_project_status` | Legacy name, still callable, described to nobody |
| `analyze_project_timeline` | Legacy name, still callable, described to nobody |
| `get_resource_allocation` | Legacy name, still callable, described to nobody |
| `generate_project_report` | Legacy name, still callable, described to nobody |
| `resource_utilization_analysis` | Legacy name, still callable, described to nobody |
| `budget_variance_analysis` | Legacy name, still callable, described to nobody |
<!-- tools:end -->

The REST listing at `GET /api/v1/tools` publishes only the described tools; `POST /mcp` publishes
all of them.

## 🤝 Contributing

Contributions welcome! See [Development Guide](https://vyogotech.github.io/frappe-mcp-server/development).

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## 📄 License

MIT License - see [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- [ERPNext](https://erpnext.com) - Open source ERP
- [Model Context Protocol](https://modelcontextprotocol.io/) - AI integration standard
- [Cursor](https://cursor.sh) - AI-powered IDE

## 📞 Support

- **Documentation**: [GitHub Pages](https://vyogotech.github.io/frappe-mcp-server/)
- **Issues**: [GitHub Issues](https://github.com/vyogotech/frappe-mcp-server/issues)
- **Discussions**: [GitHub Discussions](https://github.com/vyogotech/frappe-mcp-server/discussions)

---

Made with ❤️ by the community
