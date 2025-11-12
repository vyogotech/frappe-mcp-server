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
ERPNEXT_BASE_URL=http://your-erpnext-instance:8000
ERPNEXT_API_KEY=your_api_key
ERPNEXT_API_SECRET=your_api_secret
```

### 2. Start Services

**Option A: MCP Server + Ollama + Open WebUI** (Recommended)

```bash
docker compose up -d
```

This starts:
- ERPNext MCP Server (port 8080)
- Ollama (port 11434)
- Open WebUI (port 3000)

**Option B: Full Stack (includes local ERPNext)**

```bash
docker compose --profile full-stack up -d
```

This starts everything including a local ERPNext instance.

### 3. Initialize Ollama

After first start, pull the AI model:

```bash
# Pull the default model
docker compose exec ollama ollama pull llama3.2:1b

# Verify
docker compose exec ollama ollama list
```

### 4. Access Services

- **ERPNext MCP API**: http://localhost:8080
- **Open WebUI**: http://localhost:3000
- **Ollama API**: http://localhost:11434
- **ERPNext** (if full-stack): http://localhost:8000

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `ERPNEXT_BASE_URL` | `http://localhost:8000` | ERPNext instance URL |
| `ERPNEXT_API_KEY` | - | ERPNext API key (required) |
| `ERPNEXT_API_SECRET` | - | ERPNext API secret (required) |
| `OLLAMA_URL` | `http://ollama:11434` | Ollama service URL |
| `OLLAMA_MODEL` | `llama3.2:1b` | AI model to use |
| `MCP_PORT` | `8080` | MCP server port |
| `OLLAMA_PORT` | `11434` | Ollama port |
| `WEBUI_PORT` | `3000` | Open WebUI port |
| `LOG_LEVEL` | `info` | Logging level |

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

# Natural language query
curl -X POST http://localhost:8080/api/v1/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "List all projects"}'
```

### Using Open WebUI

1. Open http://localhost:3000
2. Sign up / Login
3. Start chatting with your ERPNext data!

### Accessing Logs

```bash
# View MCP server logs
docker compose logs -f frappe-mcp-server

# View all logs
docker compose logs -f

# View specific service
docker compose logs -f ollama
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
docker run --rm -v frappe-mcp-server_ollama_data:/data \
  -v $(pwd)/backups:/backup \
  alpine tar czf /backup/ollama-data.tar.gz -C /data .
```

Volumes:
- `ollama_data` - AI models
- `open_webui_data` - Open WebUI data
- `mcp_logs` - MCP server logs
- `erpnext_data` - ERPNext files (full-stack only)

## Troubleshooting

### Port Conflicts

If ports are already in use, change them in `.env`:

```bash
MCP_PORT=8081
WEBUI_PORT=3001
```

### Ollama Not Working

```bash
# Check Ollama is healthy
docker compose ps ollama

# Test Ollama directly
curl http://localhost:11434/api/tags

# Pull model manually
docker compose exec ollama ollama pull llama3.2:1b
```

### ERPNext Connection Failed

Check your ERPNext credentials:

```bash
# Test ERPNext API
curl http://your-erpnext:8000/api/method/frappe.auth.get_logged_user \
  -H "Authorization: token api_key:api_secret"
```

### Out of Memory

Increase Docker memory limits or use a smaller model:

```bash
OLLAMA_MODEL=llama3.2:1b  # Smaller model (1.2GB)
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
      ENABLE_METRICS: true
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

1. **Use strong secrets**:
```bash
WEBUI_SECRET_KEY=$(openssl rand -hex 32)
```

2. **Disable signup** in production:
```bash
ENABLE_SIGNUP=false
```

3. **Use HTTPS** with reverse proxy (nginx/caddy)

4. **Restrict network access**:
```yaml
services:
  ollama:
    ports: []  # Don't expose to host
```

5. **Regular backups** of volumes

## Advanced Configuration

### Using External ERPNext

Remove the `erpnext` service and set:

```bash
ERPNEXT_BASE_URL=https://your-erpnext.com
```

### Using External Ollama

Remove the `ollama` service and set:

```bash
OLLAMA_URL=http://your-ollama-server:11434
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

