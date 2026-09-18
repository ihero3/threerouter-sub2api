package service

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/textproto"
	"strings"
	"testing"
)

// request_id 是本网关的幂等标识。OpenAI 官方 API 对未知顶层参数会整单拒绝
// （Unrecognized request argument），因此无论 JSON 还是 multipart 都必须剥离，
// 同时不能破坏 model 改写和 multipart 文件 part 的内容。

func TestRewriteOpenAIImagesModelStripsRequestIDJSON(t *testing.T) {
	body := []byte(`{"model":"gpt-image-1","prompt":"a cat","request_id":"gw-abc-123"}`)

	out, contentType, err := rewriteOpenAIImagesModel(body, "application/json", "dall-e-3")
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if contentType != "application/json" {
		t.Fatalf("content-type = %q", contentType)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal: %v (raw=%s)", err, out)
	}
	if _, exists := got["request_id"]; exists {
		t.Fatalf("request_id 未被剥离: %s", out)
	}
	if got["model"] != "dall-e-3" {
		t.Fatalf("model = %v, want dall-e-3", got["model"])
	}
	if got["prompt"] != "a cat" {
		t.Fatalf("prompt 被破坏: %v", got["prompt"])
	}
}

func TestRewriteOpenAIImagesModelStripsRequestIDWithoutModelRewrite(t *testing.T) {
	// 幂等剔除不依赖 model 改写：即使不改 model 也必须把 request_id 拿掉。
	body := []byte(`{"prompt":"a dog","request_id":"gw-999"}`)

	out, _, err := rewriteOpenAIImagesModel(body, "application/json", "")
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if strings.Contains(string(out), "request_id") {
		t.Fatalf("request_id 泄漏到上游: %s", out)
	}
}

func TestRewriteOpenAIImagesModelLeavesBodyWithoutRequestID(t *testing.T) {
	body := []byte(`{"model":"x","prompt":"p"}`)

	out, _, err := rewriteOpenAIImagesModel(body, "application/json", "")
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if string(out) != string(body) {
		t.Fatalf("无 request_id 时不应改动 body: got %s want %s", out, body)
	}
}

func TestRewriteOpenAIImagesMultipartStripsRequestIDAndKeepsFile(t *testing.T) {
	fileContent := []byte("fake-png-bytes-\x00\x01\x02")
	body, contentType := buildMultipartImageBody(t, map[string]string{
		"model":      "gpt-image-1",
		"prompt":     "make it blue",
		"request_id": "gw-multipart-1",
	}, fileContent)

	out, outType, err := rewriteOpenAIImagesModel(body, contentType, "dall-e-3")
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}

	fields, gotFile := parseMultipartImageBody(t, out, outType)
	if _, exists := fields["request_id"]; exists {
		t.Fatalf("multipart 中 request_id 未被剥离: fields=%v", fields)
	}
	if fields["model"] != "dall-e-3" {
		t.Fatalf("model = %q, want dall-e-3", fields["model"])
	}
	if fields["prompt"] != "make it blue" {
		t.Fatalf("prompt = %q, want %q", fields["prompt"], "make it blue")
	}
	if !bytes.Equal(gotFile, fileContent) {
		t.Fatalf("文件 part 内容被破坏: got %q want %q", gotFile, fileContent)
	}
}

func TestRewriteOpenAIImagesMultipartKeepsModelWhenNotRewritten(t *testing.T) {
	body, contentType := buildMultipartImageBody(t, map[string]string{
		"model":      "gpt-image-1",
		"request_id": "gw-multipart-2",
	}, []byte("data"))

	out, outType, err := rewriteOpenAIImagesModel(body, contentType, "")
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}

	fields, gotFile := parseMultipartImageBody(t, out, outType)
	if _, exists := fields["request_id"]; exists {
		t.Fatalf("multipart 中 request_id 未被剥离: fields=%v", fields)
	}
	if fields["model"] != "gpt-image-1" {
		t.Fatalf("未改写 model 时应保留原值，got %q", fields["model"])
	}
	if !bytes.Equal(gotFile, []byte("data")) {
		t.Fatalf("文件 part 内容被破坏: %q", gotFile)
	}
}

func buildMultipartImageBody(t *testing.T, fields map[string]string, fileContent []byte) ([]byte, string) {
	t.Helper()
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	for name, value := range fields {
		if _, err := writer.CreateFormField(name); err != nil {
			_ = writer.Close()
			t.Fatalf("create form field %s: %v", name, err)
		}
		if _, err := io.WriteString(&buffer, value); err != nil {
			_ = writer.Close()
			t.Fatalf("write form field %s: %v", name, err)
		}
	}
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="image"; filename="a.png"`)
	header.Set("Content-Type", "image/png")
	part, err := writer.CreatePart(header)
	if err != nil {
		_ = writer.Close()
		t.Fatalf("create file part: %v", err)
	}
	if _, err := part.Write(fileContent); err != nil {
		_ = writer.Close()
		t.Fatalf("write file part: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	return buffer.Bytes(), writer.FormDataContentType()
}

func parseMultipartImageBody(t *testing.T, body []byte, contentType string) (map[string]string, []byte) {
	t.Helper()
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("parse content-type %q: %v", contentType, err)
	}
	boundary := params["boundary"]
	if boundary == "" {
		t.Fatalf("no boundary in %q", contentType)
	}
	reader := multipart.NewReader(bytes.NewReader(body), boundary)
	fields := map[string]string{}
	var fileBytes []byte
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("next part: %v", err)
		}
		data, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("read part: %v", err)
		}
		_ = part.Close()
		if part.FileName() != "" {
			fileBytes = data
			continue
		}
		fields[part.FormName()] = string(data)
	}
	return fields, fileBytes
}
