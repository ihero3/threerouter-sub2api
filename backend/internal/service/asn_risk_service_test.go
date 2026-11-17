package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// 以下为 bare 层测试（无 //go:build unit 标签），刻意不依赖真实 GeoLite2-ASN.mmdb 文件：
// 判定逻辑通过纯函数 MatchDatacenterASN 覆盖，查询链路通过 reader 为 nil / 可替换 lookupFn 覆盖。

func TestMatchDatacenterASN_ASNNumber(t *testing.T) {
	// ASN 号精确命中
	require.True(t, MatchDatacenterASN(16509, "AMAZON-AES", "16509,14061"))
	require.True(t, MatchDatacenterASN(14061, "DIGITALOCEAN-ASN", "16509,14061"))
	// 可选 AS 前缀、大小写、清单项前后空格
	require.True(t, MatchDatacenterASN(16509, "", " AS16509 , as14061 "))
	// 未知 ASN 一律放行
	require.False(t, MatchDatacenterASN(64500, "SOME-RESIDENTIAL-ISP", "16509,14061"))
}

func TestMatchDatacenterASN_OrgFragment(t *testing.T) {
	// 组织名片段命中，大小写不敏感
	require.True(t, MatchDatacenterASN(99999, "AMAZON-AES", "amazon"))
	require.True(t, MatchDatacenterASN(99999, "Google Cloud LLC", "GOOGLE"))
	require.True(t, MatchDatacenterASN(99999, "MICROSOFT-CORP-MSN-AS-BLOCK", "microsoft"))
	// 清单项带空格也能命中
	require.True(t, MatchDatacenterASN(99999, "  DIGITALOCEAN-ASN  ", " digitalocean "))
	// 不相关组织放行
	require.False(t, MatchDatacenterASN(99999, "CHINANET-BACKBONE", "amazon,digitalocean"))
}

func TestMatchDatacenterASN_EmptyAndMalformedList(t *testing.T) {
	// 空清单 / 全分隔符清单：一律未命中（管理员清空清单即关闭判定）
	require.False(t, MatchDatacenterASN(16509, "AMAZON-AES", ""))
	require.False(t, MatchDatacenterASN(16509, "AMAZON-AES", " , , ,, "))
	// 组织名为空时片段不命中，但 ASN 号仍可精确命中
	require.True(t, MatchDatacenterASN(16509, "", "16509"))
	require.False(t, MatchDatacenterASN(0, "", "amazon"))
}

func TestMatchDatacenterASN_DefaultListCoversMajorClouds(t *testing.T) {
	cases := []struct {
		asn uint
		org string
	}{
		{16509, "AMAZON-AES"},
		{15169, "GOOGLE"},
		{8075, "MICROSOFT-CORP-MSN-AS-BLOCK"},
		{14061, "DIGITALOCEAN-ASN"},
		{20473, "AS-CHOOPA"}, // Vultr 归属 Choopa
		{24940, "HETZNER-AS"},
		{16276, "OVH"},
		{31898, "ORACLE-BMC-31898"},
		{37963, "ALIBABA-CN-NET"},
	}
	for _, tc := range cases {
		require.True(t, MatchDatacenterASN(tc.asn, tc.org, DefaultRegistrationDatacenterASNs), "asn=%d org=%s", tc.asn, tc.org)
	}
	// 普通住宅 / 运营商 ISP 不应被默认清单误判
	require.False(t, MatchDatacenterASN(4837, "CHINA169-BACKBONE", DefaultRegistrationDatacenterASNs))
}

func TestASNRiskService_LookupFailOpen(t *testing.T) {
	svc := &ASNRiskService{} // reader 为 nil
	for _, ip := range []string{
		"", "not-an-ip",
		"8.8.8.8", // 公网 IP，但 reader 不可用 → 未命中
		"192.168.1.1", "10.0.0.1", "172.16.0.1", // 私有地址
		"127.0.0.1", "::1", // 回环
		"169.254.1.1", // 链路本地单播
		"224.0.0.1", // 链路本地多播
	} {
		_, ok := svc.Lookup(ip)
		require.False(t, ok, "ip=%q should fail open", ip)
	}
}

func TestASNRiskService_NilReceiverFailOpen(t *testing.T) {
	var svc *ASNRiskService
	require.False(t, svc.Available())
	_, ok := svc.Lookup("8.8.8.8")
	require.False(t, ok)
	require.False(t, svc.IsDatacenterIP(context.Background(), "8.8.8.8", "16509"))
	require.NoError(t, svc.Close())
}

func TestASNRiskService_IsDatacenterIPFailOpenWhenReaderNil(t *testing.T) {
	svc := &ASNRiskService{}
	// 即使清单能命中 AWS ASN，离线库不可用也一律放行（绝不阻断注册）
	require.False(t, svc.IsDatacenterIP(context.Background(), "8.8.8.8", "16509,amazon"))
	require.False(t, svc.Available())
	require.NoError(t, svc.Close())
}

func TestASNRiskService_IsDatacenterIPWithLookupFn(t *testing.T) {
	svc := &ASNRiskService{
		lookupFn: func(ip string) (ASNRecord, bool) {
			return ASNRecord{AutonomousSystemNumber: 16509, AutonomousSystemOrganization: "AMAZON-AES"}, true
		},
	}
	require.True(t, svc.IsDatacenterIP(context.Background(), "8.8.8.8", "16509"))
	require.True(t, svc.IsDatacenterIP(context.Background(), "8.8.8.8", "amazon"))
	require.False(t, svc.IsDatacenterIP(context.Background(), "8.8.8.8", "digitalocean"))
	// 非法 / 私有 IP 在查询前被拦截，一律未命中
	require.False(t, svc.IsDatacenterIP(context.Background(), "10.0.0.1", "16509"))
	require.False(t, svc.IsDatacenterIP(context.Background(), "not-an-ip", "16509"))
	// 查询本身失败 → 未命中
	svc.lookupFn = func(ip string) (ASNRecord, bool) { return ASNRecord{}, false }
	require.False(t, svc.IsDatacenterIP(context.Background(), "8.8.8.8", "16509"))
}

func TestNewASNRiskService_FailOpenWithoutDatabase(t *testing.T) {
	// cfg 为 nil / 路径为空 / 文件不存在：服务始终非 nil，加载失败只记日志不 panic、不阻断启动。
	require.NotNil(t, NewASNRiskService(nil))
	require.NotNil(t, NewASNRiskService(&config.Config{}))

	cfg := &config.Config{}
	cfg.Compliance.GeoIP.ASNDatabasePath = "/definitely/not/existing/GeoLite2-ASN.mmdb"
	svc := NewASNRiskService(cfg)
	require.NotNil(t, svc)
	require.False(t, svc.Available())
	require.False(t, svc.IsDatacenterIP(context.Background(), "8.8.8.8", "16509"))
	require.NoError(t, svc.Close())
}
