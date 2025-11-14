#!/bin/bash
# Test with real sid from environment variable

if [ -z "$SID" ]; then
    echo "Error: SID environment variable not set"
    echo "Usage: export SID='your-sid-value' && ./test_with_real_sid.sh"
    exit 1
fi

echo "=========================================="
echo "Testing with Real SID Cookie"
echo "=========================================="

echo ""
echo "Sending query to MCP server..."
curl -v -b "sid=$SID" \
  -H "Content-Type: application/json" \
  -X POST http://localhost:8080/api/v1/chat \
  -d '{
    "message": "show me top 5 customers",
    "context": {
      "user_id": "test",
      "timestamp": "'$(date -u +"%Y-%m-%dT%H:%M:%S")'"
    }
  }' | jq '.'

echo ""
echo "=========================================="

