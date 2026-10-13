package strategies

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

// ErrFrappeUnavailable marks a session Frappe never judged, because it did not answer: an outage, not a rejection.
var ErrFrappeUnavailable = errors.New("frappe did not answer")

type OAuth2Strategy struct {
	tokenInfoURL   string
	baseURL        string
	trustedClients map[string]bool
	cache          *cache.Cache
	httpClient     *http.Client
	validateRemote bool
	mu             sync.RWMutex
}

type OAuth2StrategyConfig struct {
	TokenInfoURL string
	// BaseURL is the Frappe site the sessions belong to: a sid is valid only on the site that issued it.
	BaseURL        string
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
		baseURL:        config.BaseURL,
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
		user, err := s.validateSessionCookie(ctx, sidCookie)
		if err == nil {
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
	cacheKey := tokenCacheKeyForSite(token, types.SiteFromContext(ctx), r)

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
	return tokenCacheKeyForSite(token, types.SiteFromContext(r.Context()), r)
}

// tokenCacheKeyForSite also binds the tenant site: a token introspected on one bench must not answer for another.
func tokenCacheKeyForSite(token, site string, r *http.Request) string {
	h := sha256.New()
	h.Write([]byte(site))
	h.Write([]byte{0})
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

	req, err := http.NewRequestWithContext(ctx, "GET", s.tokenInfoURLFor(ctx), nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	if site := types.SiteFromContext(ctx); site != "" {
		req.Header.Set("X-Frappe-Site-Name", site)
	}

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

// tokenInfoURLFor is the introspection endpoint for this request's tenant. An endpoint on the configured Frappe base
// URL follows the tenant's routed bench, so a token is judged by the site that issued it; an endpoint elsewhere (a
// separate identity provider) is the same for every tenant and is used as configured.
func (s *OAuth2Strategy) tokenInfoURLFor(ctx context.Context) string {
	routed := strings.TrimSuffix(types.BaseURLFromContext(ctx), "/")
	configured := strings.TrimSuffix(s.baseURL, "/")
	if routed == "" || configured == "" || routed == configured {
		return s.tokenInfoURL
	}
	if path, onBase := strings.CutPrefix(s.tokenInfoURL, configured); onBase && strings.HasPrefix(path, "/") {
		return routed + path
	}
	return s.tokenInfoURL
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

func (s *OAuth2Strategy) validateSessionCookie(ctx context.Context, sidCookie *http.Cookie) (*types.User, error) {
	site := types.SiteFromContext(ctx)
	cacheKey := types.SidCacheKey(site, sidCookie.Value)
	if cached, found := s.cache.Get(cacheKey); found {
		if user, ok := cached.(*types.User); ok {
			return user, nil
		}
	}

	if !s.validateRemote {
		// Skip remote validation (for development)
		user := &types.User{
			ID:        "anonymous",
			Email:     "anonymous@example.com",
			SessionID: sidCookie.Value,
		}
		s.cache.Set(cacheKey, user, cache.DefaultExpiration)
		return user, nil
	}

	baseURL := s.baseURL
	if u := types.BaseURLFromContext(ctx); u != "" {
		baseURL = u
	}

	// Validate session with Frappe by calling /api/method/frappe.auth.get_logged_user
	req, err := http.NewRequestWithContext(ctx, "GET",
		strings.TrimSuffix(baseURL, "/")+"/api/method/frappe.auth.get_logged_user", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create session validation request: %w", err)
	}

	// Add the sid cookie to the request
	req.AddCookie(sidCookie)
	if site != "" {
		req.Header.Set("X-Frappe-Site-Name", site)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("session validation failed: %w: %w", ErrFrappeUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode >= 500 {
			return nil, fmt.Errorf("session validation failed: status %d: %w", resp.StatusCode, ErrFrappeUnavailable)
		}
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
	s.cache.Set(cacheKey, user, cache.DefaultExpiration)
	return user, nil
}

func (s *OAuth2Strategy) ClearCache() {
	s.cache.Flush()
}
