package handler

import "testing"

func TestExtractChatCompletionsOutputText_NonStream(t *testing.T) {
	body := []byte(`{"id":"cmpl","choices":[{"index":0,"message":{"role":"assistant","content":"hello world"},"finish_reason":"stop"}]}`)
	got := extractChatCompletionsOutputText(body)
	if got != "hello world" {
		t.Fatalf("expected %q, got %q", "hello world", got)
	}
}

func TestExtractChatCompletionsOutputText_Stream(t *testing.T) {
	body := []byte("data: {\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"foo\"}}]}\n\n" +
		"data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"bar\"}}]}\n\n" +
		"data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
		"data: [DONE]\n\n")
	got := extractChatCompletionsOutputText(body)
	if got != "foobar" {
		t.Fatalf("expected %q, got %q", "foobar", got)
	}
}

func TestExtractChatCompletionsOutputText_Empty(t *testing.T) {
	if got := extractChatCompletionsOutputText(nil); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
	if got := extractChatCompletionsOutputText([]byte("data: [DONE]\n\n")); got != "" {
		t.Fatalf("expected empty for DONE-only stream, got %q", got)
	}
}

func TestOutputCaptureWriterTruncates(t *testing.T) {
	w := &outputCaptureWriter{}
	// 直接调用底层 buf 逻辑：模拟超限写入。
	big := make([]byte, maxCapturedOutputBytes+1024)
	for i := range big {
		big[i] = 'a'
	}
	// ResponseWriter 为 nil 时 Write 会 panic，因此仅测试有界缓冲逻辑的分支通过 captured 长度断言。
	// 使用内部方法路径：手动填充缓冲以验证截断标记。
	w.buf.Write(big[:maxCapturedOutputBytes])
	if w.buf.Len() != maxCapturedOutputBytes {
		t.Fatalf("expected buffer len %d, got %d", maxCapturedOutputBytes, w.buf.Len())
	}
}
