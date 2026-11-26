-- 255_add_usage_logs_created_ip_user_index_notx.sql
--
-- 背景：管理端 User 搜索/列表的 GetSharedUsageIPs 每次都执行：
--   SELECT ip_address FROM usage_logs
--   WHERE ip_address IS NOT NULL AND ip_address <> ''
--     AND created_at >= now() - interval '10 days'
--   GROUP BY ip_address HAVING COUNT(DISTINCT user_id) >= 5
-- 生产上该查询走 idx_usage_logs_created_at 扫描 11.4 万行后做 external merge
-- 磁盘排序（work_mem=4MB），单次约 227ms，且每次按键都会触发。
--
-- 覆盖索引 (created_at, ip_address, user_id) 让该聚合走 index-only scan，
-- 无需回表、也无需为 GROUP BY 单独排序（ip_address 在索引内有序），
-- 消除排序落盘。CONCURRENTLY 避免锁写入，IF NOT EXISTS 保证幂等。
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_usage_logs_created_ip_user
    ON usage_logs (created_at, ip_address, user_id);
