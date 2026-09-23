# ERPNext MCP Server

> **Model Context Protocol server publishing ERPNext and Frappe-based apps to AI assistants as tools**

## Overview

ERPNext MCP Server bridges ERPNext and other Frappe-based applications with AI assistants through the Model Context Protocol (MCP). It publishes the tools; the assistant on the other side decides which to call. It integrates with Cursor IDE, Claude Desktop and any other MCP client.

## Key Features

- 🔌 **MCP Protocol Support** - STDIO and HTTP interfaces for AI tool integration
- 📊 **Generic Document Tools** - Works with ANY ERPNext doctype (standard or custom)
- 📈 **Advanced Analytics** - Aggregation queries (SUM, COUNT, AVG, TOP N) and report execution
- 🔐 **OAuth2 Authentication** - Standard OAuth2 security with token caching
- 🚀 **Production-Ready** - Built with Go for performance and reliability

## Quick Links

### Getting Started
- **[Installation Guide](installation.md)** - Complete installation options
- [Quick Start Guide](quick-start.md) - Get up and running in 5 minutes
- [Configuration](configuration.md) - Setup and customize your server

### Security (NEW!)
- **[OAuth2 Authentication](authentication.md)** - 🔐 Complete OAuth2 authentication guide
- [Auth Quick Start](auth-quickstart.md) - Set up authentication in 5 minutes
- [Implementation Details](oauth2-implementation.md) - Technical implementation deep dive

### Deployment & Operations
- [Docker Deployment](docker.md) - Deploy with Docker Compose
- [Distribution & Releases](distribution.md) - Release process and distribution system

### Usage & Development
- [Analytics & Reporting](analytics-features.md) - 🆕 Aggregations and report execution
- [API Reference](api-reference.md) - Complete API documentation
- [Development](development.md) - Contributing and extending

## Use Cases

### 1. **IDE Integration (Cursor)**
Ask questions directly in your IDE:
- *"Show me details of project PROJ-0001"*
- *"List all open sales orders"*
- *"What are the pending tasks for customer CUST-123?"*

### 2. **Chat Interfaces (Claude Desktop)**
Natural conversations with your ERPNext data:
- Analyze project timelines
- Generate reports
- Query any document type

### 3. **Business Analytics** 🆕
Ask complex analytical questions:
- *"Show me top 5 customers by revenue"*
- *"What are total sales by item this month?"*
- *"Run Sales Analytics report"*
- *"Which products sold the most?"*

### 4. **Automation & Integration**
Build custom workflows and integrations using MCP tools.

## Architecture

```
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

## Getting Started

### Quick Install

```bash
# One-command installation
curl -fsSL https://raw.githubusercontent.com/vyogotech/frappe-mcp-server/main/install.sh | bash
```

Or download pre-built binaries from [Releases](https://github.com/vyogotech/frappe-mcp-server/releases/latest).

### Prerequisites

- Frappe/ERPNext instance with API access
- MCP client (Cursor IDE or Claude Desktop)

See [Installation Guide](installation.md) for complete installation options.

## Community

- **GitHub**: [frappe-mcp-server](https://github.com/vyogotech/frappe-mcp-server)
- **Issues**: Report bugs or request features
- **Discussions**: Share ideas and get help

## License

MIT License - see LICENSE file for details.

