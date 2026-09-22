/**
 * Admin Affiliate API endpoints
 * Manage per-user affiliate (邀请返利) configurations:
 * exclusive invite codes (overrides aff_code) and exclusive rebate rates.
 */

import { apiClient } from '../client'
import type { AffiliateRebateType, PaginatedResponse } from '@/types'

export interface AffiliateAdminEntry {
  user_id: number
  email: string
  username: string
  aff_code: string
  aff_code_custom: boolean
  aff_rebate_rate_percent?: number | null
  aff_count: number
}

export interface ListAffiliateUsersParams {
  page?: number
  page_size?: number
  search?: string
}

export interface ListAffiliateRecordsParams {
  page?: number
  page_size?: number
  search?: string
  start_at?: string
  end_at?: string
  sort_by?: string
  sort_order?: 'asc' | 'desc'
  timezone?: string
}

export interface AffiliateInviteRecord {
  inviter_id: number
  inviter_email: string
  inviter_username: string
  invitee_id: number
  invitee_email: string
  invitee_username: string
  aff_code: string
  /** 邀请返利：注册奖励合计 */
  invite_rebate: number
  /** 充值返利：充值/兑换返利合计 */
  recharge_rebate: number
  /** 累计返利 = invite_rebate + recharge_rebate */
  total_rebate: number
  created_at: string
}

export interface AffiliateRebateRecord {
  /** 邀请返利（注册奖励）无关联订单，order_id 为 null */
  order_id: number | null
  /** 无关联订单时为空串 */
  out_trade_no: string
  inviter_id: number
  inviter_email: string
  inviter_username: string
  invitee_id: number
  invitee_email: string
  invitee_username: string
  /** invite=邀请返利，recharge=充值返利 */
  rebate_type: AffiliateRebateType
  order_amount: number | null
  pay_amount: number | null
  rebate_amount: number
  payment_type: string
  order_status: string
  created_at: string
}

export interface AffiliateTransferRecord {
  ledger_id: number
  user_id: number
  user_email: string
  username: string
  amount: number
  balance_after?: number | null
  available_quota_after?: number | null
  frozen_quota_after?: number | null
  history_quota_after?: number | null
  snapshot_available: boolean
  created_at: string
}

export interface AffiliateUserOverview {
  user_id: number
  email: string
  username: string
  aff_code: string
  rebate_rate_percent: number
  invited_count: number
  rebated_invitee_count: number
  available_quota: number
  history_quota: number
}

export interface UpdateAffiliateUserRequest {
  aff_code?: string
  aff_rebate_rate_percent?: number | null
  /** Set true to explicitly clear the per-user rate (sets it to NULL). */
  clear_rebate_rate?: boolean
}

export interface BatchSetRateRequest {
  user_ids: number[]
  aff_rebate_rate_percent?: number | null
  /** Set true to clear rates instead of setting. */
  clear?: boolean
}

export interface SimpleUser {
  id: number
  email: string
  username: string
}

export async function listUsers(
  params: ListAffiliateUsersParams = {},
): Promise<PaginatedResponse<AffiliateAdminEntry>> {
  const { data } = await apiClient.get<PaginatedResponse<AffiliateAdminEntry>>(
    '/admin/affiliates/users',
    {
      params: {
        page: params.page ?? 1,
        page_size: params.page_size ?? 20,
        search: params.search ?? '',
      },
    },
  )
  return data
}

export async function lookupUsers(q: string): Promise<SimpleUser[]> {
  const { data } = await apiClient.get<SimpleUser[]>(
    '/admin/affiliates/users/lookup',
    { params: { q } },
  )
  return data
}

export async function updateUserSettings(
  userId: number,
  payload: UpdateAffiliateUserRequest,
): Promise<{ user_id: number }> {
  const { data } = await apiClient.put<{ user_id: number }>(
    `/admin/affiliates/users/${userId}`,
    payload,
  )
  return data
}

export async function clearUserSettings(
  userId: number,
): Promise<{ user_id: number }> {
  const { data } = await apiClient.delete<{ user_id: number }>(
    `/admin/affiliates/users/${userId}`,
  )
  return data
}

export async function batchSetRate(
  payload: BatchSetRateRequest,
): Promise<{ affected: number }> {
  const { data } = await apiClient.post<{ affected: number }>(
    '/admin/affiliates/users/batch-rate',
    payload,
  )
  return data
}

function recordParams(params: ListAffiliateRecordsParams = {}) {
  return {
    page: params.page ?? 1,
    page_size: params.page_size ?? 20,
    search: params.search ?? '',
    start_at: params.start_at || undefined,
    end_at: params.end_at || undefined,
    sort_by: params.sort_by || undefined,
    sort_order: params.sort_order || undefined,
    timezone: params.timezone || undefined,
  }
}

export async function listInviteRecords(
  params: ListAffiliateRecordsParams = {},
): Promise<PaginatedResponse<AffiliateInviteRecord>> {
  const { data } = await apiClient.get<PaginatedResponse<AffiliateInviteRecord>>(
    '/admin/affiliates/invites',
    { params: recordParams(params) },
  )
  return data
}

export async function listRebateRecords(
  params: ListAffiliateRecordsParams = {},
): Promise<PaginatedResponse<AffiliateRebateRecord>> {
  const { data } = await apiClient.get<PaginatedResponse<AffiliateRebateRecord>>(
    '/admin/affiliates/rebates',
    { params: recordParams(params) },
  )
  return data
}

export async function listTransferRecords(
  params: ListAffiliateRecordsParams = {},
): Promise<PaginatedResponse<AffiliateTransferRecord>> {
  const { data } = await apiClient.get<PaginatedResponse<AffiliateTransferRecord>>(
    '/admin/affiliates/transfers',
    { params: recordParams(params) },
  )
  return data
}

export async function getUserOverview(
  userId: number,
): Promise<AffiliateUserOverview> {
  const { data } = await apiClient.get<AffiliateUserOverview>(
    `/admin/affiliates/users/${userId}/overview`,
  )
  return data
}

export interface AffiliateRelationUser {
  user_id: number
  email: string
  username: string
  created_at: string
}

export interface AffiliateRelationNode {
  user_id: number
  email: string
  username: string
  created_at: string
  /** 距查询用户的层数：1 = 直接上级 / 直接下级 */
  depth: number
  /** 该用户为其**直接上级**贡献的返利合计（返利只向上走一级） */
  rebate_amount: number
}

export interface AffiliateInviteRelation {
  user: AffiliateRelationUser
  /** null 表示来路不明（无邀请来源） */
  inviter: AffiliateRelationUser | null
  /** 从链路顶端到直接邀请人，不含查询用户 */
  ancestors: AffiliateRelationNode[]
  descendants: AffiliateRelationNode[]
  descendant_count: number
  /** true 表示达到深度/行数上限，结果不完整 */
  chain_truncated: boolean
  bound_at?: string | null
}

export interface AffiliateUnsourcedUser {
  user_id: number
  email: string
  username: string
  created_at: string
  balance: number
  total_recharged: number
  /** false = 连邀请档案都没有（多为功能上线前注册的老账号） */
  has_affiliate_profile: boolean
}

export async function getInviteRelations(
  userId: number,
): Promise<AffiliateInviteRelation> {
  const { data } = await apiClient.get<AffiliateInviteRelation>(
    '/admin/affiliates/relations',
    { params: { user_id: userId } },
  )
  return data
}

export async function listUnsourcedUsers(
  params: ListAffiliateRecordsParams = {},
): Promise<PaginatedResponse<AffiliateUnsourcedUser>> {
  const { data } = await apiClient.get<PaginatedResponse<AffiliateUnsourcedUser>>(
    '/admin/affiliates/relations/unsourced',
    { params: recordParams(params) },
  )
  return data
}

export const affiliatesAPI = {
  listUsers,
  lookupUsers,
  updateUserSettings,
  clearUserSettings,
  batchSetRate,
  listInviteRecords,
  listRebateRecords,
  listTransferRecords,
  getUserOverview,
  getInviteRelations,
  listUnsourcedUsers,
}

export default affiliatesAPI
