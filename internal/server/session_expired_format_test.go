package server

import (
	"strings"
	"testing"
)

// /api/v1/chat formats the same tool error for its own clients, so it has to give the same advice as the tools channel.
func TestChatTellsAnEndedSessionToSignInAgain(t *testing.T) {
	out := formatFrappeError(
		"failed to get document Sales Invoice/SINV-00001: Session expired. Please sign in again.",
		"how many invoices are open?")

	if !strings.Contains(out, "Session expired. Please sign in again.") {
		t.Errorf("the user is not asked to sign in again:\n%s", out)
	}
	for _, wrong := range []string{"administrator", "different account", "rephrasing"} {
		if strings.Contains(out, wrong) {
			t.Errorf("the ended session is still answered with %q:\n%s", wrong, out)
		}
	}
}
