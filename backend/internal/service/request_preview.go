package service

import (
	"encoding/json"
	"regexp"
	"strings"
	"unicode/utf8"
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
func ExtractRequestPreview(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
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
func extractLatestUserText(root map[string]any) string {
	if root == nil {
		return ""
	}
	if text := extractUserFromMessages(root["messages"]); text != "" {
		return text
	}
	if text := extractUserFromResponsesInput(root["input"]); text != "" {
		return text
	}
	if text := extractUserFromMediaPrompt(root); text != "" {
		return text
	}
	return ""
}

func extractUserFromMessages(value any) string {
	items, ok := value.([]any)
	if !ok {
		return ""
	}
	for i := len(items) - 1; i >= 0; i-- {
		msg, ok := items[i].(map[string]any)
		if !ok {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(stringValueAny(msg["role"])))
		if role != "user" {
			continue
		}
		texts := collectContentTexts(msg["content"])
		if len(texts) == 0 {
			continue
		}
		return strings.Join(texts, "\n")
	}
	return ""
}

func extractUserFromResponsesInput(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []any:
		for i := len(typed) - 1; i >= 0; i-- {
			switch entry := typed[i].(type) {
			case string:
				if t := strings.TrimSpace(entry); t != "" {
					return t
				}
			case map[string]any:
				role := strings.ToLower(strings.TrimSpace(stringValueAny(entry["role"])))
				if role != "" && role != "user" {
					continue
				}
				if content, ok := entry["content"]; ok {
					if texts := collectContentTexts(content); len(texts) > 0 {
						return strings.Join(texts, "\n")
					}
				}
				if text := stringValueAny(entry["text"]); text != "" {
					return text
				}
			}
		}
	}
	return ""
}

func extractUserFromMediaPrompt(root map[string]any) string {
	// Walk common media-prompt keys; only fires when nothing else matched.
	keys := []string{"prompt", "input", "description", "query", "input_prompt", "text_prompt"}
	for _, k := range keys {
		if v, ok := root[k]; ok {
			if t := stringValueAny(v); t != "" {
				return t
			}
		}
	}
	return ""
}

// collectContentTexts handles both string content and structured content arrays
// (text blocks inside multimodal requests).
func collectContentTexts(value any) []string {
	switch typed := value.(type) {
	case string:
		if t := strings.TrimSpace(typed); t != "" {
			return []string{t}
		}
	case []any:
		var out []string
		for _, part := range typed {
			object, ok := part.(map[string]any)
			if !ok {
				continue
			}
			typeName := strings.ToLower(stringValueAny(object["type"]))
			if typeName != "" && typeName != "text" && typeName != "input_text" && typeName != "output_text" {
				continue
			}
			if text := stringValueAny(object["text"]); text != "" {
				out = append(out, text)
			}
		}
		return out
	case map[string]any:
		if text := stringValueAny(typed["text"]); text != "" {
			return []string{text}
		}
	}
	return nil
}

func stringValueAny(value any) string {
	if s, ok := value.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
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
