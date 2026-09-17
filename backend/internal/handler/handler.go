package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/securityaudit"
)

// AdminHandlers contains all admin-related HTTP handlers
type AdminHandlers struct {
	Dashboard              *admin.DashboardHandler
	User                   *admin.UserHandler
	Group                  *admin.GroupHandler
	Account                *admin.AccountHandler
	Announcement           *admin.AnnouncementHandler
	Blog                   *admin.BlogHandler
	DataManagement         *admin.DataManagementHandler
	Backup                 *admin.BackupHandler
	OAuth                  *admin.OAuthHandler
	OpenAIOAuth            *admin.OpenAIOAuthHandler
	GeminiOAuth            *admin.GeminiOAuthHandler
	AntigravityOAuth       *admin.AntigravityOAuthHandler
	GrokOAuth              *admin.GrokOAuthHandler
	CNProvider             *admin.CNProviderHandler
	Proxy                  *admin.ProxyHandler
	Redeem                 *admin.RedeemHandler
	Promo                  *admin.PromoHandler
	Setting                *admin.SettingHandler
	Ops                    *admin.OpsHandler
	System                 *admin.SystemHandler
	Subscription           *admin.SubscriptionHandler
	Usage                  *admin.UsageHandler
	UserAttribute          *admin.UserAttributeHandler
	ErrorPassthrough       *admin.ErrorPassthroughHandler
	TLSFingerprintProfile  *admin.TLSFingerprintProfileHandler
	Plugin                 *admin.PluginHandler
	APIKey                 *admin.AdminAPIKeyHandler
	ScheduledTest          *admin.ScheduledTestHandler
	Channel                *admin.ChannelHandler
	ChannelMonitor         *admin.ChannelMonitorHandler
	ChannelMonitorTemplate *admin.ChannelMonitorRequestTemplateHandler
	ContentModeration      *admin.ContentModerationHandler
	PromptAudit            *securityaudit.PromptAdminHandler
	Payment                *admin.PaymentHandler
	Affiliate              *admin.AffiliateHandler
	Compliance             *admin.ComplianceHandler
	Governance             *admin.GovernanceHandler
	ModerationRule         *admin.ModerationRuleHandler
	Ticket                 *admin.TicketHandler
	AuditLog               *admin.AuditLogHandler
	MediaTask              *admin.MediaTaskAdminHandler
}

// Handlers contains all HTTP handlers
type Handlers struct {
	Auth                     *AuthHandler
	RegistrationClickCaptcha *RegistrationClickCaptchaHandler
	User                     *UserHandler
	APIKey                   *APIKeyHandler
	Usage                    *UsageHandler
	Redeem                   *RedeemHandler
	Subscription             *SubscriptionHandler
	Announcement             *AnnouncementHandler
	Blog                     *BlogHandler
	ChannelMonitor           *ChannelMonitorUserHandler
	ChannelMonitorV2         *ChannelMonitorV2Handler
	Admin                    *AdminHandlers
	Gateway                  *GatewayHandler
	OpenAIGateway            *OpenAIGatewayHandler
	Setting                  *SettingHandler
	Totp                     *TotpHandler
	Passkey                  *PasskeyHandler
	Payment                  *PaymentHandler
	PaymentWebhook           *PaymentWebhookHandler
	AvailableChannel         *AvailableChannelHandler
	Ticket                   *TicketHandler
	Governance               *GovernanceUserHandler
	Team                     *TeamHandler
	Department               *DepartmentHandler
	Consumer                 *ConsumerHandler
	TeamAnalytics            *TeamAnalyticsHandler
	ModelPlaza               *ModelPlazaHandler
	AsyncImage               *AsyncImageHandler
	BatchImage               *BatchImageHandler
	VideoGateway             *VideoGatewayHandler
	MediaGateway             *MediaGatewayHandler
}

// BuildInfo contains build-time information
type BuildInfo struct {
	Version   string
	BuildType string // "source" for manual builds, "release" for CI builds
}
