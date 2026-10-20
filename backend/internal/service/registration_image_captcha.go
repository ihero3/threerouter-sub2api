package service

import (
	"image/color"
	mathrand "math/rand"
)

// 图片点选画布尺寸。固定尺寸让前端可以直接按像素回传坐标。
const (
	imageClickCanvasWidth  = 320
	imageClickCanvasHeight = 200
)

// ClickPoint 客户端回传的一次点击（图片像素坐标）。
type ClickPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// ImageClickTarget 服务端保存的目标答案（只存 Redis，绝不下发）。
type ImageClickTarget struct {
	X         int `json:"x"`
	Y         int `json:"y"`
	Tolerance int `json:"tolerance"`
}

// ImageClickAnswer 图片点选的答案载荷。
type ImageClickAnswer struct {
	Width   int                `json:"width"`
	Height  int                `json:"height"`
	Targets []ImageClickTarget `json:"targets"`
}

// imageClickParams 由难度决定的参数。
type imageClickParams struct {
	clickCount int
	tolerance  int
	totalIcons int
	minHalf    int
	maxHalf    int
}

func imageClickParamsFor(difficulty string) imageClickParams {
	switch difficulty {
	case RegistrationCaptchaDifficultyLow:
		return imageClickParams{clickCount: 2, tolerance: 24, totalIcons: 6, minHalf: 18, maxHalf: 26}
	case RegistrationCaptchaDifficultyHigh:
		return imageClickParams{clickCount: 4, tolerance: 12, totalIcons: 10, minHalf: 16, maxHalf: 22}
	default:
		return imageClickParams{clickCount: 3, tolerance: 18, totalIcons: 8, minHalf: 17, maxHalf: 24}
	}
}

// iconSpec 一个图标（"颜色+形状"组合在整张图内唯一）。
type iconSpec struct {
	Shape string
	Color string
}

// buildIconSpecs 生成图标集合：先定下目标组合，再为每个目标补"同形异色"与
// "同色异形"的诱饵，最后用随机组合补足总数。所有组合互不重复，因此提示中的
// "颜色+形状"能唯一指向一个图标；只认形状或只认颜色的脚本必然选错。
func buildIconSpecs(rng *mathrand.Rand, p imageClickParams) (targets []iconSpec, icons []iconSpec) {
	used := map[iconSpec]bool{}
	pick := func(shape, col string) (iconSpec, bool) {
		s := iconSpec{Shape: shape, Color: col}
		if used[s] {
			return iconSpec{}, false
		}
		used[s] = true
		return s, true
	}

	shuffledShapes := append([]string{}, captchaShapeNames...)
	shuffledColors := append([]string{}, captchaColorNames...)
	rng.Shuffle(len(shuffledShapes), func(i, j int) { shuffledShapes[i], shuffledShapes[j] = shuffledShapes[j], shuffledShapes[i] })
	rng.Shuffle(len(shuffledColors), func(i, j int) { shuffledColors[i], shuffledColors[j] = shuffledColors[j], shuffledColors[i] })

	for _, shape := range shuffledShapes {
		for _, col := range shuffledColors {
			if len(targets) >= p.clickCount {
				break
			}
			if s, ok := pick(shape, col); ok {
				targets = append(targets, s)
			}
		}
		if len(targets) >= p.clickCount {
			break
		}
	}

	decoys := make([]iconSpec, 0, p.totalIcons)
	addDecoy := func(s iconSpec) {
		if len(targets)+len(decoys) < p.totalIcons {
			decoys = append(decoys, s)
		}
	}
	for _, t := range targets {
		for _, col := range shuffledColors {
			if col == t.Color {
				continue
			}
			if s, ok := pick(t.Shape, col); ok {
				addDecoy(s)
				break
			}
		}
		for _, shape := range shuffledShapes {
			if shape == t.Shape {
				continue
			}
			if s, ok := pick(shape, t.Color); ok {
				addDecoy(s)
				break
			}
		}
	}
	for _, shape := range shuffledShapes {
		if len(targets)+len(decoys) >= p.totalIcons {
			break
		}
		for _, col := range shuffledColors {
			if len(targets)+len(decoys) >= p.totalIcons {
				break
			}
			if s, ok := pick(shape, col); ok {
				decoys = append(decoys, s)
			}
		}
	}

	icons = append(append([]iconSpec{}, decoys...), targets...)
	rng.Shuffle(len(icons), func(i, j int) { icons[i], icons[j] = icons[j], icons[i] })
	return targets, icons
}

// buildImageClickChallenge 服务端渲染图片并生成答案。
// 返回给客户端的是图片 + "颜色+形状"提示，坐标答案只进 Redis。
func buildImageClickChallenge(difficulty string) (*ImageClickChallenge, *ImageClickAnswer, error) {
	p := imageClickParamsFor(difficulty)
	w, h := imageClickCanvasWidth, imageClickCanvasHeight
	rng := newCaptchaRand(cryptoSeed())

	img := renderNoiseBackground(w, h, rng)
	targets, icons := buildIconSpecs(rng, p)

	type placed struct {
		iconSpec
		cx, cy, half int
	}
	placedIcons := make([]placed, 0, len(icons))
	for _, spec := range icons {
		half := p.minHalf + rng.Intn(p.maxHalf-p.minHalf+1)
		cx, cy := 0, 0
		ok := false
		for attempt := 0; attempt < 240; attempt++ {
			gap := 8
			if attempt > 120 {
				gap = 2
			}
			cx = half + rng.Intn(maxInt(w-2*half, 1))
			cy = half + rng.Intn(maxInt(h-2*half, 1))
			ok = true
			for _, other := range placedIcons {
				need := half + other.half + gap
				dx, dy := cx-other.cx, cy-other.cy
				if dx*dx+dy*dy < need*need {
					ok = false
					break
				}
			}
			if ok {
				break
			}
		}
		if !ok {
			continue
		}
		placedIcons = append(placedIcons, placed{iconSpec: spec, cx: cx, cy: cy, half: half})
	}
	if len(placedIcons) < p.clickCount {
		return nil, nil, ErrRegistrationClickCaptchaInvalid
	}

	// 先用半透明深色描边，再画实色图形：提高对比度，也便于人眼在噪声背景上分辨。
	for _, it := range placedIcons {
		rot := rng.Float64() - 0.5
		outline := color.RGBA{R: 30, G: 30, B: 30, A: 150}
		drawShape(img, it.Shape, outline, float64(it.cx), float64(it.cy), float64(it.half)+2, rot)
		drawShape(img, it.Shape, captchaShapeColors[it.Color], float64(it.cx), float64(it.cy), float64(it.half), rot)
	}
	// 再叠加 2~3 条半透明游走笔刷压住部分图形，破坏"干净轮廓"，干扰轮廓分割。
	for i := 0; i < 2+rng.Intn(2); i++ {
		drawWanderingStroke(img, rng, color.RGBA{
			R: uint8(rng.Intn(256)),
			G: uint8(rng.Intn(256)),
			B: uint8(rng.Intn(256)),
			A: 70,
		})
	}

	targetSet := map[iconSpec]bool{}
	for _, t := range targets {
		targetSet[t] = true
	}
	type targetEntry struct {
		target ImageClickTarget
		spec   iconSpec
	}
	entries := make([]targetEntry, 0, p.clickCount)
	for _, it := range placedIcons {
		spec := iconSpec{Shape: it.Shape, Color: it.Color}
		if !targetSet[spec] {
			continue
		}
		entries = append(entries, targetEntry{
			target: ImageClickTarget{X: it.cx, Y: it.cy, Tolerance: p.tolerance},
			spec:   spec,
		})
	}
	if len(entries) < p.clickCount {
		return nil, nil, ErrRegistrationClickCaptchaInvalid
	}
	// 打乱目标顺序，形成"依次点击"的顺序要求（图片里不体现顺序，只在提示里给出）。
	rng.Shuffle(len(entries), func(i, j int) { entries[i], entries[j] = entries[j], entries[i] })

	answer := &ImageClickAnswer{Width: w, Height: h, Targets: make([]ImageClickTarget, 0, len(entries))}
	prompt := make([]TargetHint, 0, len(entries))
	for _, e := range entries {
		answer.Targets = append(answer.Targets, e.target)
		prompt = append(prompt, TargetHint{Shape: e.spec.Shape, Color: e.spec.Color})
	}

	image64, err := encodePNGBase64(img)
	if err != nil {
		return nil, nil, err
	}
	challenge := &ImageClickChallenge{
		Image:      image64,
		Width:      w,
		Height:     h,
		Prompt:     prompt,
		ClickCount: p.clickCount,
	}
	return challenge, answer, nil
}

// verifyImageClick 按顺序校验点击坐标：每个点击必须落在对应目标的容差圆内。
func verifyImageClick(answer *ImageClickAnswer, clicks []ClickPoint) error {
	if answer == nil || len(answer.Targets) == 0 {
		return ErrRegistrationClickCaptchaInvalid
	}
	if len(clicks) != len(answer.Targets) {
		return ErrRegistrationClickCaptchaInvalid
	}
	for i, target := range answer.Targets {
		dx := clicks[i].X - float64(target.X)
		dy := clicks[i].Y - float64(target.Y)
		tol := float64(target.Tolerance)
		if dx*dx+dy*dy > tol*tol {
			return ErrRegistrationClickCaptchaInvalid
		}
	}
	return nil
}
