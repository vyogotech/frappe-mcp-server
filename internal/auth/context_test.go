package auth

import (
	"context"
	"testing"

	"frappe-mcp-server/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestWithUser(t *testing.T) {
	ctx := context.Background()
	user := &types.User{
		ID:       "user123",
		Email:    "test@example.com",
		FullName: "Test User",
		Roles:    []string{"User", "Admin"},
	}

	newCtx := WithUser(ctx, user)
	assert.NotNil(t, newCtx)

	retrievedUser := UserFromContext(newCtx)
	assert.NotNil(t, retrievedUser)
	assert.Equal(t, user.ID, retrievedUser.ID)
	assert.Equal(t, user.Email, retrievedUser.Email)
	assert.Equal(t, user.FullName, retrievedUser.FullName)
	assert.Equal(t, user.Roles, retrievedUser.Roles)
}

func TestUserFromContext_NoUser(t *testing.T) {
	ctx := context.Background()
	user := UserFromContext(ctx)
	assert.Nil(t, user)
}

func TestGetUserFromContext(t *testing.T) {
	t.Run("with user", func(t *testing.T) {
		ctx := context.Background()
		user := &types.User{
			ID:    "user123",
			Email: "test@example.com",
		}

		newCtx := WithUser(ctx, user)
		retrievedUser, found := GetUserFromContext(newCtx)

		assert.True(t, found)
		assert.NotNil(t, retrievedUser)
		assert.Equal(t, user.ID, retrievedUser.ID)
	})

	t.Run("without user", func(t *testing.T) {
		ctx := context.Background()
		retrievedUser, found := GetUserFromContext(ctx)

		assert.False(t, found)
		assert.Nil(t, retrievedUser)
	})
}

func TestUserMethods(t *testing.T) {
	user := &types.User{
		ID:       "user123",
		Email:    "test@example.com",
		FullName: "Test User",
		Roles:    []string{"User", "Admin"},
		Metadata: map[string]interface{}{
			"department": "Engineering",
			"location":   "San Francisco",
		},
	}

	t.Run("GetID", func(t *testing.T) {
		assert.Equal(t, "user123", user.GetID())
	})

	t.Run("GetUserName", func(t *testing.T) {
		assert.Equal(t, "test@example.com", user.GetUserName())
	})

	t.Run("GetGroups", func(t *testing.T) {
		groups := user.GetGroups()
		assert.Equal(t, []string{"User", "Admin"}, groups)
	})

	t.Run("GetExtensions", func(t *testing.T) {
		extensions := user.GetExtensions()
		assert.Equal(t, []string{"Engineering"}, extensions["department"])
		assert.Equal(t, []string{"San Francisco"}, extensions["location"])
	})
}






