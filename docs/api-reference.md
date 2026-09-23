# API Reference

Complete reference for ERPNext MCP Server HTTP API.

## Base URL

```text
http://localhost:8080/api/v1
```

## Authentication

Every path except `/health` and `/api/v1/health` goes through the auth middleware when
`auth.enabled` is on. Each request carries the caller's own credential — an OAuth2 bearer token in
`Authorization`, or a Frappe session in a `sid` cookie — and the server forwards it to Frappe, so
Frappe enforces that user's permissions. The `erpnext.api_key`/`api_secret` pair in `config.yaml` is
the fallback used when a request carries no credential of its own, and it runs as that key's user.
With `auth.require_auth` on, a request without a credential is answered `401` instead.
See [Authentication](authentication.md).

## Endpoints

### MCP (Streamable HTTP)

**POST** `/mcp`

The Model Context Protocol endpoint, served by the official Go SDK. This is what an MCP client
connects to; `tools/list` here publishes every tool in the catalogue, including the legacy names
that the REST listing below leaves out. Request bodies over 1 MiB are answered `413`.

---

### Health Check

**GET** `/api/v1/health`

Report that the process is up. It makes no call to Frappe or to an LLM, so it says nothing about
either.

**Response:**

```json
{
  "status": "healthy",
  "timestamp": "2025-11-12T10:30:00Z"
}
```

---

### List Tools

**GET** `/api/v1/tools`

List the described tools. The legacy names are callable but carry no description, so they are left
out here; `POST /mcp` publishes the whole catalogue. The README's table is generated from the same
catalogue.

**Response:**

```json
{
  "count": 10,
  "tools": [
    {
      "name": "get_document",
      "description": "Retrieve a single ERPNext document by doctype and name",
      "inputSchema": {
        "type": "object",
        "properties": {
          "doctype": {"type": "string", "description": "ERPNext document type (e.g., Customer, Sales Order)"},
          "name": {"type": "string", "description": "Document name or ID"}
        },
        "required": ["doctype", "name"]
      }
    }
  ]
}
```

---

### Execute Tool

**POST** `/api/v1/tools/{tool_name}`

Execute a specific MCP tool directly. The body is `{"params": <the tool's parameters>}`; the JSON shown under each
tool below is that `params` value. `/tool/{tool_name}` is the same handler under its older path.

#### Get Document

**POST** `/api/v1/tools/get_document`

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

**POST** `/api/v1/tools/list_documents`

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

**POST** `/api/v1/tools/search_documents`

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

**POST** `/api/v1/tools/aggregate_documents`

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

`id` is optional and is echoed back; the tool name comes from the path. The reply is the tool
response: a summary line and the JSON payload, as text blocks, plus `structured` for a tool that
declares an output schema.

```json
{
  "id": "http-1790154742362156000",
  "content": [
    {"type": "text", "text": "Retrieved Project document: PROJ-0001"},
    {"type": "text", "text": "{\"doctype\":\"Project\",\"name\":\"PROJ-0001\",\"status\":\"Open\"}"}
  ]
}
```

The argument names and types of each tool are not repeated here, because they would drift: `GET
/api/v1/tools` publishes the JSON Schema of every described tool, `POST /mcp` publishes the whole
catalogue, and the README's table is generated from it. The names worth knowing when reading older
examples: `list_documents` and `search_documents` take `page_length`, not `limit`;
`search_documents` takes `search`, not `query`; `aggregate_documents` takes `metric` (one of
`sum`, `count`, `avg`, `min`, `max`), the `field` to aggregate, `group_by` and `top_n`, not a
`fields` list of SQL expressions.

---

### Legacy tools

**POST** `/api/v1/tools/run_report`

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

**POST** `/api/v1/tools/create_document`

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

**POST** `/api/v1/tools/update_document`

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

**POST** `/api/v1/tools/delete_document`

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

**POST** `/api/v1/tools/analyze_document`

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

**POST** `/api/v1/tools/get_project_status`

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

## Error Responses

There is no single error envelope. The REST endpoints answer in two shapes.

The auth middleware answers JSON:

```json
{"error": "Unauthorized", "message": "Valid authentication required"}
```

Everything else answers `text/plain` with one line, as Go's `http.Error` writes it:

| Status | Body | When |
| --- | --- | --- |
| `400` | `Tool name is required` / `Invalid request body` | no tool in the path, a body that is not JSON, or a body over 1 MiB |
| `404` | `Tool not found` | no tool of that name in the catalogue |
| `405` | `Method not allowed` | a tool call that is not a POST |
| `500` | the tool's own error, one line | the tool failed; a Frappe failure carries Frappe's one-line exception, never its body |

`POST /mcp` is JSON-RPC 2.0 and follows the MCP specification instead: a body over 1 MiB is
answered `413 Request body too large`, and a failure inside a tool comes back as a result with
`"isError": true`, not as a protocol error.

---

## Rate Limiting

The server does not rate-limit its callers and sends no `X-RateLimit-*` headers. The
`erpnext.rate_limit` block bounds the calls the server makes *to Frappe*, so that a burst of tool
calls cannot overrun the site:

```yaml
erpnext:
  rate_limit:
    requests_per_second: 10
    burst: 20
```

Frappe's own rate limiting still applies to those calls.

---

## MCP Protocol

Both binaries speak the Model Context Protocol: the stdio binary over stdin and stdout, for Cursor
and Claude Desktop, and the HTTP server at `POST /mcp`.

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

---

## Examples

Each call carries the caller's credential; `$TOKEN` below is an OAuth2 access token, and a Frappe
session works the same way as `-H "Cookie: sid=$SID"`.

### cURL

```bash
# Health check (no credential: it is one of the two public paths)
curl http://localhost:8080/api/v1/health

# Get specific document
curl -X POST http://localhost:8080/api/v1/tools/get_document \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"params": {"doctype": "Project", "name": "PROJ-0001"}}'

# Aggregation query
curl -X POST http://localhost:8080/api/v1/tools/aggregate_documents \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "params": {
      "doctype": "Sales Invoice",
      "metric": "sum",
      "field": "grand_total",
      "group_by": "customer",
      "top_n": 5,
      "filters": {"status": "Paid"}
    }
  }'

# Run report
curl -X POST http://localhost:8080/api/v1/tools/run_report \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "params": {
      "report_name": "Sales Analytics",
      "filters": {"company": "My Company"}
    }
  }'

# The same tool call over MCP
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"jsonrpc": "2.0", "id": 1, "method": "tools/call",
       "params": {"name": "get_document",
                  "arguments": {"doctype": "Project", "name": "PROJ-0001"}}}'
```

### JavaScript

```javascript
// List open projects
const response = await fetch('http://localhost:8080/api/v1/tools/list_documents', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    Authorization: `Bearer ${token}`,
  },
  body: JSON.stringify({
    params: { doctype: 'Project', filters: { status: 'Open' } }
  })
});
const data = await response.json();
console.log(data.content);
```

### Python

```python
import requests

# Get document
response = requests.post(
    'http://localhost:8080/api/v1/tools/get_document',
    json={'doctype': 'Project', 'name': 'PROJ-0001'}
)
summary, payload = response.json()["content"]
print(summary["text"])
```

---

Next: [Development Guide](development.md)
