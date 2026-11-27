package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	captcha "github.com/alibabacloud-go/captcha-20230305/client"
	openapiutil "github.com/alibabacloud-go/darabonba-openapi/v2/utils"
	"github.com/alibabacloud-go/tea/dara"
	"github.com/alibabacloud-go/tea/tea"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const aliyunCaptchaTimeoutMillis = 10_000

type aliyunCaptchaVerifier struct {
	protocol      string // "HTTPS"；测试注入 "HTTP" 指向 httptest.Server
	timeoutMillis int
}

func NewAliyunCaptchaVerifier() service.AliyunCaptchaVerifier {
	return &aliyunCaptchaVerifier{
		protocol:      "HTTPS",
		timeoutMillis: aliyunCaptchaTimeoutMillis,
	}
}

// VerifyCaptcha 调用阿里云验证码 2.0 VerifyIntelligentCaptcha。
// AK/SK 是可热更的后台设置，每次调用按当前凭证新建 client。
func (v *aliyunCaptchaVerifier) VerifyCaptcha(ctx context.Context, cred service.AliyunCaptchaCredentials, captchaVerifyParam string) (*service.AliyunCaptchaVerifyResult, error) {
	client, err := captcha.NewClient(&openapiutil.Config{
		AccessKeyId:     dara.String(cred.AccessKeyID),
		AccessKeySecret: dara.String(cred.AccessKeySecret),
		Endpoint:        dara.String(cred.Endpoint),
		Protocol:        dara.String(v.protocol),
		ConnectTimeout:  dara.Int(v.timeoutMillis),
		ReadTimeout:     dara.Int(v.timeoutMillis),
	})
	if err != nil {
		return nil, fmt.Errorf("create aliyun captcha client: %w", err)
	}

	request := &captcha.VerifyIntelligentCaptchaRequest{
		CaptchaVerifyParam: dara.String(captchaVerifyParam),
		SceneId:            dara.String(cred.SceneID),
	}

	response, err := client.VerifyIntelligentCaptchaWithContext(ctx, request, &dara.RuntimeOptions{})
	if err != nil {
		return nil, normalizeAliyunCaptchaError(err)
	}

	result := &service.AliyunCaptchaVerifyResult{}
	if body := response.Body; body != nil && body.Result != nil {
		result.VerifyResult = dara.BoolValue(body.Result.VerifyResult)
		result.VerifyCode = dara.StringValue(body.Result.VerifyCode)
	}
	return result, nil
}

// normalizeAliyunCaptchaError 把 SDK 的两种错误类型归一化为 service.AliyunCaptchaAPIError，
// 其余错误（网络/超时等）原样返回。
func normalizeAliyunCaptchaError(err error) error {
	var teaErr *tea.SDKError
	if errors.As(err, &teaErr) && aliyunSDKErrorHasBusinessCode(tea.StringValue(teaErr.Code), tea.IntValue(teaErr.StatusCode), tea.StringValue(teaErr.Message)) {
		return &service.AliyunCaptchaAPIError{
			Code:    tea.StringValue(teaErr.Code),
			Message: tea.StringValue(teaErr.Message),
		}
	}
	var daraErr *dara.SDKError
	if errors.As(err, &daraErr) && aliyunSDKErrorHasBusinessCode(dara.StringValue(daraErr.Code), dara.IntValue(daraErr.StatusCode), dara.StringValue(daraErr.Message)) {
		return &service.AliyunCaptchaAPIError{
			Code:    dara.StringValue(daraErr.Code),
			Message: dara.StringValue(daraErr.Message),
		}
	}
	return err
}

func aliyunSDKErrorHasBusinessCode(code string, statusCode int, message string) bool {
	code = strings.TrimSpace(code)
	if code == "<nil>" {
		code = ""
	}
	if code == "" {
		return false
	}
	// Connection failures are wrapped by Tea as an SDKError with a 503 status and
	// a generic "code: 503" message, but no OpenAPI Code. HTTP 5xx without a real
	// service error code is transport/upstream availability, not a business verdict.
	if statusCode >= 500 && aliyunCodeIsHTTPStatus(code) && strings.Contains(message, "request id: <nil>") {
		return false
	}
	return true
}

func aliyunCodeIsHTTPStatus(code string) bool {
	if len(code) != 3 && len(code) != 4 {
		return false
	}
	for _, ch := range code {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}
