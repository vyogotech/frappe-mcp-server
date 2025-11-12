package types

import "time"

// ERPNextError represents an error from ERPNext API
type ERPNextError struct {
	Message    string `json:"message"`
	StatusCode int    `json:"status_code"`
	Exc        string `json:"exc,omitempty"`
}

func (e *ERPNextError) Error() string {
	return e.Message
}

// Document represents a generic ERPNext document
type Document map[string]interface{}

// DocumentList represents a list of documents with pagination
type DocumentList struct {
	Data     []Document `json:"data"`
	Total    int        `json:"total_count"`
	PageSize int        `json:"page_length"`
	Page     int        `json:"start"`
	HasMore  bool       `json:"has_more"`
}

// CreateDocumentRequest represents a request to create a document
type CreateDocumentRequest struct {
	DocType string   `json:"doctype" validate:"required"`
	Data    Document `json:"data" validate:"required"`
}

// UpdateDocumentRequest represents a request to update a document
type UpdateDocumentRequest struct {
	DocType string   `json:"doctype" validate:"required"`
	Name    string   `json:"name" validate:"required"`
	Data    Document `json:"data" validate:"required"`
}

// SearchRequest represents a search request
type SearchRequest struct {
	DocType  string                 `json:"doctype" validate:"required"`
	Fields   []string               `json:"fields,omitempty"`
	Filters  map[string]interface{} `json:"filters,omitempty"`
	OrderBy  string                 `json:"order_by,omitempty"`
	PageSize int                    `json:"page_length,omitempty"`
	Page     int                    `json:"start,omitempty"`
	Search   string                 `json:"search,omitempty"`
}

// ProjectStatus represents project status information
type ProjectStatus struct {
	Name        string    `json:"name"`
	Title       string    `json:"project_name"`
	Status      string    `json:"status"`
	Priority    string    `json:"priority"`
	StartDate   time.Time `json:"expected_start_date"`
	EndDate     time.Time `json:"expected_end_date"`
	ActualStart time.Time `json:"actual_start_date"`
	ActualEnd   time.Time `json:"actual_end_date"`
	Progress    float64   `json:"percent_complete"`
	Tasks       []Task    `json:"tasks"`
	Budget      Budget    `json:"budget"`
	TeamMembers []string  `json:"users"`
}

// Task represents a project task
type Task struct {
	Name        string    `json:"name"`
	Subject     string    `json:"subject"`
	Status      string    `json:"status"`
	Priority    string    `json:"priority"`
	Progress    float64   `json:"progress"`
	StartDate   time.Time `json:"expected_start_date"`
	EndDate     time.Time `json:"expected_end_date"`
	ActualStart time.Time `json:"actual_start_date"`
	ActualEnd   time.Time `json:"actual_end_date"`
	AssignedTo  string    `json:"assigned_to"`
	Project     string    `json:"project"`
}

// Budget represents project budget information
type Budget struct {
	TotalBudget    float64 `json:"total_budget"`
	ActualCost     float64 `json:"actual_cost"`
	BudgetConsumed float64 `json:"budget_consumed"`
	Variance       float64 `json:"variance"`
}

// ProjectMetrics represents calculated project metrics
type ProjectMetrics struct {
	BurnRate   float64 `json:"burn_rate"`
	Velocity   float64 `json:"velocity"`
	Efficiency float64 `json:"efficiency"`
	RiskScore  float64 `json:"risk_score"`
	Health     string  `json:"health"`
}
