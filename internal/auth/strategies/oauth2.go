package strategies

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"frappe-mcp-server/internal/types"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
)

// csrfTokenPattern scrapes a sid's CSRF token from the desk HTML: frappe.sessions.get_csrf_token is not whitelisted.
var csrfTokenPattern = regexp.MustCompile(`csrf_token\s*=\s*"([a-f0-9]{20,64})"`)

type OAuth2Strategy struct {
	tokenInfoURL   string
	issuerURL      string
	trustedClients map[string]bool
	cache          *cache.Cache
	httpClient     *http.Client
	validateRemote bool
	mu             sync.RWMutex
}

type OAuth2StrategyConfig struct {
	TokenInfoURL   string
	IssuerURL      string
	TrustedClients []string
	Timeout        time.Duration
	CacheTTL       time.Duration
	ValidateRemote bool
}

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

// Authenticate tries the sid cookie first, then the Bearer token.
func (s *OAuth2Strategy) Authenticate(ctx context.Context, r *http.Request) (*types.User, error) {
	// Strategy 1: Try sid cookie first (Frappe session - user-level permissions)
	var sidErr error
	if sidCookie, err := r.Cookie("sid"); err == nil && sidCookie.Value != "" {
		// Check cache first
		cacheKey := "sid:" + sidCookie.Value
		if cached, found := s.cache.Get(cacheKey); found {
			if user, ok := cached.(*types.User); ok {
				slog.Debug("Using cached user for sid", "csrf_token_len", strconv.Itoa(len(user.CSRFToken)))
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
		sidErr = err
	}

	// Strategy 2: Try Bearer token (OAuth2)
	token := extractBearerToken(r)
	if token == "" {
		if sidErr != nil {
			// keep why the sid failed: Frappe down, a timeout and an expired session need different fixes
			return nil, fmt.Errorf("missing or invalid Bearer token, and the sid was not accepted: %w", sidErr)
		}
		return nil, errors.New("missing or invalid Bearer token")
	}

	// Cache key binds the token AND the X-MCP-User-* impersonation headers so a
	// trusted backend client cannot retrieve a different user's cached identity
	// by reusing the same token. See tokenCacheKey.
	cacheKey := tokenCacheKey(token, r)

	// Check cache first
	if cached, found := s.cache.Get(cacheKey); found {
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
	s.cache.Set(cacheKey, user, cache.DefaultExpiration)

	return user, nil
}

func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

// tokenCacheKey must hash the X-MCP-User-* headers with the token: a trusted client reusing one token for two users
// would otherwise get the first user's cached identity.
func tokenCacheKey(token string, r *http.Request) string {
	h := sha256.New()
	h.Write([]byte(token))
	h.Write([]byte{0})
	h.Write([]byte(r.Header.Get("X-MCP-User-ID")))
	h.Write([]byte{0})
	h.Write([]byte(r.Header.Get("X-MCP-User-Email")))
	h.Write([]byte{0})
	h.Write([]byte(r.Header.Get("X-MCP-User-Name")))
	return hex.EncodeToString(h.Sum(nil))
}

// validateToken accepts any token as an anonymous user unless validate_remote is on.
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

func (s *OAuth2Strategy) extractUserFromHeaders(r *http.Request) *types.User {
	return &types.User{
		ID:       r.Header.Get("X-MCP-User-ID"),
		Email:    r.Header.Get("X-MCP-User-Email"),
		FullName: r.Header.Get("X-MCP-User-Name"),
	}
}

func (s *OAuth2Strategy) isTrustedClient(clientID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.trustedClients[clientID]
}

// validateSessionCookie accepts any sid as an anonymous user unless validate_remote is on.
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

	// Writes under sid auth need the session's CSRF token, and get_logged_user does not return it.
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

// fetchCSRFToken reads the sid's CSRF token out of the desk page's inline JS.
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

func (s *OAuth2Strategy) ClearCache() {
	s.cache.Flush()
}
