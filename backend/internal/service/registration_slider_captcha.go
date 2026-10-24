package service

import "math"

// 滑块画布尺寸。
const (
	sliderCanvasWidth  = 320
	sliderCanvasHeight = 160
)

// SliderAnswer 服务端保存的滑块答案（只存 Redis，绝不把 TargetX 下发）。
type SliderAnswer struct {
	Width     int `json:"width"`
	Height    int `json:"height"`
	TargetX   int `json:"target_x"`
	PieceY    int `json:"piece_y"`
	PieceSize int `json:"piece_size"`
	Tolerance int `json:"tolerance"`
}

// sliderParams 由难度决定的滑块参数。
type sliderParams struct {
	pieceSize  int
	tolerance  int
	trajectory TrajectoryParams
}

func sliderParamsFor(difficulty string) sliderParams {
	switch difficulty {
	case RegistrationCaptchaDifficultyLow:
		return sliderParams{
			pieceSize:  48,
			tolerance:  12,
			trajectory: TrajectoryParams{MinSamples: 8, MinMs: 150, MaxMs: 10000, PassScore: 0.45},
		}
	case RegistrationCaptchaDifficultyHigh:
		return sliderParams{
			pieceSize:  32,
			tolerance:  5,
			trajectory: TrajectoryParams{MinSamples: 16, MinMs: 150, MaxMs: 8000, PassScore: 0.65},
		}
	default:
		return sliderParams{
			pieceSize:  40,
			tolerance:  8,
			trajectory: TrajectoryParams{MinSamples: 12, MinMs: 150, MaxMs: 10000, PassScore: 0.55},
		}
	}
}

// buildSliderChallenge 服务端渲染带缺口背景与拼图块。
// 缺口位置（TargetX/PieceY）只写入 Redis 答案，响应里只有两张图片与拼图块纵向位置。
func buildSliderChallenge(difficulty string) (*SliderChallenge, *SliderAnswer, error) {
	p := sliderParamsFor(difficulty)
	w, h := sliderCanvasWidth, sliderCanvasHeight
	rng := newCaptchaRand(cryptoSeed())
	img := renderNoiseBackground(w, h, rng)

	size := p.pieceSize
	extra := bumpSpace(size)
	const margin = 10

	// 拼图块（含右侧凸起）必须完整落在画布内。
	minX := size + margin
	maxX := w - size - extra - margin
	if maxX <= minX {
		maxX = minX + 1
	}
	targetX := minX + rng.Intn(maxInt(maxX-minX, 1))
	targetY := margin + rng.Intn(maxInt(h-size-2*margin, 1))

	piece := carveSliderPuzzle(img, targetX, targetY, size)

	bg64, err := encodePNGBase64(img)
	if err != nil {
		return nil, nil, err
	}
	piece64, err := encodePNGBase64(piece)
	if err != nil {
		return nil, nil, err
	}

	challenge := &SliderChallenge{
		BgImage:    bg64,
		PieceImage: piece64,
		Width:      w,
		Height:     h,
		PieceY:     targetY,
		PieceSize:  size,
	}
	answer := &SliderAnswer{
		Width:     w,
		Height:    h,
		TargetX:   targetX,
		PieceY:    targetY,
		PieceSize: size,
		Tolerance: p.tolerance,
	}
	return challenge, answer, nil
}

// verifySlider 同时校验落点容差与轨迹自然性，两者都通过才判成功。
func verifySlider(answer *SliderAnswer, dropX float64, trajectory []TrajectoryPoint, p TrajectoryParams) error {
	if answer == nil {
		return ErrRegistrationClickCaptchaInvalid
	}
	if math.Abs(dropX-float64(answer.TargetX)) > float64(answer.Tolerance) {
		return ErrRegistrationClickCaptchaInvalid
	}
	if _, ok, _ := ScoreTrajectory(trajectory, p); !ok {
		return ErrRegistrationClickCaptchaInvalid
	}
	return nil
}
