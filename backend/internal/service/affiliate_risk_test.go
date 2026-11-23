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
		{"ancestor depth 3 is suspect", 3, 0, AffiliateRiskLevelSuspect},
		{"descendant depth 3 is suspect", 0, 3, AffiliateRiskLevelSuspect},
		{"depth 4 is abnormal", 0, 4, AffiliateRiskLevelAbnormal},
		{"deepest direction wins", 2, 4, AffiliateRiskLevelAbnormal},
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
