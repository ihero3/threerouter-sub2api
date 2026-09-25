package apicompat

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
)

// itemIDCounter is a monotonic counter for generating unique item IDs.
// 必须是原子自增：网关并发转换多个请求时，普通 ++ 会 data race 并让两个
// 请求拿到同一个 ID，上游可能因重复 item id 拒绝整个 input 数组。
var itemIDCounter uint64

// generateItemID produces a Responses protocol-compliant item ID.
// Prefix must match the item type: "msg" for messages, "rs" for reasoning,
// "fc" for function/custom/tool_search calls. Upstream providers reject
// IDs that do not follow this convention.
func generateItemID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, atomic.AddUint64(&itemIDCounter, 1))
}

type chatMessageContent struct {
	Text  *string
	Parts []ChatContentPart
}

// ChatCompletionsToResponsesOptions controls optional history restoration
// when converting a Chat Completions request into Responses input items.
type ChatCompletionsToResponsesOptions struct {
	// ReasoningContentByCallID returns the plaintext reasoning that produced
	// the tool call identified by callID. DeepSeek/Kimi-style thinking
	// providers behind the Responses protocol require that reasoning to be
	// replayed on every follow-up turn as a reasoning item
	// (content[].reasoning_text) preceding the function_call item; clients
	// that only store Chat Completions history do not round-trip any
	// Responses reasoning item id, so the tool call id is the only stable
	// correlation key. Omitting it yields HTTP 400 "The `reasoning_text` in
	// the thinking mode must be passed back to the API".
	ReasoningContentByCallID func(callID string) string
}

// ChatCompletionsToResponses converts a Chat Completions request into a
// Responses API request. The upstream always streams, so Stream is forced to
// true. store is always false and reasoning.encrypted_content is always
// included so that the response translator has full context.
func ChatCompletionsToResponses(req *ChatCompletionsRequest) (*ResponsesRequest, error) {
	return ChatCompletionsToResponsesWithOptions(req, nil)
}

// ChatCompletionsToResponsesWithOptions is ChatCompletionsToResponses with
// optional history restoration (see ChatCompletionsToResponsesOptions).
func ChatCompletionsToResponsesWithOptions(req *ChatCompletionsRequest, opts *ChatCompletionsToResponsesOptions) (*ResponsesRequest, error) {
	input, err := convertChatMessagesToResponsesInput(req.Messages, opts)
	if err != nil {
		return nil, err
	}

	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}

	out := &ResponsesRequest{
		Model:             req.Model,
		Instructions:      req.Instructions,
		Input:             inputJSON,
		Stream:            true, // upstream always streams
		Include:           []string{"reasoning.encrypted_content"},
		ServiceTier:       req.ServiceTier,
		ParallelToolCalls: req.ParallelToolCalls,
	}

	// Reasoning models (gpt-5.x) do not accept sampling parameters.
	// See isReasoningModel in anthropic_to_responses.go.
	if !isReasoningModel(req.Model) {
		out.Temperature = req.Temperature
		out.TopP = req.TopP
	}

	storeFalse := false
	out.Store = &storeFalse

	// max_tokens / max_completion_tokens → max_output_tokens, prefer max_completion_tokens
	maxTokens := 0
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	}
	if req.MaxCompletionTokens != nil {
		maxTokens = *req.MaxCompletionTokens
	}
	if maxTokens >= minMaxOutputTokens {
		v := maxTokens
		out.MaxOutputTokens = &v
	}

	// reasoning_effort → reasoning.effort + reasoning.summary="auto"
	if req.ReasoningEffort != "" {
		out.Reasoning = &ResponsesReasoning{
			Effort:  req.ReasoningEffort,
			Summary: "auto",
		}
	}

	if format := chatResponseFormatToResponsesTextFormat(req.ResponseFormat); len(format) > 0 {
		if out.Text == nil {
			out.Text = &ResponsesText{}
		}
		out.Text.Format = format
	}

	// tools[] and legacy functions[] → ResponsesTool[]
	if len(req.Tools) > 0 || len(req.Functions) > 0 {
		out.Tools = convertChatToolsToResponses(req.Tools, req.Functions)
	}

	// tool_choice: already compatible format — pass through directly.
	// Legacy function_call needs mapping.
	if len(req.ToolChoice) > 0 {
		out.ToolChoice = req.ToolChoice
	} else if len(req.FunctionCall) > 0 {
		tc, err := convertChatFunctionCallToToolChoice(req.FunctionCall)
		if err != nil {
			return nil, fmt.Errorf("convert function_call: %w", err)
		}
		out.ToolChoice = tc
	}

	return out, nil
}

// convertChatMessagesToResponsesInput converts the Chat Completions messages
// array into a Responses API input items array.
func convertChatMessagesToResponsesInput(msgs []ChatMessage, opts *ChatCompletionsToResponsesOptions) ([]ResponsesInputItem, error) {
	var out []ResponsesInputItem
	for _, m := range msgs {
		items, err := chatMessageToResponsesItems(m, opts)
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
	}
	return out, nil
}

// chatMessageToResponsesItems converts a single ChatMessage into one or more
// ResponsesInputItem values.
func chatMessageToResponsesItems(m ChatMessage, opts *ChatCompletionsToResponsesOptions) ([]ResponsesInputItem, error) {
	switch m.Role {
	case "system":
		return chatSystemToResponses(m)
	case "user":
		return chatUserToResponses(m)
	case "assistant":
		return chatAssistantToResponses(m, opts)
	case "tool":
		return chatToolToResponses(m)
	case "function":
		return chatFunctionToResponses(m)
	default:
		return chatUserToResponses(m)
	}
}

// chatSystemToResponses converts a system message.
func chatSystemToResponses(m ChatMessage) ([]ResponsesInputItem, error) {
	parsed, err := parseChatMessageContent(m.Content)
	if err != nil {
		return nil, err
	}
	content, err := marshalChatInputContent(parsed)
	if err != nil {
		return nil, err
	}
	return []ResponsesInputItem{{Role: "system", Content: content}}, nil
}

// chatUserToResponses converts a user message, handling both plain strings and
// multi-modal content arrays.
func chatUserToResponses(m ChatMessage) ([]ResponsesInputItem, error) {
	parsed, err := parseChatMessageContent(m.Content)
	if err != nil {
		return nil, fmt.Errorf("parse user content: %w", err)
	}
	content, err := marshalChatInputContent(parsed)
	if err != nil {
		return nil, err
	}
	return []ResponsesInputItem{{Role: "user", Content: content}}, nil
}

// chatAssistantToResponses converts an assistant message. If there is both
// text content and tool_calls, the text is emitted as an assistant message
// first, then each tool_call becomes a function_call item. If the content is
// empty/nil and there are tool_calls, only function_call items are emitted.
//
// Tool-bearing turns carry special handling for DeepSeek/Kimi-style thinking
// providers on the Responses protocol: the reasoning that produced the tool
// calls must be replayed as a standalone reasoning item
// (content[].reasoning_text) before the function_call items — wrapping it in
// visible <thinking> text does not satisfy the upstream contract and yields
// HTTP 400 ("The `reasoning_text` in the thinking mode must be passed back
// to the API").
func chatAssistantToResponses(m ChatMessage, opts *ChatCompletionsToResponsesOptions) ([]ResponsesInputItem, error) {
	var items []ResponsesInputItem

	// Resolve the reasoning that produced this turn: prefer what the client
	// echoed, otherwise restore from the gateway-side cache keyed by tool
	// call id — the only id that round-trips in Chat Completions history.
	reasoning := strings.TrimSpace(m.ReasoningContent)
	if len(m.ToolCalls) > 0 && reasoning == "" && opts != nil && opts.ReasoningContentByCallID != nil {
		for _, tc := range m.ToolCalls {
			if cached := strings.TrimSpace(opts.ReasoningContentByCallID(tc.ID)); cached != "" {
				reasoning = cached
				break
			}
		}
	}

	emittedReasoningItem := false
	if len(m.ToolCalls) > 0 && reasoning != "" {
		reasoningItem, err := makeReasoningReplayInputItem(reasoning)
		if err != nil {
			return nil, err
		}
		items = append(items, reasoningItem)
		emittedReasoningItem = true
	}

	content := ""

	// Without a standalone reasoning item, keep the historical behavior of
	// carrying reasoning as tagged visible text. When a reasoning item is
	// emitted above, the same text must not also appear in the message.
	if !emittedReasoningItem && m.ReasoningContent != "" {
		content = "<thinking>" + m.ReasoningContent + "</thinking>"
	}

	// Emit assistant message with output_text if content is non-empty.
	if len(m.Content) > 0 {
		s, err := parseAssistantContent(m.Content)
		if err != nil {
			return nil, err
		}
		if s != "" {
			if content != "" {
				content += "\n"
			}
			content += s
		}
	}

	if content != "" {
		parts := []ResponsesContentPart{{Type: "output_text", Text: content}}
		partsJSON, err := json.Marshal(parts)
		if err != nil {
			return nil, err
		}
		items = append(items, ResponsesInputItem{Role: "assistant", Content: partsJSON})
	}

	// Emit one function_call item per tool_call.
	for _, tc := range m.ToolCalls {
		args := tc.Function.Arguments
		if args == "" {
			args = "{}"
		}
		items = append(items, ResponsesInputItem{
			Type:      "function_call",
			CallID:    tc.ID,
			Name:      tc.Function.Name,
			Arguments: args,
		})
	}

	return items, nil
}

// makeReasoningReplayInputItem builds the reasoning input item that
// DeepSeek/Kimi Responses endpoints require replaying before the
// function_call items of a tool-bearing turn. Both the portable summary and
// the reasoning_text content are populated: some providers validate
// content[].reasoning_text while others only read summary.
func makeReasoningReplayInputItem(reasoning string) (ResponsesInputItem, error) {
	contentJSON, err := json.Marshal([]ResponsesContentPart{{Type: "reasoning_text", Text: reasoning}})
	if err != nil {
		return ResponsesInputItem{}, err
	}
	return ResponsesInputItem{
		Type:    "reasoning",
		ID:      generateItemID("rs"),
		Summary: []ResponsesSummary{{Type: "summary_text", Text: reasoning}},
		Content: contentJSON,
	}, nil
}

// parseAssistantContent returns assistant content as plain text.
//
// Supported formats:
// - JSON string
// - JSON array of typed parts (e.g. [{"type":"text","text":"..."}])
//
// For structured thinking/reasoning parts, it preserves semantics by wrapping
// the text in explicit tags so downstream can still distinguish it from normal text.
func parseAssistantContent(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}

	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, nil
	}

	var parts []map[string]any
	if err := json.Unmarshal(raw, &parts); err != nil {
		// Keep compatibility with prior behavior: unsupported assistant content
		// formats are ignored instead of failing the whole request conversion.
		return "", nil
	}

	var b strings.Builder
	write := func(v string) error {
		_, err := b.WriteString(v)
		return err
	}
	for _, p := range parts {
		typ, _ := p["type"].(string)
		text, _ := p["text"].(string)
		thinking, _ := p["thinking"].(string)

		switch typ {
		case "thinking", "reasoning":
			if thinking != "" {
				if err := write("<thinking>"); err != nil {
					return "", err
				}
				if err := write(thinking); err != nil {
					return "", err
				}
				if err := write("</thinking>"); err != nil {
					return "", err
				}
			} else if text != "" {
				if err := write("<thinking>"); err != nil {
					return "", err
				}
				if err := write(text); err != nil {
					return "", err
				}
				if err := write("</thinking>"); err != nil {
					return "", err
				}
			}
		default:
			if text != "" {
				if err := write(text); err != nil {
					return "", err
				}
			}
		}
	}

	return b.String(), nil
}

// chatToolToResponses converts a tool result message (role=tool) into a
// function_call_output item.
func chatToolToResponses(m ChatMessage) ([]ResponsesInputItem, error) {
	output, err := parseChatContent(m.Content)
	if err != nil {
		return nil, err
	}
	if output == "" {
		output = "(empty)"
	}
	return []ResponsesInputItem{{
		Type:   "function_call_output",
		CallID: m.ToolCallID,
		Output: output,
	}}, nil
}

// chatFunctionToResponses converts a legacy function result message
// (role=function) into a function_call_output item. The Name field is used as
// call_id since legacy function calls do not carry a separate call_id.
func chatFunctionToResponses(m ChatMessage) ([]ResponsesInputItem, error) {
	output, err := parseChatContent(m.Content)
	if err != nil {
		return nil, err
	}
	if output == "" {
		output = "(empty)"
	}
	return []ResponsesInputItem{{
		Type:   "function_call_output",
		CallID: m.Name,
		Output: output,
	}}, nil
}

// parseChatContent returns the string value of a ChatMessage Content field.
// Content can be a JSON string or an array of typed parts. Array content is
// flattened to text by concatenating text parts and ignoring non-text parts.
func parseChatContent(raw json.RawMessage) (string, error) {
	parsed, err := parseChatMessageContent(raw)
	if err != nil {
		return "", err
	}
	if parsed.Text != nil {
		return *parsed.Text, nil
	}
	return flattenChatContentParts(parsed.Parts), nil
}

func parseChatMessageContent(raw json.RawMessage) (chatMessageContent, error) {
	if len(raw) == 0 {
		return chatMessageContent{Text: stringPtr("")}, nil
	}

	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return chatMessageContent{Text: &s}, nil
	}

	var parts []ChatContentPart
	if err := json.Unmarshal(raw, &parts); err == nil {
		return chatMessageContent{Parts: parts}, nil
	}

	return chatMessageContent{}, fmt.Errorf("parse content as string or parts array")
}

func marshalChatInputContent(content chatMessageContent) (json.RawMessage, error) {
	if content.Text != nil {
		return json.Marshal(*content.Text)
	}
	parts := convertChatContentPartsToResponses(content.Parts)
	if len(parts) == 0 {
		// A nil slice marshals to JSON null, which the upstream Responses API
		// rejects ("expected an array of objects or string, but got null").
		// Fall back to an empty string when no usable parts remain.
		return json.Marshal("")
	}
	return json.Marshal(parts)
}

func convertChatContentPartsToResponses(parts []ChatContentPart) []ResponsesContentPart {
	var responseParts []ResponsesContentPart
	for _, p := range parts {
		switch p.Type {
		case "text":
			if p.Text != "" {
				responseParts = append(responseParts, ResponsesContentPart{
					Type: "input_text",
					Text: p.Text,
				})
			}
		case "image_url":
			if p.ImageURL != nil && p.ImageURL.URL != "" && !isEmptyBase64DataURI(p.ImageURL.URL) {
				responseParts = append(responseParts, ResponsesContentPart{
					Type:     "input_image",
					ImageURL: p.ImageURL.URL,
				})
			}
		case "file":
			if p.File != nil && (p.File.FileData != "" || p.File.FileID != "") {
				responseParts = append(responseParts, ResponsesContentPart{
					Type:     "input_file",
					Filename: p.File.Filename,
					FileData: p.File.FileData,
					FileID:   p.File.FileID,
				})
			}
		}
	}
	return responseParts
}

func isEmptyBase64DataURI(raw string) bool {
	if !strings.HasPrefix(raw, "data:") {
		return false
	}
	rest := strings.TrimPrefix(raw, "data:")
	semicolonIdx := strings.Index(rest, ";")
	if semicolonIdx < 0 {
		return false
	}
	rest = rest[semicolonIdx+1:]
	if !strings.HasPrefix(rest, "base64,") {
		return false
	}
	return strings.TrimSpace(strings.TrimPrefix(rest, "base64,")) == ""
}

func flattenChatContentParts(parts []ChatContentPart) string {
	var textParts []string
	for _, p := range parts {
		if p.Type == "text" && p.Text != "" {
			textParts = append(textParts, p.Text)
		}
	}
	return strings.Join(textParts, "")
}

func stringPtr(s string) *string {
	return &s
}

// convertChatToolsToResponses maps Chat Completions tool definitions and legacy
// function definitions to Responses API tool definitions.
func convertChatToolsToResponses(tools []ChatTool, functions []ChatFunction) []ResponsesTool {
	var out []ResponsesTool

	for _, t := range tools {
		toolType := strings.ToLower(strings.TrimSpace(t.Type))
		if toolType == "x_search" {
			out = append(out, ResponsesTool{
				Type:                     "x_search",
				AllowedXHandles:          t.AllowedXHandles,
				ExcludedXHandles:         t.ExcludedXHandles,
				FromDate:                 t.FromDate,
				ToDate:                   t.ToDate,
				EnableImageUnderstanding: t.EnableImageUnderstanding,
				EnableVideoUnderstanding: t.EnableVideoUnderstanding,
			})
			continue
		}
		if toolType == "web_search" || toolType == "code_execution" {
			out = append(out, ResponsesTool{Type: toolType})
			continue
		}
		if t.Type != "function" || t.Function == nil {
			continue
		}
		rt := ResponsesTool{
			Type:        "function",
			Name:        t.Function.Name,
			Description: t.Function.Description,
			Parameters:  t.Function.Parameters,
			Strict:      defaultStrictFalse(t.Function.Strict),
		}
		out = append(out, rt)
	}

	// Legacy functions[] are treated as function-type tools.
	for _, f := range functions {
		rt := ResponsesTool{
			Type:        "function",
			Name:        f.Name,
			Description: f.Description,
			Parameters:  f.Parameters,
			Strict:      defaultStrictFalse(f.Strict),
		}
		out = append(out, rt)
	}

	return out
}

func defaultStrictFalse(src *bool) *bool {
	if src == nil {
		value := false
		return &value
	}
	return src
}

// convertChatFunctionCallToToolChoice maps the legacy function_call field to a
// Responses API tool_choice value.
//
//	"auto" → "auto"
//	"none" → "none"
//	{"name":"X"} → {"type":"function","name":"X"}
func convertChatFunctionCallToToolChoice(raw json.RawMessage) (json.RawMessage, error) {
	// Try string first ("auto", "none", etc.) — pass through as-is.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return json.Marshal(s)
	}

	// Object form: {"name":"X"}
	var obj struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"type": "function",
		"name": obj.Name,
	})
}
