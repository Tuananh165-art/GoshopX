package auth

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/Tuananh165art/GoshopX/pkg/contextkeys"
)

func GetUserId(ctx context.Context, abort bool) string {
	userId, err := GetUserIdInt(ctx, abort)
	if err != nil {
		return ""
	}
	return strconv.Itoa(userId)
}

func GetUserIdInt(ctx context.Context, abort bool) (int, error) {
	accountId, ok := ctx.Value(contextkeys.UserIDKey).(uint64)
	if !ok {
		// GraphQL resolvers do not own the Gin response lifecycle. Returning an
		// error here lets the GraphQL transport produce a structured error
		// instead of panicking while trying to cast UserIDKey to *gin.Context.
		return 0, errors.New("UserId not found in context")
	}
	return int(accountId), nil
}

func GetRole(ctx context.Context) string {
	role, _ := ctx.Value(contextkeys.RoleKey).(string)
	return role
}

func HasAnyRole(ctx context.Context, roles ...string) bool {
	current := GetRole(ctx)
	for _, role := range roles {
		if current == role || (current == "admin" && strings.HasSuffix(role, "_admin")) {
			return true
		}
	}
	return false
}
