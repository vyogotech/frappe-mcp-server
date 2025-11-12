#!/bin/bash

# ERPNext MCP Server API Test Script
echo "🧪 Testing ERPNext MCP Server API"
echo "================================="
echo

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Server URL
SERVER_URL="http://localhost:8080"

# Function to test endpoint
test_endpoint() {
    local method=$1
    local endpoint=$2
    local data=$3
    local description=$4
    
    echo -n "Testing $description... "
    
    if [ "$method" == "GET" ]; then
        response=$(curl -s -w "\n%{http_code}" "$SERVER_URL$endpoint" 2>/dev/null)
    else
        response=$(curl -s -w "\n%{http_code}" -X "$method" "$SERVER_URL$endpoint" \
            -H "Content-Type: application/json" \
            -d "$data" 2>/dev/null)
    fi
    
    # Get status code (last line)
    status_code=$(echo "$response" | tail -n1)
    # Get body (all but last line)
    body=$(echo "$response" | sed '$d')
    
    if [ "$status_code" = "200" ]; then
        echo -e "${GREEN}✓ PASS${NC} (Status: $status_code)"
        return 0
    else
        echo -e "${RED}✗ FAIL${NC} (Status: $status_code)"
        echo "  Response: $body"
        return 1
    fi
}

# Check if server is running
echo "Checking if server is running..."
if ! curl -s "$SERVER_URL/api/v1/health" > /dev/null 2>&1; then
    echo -e "${RED}✗ Server is not running at $SERVER_URL${NC}"
    echo ""
    echo "Please start the server first:"
    echo "  ./bin/frappe-mcp-server"
    echo ""
    exit 1
fi
echo -e "${GREEN}✓ Server is running${NC}"
echo ""

# Test suite
echo "Running API Tests:"
echo "=================="

passed=0
failed=0

# Test 1: Health check (new endpoint)
if test_endpoint "GET" "/api/v1/health" "" "Health Check (new /api/v1/health)"; then
    ((passed++))
else
    ((failed++))
fi

# Test 2: Legacy health check
if test_endpoint "GET" "/health" "" "Health Check (legacy /health)"; then
    ((passed++))
else
    ((failed++))
fi

# Test 3: List tools
if test_endpoint "GET" "/api/v1/tools" "" "List Tools"; then
    ((passed++))
else
    ((failed++))
fi

# Test 4: Chat endpoint - Portfolio Dashboard
if test_endpoint "POST" "/api/v1/chat" '{"message":"Show me my portfolio dashboard"}' "Chat Query - Portfolio"; then
    ((passed++))
else
    ((failed++))
fi

# Test 5: Chat endpoint - List Projects
if test_endpoint "POST" "/api/v1/chat" '{"message":"List all projects"}' "Chat Query - List Projects"; then
    ((passed++))
else
    ((failed++))
fi

# Test 6: Direct tool call - Portfolio Dashboard
if test_endpoint "POST" "/api/v1/tools/portfolio_dashboard" '{"params":{}}' "Direct Tool - Portfolio Dashboard"; then
    ((passed++))
else
    ((failed++))
fi

# Test 7: Direct tool call - List Documents
if test_endpoint "POST" "/api/v1/tools/list_documents" '{"params":{"doctype":"Project","page_size":10}}' "Direct Tool - List Documents"; then
    ((passed++))
else
    ((failed++))
fi

# Test 8: OpenAPI spec
if test_endpoint "GET" "/api/v1/openapi.json" "" "OpenAPI Specification"; then
    ((passed++))
else
    ((failed++))
fi

echo ""
echo "Test Summary:"
echo "============="
echo -e "Passed: ${GREEN}$passed${NC}"
echo -e "Failed: ${RED}$failed${NC}"
echo ""

if [ $failed -eq 0 ]; then
    echo -e "${GREEN}🎉 All tests passed!${NC}"
    echo ""
    echo "API is working correctly. You can now:"
    echo "  • Use the /api/v1/chat endpoint for natural language queries"
    echo "  • Call tools directly via /api/v1/tools/{tool_name}"
    echo "  • Integrate with Open WebUI using open_webui_functions/erpnext_integration.py"
    echo ""
    exit 0
else
    echo -e "${YELLOW}⚠️  Some tests failed. Check the errors above.${NC}"
    echo ""
    echo "Common issues:"
    echo "  • ERPNext instance not accessible"
    echo "  • Invalid API credentials in config.yaml"
    echo "  • ERPNext has no data yet"
    echo ""
    exit 1
fi





