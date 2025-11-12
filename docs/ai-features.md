# AI-Powered Features

ERPNext MCP Server leverages local AI (Ollama) for intelligent query processing and entity extraction.

## Natural Language Query Processing

Ask questions in plain English - the AI extracts intent, doctype, and entities.

### Examples

```bash
# List queries
"Show me all projects"
"List open sales orders"
"What customers do we have?"

# Get specific documents
"Show me project PROJ-0001"
"Get customer CUST-12345"
"Display task TASK-2024-001"

# Search queries
"Find projects for customer ABC Corp"
"Show invoices from last month"
"List pending tasks for John"
```

## How It Works

```
User Query: "Show me details of project PROJ-0001"
     ↓
┌──────────────────────────────────┐
│   AI Intent Extraction (Ollama) │
└────────────┬─────────────────────┘
             ↓
    {
      action: "get",
      doctype: "Project",
      entity_name: "PROJ-0001",
      tool: "get_document"
    }
     ↓
┌──────────────────────────────────┐
│  Execute Tool with Parameters    │
└────────────┬─────────────────────┘
             ↓
┌──────────────────────────────────┐
│     Fetch from ERPNext API       │
└────────────┬─────────────────────┘
             ↓
    Return structured data
```

## AI Prompt Engineering

The server uses carefully crafted prompts to extract:

### 1. **Action** (What to do)
- `get` - Fetch specific document
- `list` - Get all documents
- `search` - Find documents by criteria
- `analyze` - Deep analysis with related docs
- `create`, `update`, `delete` - CRUD operations

### 2. **DocType** (What entity)
Automatically maps common terms to ERPNext doctypes:
- "projects" → `Project`
- "customers" → `Customer`
- "sales orders" → `Sales Order`
- "invoices" → `Sales Invoice`

### 3. **Entity Name/ID**
Extracts specific identifiers:
- Project codes: `PROJ-0001`, `PROJECT-123`
- Customer IDs: `CUST-456`, `CUSTOMER-789`
- Generic IDs: `TASK-2024-001`

### 4. **Search Requirements**
Determines if search is needed before action.

## Generic Document Support

**Key Feature**: Works with ANY ERPNext doctype without hardcoding!

### Standard DocTypes
- Project, Task, Issue
- Customer, Supplier, Lead
- Sales Order, Purchase Order
- Sales Invoice, Purchase Invoice
- Item, Stock Entry
- Employee, User
- ... and all others

### Custom DocTypes
If you've created custom doctypes in ERPNext:

```
Query: "Show me custom sales lead CSL-001"
```

The AI will:
1. Extract `Custom Sales Lead` as doctype
2. Extract `CSL-001` as entity name
3. Call `get_document` with correct parameters

**No code changes needed!**

## Entity Extraction Strategies

### 1. AI-Powered (Primary)
Uses Ollama LLM to understand context and extract entities intelligently.

```
Query: "What's the status of the website redesign project?"
AI extracts: doctype=Project, search_query="website redesign"
```

### 2. Regex Fallback
For exact ERPNext ID patterns (e.g., `PROJ-0001`):

```
Query: "Show PROJ-0001"
Regex detects: Exact ID → skip search, direct get_document
```

### 3. Hybrid Approach
Combines both for best results:
- Use AI for natural language understanding
- Use regex for exact ID detection
- Fall back to simpler routing if AI fails

## Available Tools

### Basic CRUD

#### `get_document`
Fetch a specific document by name.

```json
{
  "doctype": "Project",
  "name": "PROJ-0001"
}
```

#### `list_documents`
List all documents of a type.

```json
{
  "doctype": "Customer",
  "limit": 20,
  "filters": {"status": "Active"}
}
```

#### `search_documents`
Search documents by query.

```json
{
  "doctype": "Project",
  "query": "website redesign",
  "limit": 10
}
```

#### `create_document`
Create a new document.

```json
{
  "doctype": "Task",
  "data": {
    "subject": "New Task",
    "status": "Open"
  }
}
```

#### `update_document`
Update an existing document.

```json
{
  "doctype": "Project",
  "name": "PROJ-0001",
  "data": {
    "status": "Completed"
  }
}
```

#### `delete_document`
Delete a document.

```json
{
  "doctype": "Task",
  "name": "TASK-001"
}
```

### Advanced Analysis

#### `analyze_document`
**Generic tool** that works with ANY doctype!

Fetches document and optionally related documents:

```json
{
  "doctype": "Project",
  "name": "PROJ-0001",
  "include_related": true
}
```

Returns:
- Main document
- Related tasks
- Related timesheets
- Related documents (generic relationship discovery)

### Project-Specific Tools

#### `get_project_status`
Comprehensive project analysis.

```json
{
  "project_name": "PROJ-0001"
}
```

#### `portfolio_dashboard`
Portfolio-wide metrics.

```json
{
  "status_filter": "Open"
}
```

#### `analyze_project_timeline`
Timeline analysis and delay detection.

## Configuration

### Model Selection

Choose based on your needs:

```yaml
ollama:
  # Fast, good for simple queries
  model: "llama3.2:1b"
  
  # OR: More accurate, slower
  # model: "llama3.1"
```

### Timeout Adjustment

For complex queries:

```yaml
ollama:
  timeout: "120s"  # Longer for complex analysis
```

## Usage Examples

### In Cursor IDE

```
@erpnext List all open projects

@erpnext Show me customer ABC-CORP

@erpnext What are the pending tasks?

@erpnext Analyze project PROJ-0001
```

### HTTP API

```bash
# Natural language query
curl -X POST http://localhost:8080/api/v1/chat \
  -H "Content-Type: application/json" \
  -d '{
    "message": "Show me all customers created this month"
  }'

# Direct tool call
curl -X POST http://localhost:8080/api/v1/tool/get_document \
  -H "Content-Type: application/json" \
  -d '{
    "doctype": "Project",
    "name": "PROJ-0001"
  }'
```

## Best Practices

### 1. Be Specific
```
❌ "Show project"
✅ "Show project PROJ-0001"
```

### 2. Use Natural Language
```
✅ "What are the open sales orders?"
✅ "List customers created this week"
✅ "Show me the status of project ABC"
```

### 3. Include Context
```
❌ "Status?"
✅ "What's the status of project PROJ-0001?"
```

### 4. Use Exact IDs When Known
```
✅ "Show PROJ-0001"  (fastest - direct get)
✅ "Show website redesign project"  (uses search)
```

## Troubleshooting

### AI Not Understanding Queries

**Check Ollama**:
```bash
curl http://localhost:11434/api/tags
```

**Try different model**:
```yaml
ollama:
  model: "llama3.1"  # More capable
```

### Wrong DocType Extracted

Use explicit names:
```
Instead of: "show sales"
Use: "show sales orders"
```

### Slow Responses

- Use smaller model (`llama3.2:1b`)
- Reduce timeout for faster failures
- Consider caching (future feature)

## Privacy & Security

All AI processing happens **locally** via Ollama:
- ✅ No data sent to external AI services
- ✅ No API keys for OpenAI/Anthropic needed
- ✅ Complete data privacy
- ✅ Works offline (after model download)

Next: [API Reference](api-reference.md)

