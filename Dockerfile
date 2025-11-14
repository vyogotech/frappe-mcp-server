# Multi-stage Docker build for ERPNext MCP Server

# Build stage
FROM golang:1.24-alpine AS builder

# Install git (required for some Go modules)
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o frappe-mcp-server main.go

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests and curl for healthchecks
RUN apk --no-cache add ca-certificates tzdata curl

# Set working directory first
WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/frappe-mcp-server /app/frappe-mcp-server

# Create logs directory and set permissions before creating user
RUN mkdir -p /app/logs

# Create non-root user with explicit home directory
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup -h /app && \
    chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Run the application
CMD ["./frappe-mcp-server"]
