package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"frappe-mcp-server/internal/utils"

	"github.com/ollama/ollama/api"
)

// MCPRequest represents a JSON-RPC 2.0 request to the MCP server
type MCPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

// MCPResponse represents a JSON-RPC 2.0 response from the MCP server
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// MCPError represents an MCP error
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Tool represents an MCP tool
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

// ToolsListResult represents the result of tools/list
type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

// ToolCallParams represents parameters for tools/call
type ToolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// ToolCallResult represents the result of tools/call
type ToolCallResult struct {
	Content []ToolContent `json:"content"`
}

// ToolContent represents tool call content
type ToolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// OllamaERPNextClient manages the integration between Ollama and ERPNext MCP server
type OllamaERPNextClient struct {
	mcpServerPath string
	ollamaModel   string
	mcpProcess    *exec.Cmd
	mcpStdin      io.WriteCloser
	mcpStdout     *bufio.Scanner
	requestID     int
	tools         []Tool
	ollamaClient  *api.Client
	debugMode     bool // New: enable debug/raw mode
}

// New: Enable debug logging globally
func init() {
	// Enable detailed debug logging with timestamps and file info
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("[DEBUG] Logging initialized: timestamps and file info enabled")
}

// NewOllamaERPNextClient creates a new client
func NewOllamaERPNextClient(mcpServerPath, ollamaModel string, debugMode bool) *OllamaERPNextClient {
	log.Printf("[INIT] Creating OllamaERPNextClient | MCP: %s | Model: %s | Debug: %v", mcpServerPath, ollamaModel, debugMode)

	// Get Ollama host from environment or use default
	ollamaHost := os.Getenv("OLLAMA_HOST")
	if ollamaHost == "" {
		ollamaHost = "http://localhost:11434"
	}

	// Parse the URL to ensure it's valid
	ollamaURL, err := url.Parse(ollamaHost)
	if err != nil {
		log.Printf("Invalid OLLAMA_HOST URL: %v, using default", err)
		ollamaURL, _ = url.Parse("http://localhost:11434")
	}

	// Create client with proper URL
	client := api.NewClient(ollamaURL, http.DefaultClient)

	return &OllamaERPNextClient{
		mcpServerPath: mcpServerPath,
		ollamaModel:   ollamaModel,
		requestID:     1,
		ollamaClient:  client,
		debugMode:     debugMode,
	}
}

// StartMCPServer starts the MCP server subprocess
func (c *OllamaERPNextClient) StartMCPServer() error {
	log.Printf("[DEBUG] Starting MCP server subprocess: %s", c.mcpServerPath)

	// Start the MCP server process
	c.mcpProcess = exec.Command(c.mcpServerPath)
	c.mcpProcess.Stderr = os.Stderr // Forward stderr for debugging

	stdin, err := c.mcpProcess.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	c.mcpStdin = stdin

	stdout, err := c.mcpProcess.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	c.mcpStdout = bufio.NewScanner(stdout)

	if err := c.mcpProcess.Start(); err != nil {
		log.Printf("[ERROR] Failed to start MCP server: %v", err)
		return fmt.Errorf("failed to start MCP server: %w", err)
	}
	log.Printf("[DEBUG] MCP server process started (PID: %d)", c.mcpProcess.Process.Pid)

	// Initialize the MCP server
	initRequest := MCPRequest{
		JSONRPC: "2.0",
		ID:      c.requestID,
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"roots":    map[string]bool{"listChanged": true},
				"sampling": map[string]interface{}{},
			},
			"clientInfo": map[string]string{
				"name":    "ollama-mcp-client",
				"version": "1.0.0",
			},
		},
	}

	log.Printf("[DEBUG] Sending MCP initialize request: %+v", initRequest)
	_, err = c.sendMCPRequest(initRequest)
	if err != nil {
		log.Printf("[ERROR] Failed to initialize MCP server: %v", err)
		return fmt.Errorf("failed to initialize MCP server: %w", err)
	}
	c.requestID++

	// Get available tools
	toolsRequest := MCPRequest{
		JSONRPC: "2.0",
		ID:      c.requestID,
		Method:  "tools/list",
		Params:  map[string]interface{}{},
	}

	log.Printf("[DEBUG] Sending MCP tools/list request: %+v", toolsRequest)
	response, err := c.sendMCPRequest(toolsRequest)
	if err != nil {
		log.Printf("[ERROR] Failed to get tools list: %v", err)
		return fmt.Errorf("failed to get tools list: %w", err)
	}

	if response.Result != nil {
		resultBytes, _ := json.Marshal(response.Result)
		var toolsResult ToolsListResult
		if err := json.Unmarshal(resultBytes, &toolsResult); err == nil {
			c.tools = toolsResult.Tools
			log.Printf("[DEBUG] Loaded %d ERPNext tools from MCP server", len(c.tools))
			fmt.Printf("✅ Loaded %d ERPNext tools\n", len(c.tools))
		}
	}

	c.requestID++
	return nil
}

// sendMCPRequest sends a request to the MCP server and returns the response
func (c *OllamaERPNextClient) sendMCPRequest(request MCPRequest) (*MCPResponse, error) {
	requestBytes, err := json.Marshal(request)
	if err != nil {
		log.Printf("[ERROR] Failed to marshal MCP request: %v", err)
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	log.Printf("[DEBUG] MCP Request (ID %d): %s", request.ID, string(requestBytes))

	// Send request
	_, err = c.mcpStdin.Write(append(requestBytes, '\n'))
	if err != nil {
		log.Printf("[ERROR] Failed to write to MCP server: %v", err)
		return nil, fmt.Errorf("failed to write to MCP server: %w", err)
	}

	// Read response
	if !c.mcpStdout.Scan() {
		log.Printf("[ERROR] Failed to read from MCP server (ID %d)", request.ID)
		return nil, fmt.Errorf("failed to read from MCP server")
	}
	log.Printf("[DEBUG] MCP Response (ID %d): %s", request.ID, string(c.mcpStdout.Bytes()))

	var response MCPResponse
	if err := json.Unmarshal(c.mcpStdout.Bytes(), &response); err != nil {
		log.Printf("[ERROR] Failed to unmarshal MCP response: %v", err)
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &response, nil
} // CallTool calls an ERPNext tool via MCP
func (c *OllamaERPNextClient) CallTool(toolName string, arguments map[string]interface{}) (string, error) {
	log.Printf("[DEBUG] Calling tool: %s with arguments: %+v", toolName, arguments)

	request := MCPRequest{
		JSONRPC: "2.0",
		ID:      c.requestID,
		Method:  "tools/call",
		Params: ToolCallParams{
			Name:      toolName,
			Arguments: arguments,
		},
	}

	response, err := c.sendMCPRequest(request)
	if err != nil {
		log.Printf("[ERROR] MCP tool call failed: %v", err)
		return "", err
	}
	c.requestID++

	if response.Error != nil {
		log.Printf("[ERROR] Tool error from MCP: %s", response.Error.Message)
		return "", fmt.Errorf("tool error: %s", response.Error.Message)
	}

	if response.Result != nil {
		log.Printf("[DEBUG] Tool %s returned result: %+v", toolName, response.Result)

		// First convert the result to bytes properly
		resultBytes, err := json.Marshal(response.Result)
		if err != nil {
			log.Printf("[ERROR] Error marshaling tool result: %v", err)
			return fmt.Sprintf("%v", response.Result), nil
		}

		// DEBUG: Log the raw JSON result for inspection
		log.Printf("[DEBUG] Tool %s raw JSON result: %s", toolName, string(resultBytes))

		var toolResult ToolCallResult
		if err := json.Unmarshal(resultBytes, &toolResult); err == nil {
			if len(toolResult.Content) > 0 {
				log.Printf("[DEBUG] Tool %s content parts: %d", toolName, len(toolResult.Content))

				// Combine all content parts for a complete response
				var contentParts []string
				for i, content := range toolResult.Content {
					if content.Text != "" {
						// DEBUG: Log each content part
						log.Printf("[DEBUG] Tool %s content part %d: %s", toolName, i, content.Text)

						// Try to pretty-print JSON in content parts after the first one
						if i > 0 && strings.HasPrefix(content.Text, "{") {
							var jsonData interface{}
							if json.Unmarshal([]byte(content.Text), &jsonData) == nil {
								if prettyJSON, err := json.MarshalIndent(jsonData, "", "  "); err == nil {
									contentParts = append(contentParts, string(prettyJSON))
									continue
								}
							}
						}
						contentParts = append(contentParts, content.Text)
					}
				}
				result := strings.Join(contentParts, "\n\n")
				log.Printf("[DEBUG] Tool %s final combined result: %s", toolName, result)
				return result, nil
			}
		}
		// Fallback to string representation
		return fmt.Sprintf("%v", response.Result), nil
	}

	log.Printf("[WARN] Tool %s returned no response", toolName)
	return "No response from tool", nil
}

// GetToolsDescription returns a description of available tools for the LLM
func (c *OllamaERPNextClient) GetToolsDescription() string {
	var builder strings.Builder
	builder.WriteString("Available ERPNext tools:\n\n")

	for _, tool := range c.tools {
		builder.WriteString(fmt.Sprintf("**%s**: %s\n", tool.Name, tool.Description))

		// Add schema info if available
		if tool.InputSchema != nil {
			schemaBytes, _ := json.Marshal(tool.InputSchema)
			var schema map[string]interface{}
			if json.Unmarshal(schemaBytes, &schema) == nil {
				if props, ok := schema["properties"].(map[string]interface{}); ok {
					required, _ := schema["required"].([]interface{})
					requiredSet := make(map[string]bool)
					for _, r := range required {
						if str, ok := r.(string); ok {
							requiredSet[str] = true
						}
					}

					builder.WriteString("Parameters:\n")
					for propName, propInfo := range props {
						if info, ok := propInfo.(map[string]interface{}); ok {
							desc, _ := info["description"].(string)
							if desc == "" {
								desc = "No description"
							}
							reqMarker := ""
							if requiredSet[propName] {
								reqMarker = " (required)"
							}
							builder.WriteString(fmt.Sprintf("  - %s: %s%s\n", propName, desc, reqMarker))
						}
					}
				}
			}
		}
		builder.WriteString("\n")
	}

	return builder.String()
}

// ChatWithOllama sends a message to Ollama with tool context
func (c *OllamaERPNextClient) ChatWithOllama(ctx context.Context, userMessage string) (string, string, error) {
	log.Printf("[DEBUG] Sending prompt to Ollama (model: %s): %s", c.ollamaModel, userMessage)

	req := &api.ChatRequest{
		Model: c.ollamaModel,
		Messages: []api.Message{
			{
				Role: "system",
				Content: fmt.Sprintf(`You are an ERPNext Data Analysis Assistant that provides insights based STRICTLY on actual ERPNext data. You help users understand their real business data through natural language queries.

%s

🎯 DATA-DRIVEN PRINCIPLES:
- ONLY use actual data returned from ERPNext MCP tools
- NEVER create fictional metrics, insights, or recommendations
- Be transparent about data availability and limitations
- If data is insufficient, explicitly state what's missing
- Focus on facts, not speculation or assumptions

NATURAL LANGUAGE UNDERSTANDING:
You interpret business questions and map them to appropriate ERPNext data queries:

CASUAL LANGUAGE → DATA QUERIES:
• "What's up?" / "How's business?" → portfolio_dashboard (actual project data)
• "Any problems?" / "Everything ok?" → portfolio_dashboard + search for issues
• "Money situation?" → budget_variance_analysis (real financial data)
• "Team status?" → resource_utilization_analysis (actual team assignments)

TOOL SELECTION FOR REAL DATA:
- General status → portfolio_dashboard (real portfolio metrics)
- Specific projects → search_documents + get_document (actual project records)
- Financial queries → budget_variance_analysis (real budget data)
- Team questions → resource_utilization_analysis (actual resource data)
- Problems → search_documents with issue filters (real problem data)

DATA ANALYSIS APPROACH:
1. 🔍 QUERY ERPNext: Use appropriate tools to get actual data
2. 📊 ANALYZE FACTS: Only work with data that was actually returned
3. 🚨 IDENTIFY GAPS: Be clear about missing or insufficient data
4. 💡 DATA-BASED INSIGHTS: Provide recommendations only when data supports them
5. 🎯 ACTIONABLE NEXT STEPS: Suggest specific data queries if more info needed

RESPONSE STYLE:
- Lead with what data was actually found
- Be explicit about data sources and timestamps
- Highlight data quality and completeness
- Provide insights only when data is sufficient
- Suggest additional data queries when needed

CRITICAL RULE: Never provide business insights unless you have actual ERPNext data to support them. If you don't have enough data, say I can't help you with this query.
User question: %s`, c.GetToolsDescription(), userMessage),
			},
			{
				Role:    "user",
				Content: userMessage,
			},
		},
	}

	var responseContent strings.Builder
	err := c.ollamaClient.Chat(ctx, req, func(resp api.ChatResponse) error {
		log.Printf("[DEBUG] Ollama partial response: %s", resp.Message.Content)
		responseContent.WriteString(resp.Message.Content)
		return nil
	})

	if err != nil {
		log.Printf("[ERROR] Error communicating with Ollama: %v", err)
		return "", "", fmt.Errorf("error communicating with Ollama: %w", err)
	}
	log.Printf("[DEBUG] Ollama full response: %s", responseContent.String())

	return userMessage, responseContent.String(), nil
}

// ProcessOllamaResponse processes Ollama response and executes tool calls if needed
// Now takes userQuestion as argument for LLM grounding
func (c *OllamaERPNextClient) ProcessOllamaResponse(response string, userQuestion string) (string, error) {
	log.Printf("[DEBUG] Processing Ollama response for tool calls: %s", response)

	toolCallRegex := regexp.MustCompile(`CALL_TOOL:([^:]+):(.+)`)
	lines := strings.Split(response, "\n")
	var resultParts []string
	var hasToolCalls bool

	for _, line := range lines {
		if matches := toolCallRegex.FindStringSubmatch(line); matches != nil {
			hasToolCalls = true
			toolName := matches[1]
			argumentsJSON := matches[2]
			log.Printf("[DEBUG] Detected CALL_TOOL directive: tool=%s, args=%s", toolName, argumentsJSON)

			var arguments map[string]interface{}
			if err := json.Unmarshal([]byte(argumentsJSON), &arguments); err != nil {
				log.Printf("[ERROR] Error parsing tool arguments: %v", err)
				resultParts = append(resultParts, fmt.Sprintf("Error parsing tool arguments: %v", err))
				continue
			}
			// Entity resolution for project/entity fields
			if toolName == "get_project_status" || toolName == "analyze_project_timeline" || toolName == "calculate_project_metrics" || toolName == "generate_project_report" {
				if userInput, ok := arguments["project_name"].(string); ok && userInput != "" {
					resolved, err := c.ResolveEntityWithFallback(context.Background(), "Project", userInput)
					if err == nil && resolved != "" {
						arguments["project_name"] = resolved
						log.Printf("[DEBUG] Resolved project_name '%s' to '%s'", userInput, resolved)
					}
				}
			}
			// Call the tool with the resolved arguments
			toolResult, err := c.CallTool(toolName, arguments)
			if err != nil {
				log.Printf("[ERROR] Error calling tool %s: %v", toolName, err)
				resultParts = append(resultParts, fmt.Sprintf("❌ Error calling tool %s: %v", toolName, err))
			} else {
				log.Printf("[DEBUG] Tool %s returned data: %s", toolName, toolResult)

				// Additional debug: Extract and log JSON content for verification
				if toolName == "get_document" {
					jsonRe := regexp.MustCompile(`\{.*\}`)
					if matches := jsonRe.FindStringSubmatch(toolResult); len(matches) > 0 {
						jsonContent := matches[0]
						var doc map[string]interface{}
						if err := json.Unmarshal([]byte(jsonContent), &doc); err == nil {
							// Check and log key fields
							importantFields := []string{"name", "project_name", "expected_start_date", "expected_end_date", "status"}
							foundFields := []string{}
							for _, field := range importantFields {
								if val, exists := doc[field]; exists {
									foundFields = append(foundFields, fmt.Sprintf("%s: %v", field, val))
									log.Printf("[INFO] Document field found: %s = %v", field, val)
									// Ensure important dates are clearly highlighted in the result
									if field == "expected_start_date" || field == "expected_end_date" {
										toolResult = strings.ReplaceAll(
											toolResult,
											fmt.Sprintf(`"%s":"%v"`, field, val),
											fmt.Sprintf(`"%s":"%v" /* IMPORTANT DATE */`, field, val))
									}
								} else {
									log.Printf("[INFO] Document field not found: %s", field)
								}
							}

							// Add a summary section to make it easier for the LLM to find key fields
							if len(foundFields) > 0 {
								summary := fmt.Sprintf("\n\nKEY FIELDS SUMMARY:\n%s", strings.Join(foundFields, "\n"))
								toolResult = toolResult + summary
							}
						} else {
							log.Printf("[ERROR] Failed to parse JSON content: %v", err)
						}
					} else {
						log.Printf("[ERROR] No JSON content found in tool result")
					}
				}

				// Inject the tool call directive directly into the prompt to force execution
				if toolName == "get_document" {
					toolResult = fmt.Sprintf("### TOOL CALL RESULTS ###\nTool: %s\nArguments: %+v\nResult:\n%s", toolName, arguments, toolResult)
				}

				// Patch: After tool call, send user question + raw tool result to LLM for grounded answer

				llmPrompt := fmt.Sprintf(`You are an ERPNext Data Assistant. Answer the following user question using ONLY the ERPNext data provided below. You must NOT rely on any other information or make up any data.

USER QUESTION: %s

REAL ERPNEXT DATA:
%s

YOUR TASK:
1. FIRST, examine the JSON data carefully to locate any fields relevant to the question
2. If fields like 'expected_start_date' are present, extract and quote their EXACT values
3. Check for data in both the direct fields and in any nested objects/arrays

STRICT RULES:
1. DIRECTLY QUOTE values from the data - do not paraphrase dates, numbers or field values
2. Look for data in BOTH the JSON object and any text descriptions around it
3. For dates specifically, COPY THE EXACT STRING as it appears (e.g., "2024-05-15")
4. When answering, first state "Based on the data provided, I found: [specific field: exact value]"
5. If information is missing, say "I cannot find [field] in the data provided"
6. Do not make up any information not in the data

YOUR RESPONSE MUST BE FACTUAL AND CITE THE EXACT DATA.`,
					userQuestion,
					toolResult)

				log.Printf("[DEBUG] Second LLM prompt for grounding (after tool call):\n%s", llmPrompt)
				_, llmAnswer, err := c.ChatWithOllama(context.Background(), llmPrompt)
				if err != nil {
					resultParts = append(resultParts, "[LLM ERROR] "+err.Error())
				} else {
					log.Printf("[DEBUG] LLM answer after data grounding: %s", llmAnswer)
					resultParts = append(resultParts, llmAnswer)
				}
			}
		}
	}

	// If there were no tool calls, return the response as is
	if !hasToolCalls {
		log.Printf("[INFO] No tool calls detected in response")
		return response, nil
	}

	// Combine and return the results from all tool calls
	finalResult := strings.Join(resultParts, "\n\n")
	log.Printf("[DEBUG] Combined result from tool calls: %s", finalResult)
	return finalResult, nil
}

// ResolveEntityWithFallback resolves an entity name with fallback options
func (c *OllamaERPNextClient) ResolveEntityWithFallback(ctx context.Context, doctype, userInput string) (string, error) {
	// 1. Try global search
	searchArgs := map[string]interface{}{
		"doctype":      doctype,
		"search_query": userInput,
		"fields":       []string{"name", "project_name", "title"},
		"page_length":  10,
	}
	result, err := c.CallTool("search_documents", searchArgs)
	if err == nil && result != "" {
		// Try to extract the best match (assume JSON in result)
		var docList struct {
			Data []map[string]interface{} `json:"data"`
		}
		if err := json.Unmarshal([]byte(result), &docList); err == nil && len(docList.Data) > 0 {
			// Try exact or fuzzy match on name/title
			candidates := []string{}
			for _, doc := range docList.Data {
				if name, ok := doc["name"].(string); ok {
					candidates = append(candidates, name)
				}
				if title, ok := doc["project_name"].(string); ok {
					candidates = append(candidates, title)
				}
				if title, ok := doc["title"].(string); ok {
					candidates = append(candidates, title)
				}
			}
			if id, ok := utils.ResolveEntity(userInput, candidates); ok {
				return id, nil
			}
			// Fallback: return first result
			if len(candidates) > 0 {
				return candidates[0], nil
			}
		}
	}
	// 2. Fallback: try fuzzy/entity matching against a static or cached list (not implemented here)
	return "", fmt.Errorf("could not resolve entity for input: %s", userInput)
}

// preprocessUserInput uses LLM to extract intent and entities from user input
func (c *OllamaERPNextClient) preprocessUserInput(ctx context.Context, userInput string) (intent string, entities map[string]string, llmInput string, llmOutput string, err error) {
	prompt := fmt.Sprintf(`Extract the business intent and all relevant entities (like project names, customer names, dates, specific fields being queried) from the following user message. Respond ONLY in JSON with fields 'intent' and 'entities' (a map of entity type to value). Do not include any explanation or markdown, just the JSON object.

For date-related questions, add a "queried_field" entity to specify which date field the user is asking about (e.g. expected_start_date, expected_end_date, posting_date).

User message: "%s"

Example responses:
{"intent": "get_project_status", "entities": {"project_name": "24 villas"}}
{"intent": "get_document", "entities": {"doctype": "Project", "name": "PROJ-0001", "queried_field": "expected_start_date"}}`, userInput)
	llmInput, llmOutput, err = c.ChatWithOllama(ctx, prompt)
	if err != nil {
		return "", nil, llmInput, llmOutput, err
	}
	// Extract the first JSON object from the response
	jsonRe := regexp.MustCompile(`(?s)\{.*\}`)
	jsonStr := ""
	if matches := jsonRe.FindStringSubmatch(llmOutput); len(matches) > 0 {
		jsonStr = matches[0]
	} else {
		return "", nil, llmInput, llmOutput, fmt.Errorf("no JSON object found in LLM response: %s", llmOutput)
	}
	var parsed struct {
		Intent   string            `json:"intent"`
		Entities map[string]string `json:"entities"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		return "", nil, llmInput, llmOutput, fmt.Errorf("failed to parse LLM entity extraction: %w", err)
	}
	return parsed.Intent, parsed.Entities, llmInput, llmOutput, nil
}

func (c *OllamaERPNextClient) InteractiveSession() error {
	scanner := bufio.NewScanner(os.Stdin)
	ctx := context.Background()
	for {
		fmt.Print("\n💬 You: ")
		if !scanner.Scan() {
			break
		}
		userInput := strings.TrimSpace(scanner.Text())
		if userInput == "quit" || userInput == "exit" {
			fmt.Println("👋 Goodbye!")
			return nil
		}
		fmt.Println("🤖 Thinking...")

		// Step 1: Extract intent and entities from the user input
		intent, entities, llmInput, llmOutput, err := c.preprocessUserInput(ctx, userInput)
		if err != nil {
			fmt.Printf("❌ NLP error: %v\n", err)
			if c.debugMode {
				fmt.Printf("[DEBUG] LLM Input: %s\n[DEBUG] LLM Output: %s\n", llmInput, llmOutput)
			}
			continue
		}
		fmt.Printf("[NLP] Intent: %s, Entities: %+v\n", intent, entities)

		// Step 2: Directly call the relevant MCP tool based on intent
		var toolResult string
		var toolName string
		var toolArgs map[string]interface{}

		// Map intent to tool calls
		switch intent {
		case "get_project_status", "project_status", "status":
			toolName = "get_document"
			toolArgs = map[string]interface{}{
				"doctype": "Project",
			}
			if projectName, ok := entities["project_name"]; ok {
				// Try to resolve entity if needed
				resolved, err := c.ResolveEntityWithFallback(ctx, "Project", projectName)
				if err == nil && resolved != "" {
					toolArgs["name"] = resolved
				} else {
					toolArgs["name"] = projectName
				}
			}
		case "get_document":
			toolName = "get_document"
			toolArgs = map[string]interface{}{}

			// Map entities to tool arguments
			if doctype, ok := entities["doctype"]; ok {
				toolArgs["doctype"] = doctype
			}
			if name, ok := entities["name"]; ok {
				toolArgs["name"] = name
			}
		default:
			// For other intents, fall back to regular LLM handling
			_, ollamaResponse, err := c.ChatWithOllama(ctx, userInput)
			if err != nil {
				fmt.Printf("❌ Error: %v\n", err)
				continue
			}
			finalResponse, err := c.ProcessOllamaResponse(ollamaResponse, userInput)
			if err != nil {
				fmt.Printf("❌ Error processing response: %v\n", err)
				continue
			}
			fmt.Printf("🤖 Assistant: %s\n", finalResponse)
			if c.debugMode {
				fmt.Printf("[DEBUG] LLM Input: %s\n[DEBUG] LLM Output: %s\n[DEBUG] Ollama Output: %s\n", llmInput, llmOutput, ollamaResponse)
			}
			continue
		}

		// Step 3: Execute the tool call if we have arguments
		if toolName != "" && len(toolArgs) > 0 {
			log.Printf("[DEBUG] Directly calling tool %s with args: %+v", toolName, toolArgs)
			var err error
			toolResult, err = c.CallTool(toolName, toolArgs)
			if err != nil {
				fmt.Printf("❌ Error calling tool %s: %v\n", toolName, err)
				continue
			}
			log.Printf("[DEBUG] Tool %s returned data: %s", toolName, toolResult)
		}

		// Step 4: Send the query and the real MCP data to the LLM for interpretation
		if toolResult != "" {
			llmPrompt := fmt.Sprintf(`You are an ERPNext Data Assistant. Answer the following user question using ONLY the ERPNext data provided below. You must NOT rely on any other information or make up any data.

User question: %s

REAL ERPNext data returned from MCP server tool '%s':
%s

STRICT RULES:
1. DIRECTLY QUOTE values from the data - do not paraphrase dates, numbers, or field values
2. For dates specifically, copy them CHARACTER BY CHARACTER from the data (e.g., if data shows "2024-05-15", don't say "May 15, 2024")
3. When looking for a specific field (like expected_start_date, status, etc.), extract the EXACT value from the data
4. If information is missing or you can't find the specific field, say "I cannot find [field] in the data provided"
5. Do not make up any information not in the data
6. Do not fabricate or hallucinate any data
7. When you cite a value from the data, include both the field name and its exact value in your answer

Be precise and accurate. Your response will be verified against the actual data.`,
				userInput,
				toolName,
				toolResult)

			log.Printf("[DEBUG] Sending LLM prompt with real data for interpretation:\n%s", llmPrompt)
			_, llmAnswer, err := c.ChatWithOllama(ctx, llmPrompt)
			if err != nil {
				fmt.Printf("❌ LLM error: %v\n", err)
				continue
			}

			fmt.Printf("🤖 Assistant: %s\n", llmAnswer)
			if c.debugMode {
				fmt.Printf("[DEBUG] Tool: %s\n[DEBUG] Tool Args: %+v\n[DEBUG] Tool Result: %s\n",
					toolName, toolArgs, toolResult)
			}
		}
	}
	return scanner.Err()
}

func main() {
	// Parse command line arguments
	mcpServerPath := "./bin/frappe-mcp-server-stdio"
	ollamaModel := "llama3.1"
	mode := "cli" // Default to CLI mode

	if len(os.Args) > 1 {
		for i, arg := range os.Args[1:] {
			switch arg {
			case "--model":
				if i+2 < len(os.Args) {
					ollamaModel = os.Args[i+2]
				}
			case "--mcp-server":
				if i+2 < len(os.Args) {
					mcpServerPath = os.Args[i+2]
				}
			case "--mode":
				if i+2 < len(os.Args) {
					mode = os.Args[i+2]
				}
			case "--api":
				mode = "api"
			case "--help", "-h":
				fmt.Println("ERPNext MCP + Ollama Integration")
				fmt.Println("Usage: ollama-client [options]")
				fmt.Println("Options:")
				fmt.Println("  --model MODEL          Ollama model to use (default: llama3.1)")
				fmt.Println("  --mcp-server PATH      Path to ERPNext MCP server (default: ./bin/frappe-mcp-server-stdio)")
				fmt.Println("  --mode MODE            Run mode: 'cli' or 'api' (default: cli)")
				fmt.Println("  --api                  Shortcut for --mode api")
				fmt.Println("  --help, -h             Show this help message")
				fmt.Println()
				fmt.Println("Examples:")
				fmt.Println("  ollama-client                    Start in interactive CLI mode")
				fmt.Println("  ollama-client --api              Start API server on port 8080")
				fmt.Println("  ollama-client --api --port 3000  Start API server on port 3000")
				return
			}
		}
	}

	// Validate mode
	if mode != "cli" && mode != "api" {
		fmt.Printf("❌ Invalid mode: %s. Must be 'cli' or 'api'\n", mode)
		return
	}

	// Check if MCP server exists
	if _, err := os.Stat(mcpServerPath); os.IsNotExist(err) {
		fmt.Printf("❌ MCP server not found at %s\n", mcpServerPath)
		fmt.Println("Run 'make build-stdio' to build the MCP server")
		return
	}

	if mode == "cli" {
		// CLI mode
		fmt.Printf("💻 Starting in CLI mode...\n")
		debugMode := false
		for _, arg := range os.Args {
			if arg == "--debug" || arg == "--raw" {
				debugMode = true
				os.Setenv("DEBUG", "1")
			}
		}
		client := NewOllamaERPNextClient(mcpServerPath, ollamaModel, debugMode)
		if err := client.StartMCPServer(); err != nil {
			fmt.Printf("❌ Failed to start MCP server: %v\n", err)
			return
		}
		scanner := bufio.NewScanner(os.Stdin)
		ctx := context.Background()
		for {
			fmt.Print("\n💬 You: ")
			if !scanner.Scan() {
				break
			}
			userInput := strings.TrimSpace(scanner.Text())
			if userInput == "quit" || userInput == "exit" {
				fmt.Println("👋 Goodbye!")
				return
			}
			fmt.Println("🤖 Thinking...")
			intent, entities, llmInput, llmOutput, err := client.preprocessUserInput(ctx, userInput)
			if err != nil {
				fmt.Printf("❌ NLP error: %v\n", err)
				if debugMode {
					fmt.Printf("[DEBUG] LLM Input: %s\n[DEBUG] LLM Output: %s\n", llmInput, llmOutput)
				}
				continue
			}
			fmt.Printf("[NLP] Intent: %s, Entities: %+v\n", intent, entities)
			// If we identified specific entities, directly inject tool calls when possible
			var ollamaResponse string
			if intent == "get_document" && entities["name"] != "" && (entities["doctype"] != "" || entities["queried_field"] == "expected_start_date") {
				// Directly inject a tool call for document retrieval to avoid hallucination
				docType := entities["doctype"]
				if docType == "" {
					docType = "Project" // Default for expected_start_date queries
				}
				ollamaResponse = fmt.Sprintf("CALL_TOOL:get_document:{\"doctype\":\"%s\",\"name\":\"%s\"}",
					docType, entities["name"])
				log.Printf("[INFO] Injecting direct tool call for document: %s", ollamaResponse)
			} else {
				// Use normal LLM flow for other queries
				_, ollamaResponseTemp, err := client.ChatWithOllama(ctx, userInput)
				if err != nil {
					fmt.Printf("❌ Error: %v\n", err)
					continue
				}
				ollamaResponse = ollamaResponseTemp
			}

			finalResponse, err := client.ProcessOllamaResponse(ollamaResponse, userInput)
			if err != nil {
				fmt.Printf("❌ Error processing response: %v\n", err)
				continue
			}
			fmt.Printf("🤖 Assistant: %s\n", finalResponse)
			if debugMode {
				fmt.Printf("[DEBUG] LLM Input: %s\n[DEBUG] LLM Output: %s\n[DEBUG] Ollama Output: %s\n", llmInput, llmOutput, ollamaResponse)
			}
		}
		return
	}

	if mode == "api" {
		// API mode: start HTTP server with OpenAPI endpoints and OpenAI-compatible LLM chat API
		mux := http.NewServeMux()
		debugMode := false
		for _, arg := range os.Args {
			if arg == "--debug" || arg == "--raw" {
				debugMode = true
				os.Setenv("DEBUG", "1")
			}
		}
		client := NewOllamaERPNextClient(mcpServerPath, ollamaModel, debugMode)
		if err := client.StartMCPServer(); err != nil {
			fmt.Printf("❌ Failed to start MCP server: %v\n", err)
			return
		}
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		})
		mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				_, _ = w.Write([]byte("Method not allowed"))
				return
			}
			var req struct {
				Message string `json:"message"`
				Raw     bool   `json:"raw"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte("Invalid request body"))
				return
			}
			ctx := context.Background()
			intent, entities, llmInput, llmOutput, err := client.preprocessUserInput(ctx, req.Message)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error(), "llm_input": llmInput, "llm_output": llmOutput})
				return
			}
			log.Printf("[NLP] Intent: %s, Entities: %+v", intent, entities)

			// Step 2: Directly call the relevant MCP tool based on intent
			var toolResult string
			var toolName string
			var toolArgs map[string]interface{}
			var finalResp string
			var ollamaInput, ollamaResp string

			// Map intent to tool calls
			switch intent {
			case "get_project_status", "project_status", "status":
				toolName = "get_document"
				toolArgs = map[string]interface{}{
					"doctype": "Project",
				}
				if projectName, ok := entities["project_name"]; ok {
					// Try to resolve entity if needed
					resolved, err := client.ResolveEntityWithFallback(ctx, "Project", projectName)
					if err == nil && resolved != "" {
						toolArgs["name"] = resolved
					} else {
						toolArgs["name"] = projectName
					}
				}
			case "get_document":
				toolName = "get_document"
				toolArgs = map[string]interface{}{}

				// Map entities to tool arguments
				if doctype, ok := entities["doctype"]; ok {
					toolArgs["doctype"] = doctype
				} else if entities["queried_field"] == "expected_start_date" {
					// Default to Project for expected_start_date queries
					toolArgs["doctype"] = "Project"
				}
				if name, ok := entities["name"]; ok {
					toolArgs["name"] = name
				}
			default:
				// For other intents, fall back to regular LLM handling
				ollamaInput, ollamaResp, err = client.ChatWithOllama(ctx, req.Message)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"error":         err.Error(),
						"ollama_input":  ollamaInput,
						"ollama_output": ollamaResp,
					})
					return
				}
				finalResp, err = client.ProcessOllamaResponse(ollamaResp, req.Message)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
					return
				}
			}

			// Step 3: Execute the tool call if we have arguments
			if toolName != "" && len(toolArgs) > 0 {
				log.Printf("[DEBUG] API: Directly calling tool %s with args: %+v", toolName, toolArgs)
				var err error
				toolResult, err = client.CallTool(toolName, toolArgs)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"error": fmt.Sprintf("Error calling tool %s: %v", toolName, err),
					})
					return
				}
				log.Printf("[DEBUG] API: Tool %s returned data: %s", toolName, toolResult)

				// Step 4: Send the query and the real MCP data to the LLM for interpretation
				llmPrompt := fmt.Sprintf(`You are an ERPNext Data Assistant. Answer the following user question using ONLY the ERPNext data provided below. You must NOT rely on any other information or make up any data.

User question: %s

REAL ERPNext data returned from MCP server tool '%s':
%s

STRICT RULES:
1. DIRECTLY QUOTE values from the data - do not paraphrase dates, numbers, or field values
2. For dates specifically, copy them CHARACTER BY CHARACTER from the data (e.g., if data shows "2024-05-15", don't say "May 15, 2024")
3. When looking for a specific field (like expected_start_date, status, etc.), extract the EXACT value from the data
4. If information is missing or you can't find the specific field, say "I cannot find [field] in the data provided"
5. Do not make up any information not in the data
6. Do not fabricate or hallucinate any data
7. When you cite a value from the data, include both the field name and its exact value in your answer

Be precise and accurate. Your response will be verified against the actual data.`,
					req.Message,
					toolName,
					toolResult)

				log.Printf("[DEBUG] API: Sending LLM prompt with real data for interpretation:\n%s", llmPrompt)
				ollamaInput = llmPrompt
				_, finalResp, err = client.ChatWithOllama(ctx, llmPrompt)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"error":        fmt.Sprintf("LLM error: %v", err),
						"ollama_input": ollamaInput,
					})
					return
				}
			}
			w.Header().Set("Content-Type", "application/json")
			if req.Raw || debugMode {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"response":      finalResp,
					"llm_input":     llmInput,
					"llm_output":    llmOutput,
					"ollama_input":  ollamaInput,
					"ollama_output": ollamaResp,
					"intent":        intent,
					"entities":      entities,
				})
			} else {
				_ = json.NewEncoder(w).Encode(map[string]string{"response": finalResp})
			}
		})
		fmt.Println("🌐 API server running on :8080 ...")
		if err := http.ListenAndServe(":8080", mux); err != nil {
			log.Fatal(err)
		}
	}
}
