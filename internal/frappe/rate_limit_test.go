package frappe

import (
	"context"
	"net/http"
	"testing"
	"time"

	"frappe-mcp-server/internal/auth"
	"frappe-mcp-server/internal/types"
)

// One user's loop must not spend another user's rate-limit budget.
func TestRateLimitIsPerUser(t *testing.T) {
	client := limitedClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"name":"PROJ-0001"}}`))
	}, 1, 1) // one request per second, one token in the bucket

	alice := auth.WithUser(context.Background(), &types.User{Email: "alice@example.com", SessionID: "sid-a"})
	bob := auth.WithUser(context.Background(), &types.User{Email: "bob@example.com", SessionID: "sid-b"})

	if _, err := client.GetDocument(alice, "Project", "PROJ-0001"); err != nil {
		t.Fatalf("alice: %v", err)
	}

	start := time.Now()
	if _, err := client.GetDocument(bob, "Project", "PROJ-0001"); err != nil {
		t.Fatalf("bob: %v", err)
	}
	if waited := time.Since(start); waited > 200*time.Millisecond {
		t.Errorf("bob waited %v for alice's request; the bucket is shared", waited)
	}
}
