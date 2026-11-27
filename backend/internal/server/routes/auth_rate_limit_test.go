package routes

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/middleware"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

var errAuthRouteRedisUnavailable = errors.New("redis unavailable")

type authRouteSettingRepo struct{}

func (*authRouteSettingRepo) Get(context.Context, string) (*service.Setting, error) {
	return nil, service.ErrSettingNotFound
}

func (*authRouteSettingRepo) GetValue(context.Context, string) (string, error) {
	return "", service.ErrSettingNotFound
}

func (*authRouteSettingRepo) Set(context.Context, string, string) error { return nil }

func (*authRouteSettingRepo) GetMultiple(context.Context, []string) (map[string]string, error) {
	return map[string]string{}, nil
}

func (*authRouteSettingRepo) SetMultiple(context.Context, map[string]string) error { return nil }

func (*authRouteSettingRepo) GetAll(context.Context) (map[string]string, error) {
	return map[string]string{}, nil
}

func (*authRouteSettingRepo) Delete(context.Context, string) error { return nil }

func newAuthRoutesTestRouter(redisClient *redis.Client) *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	_ = router.SetTrustedProxies(nil)
	v1 := router.Group("/api/v1")
	noop := func(c *gin.Context) { c.Next() }

	RegisterAuthRoutes(
		v1,
		&handler.Handlers{
			Auth:    &handler.AuthHandler{},
			Setting: &handler.SettingHandler{},
			Admin: &handler.AdminHandlers{
				Channel: &admin.ChannelHandler{},
			},
			Passkey:                  &handler.PasskeyHandler{},
			RegistrationClickCaptcha: handler.NewRegistrationClickCaptchaHandler(service.NewRegistrationClickCaptchaService(nil, nil)),
		},
		servermiddleware.JWTAuthMiddleware(noop),
		servermiddleware.AuditLogMiddleware(noop),
		redisClient,
		nil,
		servermiddleware.NewPanelRateLimiter(redisClient, nil),
		nil,
	)

	return router
}

func TestAuthRoutesRateLimitFailCloseWhenRedisUnavailable(t *testing.T) {
	originalRateLimitRun := middleware.SetRateLimitRunForTest(func(context.Context, *redis.Client, string, int64) (int64, bool, error) {
		return 0, false, errAuthRouteRedisUnavailable
	})
	t.Cleanup(func() { middleware.SetRateLimitRunForTest(originalRateLimitRun) })

	rdb := redis.NewClient(&redis.Options{
		Addr:         "127.0.0.1:1",
		DialTimeout:  50 * time.Millisecond,
		ReadTimeout:  50 * time.Millisecond,
		WriteTimeout: 50 * time.Millisecond,
	})
	t.Cleanup(func() {
		_ = rdb.Close()
	})

	router := newAuthRoutesTestRouter(rdb)
	paths := []string{
		"/api/v1/auth/captcha/challenge",
	}

	for _, path := range paths {
		for i := 0; i < 11; i++ {
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
			req.Header.Set("Content-Type", "application/json")
			req.RemoteAddr = "203.0.113.10:12345"
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if i < 10 {
				require.NotEqual(t, http.StatusTooManyRequests, w.Code, "requests within fallback quota should pass; path=%s attempt=%d", path, i+1)
				continue
			}
			require.Equal(t, http.StatusTooManyRequests, w.Code, "request above fallback quota should be rejected; path=%s", path)
			require.Contains(t, w.Body.String(), "rate limit exceeded", "path=%s", path)
		}
	}
}
