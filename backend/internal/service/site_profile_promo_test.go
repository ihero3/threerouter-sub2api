package service

import (
	"encoding/json"
	"testing"
)

// TestSitePromoConfigEnabledSemantics locks the backward-compatible default:
// a legacy config without the "enabled" key must keep showing the banner,
// while an explicit false hides it everywhere.
func TestSitePromoConfigEnabledSemantics(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{"legacy config without enabled defaults to shown", `{"badge":"x","items":[]}`, true},
		{"explicit false hides", `{"badge":"x","enabled":false}`, false},
		{"explicit true shows", `{"badge":"x","enabled":true}`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg SitePromoConfig
			if err := json.Unmarshal([]byte(tt.raw), &cfg); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got := cfg.PromoEnabled(); got != tt.want {
				t.Fatalf("PromoEnabled() = %v, want %v", got, tt.want)
			}
		})
	}

	// Direct struct construction (admin UI always sends an explicit value).
	explicitFalse := false
	if !(SitePromoConfig{}).PromoEnabled() {
		t.Fatal("zero-value config must default to shown")
	}
	if (SitePromoConfig{Enabled: &explicitFalse}).PromoEnabled() {
		t.Fatal("explicit false must hide")
	}
}

// TestDefaultSiteProfilesHavePromoEnabled ensures freshly seeded configs carry
// an explicit true so the admin toggle reflects the real state on first load.
func TestDefaultSiteProfilesHavePromoEnabled(t *testing.T) {
	profiles := parseSiteProfiles(DefaultSiteProfilesJSON)
	for key, profile := range profiles {
		if profile.Promo.Enabled == nil || !*profile.Promo.Enabled {
			t.Fatalf("site %q default promo must be explicitly enabled", key)
		}
	}
}
