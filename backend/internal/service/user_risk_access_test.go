package service

import "testing"

// TestUserRiskAccessBlocked 锁定「待复核冻结」的判定口径：只有精确等于
// suspect / abnormal 两个常量才拦截，其余一律放行。未知值与脏数据必须放行，
// 否则新增风险等级或写入了非预期值时会把全站用户锁死。
func TestUserRiskAccessBlocked(t *testing.T) {
	cases := []struct {
		name      string
		user      *User
		wantBlock bool
	}{
		{name: "nil receiver is allowed", user: nil, wantBlock: false},
		{name: "empty risk level is allowed", user: &User{RiskLevel: ""}, wantBlock: false},
		{name: "none constant is allowed", user: &User{RiskLevel: AffiliateRiskLevelNone}, wantBlock: false},
		{name: "suspect is blocked", user: &User{RiskLevel: AffiliateRiskLevelSuspect}, wantBlock: true},
		{name: "abnormal is blocked", user: &User{RiskLevel: AffiliateRiskLevelAbnormal}, wantBlock: true},
		{name: "unknown value is allowed", user: &User{RiskLevel: "critical"}, wantBlock: false},
		{name: "future level is allowed", user: &User{RiskLevel: "review_v2"}, wantBlock: false},
		{name: "uppercase suspect is allowed (exact match only)", user: &User{RiskLevel: "SUSPECT"}, wantBlock: false},
		{name: "mixed case suspect is allowed", user: &User{RiskLevel: "Suspect"}, wantBlock: false},
		{name: "uppercase abnormal is allowed", user: &User{RiskLevel: "ABNORMAL"}, wantBlock: false},
		{name: "whitespace padded is allowed", user: &User{RiskLevel: " suspect "}, wantBlock: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.user.RiskAccessBlocked(); got != tc.wantBlock {
				t.Fatalf("RiskAccessBlocked() = %v, want %v", got, tc.wantBlock)
			}
		})
	}
}

// TestUserRiskAccessBlockedClientFacingTextIsEnglish 保证 403 响应里下发的
// code / message 是纯 ASCII 英语，不混入中文字符。
func TestUserRiskAccessBlockedClientFacingTextIsEnglish(t *testing.T) {
	if ErrUserRiskAccessBlocked == nil {
		t.Fatal("ErrUserRiskAccessBlocked must not be nil")
	}
	for label, text := range map[string]string{
		"code":    UserRiskAccessBlockedCode,
		"message": UserRiskAccessBlockedMessage,
	} {
		if text == "" {
			t.Fatalf("client-facing %s must not be empty", label)
		}
		for _, r := range text {
			if r > 127 {
				t.Fatalf("client-facing %s contains non-ASCII rune %q in %q", label, r, text)
			}
		}
	}
}
