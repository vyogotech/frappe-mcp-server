# Smart Query Understanding - Implementation Plan

## Problem Statement

User queries like "what's the default currency?" and "give details of the current company" are failing because:
1. System treats "default" and "current" as literal entity names
2. List queries don't fetch all important fields by default
3. Need better understanding of contextual references

## Solutions Implemented

### 1. Contextual Query Understanding ✅

**Added to Intent Extraction Prompt:**
- Rule: Contextual words like "default", "current", "active", "primary" should trigger `list` action, not `get`
- Examples added for:
  - "what's the default currency?" → list Company
  - "give details of the current company" → list Company
  - "show me the active user" → list User

**How it works:**
```
Query: "what's the default currency?"
Intent: {action: "list", doctype: "Company", entity_name: ""}
Tool: list_documents
Result: Returns all companies with full fields
LLM Format: Extracts default_currency field from result
```

### 2. Smart Field Extraction for List Queries

**Need to implement:** Add LLM-based field extraction for list queries

When user asks: "what's the default currency?"
- Intent: list Company
- Extract semantic fields: ["name", "default_currency", "company_name"]
- Call list_documents with fields parameter
- LLM formats response focusing on default_currency

**Implementation:**
```go
func (s *MCPServer) extractListQueryFields(ctx context.Context, query string, doctype string) ([]string, error) {
    // Use LLM to determine what fields user is interested in
    // Based on query context
}
```

### 3. Report Functionality ✅

Already implemented:
- `run_report` tool exists
- Intent extraction recognizes "run X report" → action="report"
- Example added: "run accounts receivable report"

**How it works:**
```
Query: "run sales analytics report"
Intent: {action: "report", doctype: "", entity_name: ""}
Extract: {report_name: "Sales Analytics", filters: {}}
Tool: run_report
```

## Architecture Flow

```
User Query: "what's the default currency?"
    ↓
Intent Extraction (Groq)
    → Detects contextual reference
    → action="list", doctype="Company"
    ↓
Field Extraction (NEW - LLM)
    → Analyzes semantic meaning
    → fields=["name", "default_currency", "company_name"]
    ↓
list_documents Tool
    → GET /api/resource/Company?fields=["name","default_currency","company_name"]
    ↓
Response Formatting (LLM)
    → Understands user asked about currency
    → Formats: "The default currency is USD"
```

## Key Fields by DocType

### Common Key Fields to Always Fetch:

**Company:**
- name, company_name, default_currency, country, abbr

**Customer:**
- name, customer_name, customer_type, customer_group, territory, email

**User:**
- name, email, first_name, last_name, enabled, role_profile_name

**Project:**
- name, project_name, status, priority, expected_start_date, expected_end_date

**Item:**
- name, item_code, item_name, item_group, stock_uom, standard_rate

**Sales Invoice:**
- name, customer, posting_date, grand_total, status, outstanding_amount

## Implementation Priority

### Phase 1: ✅ DONE
- [x] Add contextual query examples to intent prompt
- [x] Update entity_name rules to handle "default", "current", etc.

### Phase 2: Next Steps
- [ ] Implement `extractListQueryFields()` function
- [ ] Add doctype-to-key-fields mapping
- [ ] Wire up field extraction in handleChat for list queries
- [ ] Test with contextual queries

### Phase 3: Optimization
- [ ] Cache common doctype field schemas
- [ ] Add field inference based on query patterns
- [ ] Improve response formatting to highlight requested information

## Testing Scenarios

### Should Work After Implementation:

| Query | Expected Behavior |
|-------|-------------------|
| "what's the default currency?" | List Company, extract default_currency, format answer |
| "give details of current company" | List Company with all key fields, format nicely |
| "show active users" | List enabled Users with key fields |
| "run sales report" | Execute Sales Analytics report |
| "what's the company name?" | List Company, extract company_name |

## Next Steps

1. Build and test current changes (contextual examples)
2. Implement field extraction if needed
3. Test with real queries
4. Iterate based on results

---

**Status:** Contextual understanding improved with prompt examples. Ready for testing.

