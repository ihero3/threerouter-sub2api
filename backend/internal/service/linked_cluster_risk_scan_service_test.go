package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// 复刻截图里的作弊团伙：4 个新号，同注册 IP 只有 3 个、同使用 IP 也只有 3 个，
// 单维度都不够现有阈值（注册关联 4 / 共享 IP 5），但图连通分量 = 4 应被一网打尽。
func TestBuildLinkedClusters_TransitiveAcrossTwoDimensions(t *testing.T) {
	nodes := []RecentRegistrationIP{
		{UserID: 6172, RegisterIP: "80.64.84.158"},
		{UserID: 6174, RegisterIP: "80.64.84.158"},
		{UserID: 6173, RegisterIP: "80.64.84.158"},
		{UserID: 6175, RegisterIP: "31.14.252.4"}, // 注册 IP 不同，但共用最后使用 IP
	}
	usedIP := map[int64]string{
		6172: "35.227.139.97",
		6174: "35.227.139.97",
		6175: "35.227.139.97",
		// 6173 没调用过，无最后使用 IP——靠注册 IP 连进来
	}

	got := buildLinkedClusters(nodes, usedIP, 3)
	require.ElementsMatch(t, []int64{6172, 6173, 6174, 6175}, got,
		"单维度都不够阈值，但跨维度连通分量=4 应全抓")
}

// 两个维度都没达到 minSize 的孤立小团伙不应命中（防止 2 人互相共享被误伤）。
func TestBuildLinkedClusters_BelowMinSizeIgnored(t *testing.T) {
	nodes := []RecentRegistrationIP{
		{UserID: 1, RegisterIP: "1.1.1.1"},
		{UserID: 2, RegisterIP: "1.1.1.1"},
		{UserID: 3, RegisterIP: "2.2.2.2"},
	}
	usedIP := map[int64]string{3: "3.3.3.3"}
	require.Empty(t, buildLinkedClusters(nodes, usedIP, 3))
}

// 空 IP 不参与建边：所有空注册 IP 不应被合并成一个巨大团伙。
func TestBuildLinkedClusters_EmptyIPsNotConnected(t *testing.T) {
	nodes := []RecentRegistrationIP{
		{UserID: 1, RegisterIP: ""},
		{UserID: 2, RegisterIP: ""},
		{UserID: 3, RegisterIP: ""},
	}
	require.Empty(t, buildLinkedClusters(nodes, map[int64]string{}, 3),
		"空 IP 不能当边，否则所有无 IP 用户被连成一伙")
}

// 两个独立团伙各自达阈值：应全部返回，且互不串连。
func TestBuildLinkedClusters_TwoSeparateClusters(t *testing.T) {
	nodes := []RecentRegistrationIP{
		{UserID: 1, RegisterIP: "10.0.0.1"},
		{UserID: 2, RegisterIP: "10.0.0.1"},
		{UserID: 3, RegisterIP: "10.0.0.1"},
		{UserID: 11, RegisterIP: "10.0.0.2"},
		{UserID: 12, RegisterIP: "10.0.0.2"},
		{UserID: 13, RegisterIP: "10.0.0.2"},
	}
	require.ElementsMatch(t, []int64{1, 2, 3, 11, 12, 13},
		buildLinkedClusters(nodes, map[int64]string{}, 3))
}

// 节点数不足 minSize 直接返回空。
func TestBuildLinkedClusters_TooFewNodes(t *testing.T) {
	nodes := []RecentRegistrationIP{
		{UserID: 1, RegisterIP: "1.1.1.1"},
		{UserID: 2, RegisterIP: "1.1.1.1"},
	}
	require.Empty(t, buildLinkedClusters(nodes, map[int64]string{}, 3))
}

// minSize<=1 视为无效配置，不判任何团伙（防御性）。
func TestBuildLinkedClusters_InvalidMinSize(t *testing.T) {
	nodes := []RecentRegistrationIP{{UserID: 1, RegisterIP: "1.1.1.1"}}
	require.Empty(t, buildLinkedClusters(nodes, map[int64]string{}, 1))
}
