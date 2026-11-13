package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/authidentity"
	"github.com/Wei-Shaw/sub2api/ent/authidentitychannel"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

// abnormal 账号未经管理员人工复核不得复活：UpdateUser 与内容风控 UnbanUser
// 共用这一份客户端可见文案，保证两处拒绝口径一致。纯英语，不混汉语。
const (
	riskReviewRequiredCode    = "RISK_REVIEW_REQUIRED"
	riskReviewRequiredMessage = "This account requires a risk review before it can be reactivated"
)

// User management implementations
func (s *adminServiceImpl) ListUsers(ctx context.Context, page, pageSize int, filters UserListFilters, sortBy, sortOrder string) ([]User, int64, error) {
	params := pagination.PaginationParams{Page: page, PageSize: pageSize, SortBy: sortBy, SortOrder: sortOrder}
	users, result, err := s.userRepo.ListWithFilters(ctx, params, filters)
	if err != nil {
		return nil, 0, err
	}
	if len(users) > 0 {
		userIDs := make([]int64, 0, len(users))
		for i := range users {
			userIDs = append(userIDs, users[i].ID)
		}
		lastUsedByUserID, latestErr := s.userRepo.GetLatestUsedAtByUserIDs(ctx, userIDs)
		if latestErr != nil {
			logger.LegacyPrintf("service.admin", "failed to load user last_used_at in batch: err=%v", latestErr)
		} else {
			for i := range users {
				users[i].LastUsedAt = lastUsedByUserID[users[i].ID]
			}
		}
	}
	// 批量加载最后使用 IP 与共享（Suspect）标记
	if s.userRepo != nil && len(users) > 0 {
		ids := make([]int64, 0, len(users))
		for i := range users {
			ids = append(ids, users[i].ID)
		}
		lastIPByUserID, ipErr := s.userRepo.GetLatestUsedIPByUserIDs(ctx, ids)
		if ipErr != nil {
			logger.LegacyPrintf("service.admin", "failed to load user last_used_ip in batch: err=%v", ipErr)
		} else {
			sharedIPs, sharedErr := s.userRepo.GetSharedUsageIPs(ctx)
			if sharedErr != nil {
				logger.LegacyPrintf("service.admin", "failed to load shared usage IPs: err=%v", sharedErr)
			} else {
				for i := range users {
					if ip, ok := lastIPByUserID[users[i].ID]; ok && ip != "" {
						users[i].LastUsedIP = ip
						users[i].LastUsedIPShared = sharedIPs[ip]
					}
				}
			}
		}
	}
	// 批量加载用户专属分组倍率
	if s.userGroupRateRepo != nil && len(users) > 0 {
		if batchRepo, ok := s.userGroupRateRepo.(userGroupRateBatchReader); ok {
			userIDs := make([]int64, 0, len(users))
			for i := range users {
				userIDs = append(userIDs, users[i].ID)
			}
			ratesByUser, err := batchRepo.GetByUserIDs(ctx, userIDs)
			if err != nil {
				logger.LegacyPrintf("service.admin", "failed to load user group rates in batch: err=%v", err)
				s.loadUserGroupRatesOneByOne(ctx, users)
			} else {
				for i := range users {
					if rates, ok := ratesByUser[users[i].ID]; ok {
						users[i].GroupRates = rates
					}
				}
			}
		} else {
			s.loadUserGroupRatesOneByOne(ctx, users)
		}
	}
	return users, result.Total, nil
}

func (s *adminServiceImpl) loadUserGroupRatesOneByOne(ctx context.Context, users []User) {
	if s.userGroupRateRepo == nil {
		return
	}
	for i := range users {
		rates, err := s.userGroupRateRepo.GetByUserID(ctx, users[i].ID)
		if err != nil {
			logger.LegacyPrintf("service.admin", "failed to load user group rates: user_id=%d err=%v", users[i].ID, err)
			continue
		}
		users[i].GroupRates = rates
	}
}

func (s *adminServiceImpl) GetUser(ctx context.Context, id int64) (*User, error) {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	lastUsedAt, latestErr := s.userRepo.GetLatestUsedAtByUserID(ctx, id)
	if latestErr != nil {
		logger.LegacyPrintf("service.admin", "failed to load user last_used_at: user_id=%d err=%v", id, latestErr)
	} else {
		user.LastUsedAt = lastUsedAt
	}
	// 加载最后使用 IP 与共享（Suspect）标记
	lastIP, ipErr := s.userRepo.GetLatestUsedIPByUserIDs(ctx, []int64{id})
	if ipErr != nil {
		logger.LegacyPrintf("service.admin", "failed to load user last_used_ip: user_id=%d err=%v", id, ipErr)
	} else if ip, ok := lastIP[id]; ok && ip != "" {
		sharedIPs, sharedErr := s.userRepo.GetSharedUsageIPs(ctx)
		if sharedErr != nil {
			logger.LegacyPrintf("service.admin", "failed to load shared usage IPs: user_id=%d err=%v", id, sharedErr)
		} else {
			user.LastUsedIP = ip
			user.LastUsedIPShared = sharedIPs[ip]
		}
	}
	// 加载用户专属分组倍率
	if s.userGroupRateRepo != nil {
		rates, err := s.userGroupRateRepo.GetByUserID(ctx, id)
		if err != nil {
			logger.LegacyPrintf("service.admin", "failed to load user group rates: user_id=%d err=%v", id, err)
		} else {
			user.GroupRates = rates
		}
	}
	return user, nil
}

func (s *adminServiceImpl) GetUserIncludeDeleted(ctx context.Context, id int64) (*User, error) {
	return s.userRepo.GetByIDIncludeDeleted(ctx, id)
}

// normalizeUserRole 校验并归一化角色输入。
// 空字符串返回 fallback(未提供时的默认角色);非法值返回错误。
func normalizeUserRole(role, fallback string) (string, error) {
	if role == "" {
		return fallback, nil
	}
	if role != RoleAdmin && role != RoleUser {
		return "", fmt.Errorf("invalid role: %q (must be %s or %s)", role, RoleAdmin, RoleUser)
	}
	return role, nil
}

func (s *adminServiceImpl) CreateUser(ctx context.Context, input *CreateUserInput) (*User, error) {
	balance := 0.0
	if input.Balance != nil {
		balance = *input.Balance
	} else if s.settingService != nil {
		balance = s.settingService.GetDefaultBalance(ctx)
	}

	// 角色可由管理员在创建时指定(admin/user);未提供时默认 user。
	role, err := normalizeUserRole(input.Role, RoleUser)
	if err != nil {
		return nil, err
	}

	user := &User{
		Email:         input.Email,
		Username:      input.Username,
		Notes:         input.Notes,
		Role:          role,
		Balance:       balance,
		Concurrency:   input.Concurrency,
		RPMLimit:      input.RPMLimit,
		Status:        StatusActive,
		AllowedGroups: input.AllowedGroups,

		RestrictPublicGroups: input.RestrictPublicGroups,
	}
	if err := user.SetPassword(input.Password); err != nil {
		return nil, err
	}
	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}
	// 创建管理员属权限敏感操作，落审计日志（含操作者），便于事后追溯。
	if user.Role == RoleAdmin {
		logger.LegacyPrintf("service.admin", "audit: admin user created actor_admin_id=%d target_user_id=%d",
			input.ActorAdminID, user.ID)
	}
	s.assignDefaultSubscriptions(ctx, user.ID)
	return user, nil
}

// ensureNotLastAdmin 降级管理员前确认系统中仍存在其他管理员，防止零 admin 锁死。
// 注：读取与写入之间存在竞态窗口，极端并发下仍可能双双降级；作为后台低频操作
// 的兜底保护足够，彻底防护需依赖数据库层约束。
func (s *adminServiceImpl) ensureNotLastAdmin(ctx context.Context) error {
	noSubs := false
	_, result, err := s.userRepo.ListWithFilters(ctx,
		pagination.PaginationParams{Page: 1, PageSize: 1},
		UserListFilters{Role: RoleAdmin, IncludeSubscriptions: &noSubs},
	)
	if err != nil {
		return fmt.Errorf("count admin users: %w", err)
	}
	if result == nil || result.Total <= 1 {
		return infraerrors.BadRequest("LAST_ADMIN_DEMOTE_FORBIDDEN", "cannot demote the last admin user")
	}
	return nil
}

func (s *adminServiceImpl) assignDefaultSubscriptions(ctx context.Context, userID int64) {
	if s.settingService == nil || s.defaultSubAssigner == nil || userID <= 0 {
		return
	}
	items := s.settingService.GetDefaultSubscriptions(ctx)
	for _, item := range items {
		if _, _, err := s.defaultSubAssigner.AssignOrExtendSubscription(ctx, &AssignSubscriptionInput{
			UserID:       userID,
			GroupID:      item.GroupID,
			ValidityDays: item.ValidityDays,
			Notes:        "auto assigned by default user subscriptions setting",
		}); err != nil {
			logger.LegacyPrintf("service.admin", "failed to assign default subscription: user_id=%d group_id=%d err=%v", userID, item.GroupID, err)
		}
	}
}

func (s *adminServiceImpl) UpdateUser(ctx context.Context, id int64, input *UpdateUserInput) (*User, error) {
	// 校验用户专属分组倍率：必须 > 0（nil 合法，表示清除专属倍率）
	if input.GroupRates != nil {
		for groupID, rate := range input.GroupRates {
			if rate != nil && *rate <= 0 {
				return nil, fmt.Errorf("rate_multiplier must be > 0 (group_id=%d)", groupID)
			}
		}
	}

	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Protect admin users: cannot disable admin accounts
	if user.Role == "admin" && input.Status == "disabled" {
		return nil, infraerrors.BadRequest("ADMIN_DISABLE_FORBIDDEN", "cannot disable admin user; change the role to user first")
	}

	// 风险复核冻结：abnormal 账号必须先经 POST /:id/risk-review 人工复核并清空
	// risk_level，才允许把 status 改回 active；否则禁止通过普通编辑绕过冻结。
	// 只拦「目标状态为 active」的写请求（最小条件），改其它字段不受影响。
	if user.RiskLevel == AffiliateRiskLevelAbnormal && input.Status == StatusActive {
		return nil, infraerrors.BadRequest(riskReviewRequiredCode, riskReviewRequiredMessage)
	}

	oldConcurrency := user.Concurrency
	oldStatus := user.Status
	oldRole := user.Role
	oldRPMLimit := user.RPMLimit
	oldAllowedGroups := append([]int64(nil), user.AllowedGroups...)

	// fields 与下面的 input.X 判空条件一一对应：管理员没提交的列不写回，
	// 避免这份快照回滚并发的扣费、状态变更或批量限额调整。
	var fields UserUpdateFields

	if input.Email != "" {
		user.Email = input.Email
		fields.Email = true
	}
	if input.Password != "" {
		if err := user.SetPassword(input.Password); err != nil {
			return nil, err
		}
		fields.PasswordHash = true
	}

	if input.Username != nil {
		user.Username = *input.Username
		fields.Username = true
	}
	if input.Notes != nil {
		user.Notes = *input.Notes
		fields.Notes = true
	}

	if input.Status != "" {
		user.Status = input.Status
		fields.Status = true
	}

	// 角色变更(admin/user);空字符串表示不修改。
	if input.Role != "" {
		role, err := normalizeUserRole(input.Role, user.Role)
		if err != nil {
			return nil, err
		}
		// 防锁死保护：不允许降级系统中最后一个管理员（自我降级已在 handler 层拦截，
		// 此处兜底覆盖跨管理员互降导致零 admin 的场景）。
		if user.Role == RoleAdmin && role == RoleUser {
			if err := s.ensureNotLastAdmin(ctx); err != nil {
				return nil, err
			}
		}
		user.Role = role
		fields.Role = true
	}

	if input.Concurrency != nil {
		user.Concurrency = *input.Concurrency
		fields.Concurrency = true
	}

	if input.RPMLimit != nil {
		user.RPMLimit = *input.RPMLimit
		fields.RPMLimit = true
	}

	if input.AllowedGroups != nil {
		user.AllowedGroups = *input.AllowedGroups
		fields.AllowedGroups = true
	}

	oldRestrictPublicGroups := user.RestrictPublicGroups
	if input.RestrictPublicGroups != nil {
		user.RestrictPublicGroups = *input.RestrictPublicGroups
		fields.RestrictPublicGroups = true
	}

	if err := s.userRepo.Update(ctx, user, fields); err != nil {
		return nil, err
	}

	// 管理员变更该用户的允许分组后，让 API Key 的分组自动跟随，避免 Key 停留在
	// 已不再允许的分组上：那种状态下 Key 列表徽标仍显示旧分组，但请求会被
	// 403 GROUP_NOT_ALLOWED 拦截，用户只能逐个手动改 Key，且很容易漏掉。
	if input.AllowedGroups != nil && !sameInt64Set(user.AllowedGroups, oldAllowedGroups) {
		s.realignUserAPIKeyGroups(ctx, user)
	}

	// 角色变更属权限敏感操作，落审计日志（含操作者），便于事后追溯。
	if user.Role != oldRole {
		logger.LegacyPrintf("service.admin", "audit: user role changed actor_admin_id=%d target_user_id=%d old_role=%s new_role=%s",
			input.ActorAdminID, user.ID, oldRole, user.Role)
	}

	// 同步用户专属分组倍率
	if input.GroupRates != nil && s.userGroupRateRepo != nil {
		if err := s.userGroupRateRepo.SyncUserGroupRates(ctx, user.ID, input.GroupRates); err != nil {
			logger.LegacyPrintf("service.admin", "failed to sync user group rates: user_id=%d err=%v", user.ID, err)
		}
	}

	if s.authCacheInvalidator != nil {
		// RPMLimit 直接参与 billing_cache_service.checkRPM 的三级级联，
		// allowed_groups 参与 API Key 专属分组授权判断；不失效缓存会让修改在一个 L2 TTL 内失去效果。
		if user.Concurrency != oldConcurrency || user.Status != oldStatus || user.Role != oldRole || user.RPMLimit != oldRPMLimit || user.RestrictPublicGroups != oldRestrictPublicGroups || !sameInt64Set(user.AllowedGroups, oldAllowedGroups) {
			s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, user.ID)
		}
	}

	concurrencyDiff := user.Concurrency - oldConcurrency
	if concurrencyDiff != 0 {
		code, err := GenerateRedeemCode()
		if err != nil {
			logger.LegacyPrintf("service.admin", "failed to generate adjustment redeem code: %v", err)
			return user, nil
		}
		adjustmentRecord := &RedeemCode{
			Code:   code,
			Type:   AdjustmentTypeAdminConcurrency,
			Value:  float64(concurrencyDiff),
			Status: StatusUsed,
			UsedBy: &user.ID,
		}
		now := time.Now()
		adjustmentRecord.UsedAt = &now
		if err := s.redeemCodeRepo.Create(ctx, adjustmentRecord); err != nil {
			logger.LegacyPrintf("service.admin", "failed to create concurrency adjustment redeem code: %v", err)
		}
	}

	return user, nil
}

// AdminReviewUserRisk 管理员显式复核放行一个待复核账号：先把该账号被 IP 一致性
// 风控扣掉的余额加回（见 IPPenaltyRefundRepository），再清空风险标记、把 status
// 恢复 active 并登记全量扫描豁免（见 ClearUserRiskReview），随后失效认证缓存与
// 余额缓存，让放行在下一次请求即生效。
//
// 返还只对「真正扣过且尚未返还」的流水生效：没扣过时 SQL 返回 0，不写流水、不动余额。
//
// 豁免只作用于全量扫描：同一批静态证据不会再把账号标回待复核；实时邀请绑定
// 属于新增证据，仍会重新打标，届时账号需再次人工复核。
//
// 这是「风险标记只升不降」规则的唯一显式例外——必须由管理员人工触发，
// 任何自动链路（实时打标 / 全量扫描 / 内容风控解封）都不得调用本方法。
// 清标记走 UserRiskReviewRepository 可选接口 + 类型断言，避免逼所有
// UserRepository 测试桩实现这一管理端专用方法。
func (s *adminServiceImpl) AdminReviewUserRisk(ctx context.Context, userID int64) error {
	if userID <= 0 {
		return infraerrors.BadRequest("INVALID_USER_ID", "user id must be greater than 0")
	}
	if s == nil || s.userRepo == nil {
		return infraerrors.ServiceUnavailable("USER_REPOSITORY_UNAVAILABLE", "user repository is unavailable")
	}
	// 确认用户存在，避免对不存在的 id 返回误导性的成功。
	if _, err := s.userRepo.GetByID(ctx, userID); err != nil {
		return err
	}
	// 先返还再清标记：返还失败时风险标记仍在、按钮不消失，管理员可以重试；
	// 反过来会留下「标记已清但钱没退」且入口跟着消失的死角。
	// 可选接口：测试桩/其它仓储实现未接入时静默跳过，不影响既有复核放行。
	if refundRepo, ok := s.userRepo.(IPPenaltyRefundRepository); ok {
		if _, err := refundRepo.RefundIPBalancePenalties(ctx, userID); err != nil {
			return fmt.Errorf("refund ip balance penalties: %w", err)
		}
	}
	reviewRepo, ok := s.userRepo.(UserRiskReviewRepository)
	if !ok {
		return infraerrors.ServiceUnavailable("RISK_REVIEW_UNAVAILABLE", "risk review is not supported by the user repository")
	}
	if err := reviewRepo.ClearUserRiskReview(ctx, userID); err != nil {
		return err
	}
	// 放行后失效认证缓存（内含 risk_level/status 快照）与余额缓存；失败只记日志，
	// 不回滚已提交的放行。adminServiceImpl 自身即持有这两个 invalidator。
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
	if s.billingCacheService != nil {
		if err := s.billingCacheService.InvalidateUserBalance(ctx, userID); err != nil {
			logger.LegacyPrintf("service.admin", "invalidate user balance cache after risk review failed: user_id=%d err=%v", userID, err)
		}
	}
	return nil
}

// realignUserAPIKeyGroups 让用户的 API Key 分组跟随管理员的允许分组配置。
//
// 规则：
//   - 管理员只指定了一个分组：该用户所有已绑定分组的 Key 统一迁移到该分组，
//     用户无需自己调整；
//   - 管理员指定了多个分组：只迁移那些已不在允许范围内的分组，保留用户在
//     允许范围内自选的 Key 分组；
//   - 管理员清空了允许分组：无法确定迁移目标，跳过并记录日志（需管理员重新指定）。
//
// 与 AdminUpdateAPIKeyGroupID 保持同一套不变量：目标分组必须存在且启用、订阅型
// 分组需要用户持有有效订阅；订阅型分组自身承载用户已购买的订阅，不参与迁移。
// 这里的失败不阻断用户更新本身（与 GroupRates 同步保持一致），但会落日志便于排查。
func (s *adminServiceImpl) realignUserAPIKeyGroups(ctx context.Context, user *User) {
	if s.apiKeyRepo == nil || s.groupRepo == nil || user == nil {
		return
	}
	if len(user.AllowedGroups) == 0 {
		logger.LegacyPrintf("service.admin", "skip api key group realign: user_id=%d has no allowed groups", user.ID)
		return
	}

	target := user.AllowedGroups[0]
	targetGroup, err := s.groupRepo.GetByIDLite(ctx, target)
	if err != nil || targetGroup == nil {
		logger.LegacyPrintf("service.admin", "skip api key group realign: user_id=%d target_group_id=%d not found err=%v", user.ID, target, err)
		return
	}
	if !targetGroup.IsActive() {
		logger.LegacyPrintf("service.admin", "skip api key group realign: user_id=%d target_group_id=%d is not active", user.ID, target)
		return
	}
	if targetGroup.IsSubscriptionType() {
		if s.userSubRepo == nil {
			logger.LegacyPrintf("service.admin", "skip api key group realign: user_id=%d target_group_id=%d subscription repo unavailable", user.ID, target)
			return
		}
		if _, err := s.userSubRepo.GetActiveByUserIDAndGroupID(ctx, user.ID, target); err != nil {
			logger.LegacyPrintf("service.admin", "skip api key group realign: user_id=%d target_group_id=%d requires active subscription err=%v", user.ID, target, err)
			return
		}
	}

	keys, err := s.listUserAPIKeysForDeletion(ctx, user.ID)
	if err != nil {
		logger.LegacyPrintf("service.admin", "failed to list api keys for group realign: user_id=%d err=%v", user.ID, err)
		return
	}

	// 收集 Key 上实际出现的分组（去重），避免按 Key 逐条写库。
	present := make(map[int64]struct{}, len(keys))
	for i := range keys {
		if keys[i].GroupID != nil {
			present[*keys[i].GroupID] = struct{}{}
		}
	}
	if len(present) == 0 {
		return
	}

	for groupID := range present {
		if groupID == target {
			continue
		}
		srcGroup, err := s.groupRepo.GetByIDLite(ctx, groupID)
		if err != nil || srcGroup == nil {
			// 无法判断来源分组，保守跳过；该 Key 的可用性仍由网关原有逻辑判断。
			logger.LegacyPrintf("service.admin", "skip api key group realign: user_id=%d source_group_id=%d not found err=%v", user.ID, groupID, err)
			continue
		}
		// 订阅型分组承载用户已购买的订阅，不随管理员调整分组而迁移。
		if srcGroup.IsSubscriptionType() {
			continue
		}
		// 多分组场景下，仍在允许范围内且可绑定的分组保留用户的选择。
		if len(user.AllowedGroups) > 1 &&
			!strings.EqualFold(srcGroup.Status, "deleted") &&
			user.CanBindGroup(srcGroup.ID, srcGroup.IsExclusive) {
			continue
		}
		affected, err := s.apiKeyRepo.UpdateGroupIDByUserAndGroup(ctx, user.ID, groupID, target)
		if err != nil {
			logger.LegacyPrintf("service.admin", "failed to realign api key group: user_id=%d from_group_id=%d to_group_id=%d err=%v",
				user.ID, groupID, target, err)
			continue
		}
		if affected > 0 {
			logger.LegacyPrintf("service.admin", "audit: realigned api key groups user_id=%d from_group_id=%d to_group_id=%d affected=%d",
				user.ID, groupID, target, affected)
		}
	}
}

func sameInt64Set(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	if len(a) == 0 {
		return true
	}
	counts := make(map[int64]int, len(a))
	for _, v := range a {
		counts[v]++
	}
	for _, v := range b {
		if counts[v] == 0 {
			return false
		}
		counts[v]--
	}
	return true
}

func (s *adminServiceImpl) DeleteUser(ctx context.Context, id int64) error {
	// Protect admin users: cannot delete admin accounts
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if user.Role == "admin" {
		return infraerrors.BadRequest("ADMIN_DELETE_FORBIDDEN", "cannot delete admin user; change the role to user first")
	}

	apiKeys, err := s.listUserAPIKeysForDeletion(ctx, id)
	if err != nil {
		return err
	}

	if s.entClient != nil {
		tx, err := s.entClient.Tx(ctx)
		if err != nil {
			return err
		}
		defer func() { _ = tx.Rollback() }()

		opCtx := dbent.NewTxContext(ctx, tx)
		if err := s.deleteUserWithAPIKeys(opCtx, id, apiKeys); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	} else {
		if err := s.deleteUserWithAPIKeys(ctx, id, apiKeys); err != nil {
			return err
		}
	}

	if s.authCacheInvalidator != nil {
		for _, key := range apiKeys {
			if keyValue := strings.TrimSpace(key.Key); keyValue != "" {
				s.authCacheInvalidator.InvalidateAuthCacheByKey(ctx, keyValue)
			}
		}
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, id)
	}
	return nil
}

func (s *adminServiceImpl) listUserAPIKeysForDeletion(ctx context.Context, userID int64) ([]APIKey, error) {
	if s.apiKeyRepo == nil {
		return nil, nil
	}

	const pageSize = 1000
	keys := make([]APIKey, 0)
	for page := 1; ; page++ {
		batch, result, err := s.apiKeyRepo.ListByUserID(ctx, userID, pagination.PaginationParams{
			Page:      page,
			PageSize:  pageSize,
			SortBy:    "id",
			SortOrder: pagination.SortOrderAsc,
		}, APIKeyListFilters{})
		if err != nil {
			return nil, fmt.Errorf("list user api keys: %w", err)
		}
		keys = append(keys, batch...)
		if len(batch) == 0 || len(batch) < pageSize || result == nil || int64(len(keys)) >= result.Total {
			break
		}
	}
	return keys, nil
}

func (s *adminServiceImpl) deleteUserWithAPIKeys(ctx context.Context, userID int64, apiKeys []APIKey) error {
	if s.apiKeyRepo != nil {
		for _, key := range apiKeys {
			if key.ID <= 0 {
				continue
			}
			if err := s.apiKeyRepo.DeleteWithAudit(ctx, key.ID); err != nil {
				logger.LegacyPrintf("service.admin", "delete user api key failed: user_id=%d api_key_id=%d err=%v", userID, key.ID, err)
				return fmt.Errorf("delete user api key %d: %w", key.ID, err)
			}
		}
	}

	if err := s.userRepo.Delete(ctx, userID); err != nil {
		logger.LegacyPrintf("service.admin", "delete user failed: user_id=%d err=%v", userID, err)
		return err
	}
	return nil
}

func (s *adminServiceImpl) BatchUpdateConcurrency(ctx context.Context, userIDs []int64, value int, mode string) (int, error) {
	cleaned := make([]int64, 0, len(userIDs))
	for _, uid := range userIDs {
		if uid > 0 {
			cleaned = append(cleaned, uid)
		}
	}
	if len(cleaned) == 0 {
		return 0, nil
	}

	var affected int
	var err error
	switch mode {
	case "set":
		affected, err = s.userRepo.BatchSetConcurrency(ctx, cleaned, value)
	case "add":
		affected, err = s.userRepo.BatchAddConcurrency(ctx, cleaned, value)
	default:
		return 0, errors.New("invalid mode: must be 'set' or 'add'")
	}
	if err != nil {
		return 0, err
	}

	if s.authCacheInvalidator != nil {
		for _, uid := range cleaned {
			s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, uid)
		}
	}
	return affected, nil
}

func (s *adminServiceImpl) BatchUpdateLimits(ctx context.Context, userIDs []int64, concurrency, rpmLimit *int) (int, error) {
	if concurrency == nil && rpmLimit == nil {
		return 0, fmt.Errorf("at least one of concurrency or rpm_limit is required")
	}

	cleaned := make([]int64, 0, len(userIDs))
	seen := make(map[int64]struct{}, len(userIDs))
	for _, userID := range userIDs {
		if userID <= 0 {
			continue
		}
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		cleaned = append(cleaned, userID)
	}
	if len(cleaned) == 0 {
		return 0, nil
	}

	affected, err := s.userRepo.BatchUpdateLimits(ctx, cleaned, concurrency, rpmLimit)
	if err != nil {
		return 0, err
	}
	if s.authCacheInvalidator != nil {
		for _, userID := range cleaned {
			s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
		}
	}
	return affected, nil
}

// IncrementUserLookedCount 把管理员「看过」该账号的次数加一，返回累加后的值。
// 该标记只用于管理端展示，不改变任何业务行为。
func (s *adminServiceImpl) IncrementUserLookedCount(ctx context.Context, userID int64) (int, error) {
	repo, ok := s.userRepo.(UserFollowUpMarkRepository)
	if !ok {
		return 0, fmt.Errorf("user repository does not support follow-up marks")
	}
	return repo.IncrementLookedCount(ctx, userID)
}

// SetUserTicketFlag 设置该账号的「提过工单」标记。
func (s *adminServiceImpl) SetUserTicketFlag(ctx context.Context, userID int64, flagged bool) error {
	repo, ok := s.userRepo.(UserFollowUpMarkRepository)
	if !ok {
		return fmt.Errorf("user repository does not support follow-up marks")
	}
	return repo.SetTicketFlag(ctx, userID, flagged)
}

func (s *adminServiceImpl) UpdateUserBalance(ctx context.Context, userID int64, balance float64, operation string, notes string) (*User, error) {
	// 余额调整必须走原子接口：先读后整行写回会把并发的计费扣款覆盖掉。
	var (
		change BalanceChange
		err    error
	)
	applyBalance := func(applyCtx context.Context) (BalanceChange, error) {
		switch operation {
		case "set":
			return s.userRepo.SetBalance(applyCtx, userID, balance)
		case "add":
			return s.userRepo.AdjustBalance(applyCtx, userID, balance)
		case "subtract":
			return s.userRepo.AdjustBalance(applyCtx, userID, -balance)
		default:
			return BalanceChange{}, fmt.Errorf("unsupported balance operation: %q", operation)
		}
	}
	if operation != "set" && operation != "add" && operation != "subtract" {
		return nil, fmt.Errorf("unsupported balance operation: %q", operation)
	}
	change, err = s.applyBalanceWithAdjustmentRecord(ctx, userID, notes, applyBalance)
	if errors.Is(err, ErrBalanceNegative) {
		return nil, fmt.Errorf("balance cannot be negative, current balance: %.2f, requested operation would result in: %.2f", change.Old, change.New)
	}
	if err != nil {
		return nil, err
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	balanceDiff := change.New - change.Old
	if s.authCacheInvalidator != nil && balanceDiff != 0 {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
	s.tryAccrueAffiliateRebateForAdminRecharge(ctx, userID, operation, balance)

	if s.billingCacheService != nil {
		go func() {
			cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := s.billingCacheService.InvalidateUserBalance(cacheCtx, userID); err != nil {
				logger.LegacyPrintf("service.admin", "invalidate user balance cache failed: user_id=%d err=%v", userID, err)
			}
		}()
	}

	return user, nil
}

// balanceFloorReducer 是 userRepository 提供的可选原子能力。
// 刻意用窄接口 + 类型断言，不往 UserRepository 大接口加方法，避免所有测试桩被迫实现。
type balanceFloorReducer interface {
	ReduceBalanceToFloor(ctx context.Context, id int64, floor float64) (BalanceChange, error)
}

// ApplyIPBalancePenalty IP 一致性风控的「减额度」处罚：把余额原子降到 floor，
// 并按 notes 写调账流水（与后台「- 退款」完全同款，前端显示「余额扣除（管理员）」）。
//
// 幂等：条件 UPDATE 只在 balance > floor 时命中；余额已达标时 Old==New，
// applyBalanceWithAdjustmentRecord 跳过流水，返回 applied=false，调用方据此不发信。
// 返回用户邮箱供通知邮件使用；取邮箱失败只记日志，不算处罚失败（否则会释放幂等锁，
// 而余额和流水其实已经落库了）。
func (s *adminServiceImpl) ApplyIPBalancePenalty(ctx context.Context, userID int64, floor float64, notes string) (string, bool, error) {
	reducer, ok := s.userRepo.(balanceFloorReducer)
	if !ok {
		return "", false, fmt.Errorf("balance floor reducer unavailable")
	}
	change, err := s.applyBalanceWithAdjustmentRecord(ctx, userID, notes, func(applyCtx context.Context) (BalanceChange, error) {
		return reducer.ReduceBalanceToFloor(applyCtx, userID, floor)
	})
	if err != nil {
		return "", false, err
	}
	if change.New == change.Old {
		return "", false, nil
	}

	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
	if s.billingCacheService != nil {
		go func() {
			cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := s.billingCacheService.InvalidateUserBalance(cacheCtx, userID); err != nil {
				logger.LegacyPrintf("service.admin", "invalidate user balance cache failed after ip penalty: user_id=%d err=%v", userID, err)
			}
		}()
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		logger.LegacyPrintf("service.admin", "ip penalty applied but failed to load user email: user_id=%d err=%v", userID, err)
		return "", true, nil
	}
	return user.Email, true, nil
}

// applyBalanceWithAdjustmentRecord 在单个事务内完成余额变更与调账流水写入，
// 避免出现“余额已变、流水丢失”的对账缺口。entClient 未注入时（如单测）退化为顺序执行。
func (s *adminServiceImpl) applyBalanceWithAdjustmentRecord(ctx context.Context, userID int64, notes string, apply func(context.Context) (BalanceChange, error)) (BalanceChange, error) {
	if s.entClient == nil {
		change, err := apply(ctx)
		if err != nil || change.New == change.Old {
			return change, err
		}
		if err := s.createBalanceAdjustmentRecord(ctx, userID, change.New-change.Old, notes); err != nil {
			return change, err
		}
		return change, nil
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return BalanceChange{}, err
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)

	change, err := apply(txCtx)
	if err != nil {
		return change, err
	}
	if change.New == change.Old {
		if err := tx.Commit(); err != nil {
			return BalanceChange{}, err
		}
		return change, nil
	}
	if err := s.createBalanceAdjustmentRecord(txCtx, userID, change.New-change.Old, notes); err != nil {
		return change, err
	}
	if err := tx.Commit(); err != nil {
		return BalanceChange{}, err
	}
	return change, nil
}

func (s *adminServiceImpl) createBalanceAdjustmentRecord(ctx context.Context, userID int64, diff float64, notes string) error {
	code, err := GenerateRedeemCode()
	if err != nil {
		return fmt.Errorf("generate adjustment redeem code: %w", err)
	}
	now := time.Now()
	adjustmentRecord := &RedeemCode{
		Code:   code,
		Type:   AdjustmentTypeAdminBalance,
		Value:  diff,
		Status: StatusUsed,
		UsedBy: &userID,
		Notes:  notes,
		UsedAt: &now,
	}
	if err := s.redeemCodeRepo.Create(ctx, adjustmentRecord); err != nil {
		return fmt.Errorf("create balance adjustment redeem code: %w", err)
	}
	return nil
}

func (s *adminServiceImpl) tryAccrueAffiliateRebateForAdminRecharge(ctx context.Context, userID int64, operation string, amount float64) {
	if operation != "add" || amount <= 0 || s.settingService == nil || s.affiliateService == nil {
		return
	}
	if !s.settingService.IsAffiliateAdminRechargeEnabled(ctx) {
		return
	}

	rebate, err := s.affiliateService.AccrueInviteRebate(ctx, userID, amount)
	if err != nil {
		logger.LegacyPrintf("service.admin", "affiliate rebate failed for admin recharge: user_id=%d amount=%.8f err=%v", userID, amount, err)
		return
	}
	if rebate > 0 {
		logger.LegacyPrintf("service.admin", "affiliate rebate accrued for admin recharge: user_id=%d amount=%.8f rebate=%.8f", userID, amount, rebate)
	}
}

func (s *adminServiceImpl) GetUserAPIKeys(ctx context.Context, userID int64, page, pageSize int, sortBy, sortOrder string) ([]APIKey, int64, error) {
	params := pagination.PaginationParams{Page: page, PageSize: pageSize, SortBy: sortBy, SortOrder: sortOrder}
	keys, result, err := s.apiKeyRepo.ListByUserID(ctx, userID, params, APIKeyListFilters{})
	if err != nil {
		return nil, 0, err
	}
	return keys, result.Total, nil
}

func (s *adminServiceImpl) GetUserRPMStatus(ctx context.Context, userID int64) (*UserRPMStatus, error) {
	if s.userRPMCache == nil {
		return nil, ErrRPMStatusUnavailable
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	userRPMUsed, err := s.userRPMCache.GetUserRPM(ctx, userID)
	if err != nil {
		logger.LegacyPrintf("service.admin", "failed to get user rpm: user_id=%d err=%v", userID, err)
	}

	keys, _, err := s.GetUserAPIKeys(ctx, userID, 1, 1000, "", "")
	if err != nil {
		return nil, err
	}

	groupIDSet := make(map[int64]struct{})
	for _, key := range keys {
		if key.GroupID != nil && *key.GroupID > 0 {
			groupIDSet[*key.GroupID] = struct{}{}
		}
	}

	groupIDs := make([]int64, 0, len(groupIDSet))
	for groupID := range groupIDSet {
		groupIDs = append(groupIDs, groupID)
	}
	sort.Slice(groupIDs, func(i, j int) bool { return groupIDs[i] < groupIDs[j] })

	var perGroup []UserGroupRPMStatus
	for _, groupID := range groupIDs {
		used, getErr := s.userRPMCache.GetUserGroupRPM(ctx, userID, groupID)
		if getErr != nil {
			logger.LegacyPrintf("service.admin", "failed to get user group rpm: user_id=%d group_id=%d err=%v", userID, groupID, getErr)
		}

		entry := UserGroupRPMStatus{
			GroupID: groupID,
			Used:    used,
		}

		if s.groupRepo != nil {
			if group, groupErr := s.groupRepo.GetByIDLite(ctx, groupID); groupErr == nil && group != nil {
				entry.GroupName = group.Name
				entry.Limit = group.RPMLimit
				entry.Source = "group"
			} else if groupErr != nil {
				logger.LegacyPrintf("service.admin", "failed to get group rpm status metadata: group_id=%d err=%v", groupID, groupErr)
			}
		}

		if s.userGroupRateRepo != nil {
			override, overrideErr := s.userGroupRateRepo.GetRPMOverrideByUserAndGroup(ctx, userID, groupID)
			if overrideErr != nil {
				logger.LegacyPrintf("service.admin", "failed to get rpm override: user_id=%d group_id=%d err=%v", userID, groupID, overrideErr)
			} else if override != nil {
				entry.Limit = *override
				entry.Source = "override"
			}
		}

		perGroup = append(perGroup, entry)
	}

	return &UserRPMStatus{
		UserRPMUsed:  userRPMUsed,
		UserRPMLimit: user.RPMLimit,
		PerGroup:     perGroup,
	}, nil
}

func (s *adminServiceImpl) GetUserUsageStats(ctx context.Context, userID int64, period string) (any, error) {
	// Return mock data for now
	return map[string]any{
		"period":          period,
		"total_requests":  0,
		"total_cost":      0.0,
		"total_tokens":    0,
		"avg_duration_ms": 0,
	}, nil
}

// GetUserBalanceHistory returns paginated balance/concurrency change records for a user.
func (s *adminServiceImpl) GetUserBalanceHistory(ctx context.Context, userID int64, page, pageSize int, codeType string) ([]RedeemCode, int64, float64, error) {
	params := pagination.PaginationParams{Page: page, PageSize: pageSize}
	if codeType == RedeemTypeAffiliateBalance {
		codes, total, err := s.listAffiliateBalanceHistory(ctx, userID, params)
		if err != nil {
			return nil, 0, 0, err
		}
		totalRecharged, err := s.redeemCodeRepo.SumPositiveBalanceByUser(ctx, userID)
		if err != nil {
			return nil, 0, 0, err
		}
		return codes, total, totalRecharged, nil
	}

	if codeType == "" {
		return s.getAllUserBalanceHistory(ctx, userID, params)
	}

	codes, result, err := s.redeemCodeRepo.ListByUserPaginated(ctx, userID, params, codeType)
	if err != nil {
		return nil, 0, 0, err
	}
	total := result.Total
	// Aggregate total recharged amount (only once, regardless of type filter)
	totalRecharged, err := s.redeemCodeRepo.SumPositiveBalanceByUser(ctx, userID)
	if err != nil {
		return nil, 0, 0, err
	}
	return codes, total, totalRecharged, nil
}

func (s *adminServiceImpl) getAllUserBalanceHistory(ctx context.Context, userID int64, params pagination.PaginationParams) ([]RedeemCode, int64, float64, error) {
	needed := params.Offset() + params.Limit()
	if needed < params.Limit() {
		needed = params.Limit()
	}

	redeemCodes, redeemTotal, err := s.listRedeemBalanceHistoryForMerge(ctx, userID, needed)
	if err != nil {
		return nil, 0, 0, err
	}
	affiliateCodes, affiliateTotal, err := s.listAffiliateBalanceHistoryForMerge(ctx, userID, needed)
	if err != nil {
		return nil, 0, 0, err
	}
	codes := mergeBalanceHistoryCodes(redeemCodes, affiliateCodes, params)

	totalRecharged, err := s.redeemCodeRepo.SumPositiveBalanceByUser(ctx, userID)
	if err != nil {
		return nil, 0, 0, err
	}
	return codes, redeemTotal + affiliateTotal, totalRecharged, nil
}

func (s *adminServiceImpl) listRedeemBalanceHistoryForMerge(ctx context.Context, userID int64, needed int) ([]RedeemCode, int64, error) {
	if needed <= 0 {
		return nil, 0, nil
	}

	var (
		out   []RedeemCode
		total int64
	)
	for page := 1; len(out) < needed; page++ {
		params := pagination.PaginationParams{Page: page, PageSize: 1000}
		codes, result, err := s.redeemCodeRepo.ListByUserPaginated(ctx, userID, params, "")
		if err != nil {
			return nil, 0, err
		}
		if result != nil {
			total = result.Total
		}
		out = append(out, codes...)
		if len(codes) < params.Limit() || int64(len(out)) >= total {
			break
		}
	}
	if len(out) > needed {
		out = out[:needed]
	}
	return out, total, nil
}

func (s *adminServiceImpl) listAffiliateBalanceHistoryForMerge(ctx context.Context, userID int64, needed int) ([]RedeemCode, int64, error) {
	if needed <= 0 {
		return nil, 0, nil
	}

	var (
		out   []RedeemCode
		total int64
	)
	for page := 1; len(out) < needed; page++ {
		params := pagination.PaginationParams{Page: page, PageSize: 1000}
		codes, currentTotal, err := s.listAffiliateBalanceHistory(ctx, userID, params)
		if err != nil {
			return nil, 0, err
		}
		total = currentTotal
		out = append(out, codes...)
		if len(codes) < params.Limit() || int64(len(out)) >= total {
			break
		}
	}
	if len(out) > needed {
		out = out[:needed]
	}
	return out, total, nil
}

func (s *adminServiceImpl) listAffiliateBalanceHistory(ctx context.Context, userID int64, params pagination.PaginationParams) ([]RedeemCode, int64, error) {
	if s == nil || s.entClient == nil || userID <= 0 {
		return nil, 0, nil
	}

	rows, err := s.entClient.QueryContext(ctx, `
SELECT id,
       amount::double precision,
       created_at
FROM user_affiliate_ledger
WHERE user_id = $1
  AND action = 'transfer'
ORDER BY created_at DESC, id DESC
OFFSET $2
LIMIT $3`, userID, params.Offset(), params.Limit())
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	codes := make([]RedeemCode, 0, params.Limit())
	for rows.Next() {
		var id int64
		var amount float64
		var createdAt time.Time
		if err := rows.Scan(&id, &amount, &createdAt); err != nil {
			return nil, 0, err
		}
		usedBy := userID
		usedAt := createdAt
		codes = append(codes, RedeemCode{
			ID:        -id,
			Code:      fmt.Sprintf("AFF-%d", id),
			Type:      RedeemTypeAffiliateBalance,
			Value:     amount,
			Status:    StatusUsed,
			UsedBy:    &usedBy,
			UsedAt:    &usedAt,
			CreatedAt: createdAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	total, err := countAffiliateBalanceHistory(ctx, s.entClient, userID)
	if err != nil {
		return nil, 0, err
	}
	return codes, total, nil
}

func countAffiliateBalanceHistory(ctx context.Context, client *dbent.Client, userID int64) (int64, error) {
	rows, err := client.QueryContext(ctx, `
SELECT COUNT(*)
FROM user_affiliate_ledger
WHERE user_id = $1
  AND action = 'transfer'`, userID)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()

	var total sql.NullInt64
	if rows.Next() {
		if err := rows.Scan(&total); err != nil {
			return 0, err
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if !total.Valid {
		return 0, nil
	}
	return total.Int64, nil
}

func mergeBalanceHistoryCodes(redeemCodes, affiliateCodes []RedeemCode, params pagination.PaginationParams) []RedeemCode {
	combined := append(append([]RedeemCode{}, redeemCodes...), affiliateCodes...)
	sort.SliceStable(combined, func(i, j int) bool {
		return redeemCodeHistoryTime(combined[i]).After(redeemCodeHistoryTime(combined[j]))
	})
	offset := params.Offset()
	if offset >= len(combined) {
		return []RedeemCode{}
	}
	end := offset + params.Limit()
	if end > len(combined) {
		end = len(combined)
	}
	return combined[offset:end]
}

func redeemCodeHistoryTime(code RedeemCode) time.Time {
	if code.UsedAt != nil {
		return *code.UsedAt
	}
	return code.CreatedAt
}

func (s *adminServiceImpl) BindUserAuthIdentity(ctx context.Context, userID int64, input AdminBindAuthIdentityInput) (*AdminBoundAuthIdentity, error) {
	if userID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_INPUT", "user_id must be greater than 0")
	}
	if s == nil || s.entClient == nil || s.userRepo == nil {
		return nil, infraerrors.InternalServer("ADMIN_AUTH_IDENTITY_BIND_UNAVAILABLE", "auth identity binding service is unavailable")
	}
	if _, err := s.userRepo.GetByID(ctx, userID); err != nil {
		return nil, err
	}

	providerType := normalizeAdminAuthIdentityProviderType(input.ProviderType)
	providerKey := strings.TrimSpace(input.ProviderKey)
	providerSubject := strings.TrimSpace(input.ProviderSubject)
	if providerType == "" {
		return nil, infraerrors.BadRequest("INVALID_INPUT", "provider_type must be one of email, linuxdo, oidc, wechat, or dingtalk")
	}
	if providerKey == "" || providerSubject == "" {
		return nil, infraerrors.BadRequest("INVALID_INPUT", "provider_type, provider_key, and provider_subject are required")
	}
	canonicalProviderKey := canonicalAdminAuthIdentityProviderKey(providerType, "", providerKey)
	compatibleProviderKeys := compatibleAdminAuthIdentityProviderKeys(providerType, providerKey)

	var issuer *string
	if input.Issuer != nil {
		trimmed := strings.TrimSpace(*input.Issuer)
		if trimmed != "" {
			issuer = &trimmed
		}
	}

	channelInput := normalizeAdminBindChannelInput(input.Channel)
	if input.Channel != nil && channelInput == nil {
		return nil, infraerrors.BadRequest("INVALID_INPUT", "channel, channel_app_id, and channel_subject are required when channel binding is provided")
	}

	verifiedAt := time.Now().UTC()
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, infraerrors.InternalServer("ADMIN_AUTH_IDENTITY_BIND_TX_FAILED", "failed to start auth identity bind transaction").WithCause(err)
	}
	defer func() { _ = tx.Rollback() }()

	identityRecords, err := tx.AuthIdentity.Query().
		Where(
			authidentity.ProviderTypeEQ(providerType),
			authidentity.ProviderKeyIn(compatibleProviderKeys...),
			authidentity.ProviderSubjectEQ(providerSubject),
		).
		All(ctx)
	if err != nil {
		return nil, infraerrors.InternalServer("ADMIN_AUTH_IDENTITY_BIND_LOOKUP_FAILED", "failed to inspect auth identity ownership").WithCause(err)
	}
	if hasAdminAuthIdentityOwnershipConflict(identityRecords, userID) {
		return nil, infraerrors.Conflict("AUTH_IDENTITY_OWNERSHIP_CONFLICT", "auth identity already belongs to another user")
	}
	identity := selectOwnedAdminAuthIdentity(identityRecords, userID)

	if identity == nil {
		create := tx.AuthIdentity.Create().
			SetUserID(userID).
			SetProviderType(providerType).
			SetProviderKey(canonicalProviderKey).
			SetProviderSubject(providerSubject).
			SetVerifiedAt(verifiedAt)
		if issuer != nil {
			create = create.SetIssuer(*issuer)
		}
		if input.Metadata != nil {
			create = create.SetMetadata(cloneAdminAuthIdentityMetadata(input.Metadata))
		}
		identity, err = create.Save(ctx)
		if err != nil {
			return nil, infraerrors.InternalServer("ADMIN_AUTH_IDENTITY_BIND_SAVE_FAILED", "failed to save auth identity").WithCause(err)
		}
	} else {
		update := tx.AuthIdentity.UpdateOneID(identity.ID).
			SetVerifiedAt(verifiedAt).
			SetProviderKey(canonicalProviderKey)
		if issuer != nil {
			update = update.SetIssuer(*issuer)
		}
		if input.Metadata != nil {
			update = update.SetMetadata(cloneAdminAuthIdentityMetadata(input.Metadata))
		}
		identity, err = update.Save(ctx)
		if err != nil {
			return nil, infraerrors.InternalServer("ADMIN_AUTH_IDENTITY_BIND_SAVE_FAILED", "failed to save auth identity").WithCause(err)
		}
	}

	var channel *dbent.AuthIdentityChannel
	if channelInput != nil {
		channelRecords, err := tx.AuthIdentityChannel.Query().
			Where(
				authidentitychannel.ProviderTypeEQ(providerType),
				authidentitychannel.ProviderKeyIn(compatibleProviderKeys...),
				authidentitychannel.ChannelEQ(channelInput.Channel),
				authidentitychannel.ChannelAppIDEQ(channelInput.ChannelAppID),
				authidentitychannel.ChannelSubjectEQ(channelInput.ChannelSubject),
			).
			WithIdentity().
			All(ctx)
		if err != nil {
			return nil, infraerrors.InternalServer("ADMIN_AUTH_IDENTITY_CHANNEL_LOOKUP_FAILED", "failed to inspect auth identity channel ownership").WithCause(err)
		}
		if hasAdminAuthIdentityChannelOwnershipConflict(channelRecords, userID) {
			return nil, infraerrors.Conflict("AUTH_IDENTITY_CHANNEL_OWNERSHIP_CONFLICT", "auth identity channel already belongs to another user")
		}
		channel = selectOwnedAdminAuthIdentityChannel(channelRecords, userID)
		if channel == nil {
			create := tx.AuthIdentityChannel.Create().
				SetIdentityID(identity.ID).
				SetProviderType(providerType).
				SetProviderKey(canonicalProviderKey).
				SetChannel(channelInput.Channel).
				SetChannelAppID(channelInput.ChannelAppID).
				SetChannelSubject(channelInput.ChannelSubject)
			if channelInput.Metadata != nil {
				create = create.SetMetadata(cloneAdminAuthIdentityMetadata(channelInput.Metadata))
			}
			channel, err = create.Save(ctx)
			if err != nil {
				return nil, infraerrors.InternalServer("ADMIN_AUTH_IDENTITY_CHANNEL_SAVE_FAILED", "failed to save auth identity channel").WithCause(err)
			}
		} else {
			update := tx.AuthIdentityChannel.UpdateOneID(channel.ID).
				SetIdentityID(identity.ID).
				SetProviderKey(canonicalProviderKey)
			if channelInput.Metadata != nil {
				update = update.SetMetadata(cloneAdminAuthIdentityMetadata(channelInput.Metadata))
			}
			channel, err = update.Save(ctx)
			if err != nil {
				return nil, infraerrors.InternalServer("ADMIN_AUTH_IDENTITY_CHANNEL_SAVE_FAILED", "failed to save auth identity channel").WithCause(err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, infraerrors.InternalServer("ADMIN_AUTH_IDENTITY_BIND_COMMIT_FAILED", "failed to commit auth identity bind").WithCause(err)
	}
	return buildAdminBoundAuthIdentity(identity, channel), nil
}

func compatibleAdminAuthIdentityProviderKeys(providerType, providerKey string) []string {
	providerType = strings.TrimSpace(strings.ToLower(providerType))
	providerKey = strings.TrimSpace(providerKey)
	if providerKey == "" {
		return []string{providerKey}
	}
	if providerType != "wechat" {
		return []string{providerKey}
	}

	keys := []string{providerKey}
	if !strings.EqualFold(providerKey, "wechat-main") {
		keys = append(keys, "wechat-main")
	}
	if !strings.EqualFold(providerKey, "wechat") {
		keys = append(keys, "wechat")
	}
	return keys
}

func canonicalAdminAuthIdentityProviderKey(providerType, existingKey, requestedKey string) string {
	providerType = strings.TrimSpace(strings.ToLower(providerType))
	existingKey = strings.TrimSpace(existingKey)
	requestedKey = strings.TrimSpace(requestedKey)
	if providerType != "wechat" {
		if requestedKey != "" {
			return requestedKey
		}
		return existingKey
	}
	if strings.EqualFold(existingKey, "wechat") || strings.EqualFold(existingKey, "wechat-main") || strings.EqualFold(requestedKey, "wechat-main") {
		return "wechat-main"
	}
	if requestedKey != "" {
		return requestedKey
	}
	return existingKey
}

func adminAuthIdentityProviderKeyRank(providerType, providerKey string) int {
	providerType = strings.TrimSpace(strings.ToLower(providerType))
	providerKey = strings.TrimSpace(providerKey)
	if providerType != "wechat" {
		return 0
	}
	switch {
	case strings.EqualFold(providerKey, "wechat-main"):
		return 0
	case strings.EqualFold(providerKey, "wechat"):
		return 2
	default:
		return 1
	}
}

func selectOwnedAdminAuthIdentity(records []*dbent.AuthIdentity, userID int64) *dbent.AuthIdentity {
	var selected *dbent.AuthIdentity
	for _, record := range records {
		if record.UserID != userID {
			continue
		}
		if selected == nil || adminAuthIdentityProviderKeyRank(record.ProviderType, record.ProviderKey) < adminAuthIdentityProviderKeyRank(selected.ProviderType, selected.ProviderKey) {
			selected = record
		}
	}
	return selected
}

func hasAdminAuthIdentityOwnershipConflict(records []*dbent.AuthIdentity, userID int64) bool {
	for _, record := range records {
		if record.UserID != userID {
			return true
		}
	}
	return false
}

func selectOwnedAdminAuthIdentityChannel(records []*dbent.AuthIdentityChannel, userID int64) *dbent.AuthIdentityChannel {
	var selected *dbent.AuthIdentityChannel
	for _, record := range records {
		if record.Edges.Identity == nil || record.Edges.Identity.UserID != userID {
			continue
		}
		if selected == nil || adminAuthIdentityProviderKeyRank(record.ProviderType, record.ProviderKey) < adminAuthIdentityProviderKeyRank(selected.ProviderType, selected.ProviderKey) {
			selected = record
		}
	}
	return selected
}

func hasAdminAuthIdentityChannelOwnershipConflict(records []*dbent.AuthIdentityChannel, userID int64) bool {
	for _, record := range records {
		if record.Edges.Identity != nil && record.Edges.Identity.UserID != userID {
			return true
		}
	}
	return false
}

func normalizeAdminBindChannelInput(input *AdminBindAuthIdentityChannelInput) *AdminBindAuthIdentityChannelInput {
	if input == nil {
		return nil
	}
	channel := &AdminBindAuthIdentityChannelInput{
		Channel:        strings.TrimSpace(input.Channel),
		ChannelAppID:   strings.TrimSpace(input.ChannelAppID),
		ChannelSubject: strings.TrimSpace(input.ChannelSubject),
		Metadata:       cloneAdminAuthIdentityMetadata(input.Metadata),
	}
	if channel.Channel == "" || channel.ChannelAppID == "" || channel.ChannelSubject == "" {
		return nil
	}
	return channel
}

func normalizeAdminAuthIdentityProviderType(input string) string {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "email":
		return "email"
	case "linuxdo":
		return "linuxdo"
	case "oidc":
		return "oidc"
	case "wechat":
		return "wechat"
	case "dingtalk":
		return "dingtalk"
	default:
		return ""
	}
}

func buildAdminBoundAuthIdentity(identity *dbent.AuthIdentity, channel *dbent.AuthIdentityChannel) *AdminBoundAuthIdentity {
	if identity == nil {
		return nil
	}
	result := &AdminBoundAuthIdentity{
		UserID:          identity.UserID,
		ProviderType:    strings.TrimSpace(identity.ProviderType),
		ProviderKey:     strings.TrimSpace(identity.ProviderKey),
		ProviderSubject: strings.TrimSpace(identity.ProviderSubject),
		VerifiedAt:      identity.VerifiedAt,
		Issuer:          identity.Issuer,
		Metadata:        cloneAdminAuthIdentityMetadata(identity.Metadata),
		CreatedAt:       identity.CreatedAt,
		UpdatedAt:       identity.UpdatedAt,
	}
	if channel != nil {
		result.Channel = &AdminBoundAuthIdentityChannel{
			Channel:        strings.TrimSpace(channel.Channel),
			ChannelAppID:   strings.TrimSpace(channel.ChannelAppID),
			ChannelSubject: strings.TrimSpace(channel.ChannelSubject),
			Metadata:       cloneAdminAuthIdentityMetadata(channel.Metadata),
			CreatedAt:      channel.CreatedAt,
			UpdatedAt:      channel.UpdatedAt,
		}
	}
	return result
}

func cloneAdminAuthIdentityMetadata(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	if len(input) == 0 {
		return map[string]any{}
	}
	data, err := json.Marshal(input)
	if err != nil {
		out := make(map[string]any, len(input))
		for key, value := range input {
			out[key] = value
		}
		return out
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		out = make(map[string]any, len(input))
		for key, value := range input {
			out[key] = value
		}
	}
	return out
}

// Redeem code management implementations
func (s *adminServiceImpl) ListRedeemCodes(ctx context.Context, page, pageSize int, codeType, status, search string, sortBy, sortOrder string) ([]RedeemCode, int64, error) {
	params := pagination.PaginationParams{Page: page, PageSize: pageSize, SortBy: sortBy, SortOrder: sortOrder}
	codes, result, err := s.redeemCodeRepo.ListWithFilters(ctx, params, codeType, status, search)
	if err != nil {
		return nil, 0, err
	}
	return codes, result.Total, nil
}

func (s *adminServiceImpl) GetRedeemCode(ctx context.Context, id int64) (*RedeemCode, error) {
	return s.redeemCodeRepo.GetByID(ctx, id)
}

func (s *adminServiceImpl) GenerateRedeemCodes(ctx context.Context, input *GenerateRedeemCodesInput) ([]RedeemCode, error) {
	if input.ExpiresAt != nil && !input.ExpiresAt.After(time.Now()) {
		return nil, ErrRedeemCodeExpired
	}

	// 如果是订阅类型，验证必须有 GroupID
	if input.Type == RedeemTypeSubscription {
		if input.GroupID == nil {
			return nil, errors.New("group_id is required for subscription type")
		}
		// 验证分组存在且为订阅类型
		group, err := s.groupRepo.GetByID(ctx, *input.GroupID)
		if err != nil {
			return nil, fmt.Errorf("group not found: %w", err)
		}
		if !group.IsSubscriptionType() {
			return nil, errors.New("group must be subscription type")
		}
	}

	codes := make([]RedeemCode, 0, input.Count)
	for i := 0; i < input.Count; i++ {
		codeValue, err := GenerateRedeemCode()
		if err != nil {
			return nil, err
		}
		code := RedeemCode{
			Code:      codeValue,
			Type:      input.Type,
			Value:     input.Value,
			Status:    StatusUnused,
			ExpiresAt: input.ExpiresAt,
		}
		// 订阅类型专用字段
		if input.Type == RedeemTypeSubscription {
			code.GroupID = input.GroupID
			code.ValidityDays = input.ValidityDays
			if code.ValidityDays <= 0 {
				code.ValidityDays = 30 // 默认30天
			}
		}
		if err := s.redeemCodeRepo.Create(ctx, &code); err != nil {
			return nil, err
		}
		codes = append(codes, code)
	}
	return codes, nil
}

func (s *adminServiceImpl) DeleteRedeemCode(ctx context.Context, id int64) error {
	return s.redeemCodeRepo.Delete(ctx, id)
}

func (s *adminServiceImpl) BatchDeleteRedeemCodes(ctx context.Context, ids []int64) (int64, error) {
	var deleted int64
	for _, id := range ids {
		if err := s.redeemCodeRepo.Delete(ctx, id); err == nil {
			deleted++
		}
	}
	return deleted, nil
}

func (s *adminServiceImpl) ExpireRedeemCode(ctx context.Context, id int64) (*RedeemCode, error) {
	code, err := s.redeemCodeRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	code.Status = StatusExpired
	if err := s.redeemCodeRepo.Update(ctx, code); err != nil {
		return nil, err
	}
	return code, nil
}
