# Analytics & Reporting Features

> **NEW** Advanced analytics capabilities for ERPNext MCP Server

## Overview

The ERPNext MCP Server now includes powerful analytics and reporting capabilities that allow you to:
- Perform complex aggregations (SUM, COUNT, AVG, TOP N)
- Execute native ERPNext reports
- Ask analytical questions in natural language

## Features

### 1. Aggregation Queries

Perform SQL-like aggregations on your ERPNext data without writing SQL.

**Natural Language Examples:**
```
"Show me top 5 customers by revenue"
"What are total sales by item this month?"
"Which products sold the most?"
"Average order value by customer"
"Count of open sales orders by status"
```

**Direct API:**
```bash
curl -X POST http://localhost:8080/api/v1/tool/aggregate_documents \
  -H "Content-Type: application/json" \
  -d '{
    "doctype": "Sales Invoice",
    "fields": ["customer", "SUM(grand_total) as total_revenue"],
    "group_by": "customer",
    "order_by": "total_revenue desc",
    "limit": 5,
    "filters": {"status": "Paid"}
  }'
```

**Supported Aggregation Functions:**
- `SUM(field)` - Sum of all values
- `COUNT(*)` or `COUNT(field)` - Count of records
- `AVG(field)` - Average value
- `MAX(field)` - Maximum value
- `MIN(field)` - Minimum value

**Common Use Cases:**
- **Top N Queries**: "top 10 customers by revenue"
- **Rankings**: "highest selling products"
- **Totals by Category**: "total sales by item"
- **Counting**: "number of orders by status"
- **Business Metrics**: "average deal size by sales person"

### 2. Report Execution

Execute any standard or custom ERPNext report with filters.

**Natural Language Examples:**
```
"Run Sales Analytics report"
"Execute Customer Ledger Summary"
"Show Stock Balance report"
"Generate Profit and Loss statement for this year"
```

**Direct API:**
```bash
curl -X POST http://localhost:8080/api/v1/tool/run_report \
  -H "Content-Type: application/json" \
  -d '{
    "report_name": "Sales Analytics",
    "filters": {
      "company": "My Company",
      "from_date": "2024-01-01",
      "to_date": "2024-12-31"
    }
  }'
```

**Common Reports:**

| Category | Reports |
|----------|---------|
| **Sales** | Sales Analytics, Sales Register, Sales Order Analysis, Sales Person-wise Transaction Summary |
| **Purchase** | Purchase Register, Purchase Analytics, Supplier-wise Purchase Analytics |
| **Accounting** | Customer Ledger Summary, Supplier Ledger Summary, General Ledger, Trial Balance |
| **Inventory** | Stock Balance, Stock Ledger, Stock Analytics, Item-wise Sales Register |
| **Financial** | Profit and Loss Statement, Balance Sheet, Cash Flow Statement |
| **Project** | Project wise Stock Tracking, Timesheet Billing Summary |

## API Reference

### aggregate_documents

**Endpoint:** `POST /api/v1/tool/aggregate_documents`

**Parameters:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `doctype` | string | Yes | ERPNext DocType to query |
| `fields` | array | Yes | Fields to select (with aggregations) |
| `group_by` | string | No | Field to group by |
| `order_by` | string | No | Sort order (e.g., "revenue desc") |
| `limit` | number | No | Limit results (for TOP N) |
| `filters` | object | No | WHERE clause filters |

**Example Request:**
```json
{
  "doctype": "Sales Invoice",
  "fields": [
    "customer",
    "SUM(grand_total) as total_revenue",
    "COUNT(*) as invoice_count"
  ],
  "group_by": "customer",
  "order_by": "total_revenue desc",
  "limit": 10,
  "filters": {
    "status": "Paid",
    "posting_date": [">=", "2024-01-01"]
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
      "total_revenue": 125000.50,
      "invoice_count": 45
    },
    {
      "customer": "XYZ Ltd",
      "total_revenue": 98500.75,
      "invoice_count": 32
    }
  ],
  "count": 10
}
```

### run_report

**Endpoint:** `POST /api/v1/tool/run_report`

**Parameters:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `report_name` | string | Yes | Exact name of the ERPNext report |
| `filters` | object | No | Report-specific filters |
| `user` | string | No | User context (optional) |

**Example Request:**
```json
{
  "report_name": "Sales Analytics",
  "filters": {
    "company": "My Company",
    "from_date": "2024-01-01",
    "to_date": "2024-12-31",
    "range": "Monthly"
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
    ["XYZ Ltd", 98500.75],
    ["DEF Inc", 75300.25]
  ],
  "row_count": 3
}
```

## Natural Language Processing

The AI automatically detects analytical queries and routes them appropriately:

### Detection Rules

1. **Keywords trigger aggregation:**
   - "top N", "bottom N"
   - "highest", "lowest", "most", "least"
   - "sum", "total", "average", "count"
   - "by [field]" (grouping indicator)

2. **Keywords trigger reports:**
   - "run [report name] report"
   - "execute [report name]"
   - "show [report name] report"

### Examples with AI Flow

**Query:** "top 5 customers by revenue"

```
User Input → AI Intent Extraction
            ↓
    {action: "aggregate", doctype: "Sales Invoice"}
            ↓
    AI Parameter Extraction
            ↓
    {
      fields: ["customer", "SUM(grand_total) as revenue"],
      group_by: "customer",
      order_by: "revenue desc",
      limit: 5
    }
            ↓
    Execute aggregate_documents tool
            ↓
    Format results as table (AI formatting)
```

**Query:** "run Sales Analytics report"

```
User Input → AI Intent Extraction
            ↓
    {action: "report"}
            ↓
    AI Parameter Extraction
            ↓
    {
      report_name: "Sales Analytics",
      filters: {}
    }
            ↓
    Execute run_report tool
            ↓
    Format results as table (AI formatting)
```

## Usage Patterns

### In Cursor IDE

```
@erpnext Show me top 10 customers by revenue

@erpnext What are total sales by item?

@erpnext Run Sales Analytics report for this year

@erpnext Which products have the highest profit margin?
```

### In Open WebUI

Simply type your analytical question:
- "Show me top customers"
- "What's our average order value?"
- "Run the Customer Ledger Summary"

### Programmatic Access

```python
import requests

# Aggregation
response = requests.post(
    'http://localhost:8080/api/v1/tool/aggregate_documents',
    json={
        'doctype': 'Sales Invoice',
        'fields': ['customer', 'SUM(grand_total) as revenue'],
        'group_by': 'customer',
        'order_by': 'revenue desc',
        'limit': 5
    }
)
top_customers = response.json()['results']

# Report
response = requests.post(
    'http://localhost:8080/api/v1/tool/run_report',
    json={
        'report_name': 'Sales Analytics',
        'filters': {'company': 'My Company'}
    }
)
report_data = response.json()
```

## Advanced Examples

### Complex Aggregations

**Multi-level grouping:**
```json
{
  "doctype": "Sales Invoice",
  "fields": [
    "customer_group",
    "status",
    "SUM(grand_total) as total",
    "COUNT(*) as count",
    "AVG(grand_total) as avg_value"
  ],
  "group_by": "customer_group, status",
  "order_by": "total desc"
}
```

**Time-based analysis:**
```json
{
  "doctype": "Sales Invoice",
  "fields": [
    "MONTH(posting_date) as month",
    "SUM(grand_total) as monthly_revenue"
  ],
  "group_by": "MONTH(posting_date)",
  "filters": {
    "posting_date": [">=", "2024-01-01"]
  }
}
```

### Report Filters

**Date range filtering:**
```json
{
  "report_name": "Sales Register",
  "filters": {
    "from_date": "2024-01-01",
    "to_date": "2024-12-31",
    "customer": "ABC Corp"
  }
}
```

**Multi-criteria filtering:**
```json
{
  "report_name": "Stock Balance",
  "filters": {
    "warehouse": "Main Store",
    "item_group": "Electronics",
    "show_zero_values": 0
  }
}
```

## Best Practices

### 1. Be Specific in Queries
```
❌ "Show sales"
✅ "Show top 10 customers by total sales this year"
```

### 2. Use Appropriate DocTypes
For aggregations, use transaction doctypes:
- `Sales Invoice` - for revenue analysis
- `Sales Order` - for order analysis
- `Purchase Invoice` - for expense analysis
- `Stock Entry` - for inventory movements

### 3. Add Filters for Performance
```json
{
  "doctype": "Sales Invoice",
  "fields": ["customer", "SUM(grand_total) as total"],
  "group_by": "customer",
  "filters": {
    "posting_date": [">=", "2024-01-01"],  // Limit time range
    "status": "Paid"                       // Filter status
  }
}
```

### 4. Request Table Format
For better readability, add "in table format" to your queries:
```
"Show me top 5 customers by revenue in table format"
```

## Troubleshooting

### Query Not Recognized as Analytics

**Problem:** "top customers" returns a list instead of aggregation.

**Solution:** Be more explicit:
```
"Show me top 10 customers by total revenue"
```

### Report Not Found

**Problem:** Error: "Report 'sales analytics' not found"

**Solution:** Use exact report name from ERPNext:
```
"Run Sales Analytics report"  (with capital letters)
```

### Empty Results

**Problem:** Aggregation returns no data.

**Solution:** 
1. Check if the DocType has data
2. Verify filter values are correct
3. Ensure field names match ERPNext schema

### Slow Queries

**Problem:** Aggregation takes too long.

**Solutions:**
1. Add date filters to limit data
2. Use indexed fields in filters
3. Consider using ERPNext reports instead for complex analyses

## Architecture Notes

### Hybrid Approach

The system follows a **hybrid architecture**:

**Pure MCP Interface:**
- Tools (`aggregate_documents`, `run_report`) return structured data
- MCP clients (Cursor, Claude) call tools directly
- Client handles formatting

**Conversational API:**
- AI extracts intent from natural language
- AI determines parameters
- AI formats results for user
- Suitable for Open WebUI and simple clients

### Under the Hood

**Aggregation Tool:**
```
Natural Language → LLM Intent → LLM Params → Frappe API
                                              (frappe.client.get_list)
```

**Report Tool:**
```
Natural Language → LLM Intent → LLM Params → Frappe API
                                              (frappe.desk.query_report.run)
```

## Future Enhancements

Planned features:
- Chart/visualization hints
- Scheduled report execution
- Export to CSV/Excel/PDF
- Custom report builder via conversation
- Caching for frequently-run queries
- Time-series analysis helpers

## See Also

- [AI Features](ai-features.md) - Overview of AI capabilities
- [API Reference](api-reference.md) - Complete API documentation
- [Quick Start](quick-start.md) - Getting started guide

---

**Status:** Production Ready ✅  
**Added:** November 2025  
**Tools:** `aggregate_documents`, `run_report`

