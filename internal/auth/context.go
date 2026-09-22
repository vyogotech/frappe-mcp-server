package auth

import (
	"context"
	"frappe-mcp-server/internal/types"
)

type contextKey string

const userContextKey contextKey = "user"

func WithUser(ctx context.Context, user *types.User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

func UserFromContext(ctx context.Context) *types.User {
	if user, ok := ctx.Value(userContextKey).(*types.User); ok {
		return user
	}
	return nil
}

func GetUserFromContext(ctx context.Context) (*types.User, bool) {
	user := UserFromContext(ctx)
	return user, user != nil
}
