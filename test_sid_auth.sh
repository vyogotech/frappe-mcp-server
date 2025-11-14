#!/bin/bash
# Test script for sid-based authentication

echo "=========================================="
echo "Testing sid Cookie Authentication"
echo "=========================================="

# Test 1: Health check (no auth required)
echo ""
echo "Test 1: Health check..."
curl -s http://localhost:8080/health | jq '.'

# Test 2: Try to access with fake sid (should fail gracefully)
echo ""
echo "Test 2: Testing with invalid sid cookie..."
curl -s -b "sid=invalid-session-12345" \
  -H "Content-Type: application/json" \
  -X POST http://localhost:8080/api/v1/chat \
  -d '{"message": "show me top 5 customers"}' | jq '.'

echo ""
echo "=========================================="
echo "To test with real sid:"
echo "1. Log into ERPNext at http://localhost:8000"
echo "2. Open browser DevTools > Application > Cookies"
echo "3. Copy the 'sid' cookie value"
echo "4. Run: export SID='your-sid-value'"
echo "5. Run: ./test_with_real_sid.sh"
echo "=========================================="

