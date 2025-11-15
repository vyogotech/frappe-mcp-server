package frappe

// Package frappe provides a client for the Frappe Framework REST API.
// This client works with ANY Frappe-based application including ERPNext,
// Frappe HR, Healthcare, Education, and custom Frappe apps.
// The API endpoints used are generic Frappe Framework endpoints:
//   - /api/resource/{doctype}           - CRUD operations
//   - /api/method/frappe.desk.search.*  - Search operations

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

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
	cache       sync.Map // Simple in-memory cache
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

	// Create HTTP client with connection pooling
	transport := &http.Transport{
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
		MaxIdleConnsPerHost: 10,
	}

	httpClient := &http.Client{
		Transport: transport,
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
	// Check cache first
	cacheKey := fmt.Sprintf("doc:%s:%s", docType, name)
	if cached, ok := c.cache.Load(cacheKey); ok {
		if doc, ok := cached.(types.Document); ok {
			slog.Debug("Document retrieved from cache", "doctype", docType, "name", name)
			return doc, nil
		}
	}

	endpoint := fmt.Sprintf("/api/resource/%s/%s", url.PathEscape(docType), url.PathEscape(name))

	var response struct {
		Data types.Document `json:"data"`
	}

	if err := c.makeRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, fmt.Errorf("failed to get document %s/%s: %w", docType, name, err)
	}

	// Cache the result
	c.cache.Store(cacheKey, response.Data)

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

	// Invalidate cache for this doctype
	c.invalidateCache(req.DocType)

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

	// Invalidate cache for this specific document
	cacheKey := fmt.Sprintf("doc:%s:%s", req.DocType, req.Name)
	c.cache.Delete(cacheKey)

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

	// Invalidate cache
	cacheKey := fmt.Sprintf("doc:%s:%s", docType, name)
	c.cache.Delete(cacheKey)

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

// makeRequest makes an HTTP request to Frappe API with retry logic
func (c *Client) makeRequest(ctx context.Context, method, endpoint string, body interface{}, result interface{}) error {
	// Apply rate limiting
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limit error: %w", err)
	}

	var attempt int
	for attempt = 0; attempt < c.retryConfig.MaxAttempts; attempt++ {
		if attempt > 0 {
			// Calculate delay for retry with exponential backoff
			delay := time.Duration(attempt) * c.retryConfig.InitialDelay
			if delay > c.retryConfig.MaxDelay {
				delay = c.retryConfig.MaxDelay
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}

			slog.Debug("Retrying request", "attempt", attempt+1, "delay", delay)
		}

		err := c.doRequest(ctx, method, endpoint, body, result)
		if err == nil {
			return nil
		}

		// Check if error is retryable
		if !isRetryableError(err) {
			return err
		}

		slog.Warn("Request failed, will retry", "error", err, "attempt", attempt+1)
	}

	return fmt.Errorf("request failed after %d attempts", attempt)
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
	
	// Authentication priority:
	// 1. sid cookie (user session) - best, user-level permissions
	// 2. OAuth2 Bearer token - good, can be user or system level
	// 3. API key/secret - fallback, system-level permissions
	
	user := auth.UserFromContext(ctx)
	
	if user != nil && user.SessionID != "" {
		// Priority 1: Use Frappe session cookie (user-level permissions)
		req.AddCookie(&http.Cookie{
			Name:  "sid",
			Value: user.SessionID,
		})
		slog.Info("DEBUG Client: Using sid cookie", "user", user.Email, "method", method, "csrf_token_len", len(user.CSRFToken))
		// Add CSRF token for POST/PUT/DELETE requests
		if method == "POST" || method == "PUT" || method == "DELETE" {
			if user.CSRFToken != "" {
				req.Header.Set("X-Frappe-CSRF-Token", user.CSRFToken)
				slog.Info("DEBUG Client: Set CSRF token header", "user", user.Email, "method", method, "token", user.CSRFToken[:min(20, len(user.CSRFToken))]+"...")
			} else {
				slog.Error("CSRF token missing for mutating operation", "user", user.Email, "method", method, "endpoint", endpoint)
				return fmt.Errorf("CSRF token required for %s operation but not available for user %s. This usually indicates a session validation issue", method, user.Email)
			}
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
				"api_key", c.apiKey,
				"api_secret", c.apiSecret,
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
		var erpError types.ERPNextError
		if err := json.Unmarshal(responseBody, &erpError); err != nil {
			// If we can't parse the error, create a generic one
			erpError = types.ERPNextError{
				Message:    string(responseBody),
				StatusCode: resp.StatusCode,
			}
		}
		erpError.StatusCode = resp.StatusCode
		
		// If message is empty, provide a more helpful error based on status code
		if erpError.Message == "" {
			switch resp.StatusCode {
			case 401:
				erpError.Message = fmt.Sprintf("Authentication failed (HTTP %d). Please check your API credentials or OAuth2 token. Raw response: %s", resp.StatusCode, string(responseBody))
			case 403:
				erpError.Message = fmt.Sprintf("Permission denied (HTTP %d). The current user/API key does not have permission for this operation. Raw response: %s", resp.StatusCode, string(responseBody))
			case 404:
				erpError.Message = fmt.Sprintf("Resource not found (HTTP %d). Endpoint: %s. Raw response: %s", resp.StatusCode, endpoint, string(responseBody))
			case 500:
				erpError.Message = fmt.Sprintf("Internal server error (HTTP %d). Raw response: %s", resp.StatusCode, string(responseBody))
			default:
				erpError.Message = fmt.Sprintf("HTTP error %d. Raw response: %s", resp.StatusCode, string(responseBody))
			}
		}
		
		slog.Error("Frappe API error",
			"status_code", resp.StatusCode,
			"endpoint", endpoint,
			"message", erpError.Message,
			"response_body", string(responseBody))
		
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

// isRetryableError determines if an error is retryable
func isRetryableError(err error) bool {
	if erpErr, ok := err.(*types.ERPNextError); ok {
		// Retry on server errors but not client errors
		return erpErr.StatusCode >= 500
	}
	// Retry on network errors
	return true
}

// invalidateCache removes all cache entries for a given doctype
func (c *Client) invalidateCache(docType string) {
	prefix := fmt.Sprintf("doc:%s:", docType)
	c.cache.Range(func(key, value interface{}) bool {
		if keyStr, ok := key.(string); ok && strings.HasPrefix(keyStr, prefix) {
			c.cache.Delete(key)
		}
		return true
	})
}

// ClearCache clears all cached data
func (c *Client) ClearCache() {
	c.cache.Range(func(key, value interface{}) bool {
		c.cache.Delete(key)
		return true
	})
}

// RunAggregationQuery executes an aggregation query using frappe.client.get_list
func (c *Client) RunAggregationQuery(ctx context.Context, req types.AggregationRequest) ([]types.Document, error) {
	endpoint := "/api/method/frappe.client.get_list"
	
	// Build request body
	requestBody := map[string]interface{}{
		"doctype": req.DocType,
	}
	
	// Add fields (for aggregation like SUM, COUNT, etc.)
	if len(req.Fields) > 0 {
		requestBody["fields"] = req.Fields
	}
	
	// Add filters
	if len(req.Filters) > 0 {
		requestBody["filters"] = req.Filters
	}
	
	// Add group by
	if req.GroupBy != "" {
		requestBody["group_by"] = req.GroupBy
	}
	
	// Add order by
	if req.OrderBy != "" {
		requestBody["order_by"] = req.OrderBy
	}
	
	// Add limit
	if req.Limit > 0 {
		requestBody["limit_page_length"] = req.Limit
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

// GetReportFilters fetches the filter metadata for a report
func (c *Client) GetReportFilters(ctx context.Context, reportName string) ([]types.ReportFilter, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("report_filters:%s", reportName)
	if cached, ok := c.cache.Load(cacheKey); ok {
		if filters, ok := cached.([]types.ReportFilter); ok {
			slog.Debug("Report filters retrieved from cache", "report_name", reportName)
			return filters, nil
		}
	}

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
	
	// Cache the result for 5 minutes
	c.cache.Store(cacheKey, filters)
	
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
			Result  [][]interface{}      `json:"result"`
		} `json:"message"`
	}
	
	// Use GET request instead of POST to avoid CSRF token issues
	if err := c.makeRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, fmt.Errorf("report query failed for %s: %w", req.ReportName, err)
	}
	
	result := &types.ReportResponse{
		Columns: response.Message.Columns,
		Data:    response.Message.Result,
	}
	
	slog.Info("Report executed successfully",
		"report_name", req.ReportName,
		"columns", len(result.Columns),
		"rows", len(result.Data))
	
	return result, nil
}
