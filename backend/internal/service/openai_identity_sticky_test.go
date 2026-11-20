package service

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func identityStickyTestContext(apiKeyID, userID int64) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx := context.Background()
	if apiKeyID > 0 {
		ctx = context.WithValue(ctx, ctxkey.APIKeyID, apiKeyID)
	}
	if userID > 0 {
		ctx = context.WithValue(ctx, ctxkey.UserID, userID)
	}
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil).WithContext(ctx)
	return c
}

func identityStickyService(enabled bool) *OpenAIGatewayService {
	return &OpenAIGatewayService{cfg: &config.Config{
		Gateway: config.GatewayConfig{
			OpenAIScheduler: config.GatewayOpenAISchedulerConfig{
				IdentityStickyEnabled: enabled,
			},
		},
	}}
}

// Two independent requests from the same API key (different first user message,
// no explicit session anchor) must still derive the same sticky hash when the
// switch is on. This is the exact scenario that caused one key to be scattered
// across several upstream accounts within seconds.
func TestIdentitySticky_SameKeyStableAcrossIndependentRequests(t *testing.T) {
	svc := identityStickyService(true)
	bodyA := []byte(`{"model":"deepseek-v4-flash","messages":[{"role":"user","content":"question one"}]}`)
	bodyB := []byte(`{"model":"deepseek-v4-flash","messages":[{"role":"user","content":"totally different question"}]}`)

	hA := svc.GenerateSessionHash(identityStickyTestContext(101, 10), bodyA)
	hB := svc.GenerateSessionHash(identityStickyTestContext(101, 10), bodyB)

	require.NotEmpty(t, hA)
	require.Equal(t, hA, hB, "same key must stick to one account regardless of content")
}

func TestIdentitySticky_DifferentKeysDiffer(t *testing.T) {
	svc := identityStickyService(true)
	body := []byte(`{"model":"deepseek-v4-flash","messages":[{"role":"user","content":"same question"}]}`)

	h1 := svc.GenerateSessionHash(identityStickyTestContext(101, 10), body)
	h2 := svc.GenerateSessionHash(identityStickyTestContext(102, 10), body)

	require.NotEqual(t, h1, h2, "different keys under one user must not collide")
}

func TestIdentitySticky_ExplicitAnchorWins(t *testing.T) {
	svc := identityStickyService(true)
	body := []byte(`{"model":"m","messages":[{"role":"user","content":"question"}]}`)

	c := identityStickyTestContext(101, 10)
	c.Request.Header.Set("session-id", "client-conversation-xyz")
	withHeader := svc.GenerateSessionHash(c, body)
	withoutHeader := svc.GenerateSessionHash(identityStickyTestContext(101, 10), body)

	require.NotEqual(t, withHeader, withoutHeader, "explicit session header must take priority over identity")
}

func TestIdentitySticky_SwitchOffKeepsContentFallback(t *testing.T) {
	svc := identityStickyService(false)
	bodyA := []byte(`{"model":"m","messages":[{"role":"user","content":"question one"}]}`)
	bodyB := []byte(`{"model":"m","messages":[{"role":"user","content":"question two"}]}`)

	hA := svc.GenerateSessionHash(identityStickyTestContext(101, 10), bodyA)
	hB := svc.GenerateSessionHash(identityStickyTestContext(101, 10), bodyB)

	require.NotEqual(t, hA, hB, "with switch off, content-based fallback churn must be preserved")
}

func TestIdentitySticky_NoIdentityFallsBackToContent(t *testing.T) {
	svc := identityStickyService(true)
	bodyA := []byte(`{"model":"m","messages":[{"role":"user","content":"question one"}]}`)
	bodyB := []byte(`{"model":"m","messages":[{"role":"user","content":"question two"}]}`)

	hA := svc.GenerateSessionHash(identityStickyTestContext(0, 0), bodyA)
	hB := svc.GenerateSessionHash(identityStickyTestContext(0, 0), bodyB)

	require.NotEqual(t, hA, hB, "without authenticated identity, content fallback must apply")
}

func TestIdentitySticky_NilConfigDefaultsOff(t *testing.T) {
	svc := &OpenAIGatewayService{}
	body := []byte(`{"model":"m","messages":[{"role":"user","content":"question one"}]}`)

	require.Empty(t, svc.openAIIdentityStickySeed(identityStickyTestContext(101, 10)))
	require.NotEmpty(t, svc.GenerateSessionHash(identityStickyTestContext(101, 10), body), "content fallback still runs")
}
