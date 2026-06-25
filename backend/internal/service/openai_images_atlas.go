package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
)

// AtlasCloud GPT-Image-2 adapter.
//
// AtlasCloud exposes image generation through a non-standard asynchronous
// endpoint:
//
//	POST /api/v1/model/generateImage
//	GET  /api/v1/model/prediction/{id}
//
// This adapter translates the OpenAI /v1/images/generations protocol to the
// AtlasCloud protocol, polls the prediction URL until completion, and returns
// an OpenAI-compatible response. Keeping the code in a separate file makes
// upstream merges easier because the standard OpenAI image path in
// openai_images.go is touched only by a single early-return branch.

const (
	atlasCloudGenerateImagePath = "/api/v1/model/generateImage"
	atlasCloudPollInterval      = 1 * time.Second
	atlasCloudPollTimeout       = 180 * time.Second
)

// atlasCloudGenerateRequest is the payload sent to AtlasCloud.
type atlasCloudGenerateRequest struct {
	Model   string `json:"model"`
	Prompt  string `json:"prompt"`
	Size    string `json:"size,omitempty"`
	Quality string `json:"quality,omitempty"`
}

// atlasCloudGenerateResponse wraps the create/poll response body from AtlasCloud.
type atlasCloudGenerateResponse struct {
	Code    int                 `json:"code"`
	Message string              `json:"message"`
	Data    atlasCloudImageTask `json:"data"`
}

// atlasCloudImageTask is the nested task object returned by AtlasCloud.
type atlasCloudImageTask struct {
	ID      string               `json:"id"`
	Model   string               `json:"model"`
	Status  string               `json:"status"`
	Error   string               `json:"error"`
	URLs    atlasCloudTaskURLs   `json:"urls"`
	Outputs json.RawMessage      `json:"outputs"`
	Output  *atlasCloudImageOutput `json:"output"`
}

type atlasCloudTaskURLs struct {
	Get string `json:"get"`
}

type atlasCloudImageOutput struct {
	URL      string `json:"url"`
	ImageURL string `json:"image_url"`
	B64JSON  string `json:"b64_json"`
}

// openAIImagesGenerationsResponse is a best-effort OpenAI-compatible response
// written back to the downstream client.
type openAIImagesGenerationsResponse struct {
	Created int64                      `json:"created"`
	Data    []openAIImagesGenerationData `json:"data"`
}

type openAIImagesGenerationData struct {
	URL      string `json:"url,omitempty"`
	B64JSON  string `json:"b64_json,omitempty"`
}

// isAtlasCloudImagesAccount detects whether an account is configured to use
// the AtlasCloud non-standard image generation endpoint.
func isAtlasCloudImagesAccount(account *Account, upstreamModel string) bool {
	baseURL := strings.TrimSpace(account.GetOpenAIBaseURL())
	if baseURL == "" {
		return false
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return false
	}
	path := strings.ToLower(strings.TrimRight(u.Path, "/"))
	if strings.HasSuffix(path, strings.ToLower(atlasCloudGenerateImagePath)) {
		return true
	}
	// Fallback: the upstream model itself follows AtlasCloud's naming.
	return strings.Contains(strings.ToLower(upstreamModel), "gpt-image-2")
}

// forwardAtlasCloudImages handles the complete AtlasCloud image generation flow
// for an API-key account.
func (s *OpenAIGatewayService) forwardAtlasCloudImages(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	parsed *OpenAIImagesRequest,
	requestModel string,
	upstreamModel string,
) (*OpenAIForwardResult, error) {
	startTime := time.Now()

	if parsed.Prompt == "" {
		return nil, fmt.Errorf("prompt is required for AtlasCloud image generation")
	}

	atlasReq := atlasCloudGenerateRequest{
		Model:   upstreamModel,
		Prompt:  strings.TrimSpace(parsed.Prompt),
		Size:    strings.TrimSpace(parsed.Size),
		Quality: strings.TrimSpace(parsed.Quality),
	}

	forwardBody, err := json.Marshal(atlasReq)
	if err != nil {
		return nil, fmt.Errorf("marshal atlascloud image request: %w", err)
	}

	baseURL := strings.TrimSpace(account.GetOpenAIBaseURL())
	validatedURL, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, validatedURL, bytes.NewReader(forwardBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	token, _, err := s.GetAccessToken(ctx, account)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}

	logger.LegacyPrintf(
		"service.openai_gateway",
		"[OpenAI] AtlasCloud image generation request_model=%s upstream_model=%s endpoint=%s",
		requestModel,
		upstreamModel,
		parsed.Endpoint,
	)

	upstreamStart := time.Now()
	resp, err := s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(upstreamStart).Milliseconds())
	if err != nil {
		safeErr := sanitizeUpstreamErrorMessage(err.Error())
		setOpsUpstreamError(c, 0, safeErr, "")
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform:           account.Platform,
			AccountID:          account.ID,
			AccountName:        account.Name,
			UpstreamStatusCode: 0,
			UpstreamURL:        safeUpstreamURL(req.URL.String()),
			Kind:               "request_error",
			Message:            safeErr,
		})
		return nil, fmt.Errorf("atlascloud image generation request failed: %s", safeErr)
	}

	respBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	_ = resp.Body.Close()
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		upstreamMsg := strings.TrimSpace(extractUpstreamErrorMessage(respBody))
		upstreamMsg = sanitizeUpstreamErrorMessage(upstreamMsg)
		setOpsUpstreamError(c, resp.StatusCode, upstreamMsg, "")
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform:           account.Platform,
			AccountID:          account.ID,
			AccountName:        account.Name,
			UpstreamStatusCode: resp.StatusCode,
			UpstreamRequestID:  resp.Header.Get("x-request-id"),
			UpstreamURL:        safeUpstreamURL(req.URL.String()),
			Kind:               "upstream_error",
			Message:            upstreamMsg,
		})
		return nil, fmt.Errorf("atlascloud image generation upstream error: status=%d message=%s", resp.StatusCode, upstreamMsg)
	}

	var atlasResp atlasCloudGenerateResponse
	if err := json.Unmarshal(respBody, &atlasResp); err != nil {
		return nil, fmt.Errorf("decode atlascloud image generation response: %w", err)
	}
	if atlasResp.Code != 0 && atlasResp.Code != 200 {
		return nil, fmt.Errorf("atlascloud image generation error: code=%d message=%s", atlasResp.Code, atlasResp.Message)
	}

	result, err := s.pollAtlasCloudImageTask(ctx, c, account, &atlasResp.Data, proxyURL, token)
	if err != nil {
		return nil, err
	}

	openAIResp := buildAtlasCloudOpenAIResponse(requestModel, result)
	respBytes, err := json.Marshal(openAIResp)
	if err != nil {
		return nil, fmt.Errorf("encode openai image generation response: %w", err)
	}

	c.Data(http.StatusOK, "application/json", respBytes)

	forwardResult := &OpenAIForwardResult{
		RequestID:     resp.Header.Get("x-request-id"),
		Model:         requestModel,
		UpstreamModel: upstreamModel,
		ImageCount:    len(openAIResp.Data),
		ImageSize:     strings.TrimSpace(parsed.Size),
		Duration:      time.Since(startTime),
	}
	ApplyOpenAIImageBillingResolution(forwardResult)
	return forwardResult, nil
}

// pollAtlasCloudImageTask polls the AtlasCloud prediction endpoint until the
// task reaches a terminal state or the configured timeout expires.
func (s *OpenAIGatewayService) pollAtlasCloudImageTask(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	initial *atlasCloudImageTask,
	proxyURL string,
	token string,
) (*atlasCloudImageTask, error) {
	status := strings.ToLower(strings.TrimSpace(initial.Status))
	if status == "success" || status == "completed" {
		return initial, nil
	}

	pollURL := strings.TrimSpace(initial.URLs.Get)
	if pollURL == "" {
		return nil, fmt.Errorf("atlascloud image task has no polling URL")
	}

	timeout := time.NewTimer(atlasCloudPollTimeout)
	defer timeout.Stop()

	for {
		select {
		case <-timeout.C:
			return nil, fmt.Errorf("atlascloud image generation timed out after %s", atlasCloudPollTimeout)
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, pollURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")

		resp, err := s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
		if err != nil {
			return nil, fmt.Errorf("atlascloud image poll request failed: %w", err)
		}

		body, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
		_ = resp.Body.Close()
		if err != nil {
			return nil, err
		}

		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("atlascloud image poll upstream error: status=%d body=%s", resp.StatusCode, string(body))
		}

		var pollResp atlasCloudGenerateResponse
		if err := json.Unmarshal(body, &pollResp); err != nil {
			return nil, fmt.Errorf("decode atlascloud image poll response: %w", err)
		}
		if pollResp.Code != 0 && pollResp.Code != 200 {
			return nil, fmt.Errorf("atlascloud image poll error: code=%d message=%s", pollResp.Code, pollResp.Message)
		}

		status := strings.ToLower(strings.TrimSpace(pollResp.Data.Status))
		logger.LegacyPrintf(
			"service.openai_gateway",
			"[OpenAI] AtlasCloud image poll prediction_id=%s status=%s",
			pollResp.Data.ID,
			pollResp.Data.Status,
		)
		switch status {
		case "success", "completed":
			return &pollResp.Data, nil
		case "failed", "error":
			return nil, fmt.Errorf("atlascloud image generation failed: %s", pollResp.Data.Error)
		case "processing", "pending":
			// continue polling
		default:
			// Treat unknown statuses as still processing.
		}

		select {
		case <-timeout.C:
			return nil, fmt.Errorf("atlascloud image generation timed out after %s", atlasCloudPollTimeout)
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(atlasCloudPollInterval):
		}
	}
}

// buildAtlasCloudOpenAIResponse converts the AtlasCloud task result into an an
// OpenAI-compatible images/generations response. It downloads each generated
// image from the AtlasCloud CDN and encodes it as base64 so that clients can
// use the image without worrying about OSS referer/browser 403 issues.
func buildAtlasCloudOpenAIResponse(requestedModel string, task *atlasCloudImageTask) openAIImagesGenerationsResponse {
	resp := openAIImagesGenerationsResponse{
		Created: time.Now().Unix(),
		Data:    []openAIImagesGenerationData{},
	}

	for _, out := range collectAtlasCloudImageOutputs(task) {
		imageURL := out.URL
		if imageURL == "" {
			imageURL = out.ImageURL
		}
		if imageURL == "" {
			continue
		}

		b64, err := downloadAtlasCloudImageAsBase64(imageURL)
		if err != nil {
			logger.LegacyPrintf("service.openai_gateway", "[OpenAI] AtlasCloud image download failed: %v", err)
			// Fall back to returning the raw URL. This still works for some clients
			// and is better than losing the result.
			resp.Data = append(resp.Data, openAIImagesGenerationData{URL: imageURL})
			continue
		}

		resp.Data = append(resp.Data, openAIImagesGenerationData{B64JSON: b64})
	}

	return resp
}

const atlasCloudMaxImageDownloadBytes = 20 << 20 // 20MB

// downloadAtlasCloudImageAsBase64 downloads an image from the AtlasCloud CDN
// and returns it as a base64 encoded string.
func downloadAtlasCloudImageAsBase64(imageURL string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, imageURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	client := &http.Client{
		Timeout: 60 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("download image returned status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, atlasCloudMaxImageDownloadBytes))
	if err != nil {
		return "", err
	}
	if len(data) == 0 {
		return "", fmt.Errorf("empty image response")
	}

	return base64.StdEncoding.EncodeToString(data), nil
}

func collectAtlasCloudImageOutputs(task *atlasCloudImageTask) []atlasCloudImageOutput {
	if task == nil {
		return nil
	}
	var out []atlasCloudImageOutput
	out = append(out, parseAtlasCloudImageOutputs(task.Outputs)...)
	if task.Output != nil {
		out = append(out, *task.Output)
	}
	return out
}

// parseAtlasCloudImageOutputs handles multiple possible shapes of the AtlasCloud
// `outputs` field: an array of objects, an array of URL strings, or omitted.
func parseAtlasCloudImageOutputs(raw json.RawMessage) []atlasCloudImageOutput {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}

	// Try object array first.
	var objOutputs []atlasCloudImageOutput
	if err := json.Unmarshal(raw, &objOutputs); err == nil {
		return objOutputs
	}

	// Fall back to string array (URL list).
	var strOutputs []string
	if err := json.Unmarshal(raw, &strOutputs); err == nil {
		out := make([]atlasCloudImageOutput, 0, len(strOutputs))
		for _, url := range strOutputs {
			if url = strings.TrimSpace(url); url != "" {
				out = append(out, atlasCloudImageOutput{URL: url})
			}
		}
		return out
	}

	return nil
}
