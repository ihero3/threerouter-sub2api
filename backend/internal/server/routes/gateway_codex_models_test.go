package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGatewayRoutesCodexModelsManifestPathIsRegistered(t *testing.T) {
	router := newGatewayRoutesTestRouter()

	registered := make(map[string]string)
	for _, route := range router.Routes() {
		if route.Method == http.MethodGet {
			registered[route.Path] = route.Handler
		}
	}

	require.NotEmpty(t, registered["/backend-api/codex/models"], "GET /backend-api/codex/models should be registered")
	require.NotEmpty(t, registered["/v1/models"], "GET /v1/models should be registered")
	require.NotEmpty(t, registered["/models"], "GET /models should be registered")
	// /v1/models 按版本信号内容协商（目录形状 ↔ OpenAI 列表），
	// /backend-api/codex/models 是 Codex 专用路由、固定按目录形状响应，
	// 两者必须是两条独立注册，否则其中一种客户端会拿到错误形状。
	require.NotEqual(t, registered["/v1/models"], registered["/backend-api/codex/models"],
		"the Codex-only route must not share the content-negotiating /v1/models registration")
}

func TestDispatchCodexModelsGatewayKeepsOnlyOpenAIOnLiveManifestHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		platform   string
		wantOpenAI bool
	}{
		{platform: service.PlatformOpenAI, wantOpenAI: true},
		{platform: service.PlatformComposite},
		{platform: service.PlatformGrok},
		{platform: service.PlatformDeepseek},
	}

	for _, tt := range tests {
		t.Run(tt.platform, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodGet, "/models?client_version=0.147.0", nil)
			c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{
				Group: &service.Group{Platform: tt.platform},
			})
			called := ""

			dispatchCodexModelsGateway(c,
				func(c *gin.Context) { called = "openai" },
				func(c *gin.Context) { called = "generated" },
			)

			if tt.wantOpenAI {
				require.Equal(t, "openai", called)
			} else {
				require.Equal(t, "generated", called)
			}
		})
	}
}

// 目录形状只由「客户端是否自报版本」决定，与分组平台、版本号取值都无关。
// 无信号时保持 OpenAI 兼容的 {"object":"list","data":[…]}，供非 Codex 客户端使用。
func TestGatewayRoutesModelsShapeDependsOnClientVersionSignal(t *testing.T) {
	catalogShape := func(t *testing.T, router *gin.Engine, path string, headers map[string]string) string {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		for name, value := range headers {
			req.Header.Set(name, value)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code, "path=%s headers=%v body=%s", path, headers, w.Body.String())

		var envelope map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
		switch {
		case envelope["models"] != nil:
			return "models"
		case envelope["data"] != nil:
			return "data"
		default:
			t.Fatalf("unknown models envelope keys: %v", envelope)
			return ""
		}
	}

	cases := []struct {
		name    string
		path    string
		headers map[string]string
		want    string
	}{
		{name: "client_version query", path: "/v1/models?client_version=0.162.0", want: "models"},
		{name: "garbage client_version still uses catalog", path: "/v1/models?client_version=abc", want: "models"},
		{name: "duplicate client_version takes first non-empty", path: "/v1/models?client_version=&client_version=0.162.0", want: "models"},
		{name: "Version header", path: "/v1/models", headers: map[string]string{"Version": "0.162.0"}, want: "models"},
		{name: "X-Client-Version header", path: "/v1/models", headers: map[string]string{"X-Client-Version": "0.162.0"}, want: "models"},
		{name: "X-Codex-Version header", path: "/v1/models", headers: map[string]string{"X-Codex-Version": "0.162.0"}, want: "models"},
		{name: "repeated client_version still uses catalog", path: "/v1/models?client_version=0.162.0&client_version=0.162.0", want: "models"},
	}

	for _, platform := range []string{
		service.PlatformDeepseek,
		service.PlatformKimi,
		service.PlatformZhipu,
		service.PlatformGrok,
		service.PlatformComposite,
	} {
		for _, tc := range cases {
			t.Run(platform+"/"+tc.name, func(t *testing.T) {
				require.Equal(t, tc.want, catalogShape(t, newGatewayRoutesTestRouter(platform), tc.path, tc.headers))
			})
		}
	}
}

// 目录端点在任何分组下都不得 404/5xx：Codex 对非 2xx 目录没有任何容错，
// 会静默丢弃整份自定义目录并回退内置模型列表。
func TestGatewayRoutesModelsNeverFailsClosedForAnyPlatform(t *testing.T) {
	// OpenAI 分组不在这一层测：测试路由没有注入 gatewayService，上游分支无法
	// 在真实路由上构建目录，其 fail-open 语义由 handler 层单测覆盖。
	for _, platform := range []string{
		service.PlatformComposite,
		service.PlatformKimi,
		service.PlatformZhipu,
		service.PlatformDeepseek,
		service.PlatformGrok,
		service.PlatformAnthropic,
	} {
		t.Run(platform, func(t *testing.T) {
			router := newGatewayRoutesTestRouter(platform)
			// 不带版本信号的 /v1/models 走的是 GatewayHandler，测试路由没有
			// 注入 gatewayService，因此不在这一层断言；它属于非 Codex 客户端的
			// OpenAI 兼容列表，与目录端点无关。
			for _, path := range []string{
				"/v1/models?client_version=0.162.0",
				"/backend-api/codex/models?client_version=0.162.0",
				"/backend-api/codex/models",
			} {
				req := httptest.NewRequest(http.MethodGet, path, nil)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				require.NotContains(t, []int{http.StatusNotFound, http.StatusInternalServerError}, w.Code,
					"platform=%s path=%s must not fail closed; body=%s", platform, path, w.Body.String())
			}
		})
	}
}

func TestDispatchCodexModelsGatewayFallsBackToLocalCatalogWhenOpenAIHandlerDefers(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		platform   string
		deferTo    bool
		wantOpenAI bool
		wantLocal  bool
	}{
		{name: "openai group served by upstream handler", platform: service.PlatformOpenAI, wantOpenAI: true},
		{name: "openai group defers when upstream catalog is unavailable", platform: service.PlatformOpenAI, deferTo: true, wantOpenAI: true, wantLocal: true},
		{name: "deepseek group always uses the local catalog", platform: service.PlatformDeepseek, wantLocal: true},
		{name: "grok group always uses the local catalog", platform: service.PlatformGrok, wantLocal: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodGet, "/v1/models?client_version=0.162.0", nil)
			c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{
				Group: &service.Group{Platform: tt.platform},
			})

			openAICalled, localCalled := false, false
			dispatchCodexModelsGateway(c,
				func(c *gin.Context) {
					openAICalled = true
					if tt.deferTo {
						c.Set(service.CodexModelsLocalFallbackContextKey, true)
					}
				},
				func(*gin.Context) { localCalled = true },
			)

			require.Equal(t, tt.wantOpenAI, openAICalled, "OpenAI handler call")
			require.Equal(t, tt.wantLocal, localCalled, "local catalog handler call")
		})
	}
}

// OpenAI 分支已经交付目录时不得被本地目录覆盖（尤其是 304 与既有 body）。
func TestDispatchCodexModelsGatewayKeepsDeliveredOpenAIResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models?client_version=0.162.0", nil)
	c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{
		Group: &service.Group{Platform: service.PlatformOpenAI},
	})

	localCalled := false
	dispatchCodexModelsGateway(c,
		func(c *gin.Context) {
			c.Set(service.CodexModelsLocalFallbackContextKey, true)
			c.Data(http.StatusOK, "application/json", []byte(`{"models":[]}`))
		},
		func(*gin.Context) { localCalled = true },
	)

	require.False(t, localCalled, "a delivered catalog must not be overwritten by the local catalog")
	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"models":[]}`, rec.Body.String())
}
