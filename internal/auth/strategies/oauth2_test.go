package strategies

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOAuth2Strategy(t *testing.T) {
	config := OAuth2StrategyConfig{
		TokenInfoURL:   "http://localhost:8000/userinfo",
		IssuerURL:      "http://localhost:8000",
		TrustedClients: []string{"client1", "client2"},
		Timeout:        10 * time.Second,
		CacheTTL:       5 * time.Minute,
		ValidateRemote: true,
	}

	strategy := NewOAuth2Strategy(config)

	assert.NotNil(t, strategy)
	assert.Equal(t, "http://localhost:8000/userinfo", strategy.tokenInfoURL)
	assert.Equal(t, "http://localhost:8000", strategy.issuerURL)
	assert.True(t, strategy.isTrustedClient("client1"))
	assert.True(t, strategy.isTrustedClient("client2"))
	assert.False(t, strategy.isTrustedClient("client3"))
	assert.NotNil(t, strategy.cache)
	assert.NotNil(t, strategy.httpClient)
}

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name           string
		authHeader     string
		expectedToken  string
	}{
		{
			name:          "Valid Bearer token",
			authHeader:    "Bearer abc123xyz",
			expectedToken: "abc123xyz",
		},
		{
			name:          "No Bearer prefix",
			authHeader:    "abc123xyz",
			expectedToken: "",
		},
		{
			name:          "Empty header",
			authHeader:    "",
			expectedToken: "",
		},
		{
			name:          "Bearer with space",
			authHeader:    "Bearer ",
			expectedToken: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			token := extractBearerToken(req)
			assert.Equal(t, tt.expectedToken, token)
		})
	}
}

func TestAuthenticate_MissingToken(t *testing.T) {
	strategy := NewOAuth2Strategy(OAuth2StrategyConfig{
		TokenInfoURL:   "http://localhost:8000/userinfo",
		ValidateRemote: true,
	})

	req := httptest.NewRequest("GET", "/test", nil)
	ctx := context.Background()

	user, err := strategy.Authenticate(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, user)
	assert.Contains(t, err.Error(), "missing or invalid Bearer token")
}

func TestAuthenticate_WithValidToken(t *testing.T) {
	// Create a mock OAuth2 server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Authorization header
		auth := r.Header.Get("Authorization")
		assert.Equal(t, "Bearer valid-token", auth)

		// Return mock user info
		response := map[string]interface{}{
			"sub":       "user123",
			"email":     "test@example.com",
			"name":      "Test User",
			"client_id": "test-client",
			"roles":     []string{"User", "Sales"},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	strategy := NewOAuth2Strategy(OAuth2StrategyConfig{
		TokenInfoURL:   mockServer.URL,
		ValidateRemote: true,
		Timeout:        5 * time.Second,
		CacheTTL:       1 * time.Minute,
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	ctx := context.Background()

	user, err := strategy.Authenticate(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, "user123", user.ID)
	assert.Equal(t, "test@example.com", user.Email)
	assert.Equal(t, "Test User", user.FullName)
	assert.Equal(t, "test-client", user.ClientID)
	assert.Equal(t, []string{"User", "Sales"}, user.Roles)
}

func TestAuthenticate_WithInvalidToken(t *testing.T) {
	// Create a mock OAuth2 server that returns 401
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": "invalid_token"}`))
	}))
	defer mockServer.Close()

	strategy := NewOAuth2Strategy(OAuth2StrategyConfig{
		TokenInfoURL:   mockServer.URL,
		ValidateRemote: true,
		Timeout:        5 * time.Second,
		CacheTTL:       1 * time.Minute,
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	ctx := context.Background()

	user, err := strategy.Authenticate(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, user)
	assert.Contains(t, err.Error(), "invalid token")
}

func TestAuthenticate_WithCache(t *testing.T) {
	callCount := 0

	// Create a mock OAuth2 server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		response := map[string]interface{}{
			"sub":       "user123",
			"email":     "test@example.com",
			"name":      "Test User",
			"client_id": "test-client",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	strategy := NewOAuth2Strategy(OAuth2StrategyConfig{
		TokenInfoURL:   mockServer.URL,
		ValidateRemote: true,
		Timeout:        5 * time.Second,
		CacheTTL:       1 * time.Minute,
	})

	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.Header.Set("Authorization", "Bearer cached-token")
	ctx := context.Background()

	// First call - should hit the server
	user1, err1 := strategy.Authenticate(ctx, req1)
	require.NoError(t, err1)
	require.NotNil(t, user1)
	assert.Equal(t, 1, callCount, "First call should hit the server")

	// Second call with same token - should use cache
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.Header.Set("Authorization", "Bearer cached-token")
	user2, err2 := strategy.Authenticate(ctx, req2)
	require.NoError(t, err2)
	require.NotNil(t, user2)
	assert.Equal(t, 1, callCount, "Second call should use cache")

	assert.Equal(t, user1.ID, user2.ID)
	assert.Equal(t, user1.Email, user2.Email)
}

func TestAuthenticate_WithTrustedClientHeaders(t *testing.T) {
	// Create a mock OAuth2 server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"sub":       "backend-service",
			"email":     "backend@example.com",
			"name":      "Backend Service",
			"client_id": "trusted-backend",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	strategy := NewOAuth2Strategy(OAuth2StrategyConfig{
		TokenInfoURL:   mockServer.URL,
		TrustedClients: []string{"trusted-backend"},
		ValidateRemote: true,
		Timeout:        5 * time.Second,
		CacheTTL:       1 * time.Minute,
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer backend-token")
	req.Header.Set("X-MCP-User-ID", "actual-user-123")
	req.Header.Set("X-MCP-User-Email", "actualuser@example.com")
	req.Header.Set("X-MCP-User-Name", "Actual User")
	ctx := context.Background()

	user, err := strategy.Authenticate(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, user)
	// Should use the user context from headers, not the token subject
	assert.Equal(t, "actual-user-123", user.ID)
	assert.Equal(t, "actualuser@example.com", user.Email)
	assert.Equal(t, "Actual User", user.FullName)
}

func TestAuthenticate_SkipRemoteValidation(t *testing.T) {
	strategy := NewOAuth2Strategy(OAuth2StrategyConfig{
		TokenInfoURL:   "http://localhost:8000/userinfo",
		ValidateRemote: false, // Skip remote validation
		Timeout:        5 * time.Second,
		CacheTTL:       1 * time.Minute,
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer any-token")
	ctx := context.Background()

	user, err := strategy.Authenticate(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, user)
	// Should return anonymous user when validation is skipped
	assert.Equal(t, "anonymous", user.ID)
	assert.Equal(t, "anonymous@example.com", user.Email)
}

func TestClearCache(t *testing.T) {
	strategy := NewOAuth2Strategy(OAuth2StrategyConfig{
		TokenInfoURL:   "http://localhost:8000/userinfo",
		ValidateRemote: false,
		Timeout:        5 * time.Second,
		CacheTTL:       1 * time.Minute,
	})

	// Add something to cache by authenticating
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	ctx := context.Background()

	user1, err1 := strategy.Authenticate(ctx, req)
	require.NoError(t, err1)
	require.NotNil(t, user1)

	// Verify cache is working
	user2, err2 := strategy.Authenticate(ctx, req)
	require.NoError(t, err2)
	require.NotNil(t, user2)

	// Clear cache
	strategy.ClearCache()

	// After clearing, should still work (but won't be from cache in this case
	// since we're not hitting a real server)
	user3, err3 := strategy.Authenticate(ctx, req)
	require.NoError(t, err3)
	require.NotNil(t, user3)
}

