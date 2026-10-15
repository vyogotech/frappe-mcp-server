package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"frappe-mcp-server/internal/auth"
	"frappe-mcp-server/internal/config"
	"frappe-mcp-server/internal/types"
)

// RefusalNotConfirmed is what a caller gets when a write carried no confirmation the user gave. The prompt has the
// model repeat a tool error to the user, so this sentence is read by a person.
const RefusalNotConfirmed = "This change was not confirmed by the user, so it was not made."

//nolint:staticcheck // ST1005: a sentence a person reads, settled by ADR-006, not a Go error string
var errNotConfirmed = errors.New(RefusalNotConfirmed)

// requireConfirmation is the gate before every write. It spends the one-time token from the X-Frappe-Confirmation
// header against Frappe, as the user; the sidebar's agent gets one only when the user allows the change. Anything but
// a redeemed token — none sent, one Frappe refuses, a method that is absent or unreachable — refuses the write. The
// model's own arguments cannot satisfy it.
//
// A client signed in with OAuth or an API key alone (Claude Desktop, Claude Code) asks its person before each call and carries no
// token: the site decides its writes instead (authorizeDirect). A browser session, which is how the sidebar's agent
// signs in, never takes that path. The site may also answer "propose" (Vyogo Forge AI): the write, `change`, is then
// filed as a proposed release, and the returned sentence says where it waits; the caller makes no write.
func (t *ToolRegistry) requireConfirmation(ctx context.Context, tool, doctype, name string, change any) (string, error) {
	user := ""
	u := auth.UserFromContext(ctx)
	if u != nil {
		user = u.Email
	}

	method := t.ConfirmationRedeemMethod
	if method == "" {
		method = config.DefaultConfirmationRedeemMethod
	}

	token := auth.ConfirmationFromContext(ctx)
	if token == "" {
		if u != nil && u.SessionID == "" && (u.Token != "" || u.APIKey != "") {
			return t.authorizeDirect(ctx, u, tool, doctype, name, change)
		}
		slog.Warn("write refused: no confirmation", "user", user, "tool", tool, "doctype", doctype, "name", name,
			"reason", "no token")
		return "", errNotConfirmed
	}
	if err := t.frappeClient.RedeemConfirmation(ctx, method, token, tool, doctype, name); err != nil {
		// the reason carries Frappe's own line, so redact the token in case a future redeem method echoes it back
		slog.Warn("write refused: no confirmation", "user", user, "tool", tool, "doctype", doctype, "name", name,
			"reason", strings.ReplaceAll(err.Error(), token, "<token>"))
		return "", errNotConfirmed
	}
	slog.Info("confirmation redeemed", "user", user, "tool", tool, "doctype", doctype, "name", name)
	return "", nil
}

// authorizeDirect asks the site whether an OAuth or API-key client's write may go ahead without a token. A refusal
// carries the site's reason, which the client shows its person; a site that cannot answer refuses.
func (t *ToolRegistry) authorizeDirect(ctx context.Context, u *types.User, tool, doctype, name string, change any) (string, error) {
	method := t.DirectWriteMethod
	if method == "" {
		method = config.DefaultDirectWriteMethod
	}
	answer, err := t.frappeClient.AuthorizeDirectWrite(ctx, method, tool, doctype, name)
	if err != nil {
		slog.Warn("direct write refused", "user", u.Email, "client", u.ClientID, "tool", tool, "doctype", doctype,
			"name", name, "reason", err.Error())
		return "", errNotConfirmed
	}
	if answer.Propose {
		return t.propose(ctx, u, tool, doctype, name, change)
	}
	if !answer.OK {
		slog.Info("direct write refused", "user", u.Email, "client", u.ClientID, "tool", tool, "doctype", doctype,
			"name", name, "reason", answer.Reason)
		if answer.Reason == "" {
			return "", errNotConfirmed
		}
		//nolint:staticcheck // ST1005: a sentence a person reads, as errNotConfirmed is
		return "", errors.New("This change was not made: " + answer.Reason)
	}
	slog.Info("direct write allowed", "user", u.Email, "client", u.ClientID, "tool", tool, "doctype", doctype, "name", name)
	return "", nil
}

// propose files the write as a proposed release and says where it waits for approval.
func (t *ToolRegistry) propose(ctx context.Context, u *types.User, tool, doctype, name string, change any) (string, error) {
	method := t.ProposeMethod
	if method == "" {
		method = config.DefaultProposeMethod
	}
	proposal, err := t.frappeClient.ProposeChange(ctx, method, tool, doctype, name, change)
	if err != nil {
		slog.Warn("proposal not filed", "user", u.Email, "tool", tool, "doctype", doctype, "name", name, "reason", err.Error())
		//nolint:staticcheck // ST1005: a sentence a person reads, as errNotConfirmed is
		return "", fmt.Errorf("This change was not made, and could not be proposed: %w", err)
	}
	slog.Info("write proposed", "user", u.Email, "tool", tool, "doctype", doctype, "name", name, "proposal", proposal.Name)
	return fmt.Sprintf("Proposed as %s for a System Manager to approve; nothing on the site changes until then. "+
		"Approve or reject it at %s.", proposal.Name, proposal.ApproveURL), nil
}
