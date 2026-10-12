package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// dispatchCodexModelsGateway 在 OpenAI 与本地生成目录两个实现之间选择。
//
// OpenAI 分组走 live 上游目录以保留上游元数据（能力、上下文窗口、推理档位），
// 其余分组（composite / kimi / zhipu / deepseek / grok / anthropic …）统一落到
// 本地生成目录。两条路径都返回 {"models":[…]}，因此分组的差异不会变成
// 「目录失效」。
//
// Codex 对目录端点没有容错：任何非 200、无法解析的 body 或缺失字段都会让整份
// 目录在客户端静默失效，回退内置模型列表且界面不报错。OpenAI 分支因此可以放弃
// 本次响应（置 service.CodexModelsLocalFallbackContextKey），由本地生成目录兜底；
// 本地生成目录只依赖平台与分组配置，不依赖上游可用性。
func dispatchCodexModelsGateway(c *gin.Context, openAIHandler, generatedHandler gin.HandlerFunc) {
	if getGroupPlatform(c) == service.PlatformOpenAI {
		openAIHandler(c)
		if !codexModelsLocalFallbackRequested(c) {
			return
		}
		// 响应已经写出（含 304 与已交付的目录）时不再改写。
		if c.Writer != nil && c.Writer.Written() {
			return
		}
	}
	generatedHandler(c)
}

func codexModelsLocalFallbackRequested(c *gin.Context) bool {
	if c == nil {
		return false
	}
	requested, exists := c.Get(service.CodexModelsLocalFallbackContextKey)
	if !exists {
		return false
	}
	flag, ok := requested.(bool)
	return ok && flag
}
