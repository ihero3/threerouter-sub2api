-- 246_add_users_looked_ticket_flag.sql
--
-- 背景：管理员在邀请关系页排查批量注册/异常账号时，需要记录「已看过该账号几次」
-- 与「该账号提过工单」这两个人工标记，便于后续复查时了解跟进进度。
--
-- 本迁移给 users 增加两个轻量标记，仅用于**管理端展示与人工研判**：
--   - 不参与任何鉴权、限流、返利结算逻辑；
--   - looked_count 由管理员在邀请关系页点击「看过」累加，只增不减；
--   - ticket_flag 为布尔开关，管理员可反复切换。
--
-- 幂等：IF NOT EXISTS，可重复执行。
ALTER TABLE users ADD COLUMN IF NOT EXISTS looked_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS ticket_flag BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN users.looked_count IS '管理员查看该账号的次数（邀请关系页手动累加，仅管理端展示）';
COMMENT ON COLUMN users.ticket_flag IS '管理员标记该账号提过工单（仅管理端展示）';
