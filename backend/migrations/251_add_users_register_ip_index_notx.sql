-- 251_add_users_register_ip_index_notx.sql
--
-- 背景：注册风控在「同一 IP 每日注册数」之外又增加了「同一 IP 每小时注册数」检查，
-- 每次注册会执行 2-3 次 WHERE register_ip = ? AND created_at >= ? 的统计查询
-- （每日配额预检、每小时风控预检、建号事务内复查）。users 表此前只有 register_ip
-- 列、没有索引，这些查询全部走全表扫描；用户量增长后注册路径会被明显拖慢，
-- 且 CreateWithRegistrationGuards 会按 IP 加锁串行化，进一步放大开销。
--
-- 部分索引：存量用户 register_ip 为空串、不参与任何 IP 统计，排除后索引体积更小。
-- service 层对空 IP 直接返回 0、不会发起查询，因此查询恒带非空 register_ip，
-- 可以命中该部分索引。
--
-- 幂等：IF NOT EXISTS，可重复执行。CONCURRENTLY 避免锁表，故使用 _notx 后缀
-- （事务外执行，见 migrations_runner 的 nonTransactionalMigrationSuffix）。

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_register_ip_created_at
    ON users (register_ip, created_at)
    WHERE register_ip <> '';
