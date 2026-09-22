package frappe

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"

	"frappe-mcp-server/internal/config"

	"github.com/stretchr/testify/require"
)

// Frappe calls go through the proxy the environment names, as the session check already does. Go reads the proxy
// variables once per process, so the call runs in a child process that starts with them set.
func TestFrappeCallsUseTheEnvironmentsProxy(t *testing.T) {
	if os.Getenv("C42_CHILD") == "1" {
		c, err := NewClient(config.ERPNextConfig{BaseURL: "http://frappe.proxy-test.invalid", APIKey: "k", APISecret: "s",
			RateLimit: config.RateLimitConfig{RequestsPerSecond: 1000, Burst: 1000}})
		require.NoError(t, err)
		_, err = c.GetDocument(context.Background(), "Note", "N-1")
		require.NoError(t, err)
		return
	}
	var asked string
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = r.Host
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": {"name": "N-1"}}`))
	}))
	defer proxy.Close()

	cmd := exec.Command(os.Args[0], "-test.run=^TestFrappeCallsUseTheEnvironmentsProxy$") //nolint:gosec // this test binary
	cmd.Env = append(os.Environ(), "C42_CHILD=1", "HTTP_PROXY="+proxy.URL, "NO_PROXY=")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	require.Equal(t, "frappe.proxy-test.invalid", asked)
}
