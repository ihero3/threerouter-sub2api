-- 239_affiliate_ledger_source_user_index.sql
--
-- 背景：user_affiliate_ledger 已有 (user_id)、(action)、(user_id, frozen_until) 三个索引，
-- 但**没有 source_user_id 上的索引**，而「谁给谁贡献了多少返利」这一类查询全部按它过滤/聚合：
--   1. 用户端「已邀请用户」明细 ListInvitees：JOIN source_user_id = 被邀请人；
--   2. 管理端邀请记录 ListAffiliateInviteRecords：按被邀请人 LEFT JOIN 聚合贡献返利；
--   3. 管理端邀请关系 GetInviteRelations：对后代集合聚合返利（当前为整表聚合，索引后走索引扫描）；
--   4. 单人返利上限 GetAccruedRebateFromInvitee：user_id + source_user_id 过滤。
--
-- 选复合 (source_user_id, action) 而非单列 source_user_id：
--   - 单列能做的（最左前缀）它都能做；
--   - 带 action IN ('accrue','register_reward') 过滤的聚合可以直接命中索引，避免全表堆扫描。
--
-- 幂等：IF NOT EXISTS，可重复执行。
CREATE INDEX IF NOT EXISTS idx_user_affiliate_ledger_source_user_action
    ON user_affiliate_ledger (source_user_id, action);
