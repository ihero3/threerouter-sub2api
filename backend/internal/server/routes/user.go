package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterUserRoutes 注册用户相关路由（需要认证）
func RegisterUserRoutes(
	v1 *gin.RouterGroup,
	h *handler.Handlers,
	jwtAuth middleware.JWTAuthMiddleware,
	settingService *service.SettingService,
) {
	// 游客工单提交
	publicTickets := v1.Group("/public/tickets")
	{
		publicTickets.POST("", h.Ticket.Create)
	}

	authenticated := v1.Group("")
	authenticated.Use(gin.HandlerFunc(jwtAuth))
	authenticated.Use(middleware.BackendModeUserGuard(settingService))
	{
		// 用户接口
		user := authenticated.Group("/user")
		{
			user.GET("/profile", h.User.GetProfile)
			user.PUT("/password", h.User.ChangePassword)
			user.PUT("", h.User.UpdateProfile)
			user.GET("/aff", h.User.GetAffiliate)
			user.POST("/aff/transfer", h.User.TransferAffiliateQuota)
			user.POST("/account-bindings/email/send-code", h.User.SendEmailBindingCode)
			user.POST("/account-bindings/email", h.User.BindEmailIdentity)
			user.DELETE("/account-bindings/:provider", h.User.UnbindIdentity)
			user.POST("/auth-identities/bind/start", h.User.StartIdentityBinding)
			user.GET("/api-keys/:id/usage/daily", h.Usage.GetMyAPIKeyDailyUsage)
			user.GET("/platform-quotas", h.User.GetMyPlatformQuotas)

			// 通知邮箱管理
			notifyEmail := user.Group("/notify-email")
			{
				notifyEmail.POST("/send-code", h.User.SendNotifyEmailCode)
				notifyEmail.POST("/verify", h.User.VerifyNotifyEmail)
				notifyEmail.PUT("/toggle", h.User.ToggleNotifyEmail)
				notifyEmail.DELETE("", h.User.RemoveNotifyEmail)
			}

			// TOTP 双因素认证
			totp := user.Group("/totp")
			{
				totp.GET("/status", h.Totp.GetStatus)
				totp.GET("/verification-method", h.Totp.GetVerificationMethod)
				totp.POST("/send-code", h.Totp.SendVerifyCode)
				totp.POST("/setup", h.Totp.InitiateSetup)
				totp.POST("/enable", h.Totp.Enable)
				totp.POST("/disable", h.Totp.Disable)
			}
		}

		// API Key管理
		keys := authenticated.Group("/keys")
		{
			keys.GET("", h.APIKey.List)
			keys.GET("/:id", h.APIKey.GetByID)
			keys.POST("", h.APIKey.Create)
			keys.PUT("/:id", h.APIKey.Update)
			keys.DELETE("/:id", h.APIKey.Delete)
		}

		// 用户可用分组（非管理员接口）
		groups := authenticated.Group("/groups")
		{
			groups.GET("/available", h.APIKey.GetAvailableGroups)
			groups.GET("/rates", h.APIKey.GetUserGroupRates)
		}

		// 用户可用渠道（非管理员接口）
		channels := authenticated.Group("/channels")
		{
			channels.GET("/available", h.AvailableChannel.List)
		}

		// 使用记录
		usage := authenticated.Group("/usage")
		{
			usage.GET("", h.Usage.List)
			usage.GET("/errors", h.Usage.ListErrors)
			usage.GET("/errors/:id", h.Usage.GetErrorDetail)
			usage.GET("/:id", h.Usage.GetByID)
			usage.GET("/stats", h.Usage.Stats)
			// User dashboard endpoints
			usage.GET("/dashboard/stats", h.Usage.DashboardStats)
			usage.GET("/dashboard/trend", h.Usage.DashboardTrend)
			usage.GET("/dashboard/models", h.Usage.DashboardModels)
			usage.GET("/dashboard/snapshot-v2", h.Usage.DashboardSnapshotV2)
			usage.POST("/dashboard/api-keys-usage", h.Usage.DashboardAPIKeysUsage)
		}

		// 工单系统
		tickets := authenticated.Group("/tickets")
		{
			tickets.POST("", h.Ticket.Create)
			tickets.GET("/my", h.Ticket.ListMine)
			tickets.GET("/:id", h.Ticket.GetMine)
			tickets.POST("/:id/messages", h.Ticket.AddMessageMine)
		}

		// 公告（用户可见）
		announcements := authenticated.Group("/announcements")
		{
			announcements.GET("", h.Announcement.List)
			announcements.POST("/:id/read", h.Announcement.MarkRead)
		}

		// 卡密兑换
		redeem := authenticated.Group("/redeem")
		{
			redeem.POST("", h.Redeem.Redeem)
			redeem.GET("/history", h.Redeem.GetHistory)
		}

		// 用户订阅
		subscriptions := authenticated.Group("/subscriptions")
		{
			subscriptions.GET("", h.Subscription.List)
			subscriptions.GET("/active", h.Subscription.GetActive)
			subscriptions.GET("/progress", h.Subscription.GetProgress)
			subscriptions.GET("/summary", h.Subscription.GetSummary)
		}

		// 渠道监控（用户只读）
		monitors := authenticated.Group("/channel-monitors")
		{
			monitors.GET("", h.ChannelMonitor.List)
			monitors.GET("/:id/status", h.ChannelMonitor.GetStatus)
		}

		// AI 治理与合规（用户端：GDPR 数据主体权利 + Account 级合规配置）
		governance := authenticated.Group("/governance")
		{
			governance.POST("/data-erasure/request", h.Governance.RequestDataErasure)
		governance.GET("/data-erasure/requests", h.Governance.ListDataErasureRequests)
		governance.POST("/data-export", h.Governance.ExportData)
			governance.GET("/consent", h.Governance.GetConsent)
			governance.POST("/consent", h.Governance.SetConsent)
			governance.GET("/profile", h.Governance.GetComplianceProfile)
			governance.PUT("/profile", h.Governance.UpdateComplianceProfile)
			governance.GET("/templates", h.Governance.ListComplianceTemplates)
			governance.POST("/templates/apply", h.Governance.ApplyComplianceTemplate)
			governance.GET("/moderation-rules", h.Governance.ListModerationRules)
			governance.GET("/moderation-rules/user", h.Governance.ListUserModerationRules)
			governance.POST("/moderation-rules/user", h.Governance.CreateUserModerationRule)
			governance.PUT("/moderation-rules/user/:ruleId", h.Governance.UpdateUserModerationRule)
			governance.DELETE("/moderation-rules/user/:ruleId", h.Governance.DeleteUserModerationRule)
			governance.GET("/status", h.Governance.GetComplianceStatus)
			governance.GET("/jurisdiction/mapping", h.Governance.GetJurisdictionMapping)
			governance.GET("/jurisdiction/mapping/user", h.Governance.GetUserJurisdictionMapping)
			governance.POST("/jurisdiction/mapping/save", h.Governance.SaveJurisdictionMapping)
			governance.POST("/gdpr/dpa/generate", h.Governance.GenerateDPA)
			governance.GET("/credentials", h.Governance.ListCredentials)
			governance.GET("/audit-logs", h.Governance.ListAuditLogs)
			governance.GET("/risk-tags", h.Governance.RiskTags)
			governance.GET("/eu-ai-act/assessment", h.Governance.EUAIActAssessment)
			governance.POST("/eu-ai-act/assessment", h.Governance.ExportEUAIActAssessment)
			governance.GET("/gdpr/data-processing-record", h.Governance.DataProcessingRecord)
		}
	}
}
