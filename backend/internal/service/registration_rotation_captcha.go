package service

import (
	"image"
	"image/color"
	"math"
	mathrand "math/rand"
)

// 旋转验证码画布尺寸（正方形）。
const rotationCanvasSize = 220

// RotationChallenge 旋转题面：一张被随机旋转过的圆形图片。
// 客户端需要把图片旋转回"箭头朝上"的正确方向。
type RotationChallenge struct {
	Image  string `json:"image"` // base64 PNG（无 data URL 前缀）
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// RotationAnswer 服务端保存的旋转答案（只存 Redis，绝不下发）。
type RotationAnswer struct {
	TargetAngle float64 `json:"target_angle"` // 用户应施加的旋转角度（度，CSS 顺时针为正）
	Tolerance   float64 `json:"tolerance"`    // 角度容差（度）
}

// rotationParams 由难度决定的旋转参数。
type rotationParams struct {
	tolerance  float64
	trajectory RotationTrajectoryParams
}

// RotationTrajectoryParams 旋转轨迹分析参数。
type RotationTrajectoryParams struct {
	MinSamples   int
	MinMs        int64
	MaxMs        int64
	MinAngleDeg  float64 // 总旋转角度下限（度），防止点击瞬移
}

func rotationParamsFor(difficulty string) rotationParams {
	switch difficulty {
	case RegistrationCaptchaDifficultyLow:
		return rotationParams{
			tolerance:  15,
			trajectory: RotationTrajectoryParams{MinSamples: 6, MinMs: 200, MaxMs: 15000, MinAngleDeg: 30},
		}
	case RegistrationCaptchaDifficultyHigh:
		return rotationParams{
			tolerance:  6,
			trajectory: RotationTrajectoryParams{MinSamples: 12, MinMs: 300, MaxMs: 12000, MinAngleDeg: 60},
		}
	default:
		return rotationParams{
			tolerance:  10,
			trajectory: RotationTrajectoryParams{MinSamples: 8, MinMs: 250, MaxMs: 15000, MinAngleDeg: 45},
		}
	}
}

// buildRotationChallenge 服务端渲染一张带明确"朝上"方向特征的圆形图片，
// 再随机旋转一个角度。正确答案是把图片旋回朝上所需的角度。
func buildRotationChallenge(difficulty string) (*RotationChallenge, *RotationAnswer, error) {
	p := rotationParamsFor(difficulty)
	size := rotationCanvasSize
	rng := newCaptchaRand(cryptoSeed())

	base := renderRotationBase(size, rng)

	// 随机旋转角度：避开 0°/360° 附近，保证题目不是"本来就正"。
	applied := 30 + float64(rng.Intn(301)) // 30° ~ 330°
	rotated := rotateRGBA(base, applied)
	circular := applyCircleMask(rotated, size)

	img64, err := encodePNGBase64(circular)
	if err != nil {
		return nil, nil, err
	}

	// 用户需要施加的旋转角 = -applied（抵消服务端的旋转）。
	target := math.Mod(-applied, 360)
	challenge := &RotationChallenge{Image: img64, Width: size, Height: size}
	answer := &RotationAnswer{TargetAngle: target, Tolerance: p.tolerance}
	return challenge, answer, nil
}

// renderRotationBase 绘制"朝上"基准图：噪声背景 + 一个明显朝上的粗箭头。
func renderRotationBase(size int, rng *mathrand.Rand) *image.RGBA {
	img := renderNoiseBackground(size, size, rng)
	cx := float64(size) / 2
	cy := float64(size) / 2

	headTop := cy - float64(size)*0.32
	headBottom := cy - float64(size)*0.06
	halfHead := float64(size) * 0.20
	shaftTop := headBottom
	shaftBottom := cy + float64(size)*0.30
	halfShaft := float64(size) * 0.09

	// 箭头整体轮廓（顺时针，从顶点开始）。
	arrow := []captchaPoint{
		{X: cx, Y: headTop},
		{X: cx - halfHead, Y: headBottom},
		{X: cx - halfShaft, Y: shaftTop},
		{X: cx - halfShaft, Y: shaftBottom},
		{X: cx + halfShaft, Y: shaftBottom},
		{X: cx + halfShaft, Y: shaftTop},
		{X: cx + halfHead, Y: headBottom},
	}
	outline := color.RGBA{R: 15, G: 15, B: 15, A: 220}
	fill := color.RGBA{R: 250, G: 250, B: 250, A: 255}
	fillPolygon(img, scalePolygon(arrow, 1.14), outline)
	fillPolygon(img, arrow, fill)

	// 顶端加一个小实心圆点，强化"上方"的方向感（同时也是非对称特征）。
	stampDisc(img, cx, headTop-float64(size)*0.06, float64(size)*0.035, fill)
	stampDisc(img, cx, headTop-float64(size)*0.06, float64(size)*0.022, color.RGBA{R: 220, G: 60, B: 60, A: 255})

	return img
}

// scalePolygon 把多边形绕重心缩放 factor 倍，用于绘制描边（大一圈的深色多边形垫底）。
func scalePolygon(pts []captchaPoint, factor float64) []captchaPoint {
	if len(pts) == 0 {
		return pts
	}
	var sx, sy float64
	for _, p := range pts {
		sx += p.X
		sy += p.Y
	}
	cx, cy := sx/float64(len(pts)), sy/float64(len(pts))
	out := make([]captchaPoint, len(pts))
	for i, p := range pts {
		out[i] = captchaPoint{X: cx + (p.X-cx)*factor, Y: cy + (p.Y-cy)*factor}
	}
	return out
}

// rotateRGBA 以画布中心为轴，把 src 顺时针旋转 angleDeg 度。
// 超出源图范围的目标像素保持透明，交由 applyCircleMask 填充。
func rotateRGBA(src *image.RGBA, angleDeg float64) *image.RGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	cx, cy := float64(w)/2, float64(h)/2
	rad := -angleDeg * math.Pi / 180 // 逆映射
	cosA, sinA := math.Cos(rad), math.Sin(rad)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx, dy := float64(x)-cx, float64(y)-cy
			sx := dx*cosA - dy*sinA + cx
			sy := dx*sinA + dy*cosA + cy
			sxi, syi := int(math.Round(sx)), int(math.Round(sy))
			if sxi < 0 || sxi >= w || syi < 0 || syi >= h {
				continue
			}
			dst.SetRGBA(x, y, src.RGBAAt(sxi, syi))
		}
	}
	return dst
}

// applyCircleMask 把方形图裁成圆形：圆内保留像素（旋转产生的透明角用深色补齐），
// 圆外透明，从而让画布边框不再暴露旋转方向。
func applyCircleMask(src *image.RGBA, size int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	r := float64(size) / 2
	rInner := (r - 1) * (r - 1)
	bg := color.RGBA{R: 38, G: 42, B: 52, A: 255}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x) - r + 0.5
			dy := float64(y) - r + 0.5
			if dx*dx+dy*dy > rInner {
				continue
			}
			c := src.RGBAAt(x, y)
			if c.A == 0 {
				c = bg
			}
			dst.SetRGBA(x, y, c)
		}
	}
	return dst
}

// verifyRotation 同时校验角度容差与旋转轨迹自然性。
func verifyRotation(answer *RotationAnswer, angle float64, trajectory []TrajectoryPoint, p RotationTrajectoryParams) error {
	if answer == nil {
		return ErrRegistrationClickCaptchaInvalid
	}
	if angleDiff(angle, answer.TargetAngle) > answer.Tolerance {
		return ErrRegistrationClickCaptchaInvalid
	}
	if err := scoreRotationTrajectory(trajectory, p); err != nil {
		return err
	}
	return nil
}

// angleDiff 计算两个角度在圆周上的最短差值（度，0~180）。
func angleDiff(a, b float64) float64 {
	d := math.Mod(a-b, 360)
	if d > 180 {
		d -= 360
	}
	if d < -180 {
		d += 360
	}
	if d < 0 {
		d = -d
	}
	return d
}

// scoreRotationTrajectory 对旋转拖拽轨迹做自然性校验。
// 轨迹点的 X 字段承载"当前累计旋转角（度）"，T 为相对起点毫秒。
// 与滑块的 ScoreTrajectory 不同：旋转无固定方向，只看采样数、时长、
// 总转角下限，以及角速度的非匀速特征（拦截脚本匀速合成）。
func scoreRotationTrajectory(points []TrajectoryPoint, p RotationTrajectoryParams) error {
	if p.MinSamples < 2 {
		p.MinSamples = 2
	}
	if len(points) < p.MinSamples {
		return ErrRegistrationClickCaptchaInvalid
	}
	first, last := points[0], points[len(points)-1]
	duration := last.T - first.T
	if duration < p.MinMs || duration > p.MaxMs {
		return ErrRegistrationClickCaptchaInvalid
	}
	for i := 1; i < len(points); i++ {
		if points[i].T < points[i-1].T {
			return ErrRegistrationClickCaptchaInvalid
		}
	}
	totalDelta := math.Abs(last.X - first.X)
	if totalDelta < p.MinAngleDeg {
		return ErrRegistrationClickCaptchaInvalid
	}

	// 角速度变异：完全匀速（std≈0）的合成轨迹扣分。
	speeds := make([]float64, 0, len(points)-1)
	for i := 1; i < len(points); i++ {
		dt := points[i].T - points[i-1].T
		if dt > 0 {
			speeds = append(speeds, math.Abs(points[i].X-points[i-1].X)/float64(dt))
		}
	}
	if len(speeds) >= 2 {
		m := mean(speeds)
		if m > 0 {
			cv := std(speeds) / m
			// 变异系数过低（< 0.08）视为匀速合成，拒绝。
			if cv < 0.08 {
				return ErrRegistrationClickCaptchaInvalid
			}
		}
	}
	return nil
}
