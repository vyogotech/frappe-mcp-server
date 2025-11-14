# API Reference

Complete reference for ERPNext MCP Server HTTP API.

## Base URL

```
http://localhost:8080/api/v1
```

## Authentication

Currently, authentication is handled at the ERPNext level using API credentials in `config.yaml`. The MCP server acts as a trusted proxy.

## Endpoints

### Health Check

**GET** `/api/v1/health`

Check server status and connectivity.

**Response:**
```json
{
  "status": "healthy",
  "erpnext_connected": true,
  "ollama_available": true,
  "timestamp": "2025-11-12T10:30:00Z"
}
```

---

### List Tools

**GET** `/api/v1/tools`

Get all available MCP tools.

**Response:**
```json
{
  "tools": [
    {
      "name": "get_document",
      "description": "Get a specific ERPNext document",
      "input_schema": {
        "type": "object",
        "properties": {
          "doctype": {"type": "string"},
          "name": {"type": "string"}
        },
        "required": ["doctype", "name"]
      }
    }
    // ... more tools
  ]
}
```

---

### Natural Language Chat

**POST** `/api/v1/chat`

Process natural language queries with AI.

**Request:**
```json
{
  "message": "Show me project PROJ-0001"
}
```

**Response:**
```json
{
  "response": "Here are the details for project PROJ-0001...",
  "data": { /* ERPNext document data */ },
  "tools_called": ["get_document"],
  "timestamp": "2025-11-12T10:30:00Z",
  "data_quality": "complete",
  "data_size": 1,
  "is_valid_data": true
}
```

**Status Codes:**
- `200` - Success
- `400` - Bad request (invalid JSON)
- `404` - Document not found
- `500` - Internal server error

---

### Execute Tool

**POST** `/api/v1/tool/{tool_name}`

Execute a specific MCP tool directly.

#### Get Document

**POST** `/api/v1/tool/get_document`

```json
{
  "doctype": "Project",
  "name": "PROJ-0001"
}
```

**Response:**
```json
{
  "doctype": "Project",
  "name": "PROJ-0001",
  "project_name": "Website Redesign",
  "status": "Open",
  "priority": "High",
  "percent_complete": 45,
  // ... all fields
}
```

---

#### List Documents

**POST** `/api/v1/tool/list_documents`

```json
{
  "doctype": "Customer",
  "limit": 20,
  "filters": {
    "customer_group": "Commercial",
    "disabled": 0
  },
  "fields": ["name", "customer_name", "email"]
}
```

**Response:**
```json
{
  "doctype": "Customer",
  "documents": [
    {
      "name": "CUST-0001",
      "customer_name": "ABC Corp",
      "email": "contact@abc.com"
    }
    // ... more documents
  ],
  "count": 20
}
```

---

#### Search Documents

**POST** `/api/v1/tool/search_documents`

```json
{
  "doctype": "Project",
  "query": "website redesign",
  "limit": 10
}
```

**Response:**
```json
{
  "doctype": "Project",
  "query": "website redesign",
  "results": [
    {
      "name": "PROJ-0001",
      "project_name": "Website Redesign",
      "status": "Open"
    }
  ],
  "count": 1
}
```

---

#### Aggregate Documents 🆕

**POST** `/api/v1/tool/aggregate_documents`

Perform SQL-like aggregation queries on ERPNext data.

**Request:**
```json
{
  "doctype": "Sales Invoice",
  "fields": ["customer", "SUM(grand_total) as total_revenue"],
  "group_by": "customer",
  "order_by": "total_revenue desc",
  "limit": 5,
  "filters": {
    "status": "Paid"
  }
}
```

**Response:**
```json
{
  "doctype": "Sales Invoice",
  "group_by": "customer",
  "results": [
    {
      "customer": "ABC Corp",
      "total_revenue": 125000.50
    },
    {
      "customer": "XYZ Ltd",
      "total_revenue": 98500.75
    }
    // ... top 5 customers
  ],
  "count": 5
}
```

**Supported Aggregations:**
- `SUM(field)` - Sum of values
- `COUNT(field)` or `COUNT(*)` - Count records
- `AVG(field)` - Average value
- `MAX(field)` - Maximum value
- `MIN(field)` - Minimum value

**Use Cases:**
- Top N queries: "top 10 customers by revenue"
- Totals by category: "total sales by item"
- Counting: "number of orders by status"
- Rankings: "highest selling products"

---

#### Run Report 🆕

**POST** `/api/v1/tool/run_report`

Execute Frappe/ERPNext standard or custom reports.

**Request:**
```json
{
  "report_name": "Sales Analytics",
  "filters": {
    "company": "My Company",
    "from_date": "2024-01-01",
    "to_date": "2024-12-31"
  }
}
```

**Response:**
```json
{
  "report_name": "Sales Analytics",
  "columns": [
    {
      "label": "Customer",
      "fieldname": "customer",
      "fieldtype": "Link",
      "width": 150
    },
    {
      "label": "Total Amount",
      "fieldname": "total_amount",
      "fieldtype": "Currency",
      "width": 120
    }
  ],
  "data": [
    ["ABC Corp", 125000.50],
    ["XYZ Ltd", 98500.75]
  ],
  "row_count": 2
}
```

**Common Reports:**
- **Sales**: Sales Analytics, Sales Register, Sales Order Analysis
- **Purchase**: Purchase Register, Purchase Analytics
- **Accounting**: Customer Ledger Summary, Supplier Ledger Summary, General Ledger
- **Inventory**: Stock Balance, Stock Ledger
- **Financial**: Profit and Loss Statement, Balance Sheet

---

#### Create Document

**POST** `/api/v1/tool/create_document`

```json
{
  "doctype": "Task",
  "data": {
    "subject": "Review design mockups",
    "status": "Open",
    "priority": "High",
    "project": "PROJ-0001"
  }
}
```

**Response:**
```json
{
  "status": "success",
  "name": "TASK-2024-0001",
  "document": { /* created document */ }
}
```

---

#### Update Document

**POST** `/api/v1/tool/update_document`

```json
{
  "doctype": "Task",
  "name": "TASK-2024-0001",
  "data": {
    "status": "Completed",
    "percent_complete": 100
  }
}
```

**Response:**
```json
{
  "status": "success",
  "name": "TASK-2024-0001",
  "document": { /* updated document */ }
}
```

---

#### Delete Document

**POST** `/api/v1/tool/delete_document`

```json
{
  "doctype": "Task",
  "name": "TASK-2024-0001"
}
```

**Response:**
```json
{
  "status": "success",
  "message": "Document deleted successfully"
}
```

---

#### Analyze Document

**POST** `/api/v1/tool/analyze_document`

Generic analysis tool for ANY doctype.

```json
{
  "doctype": "Project",
  "name": "PROJ-0001",
  "include_related": true
}
```

**Response:**
```json
{
  "doctype": "Project",
  "name": "PROJ-0001",
  "document": { /* main document */ },
  "related_documents": {
    "tasks": [ /* related tasks */ ],
    "timesheets": [ /* related timesheets */ ],
    // ... other related docs
  }
}
```

---

### Project-Specific Tools

#### Get Project Status

**POST** `/api/v1/tool/get_project_status`

```json
{
  "project_name": "PROJ-0001"
}
```

**Response:**
```json
{
  "project": { /* project details */ },
  "tasks": {
    "total": 15,
    "completed": 8,
    "open": 5,
    "overdue": 2
  },
  "timeline": {
    "start_date": "2024-01-01",
    "end_date": "2024-06-30",
    "days_remaining": 45,
    "is_delayed": false
  },
  "progress": 53.3
}
```

---

#### Portfolio Dashboard

**POST** `/api/v1/tool/portfolio_dashboard`

```json
{
  "status_filter": "Open"  // optional
}
```

**Response:**
```json
{
  "total_projects": 12,
  "by_status": {
    "Open": 8,
    "Completed": 3,
    "On Hold": 1
  },
  "overall_health": "good",
  "projects": [ /* project summaries */ ]
}
```

---

### OpenAPI Specification

**GET** `/api/v1/openapi.json`

Get the OpenAPI 3.0 specification for the entire API.

---

## Error Responses

All errors follow this format:

```json
{
  "error": {
    "code": "DOCUMENT_NOT_FOUND",
    "message": "Document Project/PROJ-9999 not found",
    "details": {
      "doctype": "Project",
      "name": "PROJ-9999"
    }
  },
  "timestamp": "2025-11-12T10:30:00Z"
}
```

### Common Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `INVALID_REQUEST` | 400 | Malformed request |
| `DOCUMENT_NOT_FOUND` | 404 | Document doesn't exist |
| `PERMISSION_DENIED` | 403 | Insufficient permissions |
| `ERPNEXT_ERROR` | 502 | ERPNext API error |
| `INTERNAL_ERROR` | 500 | Server error |
| `TIMEOUT` | 504 | Request timeout |

---

## Rate Limiting

Requests are rate-limited per configuration:

```yaml
erpnext:
  rate_limit:
    requests_per_second: 10
    burst: 20
```

**Headers:**
```
X-RateLimit-Limit: 10
X-RateLimit-Remaining: 8
X-RateLimit-Reset: 1699564800
```

---

## MCP Protocol (STDIO)

For Cursor/Claude Desktop integration, the STDIO server implements MCP protocol.

### Message Format

**JSON-RPC 2.0:**

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "get_document",
    "arguments": {
      "doctype": "Project",
      "name": "PROJ-0001"
    }
  }
}
```

### Methods

- `initialize` - Initialize connection
- `tools/list` - List available tools
- `tools/call` - Execute a tool
- `resources/list` - List available resources

---

## Examples

### cURL

```bash
# Health check
curl http://localhost:8080/api/v1/health

# Natural language query
curl -X POST http://localhost:8080/api/v1/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "List all customers"}'

# Get specific document
curl -X POST http://localhost:8080/api/v1/tool/get_document \
  -H "Content-Type: application/json" \
  -d '{"doctype": "Project", "name": "PROJ-0001"}'

# Analytics: Top 5 customers by revenue
curl -X POST http://localhost:8080/api/v1/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "show me top 5 customers by revenue in table format"}'

# Aggregation query (direct tool call)
curl -X POST http://localhost:8080/api/v1/tool/aggregate_documents \
  -H "Content-Type: application/json" \
  -d '{
    "doctype": "Sales Invoice",
    "fields": ["customer", "SUM(grand_total) as total"],
    "group_by": "customer",
    "order_by": "total desc",
    "limit": 5
  }'

# Run report
curl -X POST http://localhost:8080/api/v1/tool/run_report \
  -H "Content-Type: application/json" \
  -d '{
    "report_name": "Sales Analytics",
    "filters": {"company": "My Company"}
  }'
```

### JavaScript

```javascript
// Natural language query
const response = await fetch('http://localhost:8080/api/v1/chat', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    message: 'Show me all open projects'
  })
});
const data = await response.json();
console.log(data.response);
```

### Python

```python
import requests

# Get document
response = requests.post(
    'http://localhost:8080/api/v1/tool/get_document',
    json={'doctype': 'Project', 'name': 'PROJ-0001'}
)
project = response.json()
print(project['project_name'])
```

---

Next: [Development Guide](development.md)

