#!/bin/bash

echo "🧪 Testing ERPNext MCP Data Display"
echo "===================================="

cd /Users/varkrish/personal/frappista_sne_apps/frappe-mcp-server

# Test 1: Direct HTTP API call to see raw data
echo ""
echo "📡 Raw MCP Server Response:"
echo "----------------------------"
curl -s -X POST http://localhost:8081/tool/get_document \
  -H "Content-Type: application/json" \
  -d '{"id": "test-1", "params": {"doctype": "Project", "name": "PROJ-0002"}}' | \
  jq '.content[1].text | fromjson | {name, project_name, status, priority, expected_start_date, expected_end_date, estimated_costing}' 2>/dev/null || \
  echo "❌ Could not parse response"

echo ""
echo "🎯 Testing Ollama Client Tool Call:"
echo "------------------------------------"

# Test 2: Ollama client with improved data display
export OLLAMA_HOST=http://localhost:11434

# Create a simple test input
echo "get Project PROJ-0002" | timeout 20s ./bin/ollama-client --model llama3.1:latest 2>/dev/null | head -50

echo ""
echo "✅ Test completed. You should now see:"
echo "   1. The raw project data from ERPNext"
echo "   2. The AI's interpretation of that data"
echo "   3. Proper JSON formatting in the output"
