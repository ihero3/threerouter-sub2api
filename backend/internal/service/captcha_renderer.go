package service

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"math"
	mathrand "math/rand"
	"sort"
	"time"
)

// 验证码图形元素只画几何图形，不画任何文字：
// golang.org/x/image/font/basicfont 的 Face7x13 仅支持 ASCII，中文/emoji 无法渲染；
// 同时提示文案走 JSON 字段下发，避免把答案烘焙进图片。

// captchaShapeNames 可选形状池。提示里给出"颜色+形状"组合，图片里散布同形异色 /
// 同色异形的诱饵，使得单一特征匹配（只认形状或只认颜色）无法通过。
var captchaShapeNames = []string{"circle", "triangle", "square", "star", "hexagon"}

// captchaColorNames 可调色板名称池。
var captchaColorNames = []string{"red", "blue", "green", "yellow", "purple"}

// captchaShapeColors 调色板。图内目标使用饱和色，保证人眼易分辨。
var captchaShapeColors = map[string]color.RGBA{
	"red":    {R: 220, G: 54, B: 54, A: 255},
	"blue":   {R: 46, G: 118, B: 220, A: 255},
	"green":  {R: 44, G: 168, B: 92, A: 255},
	"yellow": {R: 226, G: 182, B: 40, A: 255},
	"purple": {R: 150, G: 78, B: 220, A: 255},
}

// captchaPoint 多边形顶点。
type captchaPoint struct {
	X float64
	Y float64
}

// cryptoSeed 用 crypto/rand 取一个 64 位种子，避免使用全局 math/rand 状态。
func cryptoSeed() int64 {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return time.Now().UnixNano()
	}
	return int64(binary.BigEndian.Uint64(buf[:]))
}

// newCaptchaRand 基于独立种子构造随机源，保证同一进程内互不干扰。
func newCaptchaRand(seed int64) *mathrand.Rand {
	return mathrand.New(mathrand.NewSource(seed))
}

// clamp8 把整数钳制到 uint8 范围。
func clamp8(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

// lerpRGBA 在两个颜色之间做线性插值。
func lerpRGBA(a, b color.RGBA, t float64) color.RGBA {
	t = math.Max(0, math.Min(1, t))
	return color.RGBA{
		R: clamp8(int(float64(a.R) + (float64(b.R)-float64(a.R))*t)),
		G: clamp8(int(float64(a.G) + (float64(b.G)-float64(a.G))*t)),
		B: clamp8(int(float64(a.B) + (float64(b.B)-float64(a.B))*t)),
		A: 255,
	}
}

// blendPixel 按 alpha 把颜色混合进目标像素（over 合成）。
func blendPixel(img *image.RGBA, x, y int, c color.RGBA) {
	b := img.Bounds()
	if x < b.Min.X || x >= b.Max.X || y < b.Min.Y || y >= b.Max.Y || c.A == 0 {
		return
	}
	if c.A == 255 {
		img.SetRGBA(x, y, c)
		return
	}
	dst := img.RGBAAt(x, y)
	a := float64(c.A) / 255
	img.SetRGBA(x, y, color.RGBA{
		R: uint8(float64(c.R)*a + float64(dst.R)*(1-a)),
		G: uint8(float64(c.G)*a + float64(dst.G)*(1-a)),
		B: uint8(float64(c.B)*a + float64(dst.B)*(1-a)),
		A: 255,
	})
}

// stampDisc 在 (cx, cy) 处盖一个半径为 radius 的实心圆点，用于绘制粗笔画。
func stampDisc(img *image.RGBA, cx, cy, radius float64, c color.RGBA) {
	r := int(math.Ceil(radius))
	icx, icy := int(math.Round(cx)), int(math.Round(cy))
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			if float64(dx*dx+dy*dy) > radius*radius {
				continue
			}
			blendPixel(img, icx+dx, icy+dy, c)
		}
	}
}

// renderNoiseBackground 程序化生成唯一背景：对角渐变 + 逐像素噪声 + 半透明游走笔刷。
// 每次挑战使用独立种子，图片永不重复，无需任何素材文件。
func renderNoiseBackground(w, h int, rng *mathrand.Rand) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	c1 := color.RGBA{
		R: uint8(30 + rng.Intn(60)),
		G: uint8(30 + rng.Intn(60)),
		B: uint8(40 + rng.Intn(60)),
		A: 255,
	}
	c2 := color.RGBA{
		R: uint8(120 + rng.Intn(90)),
		G: uint8(120 + rng.Intn(90)),
		B: uint8(120 + rng.Intn(90)),
		A: 255,
	}

	span := float64(w + h - 2)
	if span <= 0 {
		span = 1
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, lerpRGBA(c1, c2, float64(x+y)/span))
		}
	}

	// 逐像素噪声，破坏"干净背景"以便干扰基于阈值的分割。
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			p := img.RGBAAt(x, y)
			n := rng.Intn(17) - 8
			img.SetRGBA(x, y, color.RGBA{
				R: clamp8(int(p.R) + n),
				G: clamp8(int(p.G) + n),
				B: clamp8(int(p.B) + n),
				A: 255,
			})
		}
	}

	// 半透明游走笔刷：既增加纹理，也会裁切部分图标，破坏干净图形轮廓。
	for i := 0; i < 5; i++ {
		a := uint8(30 + rng.Intn(50))
		strokeColor := color.RGBA{
			R: uint8(rng.Intn(256)),
			G: uint8(rng.Intn(256)),
			B: uint8(rng.Intn(256)),
			A: a,
		}
		drawWanderingStroke(img, rng, strokeColor)
	}
	return img
}

// drawWanderingStroke 用一串随机折线的圆点印章画出一条游走粗线。
func drawWanderingStroke(img *image.RGBA, rng *mathrand.Rand, c color.RGBA) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	x := float64(rng.Intn(maxInt(w, 1)))
	y := float64(rng.Intn(maxInt(h, 1)))
	width := 2 + rng.Float64()*4
	for seg := 0; seg < 5; seg++ {
		nx := x + (rng.Float64()-0.5)*float64(w)*0.6
		ny := y + (rng.Float64()-0.5)*float64(h)*0.6
		const steps = 6
		for s := 0; s <= steps; s++ {
			t := float64(s) / float64(steps)
			stampDisc(img, x+(nx-x)*t, y+(ny-y)*t, width, c)
		}
		x, y = nx, ny
	}
}

// fillPolygon 用扫描线 + even-odd 规则在一个不透明画布上填充多边形。
func fillPolygon(img *image.RGBA, pts []captchaPoint, c color.RGBA) {
	if len(pts) < 3 {
		return
	}
	minY, maxY := pts[0].Y, pts[0].Y
	for _, p := range pts {
		minY = math.Min(minY, p.Y)
		maxY = math.Max(maxY, p.Y)
	}
	for y := int(math.Floor(minY)); y <= int(math.Ceil(maxY)); y++ {
		yc := float64(y) + 0.5
		var xs []float64
		for i := 0; i < len(pts); i++ {
			p1 := pts[i]
			p2 := pts[(i+1)%len(pts)]
			if (p1.Y <= yc && p2.Y > yc) || (p2.Y <= yc && p1.Y > yc) {
				t := (yc - p1.Y) / (p2.Y - p1.Y)
				xs = append(xs, p1.X+t*(p2.X-p1.X))
			}
		}
		if len(xs) < 2 {
			continue
		}
		sort.Float64s(xs)
		for i := 0; i+1 < len(xs); i += 2 {
			for x := int(math.Ceil(xs[i] - 0.5)); x <= int(xs[i+1]-0.5); x++ {
				if x < img.Bounds().Min.X || x >= img.Bounds().Max.X {
					continue
				}
				img.SetRGBA(x, y, c)
			}
		}
	}
}

// regularPolygon 生成正 n 边形顶点（角度以弧度计，起点由 startAngle 控制）。
func regularPolygon(cx, cy, r float64, n int, startAngle float64) []captchaPoint {
	pts := make([]captchaPoint, 0, n)
	for i := 0; i < n; i++ {
		ang := startAngle + 2*math.Pi*float64(i)/float64(n)
		pts = append(pts, captchaPoint{X: cx + r*math.Cos(ang), Y: cy + r*math.Sin(ang)})
	}
	return pts
}

// drawShapeFlat 在不透明画布上以 (cx, cy) 为中心、half 为半尺寸绘制指定形状（不旋转）。
func drawShapeFlat(img *image.RGBA, shape string, c color.RGBA, cx, cy, half float64) {
	icx, icy := int(math.Round(cx)), int(math.Round(cy))
	switch shape {
	case "circle":
		r := int(math.Ceil(half))
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if float64(dx*dx+dy*dy) <= half*half {
					img.SetRGBA(icx+dx, icy+dy, c)
				}
			}
		}
	case "square":
		s := int(math.Round(half))
		for dy := -s; dy <= s; dy++ {
			for dx := -s; dx <= s; dx++ {
				img.SetRGBA(icx+dx, icy+dy, c)
			}
		}
	case "triangle":
		fillPolygon(img, regularPolygon(float64(icx), float64(icy), half, 3, -math.Pi/2), c)
	case "hexagon":
		fillPolygon(img, regularPolygon(float64(icx), float64(icy), half, 6, -math.Pi/2), c)
	case "star":
		pts := make([]captchaPoint, 0, 10)
		for i := 0; i < 10; i++ {
			r := half
			if i%2 == 1 {
				r = half * 0.5
			}
			ang := -math.Pi/2 + math.Pi*float64(i)/5
			pts = append(pts, captchaPoint{X: cx + r*math.Cos(ang), Y: cy + r*math.Sin(ang)})
		}
		fillPolygon(img, pts, c)
	default:
		fillPolygon(img, regularPolygon(float64(icx), float64(icy), half, 5, -math.Pi/2), c)
	}
}

// drawShape 把形状先画在临时画布上，再按 rot 逆映射旋转合成到目标图片。
// 这里的旋转 + 缩放 + 描边是为了让图标无法被"干净轮廓"直接分割。
func drawShape(img *image.RGBA, shape string, c color.RGBA, cx, cy, half, rot float64) {
	r := int(math.Ceil(half)) + 2
	if r <= 0 {
		return
	}
	tmp := image.NewRGBA(image.Rect(0, 0, 2*r, 2*r))
	drawShapeFlat(tmp, shape, c, float64(r), float64(r), half)

	cosA, sinA := math.Cos(rot), math.Sin(rot)
	icx, icy := int(math.Round(cx)), int(math.Round(cy))
	for ty := 0; ty < 2*r; ty++ {
		for tx := 0; tx < 2*r; tx++ {
			px := float64(tx - r)
			py := float64(ty - r)
			sxi := int(math.Round(px*cosA+py*sinA)) + r
			syi := int(math.Round(-px*sinA+py*cosA)) + r
			if sxi < 0 || sxi >= 2*r || syi < 0 || syi >= 2*r {
				continue
			}
			src := tmp.RGBAAt(sxi, syi)
			if src.A == 0 {
				continue
			}
			blendPixel(img, icx+tx-r, icy+ty-r, src)
		}
	}
}

// encodePNGBase64 把 RGBA 画布编码为 base64 PNG（不含 data URL 前缀）。
func encodePNGBase64(img *image.RGBA) (string, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// ---------- 滑块拼图 ----------

// sliderPieceMask 判定拼图块形状：以方形为主体，右边缘中部带一个圆形凸起，
// 上边缘中部挖一个圆形缺口，形成可辨识的拼图轮廓。背景挖洞与拼图块共用该掩码，
// 保证两者形状完全一致、能严丝合缝对齐。
func sliderPieceMask(dx, dy, size int) bool {
	half := size / 2
	// 主体方形（留 1px 圆角由描边遮掩）
	inSquare := dx >= 0 && dx < size && dy >= 0 && dy < size
	// 右侧凸起
	bumpR := float64(size) / 5
	bdx, bdy := float64(dx-size), float64(dy-half)
	inBump := bdx*bdx+bdy*bdy <= bumpR*bumpR
	// 上方缺口（从主体中减去）
	notchR := float64(size) / 5
	ndx, ndy := float64(dx-half), float64(dy)
	inNotch := ndx*ndx+ndy*ndy <= notchR*notchR
	if inNotch {
		return false
	}
	return inSquare || inBump
}

// carveSliderPuzzle 从背景 src 的 (x, y, size) 处抠出拼图块，返回独立透明画布，
// 同时在 src 上把该区域挖成可见的暗色缺口（用户拖动的拼图块与之对齐即通过）。
func carveSliderPuzzle(src *image.RGBA, x, y, size int) *image.RGBA {
	piece := image.NewRGBA(image.Rect(0, 0, size+bumpSpace(size), size))
	hole := color.RGBA{R: 0, G: 0, B: 0, A: 110}
	outline := color.RGBA{R: 255, G: 255, B: 255, A: 170}

	b := src.Bounds()
	for dy := 0; dy < size; dy++ {
		for dx := 0; dx < size+bumpSpace(size); dx++ {
			if !sliderPieceMask(dx, dy, size) {
				continue
			}
			sx, sy := x+dx, y+dy
			if sx < b.Min.X || sx >= b.Max.X || sy < b.Min.Y || sy >= b.Max.Y {
				continue
			}
			piece.SetRGBA(dx, dy, color.RGBA{
				R: src.RGBAAt(sx, sy).R,
				G: src.RGBAAt(sx, sy).G,
				B: src.RGBAAt(sx, sy).B,
				A: 255,
			})
		}
	}

	// 描边：掩码内、四邻有掩码外的像素即为边界。
	for dy := 0; dy < size; dy++ {
		for dx := 0; dx < size+bumpSpace(size); dx++ {
			if !sliderPieceMask(dx, dy, size) {
				continue
			}
			if sliderPieceMask(dx-1, dy, size) && sliderPieceMask(dx+1, dy, size) &&
				sliderPieceMask(dx, dy-1, size) && sliderPieceMask(dx, dy+1, size) {
				continue
			}
			piece.SetRGBA(dx, dy, outline)
			blendPixel(src, x+dx, y+dy, outline)
		}
	}

	// 在背景上把整块区域压暗，形成可见缺口。
	for dy := 0; dy < size; dy++ {
		for dx := 0; dx < size+bumpSpace(size); dx++ {
			if !sliderPieceMask(dx, dy, size) {
				continue
			}
			blendPixel(src, x+dx, y+dy, hole)
		}
	}
	return piece
}

// bumpSpace 右侧凸起需要在画布上额外预留的宽度。
func bumpSpace(size int) int {
	return size/5 + 2
}
