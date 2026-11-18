package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// GetSiteSettings 获取双前台站点配置
// GET /api/v1/admin/settings/site
func (h *SettingHandler) GetSiteSettings(c *gin.Context) {
	settings, err := h.settingService.GetSiteSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.SiteSettings{
		ActiveSite:   settings.ActiveSite,
		SiteProfiles: settings.SiteProfiles,
	})
}

// UpdateSiteSettingsRequest 更新双前台站点配置请求
type UpdateSiteSettingsRequest struct {
	ActiveSite   string                         `json:"active_site"`
	SiteProfiles map[string]service.SiteProfile `json:"site_profiles"`
}

// UpdateSiteSettings 更新双前台站点配置（全局开关 + 两套品牌内容）
// PUT /api/v1/admin/settings/site
func (h *SettingHandler) UpdateSiteSettings(c *gin.Context) {
	var req UpdateSiteSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := h.settingService.UpdateSiteSettings(c.Request.Context(), req.ActiveSite, req.SiteProfiles); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	updated, err := h.settingService.GetSiteSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.SiteSettings{
		ActiveSite:   updated.ActiveSite,
		SiteProfiles: updated.SiteProfiles,
	})
}
