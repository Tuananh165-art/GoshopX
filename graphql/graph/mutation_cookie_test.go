package graph

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSetSessionCookieUsesHostOnlySecureCookieBehindProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	writer := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(writer)
	ctx.Request = httptest.NewRequest(http.MethodPost, "http://graphql/graphql", nil)
	ctx.Request.Header.Set("X-Forwarded-Proto", "https")

	setSessionCookie(ctx, "session-token", 3600)

	cookies := writer.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected one session cookie, got %d", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Domain != "" {
		t.Fatalf("expected host-only cookie, got domain %q", cookie.Domain)
	}
	if !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("expected Secure, HttpOnly, SameSite=Lax cookie: %#v", cookie)
	}
}
