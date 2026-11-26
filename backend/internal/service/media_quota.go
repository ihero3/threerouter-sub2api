package service

import "context"

// settleMediaReservedQuota 完成时结算：reserved 预估预扣 vs actual 实际费用（视频/媒体共用）。
// diff = reserved - actual；diff>0 退回差额，diff<0 补扣差额。
func settleMediaReservedQuota(apiKeySvc *APIKeyService, ctx context.Context, apiKeyID int64, reserved, actual float64) {
	if apiKeySvc == nil {
		return
	}
	if reserved <= 0 {
		// 无预扣：退化为完成时直接扣实际费用（兜底）。
		if actual > 0 {
			_ = apiKeySvc.UpdateQuotaUsed(ctx, apiKeyID, actual)
		}
		return
	}
	diff := reserved - actual
	if diff > 0 {
		_ = apiKeySvc.ReleaseQuotaUsed(ctx, apiKeyID, diff)
	} else if diff < 0 {
		_ = apiKeySvc.UpdateQuotaUsed(ctx, apiKeyID, -diff)
	}
}

// releaseMediaReservedQuota 失败/取消时全额退回预扣费用。
func releaseMediaReservedQuota(apiKeySvc *APIKeyService, ctx context.Context, apiKeyID int64, reserved float64) {
	if apiKeySvc == nil || reserved <= 0 {
		return
	}
	_ = apiKeySvc.ReleaseQuotaUsed(ctx, apiKeyID, reserved)
}

// reserveAndTrackQuota 在 key 配额上预扣 cost，仅当扣款真正落账时才返回可供记录
// 的 cost 指针。预扣失败返回 nil：ReservedCost 是后续退款与结算差额的唯一依据，
// 若扣款没成功却记录了它，失败退款会多退、成功结算会少扣，凭空给用户送额度。
func reserveAndTrackQuota(apiKeySvc *APIKeyService, ctx context.Context, apiKeyID int64, cost float64) (*float64, error) {
	if apiKeySvc == nil || cost <= 0 {
		return nil, nil
	}
	if err := apiKeySvc.UpdateQuotaUsed(ctx, apiKeyID, cost); err != nil {
		return nil, err
	}
	return &cost, nil
}
