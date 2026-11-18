package service

import (
	"context"
	"encoding/json"
	"fmt"
)

// SiteSettings is the admin-facing payload for the global frontend site switch.
type SiteSettings struct {
	ActiveSite   string                 `json:"active_site"`
	SiteProfiles map[string]SiteProfile `json:"site_profiles"`
}

// GetSiteSettings returns the current active site and all configured profiles,
// normalizing active_site against the profiles that actually exist.
func (s *SettingService) GetSiteSettings(ctx context.Context) (*SiteSettings, error) {
	values, err := s.settingRepo.GetMultiple(ctx, []string{SettingKeyActiveSite, SettingKeySiteProfiles})
	if err != nil {
		return nil, fmt.Errorf("get site settings: %w", err)
	}
	profiles := parseSiteProfiles(values[SettingKeySiteProfiles])
	active := normalizeActiveSite(values[SettingKeyActiveSite], profiles)
	return &SiteSettings{ActiveSite: active, SiteProfiles: profiles}, nil
}

// UpdateSiteSettings persists the active site and profile map. This path is
// intentionally separate from the monolithic UpdateSettings request: it owns
// only two keys and refreshes every cache (including the SSR HTML cache via
// the onUpdate callback registered in the router).
func (s *SettingService) UpdateSiteSettings(ctx context.Context, activeSite string, profiles map[string]SiteProfile) error {
	if len(profiles) == 0 {
		return fmt.Errorf("site_profiles cannot be empty")
	}
	if _, ok := profiles[activeSite]; !ok {
		return fmt.Errorf("active_site %q is not present in site_profiles", activeSite)
	}
	data, err := json.Marshal(profiles)
	if err != nil {
		return fmt.Errorf("marshal site profiles: %w", err)
	}
	if err := s.settingRepo.SetMultiple(ctx, map[string]string{
		SettingKeyActiveSite:   activeSite,
		SettingKeySiteProfiles: string(data),
	}); err != nil {
		return fmt.Errorf("save site settings: %w", err)
	}
	s.invalidateCachedSettingValues(SettingKeyActiveSite, SettingKeySiteProfiles)
	s.invalidateAllCachedSettingValues()
	if s.onUpdate != nil {
		s.onUpdate()
	}
	return nil
}
