package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type TestClient struct {
	serverURL string
	client    *http.Client
}

type MCPToolRequest struct {
	ID     string                 `json:"id"`
	Tool   string                 `json:"tool"`
	Params map[string]interface{} `json:"params"`
}

type MCPToolResponse struct {
	ID      string                   `json:"id"`
	Content []map[string]interface{} `json:"content"`
	Error   *string                  `json:"error,omitempty"`
}

func main() {
	var (
		serverURL = flag.String("url", "http://localhost:8081", "MCP server URL")
		mode      = flag.String("mode", "interactive", "Mode: interactive, test, or demo")
	)
	flag.Parse()

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		fmt.Printf("Warning: Could not load .env file: %v\n", err)
	}

	client := &TestClient{
		serverURL: *serverURL,
		client:    &http.Client{Timeout: 30 * time.Second},
	}

	fmt.Println("🚀 ERPNext MCP Server Test Client")
	fmt.Println("==================================")

	switch *mode {
	case "interactive":
		client.runInteractive()
	case "test":
		client.runTests()
	case "demo":
		client.runDemo()
	default:
		fmt.Printf("Unknown mode: %s\n", *mode)
		os.Exit(1)
	}
}

func (c *TestClient) runInteractive() {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("\n📋 Available Commands:")
	fmt.Println("  1. list projects     - List all projects")
	fmt.Println("  2. get project       - Get specific project")
	fmt.Println("  3. search            - Search documents")
	fmt.Println("  4. project status    - Get project status")
	fmt.Println("  5. portfolio         - Portfolio dashboard")
	fmt.Println("  6. health            - Server health check")
	fmt.Println("  7. help              - Show this help")
	fmt.Println("  8. quit              - Exit")
	fmt.Println("\nType a command number or name:")

	for {
		fmt.Print("\n> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		switch input {
		case "1", "list projects", "list":
			c.testListProjects()
		case "2", "get project", "get":
			fmt.Print("Enter project name: ")
			if scanner.Scan() {
				projectName := strings.TrimSpace(scanner.Text())
				if projectName != "" {
					c.testGetProject(projectName)
				}
			}
		case "3", "search":
			fmt.Print("Enter doctype (Project/Task/Customer): ")
			if scanner.Scan() {
				doctype := strings.TrimSpace(scanner.Text())
				fmt.Print("Enter search query: ")
				if scanner.Scan() {
					query := strings.TrimSpace(scanner.Text())
					if doctype != "" && query != "" {
						c.testSearch(doctype, query)
					}
				}
			}
		case "4", "project status", "status":
			fmt.Print("Enter project name: ")
			if scanner.Scan() {
				projectName := strings.TrimSpace(scanner.Text())
				if projectName != "" {
					c.testProjectStatus(projectName)
				}
			}
		case "5", "portfolio":
			c.testPortfolio()
		case "6", "health":
			c.testHealth()
		case "7", "help":
			fmt.Println("\n📋 Available Commands:")
			fmt.Println("  1. list projects     - List all projects")
			fmt.Println("  2. get project       - Get specific project")
			fmt.Println("  3. search            - Search documents")
			fmt.Println("  4. project status    - Get project status")
			fmt.Println("  5. portfolio         - Portfolio dashboard")
			fmt.Println("  6. health            - Server health check")
			fmt.Println("  7. help              - Show this help")
			fmt.Println("  8. quit              - Exit")
		case "8", "quit", "exit":
			fmt.Println("Goodbye! 👋")
			return
		default:
			fmt.Printf("Unknown command: %s. Type 'help' for available commands.\n", input)
		}
	}
}

func (c *TestClient) runTests() {
	fmt.Println("\n🧪 Running Test Suite...")

	tests := []struct {
		name string
		fn   func() error
	}{
		{"Health Check", c.testHealthCheck},
		{"List Projects", c.testListProjectsAPI},
		{"Portfolio Dashboard", c.testPortfolioAPI},
	}

	passed := 0
	for _, test := range tests {
		fmt.Printf("\n▶️  Running: %s", test.name)
		if err := test.fn(); err != nil {
			fmt.Printf(" ❌ FAILED: %v", err)
		} else {
			fmt.Printf(" ✅ PASSED")
			passed++
		}
	}

	fmt.Printf("\n\n📊 Test Results: %d/%d passed\n", passed, len(tests))
}

func (c *TestClient) runDemo() {
	fmt.Println("\n🎬 Running Demo Scenarios...")

	scenarios := []struct {
		name        string
		description string
		fn          func()
	}{
		{
			"Basic Data Access",
			"Demonstrates basic CRUD operations",
			c.demoBasicAccess,
		},
		{
			"Project Management",
			"Shows project management capabilities",
			c.demoProjectManagement,
		},
		{
			"Business Analytics",
			"Displays analytics and reporting features",
			c.demoAnalytics,
		},
	}

	for i, scenario := range scenarios {
		fmt.Printf("\n📋 Scenario %d: %s\n", i+1, scenario.name)
		fmt.Printf("   %s\n", scenario.description)
		fmt.Print("   Press Enter to continue...")
		_, _ = bufio.NewReader(os.Stdin).ReadBytes('\n')
		scenario.fn()
	}
}

func (c *TestClient) testListProjects() {
	fmt.Println("\n📋 Listing Projects...")

	params := map[string]interface{}{
		"doctype": "Project",
		"limit":   10,
	}

	response := c.callTool("list_documents", params)
	c.printResponse(response)
}

func (c *TestClient) testGetProject(name string) {
	fmt.Printf("\n📄 Getting Project: %s\n", name)

	params := map[string]interface{}{
		"doctype": "Project",
		"name":    name,
	}

	response := c.callTool("get_document", params)
	c.printResponse(response)
}

func (c *TestClient) testSearch(doctype, query string) {
	fmt.Printf("\n🔍 Searching %s for: %s\n", doctype, query)

	params := map[string]interface{}{
		"doctype": doctype,
		"query":   query,
		"limit":   5,
	}

	response := c.callTool("search_documents", params)
	c.printResponse(response)
}

func (c *TestClient) testProjectStatus(name string) {
	fmt.Printf("\n📊 Project Status: %s\n", name)

	params := map[string]interface{}{
		"project_name": name,
	}

	response := c.callTool("get_project_status", params)
	c.printResponse(response)
}

func (c *TestClient) testPortfolio() {
	fmt.Println("\n📈 Portfolio Dashboard...")

	params := map[string]interface{}{}

	response := c.callTool("portfolio_dashboard", params)
	c.printResponse(response)
}

func (c *TestClient) testHealth() {
	fmt.Println("\n❤️  Health Check...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", c.serverURL+"/health", nil)
	if err != nil {
		fmt.Printf("❌ Error creating request: %v\n", err)
		return
	}

	resp, err := c.client.Do(req)
	if err != nil {
		fmt.Printf("❌ Error making request: %v\n", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("❌ Error reading response: %v\n", err)
		return
	}

	if resp.StatusCode == 200 {
		fmt.Printf("✅ Server is healthy\n")
		fmt.Printf("Response: %s\n", string(body))
	} else {
		fmt.Printf("❌ Server health check failed (status: %d)\n", resp.StatusCode)
		fmt.Printf("Response: %s\n", string(body))
	}
}

func (c *TestClient) callTool(toolName string, params map[string]interface{}) *MCPToolResponse {
	url := fmt.Sprintf("%s/tool/%s", c.serverURL, toolName)

	requestData := MCPToolRequest{
		ID:     fmt.Sprintf("test-%d", time.Now().Unix()),
		Tool:   toolName,
		Params: params,
	}

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		fmt.Printf("❌ Error marshaling request: %v\n", err)
		return &MCPToolResponse{Error: &[]string{err.Error()}[0]}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		fmt.Printf("❌ Error creating request: %v\n", err)
		return &MCPToolResponse{Error: &[]string{err.Error()}[0]}
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		fmt.Printf("❌ Error making request: %v\n", err)
		return &MCPToolResponse{Error: &[]string{err.Error()}[0]}
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("❌ Error reading response: %v\n", err)
		return &MCPToolResponse{Error: &[]string{err.Error()}[0]}
	}

	var response MCPToolResponse
	if err := json.Unmarshal(body, &response); err != nil {
		fmt.Printf("❌ Error unmarshaling response: %v\n", err)
		fmt.Printf("Raw response: %s\n", string(body))
		return &MCPToolResponse{Error: &[]string{err.Error()}[0]}
	}

	return &response
}

func (c *TestClient) printResponse(response *MCPToolResponse) {
	if response.Error != nil {
		fmt.Printf("❌ Error: %s\n", *response.Error)
		return
	}

	if len(response.Content) == 0 {
		fmt.Println("📭 No content in response")
		return
	}

	for i, content := range response.Content {
		if i > 0 {
			fmt.Println("---")
		}

		if text, ok := content["text"].(string); ok {
			fmt.Printf("📄 %s\n", text)
		} else {
			contentJSON, _ := json.MarshalIndent(content, "", "  ")
			fmt.Printf("📄 %s\n", string(contentJSON))
		}
	}
}

// Test API methods
func (c *TestClient) testHealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", c.serverURL+"/health", nil)
	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return fmt.Errorf("health check failed with status %d", resp.StatusCode)
	}

	return nil
}

func (c *TestClient) testListProjectsAPI() error {
	params := map[string]interface{}{
		"doctype": "Project",
		"limit":   5,
	}

	response := c.callTool("list_documents", params)
	if response.Error != nil {
		return fmt.Errorf("list_documents error: %s", *response.Error)
	}

	return nil
}

func (c *TestClient) testPortfolioAPI() error {
	params := map[string]interface{}{}

	response := c.callTool("portfolio_dashboard", params)
	if response.Error != nil {
		return fmt.Errorf("portfolio_dashboard error: %s", *response.Error)
	}

	return nil
}

// Demo scenarios
func (c *TestClient) demoBasicAccess() {
	fmt.Println("   • Listing projects...")
	c.testListProjects()

	time.Sleep(2 * time.Second)

	fmt.Println("\n   • Searching for customers...")
	c.testSearch("Customer", "test")
}

func (c *TestClient) demoProjectManagement() {
	fmt.Println("   • Getting portfolio dashboard...")
	c.testPortfolio()

	time.Sleep(2 * time.Second)

	fmt.Println("\n   • Note: To demo project status, you'll need actual project names from your ERPNext instance")
}

func (c *TestClient) demoAnalytics() {
	fmt.Println("   • Portfolio analytics...")
	params := map[string]interface{}{}
	response := c.callTool("resource_utilization_analysis", params)
	c.printResponse(response)
}
