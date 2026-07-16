/**
 * AI 治理与合规 管理端 API 客户端
 *
 * 对应后端路由前缀 /admin/governance/*（见 docs/合规方案.md 第五章）。
 * 响应经全局拦截器解包为 { code, message, data } 中的 data 部分。
 */
import { apiClient } from '../client'

// ==================== 通用分页 ====================

export interface PaginatedResult<T> {
  items: T[]
  total: number
  page: number
  page_size: number
  total_pages: number
}

export interface PaginationParams {
  page?: number
  page_size?: number
}

// ==================== 模块状态 ====================

export interface GovernanceCapabilities {
  risk_tagging: boolean
  audit_logging: boolean
  gdpr_erasure: boolean
  gdpr_data_export: boolean
  consent_management: boolean
  eu_ai_act_report: boolean
}

export interface GovernanceStatus {
  module: string
  primary_role: string
  secondary_roles: string[]
  risk_tier: string
  capabilities: GovernanceCapabilities
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

export interface ListAuditLogsParams extends PaginationParams {
  compliance_type?: string
  subject_type?: string
  subject_id?: number
  from?: string
  to?: string
}

// ==================== 风险标签目录 ====================

export interface RiskTagDescriptor {
  tag: string
  description: string
}

export interface RiskTagsCatalog {
  model_tags: RiskTagDescriptor[]
  risk_tags: RiskTagDescriptor[]
}

// ==================== EU AI Act 评估报告 ====================

export type EUAIActAssessment = Record<string, unknown>

// ==================== GDPR 数据处理记录 (ROPA) ====================

export type DataProcessingRecord = Record<string, unknown>

// ==================== GDPR 删除请求 ====================

export interface DataErasureRequest {
  id: number
  user_id: number
  request_type: string
  status: string
  scope_details: string
  rejection_reason?: string
  operator?: string
  requested_at: string
  processed_at?: string
  completed_at?: string
}

export interface ListErasureRequestsParams extends PaginationParams {
  status?: string
  user_id?: number
}

export interface ProcessErasureRequestPayload {
  approved: boolean
  reason?: string
}

export interface ProcessErasureRequestResponse {
  id: number
  approved: boolean
}

// ==================== 行业合规模板 ====================

export interface CompliancePolicyTemplate {
  id: number
  template_code: string
  industry: string
  description: string
  rules: Array<Record<string, unknown>>
  risk_tags: string[]
  created_at: string
}

export interface CreateCustomTemplatePayload {
  template_code: string
  industry: string
  description?: string
  rules?: Array<Record<string, unknown>>
  risk_tags?: string[]
}

// ==================== 内容审核自定义规则 ====================

export interface ModerationRule {
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
  created_by: string
  created_at: string
  updated_at: string
}

export interface CreateModerationRulePayload {
  rule_id: string
  rule_name: string
  rule_type: string
  rule_pattern: string
  threshold?: number
  action?: string
  risk_category?: string
  enabled?: boolean
  priority?: number
}

export type UpdateModerationRulePayload = Partial<Omit<CreateModerationRulePayload, 'rule_id'>>

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

// ==================== API 函数 ====================

export async function getStatus(): Promise<GovernanceStatus> {
  const { data } = await apiClient.get<GovernanceStatus>('/admin/governance/status')
  return data
}

export async function listAuditLogs(
  params: ListAuditLogsParams = {}
): Promise<PaginatedResult<ComplianceAuditLog>> {
  const { data } = await apiClient.get<PaginatedResult<ComplianceAuditLog>>(
    '/admin/governance/audit-logs',
    { params }
  )
  return data
}

export async function getRiskTags(): Promise<RiskTagsCatalog> {
  const { data } = await apiClient.get<RiskTagsCatalog>('/admin/governance/risk-tags')
  return data
}

export async function getEUAIActAssessment(): Promise<EUAIActAssessment> {
  const { data } = await apiClient.get<EUAIActAssessment>(
    '/admin/governance/eu-ai-act/assessment'
  )
  return data
}

export async function exportEUAIActAssessment(): Promise<Blob> {
  const response = await apiClient.post('/admin/governance/eu-ai-act/assessment', null, {
    responseType: 'blob',
  })
  return response.data as Blob
}

export async function getDataProcessingRecord(): Promise<DataProcessingRecord> {
  const { data } = await apiClient.get<DataProcessingRecord>(
    '/admin/governance/gdpr/data-processing-record'
  )
  return data
}

export async function listErasureRequests(
  params: ListErasureRequestsParams = {}
): Promise<PaginatedResult<DataErasureRequest>> {
  const { data } = await apiClient.get<PaginatedResult<DataErasureRequest>>(
    '/admin/governance/gdpr/erasure-requests',
    { params }
  )
  return data
}

export async function processErasureRequest(
  id: number,
  payload: ProcessErasureRequestPayload
): Promise<ProcessErasureRequestResponse> {
  const { data } = await apiClient.post<ProcessErasureRequestResponse>(
    `/admin/governance/gdpr/erasure-requests/${id}/process`,
    payload
  )
  return data
}

export interface TemplatesResult {
  items: CompliancePolicyTemplate[]
  active_template_code: string
}

export async function getComplianceTemplates(
  industry?: string
): Promise<TemplatesResult> {
  const { data } = await apiClient.get<TemplatesResult>(
    '/admin/governance/templates',
    { params: industry ? { industry } : {} }
  )
  return { items: data.items ?? [], active_template_code: data.active_template_code ?? '' }
}

export async function applyComplianceTemplate(
  templateCode: string
): Promise<CompliancePolicyTemplate> {
  const { data } = await apiClient.post<CompliancePolicyTemplate>(
    '/admin/governance/templates/apply',
    { template_code: templateCode }
  )
  return data
}

export async function createCustomTemplate(
  payload: CreateCustomTemplatePayload
): Promise<CompliancePolicyTemplate> {
  const { data } = await apiClient.post<CompliancePolicyTemplate>(
    '/admin/governance/templates/custom',
    payload
  )
  return data
}

export async function listModerationRules(): Promise<ModerationRule[]> {
  const { data } = await apiClient.get<{ items: ModerationRule[] }>(
    '/admin/governance/moderation-rules'
  )
  return data.items ?? []
}

export async function createModerationRule(
  payload: CreateModerationRulePayload
): Promise<ModerationRule> {
  const { data } = await apiClient.post<ModerationRule>(
    '/admin/governance/moderation-rules',
    payload
  )
  return data
}

export async function updateModerationRule(
  ruleId: string,
  payload: UpdateModerationRulePayload
): Promise<ModerationRule> {
  const { data } = await apiClient.put<ModerationRule>(
    `/admin/governance/moderation-rules/${encodeURIComponent(ruleId)}`,
    payload
  )
  return data
}

export async function deleteModerationRule(ruleId: string): Promise<void> {
  await apiClient.delete(`/admin/governance/moderation-rules/${encodeURIComponent(ruleId)}`)
}

export async function setModerationStrategy(strategy: string): Promise<void> {
  await apiClient.post('/admin/governance/moderation-rules/strategy', { strategy })
}

export async function getSupportedJurisdictions(): Promise<string[]> {
  const { data } = await apiClient.get<SupportedJurisdictions>(
    '/admin/governance/jurisdiction/mapping'
  )
  return data.supported_jurisdictions ?? []
}

export async function getJurisdictionMapping(
  params: JurisdictionMappingParams
): Promise<JurisdictionMappingResult> {
  const { data } = await apiClient.get<JurisdictionMappingResult>(
    '/admin/governance/jurisdiction/mapping',
    { params }
  )
  return data
}

// ==================== DPA 生成 ====================

export interface GenerateDPAPayload {
  controller_name: string
  controller_contact: string
}

export async function generateDPA(payload: GenerateDPAPayload): Promise<void> {
  const response = await apiClient.post('/admin/governance/gdpr/dpa/generate', payload, {
    responseType: 'blob',
  })
  const blob = response.data as Blob
  const url = window.URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  const contentDisposition = response.headers['content-disposition']
  if (contentDisposition) {
    const match = contentDisposition.match(/filename=(.+)/)
    if (match) {
      a.download = match[1]
    } else {
      a.download = `dpa-${Date.now()}.json`
    }
  } else {
    a.download = `dpa-${Date.now()}.json`
  }
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

export interface CreateCredentialPayload {
  credential_id: string
  credential_type: string
  issuer: string
  issuer_type?: string
  scope?: string
  valid_from?: string
  valid_until?: string
  evidence_hashes?: string
  digital_signature?: string
  metadata?: Record<string, unknown>
}

export async function listCredentials(type?: string, status?: string): Promise<ComplianceCredential[]> {
  const { data } = await apiClient.get<{ items: ComplianceCredential[] }>(
    '/admin/governance/credentials',
    { params: { type, status } }
  )
  return data.items ?? []
}

export async function getCredential(id: number): Promise<ComplianceCredential> {
  const { data } = await apiClient.get<ComplianceCredential>(
    `/admin/governance/credentials/${id}`
  )
  return data
}

export async function createCredential(payload: CreateCredentialPayload): Promise<ComplianceCredential> {
  const { data } = await apiClient.post<ComplianceCredential>(
    '/admin/governance/credentials',
    payload
  )
  return data
}

export async function revokeCredential(id: number): Promise<void> {
  await apiClient.post(`/admin/governance/credentials/${id}/revoke`)
}

export async function activateCredential(id: number): Promise<void> {
  await apiClient.post(`/admin/governance/credentials/${id}/activate`)
}

export async function deleteCredential(id: number): Promise<void> {
  await apiClient.delete(`/admin/governance/credentials/${id}`)
}

export const governanceAPI = {
  getStatus,
  listAuditLogs,
  getRiskTags,
  getEUAIActAssessment,
  exportEUAIActAssessment,
  getDataProcessingRecord,
  listErasureRequests,
  processErasureRequest,
  getComplianceTemplates,
  applyComplianceTemplate,
  createCustomTemplate,
  listModerationRules,
  createModerationRule,
  updateModerationRule,
  deleteModerationRule,
  setModerationStrategy,
  getSupportedJurisdictions,
  getJurisdictionMapping,
  generateDPA,
  listCredentials,
  createCredential,
  revokeCredential,
  activateCredential,
  deleteCredential,
}
