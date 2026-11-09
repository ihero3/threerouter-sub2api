-- 252_registration_device_fingerprints.sql
--
-- 背景：线上出现「同一人通过住宅代理池轮换出口 IP、每 1-2 分钟注册一个账号」
-- 的批量注册。单 IP 配额 / 每小时上限 / 单 IP 限流对这种攻击全部失效，因为
-- 每个 IP 只使用一次，且 IP 分散在不同国家与 ASN。
--
-- 本迁移建立一张轻量的注册设备指纹流水表：前端上报 UA + 时区 + 语言 + 屏幕 +
-- canvas 的摘要，服务端混合 User-Agent 后再哈希落库。风控按「同一指纹 24 小时内
-- 成功注册数」拦截，从而把跨 IP 的注册行为重新关联到同一台设备。
--
-- 设计取舍：
--   - 不往 users 表加列，避免改动 ent schema 并重新生成代码；
--   - 只记录成功建号，语义等同于「该设备已注册账号数」；
--   - 不建外键，便于日后按窗口清理历史流水而不影响用户主数据；
--   - 表内不存邮箱等个人标识，只存哈希、用户 ID 与注册 IP。
--
-- 幂等：IF NOT EXISTS，可重复执行。新表无存量数据，普通 CREATE INDEX 即可。

CREATE TABLE IF NOT EXISTS registration_device_fingerprints (
    id           BIGSERIAL PRIMARY KEY,
    fingerprint  VARCHAR(64)  NOT NULL,
    user_id      BIGINT       NULL,
    register_ip  VARCHAR(64)  NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_registration_device_fingerprints_fp_created
    ON registration_device_fingerprints (fingerprint, created_at);

COMMENT ON TABLE registration_device_fingerprints IS '注册设备指纹流水：按同一指纹 24 小时注册数做反批量注册风控';
COMMENT ON COLUMN registration_device_fingerprints.fingerprint IS 'SHA256(前端设备摘要 + User-Agent)，不可逆';
