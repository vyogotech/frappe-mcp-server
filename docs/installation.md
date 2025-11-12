# Installation Guide

Complete installation guide for Frappe MCP Server.

## Installation Methods

### 🚀 Method 1: Automated Install Script (Recommended)

The easiest way to install:

```bash
curl -fsSL https://raw.githubusercontent.com/varkrish/frappe-mcp-server/main/install.sh | bash
```

**What it does:**
- Auto-detects your OS and architecture
- Downloads the latest release binary
- Installs to `~/.local/bin/frappe-mcp-server-stdio`
- Makes it executable and ready to use
- Creates configuration directory

**Supported platforms:**
- Linux (amd64, arm64)
- macOS (Intel, Apple Silicon)
- Windows (amd64)

### 📦 Method 2: Pre-built Binaries

Download pre-built binaries from [GitHub Releases](https://github.com/varkrish/frappe-mcp-server/releases/latest).

#### Available Downloads

| Platform | Architecture | Download |
|----------|-------------|----------|
| Linux | Intel/AMD (64-bit) | `frappe-mcp-server-stdio-linux-amd64.tar.gz` |
| Linux | ARM (64-bit) | `frappe-mcp-server-stdio-linux-arm64.tar.gz` |
| macOS | Intel | `frappe-mcp-server-stdio-darwin-amd64.tar.gz` |
| macOS | Apple Silicon (M1/M2/M3) | `frappe-mcp-server-stdio-darwin-arm64.tar.gz` |
| Windows | 64-bit | `frappe-mcp-server-stdio-windows-amd64.zip` |

#### Manual Installation Steps

**Linux/macOS:**
```bash
# Download (replace with your platform)
wget https://github.com/varkrish/frappe-mcp-server/releases/latest/download/frappe-mcp-server-stdio-darwin-arm64.tar.gz

# Extract
tar -xzf frappe-mcp-server-stdio-darwin-arm64.tar.gz

# Move to PATH
sudo mv frappe-mcp-server-stdio-darwin-arm64/frappe-mcp-server-stdio /usr/local/bin/

# Make executable
chmod +x /usr/local/bin/frappe-mcp-server-stdio

# Verify installation
frappe-mcp-server-stdio --help
```

**Windows:**
```powershell
# Download from releases page
# Extract the .zip file
# Move frappe-mcp-server-stdio.exe to a directory in your PATH
# Or use the full path when configuring MCP clients
```

### 🔨 Method 3: Build from Source

For developers or if you want the latest unreleased version:

#### Prerequisites
- **Go 1.24+** - [Download](https://go.dev/dl/)
- **Git**
- **Make** (optional, but recommended)

#### Build Steps

```bash
# Clone the repository
git clone https://github.com/varkrish/frappe-mcp-server.git
cd frappe-mcp-server

# Install dependencies
go mod download

# Build the STDIO binary (for MCP clients)
make build-stdio

# Or without Make:
go build -o bin/frappe-mcp-server-stdio ./cmd/mcp-stdio/main.go

# Binary will be at: ./bin/frappe-mcp-server-stdio
```

#### Build for Multiple Platforms

```bash
# Build for all platforms
make build-stdio-all

# Binaries will be in ./bin/:
# - frappe-mcp-server-stdio-linux-amd64
# - frappe-mcp-server-stdio-linux-arm64
# - frappe-mcp-server-stdio-darwin-amd64
# - frappe-mcp-server-stdio-darwin-arm64
# - frappe-mcp-server-stdio-windows-amd64.exe
```

## Verify Installation

After installation, verify it works:

```bash
# Check if binary is accessible
which frappe-mcp-server-stdio

# Check version (if you built with version info)
frappe-mcp-server-stdio --version

# Test with help flag
frappe-mcp-server-stdio --help
```

## Configuration

After installation, you need to configure the server:

### Option 1: Environment Variables (in MCP client config)

Add to your MCP client configuration (`~/.cursor/mcp.json` or Claude Desktop config):

```json
{
  "mcpServers": {
    "frappe": {
      "command": "/path/to/frappe-mcp-server-stdio",
      "env": {
        "ERPNEXT_BASE_URL": "https://your-frappe-instance.com",
        "ERPNEXT_API_KEY": "your_api_key",
        "ERPNEXT_API_SECRET": "your_api_secret",
        "LLM_PROVIDER_TYPE": "openai-compatible",
        "LLM_BASE_URL": "http://localhost:11434/v1",
        "LLM_MODEL": "llama3.2:1b"
      }
    }
  }
}
```

### Option 2: Configuration File

Create `config.yaml`:

```yaml
erpnext:
  base_url: "https://your-frappe-instance.com"
  api_key: "your_api_key"
  api_secret: "your_api_secret"
  timeout: "30s"

llm:
  provider_type: "openai-compatible"
  base_url: "http://localhost:11434/v1"
  model: "llama3.2:1b"
  api_key: ""
```

Then reference it in MCP config:

```json
{
  "mcpServers": {
    "frappe": {
      "command": "/path/to/frappe-mcp-server-stdio",
      "args": ["--config", "/path/to/config.yaml"]
    }
  }
}
```

## MCP Client Setup

### Cursor IDE

1. Open Cursor Settings
2. Go to MCP settings or create `~/.cursor/mcp.json`
3. Add the Frappe MCP server configuration
4. Restart Cursor

Example `~/.cursor/mcp.json`:
```json
{
  "mcpServers": {
    "frappe": {
      "command": "/usr/local/bin/frappe-mcp-server-stdio",
      "args": ["--config", "/Users/you/.config/frappe-mcp-server/config.yaml"],
      "cwd": "/Users/you/.config/frappe-mcp-server"
    }
  }
}
```

### Claude Desktop

1. Open Claude Desktop settings
2. Find MCP configuration file:
   - **macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
   - **Windows**: `%APPDATA%\Claude\claude_desktop_config.json`
3. Add the Frappe MCP server configuration
4. Restart Claude Desktop

## Troubleshooting

### Binary not found in PATH

If you get "command not found":

**Option 1:** Add to PATH
```bash
# Add to ~/.bashrc or ~/.zshrc
export PATH="$PATH:$HOME/.local/bin"

# Reload shell
source ~/.bashrc  # or source ~/.zshrc
```

**Option 2:** Use absolute path in MCP config
```json
{
  "command": "/full/path/to/frappe-mcp-server-stdio"
}
```

### Permission denied

```bash
# Make binary executable
chmod +x /path/to/frappe-mcp-server-stdio
```

### Connection errors

- Verify your Frappe instance is accessible
- Check API credentials are correct
- Test manually: `curl https://your-frappe-instance.com/api/method/ping`

### LLM/AI features not working

- Ensure Ollama is running: `ollama list`
- Check LLM configuration in config.yaml
- Verify the model is pulled: `ollama pull llama3.2:1b`

## Updating

To update to the latest version:

```bash
# With install script
curl -fsSL https://raw.githubusercontent.com/varkrish/frappe-mcp-server/main/install.sh | bash

# Or download manually from releases
# Then replace your existing binary
```

## Uninstallation

```bash
# Remove binary
rm ~/.local/bin/frappe-mcp-server-stdio
# or
sudo rm /usr/local/bin/frappe-mcp-server-stdio

# Remove configuration (optional)
rm -rf ~/.config/frappe-mcp-server

# Remove from MCP client configs
# Edit ~/.cursor/mcp.json or Claude Desktop config
```

## Next Steps

- [Quick Start Guide](quick-start.md) - Get started quickly
- [Configuration Guide](configuration.md) - Detailed configuration options
- [LLM Providers](llm-providers.md) - Setup different AI providers
- [API Reference](api-reference.md) - Explore available tools

