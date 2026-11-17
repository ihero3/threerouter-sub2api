package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateRegistrationEmailRisk(t *testing.T) {
	allowed := []string{
		"john.smith@gmail.com",
		"li.wei2024@gmail.com",
		"mike_1990@qq.com",
		"support@acme-corp.com",
		"zhang.wei@163.com",
		"a.b@outlook.com",
		"user@company.example",
	}
	for _, email := range allowed {
		require.NoErrorf(t, ValidateRegistrationEmailRisk(email), "expected allowed: %s", email)
	}

	rejected := []string{
		"john+tag@gmail.com",  // 高危符号 +
		"user#123@gmail.com",  // 高危符号 #
		"user8888@qq.com",     // 短字母串 + 末尾数字
		"xxx1234@gmail.com",   // 模板化命名
		"abcnn66@163.com",     // 模板化命名
		"asdfghj@qq.com",      // 连续 6 位无元音
		"qwrtzxc@qq.com",      // 连续无元音
		"john..doe@gmail.com", // 连续分隔符
		".john888@gmail.com",  // 首部分隔符 + 模板化命名
		"bad@tormails.com",    // 异常域名
		"bad@hastana.net",     // 异常域名
		"bad@fldubai.com",     // 异常域名
		"bad@mailinator.com",  // 临时邮箱
		"bad@tempmail.com",    // 临时邮箱
		"bad@1234domain.com",  // 域名含连续 4 位以上数字
		"bad@qwrtzxcv.com",    // 域名主体无元音
		"not-an-email",        // 非法格式
	}
	for _, email := range rejected {
		require.ErrorIsf(t, ValidateRegistrationEmailRisk(email), ErrEmailRiskRejected, "expected rejected: %s", email)
	}
}

func TestTempMailMXClusterHit(t *testing.T) {
	clusters := []string{"temp-mail-pro.com", "erinn.biz"}
	cases := []struct {
		domain  string
		mxHosts []string
		want    bool
	}{
		// 邮箱域本身就是集群域。
		{"temp-mail-pro.com", nil, true},
		{"erinn.biz", nil, true},
		// MX 主机落在集群域下（含多级子域与结尾点）。
		{"foo.com", []string{"mail.temp-mail-pro.com"}, true},
		{"foo.com", []string{"mx1.erinn.biz."}, true},
		// 普通域名 + 正常 MX，不命中。
		{"gmail.com", []string{"smtp.gmail.com"}, false},
		{"acme-corp.com", []string{"aspmx.l.google.com"}, false},
		{"example.com", nil, false},
	}
	for _, tc := range cases {
		if got := tempMailMXClusterHit(tc.domain, tc.mxHosts, clusters); got != tc.want {
			t.Errorf("tempMailMXClusterHit(%q, %v) = %v, want %v", tc.domain, tc.mxHosts, got, tc.want)
		}
	}
}

func TestLookupTempMailMXCluster_InvalidEmailNoPanic(t *testing.T) {
	// 非法邮箱不应触发 DNS，直接放行且不报错。
	hit, err := lookupTempMailMXCluster(context.Background(), "not-an-email")
	require.False(t, hit)
	require.NoError(t, err)

	hit, err = lookupTempMailMXCluster(context.Background(), "foo@")
	require.False(t, hit)
	require.NoError(t, err)
}
