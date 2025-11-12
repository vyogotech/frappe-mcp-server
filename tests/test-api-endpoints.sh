#!/bin/bash

# Test script for ERPNext Ollama MCP API endpoints

API_BASE="http://localhost:8070/api/v1"

echo "🧪 Testing ERPNext Ollama MCP API Endpoints"
echo "============================================"

# Function to test endpoint
test_endpoint() {
    local method=$1
    local endpoint=$2
    local data=$3
    local description=$4
    
    echo ""
    echo "📋 Testing: $description"
    echo "🔗 $method $endpoint"
    
    if [ "$method" = "GET" ]; then
        response=$(curl -s -w "HTTP_CODE:%{http_code}" "$API_BASE$endpoint")
    else
        response=$(curl -s -w "HTTP_CODE:%{http_code}" -X "$method" -H "Content-Type: application/json" -d "$data" "$API_BASE$endpoint")
    fi
    
    http_code=$(echo "$response" | grep -o "HTTP_CODE:[0-9]*" | cut -d: -f2)
    body=$(echo "$response" | sed 's/HTTP_CODE:[0-9]*$//')
    
    if [ "$http_code" = "200" ]; then
        echo "✅ Status: $http_code"
        echo "$body" | python3 -m json.tool 2>/dev/null || echo "$body"
    else
        echo "❌ Status: $http_code"
        echo "$body"
    fi
}

# Wait for API server to be ready
echo "⏳ Waiting for API server to be ready..."
for i in {1..30}; do
    if curl -s "$API_BASE/health" > /dev/null 2>&1; then
        echo "✅ API server is ready!"
        break
    fi
    sleep 1
    echo -n "."
done

# Test 1: Health Check
test_endpoint "GET" "/health" "" "Health Check"

# Test 2: List Tools
test_endpoint "GET" "/tools" "" "List Available Tools"

# Test 3: OpenAPI Documentation
test_endpoint "GET" "/openapi.json" "" "OpenAPI Documentation"

# Test 4: Chat - Simple Query
chat_data='{"message": "Hello, what can you help me with?"}'
test_endpoint "POST" "/chat" "$chat_data" "Simple Chat Query"

# Test 5: Chat - Business Query
business_data='{"message": "What projects are behind schedule?"}'
test_endpoint "POST" "/chat" "$business_data" "Business Intelligence Query"

# Test 6: Chat - Casual Query
casual_data='{"message": "Hey, what'\''s up with my business?"}'
test_endpoint "POST" "/chat" "$casual_data" "Casual Business Query"

# Test 7: Tool Execution (if tools are available)
echo ""
echo "🔧 Testing Tool Execution (if tools are available)..."
tools_response=$(curl -s "$API_BASE/tools")
if echo "$tools_response" | grep -q '"tools"'; then
    echo "📋 Tools available, testing tool execution..."
    tool_data='{"arguments": {}}'
    test_endpoint "POST" "/tools/list_projects" "$tool_data" "Execute list_projects tool"
else
    echo "⚠️  No tools available for testing"
fi

echo ""
echo "🎉 API endpoint testing completed!"
echo "📊 Check the results above for any issues."
