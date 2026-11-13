-- 253_user_risk_review_exemptions.sql
--
-- 背景：管理员 POST /:id/risk-review 复核放行后，users 上的 risk_level /
-- risk_reasons / risk_updated_at 被全部清空，库里不留「已人工放行」的证据。
-- 下一次手动全量扫描会用同一批静态信号（一次性邮箱、随机邮箱名、共享密码、
-- 邀请链深等）把账号标回 suspect/abnormal，「复核通过」按钮反复出现，
-- 管理员的复核结论无法沉淀。
--
-- 设计取舍：
--   - 不往 users 表加列，避免改动 ent schema 并重新生成代码（同 252）；
--   - 只记 user_id + 复核时间，不存邮箱等个人标识，也不建外键；
--   - 豁免仅作用于全量扫描（见 listUserIDsForRiskScanSQL）：实时邀请绑定
--     是新增证据，仍会重新评估并打标，为后续行为保留兜底；
--   - 撤销豁免只需 DELETE 该行，无需改动 users 主数据。
--
-- 幂等：IF NOT EXISTS，可重复执行；重复复核走 ON CONFLICT 刷新 reviewed_at。

CREATE TABLE IF NOT EXISTS user_risk_review_exemptions (
    user_id     BIGINT PRIMARY KEY,
    reviewed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE user_risk_review_exemptions IS '已人工复核放行的用户：全量邀请风险扫描跳过，实时打标仍可重新标记';
COMMENT ON COLUMN user_risk_review_exemptions.reviewed_at IS '最近一次管理员复核放行时间';
