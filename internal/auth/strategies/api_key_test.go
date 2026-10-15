package strategies

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testAPIKey is a made-up Frappe API key and secret, as "key:secret".
const testAPIKey = "examplia-key:examplia-secret" //nolint:gosec // G101: a made-up key for a fake site

// keySite is a Frappe that knows one API key, answering frappe.auth.get_logged_user as Frappe does
// (frappe/auth.py): the key's user, or Guest-free 401 for a key it does not know.
func keySite(t *testing.T, asked *atomic.Int32) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/method/frappe.auth.get_logged_user" {
			http.NotFound(w, r)
			return
		}
		asked.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") != "token "+testAPIKey {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"exc_type":"AuthenticationError"}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "writer@example.test"})
	}))
	t.Cleanup(server.Close)
	return server
}

func keyStrategy(baseURL string) *OAuth2Strategy {
	return NewOAuth2Strategy(OAuth2StrategyConfig{
		TokenInfoURL:   baseURL + "/api/method/frappe.integrations.oauth2.openid_profile",
		BaseURL:        baseURL,
		ValidateRemote: true,
		Timeout:        5 * time.Second,
		CacheTTL:       time.Minute,
	})
}

func TestAnAPIKeySignsInAsItsUser(t *testing.T) {
	var asked atomic.Int32
	site := keySite(t, &asked)
	strategy := keyStrategy(site.URL)

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "token "+testAPIKey)
	user, err := strategy.Authenticate(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, "writer@example.test", user.Email)
	assert.Equal(t, testAPIKey, user.APIKey, "kept to act as the user on every call")
	assert.Empty(t, user.Token)
	assert.Empty(t, user.SessionID)

	// a second request is answered from the cache
	_, err = strategy.Authenticate(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, int32(1), asked.Load())
}

func TestAnUnknownAPIKeyIsRefused(t *testing.T) {
	var asked atomic.Int32
	site := keySite(t, &asked)
	strategy := keyStrategy(site.URL)

	for _, header := range []string{"token examplia-key:wrong", "token not-a-pair", "token "} {
		req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		req.Header.Set("Authorization", header)
		user, err := strategy.Authenticate(context.Background(), req)
		assert.Error(t, err, header)
		assert.Nil(t, user, header)
	}
}

func TestAPIKeyCacheKeyHoldsNoSecret(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	key := tokenCacheKeyForSite("api-key\x00"+testAPIKey, "site.test", req)
	assert.NotContains(t, key, "examplia")
	assert.NotEqual(t, key, tokenCacheKeyForSite(testAPIKey, "site.test", req),
		"an API key and a Bearer token with the same text must not share a cache entry")
}
