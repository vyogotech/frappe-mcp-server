package testutils

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"frappe-mcp-server/internal/types"
)

// ConfirmRedeemMethod is a whitelisted method that answers like frappe_ai's one-time write confirmation, so a tool
// test can reach the write behind the confirmation gate without naming another project's API.
const ConfirmRedeemMethod = "test_confirm.redeem"

func MockERPNextServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set response headers
		w.Header().Set("Content-Type", "application/json")

		// Frappe routes a write by doctype and name, not by a table of paths (frappe/api/v1.py url_rules).
		if doctype, name, ok := resourcePath(r.URL.Path); ok && r.Method != http.MethodGet {
			handleWrite(w, r, doctype, name)
			return
		}

		// Handle different endpoints
		switch r.URL.Path {
		case "/api/method/" + ConfirmRedeemMethod:
			handleConfirmRedeem(w, r)
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
		case "/api/method/frappe.utils.global_search.search":
			handleGlobalSearch(w, r)
		default:
			handleDefault(w, r)
		}
	}))
}

// resourcePath splits /api/resource/<doctype>[/<name>], the two routes frappe/api/v1.py mounts for a document.
// The trailing slash is optional because frappe/api/__init__.py binds the map with strict_slashes=False.
func resourcePath(path string) (doctype, name string, ok bool) {
	rest, ok := strings.CutPrefix(path, "/api/resource/")
	if !ok {
		return "", "", false
	}
	doctype, name, _ = strings.Cut(strings.TrimSuffix(rest, "/"), "/")
	return doctype, name, doctype != ""
}

// handleWrite answers as frappe/api/v1.py does: create_doc and update_doc return the document under "data", and
// delete_doc answers 202 with "ok". The document echoes the fields it was sent, so a request that lost its body,
// its method or its path cannot be mistaken for one that arrived.
func handleWrite(w http.ResponseWriter, r *http.Request, doctype, name string) {
	if _, err := r.Cookie("sid"); err == nil && r.Header.Get("X-Frappe-CSRF-Token") == "" {
		// frappe/auth.py:83-99: an unsafe method on a session cookie without the session's token is refused
		frappeError(w, http.StatusBadRequest, "CSRFTokenError", "Invalid Request")
		return
	}

	switch {
	case r.Method == http.MethodPost && name == "":
		data, ok := decodeBody(w, r)
		if !ok {
			return
		}
		writeData(w, http.StatusOK, document(doctype, "NEW-"+strings.ToUpper(doctype)+"-0001", data))
	case r.Method == http.MethodPut && name != "":
		data, ok := decodeBody(w, r)
		if !ok {
			return
		}
		writeData(w, http.StatusOK, document(doctype, name, data))
	case r.Method == http.MethodDelete && name != "":
		writeData(w, http.StatusAccepted, "ok")
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func decodeBody(w http.ResponseWriter, r *http.Request) (types.Document, bool) {
	var data types.Document
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		frappeError(w, http.StatusBadRequest, "ValidationError", err.Error())
		return nil, false
	}
	return data, true
}

// document is what Frappe hands back after an insert or a save: the fields it was given plus the ones it owns.
func document(doctype, name string, data types.Document) types.Document {
	doc := types.Document{}
	for k, v := range data {
		doc[k] = v
	}
	doc["name"] = name
	doc["doctype"] = doctype
	doc["owner"] = "Administrator"
	doc["modified_by"] = "Administrator"
	doc["creation"] = "2024-01-01 00:00:00.000000"
	doc["modified"] = "2024-01-02 00:00:00.000000"
	doc["docstatus"] = 0
	doc["idx"] = 0
	return doc
}

func writeData(w http.ResponseWriter, status int, data interface{}) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": data})
}

// frappeError answers as frappe/utils/response.py's report_error does: exc_type always, the message inside
// _server_messages, and no exception, which is sent only where a traceback is allowed — never for a CSRFTokenError,
// whose throw sets disable_traceback first (frappe/auth.py:97).
func frappeError(w http.ResponseWriter, status int, excType, message string) {
	one, _ := json.Marshal(map[string]string{"message": message})
	messages, _ := json.Marshal([]string{string(one)})
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"exc_type": excType, "_server_messages": string(messages)})
}

func handleConfirmRedeem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"message": map[string]interface{}{"ok": true}})
}

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
			"total_budget":     100000.0,
			"actual_cost":      80000.0,
		},
		{
			"name":             "TEST-PROJ-002",
			"project_name":     "Test Project 2",
			"status":           "Completed",
			"percent_complete": 100.0,
			"priority":         "Medium",
			"total_budget":     50000.0,
			"actual_cost":      60000.0,
		},
	}

	response := map[string]interface{}{
		"data": projects,
	}

	_ = json.NewEncoder(w).Encode(response)
}

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

func handleDefault(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"message": "Endpoint not mocked",
		"path":    r.URL.Path,
		"method":  r.Method,
	}

	w.WriteHeader(http.StatusNotFound)
	_ = json.NewEncoder(w).Encode(response)
}

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

func handleGlobalSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	results := []map[string]interface{}{
		{
			"name":    "TEST-PROJ-001",
			"doctype": "Project",
			"content": "Test Project 1 – Open",
			"route":   "project/TEST-PROJ-001",
		},
		{
			"name":    "TEST-TASK-001",
			"doctype": "Task",
			"content": "Design Homepage",
			"route":   "task/TEST-TASK-001",
		},
	}

	response := map[string]interface{}{
		"message": results,
	}
	_ = json.NewEncoder(w).Encode(response)
}
