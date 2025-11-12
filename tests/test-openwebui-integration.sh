#!/bin/zsh

# Test ERPNext MCP Client + OpenWebUI Integration
# This script tests the complete integration stack

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

echo -e "${CYAN}🧪 Testing ERPNext MCP Client + OpenWebUI Integration${NC}"
echo -e "${CYAN}====================================================${NC}"
echo ""

# Service URLs
OPENWEBUI_URL="http://localhost:3000"
MCP_API_URL="http://localhost:8070/api/v1"
OLLAMA_URL="http://localhost:11434"
ERPNEXT_URL="http://localhost:8000"

# Test results tracking
TESTS_PASSED=0
TESTS_FAILED=0
TOTAL_TESTS=0

# Function to run a test
run_test() {
    local test_name="$1"
    local test_command="$2"
    local expected_status="$3"
    
    ((TOTAL_TESTS++))
    echo -e "${BLUE}🔍 Testing: $test_name${NC}"
    
    if eval "$test_command" >/dev/null 2>&1; then
        if [ "$expected_status" = "success" ]; then
            echo -e "  ${GREEN}✅ PASS${NC}"
            ((TESTS_PASSED++))
        else
            echo -e "  ${RED}❌ FAIL (expected failure but got success)${NC}"
            ((TESTS_FAILED++))
        fi
    else
        if [ "$expected_status" = "fail" ]; then
            echo -e "  ${YELLOW}✅ PASS (expected failure)${NC}"
            ((TESTS_PASSED++))
        else
            echo -e "  ${RED}❌ FAIL${NC}"
            ((TESTS_FAILED++))
        fi
    fi
}

# Function to test API endpoint with response validation
test_api_endpoint() {
    local endpoint="$1"
    local method="$2"
    local data="$3"
    local description="$4"
    
    ((TOTAL_TESTS++))
    echo -e "${BLUE}🔍 Testing API: $description${NC}"
    
    if [ "$method" = "GET" ]; then
        response=$(curl -s -w "HTTP_CODE:%{http_code}" "$endpoint")
    else
        response=$(curl -s -w "HTTP_CODE:%{http_code}" -X "$method" -H "Content-Type: application/json" -d "$data" "$endpoint")
    fi
    
    http_code=$(echo "$response" | grep -o "HTTP_CODE:[0-9]*" | cut -d: -f2)
    body=$(echo "$response" | sed 's/HTTP_CODE:[0-9]*$//')
    
    if [ "$http_code" = "200" ]; then
        echo -e "  ${GREEN}✅ PASS (HTTP $http_code)${NC}"
        ((TESTS_PASSED++))
        
        # Show sample of response
        if [ ${#body} -gt 100 ]; then
            echo -e "  ${CYAN}📋 Response sample: ${body:0:100}...${NC}"
        else
            echo -e "  ${CYAN}📋 Response: $body${NC}"
        fi
    else
        echo -e "  ${RED}❌ FAIL (HTTP $http_code)${NC}"
        echo -e "  ${RED}   Response: $body${NC}"
        ((TESTS_FAILED++))
    fi
}

# Function to check service health
check_service_health() {
    echo -e "${BLUE}🔍 Checking service health...${NC}"
    
    # Test Ollama
    run_test "Ollama Service" "curl -s $OLLAMA_URL/api/tags" "success"
    
    # Test ERPNext MCP API
    run_test "ERPNext MCP API" "curl -s $MCP_API_URL/health" "success"
    
    # Test OpenWebUI
    run_test "OpenWebUI Service" "curl -s $OPENWEBUI_URL/health" "success"
}

# Function to test ERPNext MCP API endpoints
test_mcp_api_endpoints() {
    echo -e "\n${BLUE}🔧 Testing ERPNext MCP API endpoints...${NC}"
    
    # Health check
    test_api_endpoint "$MCP_API_URL/health" "GET" "" "Health Check"
    
    # Tools list
    test_api_endpoint "$MCP_API_URL/tools" "GET" "" "Available Tools"
    
    # OpenAPI documentation
    test_api_endpoint "$MCP_API_URL/openapi.json" "GET" "" "OpenAPI Documentation"
    
    # Chat endpoint - simple query
    test_api_endpoint "$MCP_API_URL/chat" "POST" '{"message": "Hello, what can you help me with?"}' "Simple Chat Query"
    
    # Chat endpoint - business query
    test_api_endpoint "$MCP_API_URL/chat" "POST" '{"message": "What is the status of my business?"}' "Business Intelligence Query"
    
    # Chat endpoint - casual query
    test_api_endpoint "$MCP_API_URL/chat" "POST" '{"message": "Hey, what'\''s up?"}' "Casual Business Query"
}

# Function to test OpenWebUI function integration
test_openwebui_functions() {
    echo -e "\n${BLUE}🌐 Testing OpenWebUI function integration...${NC}"
    
    # Check if functions directory exists
    if [ -d "open_webui_functions" ]; then
        echo -e "  ${GREEN}✅ OpenWebUI functions directory exists${NC}"
        ((TESTS_PASSED++))
        
        # Check for ERPNext integration function
        if [ -f "open_webui_functions/erpnext_integration.py" ]; then
            echo -e "  ${GREEN}✅ ERPNext integration function exists${NC}"
            ((TESTS_PASSED++))
        else
            echo -e "  ${RED}❌ ERPNext integration function missing${NC}"
            ((TESTS_FAILED++))
        fi
    else
        echo -e "  ${RED}❌ OpenWebUI functions directory missing${NC}"
        ((TESTS_FAILED++))
    fi
    
    ((TOTAL_TESTS += 2))
    
    # Check if prompts file exists
    if [ -f "open_webui_prompts.json" ]; then
        echo -e "  ${GREEN}✅ Custom prompts file exists${NC}"
        ((TESTS_PASSED++))
        
        # Validate JSON
        if python3 -m json.tool open_webui_prompts.json >/dev/null 2>&1; then
            echo -e "  ${GREEN}✅ Prompts file is valid JSON${NC}"
            ((TESTS_PASSED++))
        else
            echo -e "  ${RED}❌ Prompts file has invalid JSON${NC}"
            ((TESTS_FAILED++))
        fi
    else
        echo -e "  ${RED}❌ Custom prompts file missing${NC}"
        ((TESTS_FAILED++))
        ((TESTS_FAILED++))
    fi
    
    ((TOTAL_TESTS += 2))
}

# Function to test Docker containers
test_docker_containers() {
    echo -e "\n${BLUE}🐳 Testing Docker containers...${NC}"
    
    # Check if containers are running
    containers=("ollama-server" "erpnext-mcp-api" "open-webui")
    
    for container in "${containers[@]}"; do
        ((TOTAL_TESTS++))
        if docker ps | grep -q "$container"; then
            echo -e "  ${GREEN}✅ Container $container is running${NC}"
            ((TESTS_PASSED++))
        else
            echo -e "  ${RED}❌ Container $container is not running${NC}"
            ((TESTS_FAILED++))
        fi
    done
}

# Function to test configuration files
test_configuration() {
    echo -e "\n${BLUE}📁 Testing configuration files...${NC}"
    
    # Check Docker Compose file
    if [ -f "docker-compose.openwebui.yml" ]; then
        echo -e "  ${GREEN}✅ Docker Compose file exists${NC}"
        ((TESTS_PASSED++))
    else
        echo -e "  ${RED}❌ Docker Compose file missing${NC}"
        ((TESTS_FAILED++))
    fi
    
    # Check environment file
    if [ -f ".env.openwebui" ]; then
        echo -e "  ${GREEN}✅ Environment file exists${NC}"
        ((TESTS_PASSED++))
    else
        echo -e "  ${YELLOW}⚠️ Environment file missing (will be created on setup)${NC}"
    fi
    
    # Check setup script
    if [ -f "setup-openwebui-integration.sh" ] && [ -x "setup-openwebui-integration.sh" ]; then
        echo -e "  ${GREEN}✅ Setup script exists and is executable${NC}"
        ((TESTS_PASSED++))
    else
        echo -e "  ${RED}❌ Setup script missing or not executable${NC}"
        ((TESTS_FAILED++))
    fi
    
    ((TOTAL_TESTS += 3))
}

# Function to demonstrate key features
demonstrate_features() {
    echo -e "\n${PURPLE}🎯 Demonstrating key integration features...${NC}"
    
    echo -e "\n${CYAN}1. Natural Language Business Queries:${NC}"
    echo -e "   Query: 'What projects need my attention?'"
    response=$(curl -s -X POST "$MCP_API_URL/chat" \
        -H "Content-Type: application/json" \
        -d '{"message": "What projects need my attention?"}' | \
        python3 -c "import json,sys; data=json.load(sys.stdin); print(data.get('response', 'No response')[:200] + '...' if len(data.get('response', '')) > 200 else data.get('response', 'No response'))" 2>/dev/null || echo "API not responding")
    echo -e "   ${GREEN}Response: $response${NC}"
    
    echo -e "\n${CYAN}2. Available ERPNext Tools:${NC}"
    tools=$(curl -s "$MCP_API_URL/tools" | \
        python3 -c "import json,sys; data=json.load(sys.stdin); print(f\"Found {data.get('count', 0)} tools: {', '.join([tool['name'] for tool in data.get('tools', [])[:5]])}{'...' if len(data.get('tools', [])) > 5 else ''}\")" 2>/dev/null || echo "API not responding")
    echo -e "   ${GREEN}$tools${NC}"
    
    echo -e "\n${CYAN}3. System Health:${NC}"
    health=$(curl -s "$MCP_API_URL/health" | \
        python3 -c "import json,sys; data=json.load(sys.stdin); print(f\"Status: {data.get('status', 'unknown')}, Model: {data.get('ollama_model', 'unknown')}, Tools: {data.get('tools_count', 0)}\")" 2>/dev/null || echo "API not responding")
    echo -e "   ${GREEN}$health${NC}"
}

# Function to show integration usage examples
show_usage_examples() {
    echo -e "\n${PURPLE}💡 Integration Usage Examples:${NC}"
    echo ""
    echo -e "${CYAN}In OpenWebUI, you can use these slash commands:${NC}"
    echo -e "  ${YELLOW}/dashboard${NC} - Executive dashboard with KPIs"
    echo -e "  ${YELLOW}/projects${NC} - Project health analysis"
    echo -e "  ${YELLOW}/finance${NC} - Financial insights"
    echo -e "  ${YELLOW}/quick${NC} - Quick business status"
    echo -e "  ${YELLOW}/help${NC} - Full help and examples"
    echo ""
    echo -e "${CYAN}Or ask natural language questions:${NC}"
    echo -e "  ${BLUE}'What projects are behind schedule?'${NC}"
    echo -e "  ${BLUE}'Show me this month's financial performance'${NC}"
    echo -e "  ${BLUE}'Are we over budget on anything?'${NC}"
    echo -e "  ${BLUE}'Hey, what's up with my business?'${NC}"
    echo ""
    echo -e "${CYAN}Direct API access:${NC}"
    echo -e "  ${BLUE}curl -X POST $MCP_API_URL/chat \\${NC}"
    echo -e "  ${BLUE}  -H 'Content-Type: application/json' \\${NC}"
    echo -e "  ${BLUE}  -d '{\"message\": \"Your business question here\"}'${NC}"
}

# Function to display final results
show_results() {
    echo -e "\n${CYAN}📊 Test Results Summary${NC}"
    echo -e "${CYAN}======================${NC}"
    echo -e "  ${GREEN}✅ Passed: $TESTS_PASSED${NC}"
    echo -e "  ${RED}❌ Failed: $TESTS_FAILED${NC}"
    echo -e "  ${BLUE}📋 Total:  $TOTAL_TESTS${NC}"
    
    if [ $TESTS_FAILED -eq 0 ]; then
        echo -e "\n${GREEN}🎉 All tests passed! Your integration is working perfectly.${NC}"
        echo -e "${GREEN}🚀 Ready for production use!${NC}"
        return 0
    else
        echo -e "\n${YELLOW}⚠️ Some tests failed. Please check the issues above.${NC}"
        
        if [ $TESTS_PASSED -gt $TESTS_FAILED ]; then
            echo -e "${YELLOW}💡 Most functionality is working. Minor issues to resolve.${NC}"
        else
            echo -e "${RED}🔧 Significant issues detected. Please review the setup.${NC}"
        fi
        return 1
    fi
}

# Function to provide troubleshooting tips
show_troubleshooting() {
    if [ $TESTS_FAILED -gt 0 ]; then
        echo -e "\n${YELLOW}🔧 Troubleshooting Tips:${NC}"
        echo -e "  ${BLUE}1.${NC} Make sure all containers are running:"
        echo -e "     ${CYAN}docker-compose -f docker-compose.openwebui.yml ps${NC}"
        echo -e "  ${BLUE}2.${NC} Check container logs for errors:"
        echo -e "     ${CYAN}docker-compose -f docker-compose.openwebui.yml logs -f${NC}"
        echo -e "  ${BLUE}3.${NC} Restart services if needed:"
        echo -e "     ${CYAN}./setup-openwebui-integration.sh restart${NC}"
        echo -e "  ${BLUE}4.${NC} Verify ERPNext is accessible if using external instance"
        echo -e "  ${BLUE}5.${NC} Check firewall settings for port access"
        echo ""
    fi
}

# Main execution function
main() {
    echo -e "${BLUE}Starting comprehensive integration test...${NC}"
    echo ""
    
    # Run all test suites
    check_service_health
    test_configuration
    test_docker_containers
    test_mcp_api_endpoints
    test_openwebui_functions
    
    # Demonstrate features
    demonstrate_features
    
    # Show usage examples
    show_usage_examples
    
    # Display results
    show_results
    
    # Show troubleshooting if needed
    show_troubleshooting
    
    echo -e "\n${CYAN}🌐 Access your integration:${NC}"
    echo -e "  ${PURPLE}OpenWebUI:${NC} http://localhost:3000"
    echo -e "  ${PURPLE}API Docs:${NC} http://localhost:8080/api/v1/openapi.json"
    echo -e "  ${PURPLE}Health Check:${NC} http://localhost:8080/api/v1/health"
}

# Handle command line options
case "${1:-}" in
    "quick")
        echo -e "${YELLOW}Running quick health check...${NC}"
        check_service_health
        show_results
        ;;
    "api")
        echo -e "${YELLOW}Testing API endpoints only...${NC}"
        test_mcp_api_endpoints
        show_results
        ;;
    "demo")
        echo -e "${YELLOW}Running feature demonstration...${NC}"
        demonstrate_features
        show_usage_examples
        ;;
    "help"|"-h"|"--help")
        echo "ERPNext MCP + OpenWebUI Integration Test Suite"
        echo ""
        echo "Usage: $0 [option]"
        echo ""
        echo "Options:"
        echo "  (no args)  Run complete test suite"
        echo "  quick      Quick health check only"
        echo "  api        Test API endpoints only"
        echo "  demo       Demonstrate key features"
        echo "  help       Show this help"
        ;;
    "")
        main
        ;;
    *)
        echo -e "${RED}Unknown option: $1${NC}"
        echo "Use '$0 help' for usage information"
        exit 1
        ;;
esac
