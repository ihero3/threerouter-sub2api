-- 241_affiliate_guard_events.sql
--
-- 背景：邀请注册奖励（register_reward）此前无任何反刷限制，同一 IP 下可批量注册
-- 多账号并共用一个邀请人，从而反复套取注册奖励。本迁移引入「注册 IP 与邀请人关联
-- 校验」所需的人工审核事件表。
--
-- 运行机制：
--   - 注册绑定邀请人时，若同一邀请人已绑定的同 IP 账号数达到阈值：
--       · block  模式：直接拦截（不绑定邀请人、不发奖励），不落事件；
--       · review 模式：拦截并在此表记录一条 pending 事件，管理员可批准补发或拒绝。
--   - 每个被邀请人最多一条事件（UNIQUE invitee_id），避免重复注册重试产生多条。
--
-- 幂等：IF NOT EXISTS，可重复执行。
CREATE TABLE IF NOT EXISTS affiliate_guard_events (
    id BIGSERIAL PRIMARY KEY,
    inviter_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    invitee_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    register_ip VARCHAR(64) NOT NULL DEFAULT '',
    reason VARCHAR(32) NOT NULL DEFAULT 'ip_share',
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    reward_amount DECIMAL(20,8) NOT NULL DEFAULT 0,
    reviewed_by BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_affiliate_guard_events_invitee UNIQUE (invitee_id)
);

CREATE INDEX IF NOT EXISTS idx_affiliate_guard_events_inviter_id
    ON affiliate_guard_events(inviter_id);

CREATE INDEX IF NOT EXISTS idx_affiliate_guard_events_status
    ON affiliate_guard_events(status);

COMMENT ON TABLE affiliate_guard_events IS '邀请注册 IP 关联风控人工审核事件';
COMMENT ON COLUMN affiliate_guard_events.inviter_id IS '被绑定/试图绑定的邀请人用户ID';
COMMENT ON COLUMN affiliate_guard_events.invitee_id IS '触发风控的被邀请人用户ID（唯一）';
COMMENT ON COLUMN affiliate_guard_events.register_ip IS '被邀请人注册时的客户端 IP';
COMMENT ON COLUMN affiliate_guard_events.reason IS '命中原因：ip_share=同IP多账号共用一个邀请人';
COMMENT ON COLUMN affiliate_guard_events.status IS '待审核 pending / 已批准 approved / 已拒绝 rejected';
COMMENT ON COLUMN affiliate_guard_events.reward_amount IS '批准时补发的注册奖励金额，拒绝时为 0';
COMMENT ON COLUMN affiliate_guard_events.reviewed_by IS '审核管理员用户ID';
COMMENT ON COLUMN affiliate_guard_events.reviewed_at IS '审核时间';
