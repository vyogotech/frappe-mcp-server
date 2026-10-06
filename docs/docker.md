# Docker Deployment

Run ERPNext MCP Server and dependencies using Docker Compose.

## Quick Start

### 1. Setup Environment

```bash
# Copy environment template
cp env.example .env

# Edit .env with your ERPNext credentials
nano .env
```

Required variables in `.env`:

```bash
FRAPPE_BASE_URL=http://your-frappe-instance:8000
FRAPPE_API_KEY=your_api_key
FRAPPE_API_SECRET=your_api_secret
```

### 2. Start Services

#### Option A: MCP server only (recommended)

```bash
docker compose up -d
```

This starts:

- ERPNext MCP Server (port 8080)

#### Option B: full stack, including a local ERPNext

```bash
docker compose --profile full-stack up -d
```

This starts everything including a local ERPNext instance.

### 3. Access Services

- **ERPNext MCP API**: <http://localhost:8080>
- **ERPNext** (if full-stack): <http://localhost:8000>

## Configuration

### Environment Variables

| Variable | Default | Description |
| ---------- | --------- | ------------- |
| `FRAPPE_BASE_URL` | `http://localhost:8000` | Frappe instance URL |
| `FRAPPE_API_KEY` | - | Frappe API key (required) |
| `FRAPPE_API_SECRET` | - | Frappe API secret (required) |
| `MCP_PORT` | `8080` | Host port the MCP server is published on (read by `compose.yml`, not by the server) |
| `LOG_LEVEL` | `info` | Logging level |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | unset | OTLP/HTTP collector; tracing stays off while it is unset |

### Custom Configuration

Override `config.yaml` by mounting your own:

```yaml
services:
  frappe-mcp-server:
    volumes:
      - ./my-config.yaml:/app/config.yaml:ro
```

## Usage

### Testing the API

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

### Accessing Logs

```bash
# View MCP server logs
docker compose logs -f frappe-mcp-server

# View all logs
docker compose logs -f

# View specific service
docker compose logs -f erpnext
```

## Management

### Start/Stop Services

```bash
# Start all services
docker compose up -d

# Stop all services
docker compose down

# Restart a service
docker compose restart frappe-mcp-server
```

### Update Services

```bash
# Pull latest images
docker compose pull

# Rebuild and restart
docker compose up -d --build
```

### View Status

```bash
# Check service status
docker compose ps

# Check resource usage
docker compose stats
```

## Data Persistence

Data is stored in Docker volumes:

```bash
# List volumes
docker volume ls | grep erpnext-mcp

# Backup a volume
docker run --rm -v frappe-mcp-server_mcp_logs:/data \
  -v $(pwd)/backups:/backup \
  alpine tar czf /backup/mcp-logs.tar.gz -C /data .
```

Volumes:
<<<<<<< HEAD

- `mcp_logs` - MCP server logs
=======
- `ollama_data` - AI models
- `open_webui_data` - Open WebUI data

>>>>>>> 475c30e (build(docker): build the server image from the repo itself and state the Go version once, in go.mod)

- `erpnext_data` - ERPNext files (full-stack only)

## Troubleshooting

### Port Conflicts

If ports are already in use, change them in `.env`:

```bash
MCP_PORT=8081
```

### ERPNext Connection Failed

Check your ERPNext credentials:

```bash
# Test ERPNext API
curl http://your-erpnext:8000/api/method/frappe.auth.get_logged_user \
  -H "Authorization: token api_key:api_secret"
```

### View Container Health

```bash
docker compose ps
# Look for "healthy" status
```

## Production Deployment

For production, create a separate `compose.prod.yml`:

```yaml
services:
  frappe-mcp-server:
    restart: always
    environment:
      LOG_LEVEL: warn
      OTEL_EXPORTER_OTLP_ENDPOINT: http://otel-collector:4318
    deploy:
      resources:
        limits:
          memory: 1G
          cpus: '1'
```

Run with:

```bash
docker compose -f compose.yml -f compose.prod.yml up -d
```

### Security Best Practices

1. **Use HTTPS** with reverse proxy (nginx/caddy)

2. **Restrict network access**:

```yaml
services:
  erpnext:
    ports: []  # Don't expose to host
```

1. **Regular backups** of volumes

## Advanced Configuration

### Using External ERPNext

Remove the `erpnext` service and set:

```bash
FRAPPE_BASE_URL=https://your-frappe.com
```

### Resource Limits

```yaml
services:
  frappe-mcp-server:
    deploy:
      resources:
        limits:
          cpus: '0.5'
          memory: 512M
```

## Monitoring

View health status:

```bash
# All services
docker compose ps

# Check health endpoint
curl http://localhost:8080/api/v1/health
```

## Cleanup

```bash
# Stop and remove containers
docker compose down

# Remove volumes (WARNING: deletes all data!)
docker compose down -v

# Remove images
docker compose down --rmi all
```

---

Back to [Documentation Home](index.md)
