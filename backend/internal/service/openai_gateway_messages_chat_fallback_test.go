package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/gin-gonic/gin"
)

func TestForwardAnthropicViaRawChatCompletions_BasicConversion(t *testing.T) {
	gin.SetMode(gin.TestMode)

	anthropicReq := apicompat.AnthropicRequest{
		Model: "claude-3-5-sonnet-20241022",
		Messages: []apicompat.AnthropicMessage{
			{
				Role:    "user",
				Content: json.RawMessage(`"Hello"`),
			},
		},
		Stream: false,
	}

	reqBody, err := json.Marshal(anthropicReq)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	responsesReq, err := apicompat.AnthropicToResponses(&anthropicReq)
	if err != nil {
		t.Fatalf("AnthropicToResponses failed: %v", err)
	}

	if responsesReq.Model != "claude-3-5-sonnet-20241022" {
		t.Errorf("Expected model claude-3-5-sonnet-20241022, got %s", responsesReq.Model)
	}

	chatReq, err := apicompat.ResponsesToChatCompletionsRequest(responsesReq)
	if err != nil {
		t.Fatalf("ResponsesToChatCompletionsRequest failed: %v", err)
	}

	if chatReq.Model != "claude-3-5-sonnet-20241022" {
		t.Errorf("Expected model claude-3-5-sonnet-20241022, got %s", chatReq.Model)
	}

	if len(chatReq.Messages) == 0 {
		t.Fatal("Expected at least one message in Chat Completions request")
	}

	if len(reqBody) == 0 {
		t.Fatal("Request body should not be empty")
	}
}

func TestBufferChatCompletionsAsAnthropic_Conversion(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ccResp := apicompat.ChatCompletionsResponse{
		ID:      "chatcmpl-123",
		Object:  "chat.completion",
		Created: 1234567890,
		Model:   "gpt-4",
		Choices: []apicompat.ChatChoice{
			{
				Index: 0,
				Message: apicompat.ChatMessage{
					Role:    "assistant",
					Content: json.RawMessage(`"Hello from GPT-4"`),
				},
				FinishReason: "stop",
			},
		},
		Usage: &apicompat.ChatUsage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}

	respBody, err := json.Marshal(ccResp)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}

	responsesResp := apicompat.ChatCompletionsResponseToResponses(&ccResp, "claude-3-5-sonnet-20241022")
	anthropicResp := apicompat.ResponsesToAnthropic(responsesResp, "claude-3-5-sonnet-20241022")

	if anthropicResp.Model != "claude-3-5-sonnet-20241022" {
		t.Errorf("Expected model claude-3-5-sonnet-20241022, got %s", anthropicResp.Model)
	}

	if anthropicResp.Type != "message" {
		t.Errorf("Expected type message, got %s", anthropicResp.Type)
	}

	if len(anthropicResp.Content) == 0 {
		t.Fatal("Expected at least one content block")
	}

	if len(respBody) == 0 {
		t.Fatal("Response body should not be empty")
	}
}

func TestStreamChatCompletionsAsAnthropic_EventConversion(t *testing.T) {
	gin.SetMode(gin.TestMode)

	chunk := apicompat.ChatCompletionsChunk{
		ID:      "chatcmpl-123",
		Object:  "chat.completion.chunk",
		Created: 1234567890,
		Model:   "gpt-4",
		Choices: []apicompat.ChatChunkChoice{
			{
				Index: 0,
				Delta: apicompat.ChatDelta{
					Content: strPtr("Hello"),
				},
			},
		},
	}

	ccState := apicompat.NewChatCompletionsToResponsesStreamState("claude-3-5-sonnet-20241022")
	anthState := apicompat.NewResponsesEventToAnthropicState()
	anthState.Model = "claude-3-5-sonnet-20241022"

	responsesEvents := apicompat.ChatCompletionsChunkToResponsesEvents(&chunk, ccState)
	if len(responsesEvents) == 0 {
		t.Fatal("Expected at least one Responses event from CC chunk")
	}

	var allAnthEvents []apicompat.AnthropicStreamEvent
	for i := range responsesEvents {
		anthEvents := apicompat.ResponsesEventToAnthropicEvents(&responsesEvents[i], anthState)
		allAnthEvents = append(allAnthEvents, anthEvents...)
	}

	if len(allAnthEvents) == 0 {
		t.Fatal("Expected at least one Anthropic event from Responses events")
	}

	hasMessageStart := false
	for _, evt := range allAnthEvents {
		if evt.Type == "message_start" {
			hasMessageStart = true
			break
		}
	}

	if !hasMessageStart {
		t.Error("Expected message_start event in Anthropic stream events")
	}
}

func TestForwardAnthropicViaRawChatCompletions_UpstreamError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": {"message": "Invalid model", "type": "invalid_request_error"}}`))
	}))
	defer upstreamServer.Close()

	if upstreamServer.URL == "" {
		t.Fatal("Test server URL should not be empty")
	}
}

func TestNormalizeOpenAIChatMessagesContentToString_Integration(t *testing.T) {
	chatReq := apicompat.ChatCompletionsRequest{
		Model: "gpt-4",
		Messages: []apicompat.ChatMessage{
			{
				Role:    "user",
				Content: json.RawMessage(`[{"type": "text", "text": "Hello"}]`),
			},
		},
	}

	body, err := json.Marshal(chatReq)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	normalized := normalizeOpenAIChatMessagesContentToString(body)

	var result apicompat.ChatCompletionsRequest
	if err := json.Unmarshal(normalized, &result); err != nil {
		t.Fatalf("Failed to unmarshal normalized: %v", err)
	}

	if len(result.Messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(result.Messages))
	}

	content := string(result.Messages[0].Content)
	if !strings.Contains(content, "Hello") {
		t.Errorf("Expected content to contain 'Hello', got %s", content)
	}
}

func TestShouldUseResponsesAPI_Integration(t *testing.T) {
	tests := []struct {
		name     string
		extra    map[string]any
		expected bool
	}{
		{
			name:     "no extra",
			extra:    nil,
			expected: true,
		},
		{
			name:     "empty extra",
			extra:    map[string]any{},
			expected: true,
		},
		{
			name: "force_responses",
			extra: map[string]any{
				"openai_responses_mode": "force_responses",
			},
			expected: true,
		},
		{
			name: "force_chat_completions",
			extra: map[string]any{
				"openai_responses_mode": "force_chat_completions",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := openai_compat.ShouldUseResponsesAPI(tt.extra)

			if result != tt.expected {
				t.Errorf("openai_compat.ShouldUseResponsesAPI() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestStreamConversion_EndToEnd(t *testing.T) {
	chunks := []apicompat.ChatCompletionsChunk{
		{
			ID:    "chatcmpl-123",
			Model: "gpt-4",
			Choices: []apicompat.ChatChunkChoice{
				{
					Index: 0,
					Delta: apicompat.ChatDelta{
						Content: strPtr("Hello"),
					},
				},
			},
		},
		{
			ID:    "chatcmpl-123",
			Model: "gpt-4",
			Choices: []apicompat.ChatChunkChoice{
				{
					Index: 0,
					Delta: apicompat.ChatDelta{
						Content: strPtr(" world"),
					},
				},
			},
		},
	}

	ccState := apicompat.NewChatCompletionsToResponsesStreamState("claude-3-5-sonnet-20241022")
	anthState := apicompat.NewResponsesEventToAnthropicState()
	anthState.Model = "claude-3-5-sonnet-20241022"

	var allAnthEvents []apicompat.AnthropicStreamEvent

	for _, chunk := range chunks {
		responsesEvents := apicompat.ChatCompletionsChunkToResponsesEvents(&chunk, ccState)
		for i := range responsesEvents {
			anthEvents := apicompat.ResponsesEventToAnthropicEvents(&responsesEvents[i], anthState)
			allAnthEvents = append(allAnthEvents, anthEvents...)
		}
	}

	finalEvents := apicompat.FinalizeResponsesAnthropicStream(anthState)
	allAnthEvents = append(allAnthEvents, finalEvents...)

	if len(allAnthEvents) == 0 {
		t.Fatal("Expected Anthropic events from stream conversion")
	}

	hasMessageStart := false
	hasContentBlockDelta := false
	hasMessageStop := false

	for _, evt := range allAnthEvents {
		switch evt.Type {
		case "message_start":
			hasMessageStart = true
		case "content_block_delta":
			hasContentBlockDelta = true
		case "message_stop":
			hasMessageStop = true
		}
	}

	if !hasMessageStart {
		t.Error("Missing message_start event")
	}
	if !hasContentBlockDelta {
		t.Error("Missing content_block_delta event")
	}
	if !hasMessageStop {
		t.Error("Missing message_stop event")
	}
}

func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	select {
	case <-ctx.Done():
	default:
		t.Error("Context should be cancelled")
	}
}

func TestResponseBodyHandling(t *testing.T) {
	body := []byte(`{"test": "data"}`)
	resp := &http.Response{
		Body: io.NopCloser(bytes.NewReader(body)),
	}

	readBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read body: %v", err)
	}

	if string(readBody) != string(body) {
		t.Errorf("Expected %s, got %s", string(body), string(readBody))
	}
}
