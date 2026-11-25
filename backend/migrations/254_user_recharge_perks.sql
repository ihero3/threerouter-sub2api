-- 254_user_recharge_perks.sql
--
-- 背景：用户首次「充值成功」（支付订单履约完成，含余额充值单与订阅单）时，系统自动
-- 赠送一次限流额度：并发数 +3、每秒请求数（RPM）+30。该赠送每个用户终身只发放一次。
--
-- 幂等实现：以 user_id 为主键的发放记录表，发放时 INSERT ... ON CONFLICT DO NOTHING，
-- 占位成功才执行加量，且占位与加量在同一事务内完成，确保并发/重复回调不会重复叠加。
--
-- 幂等迁移：IF NOT EXISTS，可重复执行。
CREATE TABLE IF NOT EXISTS user_recharge_perks (
    user_id        BIGINT PRIMARY KEY,
    source_order_id BIGINT NULL,
    concurrency_bonus INTEGER NOT NULL DEFAULT 0,
    rpm_bonus      INTEGER NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE user_recharge_perks IS '充值成功限流额度赠送发放记录（每用户终身一次，user_id 唯一）';
COMMENT ON COLUMN user_recharge_perks.source_order_id IS '触发首次赠送的支付订单 ID（仅用于溯源）';
COMMENT ON COLUMN user_recharge_perks.concurrency_bonus IS '本次赠送的并发数增量';
COMMENT ON COLUMN user_recharge_perks.rpm_bonus IS '本次赠送的每分钟请求数增量';
