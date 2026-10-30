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

		// 真实姓名形态，不应误报。
		{"firstname.lastname@gmail.com", false},
		{"john.q.public@gmail.com", false},
		{"zhangsan2024@gmail.com", false},
		{"li.wei.2020@gmail.com", false},

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
