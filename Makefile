.PHONY: build build-backend build-frontend test test-backend test-frontend test-frontend-critical setup-merge-driver

FRONTEND_CRITICAL_VITEST := \
	src/i18n/__tests__/localeKeyCompleteness.spec.ts \
	src/api/__tests__/client.spec.ts \
	src/api/__tests__/tokenRefresh.spec.ts \
	src/api/__tests__/keys.bulkUpdate.spec.ts \
	src/composables/__tests__/useModelWhitelist.spec.ts \
	src/components/account/__tests__/OpenAIReferralCell.spec.ts \
	src/components/account/__tests__/OpenAIReferralCell.transport.spec.ts \
	src/components/account/__tests__/OpenAIQuotaResetCell.spark_shadow.spec.ts \
	src/constants/__tests__/platforms.spec.ts \
	src/components/account/__tests__/credentialsBuilder.platformCatalog.spec.ts \
	src/components/account/__tests__/CreateAccountModal.spec.ts \
	src/components/account/__tests__/EditAccountModal.spec.ts \
	src/components/account/__tests__/credentialsBuilder.spec.ts \
	src/components/account/__tests__/OpenCodeGoProtocolRulesEditor.spec.ts \
	src/components/keys/__tests__/BulkEditKeysModal.spec.ts \
	src/components/admin/user/__tests__/UserPlatformQuotaModal.spec.ts \
	src/views/user/__tests__/KeysView.spec.ts \
	src/api/__tests__/channelMonitorV2.spec.ts \
	src/views/auth/__tests__/LinuxDoCallbackView.spec.ts \
	src/views/auth/__tests__/WechatCallbackView.spec.ts \
	src/views/user/__tests__/PaymentView.spec.ts \
	src/views/user/__tests__/PaymentResultView.spec.ts \
	src/views/user/__tests__/ChannelStatusView.mode.spec.ts \
	src/components/user/profile/__tests__/ProfileInfoCard.spec.ts \
	src/components/user/profile/__tests__/ProfileIdentityBindingsSection.spec.ts \
	src/views/admin/__tests__/SettingsView.spec.ts \
	src/features/channel-monitor-v2/__tests__/designSystem.structure.spec.ts \
	src/features/channel-monitor-v2/__tests__/monitorFormat.spec.ts \
	src/features/channel-monitor-v2/__tests__/monitorZoom.spec.ts \
	src/components/admin/channel/__tests__/PricingEntryCard.modelDefaultPrice.spec.ts

# 一键编译前后端
build: build-backend build-frontend

# 编译后端（复用 backend/Makefile）
build-backend:
	@$(MAKE) -C backend build

# 编译前端（需要已安装依赖）
build-frontend:
	@pnpm --dir frontend run build

# 运行测试（后端 + 前端）
test: test-backend test-frontend

test-backend:
	@$(MAKE) -C backend test

test-frontend:
	@pnpm --dir frontend run lint:check
	@pnpm --dir frontend run typecheck
	@$(MAKE) test-frontend-critical

test-frontend-critical:
	@pnpm --dir frontend exec vitest run $(FRONTEND_CRITICAL_VITEST)

# 配置上游合并保护：.gitattributes 中标记了 merge=ours 的本地权威文件，
# 需要此 driver 才生效。克隆后 / 合并 upstream 前先执行一次。
setup-merge-driver:
	@git config merge.ours.driver true
	@echo "merge.ours.driver 已启用；本地权威文件在合并 upstream 时自动保留本地版本。"
