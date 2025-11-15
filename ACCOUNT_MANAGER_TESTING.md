# Account Manager Testing Guide

## Quick Start

### Run All Tests (40+ queries across 10 categories)
```bash
cd /Users/varkrish/personal/frappe-mcp-server
./test_account_manager.sh
```

### Run with Debug Mode (see full JSON responses)
```bash
DEBUG=1 ./test_account_manager.sh
```

### Test Single Query
```bash
curl -s -X POST http://localhost:8080/api/v1/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "show me all my customers"}' | jq '.'
```

---

## Test Categories (40+ Queries)

### 1. **Customer Information** (4 queries)
- "Show me all my customers"
- "Give me a list of customers"
- "Who are my active customers?"
- "Show customer details for Acme Corp"

**Expected Tools:** `list_documents`, `get_document`, `search_documents`

---

### 2. **Revenue & Sales Analysis** (4 queries)
- "What's my total revenue this month?"
- "Show me top 5 customers by revenue"
- "Which customers generated the most revenue?"
- "Get me the sales report"

**Expected Tools:** `aggregate_documents`, `run_report`

---

### 3. **Invoice Management** (4 queries)
- "Show all outstanding invoices"
- "List unpaid invoices"
- "What invoices are overdue?"
- "Show recent sales invoices"

**Expected Tools:** `list_documents` (with filters)

---

### 4. **Account Health & Status** (4 queries)
- "Show me accounts receivable report"
- "Which customers have the highest outstanding balance?"
- "Get customer ledger summary"
- "Show me aging report"

**Expected Tools:** `run_report`, `aggregate_documents`

---

### 5. **Contextual & Configuration** (4 queries)
- "What's the default currency?"
- "Give me the current company details"
- "Show me the company information"
- "What's my company name?"

**Expected Tools:** `list_documents` (smart contextual handling)

---

### 6. **Contact Information** (3 queries)
- "Show me contact details for customer XYZ"
- "List all customer emails"
- "Who is my primary contact at Acme Corp?"

**Expected Tools:** `get_document`, `list_documents`

---

### 7. **Territory & Segmentation** (3 queries)
- "Show customers in North America"
- "List all Enterprise customers"
- "How many customers do I have by territory?"

**Expected Tools:** `list_documents` (with filters), `aggregate_documents`

---

### 8. **Opportunities & Pipeline** (4 queries)
- "Show open opportunities"
- "What deals are in my pipeline?"
- "List all quotations"
- "Show me pending sales orders"

**Expected Tools:** `list_documents`

---

### 9. **Time-Based Analysis** (3 queries)
- "What were my sales last quarter?"
- "Show this year's revenue"
- "Compare this month vs last month sales"

**Expected Tools:** `aggregate_documents`, `run_report`

---

### 10. **Financial Reports** (4 queries)
- "Get profit and loss report"
- "Show me the balance sheet"
- "Run sales analytics report"
- "Generate customer ledger report"

**Expected Tools:** `run_report`

---

## What to Look For

### ✅ Good Response Indicators

1. **Correct Tool Selection**
   - List queries → `list_documents`
   - Aggregations → `aggregate_documents`
   - Reports → `run_report`
   - Specific items → `get_document` or `search_documents`

2. **Data Quality**
   - `high` or `medium` - good!
   - `low` - might be empty data (check if ERPNext has data)
   - `error` - check error message
   - `rate_limited` - wait a few minutes

3. **Formatted Response**
   - Should be human-readable (not raw JSON)
   - Tables when appropriate
   - Lists with bullet points
   - Clear summaries

### ❌ Issues to Watch For

1. **Wrong Tool Called**
   - "list customers" → calls `get_document` ❌
   - Should call `list_documents` ✅

2. **Generic Responses**
   - "I understand you're asking about ERPNext data..."
   - Usually means: fallback routing or rate limit

3. **Rate Limit Messages**
   - "⚠️ AI service temporarily unavailable..."
   - Wait 2-3 minutes and retry

4. **Empty Data**
   - Check if ERPNext actually has data for that query
   - Some reports need authentication

---

## Interpreting Results

### Example Good Response
```json
{
  "tools_called": ["list_documents"],
  "data_quality": "medium",
  "response": "Here are your customers:\n- Acme Corp\n- TechCo Inc\n\nTotal: 2 customers"
}
```
✅ Correct tool, formatted nicely, clear answer

### Example Rate Limited
```json
{
  "tools_called": [],
  "data_quality": "rate_limited",
  "response": "⚠️ The AI service is temporarily unavailable..."
}
```
⚠️ Wait a few minutes, or switch to Ollama

### Example Empty Data
```json
{
  "tools_called": ["list_documents"],
  "data_quality": "low",
  "response": "No customers found in the system."
}
```
ℹ️ ERPNext has no data - add some test data first

### Example Error
```json
{
  "tools_called": ["run_report"],
  "data_quality": "error",
  "response": "The report could not be executed. This might be because:\n- Authentication required..."
}
```
🔧 Check Frappe API credentials in config.yaml

---

## Prerequisites

### 1. ERPNext Running
```bash
# Check if ERPNext is accessible
curl -s http://localhost:8000 | grep "ERPNext"
```

### 2. MCP Server Running
```bash
# Check health
curl -s http://localhost:8080/health
# Should return: {"status":"healthy",...}
```

### 3. Groq API Key Configured (or Ollama)
Check `config.yaml`:
```yaml
llm:
  base_url: "https://api.groq.com/openai/v1"
  api_key: "gsk_..."
  model: "llama-3.3-70b-versatile"
```

### 4. ERPNext Has Test Data
- At least a few customers
- Some sales invoices
- A company configured

---

## Customizing Tests

### Test Specific Category Only
Edit `test_account_manager.sh` and comment out categories:

```bash
# Comment this out to skip
# test_query "Show me all my customers" "Customer Listing"

# Or run specific test
test_query "What's my total revenue?" "Revenue"
```

### Add Your Own Queries
```bash
test_query "Your custom query here" "Custom Category"
```

### Save Results to File
```bash
./test_account_manager.sh > test_results.txt 2>&1
```

---

## Troubleshooting

### Issue: All queries return "rate limited"
**Solution:** Wait 2-3 minutes, or switch to Ollama:
```yaml
# In config.yaml
llm:
  base_url: "http://ollama:11434/v1"
  model: "llama3.1:latest"
```

### Issue: "Error: Connection refused"
**Solution:** Make sure containers are running:
```bash
docker compose ps
docker compose up -d
```

### Issue: All responses say "no data found"
**Solution:** Add test data to ERPNext:
1. Login to http://localhost:8000
2. Create customers, invoices, etc.
3. Retry tests

### Issue: Wrong tool being called
**Solution:** Check preprocessing vs LLM:
- Simple queries ("list X") should be instant (preprocessing)
- Complex queries use LLM
- If consistently wrong, may need prompt tuning

---

## Performance Benchmarks

With **Groq** (llama-3.3-70b):
- Simple list queries: **0.3-0.5s** (preprocessing)
- Complex queries: **0.5-1.0s** (LLM)
- Aggregations: **0.8-1.2s**
- Reports: **0.5-1.0s**

With **Ollama** (llama3.1:latest local):
- Simple queries: **0.5-1.0s**
- Complex queries: **2-5s**
- Aggregations: **3-8s**
- Reports: **2-5s**

---

## Next Steps

After running tests:

1. **Review Output**
   - Which categories work well?
   - Which need improvement?
   - Any consistent errors?

2. **Check Logs**
   ```bash
   docker compose logs frappe-mcp-server --tail=50
   ```

3. **Iterate**
   - Add more test queries
   - Tune prompts if needed
   - Add missing doctypes

4. **Production Readiness**
   - Deploy with confidence once tests pass
   - Monitor rate limits
   - Set up error alerts

---

**Created:** November 15, 2025  
**Purpose:** Local testing with real-world Account Manager queries  
**Status:** Ready to run!

