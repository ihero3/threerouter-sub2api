package service

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/tidwall/gjson"
)

// MaxRequestPreviewRunes caps the user-input head kept on usage_logs.request_preview.
// 200 runes is enough to identify the request topic without preserving anything
// resembling the full prompt. Must stay below the column width (VARCHAR(255),
// see migrations 242/243/244).
const MaxRequestPreviewRunes = 200

var (
	requestPreviewBearerPattern = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+\-/]{12,}=*`)
	requestPreviewAPIKeyPattern = regexp.MustCompile(`(?i)\b(sk|rk|pk|api[_-]?key|token|secret|password)[-_:=\s]+[A-Za-z0-9._~+\-/]{8,}`)
	requestPreviewEmailPattern  = regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`)

	// Client-injected metadata blocks (Claude Code's <system-reminder>, slash-command
	// echoes, …) that would otherwise crowd out the user's actual text.
	requestPreviewNoisePatterns = buildRequestPreviewNoisePatterns([]string{
		"system-reminder",
		"command-name",
		"command-message",
		"command-args",
		"local-command-stdout",
		"local-command-caveat",
		"user-memory",
	})
	requestPreviewBlankLinesPattern = regexp.MustCompile(`\n{2,}`)
	// Opening tag of a client envelope such as <session …> or <user_query>.
	requestPreviewLeadingTagPattern = regexp.MustCompile(`^<([a-zA-Z][a-zA-Z0-9_:-]*)[^>]*>`)
)

// maxRequestPreviewUnwrapDepth bounds how many nested envelopes we peel off so a
// pathological input cannot spin the unwrap loop.
const maxRequestPreviewUnwrapDepth = 5

func buildRequestPreviewNoisePatterns(tags []string) []*regexp.Regexp {
	patterns := make([]*regexp.Regexp, 0, len(tags))
	for _, tag := range tags {
		patterns = append(patterns, regexp.MustCompile(`(?is)<`+tag+`\b[^>]*>.*?</`+tag+`\s*>`))
	}
	return patterns
}

// stripRequestPreviewNoise removes client-injected metadata blocks and the blank
// lines they leave behind so the preview surfaces the user's real input.
func stripRequestPreviewNoise(text string) string {
	if text == "" {
		return text
	}
	for _, re := range requestPreviewNoisePatterns {
		text = re.ReplaceAllString(text, "")
	}
	text = requestPreviewBlankLinesPattern.ReplaceAllString(text, "\n")
	return strings.TrimSpace(text)
}

// unwrapRequestPreviewWrappers peels client envelopes whose inner text is the
// user's real input (e.g. <session>hi</session>, <user_query>hi</user_query>).
// Unlike the noise blocks these are kept, not dropped. Only a wrapper spanning
// the entire text is peeled, and we repeat to handle nested envelopes.
func unwrapRequestPreviewWrappers(text string) string {
	text = strings.TrimSpace(text)
	for depth := 0; depth < maxRequestPreviewUnwrapDepth; depth++ {
		m := requestPreviewLeadingTagPattern.FindStringSubmatch(text)
		if m == nil {
			return text
		}
		closeTag := "</" + m[1] + ">"
		if !strings.HasSuffix(strings.ToLower(text), strings.ToLower(closeTag)) {
			return text
		}
		text = strings.TrimSpace(text[len(m[0]) : len(text)-len(closeTag)])
	}
	return text
}

// ExtractRequestPreview returns a short, sanitised head of the latest user
// message from the raw JSON request body. Returns "" when no user text can be
// located (e.g. tool-only call, malformed JSON).
//
// 使用 gjson 按路径抽取而不是整包 Unmarshal：大体积请求（长上下文、多模态）
// 无需构建全量 map[string]any，热路径上的解析开销与 GC 压力显著降低。
func ExtractRequestPreview(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	root := gjson.ParseBytes(body)
	if !root.Exists() || !root.IsObject() {
		return ""
	}
	text := extractLatestUserText(root)
	if text == "" {
		return ""
	}
	// Drop client-injected scaffolding first so the preview budget is spent on
	// the user's real input rather than on metadata tags.
	text = stripRequestPreviewNoise(text)
	if text == "" {
		return ""
	}
	// Then peel envelopes that wrap the user's actual input (e.g. <user_query>…).
	text = unwrapRequestPreviewWrappers(text)
	if text == "" {
		return ""
	}
	text = sanitizeRequestPreview(text)
	runes := []rune(text)
	if len(runes) > MaxRequestPreviewRunes {
		text = string(runes[:MaxRequestPreviewRunes])
	}
	return strings.TrimSpace(text)
}

// extractLatestUserText walks the common request shapes (chat, responses,
// anthropic messages, gemini contents, media prompts) and returns the text of
// the most recent user turn, or "" if none.
func extractLatestUserText(root gjson.Result) string {
	if text := extractUserFromMessages(root.Get("messages")); text != "" {
		return text
	}
	if text := extractUserFromResponsesInput(root.Get("input")); text != "" {
		return text
	}
	if text := extractUserFromMediaPrompt(root); text != "" {
		return text
	}
	return ""
}

func extractUserFromMessages(value gjson.Result) string {
	if !value.IsArray() {
		return ""
	}
	items := value.Array()
	for i := len(items) - 1; i >= 0; i-- {
		msg := items[i]
		if !msg.IsObject() {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(msg.Get("role").String()))
		if role != "user" {
			continue
		}
		texts := collectContentTexts(msg.Get("content"))
		if len(texts) == 0 {
			continue
		}
		return strings.Join(texts, "\n")
	}
	return ""
}

func extractUserFromResponsesInput(value gjson.Result) string {
	switch {
	case value.Type == gjson.String:
		return strings.TrimSpace(value.String())
	case value.IsArray():
		items := value.Array()
		for i := len(items) - 1; i >= 0; i-- {
			entry := items[i]
			if entry.Type == gjson.String {
				if t := strings.TrimSpace(entry.String()); t != "" {
					return t
				}
				continue
			}
			if !entry.IsObject() {
				continue
			}
			role := strings.ToLower(strings.TrimSpace(entry.Get("role").String()))
			if role != "" && role != "user" {
				continue
			}
			if content := entry.Get("content"); content.Exists() {
				if texts := collectContentTexts(content); len(texts) > 0 {
					return strings.Join(texts, "\n")
				}
			}
			if text := strings.TrimSpace(entry.Get("text").String()); text != "" {
				return text
			}
		}
	}
	return ""
}

func extractUserFromMediaPrompt(root gjson.Result) string {
	// Walk common media-prompt keys; only fires when nothing else matched.
	keys := []string{"prompt", "input", "description", "query", "input_prompt", "text_prompt"}
	for _, k := range keys {
		if v := root.Get(k); v.Exists() && v.Type == gjson.String {
			if t := strings.TrimSpace(v.String()); t != "" {
				return t
			}
		}
	}
	return ""
}

// collectContentTexts handles both string content and structured content arrays
// (text blocks inside multimodal requests).
func collectContentTexts(value gjson.Result) []string {
	switch {
	case value.Type == gjson.String:
		if t := strings.TrimSpace(value.String()); t != "" {
			return []string{t}
		}
	case value.IsArray():
		var out []string
		for _, part := range value.Array() {
			if !part.IsObject() {
				continue
			}
			typeName := strings.ToLower(strings.TrimSpace(part.Get("type").String()))
			if typeName != "" && typeName != "text" && typeName != "input_text" && typeName != "output_text" {
				continue
			}
			if text := strings.TrimSpace(part.Get("text").String()); text != "" {
				out = append(out, text)
			}
		}
		return out
	case value.IsObject():
		if text := strings.TrimSpace(value.Get("text").String()); text != "" {
			return []string{text}
		}
	}
	return nil
}

func sanitizeRequestPreview(text string) string {
	text = requestPreviewBearerPattern.ReplaceAllString(text, "Bearer ***")
	text = requestPreviewAPIKeyPattern.ReplaceAllStringFunc(text, func(match string) string {
		if idx := strings.IndexAny(match, ":= \t"); idx >= 0 {
			return match[:idx+1] + "***"
		}
		return "***"
	})
	text = requestPreviewEmailPattern.ReplaceAllString(text, "***@***")
	return text
}

// RequestPreviewRuneCount returns the number of runes (Unicode code points) in s.
// Exposed for tests.
func RequestPreviewRuneCount(s string) int {
	return utf8.RuneCountInString(s)
}
