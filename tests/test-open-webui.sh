#!/bin/bash

# Open WebUI Integration Test Script
# Tests the complete stack with Open WebUI

set -e

echo "🧪 Testing Open WebUI Integration"
echo "================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Detect container runtime
if command -v docker-compose >/dev/null 2>&1; then
    COMPOSE_CMD="docker-compose"
    echo -e "${BLUE}🐳 Using Docker Compose${NC}"
elif command -v podman-compose >/dev/null 2>&1; then
    COMPOSE_CMD="podman-compose"
    echo -e "${BLUE}🦭 Using Podman Compose${NC}"
else
    echo -e "${RED}❌ Neither docker-compose nor podman-compose found${NC}"
    exit 1
fi

# Function to check service health
check_service() {
    local service=$1
    local url=$2
    local max_attempts=20
    local attempt=1
    
    echo -e "${BLUE}🔍 Checking $service health...${NC}"
    
    while [ $attempt -le $max_attempts ]; do
        if curl -f "$url" >/dev/null 2>&1; then
            echo -e "${GREEN}✅ $service is healthy${NC}"
            return 0
        fi
        
        echo -e "${YELLOW}⏳ Waiting for $service (attempt $attempt/$max_attempts)...${NC}"
        sleep 5
        attempt=$((attempt + 1))
    done
    
    echo -e "${RED}❌ $service failed to start after $max_attempts attempts${NC}"
    return 1
}

# Cleanup function
cleanup() {
    echo -e "${YELLOW}🧹 Cleaning up test environment...${NC}"
    $COMPOSE_CMD down -v 2>/dev/null || true
    echo -e "${GREEN}✅ Cleanup complete${NC}"
}

# Set trap for cleanup on exit
trap cleanup EXIT

# Start all services
echo -e "${BLUE}🚀 Starting all services (ERPNext, Ollama, Open WebUI, MCP Server)...${NC}"
$COMPOSE_CMD up -d erpnext ollama open-webui frappe-mcp-server

# Wait for services to be healthy
echo -e "${BLUE}⏳ Waiting for services to start...${NC}"

# Check ERPNext
if check_service "ERPNext" "http://localhost:8000"; then
    echo -e "${GREEN}✅ ERPNext is ready at http://localhost:8000${NC}"
else
    echo -e "${RED}❌ ERPNext failed to start${NC}"
    $COMPOSE_CMD logs erpnext
    exit 1
fi

# Check Ollama
if check_service "Ollama" "http://localhost:11434/api/tags"; then
    echo -e "${GREEN}✅ Ollama is ready at http://localhost:11434${NC}"
else
    echo -e "${RED}❌ Ollama failed to start${NC}"
    $COMPOSE_CMD logs ollama
    exit 1
fi

# Check Open WebUI
if check_service "Open WebUI" "http://localhost:3000"; then
    echo -e "${GREEN}✅ Open WebUI is ready at http://localhost:3000${NC}"
else
    echo -e "${RED}❌ Open WebUI failed to start${NC}"
    $COMPOSE_CMD logs open-webui
    exit 1
fi

# Check MCP Server
if check_service "MCP Server" "http://localhost:8081/health"; then
    echo -e "${GREEN}✅ MCP Server is ready at http://localhost:8081${NC}"
else
    echo -e "${RED}❌ MCP Server failed to start${NC}"
    $COMPOSE_CMD logs frappe-mcp-server
    exit 1
fi

# Test service integrations
echo -e "${BLUE}🔗 Testing service integrations...${NC}"

# Test Ollama API
echo -e "${BLUE}🤖 Testing Ollama API...${NC}"
if curl -s http://localhost:11434/api/tags | grep -q "models"; then
    echo -e "${GREEN}✅ Ollama API is responding${NC}"
else
    echo -e "${YELLOW}⚠️  Ollama API response unclear (may need models)${NC}"
fi

# Test MCP Server tools
echo -e "${BLUE}🛠️  Testing MCP Server tools...${NC}"
if curl -s http://localhost:8081/tools | grep -q "name"; then
    echo -e "${GREEN}✅ MCP Server tools are available${NC}"
    tool_count=$(curl -s http://localhost:8081/tools | jq length 2>/dev/null || echo "unknown")
    echo -e "${BLUE}📊 Available tools: $tool_count${NC}"
else
    echo -e "${YELLOW}⚠️  MCP Server tools response unclear${NC}"
fi

# Test Open WebUI health
echo -e "${BLUE}🌐 Testing Open WebUI health...${NC}"
webui_response=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:3000 || echo "000")
if [ "$webui_response" = "200" ]; then
    echo -e "${GREEN}✅ Open WebUI is serving content${NC}"
else
    echo -e "${YELLOW}⚠️  Open WebUI returned HTTP $webui_response${NC}"
fi

# Display service status
echo -e "${BLUE}📊 Service Status Summary${NC}"
echo "================================="
$COMPOSE_CMD ps

# Success summary
echo ""
echo -e "${GREEN}🎉 Integration Test Complete!${NC}"
echo ""
echo -e "${BLUE}📋 Service URLs:${NC}"
echo -e "   • ERPNext:    ${GREEN}http://localhost:8000${NC} (admin/admin)"
echo -e "   • Open WebUI: ${GREEN}http://localhost:3000${NC} (Create account to start)"
echo -e "   • MCP Server: ${GREEN}http://localhost:8081${NC}"
echo -e "   • Ollama:     ${GREEN}http://localhost:11434${NC}"
echo ""
echo -e "${BLUE}🔧 Next Steps:${NC}"
echo "1. 🌐 Open http://localhost:3000 in your browser"
echo "2. 👤 Create an account in Open WebUI"
echo "3. 🤖 Select a model (you may need to pull models first)"
echo "4. 💬 Start chatting with your ERPNext data!"
echo ""
echo -e "${BLUE}📚 Documentation:${NC}"
echo "   • Setup Guide: ./OPEN_WEBUI_COMPLETE.md"
echo "   • Integration: ./DEPLOYMENT_SIMPLIFIED.md"
echo ""
echo -e "${YELLOW}⚠️  Note: Services will continue running for testing.${NC}"
echo -e "${YELLOW}    Run '$COMPOSE_CMD down' to stop all services.${NC}"

# Prevent automatic cleanup
trap - EXIT
