package service

import "encoding/json"

// SitePromoConfig holds the backend-configurable promotion banner content for
// one frontend site. It is rendered by PromoBanner from the active profile.
type SitePromoConfig struct {
	Badge string   `json:"badge"`
	Title string   `json:"title"`
	CTA   string   `json:"cta"`
	Items []string `json:"items"`
}

// SiteProfile groups the backend-configurable branding content of one frontend
// site (site_a / site_b). The homepage layout itself lives in dedicated Vue
// components; this struct only carries content admins need to edit.
type SiteProfile struct {
	Name       string          `json:"name"`
	Logo       string          `json:"logo"`
	Subtitle   string          `json:"subtitle"`
	FooterHTML string          `json:"footer_html"`
	Promo      SitePromoConfig `json:"promo"`
}

// defaultSiteProfilesJSON seeds two sites: site_a keeps the existing English
// Three Router branding, site_b is a Chinese-branded site.
const defaultSiteProfilesJSON = `{
  "site_a": {
    "name": "Three Router",
    "logo": "/logo.webp",
    "subtitle": "AI API Gateway Platform",
    "footer_html": "",
    "promo": {
      "badge": "",
      "title": "",
      "cta": "",
      "items": []
    }
  },
  "site_b": {
    "name": "三路由",
    "logo": "/logo.webp",
    "subtitle": "安全合规的 AI API 网关服务平台",
    "footer_html": "© 三路由 · 保留所有权利 · 安全合规的 AI API 服务",
    "promo": {
      "badge": "限时特惠",
      "title": "注册即送免费额度",
      "cta": "立即开始",
      "items": ["无隐藏费用", "即时开通 API", "7×24 小时支持"]
    }
  }
}`

// parseSiteProfiles decodes the JSON setting into a profiles map, falling back
// to built-in defaults when the value is empty or malformed. Built-in keys are
// always backfilled so the frontend never sees a partially configured site.
func parseSiteProfiles(raw string) map[string]SiteProfile {
	result := map[string]SiteProfile{}
	_ = json.Unmarshal([]byte(raw), &result)
	defaults := map[string]SiteProfile{}
	_ = json.Unmarshal([]byte(defaultSiteProfilesJSON), &defaults)
	for key, profile := range defaults {
		if _, ok := result[key]; !ok {
			result[key] = profile
		}
	}
	if len(result) == 0 {
		return defaults
	}
	return result
}

// ParseSiteProfiles is the exported form used by the admin handler to turn the
// stored JSON string into the map shape returned by the settings API.
func ParseSiteProfiles(raw string) map[string]SiteProfile {
	return parseSiteProfiles(raw)
}

// DefaultSiteProfilesJSON is exposed for seeding/testing consumers.
const DefaultSiteProfilesJSON = defaultSiteProfilesJSON

// normalizeActiveSite returns a site key that actually exists in profiles.
func normalizeActiveSite(raw string, profiles map[string]SiteProfile) string {
	if raw != "" {
		if _, ok := profiles[raw]; ok {
			return raw
		}
	}
	if _, ok := profiles["site_a"]; ok {
		return "site_a"
	}
	for key := range profiles {
		return key
	}
	return "site_a"
}
