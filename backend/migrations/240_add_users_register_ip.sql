-- 240_add_users_register_ip.sql
--
-- 背景：admin 用户管理需要展示「注册 IP」（在哪注册的、哪个国家注册的）。
-- 系统此前从未记录注册 IP，本迁移为 users 增加两列：
--   - register_ip：注册请求的客户端 IP（ip.GetClientIP(c)，已考虑反代头）；
--   - register_country：注册时 GeoIP（MaxMind GeoLite2）解析的国家 ISO 代码。
--
-- 约定：
--   - 仅对启用本功能后新注册的用户生效；存量用户为空串，不做回填（无历史数据可回填）；
--   - GeoIP 未启用或解析失败时 register_country 留空（fail-open），register_ip 照常记录；
--   - 管理员代建用户不记录（无被代建者真实 IP）。
--
-- 幂等：IF NOT EXISTS，可重复执行。
ALTER TABLE users ADD COLUMN IF NOT EXISTS register_ip VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS register_country VARCHAR(8) NOT NULL DEFAULT '';
