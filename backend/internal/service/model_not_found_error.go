package service

import (
	"net/http"
	"strings"
)

var upstreamModelNotFoundKeywords = []string{
	"model not found",
	"unknown model",
	"not found",
	// sub2api/one-api 类中转站：分组内没有任何账号支持该模型。
	// 例：Model "deepseek-v4-flash" is not supported by any configured account in this group
	"not supported by any configured account",
	"not supported by any account",
	"no available account",
	// 上游直接以 "Model X does not exist" 形式返回（部分聚合/中转上游）。
	"does not exist",
	// 中文上游（或中文代理）的模型不存在/暂不可用，典型于本仓库截图里的
	// "模型 deepseek-v3-flash 不存在或暂时不可用"。仅在正文含「模型/model」时才命中，
	// 避免把「账号不存在」「服务不可用」误判为模型级错误。
	"不存在",
	"暂时不可用",
}

func isUpstreamModelNotFoundError(statusCode int, body []byte) bool {
	// 模型不存在/不可用是「账号能力」问题而非账号失效，典型返回 404；
	// 但部分上游（含中文代理）以 403 返回同样的「模型 X 不存在」，若只认 404 会把健康账号
	// 当成故障永久停用（见 #66 截图：HTTP 403 模型 deepseek-v3-flash 不存在）。故同时放行 403。
	if statusCode != http.StatusNotFound && statusCode != http.StatusForbidden {
		return false
	}
	normalized := normalizeModelNotFoundBody(body)
	if normalized == "" || (!strings.Contains(normalized, "model") && !strings.Contains(normalized, "模型")) {
		return false
	}
	return containsModelNotFoundKeyword(normalized)
}

func isModelNotFoundError(statusCode int, body []byte) bool {
	return isUpstreamModelNotFoundError(statusCode, body) || statusCode == http.StatusNotFound
}

// openAICodexPlanGatedModelPhrase matches the deterministic Codex 400 returned
// when a ChatGPT OAuth account's plan cannot serve the requested model, e.g.
// {"detail":"The 'gpt-5.6-sol' model is not supported when using Codex with a ChatGPT account."}
// The phrase is compared against the normalized body (lowercased, "_"/"-"
// folded to spaces), so it also matches the same message embedded in
// error.message-style payloads.
const openAICodexPlanGatedModelPhrase = "model is not supported when using codex"

// isOpenAICodexPlanGatedModelError reports whether the upstream response is the
// deterministic Codex rejection of a plan-gated model on a ChatGPT account.
// Unlike transient failures, retrying the same account cannot succeed until the
// account's plan changes, so callers should treat it like model-not-found and
// cool the (account, model) pair down instead of re-selecting the account.
func isOpenAICodexPlanGatedModelError(statusCode int, body []byte) bool {
	if statusCode != http.StatusBadRequest {
		return false
	}
	normalized := normalizeModelNotFoundBody(body)
	if normalized == "" {
		return false
	}
	return strings.Contains(normalized, openAICodexPlanGatedModelPhrase)
}

func containsModelNotFoundKeyword(normalizedBody string) bool {
	if normalizedBody == "" {
		return false
	}
	for _, keyword := range upstreamModelNotFoundKeywords {
		if strings.Contains(normalizedBody, keyword) {
			return true
		}
	}
	return false
}

func normalizeModelNotFoundBody(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	normalized := strings.ToLower(string(body))
	normalized = strings.NewReplacer("_", " ", "-", " ", "\n", " ", "\r", " ", "\t", " ").Replace(normalized)
	return strings.Join(strings.Fields(normalized), " ")
}
