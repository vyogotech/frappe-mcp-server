package tools

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"frappe-mcp-server/internal/auth"
	"frappe-mcp-server/internal/config"
)

// RefusalNotConfirmed is what a caller gets when a write carried no confirmation the user gave. The prompt has the
// model repeat a tool error to the user, so this sentence is read by a person.
const RefusalNotConfirmed = "This change was not confirmed by the user, so it was not made."

//nolint:staticcheck // ST1005: a sentence a person reads, settled by ADR-006, not a Go error string
var errNotConfirmed = errors.New(RefusalNotConfirmed)

// requireConfirmation spends the one-time token from the X-Frappe-Confirmation header against Frappe, as the user.
// Anything but a redeemed token — none sent, one Frappe refuses, a method that is absent or unreachable — refuses the
// write. It is the only gate: the model's own arguments cannot satisfy it.
func (t *ToolRegistry) requireConfirmation(ctx context.Context, tool, doctype, name string) error {
	user := ""
	if u := auth.UserFromContext(ctx); u != nil {
		user = u.Email
	}

	method := t.ConfirmationRedeemMethod
	if method == "" {
		method = config.DefaultConfirmationRedeemMethod
	}

	token := auth.ConfirmationFromContext(ctx)
	if token == "" {
		slog.Warn("write refused: no confirmation", "user", user, "tool", tool, "doctype", doctype, "name", name,
			"reason", "no token")
		return errNotConfirmed
	}
	if err := t.frappeClient.RedeemConfirmation(ctx, method, token, tool, doctype, name); err != nil {
		// the reason carries Frappe's own line, so redact the token in case a future redeem method echoes it back
		slog.Warn("write refused: no confirmation", "user", user, "tool", tool, "doctype", doctype, "name", name,
			"reason", strings.ReplaceAll(err.Error(), token, "<token>"))
		return errNotConfirmed
	}
	slog.Info("confirmation redeemed", "user", user, "tool", tool, "doctype", doctype, "name", name)
	return nil
}
