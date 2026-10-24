package service

import (
	"fmt"
	"math"
)

// TrajectoryPoint 一次拖拽采样点。T 为相对拖拽起点的毫秒时间戳。
type TrajectoryPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	T int64   `json:"t"`
}

// TrajectoryParams 轨迹分析参数（由难度决定）。
type TrajectoryParams struct {
	MinSamples int
	MinMs      int64
	MaxMs      int64
	PassScore  float64
}

// 软特征权重，合计 1.0。
const (
	trajWeightMonotonic  = 0.15
	trajWeightSpeedVar   = 0.25
	trajWeightAccel      = 0.15
	trajWeightPause      = 0.15
	trajWeightYJitter    = 0.10
	trajWeightTerminal   = 0.10
	trajWeightTimeGran   = 0.10
	trajRightwardMinRati = 0.65
	trajPauseThresholdMs = 120
	trajTerminalMaxRatio = 0.35
)

// ScoreTrajectory 对滑块拖拽轨迹做自然性打分，用于拦截脚本合成轨迹。
//
// 先做硬门限（样本数、时长区间、时间戳单调、整体向右），任一不满足直接失败；
// 再对 7 项可解释特征各归一化到 [0,1] 加权求和得到 score∈[0,1]。
// 返回 ok=true 表示轨迹通过。
func ScoreTrajectory(points []TrajectoryPoint, p TrajectoryParams) (float64, bool, string) {
	if p.MinSamples < 2 {
		p.MinSamples = 2
	}
	if len(points) < p.MinSamples {
		return 0, false, fmt.Sprintf("too few samples: %d < %d", len(points), p.MinSamples)
	}

	first, last := points[0], points[len(points)-1]
	duration := last.T - first.T
	if duration < p.MinMs {
		return 0, false, fmt.Sprintf("too fast: %dms < %dms", duration, p.MinMs)
	}
	if duration > p.MaxMs {
		return 0, false, fmt.Sprintf("too slow: %dms > %dms", duration, p.MaxMs)
	}
	for i := 1; i < len(points); i++ {
		if points[i].T < points[i-1].T {
			return 0, false, "timestamps not monotonic"
		}
	}

	totalDx := last.X - first.X
	if totalDx <= 0 {
		return 0, false, "no rightward displacement"
	}

	steps := len(points) - 1
	var (
		dxs       = make([]float64, 0, steps)
		speeds    = make([]float64, 0, steps)
		dys       = make([]float64, 0, steps)
		rightStep int
		pauses    int
	)
	for i := 1; i < len(points); i++ {
		dx := points[i].X - points[i-1].X
		dy := points[i].Y - points[i-1].Y
		dt := points[i].T - points[i-1].T
		dxs = append(dxs, dx)
		dys = append(dys, dy)
		if dx > 0 {
			rightStep++
		}
		if dt > trajPauseThresholdMs {
			pauses++
		}
		if dt > 0 {
			speeds = append(speeds, dx/float64(dt))
		} else {
			speeds = append(speeds, 0)
		}
	}

	rightRatio := float64(rightStep) / float64(steps)
	if rightRatio < trajRightwardMinRati {
		return 0, false, fmt.Sprintf("not rightward enough: %.2f", rightRatio)
	}

	// ① 单调性：越接近 1 越好，0.9 封顶（允许少量回拉）。
	sMonotonic := clamp01(rightRatio / 0.9)

	// ② 速度变异系数：机器人匀速 → ≈0 重罚；理想带 0.2~1.5。
	meanSpeed := mean(speeds)
	cv := 0.0
	if meanSpeed > 0 {
		cv = std(speeds) / meanSpeed
	}
	sSpeedVar := bandScore(cv, 0.05, 0.2, 1.5, 3.0)

	// ③ 加速度符号变化次数：0 次（完全匀加速/匀速）扣分，过多（人手抖动过度）也扣分。
	var accelChanges int
	var prevSign int
	for i := 1; i < len(dxs); i++ {
		sign := signOf(dxs[i] - dxs[i-1])
		if sign != 0 {
			if prevSign != 0 && sign != prevSign {
				accelChanges++
			}
			prevSign = sign
		}
	}
	sAccel := clamp01(float64(accelChanges) / 3.0)

	// ④ 停顿数：>6 次（拖沓）扣分；0 次轻微扣分（多数真人拖拽至少有一次启动/收尾迟滞）。
	sPause := 1.0
	switch {
	case pauses == 0:
		sPause = 0.6
	case pauses > 6:
		sPause = clamp01(1 - float64(pauses-6)/6.0)
	}

	// ⑤ 纵向抖动：完全笔直（std=0）扣分。
	sYJitter := clamp01(std(dys) / 1.5)

	// ⑥ 末端吸附：最后一步不应占总位移过大比例（防"瞬移到位"）。
	lastRatio := dxs[len(dxs)-1] / totalDx
	sTerminal := 1 - clamp01((lastRatio-trajTerminalMaxRatio)/(1-trajTerminalMaxRatio))

	// ⑦ 时间粒度：ΔT 全部落在 16/17ms 说明是固定帧率的合成循环。
	all16or17 := true
	for i := 1; i < len(points); i++ {
		dt := points[i].T - points[i-1].T
		if dt != 16 && dt != 17 {
			all16or17 = false
			break
		}
	}
	sTimeGran := 1.0
	if all16or17 {
		sTimeGran = 0.4
	}

	score := sMonotonic*trajWeightMonotonic +
		sSpeedVar*trajWeightSpeedVar +
		sAccel*trajWeightAccel +
		sPause*trajWeightPause +
		sYJitter*trajWeightYJitter +
		sTerminal*trajWeightTerminal +
		sTimeGran*trajWeightTimeGran

	if score < p.PassScore {
		return score, false, fmt.Sprintf("score %.3f below threshold %.2f", score, p.PassScore)
	}
	return score, true, ""
}

func mean(vs []float64) float64 {
	if len(vs) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vs {
		sum += v
	}
	return sum / float64(len(vs))
}

func std(vs []float64) float64 {
	if len(vs) < 2 {
		return 0
	}
	m := mean(vs)
	sum := 0.0
	for _, v := range vs {
		d := v - m
		sum += d * d
	}
	return math.Sqrt(sum / float64(len(vs)-1))
}

func signOf(v float64) int {
	switch {
	case v > 1e-9:
		return 1
	case v < -1e-9:
		return -1
	default:
		return 0
	}
}

// bandScore 对落在 [goodLow, goodHigh] 的理想带返回 1，带外向两侧线性衰减到 0。
func bandScore(v, zeroLow, goodLow, goodHigh, zeroHigh float64) float64 {
	switch {
	case v <= zeroLow || v >= zeroHigh:
		return 0
	case v < goodLow:
		return clamp01((v - zeroLow) / (goodLow - zeroLow))
	case v <= goodHigh:
		return 1
	default:
		return clamp01((zeroHigh - v) / (zeroHigh - goodHigh))
	}
}
