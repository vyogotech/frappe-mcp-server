#!/bin/bash
echo "Testing direct MCP data passing..."

# Kill any existing ollama-client instances
pkill -f ollama-client || true

echo "1. Get raw data from MCP server for PROJ-0001"
rawdata=$(echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_document","arguments":{"doctype":"Project","name":"PROJ-0001"}}}' | ./bin/frappe-mcp-server-stdio)

# Extract expected_start_date from JSON
expected_date=$(echo "$rawdata" | grep -o '"expected_start_date":"[^"]*"' | cut -d'"' -f4)
echo "Expected start date in raw data: $expected_date"

# Start ollama-client in API mode with debug enabled
echo "2. Starting Ollama client with debug mode..."
./bin/ollama-client --api --debug &
client_pid=$!
sleep 2  # Give it time to start

echo "3. Testing query about expected start date..."
curl -s -X POST http://localhost:8080/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "What is the expected start date for project PROJ-0001?", "raw": true}' | tee response.json

echo "4. Checking if response contains the correct date..."
response_date=$(cat response.json | grep -o "$expected_date" || echo "NOT FOUND")
if [[ "$response_date" == "$expected_date" ]]; then
  echo "✅ Date match found: $response_date"
else
  echo "❌ Date mismatch - Raw response doesn't contain: $expected_date"
  echo "Looking for partial date..."
  partial=$(echo "$expected_date" | cut -d'-' -f1,2)
  partial_match=$(cat response.json | grep -o "$partial" || echo "NOT FOUND")
  if [[ "$partial_match" != "NOT FOUND" ]]; then
    echo "Found partial date match: $partial_match"
  fi
fi

# Clean up
echo "5. Cleaning up..."
kill $client_pid
echo "Done testing."
