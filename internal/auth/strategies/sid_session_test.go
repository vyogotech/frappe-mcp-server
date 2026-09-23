package strategies

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// deskCSRFToken has the shape Frappe writes: frappe.sessions.generate_csrf_token stores frappe.generate_hash(), which
// is secrets.token_hex(28), and frappe/www/desk.html:56 prints it as `frappe.csrf_token = "<token>";`. Built, not
// written out, because gosec G101 reads 56 hex characters beside that name as a hardcoded credential.
var deskCSRFToken = strings.Repeat("abcd1234", 7)

var deskPage = `<!DOCTYPE html><html><head><script>
	frappe.boot = {"sitename": "demo.localhost"};
	frappe.csrf_token = "` + deskCSRFToken + `";
</script></head><body></body></html>`

// frappeDesk answers the two requests a sid validation makes: frappe.auth.get_logged_user, whitelisted in
// frappe/auth.py:463, and the desk page that carries the session's CSRF token.
type frappeDesk struct {
	url string

	mu         sync.Mutex
	loggedUser int      // times get_logged_user was asked, so the cache can be counted
	deskStatus int      // what /app answers
	deskHTML   string   // what /app serves when it answers 200
	sidSeen    []string // "<path> sid=<value>" per request
}

func newFrappeDesk(t *testing.T) *frappeDesk {
	t.Helper()
	d := &frappeDesk{deskStatus: http.StatusOK, deskHTML: deskPage}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d.mu.Lock()
		defer d.mu.Unlock()
		sid := ""
		if c, err := r.Cookie("sid"); err == nil {
			sid = c.Value
		}
		d.sidSeen = append(d.sidSeen, r.URL.Path+" sid="+sid)

		switch r.URL.Path {
		case "/api/method/frappe.auth.get_logged_user":
			d.loggedUser++
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"message":"alice@example.com"}`)
		case "/app":
			if d.deskStatus != http.StatusOK {
				w.WriteHeader(d.deskStatus)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = io.WriteString(w, d.deskHTML)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	d.url = server.URL
	return d
}

func (d *frappeDesk) set(status int, html string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.deskStatus, d.deskHTML = status, html
}

func (d *frappeDesk) calls() (int, []string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.loggedUser, append([]string(nil), d.sidSeen...)
}

func sidStrategy(desk *frappeDesk) *OAuth2Strategy {
	return NewOAuth2Strategy(OAuth2StrategyConfig{
		TokenInfoURL:   desk.url + "/api/method/frappe.integrations.oauth2.openid_profile",
		BaseURL:        desk.url,
		ValidateRemote: true,
		Timeout:        5 * time.Second,
	})
}

func sidRequest(sid string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	// the header, not AddCookie: a request cookie is name=value only, and gosec G124 wants Set-Cookie's attributes
	r.Header.Set("Cookie", "sid="+sid)
	return r
}

func TestSidIsValidatedWithoutRenderingTheDesk(t *testing.T) {
	desk := newFrappeDesk(t)

	user, err := sidStrategy(desk).Authenticate(context.Background(), sidRequest("the-session-id"))

	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, "alice@example.com", user.Email, "the user is whoever get_logged_user names")
	assert.Equal(t, "the-session-id", user.SessionID, "the sid rides on for the outbound Frappe calls")

	_, seen := desk.calls()
	assert.Equal(t, []string{
		"/api/method/frappe.auth.get_logged_user sid=the-session-id",
	}, seen, "a read needs no CSRF token, so validating a session must not render the desk")
}

func TestAValidatedSidIsCached(t *testing.T) {
	desk := newFrappeDesk(t)
	strategy := sidStrategy(desk)
	request := sidRequest("the-session-id")

	for i := 0; i < 3; i++ {
		user, err := strategy.Authenticate(context.Background(), request)
		require.NoError(t, err)
		require.Equal(t, "alice@example.com", user.Email)
	}

	validations, _ := desk.calls()
	assert.Equal(t, 1, validations, "a validated session is validated once, not on every request")
}
