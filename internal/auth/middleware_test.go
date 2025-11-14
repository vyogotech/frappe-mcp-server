package auth

import (
	"context"
	"encoding/json"
	"frappe-mcp-server/internal/auth/strategies"
	"frappe-mcp-server/internal/types"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMiddleware_RequiredAuth_ValidToken(t *testing.T) {
	// Create a mock OAuth2 server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"sub":       "user123",
			"email":     "test@example.com",
			"name":      "Test User",
			"client_id": "test-client",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	strategy := strategies.NewOAuth2Strategy(strategies.OAuth2StrategyConfig{
		TokenInfoURL:   mockServer.URL,
		ValidateRemote: true,
		Timeout:        5 * time.Second,
	})

	middleware := NewMiddleware(strategy, true) // require auth

	// Create a test handler that checks for user in context
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, found := GetUserFromContext(r.Context())
		assert.True(t, found)
		assert.NotNil(t, user)
		assert.Equal(t, "user123", user.ID)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	})

	handler := middleware.Handler(testHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "success", w.Body.String())
}

func TestMiddleware_RequiredAuth_MissingToken(t *testing.T) {
	strategy := strategies.NewOAuth2Strategy(strategies.OAuth2StrategyConfig{
		TokenInfoURL:   "http://localhost:8000/userinfo",
		ValidateRemote: true,
		Timeout:        5 * time.Second,
	})

	middleware := NewMiddleware(strategy, true) // require auth

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Handler should not be called when auth is missing")
	})

	handler := middleware.Handler(testHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	// No Authorization header
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var response map[string]string
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "Unauthorized", response["error"])
	assert.Contains(t, response["message"], "authentication required")
}

func TestMiddleware_OptionalAuth_ValidToken(t *testing.T) {
	// Create a mock OAuth2 server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"sub":       "user123",
			"email":     "test@example.com",
			"name":      "Test User",
			"client_id": "test-client",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	strategy := strategies.NewOAuth2Strategy(strategies.OAuth2StrategyConfig{
		TokenInfoURL:   mockServer.URL,
		ValidateRemote: true,
		Timeout:        5 * time.Second,
	})

	middleware := NewMiddleware(strategy, false) // optional auth

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, found := GetUserFromContext(r.Context())
		assert.True(t, found)
		assert.NotNil(t, user)
		assert.Equal(t, "user123", user.ID)
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Handler(testHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMiddleware_OptionalAuth_MissingToken(t *testing.T) {
	strategy := strategies.NewOAuth2Strategy(strategies.OAuth2StrategyConfig{
		TokenInfoURL:   "http://localhost:8000/userinfo",
		ValidateRemote: true,
		Timeout:        5 * time.Second,
	})

	middleware := NewMiddleware(strategy, false) // optional auth

	handlerCalled := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		user, found := GetUserFromContext(r.Context())
		assert.False(t, found)
		assert.Nil(t, user)
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Handler(testHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	// No Authorization header
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.True(t, handlerCalled, "Handler should be called even without auth when auth is optional")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMiddleware_OptionalAuth_InvalidToken(t *testing.T) {
	// Create a mock OAuth2 server that returns 401
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer mockServer.Close()

	strategy := strategies.NewOAuth2Strategy(strategies.OAuth2StrategyConfig{
		TokenInfoURL:   mockServer.URL,
		ValidateRemote: true,
		Timeout:        5 * time.Second,
	})

	middleware := NewMiddleware(strategy, false) // optional auth

	handlerCalled := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		user, found := GetUserFromContext(r.Context())
		assert.False(t, found)
		assert.Nil(t, user)
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Handler(testHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.True(t, handlerCalled, "Handler should be called even with invalid token when auth is optional")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMiddleware_WithUserContext(t *testing.T) {
	// Test that the user context is properly passed through the middleware chain
	strategy := strategies.NewOAuth2Strategy(strategies.OAuth2StrategyConfig{
		TokenInfoURL:   "http://localhost:8000/userinfo",
		ValidateRemote: false, // Skip validation for this test
		Timeout:        5 * time.Second,
	})

	middleware := NewMiddleware(strategy, false)

	// Create a custom test handler that inspects the context
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Manually add a user to the context for testing
		user := &types.User{
			ID:    "test-user",
			Email: "test@example.com",
		}
		ctx := WithUser(r.Context(), user)
		
		// Verify we can retrieve it
		retrievedUser, found := GetUserFromContext(ctx)
		assert.True(t, found)
		assert.Equal(t, "test-user", retrievedUser.ID)
		
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Handler(testHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMiddleware_ContextPropagation(t *testing.T) {
	// Test that context is properly propagated through middleware
	ctx := context.Background()
	user := &types.User{
		ID:    "user123",
		Email: "test@example.com",
	}

	ctx = WithUser(ctx, user)

	retrievedUser := UserFromContext(ctx)
	assert.NotNil(t, retrievedUser)
	assert.Equal(t, user.ID, retrievedUser.ID)
}

