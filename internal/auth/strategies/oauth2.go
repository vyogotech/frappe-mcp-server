package strategies

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"frappe-mcp-server/internal/types"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
)

// csrfTokenPattern matches the `csrf_token = "<hex>"` assignment embedded in
// Frappe's desk HTML. Frappe only emits this on desk page loads, and
// `frappe.sessions.get_csrf_token` is not whitelisted — so scraping the desk
// HTML is the only first-party way to obtain the token for a known sid.
var csrfTokenPattern = regexp.MustCompile(`csrf_token\s*=\s*"([a-f0-9]{20,64})"`)

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
		// Check cache first
		cacheKey := "sid:" + sidCookie.Value
		if cached, found := s.cache.Get(cacheKey); found {
			if user, ok := cached.(*types.User); ok {
				slog.Debug("Using cached user for sid", "csrf_token_len", len(user.CSRFToken)) //nolint:gosec // G706 false positive: slog structured-log key is a string literal, not user input
				return user, nil
			}
		}

		// Validate session and get CSRF token from Frappe
		user, err := s.validateSessionCookie(ctx, sidCookie)
		if err == nil {
			slog.Debug("Session validation successful", "csrf_token_len", len(user.CSRFToken))
			// Cache the validated user with shorter expiration for CSRF token freshness
			// CSRF tokens can expire, so use 2 minutes instead of default 5 minutes
			s.cache.Set(cacheKey, user, 2*time.Minute)
			return user, nil
		}
		// If sid validation fails, continue to try Bearer token
	}

	// Strategy 2: Try Bearer token (OAuth2)
	token := extractBearerToken(r)
	if token == "" {
		return nil, errors.New("missing or invalid Bearer token")
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

	// Frappe enforces CSRF on POST/PUT/DELETE under sid auth once
	// frappe.session.data.csrf_token is populated server-side, which happens
	// lazily on desk page render. `/api/method/frappe.auth.get_logged_user`
	// does NOT emit X-Frappe-CSRF-Token, and frappe.sessions.get_csrf_token is
	// not whitelisted, so we scrape the token out of the desk HTML.
	csrfToken, err := s.fetchCSRFToken(ctx, sidCookie)
	if err != nil {
		// Don't fail auth — reads still work without a CSRF token. Writes will
		// hit CSRFTokenError downstream, which is already the existing broken
		// behaviour; this way a CSRF-fetch outage doesn't take down GETs too.
		slog.Warn("validateSession: failed to fetch CSRF token; writes will fail", "error", err)
	}

	user := &types.User{
		ID:        result.Message,
		Email:     result.Message,
		SessionID: sidCookie.Value, // Store for pass-through to Frappe API calls
		CSRFToken: csrfToken,
	}

	slog.Debug("validateSession: created user", "csrf_token_len", len(user.CSRFToken))

	return user, nil
}

// fetchCSRFToken retrieves the per-session CSRF token by requesting the desk
// page with the sid cookie and pulling the token out of the embedded JS.
func (s *OAuth2Strategy) fetchCSRFToken(ctx context.Context, sidCookie *http.Cookie) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", s.issuerURL+"/app", nil)
	if err != nil {
		return "", fmt.Errorf("build desk request: %w", err)
	}
	req.AddCookie(sidCookie)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("desk request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("desk returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read desk body: %w", err)
	}

	m := csrfTokenPattern.FindSubmatch(body)
	if m == nil {
		return "", errors.New("csrf_token not found in desk HTML")
	}
	return string(m[1]), nil
}

// ClearCache clears the token cache
func (s *OAuth2Strategy) ClearCache() {
	s.cache.Flush()
}

