//go:build unit

package service

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func correctClicks(answer *ImageClickAnswer) []ClickPoint {
	clicks := make([]ClickPoint, 0, len(answer.Targets))
	for _, target := range answer.Targets {
		clicks = append(clicks, ClickPoint{X: float64(target.X), Y: float64(target.Y)})
	}
	return clicks
}

func TestBuildImageClickChallenge_RendersAndVerifies(t *testing.T) {
	challenge, answer, err := buildImageClickChallenge(RegistrationCaptchaDifficultyMedium)
	require.NoError(t, err)
	require.NotNil(t, challenge)
	require.NotNil(t, answer)

	require.Equal(t, imageClickCanvasWidth, challenge.Width)
	require.Equal(t, imageClickCanvasHeight, challenge.Height)
	require.Equal(t, 3, challenge.ClickCount)
	require.Len(t, challenge.Prompt, 3)
	require.Len(t, answer.Targets, 3)

	// 图片必须是合法 PNG base64。
	_, err = base64.StdEncoding.DecodeString(challenge.Image)
	require.NoError(t, err)

	// 提示只含"颜色+形状"，且取值都在已知集合内。
	for _, hint := range challenge.Prompt {
		require.Contains(t, captchaShapeNames, hint.Shape)
		require.Contains(t, captchaColorNames, hint.Color)
	}

	// 正确顺序通过。
	require.NoError(t, verifyImageClick(answer, correctClicks(answer)))

	// 顺序颠倒失败。
	reversed := correctClicks(answer)
	if len(reversed) > 1 {
		reversed[0], reversed[1] = reversed[1], reversed[0]
		require.ErrorIs(t, verifyImageClick(answer, reversed), ErrRegistrationClickCaptchaInvalid)
	}

	// 数量不符失败。
	require.ErrorIs(t, verifyImageClick(answer, correctClicks(answer)[:1]), ErrRegistrationClickCaptchaInvalid)

	// 落在容差外失败。
	far := correctClicks(answer)
	far[0] = ClickPoint{X: far[0].X + 200, Y: far[0].Y + 200}
	require.ErrorIs(t, verifyImageClick(answer, far), ErrRegistrationClickCaptchaInvalid)
}

func TestBuildImageClickChallenge_TargetsInsideCanvas(t *testing.T) {
	_, answer, err := buildImageClickChallenge(RegistrationCaptchaDifficultyHigh)
	require.NoError(t, err)
	require.NotEmpty(t, answer.Targets)
	for _, target := range answer.Targets {
		require.GreaterOrEqual(t, target.X, 0)
		require.Less(t, target.X, answer.Width)
		require.GreaterOrEqual(t, target.Y, 0)
		require.Less(t, target.Y, answer.Height)
		require.Positive(t, target.Tolerance)
	}
}
