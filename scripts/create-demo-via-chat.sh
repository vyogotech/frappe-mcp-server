#!/bin/bash
# Create Demo Data via MCP Server Chat API
# This uses the conversational interface to create test data

API_URL="http://localhost:8080/api/v1/chat"

echo "================================================"
echo "Creating Demo Data via MCP Server"
echo "================================================"

# Function to send chat request
send_query() {
    local query="$1"
    echo -e "\n📤 Query: $query"
    curl -s -X POST "$API_URL" \
        -H "Content-Type: application/json" \
        -d "{
            \"message\": \"$query\",
            \"context\": {
                \"user_id\": \"demo@test.com\",
                \"user_email\": \"demo@test.com\"
            }
        }" | python3 -m json.tool
    sleep 2  # Rate limiting
}

# 1. Create Fiscal Years
echo -e "\n\n=== STEP 1: Creating Fiscal Years ==="
send_query "Create a fiscal year 2023-2024 starting from 2023-04-01 to 2024-03-31"
send_query "Create a fiscal year 2024-2025 starting from 2024-04-01 to 2025-03-31"

# 2. Create Customers
echo -e "\n\n=== STEP 2: Creating Customers ==="
send_query "Create a customer named 'Acme Corporation' of type Company in Commercial customer group"
send_query "Create a customer named 'Tech Solutions Inc' of type Company"
send_query "Create a customer named 'Global Traders Ltd' of type Company"
send_query "Create a customer named 'John Doe' of type Individual"

# 3. Create Suppliers
echo -e "\n\n=== STEP 3: Creating Suppliers ==="
send_query "Create a supplier named 'Hardware Suppliers Co' in Hardware supplier group"
send_query "Create a supplier named 'Office Supplies Ltd'"
send_query "Create a supplier named 'Tech Components Inc' in Hardware supplier group"

# 4. Create Items
echo -e "\n\n=== STEP 4: Creating Items ==="
send_query "Create an item with code LAPTOP-001 named 'Dell Latitude Laptop' in Products group with rate 50000"
send_query "Create an item with code MOUSE-001 named 'Wireless Mouse' with rate 500"
send_query "Create an item with code KEYBOARD-001 named 'Mechanical Keyboard' with rate 3000"
send_query "Create an item with code MONITOR-001 named '24 inch LED Monitor' with rate 12000"

# 5. Create Projects
echo -e "\n\n=== STEP 5: Creating Projects ==="
send_query "Create a project named 'Website Redesign' with status Open and priority High starting from 2024-01-01"
send_query "Create a project named 'Mobile App Development' with status Open starting from 2024-02-01"
send_query "Create a project named 'Infrastructure Upgrade' with status Completed"

echo -e "\n\n================================================"
echo "✅ Demo Data Creation Complete!"
echo "================================================"
echo ""
echo "📊 You can now test queries like:"
echo "  • 'Show me all customers'"
echo "  • 'List all items'"
echo "  • 'Show me projects'"
echo "  • 'Give me a list of suppliers'"
echo ""

