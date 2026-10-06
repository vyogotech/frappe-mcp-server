package auth

import (
	"context"
	"frappe-mcp-server/internal/types"
)

type contextKey string

const userContextKey contextKey = "user"

const confirmationContextKey contextKey = "confirmation"

func WithUser(ctx context.Context, user *types.User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

func UserFromContext(ctx context.Context) *types.User {
	if user, ok := ctx.Value(userContextKey).(*types.User); ok {
		return user
	}
	return nil
}

// WithConfirmation carries the one-time write token from the X-Frappe-Confirmation header. It rides the context, not a
// tool argument, so it is in no envelope, no saved history and no audit line the model can read.
func WithConfirmation(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, confirmationContextKey, token)
}

func ConfirmationFromContext(ctx context.Context) string {
	token, _ := ctx.Value(confirmationContextKey).(string)
	return token
}
