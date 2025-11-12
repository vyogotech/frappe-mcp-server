# ERPNext MCP Server

> **AI-powered Model Context Protocol server enabling natural language interactions with ERPNext and Frappe-based apps**

## Overview

ERPNext MCP Server bridges ERPNext and other Frappe-based applications with AI assistants through the Model Context Protocol (MCP), enabling natural language queries, intelligent document analysis, and seamless integration with tools like Cursor IDE, Claude Desktop, and Open WebUI.

## Key Features

- 🤖 **AI-Powered Query Processing** - Natural language understanding using local LLM (Ollama)
- 🔌 **MCP Protocol Support** - STDIO and HTTP interfaces for AI tool integration
- 📊 **Generic Document Tools** - Works with ANY ERPNext doctype (standard or custom)
- 🔒 **Privacy-Focused** - All AI processing runs locally via Ollama
- 🚀 **Production-Ready** - Built with Go for performance and reliability

## Quick Links

### Getting Started
- **[Installation Guide](installation.md)** - Complete installation options
- [Quick Start Guide](quick-start.md) - Get up and running in 5 minutes
- [Configuration](configuration.md) - Setup and customize your server

### LLM Configuration
- [Generic LLM Config](generic-llm-config.md) - Simple 3-field config for ANY provider
- [LLM Providers](llm-providers.md) - Detailed provider guide (OpenAI, Together.ai, Groq, etc.)

### Deployment & Operations
- [Docker Deployment](docker.md) - Deploy with Docker Compose
- [Distribution & Releases](distribution.md) - Release process and distribution system

### Usage & Development
- [AI Features](ai-features.md) - Learn about NLP and AI capabilities
- [API Reference](api-reference.md) - Complete API documentation
- [Development](development.md) - Contributing and extending
- [Implementation Details](llm-implementation.md) - Technical architecture deep dive

## Use Cases

### 1. **IDE Integration (Cursor)**
Ask questions directly in your IDE:
- *"Show me details of project PROJ-0001"*
- *"List all open sales orders"*
- *"What are the pending tasks for customer CUST-123?"*

### 2. **Chat Interfaces (Open WebUI, Claude)**
Natural conversations with your ERPNext data:
- Analyze project timelines
- Generate reports
- Query any document type

### 3. **Automation & Integration**
Build custom workflows and integrations using MCP tools.

## Architecture

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

## Getting Started

### Quick Install

```bash
# One-command installation
curl -fsSL https://raw.githubusercontent.com/varkrish/frappe-mcp-server/main/install.sh | bash
```

Or download pre-built binaries from [Releases](https://github.com/varkrish/frappe-mcp-server/releases/latest).

### Prerequisites

- Frappe/ERPNext instance with API access
- Ollama (optional, for AI features)
- MCP client (Cursor IDE or Claude Desktop)

See [Installation Guide](installation.md) for complete installation options.

## Community

- **GitHub**: [frappe-mcp-server](https://github.com/varkrish/frappe-mcp-server)
- **Issues**: Report bugs or request features
- **Discussions**: Share ideas and get help

## License

MIT License - see LICENSE file for details.

