package testutils

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"frappe-mcp-server/internal/types"
)

// MockERPNextServer creates a mock ERPNext server for testing
func MockERPNextServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set response headers
		w.Header().Set("Content-Type", "application/json")

	// Handle different endpoints
	switch r.URL.Path {
	case "/api/resource/Project/TEST-PROJ-001":
		handleGetProject(w, r)
	case "/api/resource/Project":
		handleProjectList(w, r)
	case "/api/resource/Task":
		handleTaskList(w, r)
	case "/api/resource/Customer":
		handleCustomerList(w, r)
	case "/api/resource/Employee":
		handleEmployeeList(w, r)
	case "/api/method/frappe.desk.search.search_link":
		handleSearch(w, r)
	default:
		handleDefault(w, r)
	}
	}))
}

// handleGetProject handles GET requests for a specific project
func handleGetProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	project := types.Document{
		"name":                "TEST-PROJ-001",
		"project_name":        "Test Project",
		"status":              "Open",
		"priority":            "High",
		"percent_complete":    25.5,
		"expected_start_date": "2024-01-01",
		"expected_end_date":   "2024-06-30",
		"total_budget":        100000.0,
		"actual_cost":         25000.0,
		"users": []interface{}{
			map[string]interface{}{"user": "john.doe@company.com"},
			map[string]interface{}{"user": "jane.smith@company.com"},
		},
	}

	response := map[string]interface{}{
		"data": project,
	}

	_ = json.NewEncoder(w).Encode(response)
}

// handleProjectList handles GET requests for project list
func handleProjectList(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	projects := []types.Document{
		{
			"name":             "TEST-PROJ-001",
			"project_name":     "Test Project 1",
			"status":           "Open",
			"percent_complete": 25.5,
			"priority":         "High",
		},
		{
			"name":             "TEST-PROJ-002",
			"project_name":     "Test Project 2",
			"status":           "Completed",
			"percent_complete": 100.0,
			"priority":         "Medium",
		},
	}

	response := map[string]interface{}{
		"data": projects,
	}

	_ = json.NewEncoder(w).Encode(response)
}

// handleTaskList handles GET requests for task list
func handleTaskList(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	tasks := []types.Document{
		{
			"name":                "TEST-TASK-001",
			"subject":             "Design Homepage",
			"status":              "Open",
			"priority":            "High",
			"progress":            50.0,
			"project":             "TEST-PROJ-001",
			"expected_start_date": "2024-01-01",
			"expected_end_date":   "2024-01-15",
		},
		{
			"name":                "TEST-TASK-002",
			"subject":             "Develop API",
			"status":              "Working",
			"priority":            "Medium",
			"progress":            75.0,
			"project":             "TEST-PROJ-001",
			"expected_start_date": "2024-01-16",
			"expected_end_date":   "2024-02-01",
		},
	}

	response := map[string]interface{}{
		"data": tasks,
	}

	_ = json.NewEncoder(w).Encode(response)
}

// handleCustomerList handles GET requests for customer list
func handleCustomerList(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	customers := []types.Document{
		{
			"name":          "CUST-001",
			"customer_name": "Acme Corporation",
			"customer_type": "Company",
			"territory":     "United States",
		},
		{
			"name":          "CUST-002",
			"customer_name": "Tech Solutions Ltd",
			"customer_type": "Company",
			"territory":     "United Kingdom",
		},
	}

	response := map[string]interface{}{
		"data": customers,
	}

	_ = json.NewEncoder(w).Encode(response)
}

// handleEmployeeList handles GET requests for employee list
func handleEmployeeList(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	employees := []types.Document{
		{
			"name":          "EMP-001",
			"employee_name": "John Doe",
			"department":    "Engineering",
			"designation":   "Senior Developer",
			"status":        "Active",
		},
		{
			"name":          "EMP-002",
			"employee_name": "Jane Smith",
			"department":    "Design",
			"designation":   "UI/UX Designer",
			"status":        "Active",
		},
	}

	response := map[string]interface{}{
		"data": employees,
	}

	_ = json.NewEncoder(w).Encode(response)
}

// handleDefault handles default responses
func handleDefault(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"message": "Endpoint not mocked",
		"path":    r.URL.Path,
		"method":  r.Method,
	}

	w.WriteHeader(http.StatusNotFound)
	_ = json.NewEncoder(w).Encode(response)
}

// handleSearch handles search requests
func handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Mock search results based on doctype
	doctype := r.URL.Query().Get("doctype")
	var results []map[string]interface{}

	switch doctype {
	case "Customer":
		results = []map[string]interface{}{
			{"value": "CUST-001", "description": "John Doe Customer"},
			{"value": "CUST-002", "description": "Jane Smith Customer"},
		}
	case "Project":
		results = []map[string]interface{}{
			{"value": "TEST-PROJ-001", "description": "Test Project 1"},
			{"value": "TEST-PROJ-002", "description": "Test Project 2"},
		}
	default:
		results = []map[string]interface{}{
			{"value": "GENERIC-001", "description": "Generic Result"},
		}
	}

	response := map[string]interface{}{
		"message": results,
	}

	_ = json.NewEncoder(w).Encode(response)
}

// CreateTestProject creates a test project document
func CreateTestProject() types.Document {
	return types.Document{
		"name":                "TEST-PROJ-001",
		"project_name":        "Test Project",
		"status":              "Open",
		"priority":            "High",
		"percent_complete":    25.5,
		"expected_start_date": "2024-01-01",
		"expected_end_date":   "2024-06-30",
		"total_budget":        100000.0,
		"actual_cost":         25000.0,
	}
}

// CreateTestTask creates a test task document
func CreateTestTask() types.Document {
	return types.Document{
		"name":                "TEST-TASK-001",
		"subject":             "Test Task",
		"status":              "Open",
		"priority":            "High",
		"progress":            50.0,
		"project":             "TEST-PROJ-001",
		"expected_start_date": "2024-01-01",
		"expected_end_date":   "2024-01-15",
	}
}

// AssertNoError asserts that no error occurred
func AssertNoError(t *testing.T, err error, message string) {
	if err != nil {
		t.Fatalf("%s: %v", message, err)
	}
}

// AssertError asserts that an error occurred
func AssertError(t *testing.T, err error, message string) {
	if err == nil {
		t.Fatalf("%s: expected error but got none", message)
	}
}

// AssertEqual asserts that two values are equal
func AssertEqual(t *testing.T, expected, actual interface{}, message string) {
	if expected != actual {
		t.Fatalf("%s: expected %v, got %v", message, expected, actual)
	}
}

// AssertNotNil asserts that a value is not nil
func AssertNotNil(t *testing.T, value interface{}, message string) {
	if value == nil {
		t.Fatalf("%s: expected non-nil value", message)
	}
}
