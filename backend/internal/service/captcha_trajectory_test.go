//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// naturalTrajectory 拟人拖拽轨迹 fixture：速度有起伏、含一次迟滞停顿、Y 轴抖动、
// 末端收尾位移小、时间粒度非固定帧率。用于验证"真人轨迹判正"。
func naturalTrajectory() []TrajectoryPoint {
	xs := []float64{0, 3, 10, 22, 40, 60, 85, 110, 140, 170, 205, 240, 275, 300, 318, 330}
	ts := []int64{0, 30, 70, 110, 150, 280, 340, 410, 470, 540, 610, 680, 750, 810, 870, 920}
	ys := []float64{0, 1, -1, 2, -2, 1, -1, 0, 2, -1, 1, -2, 0, 1, -1, 0}
	points := make([]TrajectoryPoint, 0, len(xs))
	for i := range xs {
		points = append(points, TrajectoryPoint{X: xs[i], Y: ys[i], T: ts[i]})
	}
	return points
}

// robotTrajectory 匀速等步长、固定 16ms 帧率、Y 恒为 0 的合成轨迹。用于验证"机器人判负"。
func robotTrajectory() []TrajectoryPoint {
	points := make([]TrajectoryPoint, 0, 16)
	for i := 0; i < 16; i++ {
		points = append(points, TrajectoryPoint{X: float64(i * 10), Y: 0, T: int64(i * 16)})
	}
	return points
}

func mediumTrajectoryParams() TrajectoryParams {
	return TrajectoryParams{MinSamples: 12, MinMs: 150, MaxMs: 10000, PassScore: 0.55}
}

func TestScoreTrajectory_HumanlikePasses(t *testing.T) {
	score, ok, reason := ScoreTrajectory(naturalTrajectory(), mediumTrajectoryParams())
	require.True(t, ok, "expected pass, got reason: %s", reason)
	require.GreaterOrEqual(t, score, 0.55)
}

func TestScoreTrajectory_UniformRobotFails(t *testing.T) {
	_, ok, reason := ScoreTrajectory(robotTrajectory(), mediumTrajectoryParams())
	require.False(t, ok, "uniform robot trajectory should fail")
	require.NotEmpty(t, reason)
}

func TestScoreTrajectory_TooFewSamples(t *testing.T) {
	points := naturalTrajectory()[:4]
	_, ok, _ := ScoreTrajectory(points, mediumTrajectoryParams())
	require.False(t, ok)
}

func TestScoreTrajectory_TooFast(t *testing.T) {
	points := naturalTrajectory()
	for i := range points {
		points[i].T = int64(i * 5) // 总时长 75ms < MinMs
	}
	_, ok, _ := ScoreTrajectory(points, mediumTrajectoryParams())
	require.False(t, ok)
}

func TestScoreTrajectory_NonMonotonicTimestamps(t *testing.T) {
	points := naturalTrajectory()
	points[3].T = points[2].T - 5
	_, ok, _ := ScoreTrajectory(points, mediumTrajectoryParams())
	require.False(t, ok)
}

func TestScoreTrajectory_LeftwardFails(t *testing.T) {
	points := naturalTrajectory()
	for i := range points {
		points[i].X = -points[i].X // 整体向左
	}
	_, ok, _ := ScoreTrajectory(points, mediumTrajectoryParams())
	require.False(t, ok)
}
