# Frappe MCP Server Makefile

# Variables
BINARY_NAME=frappe-mcp-server
MAIN_PATH=./main.go
BUILD_DIR=./bin
DOCKER_IMAGE=frappe-mcp-server
VERSION?=1.0.0

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Colors for output
RED=\033[0;31m
GREEN=\033[0;32m
YELLOW=\033[1;33m
NC=\033[0m # No Color

.PHONY: all build clean test coverage lint run dev setup help

# Default target
all: clean lint test build build-stdio build-test-client build-ollama-client

# Build the application
build:
	@echo "$(GREEN)Building $(BINARY_NAME)...$(NC)"
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) -v $(MAIN_PATH)
	@echo "$(GREEN)Build complete: $(BUILD_DIR)/$(BINARY_NAME)$(NC)"

# Build stdio version for MCP clients
build-stdio:
	@echo "$(GREEN)Building stdio MCP server...$(NC)"
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-stdio -v ./cmd/mcp-stdio/main.go
	@echo "$(GREEN)Stdio build complete: $(BUILD_DIR)/$(BINARY_NAME)-stdio$(NC)"

# Build stdio for multiple platforms (for distribution)
build-stdio-all:
	@echo "$(GREEN)Building stdio for multiple platforms...$(NC)"
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-stdio-linux-amd64 ./cmd/mcp-stdio/main.go
	GOOS=linux GOARCH=arm64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-stdio-linux-arm64 ./cmd/mcp-stdio/main.go
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-stdio-darwin-amd64 ./cmd/mcp-stdio/main.go
	GOOS=darwin GOARCH=arm64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-stdio-darwin-arm64 ./cmd/mcp-stdio/main.go
	GOOS=windows GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-stdio-windows-amd64.exe ./cmd/mcp-stdio/main.go
	@echo "$(GREEN)Multi-platform stdio builds complete$(NC)"

# Build test client
build-test-client:
	@echo "$(GREEN)Building test client...$(NC)"
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/test-client -v ./cmd/test-client/main.go
	@echo "$(GREEN)Test client build complete: $(BUILD_DIR)/test-client$(NC)"

# Build Ollama client
build-ollama-client:
	@echo "$(GREEN)Building Ollama client...$(NC)"
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/ollama-client -v ./cmd/ollama-client/main.go
	@echo "$(GREEN)Ollama client build complete: $(BUILD_DIR)/ollama-client$(NC)"

# Build all client components
build-clients: build-stdio build-test-client build-ollama-client
	@echo "$(GREEN)All clients built successfully$(NC)"

# Build for multiple platforms
build-all:
	@echo "$(GREEN)Building for multiple platforms...$(NC)"
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PATH)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PATH)
	GOOS=windows GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PATH)
	@echo "$(GREEN)Multi-platform build complete$(NC)"

# Clean build artifacts
clean:
	@echo "$(YELLOW)Cleaning...$(NC)"
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	@echo "$(GREEN)Clean complete$(NC)"

# Run tests
test:
	@echo "$(GREEN)Running tests...$(NC)"
	$(GOTEST) -v ./...

# Run tests with coverage
coverage:
	@echo "$(GREEN)Running tests with coverage...$(NC)"
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)Coverage report generated: coverage.html$(NC)"

# Run benchmarks
bench:
	@echo "$(GREEN)Running benchmarks...$(NC)"
	$(GOTEST) -bench=. -benchmem ./...

# Lint the code
lint:
	@echo "$(GREEN)Running linter...$(NC)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "$(YELLOW)golangci-lint not installed, skipping lint$(NC)"; \
	fi

# Format code
fmt:
	@echo "$(GREEN)Formatting code...$(NC)"
	$(GOCMD) fmt ./...
	@if command -v goimports >/dev/null 2>&1; then \
		goimports -w .; \
	fi

# Vet code
vet:
	@echo "$(GREEN)Vetting code...$(NC)"
	$(GOCMD) vet ./...

# Run the application
run: build
	@echo "$(GREEN)Running $(BINARY_NAME)...$(NC)"
	./$(BUILD_DIR)/$(BINARY_NAME)

# Run in development mode with hot reload
dev:
	@echo "$(GREEN)Running in development mode...$(NC)"
	@if command -v air >/dev/null 2>&1; then \
		air; \
	else \
		echo "$(YELLOW)Air not installed, running normally$(NC)"; \
		$(GOCMD) run $(MAIN_PATH); \
	fi

# Install dependencies
deps:
	@echo "$(GREEN)Installing dependencies...$(NC)"
	$(GOMOD) download
	$(GOMOD) tidy

# Setup development environment
setup:
	@echo "$(GREEN)Setting up development environment...$(NC)"
	$(GOMOD) download
	@if [ ! -f .env ]; then \
		cp .env.example .env; \
		echo "$(YELLOW)Created .env file from template. Please update it with your settings.$(NC)"; \
	fi
	@echo "$(GREEN)Setup complete$(NC)"

# Install development tools
tools:
	@echo "$(GREEN)Installing development tools...$(NC)"
	$(GOGET) -u github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GOGET) -u golang.org/x/tools/cmd/goimports@latest
	$(GOGET) -u github.com/cosmtrek/air@latest
	@echo "$(GREEN)Development tools installed$(NC)"

# Docker build
docker-build:
	@echo "$(GREEN)Building Docker image...$(NC)"
	docker build -t $(DOCKER_IMAGE):$(VERSION) .
	docker tag $(DOCKER_IMAGE):$(VERSION) $(DOCKER_IMAGE):latest
	@echo "$(GREEN)Docker image built: $(DOCKER_IMAGE):$(VERSION)$(NC)"

# Docker run
docker-run:
	@echo "$(GREEN)Running Docker container...$(NC)"
	docker run --rm -p 8080:8080 --env-file .env $(DOCKER_IMAGE):latest

# Generate documentation
docs:
	@echo "$(GREEN)Generating documentation...$(NC)"
	@if command -v godoc >/dev/null 2>&1; then \
		echo "$(GREEN)Documentation server available at: http://localhost:6060/pkg/$(shell go list -m)/$(NC)"; \
		godoc -http=:6060; \
	else \
		echo "$(YELLOW)godoc not installed. Install with: go install golang.org/x/tools/cmd/godoc@latest$(NC)"; \
	fi

# Security scan
security:
	@echo "$(GREEN)Running security scan...$(NC)"
	@if command -v gosec >/dev/null 2>&1; then \
		gosec ./...; \
	else \
		echo "$(YELLOW)gosec not installed. Install with: go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest$(NC)"; \
	fi

# Check for updates
update-deps:
	@echo "$(GREEN)Checking for dependency updates...$(NC)"
	$(GOCMD) list -u -m all

# Pre-commit checks
pre-commit: fmt vet lint test
	@echo "$(GREEN)Pre-commit checks passed$(NC)"

# Release build (builds all platforms for both HTTP and STDIO servers)
release: clean lint test build-all build-stdio-all
	@echo "$(GREEN)Release build complete$(NC)"
	@echo "$(GREEN)Binaries available in $(BUILD_DIR)$(NC)"
	@ls -lh $(BUILD_DIR)

# Show help
help:
	@echo "$(GREEN)Available commands:$(NC)"
	@echo "  $(YELLOW)build$(NC)        - Build the HTTP server"
	@echo "  $(YELLOW)build-stdio$(NC)  - Build the STDIO MCP server"
	@echo "  $(YELLOW)build-all$(NC)    - Build HTTP server for multiple platforms"
	@echo "  $(YELLOW)build-stdio-all$(NC) - Build STDIO server for multiple platforms"
	@echo "  $(YELLOW)clean$(NC)        - Clean build artifacts"
	@echo "  $(YELLOW)test$(NC)         - Run tests"
	@echo "  $(YELLOW)coverage$(NC)     - Run tests with coverage report"
	@echo "  $(YELLOW)bench$(NC)        - Run benchmarks"
	@echo "  $(YELLOW)lint$(NC)         - Run linter"
	@echo "  $(YELLOW)fmt$(NC)          - Format code"
	@echo "  $(YELLOW)vet$(NC)          - Vet code"
	@echo "  $(YELLOW)run$(NC)          - Build and run the application"
	@echo "  $(YELLOW)dev$(NC)          - Run in development mode with hot reload"
	@echo "  $(YELLOW)deps$(NC)         - Install dependencies"
	@echo "  $(YELLOW)setup$(NC)        - Setup development environment"
	@echo "  $(YELLOW)tools$(NC)        - Install development tools"
	@echo "  $(YELLOW)docker-build$(NC) - Build Docker image"
	@echo "  $(YELLOW)docker-run$(NC)   - Run Docker container"
	@echo "  $(YELLOW)docs$(NC)         - Generate documentation"
	@echo "  $(YELLOW)security$(NC)     - Run security scan"
	@echo "  $(YELLOW)update-deps$(NC)  - Check for dependency updates"
	@echo "  $(YELLOW)pre-commit$(NC)   - Run pre-commit checks"
	@echo "  $(YELLOW)release$(NC)      - Create release build"
	@echo "  $(YELLOW)help$(NC)         - Show this help message"
