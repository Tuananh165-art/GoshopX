package middleware

import (
	"context"

	"github.com/Tuananh165art/GoshopX/pkg/auth"
	"github.com/Tuananh165art/GoshopX/pkg/contextkeys"
	"github.com/gin-gonic/gin"
)

func AuthorizeJWT() gin.HandlerFunc {
	return func(c *gin.Context) {
		authCookie, err := c.Cookie("token")
		if err != nil || authCookie == "" {
			c.Set("userID", "")
			c.Next()
			return
		}

		token, err := auth.ValidateToken(authCookie)
		if err != nil {
			c.Set("userID", "")
			c.Next()
			return
		}

		if claims, ok := token.Claims.(*auth.JWTCustomClaims); ok && token.Valid {
			c.Set("userID", claims.UserID)
			ctxWithVal := context.WithValue(c.Request.Context(), contextkeys.UserIDKey, claims.UserID)

			c.Request = c.Request.WithContext(ctxWithVal)
		} else {
			c.Set("userID", "")
		}

		c.Next()
	}
}

