# Aggregation and Reporting Implementation

## Overview

Successfully implemented advanced analytics capabilities for the ERPNext MCP Server following a **Hybrid Architecture**:
- **Pure MCP Interface**: STDIO, WebSocket, HTTP Tools API (for AI clients like Cursor, Claude)
- **Conversational API**: HTTP Chat API with AI-powered intent extraction and response formatting (for Open WebUI, custom apps)

## Features Implemented

### 1. **Aggregation Tool** (`aggregate_documents`)
Perform complex aggregation queries on ERPNext data with SQL-like capabilities.

**Capabilities:**
- GROUP BY operations
- Aggregation functions: SUM, COUNT, AVG, MAX, MIN
- Filtering with complex conditions
- Sorting (ORDER BY)
- Limiting results (TOP N queries)

**Example Queries:**
```
- "top 5 customers by revenue"
- "show me total sales by customer"
- "which items sold the most"
- "average order value by month"
- "count of open sales orders by customer"
```

**API Endpoint:**
```bash
POST /api/v1/tools/aggregate_documents
{
  "doctype": "Sales Invoice",
  "fields": ["customer", "SUM(grand_total) as total_revenue"],
  "group_by": "customer",
  "order_by": "total_revenue desc",
  "limit": 5,
  "filters": {"status": "Paid"}
}
```

### 2. **Report Runner Tool** (`run_report`)
Execute native Frappe/ERPNext reports with filtering capabilities.

**Capabilities:**
- Run any standard ERPNext report
- Run custom reports
- Apply dynamic filters
- Get structured report data with columns and rows

**Supported Reports:**
- Sales Analytics, Sales Register, Sales Order Analysis
- Purchase Register, Purchase Analytics
- Customer Ledger Summary, Supplier Ledger Summary
- Stock Balance, Stock Ledger
- Profit and Loss Statement, Balance Sheet
- General Ledger
- And any custom reports

**Example Queries:**
```
- "run Sales Analytics report"
- "show me Customer Ledger Summary"
- "execute Stock Balance report for warehouse Main"
```

**API Endpoint:**
```bash
POST /api/v1/tools/run_report
{
  "report_name": "Sales Analytics",
  "filters": {
    "company": "My Company",
    "from_date": "2024-01-01",
    "to_date": "2024-12-31"
  }
}
```

## Architecture Components

### Backend Changes

#### 1. **New Types** (`internal/types/types.go`)
```go
- AggregationRequest: Parameters for aggregation queries
- ReportRequest: Parameters for report execution
- ReportResponse: Structured report results
- ReportColumn: Column metadata
```

#### 2. **Frappe Client** (`internal/frappe/client.go`)
```go
- RunAggregationQuery(): Calls frappe.client.get_list with GROUP BY
- RunReport(): Calls frappe.desk.query_report.run
```

#### 3. **Tool Registry** (`internal/tools/registry.go`)
```go
- AggregateDocuments(): MCP tool wrapper for aggregation
- RunReport(): MCP tool wrapper for reports
```

#### 4. **Server** (`internal/server/server.go`)
```go
- extractAggregationParams(): LLM-powered parameter extraction
- extractReportParams(): LLM-powered report name extraction
- Updated intent extraction to recognize aggregate/report queries
- Added special handling in chat endpoint
```

### Intent Extraction Enhancement

The AI now recognizes analytical queries:

```javascript
// Old: Only recognized CRUD operations
"list", "get", "search", "create", "update", "delete"

// New: Also recognizes analytics
"aggregate" - for queries with SUM, COUNT, AVG, TOP N
"report" - for running specific reports
```

**Detection Rules:**
1. Keywords like "top N", "bottom N", "most", "highest", "sum", "total", "average" → `aggregate`
2. "run X report", "execute Y report", "show Z report" → `report`

## Usage Examples

### Via Natural Language (Conversational API)

```bash
# Aggregation queries
curl -X POST http://localhost:8080/api/v1/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "top 5 customers by revenue"}'

curl -X POST http://localhost:8080/api/v1/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "show me total sales by item in table format"}'

# Report queries
curl -X POST http://localhost:8080/api/v1/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "run Sales Analytics report"}'
```

### Via Direct Tool Call (Pure MCP)

```bash
# Aggregation
curl -X POST http://localhost:8080/api/v1/tools/aggregate_documents \
  -H "Content-Type: application/json" \
  -d '{
    "doctype": "Sales Invoice",
    "fields": ["customer", "SUM(grand_total) as revenue"],
    "group_by": "customer",
    "order_by": "revenue desc",
    "limit": 10
  }'

# Report
curl -X POST http://localhost:8080/api/v1/tools/run_report \
  -H "Content-Type: application/json" \
  -d '{
    "report_name": "Sales Analytics",
    "filters": {"company": "My Company"}
  }'
```

## Flow Diagram

```
User Query: "top 5 customers by revenue"
                    ↓
         [Intent Extraction - LLM]
                    ↓
    action: "aggregate", doctype: "Sales Invoice"
                    ↓
      [Extract Aggregation Params - LLM]
                    ↓
    fields: ["customer", "SUM(grand_total) as total"]
    group_by: "customer"
    order_by: "total desc"
    limit: 5
                    ↓
    [Call aggregate_documents tool]
                    ↓
    [frappe.client.get_list with GROUP BY]
                    ↓
         [Format Results - LLM]
                    ↓
    Markdown table with top 5 customers
```

## Testing

### Registered Tools
Total: 15 tools (9 core + 2 aggregation/reporting + 4 legacy)

```bash
$ curl http://localhost:8080/api/v1/tools | jq '.tools | map(.name)'
[
  "get_document",
  "list_documents",
  "create_document",
  "update_document",
  "delete_document",
  "search_documents",
  "aggregate_documents",  ← NEW
  "run_report",           ← NEW
  "analyze_document",
  ... (legacy tools)
]
```

### Test Queries

#### Aggregation
- ✅ "top 5 customers by revenue"
- ✅ "total sales by item"
- ✅ "which products sold the most"
- ✅ "average order value by customer"

#### Reporting
- ✅ "run Sales Analytics report"
- ✅ "execute Purchase Register"
- ✅ "show Stock Balance report"

## Benefits

### 1. **Powerful Analytics**
- Users can ask complex analytical questions in natural language
- No need to write SQL or understand ERPNext query syntax
- Supports business intelligence queries

### 2. **Report Access**
- Direct access to all ERPNext standard and custom reports
- Simpler than navigating ERPNext UI
- Can be integrated into custom dashboards

### 3. **Hybrid Architecture**
- MCP clients (Cursor, Claude) can use tools directly with full control
- Simple clients (Open WebUI) get AI-powered assistance
- Both use the same underlying tools

### 4. **Extensibility**
- Easy to add more aggregation functions
- Can support custom report filters
- Framework for future analytics features

## Configuration

No special configuration required. The tools work out of the box with:
- `LLM_BASE_URL`: For intent extraction and formatting
- `LLM_MODEL`: Should support JSON output (llama3.2, gpt-4, claude, etc.)
- `FRAPPE_BASE_URL`, `FRAPPE_API_KEY`, `FRAPPE_API_SECRET`: For ERPNext access

## Future Enhancements

### Potential Additions:
1. **Visualization Hints**: Return chart type suggestions with data
2. **Scheduled Reports**: Run reports on schedule
3. **Export Formats**: CSV, Excel, PDF export
4. **Report Builder**: Create custom reports via conversation
5. **Caching**: Cache frequently run reports
6. **Trending Analysis**: Time-series aggregations

## API Contract

### Aggregation Request
```typescript
interface AggregationRequest {
  doctype: string;              // ERPNext DocType
  fields: string[];             // SELECT clause with aggregations
  filters?: Record<string, any>; // WHERE clause
  group_by?: string;            // GROUP BY field
  order_by?: string;            // ORDER BY clause
  limit?: number;               // LIMIT clause
}
```

### Report Request
```typescript
interface ReportRequest {
  report_name: string;          // Report name in ERPNext
  filters?: Record<string, any>; // Report-specific filters
  user?: string;                // User context (optional)
}
```

### Response Format
Both tools return MCP ToolResponse with structured JSON data that gets formatted by the LLM for display.

## Documentation

See also:
- [README.md](README.md) - Main documentation
- [docs/api-reference.md](docs/api-reference.md) - API documentation
- [docs/ai-features.md](docs/ai-features.md) - AI features guide

## Status

✅ **Complete and Tested**
- All 8 implementation tasks completed
- Build successful
- Docker deployment verified
- Tools registered and accessible
- Intent extraction working
- Ready for production use

---

**Implementation Date**: November 14, 2025  
**Architecture**: Hybrid (Pure MCP + Conversational API)  
**Total Tools**: 15 (9 core + 2 analytics + 4 legacy)  
**Status**: Production Ready ✅

