package auth

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

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
		if abort {
			ginContext, _ := ctx.Value(contextkeys.UserIDKey).(*gin.Context)
			ginContext.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Unauthorized",
			})
		}
		return 0, errors.New("UserId not found in context")
	}
	return int(accountId), nil
}

