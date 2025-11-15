package server

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// formatFrappeError extracts meaningful error info and formats it user-friendly
func formatFrappeError(errorMsg, userQuery string) string {
	// Extract the core error message from Frappe's verbose error format
	coreError := extractCoreErrorMessage(errorMsg)
	
	// Return the clean, user-friendly message
	return formatUserFriendlyError(coreError, userQuery)
}

// extractCoreErrorMessage extracts the actual error from Frappe's verbose format
func extractCoreErrorMessage(errorMsg string) string {
	// Try to extract from JSON structure first
	if strings.Contains(errorMsg, "Raw response:") {
		// Extract JSON part
		jsonStart := strings.Index(errorMsg, "{")
		if jsonStart != -1 {
			jsonPart := errorMsg[jsonStart:]
			
			var errorData map[string]interface{}
			if err := json.Unmarshal([]byte(jsonPart), &errorData); err == nil {
				// Extract the exception message (the human-readable part)
				if exception, ok := errorData["exception"].(string); ok {
					// Format: "module.ExceptionType: Actual error message"
					parts := strings.SplitN(exception, ":", 2)
					if len(parts) == 2 {
						return strings.TrimSpace(parts[1])
					}
					return exception
				}
				
				// Try _server_messages as fallback
				if serverMsgs, ok := errorData["_server_messages"].(string); ok {
					var msgs []map[string]interface{}
					if err := json.Unmarshal([]byte(serverMsgs), &msgs); err == nil {
						if len(msgs) > 0 {
							if msg, ok := msgs[0]["message"].(string); ok {
								return msg
							}
						}
					}
				}
			}
		}
	}
	
	// Fallback: try to extract from pattern "Error: actual message"
	patterns := []string{
		`Error:\s*(.+?)(?:\n|$)`,
		`Exception:\s*(.+?)(?:\n|$)`,
		`Message:\s*(.+?)(?:\n|$)`,
	}
	
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(errorMsg); len(matches) > 1 {
			return strings.TrimSpace(matches[1])
		}
	}
	
	// Last resort: return first line if it looks like an error message
	lines := strings.Split(errorMsg, "\n")
	if len(lines) > 0 {
		firstLine := strings.TrimSpace(lines[0])
		// Remove "Error processing query:" prefix if present
		firstLine = strings.TrimPrefix(firstLine, "Error processing query:")
		firstLine = strings.TrimSpace(firstLine)
		return firstLine
	}
	
	return errorMsg
}

// formatUserFriendlyError creates a conversational error message
func formatUserFriendlyError(errorMsg, userQuery string) string {
	errorLower := strings.ToLower(errorMsg)
	
	var response strings.Builder
	response.WriteString("❌ **I couldn't complete that request**\n\n")
	
	// Detect specific error types and provide targeted help
	if strings.Contains(errorLower, "fiscal year") {
		response.WriteString("**The issue**: The dates you specified aren't in an active fiscal year.\n\n")
		response.WriteString("**What this means**: Your ERPNext system needs fiscal years to be set up before running financial reports.\n\n")
		response.WriteString("**How to fix it**:\n")
		response.WriteString("1. Go to **Accounts → Fiscal Year** in ERPNext\n")
		response.WriteString("2. Create a fiscal year that includes your date range\n")
		response.WriteString("3. Or try using dates within an existing fiscal year\n\n")
		response.WriteString("💡 **Tip**: Ask your finance team or admin about which fiscal years are active.")
		
	} else if strings.Contains(errorLower, "authentication") || strings.Contains(errorLower, "unauthorized") || strings.Contains(errorLower, "401") {
		response.WriteString("**The issue**: I couldn't authenticate with your ERPNext system.\n\n")
		response.WriteString("**How to fix it**:\n")
		response.WriteString("1. Check that your API credentials are correct\n")
		response.WriteString("2. Verify the API key hasn't expired\n")
		response.WriteString("3. Make sure you have permission to access this data\n\n")
		response.WriteString("💡 **Need help?** Contact your ERPNext administrator.")
		
	} else if strings.Contains(errorLower, "permission") || strings.Contains(errorLower, "forbidden") || strings.Contains(errorLower, "403") {
		response.WriteString("**The issue**: You don't have permission to access this information.\n\n")
		response.WriteString("**How to fix it**:\n")
		response.WriteString("1. Ask your administrator to grant you the necessary permissions\n")
		response.WriteString("2. Try accessing different data you have permission for\n")
		response.WriteString("3. Log in with a different account that has the right access\n\n")
		response.WriteString("💡 **Tip**: Different reports require different permission levels.")
		
	} else if strings.Contains(errorLower, "not found") || strings.Contains(errorLower, "404") || strings.Contains(errorLower, "does not exist") {
		response.WriteString("**The issue**: The item you're looking for doesn't exist.\n\n")
		response.WriteString("**How to fix it**:\n")
		response.WriteString("1. Check the spelling of names or IDs\n")
		response.WriteString("2. Verify the item hasn't been deleted\n")
		response.WriteString("3. Try searching for similar items\n\n")
		response.WriteString(fmt.Sprintf("💡 **Tip**: Try asking \"List all [items]\" to see what's available."))
		
	} else if strings.Contains(errorLower, "mandatory") || strings.Contains(errorLower, "required") {
		response.WriteString("**The issue**: Some required information is missing.\n\n")
		response.WriteString("**How to fix it**:\n")
		response.WriteString("1. Make sure you've provided all required details\n")
		response.WriteString("2. Try adding company name, dates, or other filters\n")
		response.WriteString("3. Ask me \"What do I need for [report name]?\"\n\n")
		response.WriteString("💡 **Tip**: Most financial reports need company and date range.")
		
	} else if strings.Contains(errorLower, "validation") || strings.Contains(errorLower, "invalid") {
		response.WriteString("**The issue**: Some of the information provided isn't valid.\n\n")
		response.WriteString("**How to fix it**:\n")
		response.WriteString("1. Check date formats (try YYYY-MM-DD)\n")
		response.WriteString("2. Verify company/customer names are spelled correctly\n")
		response.WriteString("3. Make sure numerical values are in the right format\n\n")
		response.WriteString("💡 **Tip**: Try rephrasing your question or providing different values.")
		
	} else {
		// Generic error - show the cleaned message
		response.WriteString(fmt.Sprintf("**What happened**: %s\n\n", errorMsg))
		response.WriteString("**How to fix it**:\n")
		response.WriteString("1. Try rephrasing your question\n")
		response.WriteString("2. Check if all required information is provided\n")
		response.WriteString("3. Verify the data exists in your system\n\n")
		response.WriteString("💡 **Need help?** Try asking me how to do what you need.")
	}
	
	return response.String()
}

// formatDataWithoutLLM provides smart fallback formatting when LLM is unavailable
func formatDataWithoutLLM(userQuery, rawData string) string {
	// Try to parse as JSON
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(rawData), &data); err != nil {
		// Not JSON, check if it's already formatted text
		if strings.Contains(rawData, "Error") || strings.Contains(rawData, "error") {
			return formatErrorMessage(rawData)
		}
		// Return as-is but make it conversational
		return fmt.Sprintf("Here's what I found:\n\n%s", rawData)
	}

	// Detect format from user query
	formatType := detectRequestedFormat(userQuery)

	// Try report format first (most common)
	if formatted := tryFormatAsReport(data, formatType); formatted != "" {
		return formatted
	}

	// Try list format
	if formatted := tryFormatAsList(data, formatType); formatted != "" {
		return formatted
	}

	// Try single document format
	if formatted := tryFormatAsDocument(data); formatted != "" {
		return formatted
	}

	// Last resort: structured summary
	return formatAsStructuredSummary(data, userQuery)
}

// detectRequestedFormat determines what format the user wants
func detectRequestedFormat(query string) string {
	queryLower := strings.ToLower(query)
	
	if strings.Contains(queryLower, "table") || strings.Contains(queryLower, "in table format") {
		return "table"
	}
	if strings.Contains(queryLower, "list") || strings.Contains(queryLower, "bullet") {
		return "list"
	}
	if strings.Contains(queryLower, "summary") || strings.Contains(queryLower, "summarize") {
		return "summary"
	}
	
	return "auto" // Let the system decide
}

// tryFormatAsReport handles ERPNext report format (columns + data)
func tryFormatAsReport(data map[string]interface{}, formatType string) string {
	// Check for report structure
	columns, hasColumns := data["columns"].([]interface{})
	rows, hasRows := data["data"].([]interface{})
	
	if !hasColumns || !hasRows {
		return ""
	}

	// Extract report name if available
	reportName := ""
	if name, ok := data["report_name"].(string); ok {
		reportName = name
	}

	// Build response
	var response strings.Builder
	
	if reportName != "" {
		response.WriteString(fmt.Sprintf("📊 **%s**\n\n", reportName))
	} else {
		response.WriteString("📊 **Report Results**\n\n")
	}

	// Check if empty
	if len(rows) == 0 {
		response.WriteString("No data found for the specified criteria.\n\n")
		response.WriteString("💡 Try adjusting your filters or date range.")
		return response.String()
	}

	// Format as table
	table := buildMarkdownTable(columns, rows)
	response.WriteString(table)
	
	// Add summary
	response.WriteString(fmt.Sprintf("\n\n📈 **Summary**: %d row(s) returned", len(rows)))
	
	return response.String()
}

// buildMarkdownTable creates a markdown table from columns and rows
func buildMarkdownTable(columns []interface{}, rows []interface{}) string {
	if len(columns) == 0 || len(rows) == 0 {
		return "No data available."
	}

	var table strings.Builder
	
	// Extract column headers
	headers := []string{}
	for _, col := range columns {
		if colMap, ok := col.(map[string]interface{}); ok {
			if label, ok := colMap["label"].(string); ok {
				headers = append(headers, label)
			} else if fieldname, ok := colMap["fieldname"].(string); ok {
				headers = append(headers, fieldname)
			}
		}
	}

	// Limit columns to avoid overwhelming display
	maxCols := 8
	displayCols := len(headers)
	if displayCols > maxCols {
		displayCols = maxCols
	}

	// Build header row
	table.WriteString("|")
	for i := 0; i < displayCols; i++ {
		table.WriteString(fmt.Sprintf(" %s |", headers[i]))
	}
	table.WriteString("\n")

	// Build separator row
	table.WriteString("|")
	for i := 0; i < displayCols; i++ {
		table.WriteString("---------|")
	}
	table.WriteString("\n")

	// Build data rows (limit to prevent overwhelming output)
	maxRows := 20
	displayRows := len(rows)
	if displayRows > maxRows {
		displayRows = maxRows
	}

	for i := 0; i < displayRows; i++ {
		if rowData, ok := rows[i].([]interface{}); ok {
			table.WriteString("|")
			for j := 0; j < displayCols && j < len(rowData); j++ {
				cellValue := formatCellValue(rowData[j])
				table.WriteString(fmt.Sprintf(" %s |", cellValue))
			}
			table.WriteString("\n")
		}
	}

	// Add note if truncated
	if len(rows) > maxRows {
		table.WriteString(fmt.Sprintf("\n*...and %d more rows*", len(rows)-maxRows))
	}
	if len(headers) > maxCols {
		table.WriteString(fmt.Sprintf(" (showing %d of %d columns)", displayCols, len(headers)))
	}

	return table.String()
}

// formatCellValue formats a cell value for display
func formatCellValue(value interface{}) string {
	if value == nil {
		return "-"
	}

	switch v := value.(type) {
	case string:
		// Truncate long strings
		if len(v) > 50 {
			return v[:47] + "..."
		}
		return v
	case float64:
		// Format numbers nicely
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%.2f", v)
	case bool:
		if v {
			return "✓"
		}
		return "✗"
	default:
		str := fmt.Sprintf("%v", v)
		if len(str) > 50 {
			return str[:47] + "..."
		}
		return str
	}
}

// tryFormatAsList handles list/array format
func tryFormatAsList(data map[string]interface{}, formatType string) string {
	// Check for list structure
	items, hasData := data["data"].([]interface{})
	if !hasData {
		return ""
	}

	if len(items) == 0 {
		return "No items found matching your criteria.\n\n💡 Try adjusting your search terms."
	}

	var response strings.Builder
	response.WriteString(fmt.Sprintf("Found %d item(s):\n\n", len(items)))

	// Limit display
	maxItems := 20
	displayCount := len(items)
	if displayCount > maxItems {
		displayCount = maxItems
	}

	for i := 0; i < displayCount; i++ {
		if itemMap, ok := items[i].(map[string]interface{}); ok {
			// Extract key fields
			name := extractField(itemMap, "name", "id", "title")
			description := extractField(itemMap, "description", "company_name", "customer_name", "title")
			
			if name != "" {
				response.WriteString(fmt.Sprintf("• **%s**", name))
				if description != "" && description != name {
					response.WriteString(fmt.Sprintf(": %s", description))
				}
				response.WriteString("\n")
			} else {
				// Show first few fields
				fieldCount := 0
				response.WriteString("• ")
				for key, val := range itemMap {
					if fieldCount >= 3 {
						break
					}
					response.WriteString(fmt.Sprintf("%s: %v, ", key, val))
					fieldCount++
				}
				response.WriteString("\n")
			}
		}
	}

	if len(items) > maxItems {
		response.WriteString(fmt.Sprintf("\n*...and %d more items*", len(items)-maxItems))
	}

	return response.String()
}

// extractField tries to extract a field from multiple possible keys
func extractField(data map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if val, ok := data[key]; ok {
			if str, ok := val.(string); ok && str != "" {
				return str
			}
		}
	}
	return ""
}

// tryFormatAsDocument handles single document display
func tryFormatAsDocument(data map[string]interface{}) string {
	// Check if it looks like a single document (has name, doctype, etc.)
	if _, hasName := data["name"]; !hasName {
		return ""
	}

	var response strings.Builder
	
	// Extract key information
	doctype := extractField(data, "doctype", "type")
	name := extractField(data, "name", "id")
	
	if doctype != "" && name != "" {
		response.WriteString(fmt.Sprintf("📄 **%s**: %s\n\n", doctype, name))
	} else if name != "" {
		response.WriteString(fmt.Sprintf("📄 **%s**\n\n", name))
	}

	// Display key fields
	keyFields := []string{"status", "company", "customer", "supplier", "project", "title", "description"}
	for _, field := range keyFields {
		if val := extractField(data, field); val != "" {
			response.WriteString(fmt.Sprintf("**%s**: %s\n", strings.Title(field), val))
		}
	}

	// Show field count
	response.WriteString(fmt.Sprintf("\n*Document has %d fields total*", len(data)))

	return response.String()
}

// formatAsStructuredSummary provides a last-resort readable format
func formatAsStructuredSummary(data map[string]interface{}, userQuery string) string {
	var response strings.Builder
	
	response.WriteString("📋 **Query Results**\n\n")
	
	// Count items/records
	if items, ok := data["data"].([]interface{}); ok {
		response.WriteString(fmt.Sprintf("Found %d record(s)\n\n", len(items)))
		
		if len(items) > 0 {
			response.WriteString("**Sample data**:\n")
			// Show first item as example
			if itemMap, ok := items[0].(map[string]interface{}); ok {
				count := 0
				for key, val := range itemMap {
					if count >= 5 {
						break
					}
					response.WriteString(fmt.Sprintf("- %s: %v\n", key, val))
					count++
				}
			}
		}
	} else {
		// Show top-level fields
		response.WriteString("**Data structure**:\n")
		count := 0
		for key, val := range data {
			if count >= 5 {
				break
			}
			response.WriteString(fmt.Sprintf("- %s: %v\n", key, val))
			count++
		}
	}

	response.WriteString("\n💡 For better formatting, try asking in a more specific way.")
	
	return response.String()
}

// formatErrorMessage makes error messages user-friendly
func formatErrorMessage(errorText string) string {
	// Extract key error information
	errorLower := strings.ToLower(errorText)
	
	var response strings.Builder
	response.WriteString("❌ **Oops! Something went wrong**\n\n")
	
	// Provide context-specific help
	if strings.Contains(errorLower, "authentication") || strings.Contains(errorLower, "unauthorized") {
		response.WriteString("**Issue**: Authentication failed\n\n")
		response.WriteString("**What you can do**:\n")
		response.WriteString("- Check your API credentials\n")
		response.WriteString("- Verify your ERPNext permissions\n")
	} else if strings.Contains(errorLower, "not found") {
		response.WriteString("**Issue**: The requested item wasn't found\n\n")
		response.WriteString("**What you can do**:\n")
		response.WriteString("- Check the spelling of names/IDs\n")
		response.WriteString("- Verify the item exists in your system\n")
	} else if strings.Contains(errorLower, "permission") || strings.Contains(errorLower, "forbidden") {
		response.WriteString("**Issue**: Permission denied\n\n")
		response.WriteString("**What you can do**:\n")
		response.WriteString("- Ask your administrator for access\n")
		response.WriteString("- Try a different query you have permission for\n")
	} else {
		response.WriteString("**Issue**: The query couldn't be completed\n\n")
		response.WriteString("**What you can do**:\n")
		response.WriteString("- Try rephrasing your question\n")
		response.WriteString("- Check if all required information is provided\n")
	}
	
	// Show technical details in a collapsed way
	response.WriteString("\n<details>\n<summary>Technical details</summary>\n\n")
	response.WriteString(fmt.Sprintf("```\n%s\n```\n", errorText))
	response.WriteString("</details>")
	
	return response.String()
}

// formatEmptyResult provides a helpful message for empty results
func formatEmptyResult(query string) string {
	return fmt.Sprintf(`No results found for: "%s"

💡 **Suggestions**:
- Check if the filters are too restrictive
- Try a broader date range
- Verify the spelling of names or IDs
- Make sure the data exists in your system

Need help? Try asking:
- "List all [items]" to see what's available
- "Show me [report] without filters"
- "What companies are in the system?"`, query)
}

