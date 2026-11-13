package routes

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/middleware"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// RegisterAuthRoutes 注册认证相关路由
func RegisterAuthRoutes(
	v1 *gin.RouterGroup,
	h *handler.Handlers,
	jwtAuth servermiddleware.JWTAuthMiddleware,
	auditLog servermiddleware.AuditLogMiddleware,
	redisClient *redis.Client,
	settingService *service.SettingService,
	panelRateLimiter *servermiddleware.PanelRateLimiter,
	opsService *service.OpsService,
) {
	// 创建速率限制器
	rateLimiter := middleware.NewRateLimiter(redisClient)

	// 公开接口
	auth := v1.Group("/auth")
	auth.Use(servermiddleware.BackendModeAuthGuard(settingService))
	// 认证事件（登录/注册/2FA/token 刷新失败）入审计
	auth.Use(gin.HandlerFunc(auditLog))
	// 登录/注册等失败（4xx）写入「错误请求」：无法确定用户时兜底记 admin(id=1)，
	// 防止登录坍塌在「使用记录 → 错误请求」中毫无痕迹。该变体仅用于 auth 路由组。
	auth.Use(handler.OpsErrorLoggerMiddlewareWithDefaultUser(opsService, 1))
	{
		// 自建顺序点击验证码（仅邮箱验证开启且启用开关时强制）。
		// 两个接口都挂限流：challenge 出题要生成位图（CPU 打点），verify 是
		// 爆破题面答案的入口，不限流时机器人可以无限换题重试。
		// 这里收紧阈值，并继续按可信客户端 IP 单一维度计数：同一 IP 下所有
		// User-Agent 共享同一个桶，轮换 UA 无法打散计数、重新拿满额度。
		if h.RegistrationClickCaptcha != nil {
			auth.POST("/captcha/challenge", rateLimiter.LimitWithOptions("auth-captcha-challenge", 10, time.Minute, middleware.RateLimitOptions{
				FailureMode: middleware.RateLimitFailClose,
			}), h.RegistrationClickCaptcha.Challenge)
			auth.POST("/captcha/verify", rateLimiter.LimitWithOptions("auth-captcha-verify", 15, time.Minute, middleware.RateLimitOptions{
				FailureMode: middleware.RateLimitFailClose,
			}), h.RegistrationClickCaptcha.Verify)
		}
		// 注册/登录/2FA/验证码发送均属于高风险入口，增加服务端兜底限流（Redis 故障时 fail-close）
		auth.POST("/register", rateLimiter.LimitWithOptions("auth-register", 5, time.Minute, middleware.RateLimitOptions{
			FailureMode: middleware.RateLimitFailClose,
		}), h.Auth.Register)
		auth.POST("/login", rateLimiter.LimitWithOptions("auth-login", 20, time.Minute, middleware.RateLimitOptions{
			FailureMode: middleware.RateLimitFailClose,
		}), h.Auth.Login)
		auth.POST("/login/2fa", rateLimiter.LimitWithOptions("auth-login-2fa", 20, time.Minute, middleware.RateLimitOptions{
			FailureMode: middleware.RateLimitFailClose,
		}), h.Auth.Login2FA)
		auth.POST("/passkey/login/begin", rateLimiter.LimitWithOptions("passkey-login-begin", 20, time.Minute, middleware.RateLimitOptions{
			FailureMode: middleware.RateLimitFailClose,
		}), h.Passkey.BeginLogin)
		auth.POST("/passkey/login/finish", rateLimiter.LimitWithOptions("passkey-login-finish", 20, time.Minute, middleware.RateLimitOptions{
			FailureMode: middleware.RateLimitFailClose,
		}), h.Passkey.FinishLogin)
		auth.POST("/send-verify-code", rateLimiter.LimitWithOptions("auth-send-verify-code", 5, time.Minute, middleware.RateLimitOptions{
			FailureMode: middleware.RateLimitFailClose,
		}), h.Auth.SendVerifyCode)
		// 注册邮箱预检：规则校验 + 重复注册拦截，不发送验证码。
		// 与注册接口同为 5/min：预检会返回 EMAIL_EXISTS，收紧限流以抑制邮箱枚举。
		auth.POST("/registration/email-check", rateLimiter.LimitWithOptions("auth-registration-email-check", 5, time.Minute, middleware.RateLimitOptions{
			FailureMode: middleware.RateLimitFailClose,
		}), h.Auth.CheckRegistrationEmail)
		// Token刷新接口添加速率限制：每分钟最多 30 次（Redis 故障时 fail-close）
		auth.POST("/refresh", rateLimiter.LimitWithOptions("refresh-token", 30, time.Minute, middleware.RateLimitOptions{
			FailureMode: middleware.RateLimitFailClose,
		}), h.Auth.RefreshToken)
		// 登出接口（公开，允许未认证用户调用以撤销Refresh Token）
		auth.POST("/logout", h.Auth.Logout)
		// 优惠码验证接口添加速率限制：每分钟最多 10 次（Redis 故障时 fail-close）
		auth.POST("/validate-promo-code", rateLimiter.LimitWithOptions("validate-promo", 10, time.Minute, middleware.RateLimitOptions{
			FailureMode: middleware.RateLimitFailClose,
		}), h.Auth.ValidatePromoCode)
		// 邀请码验证接口添加速率限制：每分钟最多 10 次（Redis 故障时 fail-close）
		auth.POST("/validate-invitation-code", rateLimiter.LimitWithOptions("validate-invitation", 10, time.Minute, middleware.RateLimitOptions{
			FailureMode: middleware.RateLimitFailClose,
		}), h.Auth.ValidateInvitationCode)
		// 忘记密码接口添加速率限制：每分钟最多 5 次（Redis 故障时 fail-close）
		auth.POST("/forgot-password", rateLimiter.LimitWithOptions("forgot-password", 5, time.Minute, middleware.RateLimitOptions{
			FailureMode: middleware.RateLimitFailClose,
		}), h.Auth.ForgotPassword)
		// 重置密码接口添加速率限制：每分钟最多 10 次（Redis 故障时 fail-close）
		auth.POST("/reset-password", rateLimiter.LimitWithOptions("reset-password", 10, time.Minute, middleware.RateLimitOptions{
			FailureMode: middleware.RateLimitFailClose,
		}), h.Auth.ResetPassword)
		auth.GET("/oauth/linuxdo/start", h.Auth.LinuxDoOAuthStart)
		auth.POST("/oauth/linuxdo/start", rateLimiter.LimitWithOptions("oauth-linuxdo-start", 20, time.Minute, middleware.RateLimitOptions{
			FailureMode: middleware.RateLimitFailClose,
		}), h.Auth.LinuxDoOAuthStart)
		auth.GET("/oauth/github/start", h.Auth.GitHubOAuthStart)
		auth.POST("/oauth/github/start", rateLimiter.LimitWithOptions("oauth-github-start", 20, time.Minute, middleware.RateLimitOptions{
			FailureMode: middleware.RateLimitFailClose,
		}), h.Auth.GitHubOAuthStart)
		auth.GET("/oauth/github/callback", h.Auth.GitHubOAuthCallback)
		auth.POST("/oauth/github/complete-registration",
			rateLimiter.LimitWithOptions("oauth-github-complete", 10, time.Minute, middleware.RateLimitOptions{
				FailureMode: middleware.RateLimitFailClose,
			}),
			h.Auth.CompleteGitHubOAuthRegistration,
		)
		auth.GET("/oauth/google/start", h.Auth.GoogleOAuthStart)
		auth.POST("/oauth/google/start", rateLimiter.LimitWithOptions("oauth-google-start", 20, time.Minute, middleware.RateLimitOptions{
			FailureMode: middleware.RateLimitFailClose,
		}), h.Auth.GoogleOAuthStart)
		auth.GET("/oauth/google/callback", h.Auth.GoogleOAuthCallback)
		auth.POST("/oauth/google/complete-registration",
			rateLimiter.LimitWithOptions("oauth-google-complete", 10, time.Minute, middleware.RateLimitOptions{
				FailureMode: middleware.RateLimitFailClose,
			}),
			h.Auth.CompleteGoogleOAuthRegistration,
		)
		auth.GET("/oauth/linuxdo/bind/start", func(c *gin.Context) {
			query := c.Request.URL.Query()
			query.Set("intent", "bind_current_user")
			c.Request.URL.RawQuery = query.Encode()
			h.Auth.LinuxDoOAuthStart(c)
		})
		auth.GET("/oauth/linuxdo/callback", h.Auth.LinuxDoOAuthCallback)
		auth.GET("/oauth/wechat/start", h.Auth.WeChatOAuthStart)
		auth.POST("/oauth/wechat/start", rateLimiter.LimitWithOptions("oauth-wechat-start", 20, time.Minute, middleware.RateLimitOptions{
			FailureMode: middleware.RateLimitFailClose,
		}), h.Auth.WeChatOAuthStart)
		auth.GET("/oauth/wechat/bind/start", func(c *gin.Context) {
			query := c.Request.URL.Query()
			query.Set("intent", "bind_current_user")
			c.Request.URL.RawQuery = query.Encode()
			h.Auth.WeChatOAuthStart(c)
		})
		auth.GET("/oauth/wechat/callback", h.Auth.WeChatOAuthCallback)
		auth.GET("/oauth/wechat/payment/start", h.Auth.WeChatPaymentOAuthStart)
		auth.GET("/oauth/wechat/payment/callback", h.Auth.WeChatPaymentOAuthCallback)
		auth.POST("/oauth/pending/exchange",
			rateLimiter.LimitWithOptions("oauth-pending-exchange", 20, time.Minute, middleware.RateLimitOptions{
				FailureMode: middleware.RateLimitFailClose,
			}),
			h.Auth.ExchangePendingOAuthCompletion,
		)
		auth.POST("/oauth/pending/send-verify-code",
			rateLimiter.LimitWithOptions("oauth-pending-send-verify-code", 5, time.Minute, middleware.RateLimitOptions{
				FailureMode: middleware.RateLimitFailClose,
			}),
			h.Auth.SendPendingOAuthVerifyCode,
		)
		auth.POST("/oauth/pending/create-account",
			rateLimiter.LimitWithOptions("oauth-pending-create-account", 10, time.Minute, middleware.RateLimitOptions{
				FailureMode: middleware.RateLimitFailClose,
			}),
			h.Auth.CreatePendingOAuthAccount,
		)
		auth.POST("/oauth/pending/bind-login",
			rateLimiter.LimitWithOptions("oauth-pending-bind-login", 10, time.Minute, middleware.RateLimitOptions{
				FailureMode: middleware.RateLimitFailClose,
			}),
			h.Auth.BindPendingOAuthLogin,
		)
		auth.POST("/oauth/linuxdo/complete-registration",
			rateLimiter.LimitWithOptions("oauth-linuxdo-complete", 10, time.Minute, middleware.RateLimitOptions{
				FailureMode: middleware.RateLimitFailClose,
			}),
			h.Auth.CompleteLinuxDoOAuthRegistration,
		)
		auth.POST("/oauth/linuxdo/bind-login",
			rateLimiter.LimitWithOptions("oauth-linuxdo-bind-login", 20, time.Minute, middleware.RateLimitOptions{
				FailureMode: middleware.RateLimitFailClose,
			}),
			h.Auth.BindLinuxDoOAuthLogin,
		)
		auth.POST("/oauth/linuxdo/create-account",
			rateLimiter.LimitWithOptions("oauth-linuxdo-create-account", 10, time.Minute, middleware.RateLimitOptions{
				FailureMode: middleware.RateLimitFailClose,
			}),
			h.Auth.CreateLinuxDoOAuthAccount,
		)
		auth.POST("/oauth/wechat/complete-registration",
			rateLimiter.LimitWithOptions("oauth-wechat-complete", 10, time.Minute, middleware.RateLimitOptions{
				FailureMode: middleware.RateLimitFailClose,
			}),
			h.Auth.CompleteWeChatOAuthRegistration,
		)
		auth.POST("/oauth/wechat/bind-login",
			rateLimiter.LimitWithOptions("oauth-wechat-bind-login", 20, time.Minute, middleware.RateLimitOptions{
				FailureMode: middleware.RateLimitFailClose,
			}),
			h.Auth.BindWeChatOAuthLogin,
		)
		auth.POST("/oauth/wechat/create-account",
			rateLimiter.LimitWithOptions("oauth-wechat-create-account", 10, time.Minute, middleware.RateLimitOptions{
				FailureMode: middleware.RateLimitFailClose,
			}),
			h.Auth.CreateWeChatOAuthAccount,
		)
		auth.GET("/oauth/oidc/start", h.Auth.OIDCOAuthStart)
		auth.POST("/oauth/oidc/start", rateLimiter.LimitWithOptions("oauth-oidc-start", 20, time.Minute, middleware.RateLimitOptions{
			FailureMode: middleware.RateLimitFailClose,
		}), h.Auth.OIDCOAuthStart)
		auth.GET("/oauth/oidc/bind/start", func(c *gin.Context) {
			query := c.Request.URL.Query()
			query.Set("intent", "bind_current_user")
			c.Request.URL.RawQuery = query.Encode()
			h.Auth.OIDCOAuthStart(c)
		})
		auth.GET("/oauth/oidc/callback", h.Auth.OIDCOAuthCallback)
		auth.POST("/oauth/oidc/complete-registration",
			rateLimiter.LimitWithOptions("oauth-oidc-complete", 10, time.Minute, middleware.RateLimitOptions{
				FailureMode: middleware.RateLimitFailClose,
			}),
			h.Auth.CompleteOIDCOAuthRegistration,
		)
		auth.POST("/oauth/oidc/bind-login",
			rateLimiter.LimitWithOptions("oauth-oidc-bind-login", 20, time.Minute, middleware.RateLimitOptions{
				FailureMode: middleware.RateLimitFailClose,
			}),
			h.Auth.BindOIDCOAuthLogin,
		)
		auth.POST("/oauth/oidc/create-account",
			rateLimiter.LimitWithOptions("oauth-oidc-create-account", 10, time.Minute, middleware.RateLimitOptions{
				FailureMode: middleware.RateLimitFailClose,
			}),
			h.Auth.CreateOIDCOAuthAccount,
		)
		auth.GET("/oauth/dingtalk/start", h.Auth.DingTalkOAuthStart)
		auth.POST("/oauth/dingtalk/start", rateLimiter.LimitWithOptions("oauth-dingtalk-start", 20, time.Minute, middleware.RateLimitOptions{
			FailureMode: middleware.RateLimitFailClose,
		}), h.Auth.DingTalkOAuthStart)
		auth.GET("/oauth/dingtalk/bind/start", func(c *gin.Context) {
			query := c.Request.URL.Query()
			query.Set("intent", "bind_current_user")
			c.Request.URL.RawQuery = query.Encode()
			h.Auth.DingTalkOAuthStart(c)
		})
		auth.GET("/oauth/dingtalk/callback", h.Auth.DingTalkOAuthCallback)
		auth.POST("/oauth/dingtalk/complete-registration",
			rateLimiter.LimitWithOptions("oauth-dingtalk-complete", 10, time.Minute, middleware.RateLimitOptions{
				FailureMode: middleware.RateLimitFailClose,
			}),
			h.Auth.CompleteDingTalkOAuthRegistration,
		)
		auth.POST("/oauth/dingtalk/bind-login",
			rateLimiter.LimitWithOptions("oauth-dingtalk-bind-login", 20, time.Minute, middleware.RateLimitOptions{
				FailureMode: middleware.RateLimitFailClose,
			}),
			h.Auth.BindDingTalkOAuthLogin,
		)
		auth.POST("/oauth/dingtalk/create-account",
			rateLimiter.LimitWithOptions("oauth-dingtalk-create-account", 10, time.Minute, middleware.RateLimitOptions{
				FailureMode: middleware.RateLimitFailClose,
			}),
			h.Auth.CreateDingTalkOAuthAccount,
		)
	}

	// 公开设置（无需认证）：每次请求都会查询 DB，按客户端 IP 兜底限流，
	// 防止匿名高频刷接口打爆数据库（反代内部地址会被自动跳过，不会误伤）。
	settings := v1.Group("/settings")
	settings.Use(panelRateLimiter.PublicIP())
	{
		settings.GET("/public", h.Setting.GetPublicSettings)
		settings.GET("/email-unsubscribe", h.Setting.UnsubscribeNotificationEmail)
	}

	// 公开模型定价查询（无需认证，仅返回定价信息）
	v1.GET("/models/pricing", h.Admin.Channel.GetModelDefaultPricing)

	// 需要认证的当前用户信息
	authenticated := v1.Group("")
	authenticated.Use(gin.HandlerFunc(jwtAuth))
	authenticated.Use(servermiddleware.BackendModeUserGuard(settingService))
	// 面板全局按用户限流
	authenticated.Use(panelRateLimiter.Global())
	{
		authenticated.GET("/auth/me", h.Auth.GetCurrentUser)
		// 撤销所有会话（需要认证）
		authenticated.POST("/auth/revoke-all-sessions", h.Auth.RevokeAllSessions)
		authenticated.POST("/auth/oauth/bind-token", h.Auth.PrepareOAuthBindAccessTokenCookie)
	}
}
