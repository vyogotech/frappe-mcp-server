// Package frappe is a client for the generic Frappe REST API, so any Frappe app works, not only ERPNext.
package frappe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/time/rate"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"frappe-mcp-server/internal/auth"
	"frappe-mcp-server/internal/config"
	"frappe-mcp-server/internal/types"
)

// Client represents a Frappe Framework API client.
// Works with ERPNext and all other Frappe-based applications.
type Client struct {
	baseURL     string
	apiKey      string
	apiSecret   string
	httpClient  *http.Client
	rateLimiter *rate.Limiter
	retryConfig config.RetryConfig
}

// NewClient creates a new Frappe client (works with any Frappe-based application)
func NewClient(cfg config.ERPNextConfig) (*Client, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("base URL is required")
	}
	// API key and secret are now optional - if not provided, will use OAuth2 token from context
	if cfg.APIKey == "" && cfg.APISecret != "" {
		return nil, fmt.Errorf("API secret provided without API key")
	}
	if cfg.APISecret == "" && cfg.APIKey != "" {
		return nil, fmt.Errorf("API key provided without API secret")
	}

	transport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
		MaxIdleConnsPerHost: 10,
	}

	httpClient := &http.Client{
		Transport: otelhttp.NewTransport(transport),
		Timeout:   cfg.Timeout,
	}

	// Create rate limiter
	rateLimiter := rate.NewLimiter(
		rate.Limit(cfg.RateLimit.RequestsPerSecond),
		cfg.RateLimit.Burst,
	)

	return &Client{
		baseURL:     strings.TrimSuffix(cfg.BaseURL, "/"),
		apiKey:      cfg.APIKey,
		apiSecret:   cfg.APISecret,
		httpClient:  httpClient,
		rateLimiter: rateLimiter,
		retryConfig: cfg.Retry,
	}, nil
}

// GetDocument retrieves a single document by doctype and name
func (c *Client) GetDocument(ctx context.Context, docType, name string) (types.Document, error) {
	endpoint := fmt.Sprintf("/api/resource/%s/%s", url.PathEscape(docType), url.PathEscape(name))

	var response struct {
		Data types.Document `json:"data"`
	}

	if err := c.makeRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, fmt.Errorf("failed to get document %s/%s: %w", docType, name, err)
	}

	slog.Info("Document retrieved successfully", "doctype", docType, "name", name)
	return response.Data, nil
}

// GetDocumentList retrieves a list of documents with pagination
func (c *Client) GetDocumentList(ctx context.Context, req types.SearchRequest) (*types.DocumentList, error) {
	endpoint := fmt.Sprintf("/api/resource/%s", url.PathEscape(req.DocType))

	// Build query parameters
	params := url.Values{}
	if len(req.Fields) > 0 {
		params.Set("fields", fmt.Sprintf(`["%s"]`, strings.Join(req.Fields, `","`)))
	}
	if len(req.Filters) > 0 {
		filtersJSON, _ := json.Marshal(req.Filters)
		params.Set("filters", string(filtersJSON))
	}
	if req.OrderBy != "" {
		params.Set("order_by", req.OrderBy)
	}
	if req.PageSize > 0 {
		params.Set("limit_page_length", fmt.Sprintf("%d", req.PageSize))
	}
	if req.Page > 0 {
		params.Set("limit_start", fmt.Sprintf("%d", req.Page))
	}

	if params.Encode() != "" {
		endpoint += "?" + params.Encode()
	}

	var response struct {
		Data []types.Document `json:"data"`
	}

	if err := c.makeRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, fmt.Errorf("failed to get document list for %s: %w", req.DocType, err)
	}

	result := &types.DocumentList{
		Data:     response.Data,
		Total:    len(response.Data),
		PageSize: req.PageSize,
		Page:     req.Page,
		HasMore:  len(response.Data) == req.PageSize, // Simple heuristic
	}

	slog.Info("Document list retrieved successfully",
		"doctype", req.DocType,
		"count", len(response.Data),
		"page", req.Page)

	return result, nil
}

// CreateDocument creates a new document
func (c *Client) CreateDocument(ctx context.Context, req types.CreateDocumentRequest) (types.Document, error) {
	endpoint := fmt.Sprintf("/api/resource/%s", url.PathEscape(req.DocType))

	var response struct {
		Data types.Document `json:"data"`
	}

	if err := c.makeRequest(ctx, "POST", endpoint, req.Data, &response); err != nil {
		return nil, fmt.Errorf("failed to create document %s: %w", req.DocType, err)
	}

	slog.Info("Document created successfully", "doctype", req.DocType)
	return response.Data, nil
}

// UpdateDocument updates an existing document
func (c *Client) UpdateDocument(ctx context.Context, req types.UpdateDocumentRequest) (types.Document, error) {
	endpoint := fmt.Sprintf("/api/resource/%s/%s",
		url.PathEscape(req.DocType),
		url.PathEscape(req.Name))

	var response struct {
		Data types.Document `json:"data"`
	}

	if err := c.makeRequest(ctx, "PUT", endpoint, req.Data, &response); err != nil {
		return nil, fmt.Errorf("failed to update document %s/%s: %w", req.DocType, req.Name, err)
	}

	slog.Info("Document updated successfully", "doctype", req.DocType, "name", req.Name)
	return response.Data, nil
}

// DeleteDocument deletes a document
func (c *Client) DeleteDocument(ctx context.Context, docType, name string) error {
	endpoint := fmt.Sprintf("/api/resource/%s/%s",
		url.PathEscape(docType),
		url.PathEscape(name))

	if err := c.makeRequest(ctx, "DELETE", endpoint, nil, nil); err != nil {
		return fmt.Errorf("failed to delete document %s/%s: %w", docType, name, err)
	}

	slog.Info("Document deleted successfully", "doctype", docType, "name", name)
	return nil
}

// SearchDocuments performs full-text search across documents
func (c *Client) SearchDocuments(ctx context.Context, req types.SearchRequest) (*types.DocumentList, error) {
	endpoint := fmt.Sprintf("/api/resource/%s", url.PathEscape(req.DocType))

	// Build query parameters for search
	params := url.Values{}
	if len(req.Fields) > 0 {
		params.Set("fields", fmt.Sprintf(`["%s"]`, strings.Join(req.Fields, `","`)))
	}
	if len(req.Filters) > 0 {
		filtersJSON, _ := json.Marshal(req.Filters)
		params.Set("filters", string(filtersJSON))
	}
	if req.Search != "" {
		// Use Frappe's search functionality
		endpoint = "/api/method/frappe.desk.search.search_link"
		params.Set("txt", req.Search)
		params.Set("doctype", req.DocType)
	}
	if req.OrderBy != "" {
		params.Set("order_by", req.OrderBy)
	}
	if req.PageSize > 0 {
		params.Set("limit_page_length", fmt.Sprintf("%d", req.PageSize))
	}
	if req.Page > 0 {
		params.Set("limit_start", fmt.Sprintf("%d", req.Page))
	}

	if params.Encode() != "" {
		endpoint += "?" + params.Encode()
	}

	var response struct {
		Data    interface{} `json:"data"`
		Message interface{} `json:"message"`
	}

	if err := c.makeRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, fmt.Errorf("failed to search documents for %s: %w", req.DocType, err)
	}

	// Handle different response formats
	var documents []types.Document

	// Choose data source based on which one is populated
	var dataSource interface{}
	if response.Data != nil {
		dataSource = response.Data
	} else if response.Message != nil {
		dataSource = response.Message
	}

	switch data := dataSource.(type) {
	case []interface{}:
		for _, item := range data {
			if doc, ok := item.(map[string]interface{}); ok {
				documents = append(documents, doc)
			}
		}
	case map[string]interface{}:
		documents = append(documents, data)
	}

	result := &types.DocumentList{
		Data:     documents,
		Total:    len(documents),
		PageSize: req.PageSize,
		Page:     req.Page,
		HasMore:  len(documents) == req.PageSize,
	}

	slog.Info("Search completed successfully",
		"doctype", req.DocType,
		"query", req.Search,
		"results", len(documents))

	return result, nil
}

// GlobalSearchResult is a single result from the Frappe global search.
type GlobalSearchResult struct {
	Name    string `json:"name"`
	DocType string `json:"doctype"`
	Content string `json:"content"`
	Route   string `json:"route"`
}

// GlobalSearchRequest holds the parameters for a global search call.
type GlobalSearchRequest struct {
	Text    string      `json:"text"`
	Doctype string      `json:"doctype,omitempty"` // restrict to one doctype
	Scope   interface{} `json:"scope,omitempty"`   // one doctype or []string
	Limit   int         `json:"limit,omitempty"`
	Start   int         `json:"start,omitempty"`
}

// SearchKnowledgeBase asks the rag app for the passages closest to a question. The app owns
// the vector search and the per-user permission filter; this only carries the call.
func (c *Client) SearchKnowledgeBase(ctx context.Context, query string, limit int, session string) ([]map[string]interface{}, error) {
	if query == "" {
		return nil, fmt.Errorf("query is required for a knowledge base search")
	}
	if limit <= 0 {
		limit = 5
	}

	params := url.Values{}
	params.Set("query", query)
	params.Set("limit", fmt.Sprintf("%d", limit))
	// the chat the question comes from: the app also searches the files attached to it
	if session != "" {
		params.Set("session", session)
	}

	var response struct {
		Message []map[string]interface{} `json:"message"`
	}
	endpoint := "/api/method/rag.search.search?" + params.Encode()
	if err := c.makeRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, fmt.Errorf("knowledge base search failed: %w", err)
	}

	return response.Message, nil
}

// GlobalSearch performs a full-text search across all indexed doctypes using
// the Frappe global search endpoint (/api/method/frappe.utils.global_search.search).
func (c *Client) GlobalSearch(ctx context.Context, req GlobalSearchRequest) ([]GlobalSearchResult, error) {
	if req.Text == "" {
		return nil, fmt.Errorf("text is required for global search")
	}
	if req.Limit <= 0 {
		req.Limit = 20
	}

	body := map[string]interface{}{
		"text":  req.Text,
		"limit": req.Limit,
		"start": req.Start,
	}
	if req.Doctype != "" {
		body["doctype"] = req.Doctype
	} else if req.Scope != nil {
		body["scope"] = req.Scope
	}

	var response struct {
		Message []GlobalSearchResult `json:"message"`
	}

	if err := c.makeRequest(ctx, "POST", "/api/method/frappe.utils.global_search.search", body, &response); err != nil {
		return nil, fmt.Errorf("global search failed: %w", err)
	}

	return response.Message, nil
}

// makeRequest makes an HTTP request to Frappe API with retry logic
func (c *Client) makeRequest(ctx context.Context, method, endpoint string, body interface{}, result interface{}) error {
	attempts := 1
	if method == http.MethodGet { // a write retried after a timeout can run twice (RFC 9110 9.2.2)
		attempts = max(1, c.retryConfig.MaxAttempts)
	}
	for attempt := 1; ; attempt++ {
		if err := c.rateLimiter.Wait(ctx); err != nil {
			return fmt.Errorf("rate limit error: %w", err)
		}
		err := c.doRequest(ctx, method, endpoint, body, result)
		if err == nil || attempt == attempts || !isRetryableError(ctx, err) {
			return err
		}
		// full jitter up to an exponential bound, so the retries of many calls do not arrive together
		bound := min(c.retryConfig.MaxDelay, c.retryConfig.InitialDelay<<min(attempt-1, 30))
		delay := rand.N(bound + 1) //nolint:gosec // G404: backoff jitter is not a secret (CWE-338 does not apply)
		slog.Debug("Retrying request", "attempt", attempt+1, "delay", delay, "error", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
}

// doRequest performs the actual HTTP request
func (c *Client) doRequest(ctx context.Context, method, endpoint string, body interface{}, result interface{}) error {
	fullURL := c.baseURL + endpoint

	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = strings.NewReader(string(bodyBytes))
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Keep this order: the sid and the user's token carry the user's permissions, the API key is system-level.
	user := auth.UserFromContext(ctx)

	if user != nil && user.SessionID != "" {
		// Priority 1: Use Frappe session cookie (user-level permissions)
		// a request carries a cookie as name=value only: Secure, HttpOnly and SameSite belong to Set-Cookie
		req.Header.Set("Cookie", "sid="+user.SessionID)
		slog.Debug("Using sid cookie for outbound request", "user", user.Email, "method", method, "csrf_token_len", len(user.CSRFToken))
		// Frappe rejects sid-auth writes without the session's CSRF token (scraped in validateSessionCookie).
		if (method == "POST" || method == "PUT" || method == "DELETE") && user.CSRFToken != "" {
			req.Header.Set("X-Frappe-CSRF-Token", user.CSRFToken)
			slog.Debug("Set X-Frappe-CSRF-Token header", "user", user.Email, "method", method, "token_len", len(user.CSRFToken))
		}
	} else if user != nil && user.Token != "" {
		// Priority 2: Use user's OAuth2 token for user-level permissions in Frappe
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", user.Token))
		slog.Debug("Using user OAuth2 token", "user", user.Email)
	} else if c.apiKey != "" && c.apiSecret != "" {
		// Priority 3: Fall back to API key/secret if no user token
		req.Header.Set("Authorization", fmt.Sprintf("token %s:%s", c.apiKey, c.apiSecret))

		// For API key auth, bypass CSRF by setting headers
		// Frappe recognizes api/method endpoints and API key auth should bypass CSRF
		req.Header.Set("X-Frappe-CSRF-Token", "bypass")

		// Warn if using placeholder credentials
		if c.apiKey == "your_api_key_here" || c.apiSecret == "your_api_secret_here" {
			slog.Warn("Using placeholder API credentials - authentication will fail",
				"endpoint", endpoint)
		}
		slog.Debug("Using API key/secret authentication")
	} else {
		return fmt.Errorf("no authentication credentials available (no session, token, or API key)")
	}

	// Log request details (without sensitive data)
	slog.Debug("Making API request",
		"method", method,
		"endpoint", endpoint,
		"url", fullURL)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Log response details
	slog.Debug("Received API response",
		"status", resp.StatusCode,
		"body_size", len(responseBody))

	// Check for HTTP errors
	if resp.StatusCode >= 400 {
		// the message carries Frappe's one-line exception, never the raw body (a traceback, sometimes document text) or
		// the endpoint (whose query string holds rag.search's question): callers log it and pass it to the model
		var erpError types.ERPNextError
		_ = json.Unmarshal(responseBody, &erpError)
		erpError.StatusCode = resp.StatusCode
		detail := erpError.Exception
		if detail == "" {
			detail = erpError.ExcType
		}

		// If message is empty, provide a more helpful error based on status code
		if erpError.Message == "" {
			switch resp.StatusCode {
			case 401:
				erpError.Message = fmt.Sprintf("Authentication failed (HTTP %d). Please check your API credentials or OAuth2 token. %s", resp.StatusCode, detail)
			case 403:
				erpError.Message = fmt.Sprintf("Permission denied (HTTP %d). The current user/API key does not have permission for this operation. %s", resp.StatusCode, detail)
			case 404:
				erpError.Message = fmt.Sprintf("Resource not found (HTTP %d). %s", resp.StatusCode, detail)
			case 500:
				erpError.Message = fmt.Sprintf("Internal server error (HTTP %d). %s", resp.StatusCode, detail)
			default:
				erpError.Message = fmt.Sprintf("HTTP error %d. %s", resp.StatusCode, detail)
			}
		}

		// the query string holds rag.search's question and the body can hold document text
		slog.Error("Frappe API error",
			"status_code", resp.StatusCode,
			"path", strings.SplitN(endpoint, "?", 2)[0],
			"exc_type", erpError.ExcType)

		return &erpError
	}

	// Parse successful response if result pointer is provided
	if result != nil {
		if err := json.Unmarshal(responseBody, result); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}
	}

	return nil
}

// isRetryableError: a network failure or a gateway error may pass; a 500 or a 4xx will not, nor will a cancelled call
func isRetryableError(ctx context.Context, err error) bool {
	if ctx.Err() != nil {
		return false
	}
	var erpErr *types.ERPNextError
	if errors.As(err, &erpErr) {
		return erpErr.StatusCode == http.StatusBadGateway || erpErr.StatusCode == http.StatusServiceUnavailable ||
			erpErr.StatusCode == http.StatusGatewayTimeout
	}
	var netErr net.Error
	return errors.As(err, &netErr)
}

// RunAggregationQuery executes an aggregation query using frappe.client.get_list
func (c *Client) RunAggregationQuery(ctx context.Context, req types.AggregationRequest) ([]types.Document, error) {
	endpoint := "/api/method/frappe.client.get_list"

	// Frappe v16's get_list refuses "SUM(x)" strings; an aggregate is a {"SUM": field} dict
	arg := req.Field
	if strings.EqualFold(req.Metric, "count") {
		arg = "name"
	}
	fields := []interface{}{map[string]string{strings.ToUpper(req.Metric): arg, "as": "value"}}
	if req.GroupBy != "" {
		fields = append([]interface{}{req.GroupBy}, fields...)
	}
	requestBody := map[string]interface{}{
		"doctype":           req.DocType,
		"fields":            fields,
		"limit_page_length": req.TopN, // 0 is every group, not Frappe's default page of 20
	}
	if len(req.Filters) > 0 {
		requestBody["filters"] = req.Filters
	}
	if req.GroupBy != "" {
		requestBody["group_by"] = req.GroupBy
	}
	if req.TopN > 0 {
		requestBody["order_by"] = "value desc"
	}

	var response struct {
		Message []types.Document `json:"message"`
	}

	if err := c.makeRequest(ctx, "POST", endpoint, requestBody, &response); err != nil {
		return nil, fmt.Errorf("aggregation query failed for %s: %w", req.DocType, err)
	}

	slog.Info("Aggregation query executed successfully",
		"doctype", req.DocType,
		"group_by", req.GroupBy,
		"result_count", len(response.Message))

	return response.Message, nil
}

// reportRows gives every row as an object: Frappe v16's query_report.run returns objects, but a prepared report read
// from an older cache still returns arrays in column order
func reportRows(columns []types.ReportColumn, result []json.RawMessage) ([]map[string]interface{}, error) {
	rows := make([]map[string]interface{}, 0, len(result))
	for i, raw := range result {
		row := map[string]interface{}{}
		if err := json.Unmarshal(raw, &row); err == nil {
			rows = append(rows, row)
			continue
		}
		var cells []interface{}
		if err := json.Unmarshal(raw, &cells); err != nil {
			return nil, fmt.Errorf("row %d is neither an object nor an array", i)
		}
		for j, cell := range cells {
			if j < len(columns) && columns[j].FieldName != "" {
				row[columns[j].FieldName] = cell
			} else {
				row[fmt.Sprintf("column_%d", j)] = cell
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// GetCount returns the total number of docType documents matching filters.
// get_list is paginated, so only get_count yields a true total. Permissions are
// identical: both run the same DatabaseQuery with ignore_permissions unset.
func (c *Client) GetCount(ctx context.Context, docType string, filters map[string]interface{}) (int64, error) {
	// get_count reads the whole body via form_dict; a stray "limit" caps the count.
	requestBody := map[string]interface{}{"doctype": docType}
	if len(filters) > 0 {
		requestBody["filters"] = filters
	}

	var response struct {
		Message int64 `json:"message"`
	}

	if err := c.makeRequest(ctx, "POST", "/api/method/frappe.client.get_count", requestBody, &response); err != nil {
		return 0, fmt.Errorf("count query failed for %s: %w", docType, err)
	}

	slog.Info("Count query executed successfully", "doctype", docType, "count", response.Message)

	return response.Message, nil
}

// GetReportFilters fetches the filter metadata for a report
func (c *Client) GetReportFilters(ctx context.Context, reportName string) ([]types.ReportFilter, error) {
	// Use Frappe's desk.query_report.get_report_doc method to get report metadata
	endpoint := "/api/method/frappe.desk.query_report.get_report_doc"

	// Build query parameters
	queryParams := url.Values{}
	queryParams.Set("report_name", reportName)
	endpoint = endpoint + "?" + queryParams.Encode()

	var response struct {
		Message struct {
			Filters interface{} `json:"filters"` // Can be string (JSON) or array
		} `json:"message"`
	}

	if err := c.makeRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, fmt.Errorf("failed to get report metadata for %s: %w", reportName, err)
	}

	// Parse the filters - they might be a JSON string or already an array
	var filters []types.ReportFilter

	switch v := response.Message.Filters.(type) {
	case string:
		// Filters are JSON string, need to unmarshal
		if v != "" {
			if err := json.Unmarshal([]byte(v), &filters); err != nil {
				slog.Warn("Failed to parse report filters from JSON string", "report_name", reportName, "error", err)
				return []types.ReportFilter{}, nil
			}
		}
	case []interface{}:
		// Filters are already an array, convert each element
		for _, item := range v {
			if filterMap, ok := item.(map[string]interface{}); ok {
				filter := types.ReportFilter{}
				if fieldname, ok := filterMap["fieldname"].(string); ok {
					filter.FieldName = fieldname
				}
				if label, ok := filterMap["label"].(string); ok {
					filter.Label = label
				}
				if fieldtype, ok := filterMap["fieldtype"].(string); ok {
					filter.FieldType = fieldtype
				}
				if mandatory, ok := filterMap["mandatory"].(float64); ok {
					filter.Mandatory = int(mandatory)
				} else if mandatory, ok := filterMap["mandatory"].(int); ok {
					filter.Mandatory = mandatory
				}
				filter.Default = filterMap["default"]
				filters = append(filters, filter)
			}
		}
	}

	slog.Info("Report filters retrieved successfully", "report_name", reportName, "filter_count", len(filters))
	return filters, nil
}

// RunReport executes a Frappe report and returns the results
func (c *Client) RunReport(ctx context.Context, req types.ReportRequest) (*types.ReportResponse, error) {
	// Use GET request with query parameters to avoid CSRF issues with API key auth
	endpoint := "/api/method/frappe.desk.query_report.run"

	// Build query parameters
	queryParams := url.Values{}
	queryParams.Set("report_name", req.ReportName)

	// Add filters if provided - need to JSON encode them
	if len(req.Filters) > 0 {
		filtersJSON, err := json.Marshal(req.Filters)
		if err != nil {
			return nil, fmt.Errorf("failed to encode filters: %w", err)
		}
		queryParams.Set("filters", string(filtersJSON))
	}

	// Add user context if provided
	if req.User != "" {
		queryParams.Set("user", req.User)
	}

	// Append query parameters to endpoint
	if len(queryParams) > 0 {
		endpoint = endpoint + "?" + queryParams.Encode()
	}

	var response struct {
		Message struct {
			Columns []types.ReportColumn `json:"columns"`
			Result  []json.RawMessage    `json:"result"`
		} `json:"message"`
	}

	// Use GET request instead of POST to avoid CSRF token issues
	if err := c.makeRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, fmt.Errorf("report query failed for %s: %w", req.ReportName, err)
	}

	rows, err := reportRows(response.Message.Columns, response.Message.Result)
	if err != nil {
		return nil, fmt.Errorf("report %s: %w", req.ReportName, err)
	}
	result := &types.ReportResponse{
		Columns: response.Message.Columns,
		Data:    rows,
	}

	slog.Info("Report executed successfully",
		"report_name", req.ReportName,
		"columns", len(result.Columns),
		"rows", len(result.Data))

	return result, nil
}
