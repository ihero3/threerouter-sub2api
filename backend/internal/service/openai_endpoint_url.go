package service

import (
	"net/url"
	"strings"
)

// openAIKnownEndpointSuffixes 是 base_url 里可能被配置成完整端点路径的已知后缀，
// 按长度降序排列（长端点优先剥离，避免 /responses 被 /responses/input_tokens 前缀截胡）。
// chat/completions 配置在 chat 请求上因「后缀完全相等」侥幸可用，但 embeddings/responses/models
// 等其它端点会拼出 /v1/chat/completions/v1/embeddings 这类双路径 URL（2026-11-26 事故根因）。
var openAIKnownEndpointSuffixes = []string{
	"/responses/input_tokens",
	"/chat/completions",
	"/completions",
	"/embeddings",
	"/responses",
	"/messages",
	"/models",
	"/images/generations",
	"/images/edits",
}

// stripOpenAIKnownEndpointSuffix 剥掉 path 尾部的已知端点段，返回剥离后的路径；
// 未命中时原样返回。仅用于 buildOpenAIEndpointURL 拼接前的归一化。
func stripOpenAIKnownEndpointSuffix(path string) string {
	for _, suffix := range openAIKnownEndpointSuffixes {
		if strings.HasSuffix(path, suffix) {
			return strings.TrimSuffix(path, suffix)
		}
	}
	return path
}

func buildOpenAIEndpointURL(base string, endpoint string) string {
	normalized := strings.TrimSpace(base)
	endpoint = "/" + strings.TrimLeft(strings.TrimSpace(endpoint), "/")
	relative := strings.TrimPrefix(endpoint, "/v1")
	parsed, err := url.Parse(normalized)
	if err != nil {
		return strings.TrimRight(normalized, "/") + endpoint
	}
	path := strings.TrimRight(parsed.Path, "/")
	if strings.HasSuffix(path, endpoint) || strings.HasSuffix(path, relative) {
		// base 已是目标端点本身：原样返回，不做剥离。
		parsed.Path = path
		parsed.RawPath = ""
		parsed.Fragment = ""
		return parsed.String()
	}
	// base 是别的端点路径（如 /v1/chat/completions）：先剥掉端点段再拼接。
	if stripped := stripOpenAIKnownEndpointSuffix(path); stripped != path {
		path = stripped
	}
	if !openAIBaseURLHasVersionSuffix(path) {
		path += endpoint
	} else {
		path += relative
	}
	parsed.Path = path
	parsed.RawPath = ""
	parsed.Fragment = ""
	return parsed.String()
}

func buildOpenAIResponsesInputTokensURL(base string) string {
	return buildOpenAIEndpointURL(base, "/v1/responses/input_tokens")
}

func openAIBaseURLHasVersionSuffix(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false
	}

	pathValue := ""
	if parsed, err := url.Parse(trimmed); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		pathValue = parsed.Path
	} else if slash := strings.Index(trimmed, "/"); slash >= 0 {
		pathValue = trimmed[slash:]
	}

	pathValue = strings.TrimRight(pathValue, "/")
	if pathValue == "" {
		return false
	}
	lastSlash := strings.LastIndex(pathValue, "/")
	segment := pathValue
	if lastSlash >= 0 {
		segment = pathValue[lastSlash+1:]
	}
	return isOpenAIAPIVersionSegment(segment)
}

func isOpenAIAPIVersionSegment(segment string) bool {
	s := strings.ToLower(strings.TrimSpace(segment))
	if len(s) < 2 || s[0] != 'v' || !isASCIIDigit(s[1]) {
		return false
	}

	i := 1
	for i < len(s) && isASCIIDigit(s[i]) {
		i++
	}
	if i == len(s) {
		return true
	}
	if s[i] == '.' {
		i++
		if i == len(s) || !isASCIIDigit(s[i]) {
			return false
		}
		for i < len(s) && isASCIIDigit(s[i]) {
			i++
		}
		return i == len(s)
	}

	suffix := s[i:]
	return strings.HasPrefix(suffix, "alpha") ||
		strings.HasPrefix(suffix, "beta") ||
		strings.HasPrefix(suffix, "preview")
}

func isASCIIDigit(b byte) bool {
	return b >= '0' && b <= '9'
}
