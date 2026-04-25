#!/bin/bash
# Account Manager Test Queries for Frappe MCP Server
# Tests various natural language patterns an account manager would use

BASE_URL="http://localhost:8080/api/v1/chat"

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to test a query
test_query() {
    local query="$1"
    local category="$2"
    
    echo -e "\n${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${YELLOW}Category: ${category}${NC}"
    echo -e "${GREEN}Query: ${query}${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    
    response=$(curl -s -X POST "$BASE_URL" \
        -H "Content-Type: application/json" \
        -d "{\"message\": \"$query\"}")
    
    # Extract key fields
    data_quality=$(echo "$response" | jq -r '.data_quality')
    tools_called=$(echo "$response" | jq -r '.tools_called | join(", ")')
    response_text=$(echo "$response" | jq -r '.response')
    
    echo -e "${YELLOW}Tools Called:${NC} $tools_called"
    echo -e "${YELLOW}Data Quality:${NC} $data_quality"
    echo -e "${YELLOW}Response:${NC}\n$response_text"
    
    # Show raw JSON for debugging
    if [ "$DEBUG" = "1" ]; then
        echo -e "\n${YELLOW}Raw JSON:${NC}"
        echo "$response" | jq '.'
    fi
    
    sleep 1 # Avoid rate limits
}

echo -e "${GREEN}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║  Account Manager NLP Test Suite for Frappe MCP Server         ║${NC}"
echo -e "${GREEN}║  Testing natural language queries typical for account managers ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════════════════════╝${NC}"

# ============================================================================
# CATEGORY 1: CUSTOMER QUERIES
# ============================================================================
echo -e "\n${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}  CATEGORY 1: Customer Information Queries${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"

test_query "Show me all my customers" "Customer Listing"
test_query "Give me a list of customers" "Customer Listing"
test_query "Who are my active customers?" "Customer Listing (Filtered)"
test_query "Show customer details for Acme Corp" "Specific Customer"

# ============================================================================
# CATEGORY 2: REVENUE & SALES QUERIES
# ============================================================================
echo -e "\n${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}  CATEGORY 2: Revenue & Sales Analysis${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"

test_query "What's my total revenue this month?" "Revenue Aggregation"
test_query "Show me top 5 customers by revenue" "Top Customers"
test_query "Which customers generated the most revenue?" "Revenue Analysis"
test_query "Get me the sales report" "Sales Report"

# ============================================================================
# CATEGORY 3: INVOICE QUERIES
# ============================================================================
echo -e "\n${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}  CATEGORY 3: Invoice Management${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"

test_query "Show all outstanding invoices" "Outstanding Invoices"
test_query "List unpaid invoices" "Unpaid Invoices"
test_query "What invoices are overdue?" "Overdue Invoices"
test_query "Show recent sales invoices" "Recent Invoices"

# ============================================================================
# CATEGORY 4: ACCOUNT HEALTH QUERIES
# ============================================================================
echo -e "\n${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}  CATEGORY 4: Account Health & Status${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"

test_query "Show me accounts receivable report" "AR Report"
test_query "Which customers have the highest outstanding balance?" "Outstanding Analysis"
test_query "Get customer ledger summary" "Ledger Report"
test_query "Show me aging report" "Aging Report"

# ============================================================================
# CATEGORY 5: CONTEXTUAL QUERIES
# ============================================================================
echo -e "\n${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}  CATEGORY 5: Contextual & Configuration${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"

test_query "What's the default currency?" "Default Currency"
test_query "Give me the current company details" "Company Info"
test_query "Show me the company information" "Company Info"
test_query "What's my company name?" "Company Name"

# ============================================================================
# CATEGORY 6: CONTACT & COMMUNICATION
# ============================================================================
echo -e "\n${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}  CATEGORY 6: Contact Information${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"

test_query "Show me contact details for customer XYZ" "Customer Contact"
test_query "List all customer emails" "Customer Emails"
test_query "Who is my primary contact at Acme Corp?" "Primary Contact"

# ============================================================================
# CATEGORY 7: TERRITORY & SEGMENTS
# ============================================================================
echo -e "\n${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}  CATEGORY 7: Territory & Customer Segmentation${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"

test_query "Show customers in North America" "Territory Filter"
test_query "List all Enterprise customers" "Customer Group"
test_query "How many customers do I have by territory?" "Territory Count"

# ============================================================================
# CATEGORY 8: OPPORTUNITIES & PIPELINE
# ============================================================================
echo -e "\n${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}  CATEGORY 8: Sales Opportunities${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"

test_query "Show open opportunities" "Open Opportunities"
test_query "What deals are in my pipeline?" "Pipeline"
test_query "List all quotations" "Quotations"
test_query "Show me pending sales orders" "Pending Orders"

# ============================================================================
# CATEGORY 9: TIME-BASED QUERIES
# ============================================================================
echo -e "\n${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}  CATEGORY 9: Time-Based Analysis${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"

test_query "What were my sales last quarter?" "Quarterly Sales"
test_query "Show this year's revenue" "Yearly Revenue"
test_query "Compare this month vs last month sales" "Month Comparison"

# ============================================================================
# CATEGORY 10: FINANCIAL REPORTS
# ============================================================================
echo -e "\n${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}  CATEGORY 10: Financial Reports${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"

test_query "Get profit and loss report" "P&L Report"
test_query "Show me the balance sheet" "Balance Sheet"
test_query "Run sales analytics report" "Sales Analytics"
test_query "Generate customer ledger report" "Customer Ledger"

# ============================================================================
# SUMMARY
# ============================================================================
echo -e "\n${GREEN}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║  Test Suite Complete!                                          ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════════════════════╝${NC}"

echo -e "\n${YELLOW}Tips for analyzing results:${NC}"
echo -e "1. Check 'Tools Called' - should match the query intent"
echo -e "2. 'Data Quality' - high/medium is good, low/error needs attention"
echo -e "3. Response should be formatted nicely (not raw JSON)"
echo -e "4. Set DEBUG=1 to see full JSON responses: ${GREEN}DEBUG=1 ./test_account_manager.sh${NC}"
echo -e "\n${YELLOW}To test specific category only:${NC}"
echo -e "Edit the script and comment out unwanted categories"

