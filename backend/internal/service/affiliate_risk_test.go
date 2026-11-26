package service

import "testing"

func TestIsStuffedEmailAlias(t *testing.T) {
	cases := []struct {
		email string
		want  bool
	}{
		// 真实案例：4 个点、含单字符段、总长 25。
		{"lanhsuychsieck.i.e.c11.23@gmail.com", true},
		{"a.b.c.d.e.gmail1234567@gmail.com", true},
		{"abc.d.ghi.jkl.mno12@googlemail.com", true},
		// 阈值下界：3 个点仍应命中，防止放宽过度把规则架空。
		{"lanhsuychsieck.i.e.c11@gmail.com", true},

		// 真实姓名形态，不应误报。
		{"firstname.lastname@gmail.com", false},
		{"john.q.public@gmail.com", false},
		{"zhangsan2024@gmail.com", false},
		{"li.wei.2020@gmail.com", false},
		// 阈值上界：2 个点 + 长姓名 + 中间名缩写，放宽后不命中。
		{"alexander.q.public@gmail.com", false},

		// 非点号别名服务商，不适用本规则。
		{"lanhsuychsieck.i.e.c11.23@outlook.com", false},
		{"lanhsuychsieck.i.e.c11.23@qq.com", false},

		// 边界：无 @。
		{"abc.def.ghi.jkl.mno12", false},
	}
	for _, tc := range cases {
		if got := isStuffedEmailAlias(tc.email); got != tc.want {
			t.Errorf("isStuffedEmailAlias(%q) = %v, want %v", tc.email, got, tc.want)
		}
	}
}

func TestEvaluateAffiliateRisk_EmailAliasStuffingMarksSuspect(t *testing.T) {
	level, reasons := evaluateAffiliateRisk(AffiliateRiskSignal{
		Email: "lanhsuychsieck.i.e.c11.23@gmail.com",
	})
	if level != AffiliateRiskLevelSuspect {
		t.Fatalf("level = %q, want %q", level, AffiliateRiskLevelSuspect)
	}
	found := false
	for _, r := range reasons {
		if r == AffiliateRiskReasonEmailAliasStuffing {
			found = true
		}
	}
	if !found {
		t.Fatalf("reasons = %v, want to contain %q", reasons, AffiliateRiskReasonEmailAliasStuffing)
	}
}

func TestEvaluateAffiliateRisk_NormalEmailNotFlagged(t *testing.T) {
	level, reasons := evaluateAffiliateRisk(AffiliateRiskSignal{
		Email: "firstname.lastname@gmail.com",
	})
	if level != AffiliateRiskLevelNone {
		t.Fatalf("level = %q, want none; reasons = %v", level, reasons)
	}
}

func TestEvaluateAffiliateRisk_InviteBurstTiering(t *testing.T) {
	cases := []struct {
		name      string
		age       int
		invites   int
		wantLevel string
	}{
		{"new account under suspect", 0, 5, AffiliateRiskLevelNone},
		{"new account at suspect", 3, 6, AffiliateRiskLevelSuspect},
		{"new account at abnormal", 1, 7, AffiliateRiskLevelAbnormal},
		{"old account under suspect", 30, 5, AffiliateRiskLevelNone},
		{"old account at suspect", 20, 6, AffiliateRiskLevelSuspect},
		{"old account at abnormal", 40, 7, AffiliateRiskLevelAbnormal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			level, _ := evaluateAffiliateRisk(AffiliateRiskSignal{
				AccountAgeDays: tc.age,
				InvitesIn24h:   tc.invites,
			})
			if level != tc.wantLevel {
				t.Fatalf("age=%d invites=%d level=%q, want %q", tc.age, tc.invites, level, tc.wantLevel)
			}
		})
	}
}

func TestEvaluateAffiliateRisk_ChainDepthTiering(t *testing.T) {
	cases := []struct {
		name       string
		ancestor   int
		descendant int
		wantLevel  string
	}{
		{"depth 2 is clean", 2, 0, AffiliateRiskLevelNone},
		{"depth 3 is clean", 3, 0, AffiliateRiskLevelNone},
		{"descendant depth 3 is clean", 0, 3, AffiliateRiskLevelNone},
		{"ancestor depth 4 is suspect", 4, 0, AffiliateRiskLevelSuspect},
		{"descendant depth 4 is suspect", 0, 4, AffiliateRiskLevelSuspect},
		{"depth 5 is abnormal", 0, 5, AffiliateRiskLevelAbnormal},
		{"deepest direction wins", 2, 5, AffiliateRiskLevelAbnormal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			level, _ := evaluateAffiliateRisk(AffiliateRiskSignal{
				AncestorDepth:   tc.ancestor,
				DescendantDepth: tc.descendant,
			})
			if level != tc.wantLevel {
				t.Fatalf("depth(%d,%d) level=%q, want %q", tc.ancestor, tc.descendant, level, tc.wantLevel)
			}
		})
	}
}

func TestEvaluateInviterFirstUseRisk_NewRules(t *testing.T) {
	cases := []struct {
		name      string
		sig       AffiliateRiskSignal
		total     int
		wantEmpty bool
		want      string
	}{
		{
			name:      "clean inviter",
			sig:       AffiliateRiskSignal{Email: "normal.user@example.com", InvitesIn24h: 3},
			total:     8,
			wantEmpty: true,
		},
		{
			name:  "24h invites over 5",
			sig:   AffiliateRiskSignal{Email: "normal.user@example.com", InvitesIn24h: 6},
			total: 6,
			want:  AffiliateRiskReasonInviteBurst24h,
		},
		{
			name:      "24h invites exactly 5 is clean",
			sig:       AffiliateRiskSignal{Email: "normal.user@example.com", InvitesIn24h: 5},
			total:     10,
			wantEmpty: true,
		},
		{
			name:  "total invites over 10 regardless of 24h window",
			sig:   AffiliateRiskSignal{Email: "normal.user@example.com", InvitesIn24h: 1},
			total: 11,
			want:  AffiliateRiskReasonInviteBurst24h,
		},
		{
			name:      "total invites exactly 10 is clean",
			sig:       AffiliateRiskSignal{Email: "normal.user@example.com", InvitesIn24h: 1},
			total:     10,
			wantEmpty: true,
		},
		{
			name:  "ancestor chain depth 4",
			sig:   AffiliateRiskSignal{Email: "normal.user@example.com", AncestorDepth: 4},
			total: 0,
			want:  AffiliateRiskReasonChainDepth,
		},
		{
			name:  "descendant chain depth 4",
			sig:   AffiliateRiskSignal{Email: "normal.user@example.com", DescendantDepth: 4},
			total: 0,
			want:  AffiliateRiskReasonChainDepth,
		},
		{
			name:      "chain depth 3 is clean",
			sig:       AffiliateRiskSignal{Email: "normal.user@example.com", AncestorDepth: 3},
			total:     0,
			wantEmpty: true,
		},
		{
			name:  "email local part over 25",
			sig:   AffiliateRiskSignal{Email: "averyverylonglocalpartname@example.com"},
			total: 0,
			want:  AffiliateRiskReasonEmailLocalPartLong,
		},
		{
			name:      "email local part exactly 25 is clean",
			sig:       AffiliateRiskSignal{Email: "1234567890123456789012345@example.com"},
			total:     0,
			wantEmpty: true,
		},
		{
			name:  "disposable email rule retained",
			sig:   AffiliateRiskSignal{Email: "user@mailinator.com"},
			total: 0,
			want:  AffiliateRiskReasonDisposableEmail,
		},
		{
			name:  "alias stuffing rule retained",
			sig:   AffiliateRiskSignal{Email: "lanhsuychsieck.i.e.c11.23@gmail.com"},
			total: 0,
			want:  AffiliateRiskReasonEmailAliasStuffing,
		},
		{
			name:  "chain contagion rule retained",
			sig:   AffiliateRiskSignal{Email: "normal.user@example.com", ChainContagion: true},
			total: 0,
			want:  AffiliateRiskReasonChainContagion,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := evaluateInviterFirstUseRisk(tc.sig, tc.total)
			if tc.wantEmpty {
				if len(got) != 0 {
					t.Fatalf("reasons = %v, want empty", got)
				}
				return
			}
			found := false
			for _, r := range got {
				if r == tc.want {
					found = true
				}
			}
			if !found {
				t.Fatalf("reasons = %v, want to contain %q", got, tc.want)
			}
		})
	}
}

func TestEmailLocalPart(t *testing.T) {
	cases := []struct {
		email string
		want  string
	}{
		{"user@example.com", "user"},
		{"a.b.c@gmail.com", "a.b.c"},
		{"@example.com", ""},
		{"no-at-sign", ""},
	}
	for _, tc := range cases {
		if got := emailLocalPart(tc.email); got != tc.want {
			t.Errorf("emailLocalPart(%q) = %q, want %q", tc.email, got, tc.want)
		}
	}
}
