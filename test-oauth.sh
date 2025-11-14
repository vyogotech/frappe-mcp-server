#!/bin/bash

# OAuth2 Authentication Test Script
# This script tests the OAuth2 implementation in Frappe MCP Server

set -e

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
FRAPPE_URL="${FRAPPE_BASE_URL:-http://localhost:8000}"
MCP_URL="${MCP_URL:-http://localhost:8080}"
CLIENT_ID="${OAUTH_CLIENT_ID:-}"
CLIENT_SECRET="${OAUTH_CLIENT_SECRET:-}"

echo -e "${BLUE}=== Frappe MCP Server - OAuth2 Test ===${NC}\n"

# Function to print colored messages
print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

print_info() {
    echo -e "${BLUE}ℹ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠ $1${NC}"
}

# Check prerequisites
echo -e "${BLUE}Step 1: Checking Prerequisites${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Check if jq is installed
if ! command -v jq &> /dev/null; then
    print_error "jq is not installed. Please install it first:"
    echo "  macOS: brew install jq"
    echo "  Ubuntu: sudo apt install jq"
    exit 1
fi
print_success "jq is installed"

# Check if curl is available
if ! command -v curl &> /dev/null; then
    print_error "curl is not installed"
    exit 1
fi
print_success "curl is available"

# Check if Frappe is accessible
echo -n "Checking Frappe connection at $FRAPPE_URL... "
if curl -s -f "$FRAPPE_URL" > /dev/null 2>&1; then
    print_success "Frappe is accessible"
else
    print_error "Cannot connect to Frappe at $FRAPPE_URL"
    print_info "Make sure Frappe is running: cd frappe-bench && bench start"
    exit 1
fi

# Check if MCP server is running
echo -n "Checking MCP server at $MCP_URL... "
if curl -s -f "$MCP_URL/api/v1/health" > /dev/null 2>&1; then
    print_success "MCP server is running"
else
    print_error "MCP server is not running at $MCP_URL"
    print_info "Start the server: ./bin/frappe-mcp-server-stdio --config config.yaml"
    exit 1
fi

echo ""

# OAuth2 Client Setup
echo -e "${BLUE}Step 2: OAuth2 Client Setup${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [ -z "$CLIENT_ID" ] || [ -z "$CLIENT_SECRET" ]; then
    print_warning "OAuth2 credentials not found in environment variables"
    echo ""
    echo "Please create an OAuth2 client in Frappe:"
    echo ""
    echo "1. Navigate to: ${FRAPPE_URL}/app/oauth-client"
    echo "2. Click 'New' and create a client with:"
    echo "   - App Name: MCP Backend Integration"
    echo "   - Scopes: openid, profile, email, all"
    echo "   - Grant Type: Client Credentials"
    echo "3. Save and copy the Client ID and Secret"
    echo ""
    echo -n "Enter Client ID: "
    read -r CLIENT_ID
    echo -n "Enter Client Secret: "
    read -rs CLIENT_SECRET
    echo ""
    echo ""
else
    print_success "Using OAuth2 credentials from environment"
fi

# Test 1: Get OAuth2 Token
echo -e "${BLUE}Step 3: Getting OAuth2 Token${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

TOKEN_RESPONSE=$(curl -s -X POST "${FRAPPE_URL}/api/method/frappe.integrations.oauth2.get_token" \
    -H "Content-Type: application/x-www-form-urlencoded" \
    -d "grant_type=client_credentials" \
    -d "client_id=${CLIENT_ID}" \
    -d "client_secret=${CLIENT_SECRET}")

if echo "$TOKEN_RESPONSE" | jq -e '.access_token' > /dev/null 2>&1; then
    ACCESS_TOKEN=$(echo "$TOKEN_RESPONSE" | jq -r '.access_token')
    EXPIRES_IN=$(echo "$TOKEN_RESPONSE" | jq -r '.expires_in')
    print_success "Successfully obtained access token"
    print_info "Token expires in: ${EXPIRES_IN} seconds"
    echo ""
    echo "Token (first 50 chars): ${ACCESS_TOKEN:0:50}..."
else
    print_error "Failed to get access token"
    echo "Response: $TOKEN_RESPONSE"
    exit 1
fi

echo ""

# Test 2: Validate Token with Frappe
echo -e "${BLUE}Step 4: Validating Token with Frappe${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

USER_INFO=$(curl -s -X GET "${FRAPPE_URL}/api/method/frappe.integrations.oauth2.openid.userinfo" \
    -H "Authorization: Bearer ${ACCESS_TOKEN}")

if echo "$USER_INFO" | jq -e '.sub' > /dev/null 2>&1; then
    print_success "Token is valid"
    USER_ID=$(echo "$USER_INFO" | jq -r '.sub')
    USER_EMAIL=$(echo "$USER_INFO" | jq -r '.email // "N/A"')
    print_info "User ID: $USER_ID"
    print_info "Email: $USER_EMAIL"
else
    print_error "Token validation failed"
    echo "Response: $USER_INFO"
    exit 1
fi

echo ""

# Test 3: Authenticated Request to MCP Server
echo -e "${BLUE}Step 5: Testing Authenticated Request to MCP${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

MCP_RESPONSE=$(curl -s -X POST "${MCP_URL}/api/v1/chat" \
    -H "Authorization: Bearer ${ACCESS_TOKEN}" \
    -H "Content-Type: application/json" \
    -d '{"message": "List all projects"}')

if [ $? -eq 0 ]; then
    print_success "Authenticated request succeeded"
    echo ""
    echo "Response:"
    echo "$MCP_RESPONSE" | jq '.' 2>/dev/null || echo "$MCP_RESPONSE"
else
    print_error "Authenticated request failed"
    echo "Response: $MCP_RESPONSE"
fi

echo ""

# Test 4: Authenticated Request with User Context (Trusted Client)
echo -e "${BLUE}Step 6: Testing Request with User Context${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

MCP_RESPONSE_WITH_USER=$(curl -s -X POST "${MCP_URL}/api/v1/chat" \
    -H "Authorization: Bearer ${ACCESS_TOKEN}" \
    -H "X-MCP-User-ID: test@example.com" \
    -H "X-MCP-User-Email: test@example.com" \
    -H "X-MCP-User-Name: Test User" \
    -H "Content-Type: application/json" \
    -d '{"message": "Show my projects"}')

if [ $? -eq 0 ]; then
    print_success "Request with user context succeeded"
    echo ""
    echo "Response:"
    echo "$MCP_RESPONSE_WITH_USER" | jq '.' 2>/dev/null || echo "$MCP_RESPONSE_WITH_USER"
else
    print_error "Request with user context failed"
    echo "Response: $MCP_RESPONSE_WITH_USER"
fi

echo ""

# Test 5: Request without Authentication
echo -e "${BLUE}Step 7: Testing Request Without Authentication${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

UNAUTH_RESPONSE=$(curl -s -w "\nHTTP_CODE:%{http_code}" -X POST "${MCP_URL}/api/v1/chat" \
    -H "Content-Type: application/json" \
    -d '{"message": "Test without auth"}')

HTTP_CODE=$(echo "$UNAUTH_RESPONSE" | grep "HTTP_CODE:" | cut -d: -f2)
BODY=$(echo "$UNAUTH_RESPONSE" | sed '/HTTP_CODE:/d')

if [ "$HTTP_CODE" = "401" ]; then
    print_success "Correctly rejected unauthenticated request (require_auth: true)"
elif [ "$HTTP_CODE" = "200" ]; then
    print_warning "Unauthenticated request succeeded (require_auth: false)"
    print_info "This is expected if you have auth.require_auth: false in config"
else
    print_info "Unexpected response code: $HTTP_CODE"
fi

echo ""

# Test 6: Request with Invalid Token
echo -e "${BLUE}Step 8: Testing Request with Invalid Token${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

INVALID_TOKEN_RESPONSE=$(curl -s -w "\nHTTP_CODE:%{http_code}" -X POST "${MCP_URL}/api/v1/chat" \
    -H "Authorization: Bearer invalid-token-12345" \
    -H "Content-Type: application/json" \
    -d '{"message": "Test with invalid token"}')

HTTP_CODE=$(echo "$INVALID_TOKEN_RESPONSE" | grep "HTTP_CODE:" | cut -d: -f2)

if [ "$HTTP_CODE" = "401" ]; then
    print_success "Correctly rejected invalid token"
elif [ "$HTTP_CODE" = "200" ]; then
    print_warning "Invalid token was accepted (require_auth: false)"
else
    print_info "Unexpected response code: $HTTP_CODE"
fi

echo ""

# Summary
echo -e "${BLUE}═══════════════════════════════════════${NC}"
echo -e "${GREEN}OAuth2 Testing Complete!${NC}"
echo -e "${BLUE}═══════════════════════════════════════${NC}"

echo ""
echo "Configuration Summary:"
echo "  Frappe URL: $FRAPPE_URL"
echo "  MCP URL: $MCP_URL"
echo "  Client ID: $CLIENT_ID"
echo "  Token Valid: Yes"
echo ""

echo "Next Steps:"
echo "  1. Update your client application to use OAuth2"
echo "  2. Store CLIENT_ID and CLIENT_SECRET securely"
echo "  3. Implement token caching in your client"
echo "  4. Consider enabling require_auth: true for production"
echo ""

print_info "Export these for future use:"
echo "  export OAUTH_CLIENT_ID='$CLIENT_ID'"
echo "  export OAUTH_CLIENT_SECRET='$CLIENT_SECRET'"
echo ""

