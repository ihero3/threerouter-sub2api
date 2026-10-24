//go:build unit

package service

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildSliderChallenge_RendersAndVerifies(t *testing.T) {
	challenge, answer, err := buildSliderChallenge(RegistrationCaptchaDifficultyMedium)
	require.NoError(t, err)
	require.NotNil(t, challenge)
	require.NotNil(t, answer)

	require.Equal(t, sliderCanvasWidth, challenge.Width)
	require.Equal(t, sliderCanvasHeight, challenge.Height)
	require.NotEmpty(t, challenge.BgImage)
	require.NotEmpty(t, challenge.PieceImage)

	_, err = base64.StdEncoding.DecodeString(challenge.BgImage)
	require.NoError(t, err)
	_, err = base64.StdEncoding.DecodeString(challenge.PieceImage)
	require.NoError(t, err)

	// 缺口位置只存答案，响应体不得包含 target_x。
	raw, err := json.Marshal(challenge)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "target_x")

	params := sliderParamsFor(RegistrationCaptchaDifficultyMedium)
	// 落点正确 + 自然轨迹通过。
	require.NoError(t, verifySlider(answer, float64(answer.TargetX), naturalTrajectory(), params.trajectory))

	// 落点偏移超容差失败。
	require.ErrorIs(t, verifySlider(answer, float64(answer.TargetX+50), naturalTrajectory(), params.trajectory),
		ErrRegistrationClickCaptchaInvalid)

	// 落点正确但机器人轨迹失败。
	require.ErrorIs(t, verifySlider(answer, float64(answer.TargetX), robotTrajectory(), params.trajectory),
		ErrRegistrationClickCaptchaInvalid)
}

func TestBuildSliderChallenge_TargetWithinTrack(t *testing.T) {
	_, answer, err := buildSliderChallenge(RegistrationCaptchaDifficultyHigh)
	require.NoError(t, err)
	require.Positive(t, answer.TargetX)
	require.Less(t, answer.TargetX, answer.Width)
	require.Positive(t, answer.PieceY)
	require.Less(t, answer.PieceY, answer.Height)
}
