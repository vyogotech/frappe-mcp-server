# ERPNext MCP Server

> **AI-powered Model Context Protocol server for ERPNext and Frappe applications**

Connect ERPNext and other Frappe-based apps with AI assistants through natural language. Use with Cursor IDE, Claude Desktop, Open WebUI, and more.

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## ✨ Features

- 🤖 **Natural Language Queries** - Ask questions in plain English
- 🎯 **Truly Generic LLM Support** - Works with ANY OpenAI-compatible API
  - Ollama, OpenAI, Together.ai, Groq, OpenRouter, Replicate, LocalAI, LM Studio, etc.
  - Simple 3-field config: `base_url`, `api_key`, `model`
- 🔌 **MCP Protocol** - Standard protocol for AI tool integration
- 📊 **Generic Tools** - Works with ANY ERPNext doctype (standard or custom)
- 📈 **Advanced Analytics** 🆕 - Aggregations (SUM, COUNT, AVG, TOP N) and ERPNext reports
- 🔐 **OAuth2 Authentication** - Standard OAuth2 security with token caching
- 🔒 **Privacy First** - Local AI option with Ollama
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
- [Generic LLM Config](https://vyogotech.github.io/frappe-mcp-server/generic-llm-config) - Simple 3-field config
- [LLM Providers](https://vyogotech.github.io/frappe-mcp-server/llm-providers) - Ollama, OpenAI, Together.ai, Groq, etc.
- [Docker Deployment](https://vyogotech.github.io/frappe-mcp-server/docker) - Deploy with Docker Compose
- [API Reference](https://vyogotech.github.io/frappe-mcp-server/api-reference) - Complete API docs

## 💡 Usage Examples

### Cursor IDE

```
@erpnext List all open projects
@erpnext Show me customer ABC-CORP
@erpnext What are the pending tasks?
@erpnext Show me top 5 customers by revenue  🆕
@erpnext Run Sales Analytics report  🆕
```

### HTTP API

```bash
# Get documents
curl -X POST http://localhost:8080/api/v1/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "Show me project PROJ-0001"}'

# Analytics 🆕
curl -X POST http://localhost:8080/api/v1/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "top 10 customers by revenue in table format"}'
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

```
┌─────────────────┐
│   AI Clients    │
│ Cursor, Claude  │
└────────┬────────┘
         │ MCP Protocol
         ↓
┌─────────────────┐     ┌──────────────┐
│  ERPNext MCP    │────→│   Ollama     │
│     Server      │     │  (llama3.2)  │
└────────┬────────┘     └──────────────┘
         │ REST API
         ↓
┌─────────────────┐
│    ERPNext      │
│   (Frappe API)  │
└─────────────────┘
```

## 🛠️ Prerequisites

- Go 1.24+
- ERPNext instance (local or remote)
- Ollama (optional, for AI features)

## 📋 Available Tools

- **CRUD Operations**: `get_document`, `list_documents`, `create_document`, `update_document`, `delete_document`
- **Search**: `search_documents` - Find documents by query
- **Analysis**: `analyze_document` - Deep analysis with related documents (works with ANY doctype)
- **Project Tools**: `get_project_status`, `portfolio_dashboard`, `analyze_project_timeline`

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
- [Ollama](https://ollama.ai) - Local AI model serving
- [Cursor](https://cursor.sh) - AI-powered IDE

## 📞 Support

- **Documentation**: [GitHub Pages](https://vyogotech.github.io/frappe-mcp-server/)
- **Issues**: [GitHub Issues](https://github.com/vyogotech/frappe-mcp-server/issues)
- **Discussions**: [GitHub Discussions](https://github.com/vyogotech/frappe-mcp-server/discussions)

---

Made with ❤️ by the community
