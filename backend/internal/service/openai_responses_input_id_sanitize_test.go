package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestSanitizeOpenAIResponsesInputItemIDs_StripsLegacyIDs(t *testing.T) {
	body, changed, err := sanitizeOpenAIResponsesInputItemIDs([]byte(`{
		"model":"gpt-5.4",
		"input":[
			{"type":"message","id":"item_msg_old","role":"assistant","content":[{"type":"output_text","text":"hi"}]},
			{"type":"function_call","id":"item_fc_old","call_id":"call_1","name":"exec","arguments":"{}"},
			{"type":"function_call_output","call_id":"call_1","output":"ok"},
			{"type":"reasoning","id":"item_rs_old","summary":[{"type":"summary_text","text":"plan"}]},
			{"type":"item_reference","id":"item_fc_old"}
		]
	}`))
	require.NoError(t, err)
	require.True(t, changed)

	// Invalid-prefix ids are removed in place, never rewritten: a fabricated
	// id could point at a different upstream object (see commit b2b2adcf8).
	require.False(t, gjson.GetBytes(body, "input.0.id").Exists(), "legacy message id must be stripped")
	require.False(t, gjson.GetBytes(body, "input.1.id").Exists(), "legacy function_call id must be stripped")
	require.False(t, gjson.GetBytes(body, "input.3.id").Exists(), "legacy reasoning id must be stripped")
	// item_reference ids are not prefix-constrained and stay untouched.
	require.Equal(t, "item_fc_old", gjson.GetBytes(body, "input.4.id").String())
	// Pairing call_ids are never part of id sanitization.
	require.Equal(t, "call_1", gjson.GetBytes(body, "input.1.call_id").String())
	require.Equal(t, "call_1", gjson.GetBytes(body, "input.2.call_id").String())
}

func TestSanitizeOpenAIResponsesInputItemIDs_KeepsValidIDs(t *testing.T) {
	original := []byte(`{
		"model":"gpt-5.4",
		"input":[
			{"type":"message","id":"msg_1","role":"assistant","content":[{"type":"output_text","text":"hi"}]},
			{"type":"function_call","id":"fc_1","call_id":"call_1","name":"exec","arguments":"{}"},
			{"type":"reasoning","id":"rs_1","summary":[{"type":"summary_text","text":"plan"}]}
		]
	}`)

	body, changed, err := sanitizeOpenAIResponsesInputItemIDs(original)
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, string(original), string(body))
}

func TestSanitizeOpenAIResponsesInputItemIDs_PreservesReasoningContent(t *testing.T) {
	// id sanitization must never touch the reasoning item body: DeepSeek/Kimi
	// thinking mode requires content[].reasoning_text (and summary) replayed
	// verbatim on follow-up tool-call turns.
	body, changed, err := sanitizeOpenAIResponsesInputItemIDs([]byte(`{
		"model":"deepseek-v4",
		"input":[
			{"type":"message","id":"msg_1","role":"user","content":[{"type":"input_text","text":"hello"}]},
			{"type":"reasoning","id":"rs_1","content":[{"type":"reasoning_text","text":"thinking step"}],"summary":[{"type":"summary_text","text":"thinking step"}]}
		]
	}`))
	require.NoError(t, err)
	require.False(t, changed)
	// reasoning item keeps both the reasoning_text content and the summary
	require.True(t, gjson.GetBytes(body, "input.1.content").Exists(), "reasoning input item must keep content")
	require.Equal(t, "thinking step", gjson.GetBytes(body, "input.1.content.0.text").String())
	require.True(t, gjson.GetBytes(body, "input.1.summary").Exists(), "reasoning input item must keep summary")
	require.Equal(t, "thinking step", gjson.GetBytes(body, "input.1.summary.0.text").String())
}

func TestSanitizeOpenAIResponsesInputItemIDs_KeepsNonReasoningContent(t *testing.T) {
	body, changed, err := sanitizeOpenAIResponsesInputItemIDs([]byte(`{
		"model":"deepseek-v4",
		"input":[
			{"type":"message","id":"msg_1","role":"user","content":[{"type":"input_text","text":"hello"}]}
		]
	}`))
	require.NoError(t, err)
	require.False(t, changed)
	// message item's content must be preserved
	require.True(t, gjson.GetBytes(body, "input.0.content").Exists(), "message content should be preserved")
}
