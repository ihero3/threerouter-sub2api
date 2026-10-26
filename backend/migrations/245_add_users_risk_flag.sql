-- 245_add_users_risk_flag.sql
--
-- 背景：邀请注册奖励（register_reward）存在批量注册套取的空间。此前只有
-- 「同 IP 多账号共用一个邀请人」一种拦截规则（见 241），且命中即拦截、无处
-- 记录「这个账号可疑」这一事实。
--
-- 本迁移给 users 增加轻量风险标记，仅用于**管理端展示与人工研判**：
--   - 不参与任何鉴权、限流、返利结算逻辑；
--   - 标记只升不降（扫描不会清除历史标记），避免证据被后续扫描抹掉；
--   - 命中原因以逗号分隔的规则 key 存放，供后台 hover 展示。
--
-- risk_level:    ''（正常）/ 'suspect'（存疑）/ 'abnormal'（异常）
-- risk_reasons:  命中规则 key，逗号分隔，如 'invite_burst_24h,chain_depth'
--
-- 幂等：IF NOT EXISTS，可重复执行。
ALTER TABLE users ADD COLUMN IF NOT EXISTS risk_level VARCHAR(16) NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS risk_reasons VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS risk_updated_at TIMESTAMPTZ NULL;

-- 部分索引：只索引有标记的行（绝大多数行为空），便于后台按风险状态筛选。
CREATE INDEX IF NOT EXISTS idx_users_risk_level
    ON users(risk_level) WHERE risk_level <> '';

COMMENT ON COLUMN users.risk_level IS '邀请注册风险等级：空=正常 / suspect=存疑 / abnormal=异常（仅管理端展示）';
COMMENT ON COLUMN users.risk_reasons IS '风险命中规则 key，逗号分隔（见 service 层 AffiliateRiskReason* 常量）';
COMMENT ON COLUMN users.risk_updated_at IS '风险标记最近一次更新时间';
