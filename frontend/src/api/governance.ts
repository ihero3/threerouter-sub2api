/**
 * AI 治理与合规 用户端 API 客户端
 *
 * 对应后端路由前缀 /governance/*（用户端）。
 * 响应经全局拦截器解包为 { code, message, data } 中的 data 部分。
 */
import { apiClient } from './client'

// ==================== 数据删除请求 ====================

export interface DataErasureRequestInput {
  reason: string
  confirmation_text: string
  request_type: string
}

export interface DataErasureRequest {
  id: string
  status: 'pending' | 'approved' | 'rejected' | 'completed'
  request_type: string
  reason: string
  requested_at: string
  processed_at?: string
  completed_at?: string
}

export async function requestDataErasure(input: DataErasureRequestInput): Promise<DataErasureRequest> {
  const { data } = await apiClient.post<DataErasureRequest>('/governance/data-erasure/request', input)
  return data
}

export async function listDataErasureRequests(): Promise<DataErasureRequest[]> {
  const { data } = await apiClient.get<DataErasureRequest[]>('/governance/data-erasure/requests')
  return data
}

// ==================== 数据导出 ====================

export async function exportUserData(): Promise<void> {
  const response = await apiClient.post('/governance/data-export', {}, { responseType: 'blob' })
  const blob = response.data as Blob
  const url = window.URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = 'personal-data-export.json'
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  window.URL.revokeObjectURL(url)
}

// ==================== 同意记录 ====================

export interface ConsentRecord {
  id: string
  user_id?: string
  consent_type: string
  version: string
  status: 'granted' | 'revoked'
  granted_at: string
  revoked_at?: string
  ip_address?: string
  user_agent?: string
  label?: string
  description?: string
}

export async function getUserConsents(): Promise<ConsentRecord[]> {
  const { data } = await apiClient.get<ConsentRecord[]>('/governance/consent')
  return data
}

export async function setConsent(consentType: string, granted: boolean): Promise<ConsentRecord> {
  const { data } = await apiClient.post<ConsentRecord>('/governance/consent', { consent_type: consentType, granted })
  return data
}

// ==================== Account 合规配置 ====================

export interface ModerationPolicy {
  enabled_rule_ids: string[]
}

export interface ComplianceProfile {
  active_template_code?: string
  zdr_mode: 'aggregate_only' | 'audit'
  detail_retention_days: number
  compliance_frameworks: string[]
  moderation_policy: ModerationPolicy
}

export interface TemplateItem {
  code: string
  industry: string
  description: string
}

export interface TemplateListResult {
  items: TemplateItem[]
  active_template_code?: string
}

export interface ModerationRuleItem {
  rule_id: string
  rule_name: string
  rule_type: string
  action: string
  enabled: boolean
}

export async function getComplianceProfile(): Promise<ComplianceProfile> {
  const { data } = await apiClient.get<ComplianceProfile>('/governance/profile')
  return data
}

export async function updateComplianceProfile(data: Partial<ComplianceProfile>): Promise<ComplianceProfile> {
  const { data: resp } = await apiClient.put<ComplianceProfile>('/governance/profile', data)
  return resp
}

export async function listComplianceTemplates(): Promise<TemplateListResult> {
  const { data } = await apiClient.get<TemplateListResult>('/governance/templates')
  return data
}

export async function applyComplianceTemplate(templateCode: string): Promise<TemplateItem> {
  const { data } = await apiClient.post<TemplateItem>('/governance/templates/apply', { template_code: templateCode })
  return data
}

export async function listModerationRules(): Promise<ModerationRuleItem[]> {
  const { data } = await apiClient.get<ModerationRuleItem[]>('/governance/moderation-rules')
  return data
}

// ==================== 用户自定义审核规则 ====================

export interface UserModerationRule {
  id: number
  rule_id: string
  rule_name: string
  rule_type: string
  rule_pattern: string
  threshold: number
  action: string
  risk_category: string
  enabled: boolean
  priority: number
  user_id?: number
  created_at: string
  updated_at: string
}

export interface CreateUserModerationRuleInput {
  rule_name: string
  rule_type: string
  rule_pattern: string
  threshold?: number
  action?: string
  risk_category?: string
  enabled?: boolean
  priority?: number
}

export interface UpdateUserModerationRuleInput {
  rule_name?: string
  rule_type?: string
  rule_pattern?: string
  threshold?: number
  action?: string
  risk_category?: string
  enabled?: boolean
  priority?: number
}

export async function listUserModerationRules(): Promise<UserModerationRule[]> {
  const { data } = await apiClient.get<UserModerationRule[]>('/governance/moderation-rules/user')
  return data
}

export async function createUserModerationRule(input: CreateUserModerationRuleInput): Promise<UserModerationRule> {
  const { data } = await apiClient.post<UserModerationRule>('/governance/moderation-rules/user', input)
  return data
}

export async function updateUserModerationRule(ruleId: string, input: UpdateUserModerationRuleInput): Promise<UserModerationRule> {
  const { data } = await apiClient.put<UserModerationRule>(`/governance/moderation-rules/user/${ruleId}`, input)
  return data
}

export async function deleteUserModerationRule(ruleId: string): Promise<void> {
  await apiClient.delete(`/governance/moderation-rules/user/${ruleId}`)
}

// ==================== 跨法域合规映射 ====================

export interface JurisdictionMappingParams {
  company_region: string
  industry?: string
  service_type?: string
}

export interface JurisdictionMappingResult {
  company_region: string
  industry: string
  service_type: string
  applicable_regulations: string[]
  required_measures: string[]
  risk_level: string
  recommended_actions: string[]
}

export interface SupportedJurisdictions {
  supported_jurisdictions: string[]
}

export async function getSupportedJurisdictions(): Promise<string[]> {
  const { data } = await apiClient.get<SupportedJurisdictions>('/governance/jurisdiction/mapping')
  return data.supported_jurisdictions ?? []
}

export async function getJurisdictionMapping(
  params: JurisdictionMappingParams
): Promise<JurisdictionMappingResult> {
  const { data } = await apiClient.get<JurisdictionMappingResult>(
    '/governance/jurisdiction/mapping',
    { params }
  )
  return data
}

export async function getUserJurisdictionMapping(): Promise<UserJurisdictionMapping | null> {
  const { data } = await apiClient.get<UserJurisdictionMapping>(
    '/governance/jurisdiction/mapping/user'
  )
  return data || null
}

export interface SaveJurisdictionMappingParams {
  company_region: string
  industry?: string
  service_type?: string
  apply_rules?: boolean
}

export interface SaveJurisdictionMappingResult {
  result: JurisdictionMappingResult
  applied_rules: string[]
}

export async function saveJurisdictionMapping(
  params: SaveJurisdictionMappingParams
): Promise<SaveJurisdictionMappingResult> {
  const { data } = await apiClient.post<SaveJurisdictionMappingResult>(
    '/governance/jurisdiction/mapping/save',
    params
  )
  return data
}

export interface UserJurisdictionMapping {
  id: number
  user_id: number
  company_region: string
  industry: string
  service_type: string
  risk_level: string
  applicable_regulations: string[]
  required_measures: string[]
  recommended_actions: string[]
  applied_rules: string[]
  created_at: string
  updated_at: string
}

// ==================== DPA 生成 ====================

export interface GenerateDPAPayload {
  controller_name: string
  controller_contact: string
}

export async function generateDPA(payload: GenerateDPAPayload): Promise<void> {
  const response = await apiClient.post('/governance/gdpr/dpa/generate', payload, {
    responseType: 'blob',
  })
  const blob = response.data as Blob
  const url = window.URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `dpa-${Date.now()}.json`
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  window.URL.revokeObjectURL(url)
}

// ==================== 合规凭证 ====================

export interface ComplianceCredential {
  id: number
  credential_id: string
  credential_type: string
  issuer: string
  issuer_type: string
  scope?: string
  status: string
  valid_from: string
  valid_until: string
  evidence_hashes?: string
  digital_signature?: string
  metadata?: Record<string, unknown>
  created_at: string
}

export async function listCredentials(type?: string, status?: string): Promise<ComplianceCredential[]> {
  const { data } = await apiClient.get<{ items: ComplianceCredential[] }>(
    '/governance/credentials',
    { params: { type, status } }
  )
  return data.items ?? []
}

// ==================== 审计日志 ====================

export interface ComplianceAuditLog {
  id: number
  compliance_type: string
  subject_type: string
  subject_id?: number
  details: string
  operator: string
  evidence_hash?: string
  created_at: string
}

export interface PaginatedResult<T> {
  items: T[]
  total: number
  page: number
  page_size: number
  total_pages: number
}

export interface ListAuditLogsParams {
  page?: number
  page_size?: number
  compliance_type?: string
  subject_type?: string
}

export async function listAuditLogs(
  params: ListAuditLogsParams = {}
): Promise<PaginatedResult<ComplianceAuditLog>> {
  const { data } = await apiClient.get<PaginatedResult<ComplianceAuditLog>>(
    '/governance/audit-logs',
    { params }
  )
  return data
}

// ==================== 风险标签 ====================

export interface RiskTagDescriptor {
  tag: string
  description: string
}

export interface RiskTagsCatalog {
  model_tags: RiskTagDescriptor[]
  risk_tags: RiskTagDescriptor[]
}

export async function getRiskTags(): Promise<RiskTagsCatalog> {
  const { data } = await apiClient.get<RiskTagsCatalog>('/governance/risk-tags')
  return data
}

// ==================== EU AI Act 评估 ====================

export type EUAIActAssessment = Record<string, unknown>

export async function getEUAIActAssessment(): Promise<EUAIActAssessment> {
  const { data } = await apiClient.get<EUAIActAssessment>(
    '/governance/eu-ai-act/assessment'
  )
  return data
}

export async function exportEUAIActAssessment(): Promise<void> {
  const response = await apiClient.post('/governance/eu-ai-act/assessment', null, {
    responseType: 'blob',
  })
  const blob = response.data as Blob
  const url = window.URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `eu-ai-act-assessment-${Date.now()}.json`
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  window.URL.revokeObjectURL(url)
}

// ==================== GDPR 数据处理记录 (ROPA) ====================

export type DataProcessingRecord = Record<string, unknown>

export async function getDataProcessingRecord(): Promise<DataProcessingRecord> {
  const { data } = await apiClient.get<DataProcessingRecord>(
    '/governance/gdpr/data-processing-record'
  )
  return data
}
