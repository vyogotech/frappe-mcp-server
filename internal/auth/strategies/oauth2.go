package strategies

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"frappe-mcp-server/internal/types"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
)

// OAuth2Strategy handles OAuth2 token validation
type OAuth2Strategy struct {
	tokenInfoURL   string
	issuerURL      string
	trustedClients map[string]bool
	cache          *cache.Cache
	httpClient     *http.Client
	validateRemote bool
	mu             sync.RWMutex
}

// OAuth2StrategyConfig represents configuration for OAuth2Strategy
type OAuth2StrategyConfig struct {
	TokenInfoURL   string
	IssuerURL      string
	TrustedClients []string
	Timeout        time.Duration
	CacheTTL       time.Duration
	ValidateRemote bool
}

// NewOAuth2Strategy creates a new OAuth2Strategy
func NewOAuth2Strategy(config OAuth2StrategyConfig) *OAuth2Strategy {
	trustedMap := make(map[string]bool)
	for _, client := range config.TrustedClients {
		trustedMap[client] = true
	}

	// Set defaults
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.CacheTTL == 0 {
		config.CacheTTL = 5 * time.Minute
	}

	return &OAuth2Strategy{
		tokenInfoURL:   config.TokenInfoURL,
		issuerURL:      config.IssuerURL,
		trustedClients: trustedMap,
		cache:          cache.New(config.CacheTTL, config.CacheTTL*2),
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		validateRemote: config.ValidateRemote,
	}
}

// Authenticate authenticates a request and returns user information
// Supports both sid cookie (Frappe session) and Bearer token (OAuth2)
func (s *OAuth2Strategy) Authenticate(ctx context.Context, r *http.Request) (*types.User, error) {
	// Strategy 1: Try sid cookie first (Frappe session - user-level permissions)
	if sidCookie, err := r.Cookie("sid"); err == nil && sidCookie.Value != "" {
		user, err := s.validateSessionCookie(ctx, sidCookie)
		if err == nil {
			// Cache the validated user
			s.cache.Set("sid:"+sidCookie.Value, user, cache.DefaultExpiration)
			return user, nil
		}
		// If sid validation fails, continue to try Bearer token
	}

	// Strategy 2: Try Bearer token (OAuth2)
	token := extractBearerToken(r)
	if token == "" {
		return nil, errors.New("missing authentication: no sid cookie or Bearer token found")
	}

	// Check cache first
	if cached, found := s.cache.Get(token); found {
		if user, ok := cached.(*types.User); ok {
			return user, nil
		}
	}

	// Validate token with OAuth2 provider (Frappe)
	user, clientID, err := s.validateToken(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("token validation failed: %w", err)
	}

	// If from trusted backend client, check for user context headers
	if s.isTrustedClient(clientID) {
		if userID := r.Header.Get("X-MCP-User-ID"); userID != "" {
			user = s.extractUserFromHeaders(r)
		}
	}

	// Cache the validated user
	s.cache.Set(token, user, cache.DefaultExpiration)

	return user, nil
}

// extractBearerToken extracts the Bearer token from the Authorization header
func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

// validateToken validates the token with the OAuth2 provider
func (s *OAuth2Strategy) validateToken(ctx context.Context, token string) (*types.User, string, error) {
	if !s.validateRemote {
		// Skip remote validation (for development or if using JWT validation)
		return &types.User{
			ID:    "anonymous",
			Email: "anonymous@example.com",
		}, "", nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", s.tokenInfoURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to validate token: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("invalid token: status %d", resp.StatusCode)
	}

	var tokenInfo struct {
		Sub      string   `json:"sub"`
		Email    string   `json:"email"`
		Name     string   `json:"name"`
		ClientID string   `json:"client_id"`
		Roles    []string `json:"roles"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenInfo); err != nil {
		return nil, "", fmt.Errorf("failed to decode token info: %w", err)
	}

	user := &types.User{
		ID:       tokenInfo.Sub,
		Email:    tokenInfo.Email,
		FullName: tokenInfo.Name,
		ClientID: tokenInfo.ClientID,
		Roles:    tokenInfo.Roles,
		Token:    token, // Store the OAuth2 token for pass-through to ERPNext
	}

	return user, tokenInfo.ClientID, nil
}

// extractUserFromHeaders extracts user information from trusted client headers
func (s *OAuth2Strategy) extractUserFromHeaders(r *http.Request) *types.User {
	return &types.User{
		ID:       r.Header.Get("X-MCP-User-ID"),
		Email:    r.Header.Get("X-MCP-User-Email"),
		FullName: r.Header.Get("X-MCP-User-Name"),
	}
}

// isTrustedClient checks if a client is trusted
func (s *OAuth2Strategy) isTrustedClient(clientID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.trustedClients[clientID]
}

// validateSessionCookie validates a Frappe session cookie (sid)
func (s *OAuth2Strategy) validateSessionCookie(ctx context.Context, sidCookie *http.Cookie) (*types.User, error) {
	// Check cache first
	cacheKey := "sid:" + sidCookie.Value
	if cached, found := s.cache.Get(cacheKey); found {
		if user, ok := cached.(*types.User); ok {
			return user, nil
		}
	}

	if !s.validateRemote {
		// Skip remote validation (for development)
		return &types.User{
			ID:        "anonymous",
			Email:     "anonymous@example.com",
			SessionID: sidCookie.Value,
		}, nil
	}

	// Validate session with Frappe by calling /api/method/frappe.auth.get_logged_user
	req, err := http.NewRequestWithContext(ctx, "GET",
		s.issuerURL+"/api/method/frappe.auth.get_logged_user", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create session validation request: %w", err)
	}

	// Add the sid cookie to the request
	req.AddCookie(sidCookie)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("session validation failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("invalid session: status %d", resp.StatusCode)
	}

	var result struct {
		Message string `json:"message"` // The user email
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode session info: %w", err)
	}

	user := &types.User{
		ID:        result.Message,
		Email:     result.Message,
		SessionID: sidCookie.Value, // Store for pass-through to Frappe API calls
	}

	return user, nil
}

// ClearCache clears the token cache
func (s *OAuth2Strategy) ClearCache() {
	s.cache.Flush()
}

