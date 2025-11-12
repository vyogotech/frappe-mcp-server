#!/bin/bash

# ERPNext MCP Server + Ollama Integration - Test Script
echo "🧪 ERPNext MCP + Ollama Integration Tests"
echo "========================================="
echo

# Function to test HTTP endpoint
test_endpoint() {
    local url=$1
    local name=$2
    local expected_status=${3:-200}
    
    echo -n "Testing $name... "
    
    status=$(curl -s -o /dev/null -w "%{http_code}" "$url" 2>/dev/null)
    
    if [ "$status" = "$expected_status" ]; then
        echo "✅ OK ($status)"
        return 0
    else
        echo "❌ FAIL ($status)"
        return 1
    fi
}

# Check if services are running
echo "📊 Checking Docker services..."
if ! docker-compose ps | grep -q "Up"; then
    echo "❌ Services are not running. Please run ./docker-setup.sh first"
    exit 1
fi

echo "✅ Docker services are running"
echo

# Test individual services
echo "🔍 Testing Individual Services:"
echo "==============================="

test_endpoint "http://localhost:8000" "ERPNext Web Interface"
test_endpoint "http://localhost:8081/health" "MCP Server Health"
test_endpoint "http://localhost:11434/api/tags" "Ollama API"

echo

# Test MCP Server tools
echo "🔧 Testing MCP Server Tools:"
echo "============================="

echo -n "Testing MCP tools list... "
tools_response=$(curl -s -X POST http://localhost:8081/tools 2>/dev/null)
if echo "$tools_response" | grep -q "get_document"; then
    echo "✅ OK (tools available)"
else
    echo "❌ FAIL (no tools found)"
fi

echo -n "Testing MCP portfolio tool... "
portfolio_response=$(curl -s -X POST http://localhost:8081/tool/portfolio_dashboard \
    -H "Content-Type: application/json" \
    -d '{}' 2>/dev/null)
    
if [ $? -eq 0 ]; then
    echo "✅ OK (tool accessible)"
else
    echo "❌ FAIL (tool not accessible)"
fi

echo

# Test Ollama models
echo "🦙 Testing Ollama Models:"
echo "========================="

echo -n "Checking available models... "
models_response=$(docker exec $(docker-compose ps -q ollama) ollama list 2>/dev/null)
if echo "$models_response" | grep -q "llama3.1"; then
    echo "✅ OK (llama3.1 available)"
else
    echo "⚠️  WARNING (no models found - pulling llama3.1...)"
    docker exec $(docker-compose ps -q ollama) ollama pull llama3.1
fi

echo

# Test MCP + Ollama integration
echo "🤝 Testing MCP + Ollama Integration:"
echo "==================================="

echo "Starting interactive Ollama client test..."
echo "This will test the complete integration pipeline."
echo

# Test if we can start the ollama client
echo "Testing Ollama client startup..."
timeout 10s docker-compose run --rm ollama-mcp-client echo "Ollama client startup test" 2>/dev/null
if [ $? -eq 0 ]; then
    echo "✅ Ollama client can start successfully"
else
    echo "⚠️  Ollama client startup issues (check logs)"
fi

echo

# Performance tests
echo "📈 Performance Tests:"
echo "===================="

echo -n "MCP Server response time... "
start_time=$(date +%s%3N)
curl -s http://localhost:8081/health >/dev/null
end_time=$(date +%s%3N)
response_time=$((end_time - start_time))
echo "${response_time}ms"

echo -n "Ollama API response time... "
start_time=$(date +%s%3N)
curl -s http://localhost:11434/api/tags >/dev/null
end_time=$(date +%s%3N)
response_time=$((end_time - start_time))
echo "${response_time}ms"

echo

# Integration summary
echo "📋 Integration Test Summary:"
echo "============================"

# Check logs for errors
echo "Checking for errors in logs..."
error_count=$(docker-compose logs --tail=100 2>/dev/null | grep -i error | wc -l)
warning_count=$(docker-compose logs --tail=100 2>/dev/null | grep -i warning | wc -l)

echo "  • Errors in logs: $error_count"
echo "  • Warnings in logs: $warning_count"

# Resource usage
echo "Resource usage:"
echo "  • Memory: $(docker stats --no-stream --format "table {{.Container}}\t{{.MemUsage}}" | grep -E "(erpnext|ollama|mcp)" | head -5)"

echo

# Final recommendations
echo "🎯 Test Results & Recommendations:"
echo "=================================="

if test_endpoint "http://localhost:8000" "ERPNext" >/dev/null 2>&1 && \
   test_endpoint "http://localhost:8081/health" "MCP Server" >/dev/null 2>&1 && \
   test_endpoint "http://localhost:11434/api/tags" "Ollama" >/dev/null 2>&1; then
    
    echo "✅ ALL CORE SERVICES WORKING"
    echo
    echo "🚀 Ready for AI conversations!"
    echo "  • Run: docker-compose run --rm ollama-mcp-client"
    echo "  • Ask: 'Show me my ERPNext portfolio dashboard'"
    echo
else
    echo "⚠️  SOME SERVICES HAVE ISSUES"
    echo
    echo "🛠  Troubleshooting steps:"
    echo "  1. Check logs: docker-compose logs"
    echo "  2. Restart services: docker-compose restart"
    echo "  3. Verify .env configuration"
    echo
fi

echo "📚 Useful commands:"
echo "  • View logs: docker-compose logs -f [service_name]"
echo "  • Restart: docker-compose restart [service_name]"
echo "  • Interactive Ollama: docker-compose run --rm ollama-mcp-client"
echo "  • ERPNext shell: docker-compose exec erpnext bash"
echo
echo "Integration testing complete! 🎉"
