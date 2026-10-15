package types

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"
)

// SessionExpiredMessage is what a user is told when Frappe reports their session ended, on every channel.
const SessionExpiredMessage = "Session expired. Please sign in again."

type ERPNextError struct {
	Message    string `json:"message"`
	StatusCode int    `json:"status_code"`
	Exc        string `json:"exc,omitempty"`
	ExcType    string `json:"exc_type,omitempty"`
	Exception  string `json:"exception,omitempty"`
	// Frappe sends its session_expired flag as 1, not true.
	SessionExpired int `json:"session_expired,omitempty"`
}

func (e *ERPNextError) Error() string {
	return e.Message
}

type Document map[string]interface{}

type DocumentList struct {
	Data     []Document `json:"data"`
	Total    int        `json:"total_count"`
	PageSize int        `json:"page_length"`
	Page     int        `json:"start"`
	HasMore  bool       `json:"has_more"`
}

type CreateDocumentRequest struct {
	DocType string   `json:"doctype" validate:"required"`
	Data    Document `json:"data" validate:"required"`
}

// CreateDocumentsRequest is one batch of new records of one doctype, inserted in one request.
type CreateDocumentsRequest struct {
	DocType string     `json:"doctype" validate:"required"`
	Rows    []Document `json:"rows" validate:"required"`
}

type UpdateDocumentRequest struct {
	DocType string   `json:"doctype" validate:"required"`
	Name    string   `json:"name" validate:"required"`
	Data    Document `json:"data" validate:"required"`
}

type SearchRequest struct {
	DocType  string                 `json:"doctype" validate:"required"`
	Fields   []string               `json:"fields,omitempty"`
	Filters  map[string]interface{} `json:"filters,omitempty"`
	OrderBy  string                 `json:"order_by,omitempty"`
	PageSize int                    `json:"page_length,omitempty"`
	Page     int                    `json:"start,omitempty"`
	Search   string                 `json:"search,omitempty"`
}

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

type Budget struct {
	TotalBudget    float64 `json:"total_budget"`
	ActualCost     float64 `json:"actual_cost"`
	BudgetConsumed float64 `json:"budget_consumed"`
	Variance       float64 `json:"variance"`
}

type User struct {
	ID        string                 `json:"id"`
	Email     string                 `json:"email"`
	FullName  string                 `json:"full_name"`
	Roles     []string               `json:"roles,omitempty"`
	ClientID  string                 `json:"client_id,omitempty"`
	Token     string                 `json:"-"` // OAuth2 token (not serialized in JSON)
	SessionID string                 `json:"-"` // Frappe session ID (sid cookie value)
	APIKey    string                 `json:"-"` // Frappe API key and secret as "key:secret", for a client that signs in with one
	CSRFToken string                 `json:"-"` // Frappe CSRF token (for POST/PUT/DELETE with session)
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

func (u *User) GetID() string {
	return u.ID
}

// GetUserName returns the user's email as the username
func (u *User) GetUserName() string {
	return u.Email
}

// GetGroups returns the user's roles
func (u *User) GetGroups() []string {
	return u.Roles
}

// GetExtensions returns the user's string-valued metadata and drops the rest.
func (u *User) GetExtensions() map[string][]string {
	// Convert metadata to string map for compatibility
	result := make(map[string][]string)
	for k, v := range u.Metadata {
		if str, ok := v.(string); ok {
			result[k] = []string{str}
		}
	}
	return result
}

type AggregationRequest struct {
	DocType string                 `json:"doctype" validate:"required"`
	Metric  string                 `json:"metric,omitempty"`   // count, sum, avg, min or max
	Field   string                 `json:"field,omitempty"`    // the field a metric other than count reads
	Filters map[string]interface{} `json:"filters,omitempty"`  // {"status": "Paid"}
	GroupBy string                 `json:"group_by,omitempty"` // "customer"
	TopN    int                    `json:"top_n,omitempty"`    // keep the N largest groups
}

type ReportRequest struct {
	ReportName string                 `json:"report_name" validate:"required"` // "Sales Analytics"
	Filters    map[string]interface{} `json:"filters,omitempty"`               // Report filters
	User       string                 `json:"user,omitempty"`                  // User context
}

type ReportResponse struct {
	Columns []ReportColumn           `json:"columns"` // Column definitions
	Data    []map[string]interface{} `json:"data"`    // one object per row, keyed by column fieldname
	Message string                   `json:"message,omitempty"`
}

type ReportColumn struct {
	Label     string `json:"label"`
	FieldName string `json:"fieldname"`
	FieldType string `json:"fieldtype"`
	Width     int    `json:"width,omitempty"`
}

type contextKey string

const (
	siteContextKey    contextKey = "frappe_site"
	baseURLContextKey contextKey = "frappe_base_url"
)

// WithSite binds the multi-tenant site name into the context.
func WithSite(ctx context.Context, site string) context.Context {
	return context.WithValue(ctx, siteContextKey, site)
}

// SiteFromContext returns the bound site name, or empty string if unset.
func SiteFromContext(ctx context.Context) string {
	if site, ok := ctx.Value(siteContextKey).(string); ok {
		return site
	}
	return ""
}

// WithBaseURL binds the resolved Frappe bench base URL into the context.
func WithBaseURL(ctx context.Context, baseURL string) context.Context {
	return context.WithValue(ctx, baseURLContextKey, baseURL)
}

// BaseURLFromContext returns the resolved Frappe bench base URL, or empty string if unset.
func BaseURLFromContext(ctx context.Context) string {
	if u, ok := ctx.Value(baseURLContextKey).(string); ok {
		return u
	}
	return ""
}

// SidCacheKey names a session's cache entry without holding the sid itself: the site namespaces it, so one sid
// value on two sites is two entries, and the SHA-256 keeps the live credential out of every map and heap dump.
func SidCacheKey(site, sid string) string {
	h := sha256.Sum256([]byte(sid))
	if site == "" {
		return fmt.Sprintf("sid:%x", h)
	}
	return fmt.Sprintf("%s:sid:%x", site, h)
}
