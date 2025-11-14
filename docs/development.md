# Development Guide

Contributing to and extending ERPNext MCP Server.

## Development Setup

### Prerequisites

- **Go 1.24+**
- **Make**
- **Git**
- **ERPNext instance** (for testing)
- **Ollama** (optional, for AI features)

### Clone and Build

```bash
git clone https://github.com/varkrish/frappe-mcp-server
cd frappe-mcp-server

# Install dependencies
make deps

# Build all binaries
make build build-stdio

# Run tests
make test

# Run with coverage
make coverage
```

## Project Structure

```
frappe-mcp-server/
├── cmd/
│   ├── mcp-stdio/          # STDIO server for Cursor/Claude
│   ├── test-client/        # Test client
│   └── ollama-client/      # Ollama test client
├── internal/
│   ├── config/             # Configuration management
│   ├── erpnext/            # ERPNext client
│   ├── mcp/                # MCP protocol implementation
│   ├── server/             # HTTP server & handlers
│   ├── tools/              # MCP tools implementation
│   ├── types/              # Shared types
│   └── utils/              # Utilities
├── docs/                   # Documentation (GitHub Pages)
├── configs/                # Configuration examples
├── main.go                 # HTTP server entry point
├── config.yaml             # Configuration file
├── Makefile                # Build automation
└── go.mod                  # Go dependencies
```

## Key Components

### 1. **MCP Protocol** (`internal/mcp/`)

Implements Model Context Protocol for AI tool integration.

```go
// Server creation
server := mcp.NewServer("frappe-mcp-server", "1.0.0")

// Tool response
response := &mcp.ToolResponse{
    Content: []mcp.Content{
        {Type: "text", Text: "Result"},
    },
}
```

### 2. **ERPNext Client** (`internal/erpnext/`)

HTTP client with retry logic and rate limiting.

```go
client, _ := erpnext.NewClient(config)
doc, _ := client.GetDocument(ctx, "Project", "PROJ-0001")
docs, _ := client.ListDocuments(ctx, "Customer", filters, fields, limit)
```

### 3. **Tools Registry** (`internal/tools/`)

Implements all MCP tools.

```go
registry := tools.NewRegistry(erpClient)
response, err := registry.GetDocument(ctx, request)
```

### 4. **HTTP Server** (`internal/server/`)

Handles HTTP API and AI query processing.

```go
server := server.New(config, erpClient, toolRegistry)
server.Start()
```

## Adding New Tools

### 1. Define Tool in Registry

Edit `internal/tools/registry.go`:

```go
// MyNewTool does something useful
func (t *ToolRegistry) MyNewTool(ctx context.Context, request mcp.ToolRequest) (*mcp.ToolResponse, error) {
    var params struct {
        Param1 string `json:"param1"`
        Param2 int    `json:"param2"`
    }
    
    if err := json.Unmarshal(request.Params, &params); err != nil {
        return nil, fmt.Errorf("invalid parameters: %w", err)
    }
    
    // Implementation
    result := doSomething(params)
    
    resultJSON, _ := json.Marshal(result)
    return &mcp.ToolResponse{
        Content: []mcp.Content{
            {Type: "text", Text: string(resultJSON)},
        },
    }, nil
}
```

### 2. Register in STDIO Server

Edit `cmd/mcp-stdio/main.go`:

```go
case "my_new_tool":
    result, execErr = s.tools.MyNewTool(ctx, toolRequest)
```

### 3. Register in HTTP Server

Edit `internal/server/server.go`:

```go
// In listTools()
{
    Name: "my_new_tool",
    Description: "Does something useful",
    InputSchema: map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "param1": map[string]interface{}{"type": "string"},
            "param2": map[string]interface{}{"type": "integer"},
        },
        "required": []string{"param1"},
    },
}
```

### 4. Add Tests

Create `internal/tools/my_new_tool_test.go`:

```go
func TestMyNewTool(t *testing.T) {
    mockClient := &mockERPNextClient{}
    registry := NewRegistry(mockClient)
    
    request := mcp.ToolRequest{
        Params: json.RawMessage(`{"param1":"value","param2":42}`),
    }
    
    response, err := registry.MyNewTool(context.Background(), request)
    assert.NoError(t, err)
    assert.NotNil(t, response)
}
```

## Testing

### Unit Tests

```bash
# Run all tests
make test

# Run with coverage
make coverage

# Run specific package
go test ./internal/tools/...

# Verbose output
go test -v ./...
```

### Integration Tests

```bash
# Requires ERPNext running
export FRAPPE_BASE_URL="http://localhost:8000"
export FRAPPE_API_KEY="your_key"
export FRAPPE_API_SECRET="your_secret"

go test -tags=integration ./...
```

### Manual Testing

```bash
# Start server in one terminal
./bin/frappe-mcp-server

# Test in another
curl http://localhost:8080/api/v1/health
curl -X POST http://localhost:8080/api/v1/chat \
  -d '{"message": "List all projects"}'
```

## Debugging

### Enable Debug Logging

```yaml
# config.yaml
server:
  log_level: "debug"
```

Or:

```bash
LOG_LEVEL=debug ./bin/frappe-mcp-server
```

### STDIO Debugging

Logs go to stderr to not interfere with STDIO protocol:

```bash
./bin/frappe-mcp-server-stdio --config ./config.yaml 2>&1 | tee debug.log
```

### Ollama Debugging

Test Ollama directly:

```bash
curl -X POST http://localhost:11434/api/generate \
  -d '{"model":"llama3.2:1b","prompt":"Test","stream":false}'
```

## Code Style

### Go Conventions

- Follow [Effective Go](https://golang.org/doc/effective_go)
- Use `gofmt` for formatting
- Run `go vet` for static analysis
- Use `golangci-lint` for comprehensive linting

```bash
# Format code
make fmt

# Vet code
make vet

# Lint
make lint
```

### Project Conventions

- **Errors**: Wrap with context using `fmt.Errorf("context: %w", err)`
- **Logging**: Use structured logging `slog.Info("message", "key", value)`
- **Configs**: Use environment variable override pattern
- **Tests**: Use table-driven tests where applicable

## Making Changes

### 1. Create Branch

```bash
git checkout -b feature/my-new-feature
```

### 2. Make Changes

- Write code
- Add tests
- Update documentation
- Run linters

### 3. Test

```bash
make test
make lint
make build
```

### 4. Commit

```bash
git add .
git commit -m "feat: add new feature"
```

Use [Conventional Commits](https://www.conventionalcommits.org/):
- `feat:` - New feature
- `fix:` - Bug fix
- `docs:` - Documentation
- `refactor:` - Code refactoring
- `test:` - Tests
- `chore:` - Maintenance

### 5. Push and PR

```bash
git push origin feature/my-new-feature
```

Create pull request on GitHub.

## Release Process

### Version Bumping

```bash
# Update version in code
# Tag release
git tag v1.1.0
git push origin v1.1.0
```

### Build Releases

```bash
# Build for multiple platforms
make build-all

# Binaries created in bin/
# - frappe-mcp-server-linux-amd64
# - frappe-mcp-server-darwin-amd64
# - frappe-mcp-server-darwin-arm64
# - frappe-mcp-server-windows-amd64.exe
```

## Troubleshooting

### Build Failures

```bash
# Clean and rebuild
make clean
make deps
make build
```

### Test Failures

```bash
# Run specific test
go test -v -run TestSpecificFunction ./internal/tools/

# With race detector
go test -race ./...
```

### ERPNext Connection Issues

```bash
# Check ERPNext is accessible
curl http://localhost:8000/api/method/frappe.auth.get_logged_user \
  -H "Authorization: token api_key:api_secret"
```

## Performance

### Profiling

```bash
# CPU profile
go test -cpuprofile=cpu.prof -bench=.
go tool pprof cpu.prof

# Memory profile
go test -memprofile=mem.prof -bench=.
go tool pprof mem.prof
```

### Benchmarking

```bash
# Run benchmarks
make bench

# Or specific benchmark
go test -bench=BenchmarkGetDocument -benchmem ./internal/tools/
```

## Documentation

### Code Documentation

```go
// MyFunction does something important.
// It takes param1 and returns a result or error.
func MyFunction(param1 string) (string, error) {
    // Implementation
}
```

### README Updates

When adding features, update:
- Main `README.md`
- Relevant docs in `docs/`
- API reference if adding endpoints

### Generate Docs

```bash
# View Go documentation
godoc -http=:6060
# Visit http://localhost:6060
```

## Resources

- [Go Documentation](https://golang.org/doc/)
- [Model Context Protocol Spec](https://modelcontextprotocol.io/)
- [ERPNext API Docs](https://frappeframework.com/docs/user/en/api)
- [Ollama API](https://github.com/ollama/ollama/blob/main/docs/api.md)

## Getting Help

- **Issues**: [GitHub Issues](https://github.com/varkrish/frappe-mcp-server/issues)
- **Discussions**: [GitHub Discussions](https://github.com/varkrish/frappe-mcp-server/discussions)
- **Email**: Create an issue for questions

---

Happy coding! 🚀

