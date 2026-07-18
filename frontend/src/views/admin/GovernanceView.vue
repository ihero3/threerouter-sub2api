<template>
  <AppLayout>
    <div class="space-y-6">
      <!-- 页头 -->
      <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">
            {{ t('admin.governance.title') }}
          </h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.governance.description') }}
          </p>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <button
            type="button"
            class="btn btn-secondary inline-flex items-center gap-2"
            :disabled="statusLoading"
            @click="loadStatus"
          >
            <Icon name="refresh" size="sm" :class="statusLoading ? 'animate-spin' : ''" />
            {{ t('admin.governance.refresh') }}
          </button>
        </div>
      </div>

      <!-- 模块状态概览卡片 -->
      <div v-if="status" class="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
        <div class="rounded-lg border border-gray-100 bg-white px-4 py-3 shadow-sm dark:border-dark-700 dark:bg-dark-800">
          <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
            {{ t('admin.governance.status.primaryRole') }}
          </p>
          <p class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">{{ status.primary_role }}</p>
        </div>
        <div class="rounded-lg border border-gray-100 bg-white px-4 py-3 shadow-sm dark:border-dark-700 dark:bg-dark-800">
          <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
            {{ t('admin.governance.status.riskTier') }}
          </p>
          <p class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">{{ status.risk_tier }}</p>
        </div>
        <div class="rounded-lg border border-gray-100 bg-white px-4 py-3 shadow-sm dark:border-dark-700 dark:bg-dark-800">
          <p class="text-xs font-medium text-gray-500 dark:text-gray-400">
            {{ t('admin.governance.status.capabilities') }}
          </p>
          <div class="mt-2 flex flex-wrap gap-1.5">
            <span
              v-for="cap in enabledCapabilities"
              :key="cap"
              class="inline-flex items-center rounded-full bg-green-100 px-2 py-0.5 text-xs font-medium text-green-700 dark:bg-green-900/30 dark:text-green-300"
            >
              {{ t(`admin.governance.capability.${cap}`) }}
            </span>
          </div>
        </div>
      </div>

      <!-- Tab 导航 -->
      <div class="border-b border-gray-200 dark:border-dark-700">
        <nav class="-mb-px flex flex-wrap gap-x-6" aria-label="Tabs">
          <button
            v-for="tab in tabs"
            :key="tab.key"
            type="button"
            class="whitespace-nowrap border-b-2 px-1 py-3 text-sm font-medium transition-colors"
            :class="
              activeTab === tab.key
                ? 'border-primary-600 text-primary-600 dark:border-primary-400 dark:text-primary-400'
                : 'border-transparent text-gray-500 hover:border-gray-300 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'
            "
            @click="switchTab(tab.key)"
          >
            {{ tab.label }}
          </button>
        </nav>
      </div>

      <!-- ========== 审计日志 ========== -->
      <section v-show="activeTab === 'audit'" class="space-y-4">
        <div class="flex flex-wrap items-end gap-3">
          <div>
            <label class="mb-1 block text-xs font-medium text-gray-500 dark:text-gray-400">
              {{ t('admin.governance.audit.complianceType') }}
            </label>
            <input
              v-model="auditFilters.compliance_type"
              type="text"
              class="input w-48 text-sm"
              :placeholder="t('admin.governance.audit.complianceTypePlaceholder')"
            />
          </div>
          <div>
            <label class="mb-1 block text-xs font-medium text-gray-500 dark:text-gray-400">
              {{ t('admin.governance.audit.subjectType') }}
            </label>
            <input
              v-model="auditFilters.subject_type"
              type="text"
              class="input w-40 text-sm"
              :placeholder="t('admin.governance.audit.subjectTypePlaceholder')"
            />
          </div>
          <button type="button" class="btn btn-primary text-sm" @click="loadAuditLogs(1)">
            {{ t('admin.governance.audit.search') }}
          </button>
          <button type="button" class="btn btn-secondary text-sm" @click="resetAuditFilters">
            {{ t('admin.governance.audit.reset') }}
          </button>
        </div>

        <div class="card overflow-hidden">
          <DataTable :columns="auditColumns" :data="auditLogs" :loading="auditLoading">
            <template #cell-subject_id="{ row }">
              {{ (row as ComplianceAuditLog).subject_id ?? '-' }}
            </template>
            <template #cell-created_at="{ row }">
              {{ formatDateTime((row as ComplianceAuditLog).created_at) }}
            </template>
            <template #cell-details="{ row }">
              <span class="block max-w-md truncate" :title="(row as ComplianceAuditLog).details">
                {{ (row as ComplianceAuditLog).details }}
              </span>
            </template>
          </DataTable>
          <Pagination
            v-if="auditTotal > 0"
            :total="auditTotal"
            :page="auditPage"
            :page-size="auditPageSize"
            @update:page="loadAuditLogs"
            @update:page-size="onAuditPageSizeChange"
          />
        </div>
      </section>

      <!-- ========== 风险分析 ========== -->
      <section v-show="activeTab === 'risk'" class="space-y-4">
        <div v-if="riskCatalog" class="grid grid-cols-1 gap-6 lg:grid-cols-2">
          <div class="card p-5">
            <h3 class="mb-3 text-base font-semibold text-gray-900 dark:text-white">
              {{ t('admin.governance.risk.modelTags') }}
            </h3>
            <ul class="space-y-2">
              <li
                v-for="item in riskCatalog.model_tags"
                :key="item.tag"
                class="flex items-start gap-3 rounded-md bg-gray-50 px-3 py-2 dark:bg-dark-900"
              >
                <code class="rounded bg-blue-100 px-1.5 py-0.5 text-xs font-medium text-blue-700 dark:bg-blue-900/30 dark:text-blue-300">
                  {{ getRiskTagLabel(item.tag) }}
                </code>
                <span class="text-sm text-gray-600 dark:text-gray-300">{{ getRiskTagDescription(item.tag) }}</span>
              </li>
            </ul>
          </div>
          <div class="card p-5">
            <h3 class="mb-3 text-base font-semibold text-gray-900 dark:text-white">
              {{ t('admin.governance.risk.riskTags') }}
            </h3>
            <ul class="space-y-2">
              <li
                v-for="item in riskCatalog.risk_tags"
                :key="item.tag"
                class="flex items-start gap-3 rounded-md bg-gray-50 px-3 py-2 dark:bg-dark-900"
              >
                <code class="rounded bg-amber-100 px-1.5 py-0.5 text-xs font-medium text-amber-700 dark:bg-amber-900/30 dark:text-amber-300">
                  {{ getRiskTagLabel(item.tag) }}
                </code>
                <span class="text-sm text-gray-600 dark:text-gray-300">{{ getRiskTagDescription(item.tag) }}</span>
              </li>
            </ul>
          </div>
        </div>
        <div v-else-if="riskLoading" class="flex justify-center py-10">
          <div class="h-6 w-6 animate-spin rounded-full border-b-2 border-primary-600"></div>
        </div>
      </section>

      <!-- ========== EU AI Act 评估报告 ========== -->
      <section v-show="activeTab === 'euaiact'" class="space-y-4">
        <div class="flex flex-wrap items-center gap-2">
          <button type="button" class="btn btn-secondary text-sm" :disabled="euLoading" @click="loadEUAIAct">
            <Icon name="refresh" size="sm" :class="euLoading ? 'animate-spin' : ''" />
            {{ t('admin.governance.refresh') }}
          </button>
          <button type="button" class="btn btn-primary text-sm" :disabled="euExporting" @click="exportEUAIAct">
            <Icon name="download" size="sm" />
            {{ t('admin.governance.euAiAct.export') }}
          </button>
        </div>
        <div class="card p-5">
          <pre v-if="euReport" class="overflow-auto whitespace-pre-wrap break-words text-xs text-gray-700 dark:text-gray-300">{{ prettyJSON(euReport) }}</pre>
          <div v-else-if="euLoading" class="flex justify-center py-10">
            <div class="h-6 w-6 animate-spin rounded-full border-b-2 border-primary-600"></div>
          </div>
          <p v-else class="py-8 text-center text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.governance.euAiAct.empty') }}
          </p>
        </div>
      </section>

      <!-- ========== GDPR 数据处理记录 (ROPA) ========== -->
      <section v-show="activeTab === 'ropa'" class="space-y-4">
        <div class="flex flex-wrap items-center gap-2">
          <button type="button" class="btn btn-secondary text-sm" :disabled="ropaLoading" @click="loadROPA">
            <Icon name="refresh" size="sm" :class="ropaLoading ? 'animate-spin' : ''" />
            {{ t('admin.governance.refresh') }}
          </button>
        </div>
        <div class="card p-5">
          <pre v-if="ropaRecord" class="overflow-auto whitespace-pre-wrap break-words text-xs text-gray-700 dark:text-gray-300">{{ prettyJSON(ropaRecord) }}</pre>
          <div v-else-if="ropaLoading" class="flex justify-center py-10">
            <div class="h-6 w-6 animate-spin rounded-full border-b-2 border-primary-600"></div>
          </div>
          <p v-else class="py-8 text-center text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.governance.ropa.empty') }}
          </p>
        </div>
      </section>

      <!-- ========== GDPR 删除请求 ========== -->
      <section v-show="activeTab === 'erasure'" class="space-y-4">
        <div class="flex flex-wrap items-end gap-3">
          <div>
            <label class="mb-1 block text-xs font-medium text-gray-500 dark:text-gray-400">
              {{ t('admin.governance.erasure.status') }}
            </label>
            <select v-model="erasureFilters.status" class="input w-40 text-sm">
              <option value="">{{ t('admin.governance.erasure.statusAll') }}</option>
              <option value="pending">{{ t('admin.governance.erasure.statusPending') }}</option>
              <option value="approved">{{ t('admin.governance.erasure.statusApproved') }}</option>
              <option value="rejected">{{ t('admin.governance.erasure.statusRejected') }}</option>
              <option value="completed">{{ t('admin.governance.erasure.statusCompleted') }}</option>
            </select>
          </div>
          <button type="button" class="btn btn-primary text-sm" @click="loadErasureRequests(1)">
            {{ t('admin.governance.audit.search') }}
          </button>
        </div>

        <div class="card overflow-hidden">
          <DataTable :columns="erasureColumns" :data="erasureRequests" :loading="erasureLoading" :actions-count="2">
            <template #cell-status="{ row }">
              <span
                class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium"
                :class="statusBadgeClass((row as DataErasureRequest).status)"
              >
                {{ (row as DataErasureRequest).status }}
              </span>
            </template>
            <template #cell-requested_at="{ row }">
              {{ formatDateTime((row as DataErasureRequest).requested_at) }}
            </template>
            <template #cell-actions="{ row }">
              <div v-if="(row as DataErasureRequest).status === 'pending'" class="flex items-center gap-2">
                <button
                  type="button"
                  class="text-xs font-medium text-green-600 hover:text-green-700 dark:text-green-400"
                  @click="openProcessDialog(row as DataErasureRequest, true)"
                >
                  {{ t('admin.governance.erasure.approve') }}
                </button>
                <button
                  type="button"
                  class="text-xs font-medium text-red-600 hover:text-red-700 dark:text-red-400"
                  @click="openProcessDialog(row as DataErasureRequest, false)"
                >
                  {{ t('admin.governance.erasure.reject') }}
                </button>
              </div>
              <span v-else class="text-xs text-gray-400">-</span>
            </template>
          </DataTable>
          <Pagination
            v-if="erasureTotal > 0"
            :total="erasureTotal"
            :page="erasurePage"
            :page-size="erasurePageSize"
            @update:page="loadErasureRequests"
            @update:page-size="onErasurePageSizeChange"
          />
        </div>
      </section>

      <!-- ========== 合规模板 ========== -->
      <section v-show="activeTab === 'templates'" class="space-y-4">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <button type="button" class="btn btn-secondary text-sm" :disabled="templatesLoading" @click="loadTemplates">
            <Icon name="refresh" size="sm" :class="templatesLoading ? 'animate-spin' : ''" />
            {{ t('admin.governance.refresh') }}
          </button>
          <button type="button" class="btn btn-primary text-sm" @click="openCreateTemplateDialog">
            <Icon name="plus" size="sm" />
            {{ t('admin.governance.templates.create') }}
          </button>
        </div>

        <div v-if="templatesLoading" class="flex justify-center py-10">
          <div class="h-6 w-6 animate-spin rounded-full border-b-2 border-primary-600"></div>
        </div>
        <div v-else-if="templates.length === 0" class="card p-8 text-center text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.governance.templates.empty') }}
        </div>
        <div v-else class="grid grid-cols-1 gap-4 lg:grid-cols-2 xl:grid-cols-3">
          <div
            v-for="tpl in templates"
            :key="tpl.template_code"
            class="card flex flex-col gap-3 p-5"
          >
            <div class="flex items-start justify-between gap-2">
              <div>
                <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ tpl.industry }}</h3>
                <code class="mt-0.5 inline-block rounded bg-gray-100 px-1.5 py-0.5 text-xs text-gray-600 dark:bg-dark-700 dark:text-gray-300">
                  {{ tpl.template_code }}
                </code>
              </div>
              <span
                v-if="activeTemplateCode === tpl.template_code"
                class="inline-flex items-center rounded-full bg-green-100 px-2.5 py-1 text-xs font-medium text-green-700 dark:bg-green-900/30 dark:text-green-300"
              >
                {{ t('admin.governance.templates.active') }}
              </span>
              <button
                v-else
                type="button"
                class="btn btn-primary btn-sm text-xs"
                :disabled="applyingCode === tpl.template_code"
                @click="applyTemplate(tpl)"
              >
                {{ t('admin.governance.templates.apply') }}
              </button>
            </div>
            <p class="text-sm text-gray-600 dark:text-gray-300">{{ tpl.description }}</p>
            <div v-if="tpl.risk_tags.length" class="flex flex-wrap gap-1.5">
              <span
                v-for="tag in tpl.risk_tags"
                :key="tag"
                class="inline-flex items-center rounded-full bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-700 dark:bg-amber-900/30 dark:text-amber-300"
              >
                {{ tag }}
              </span>
            </div>
            <div v-if="tpl.rules.length" class="mt-1 rounded-md bg-gray-50 p-3 dark:bg-dark-900">
              <p class="mb-1 text-xs font-medium text-gray-500 dark:text-gray-400">
                {{ t('admin.governance.templates.rules') }}
              </p>
              <ul class="space-y-0.5 text-xs text-gray-600 dark:text-gray-300">
                <li v-for="(rule, idx) in tpl.rules" :key="idx" class="font-mono">
                  {{ rule.name }} = {{ String(rule.value) }}
                </li>
              </ul>
            </div>
          </div>
        </div>
      </section>

      <!-- ========== 内容审核规则 ========== -->
      <section v-show="activeTab === 'rules'" class="space-y-4">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div class="flex items-center gap-2">
            <label class="text-xs font-medium text-gray-500 dark:text-gray-400">
              {{ t('admin.governance.rules.strategy') }}
            </label>
            <select v-model="ruleStrategy" class="input w-40 text-sm" @change="onStrategyChange">
              <option value="OR">OR</option>
              <option value="AND">AND</option>
              <option value="WEIGHTED">WEIGHTED</option>
            </select>
          </div>
          <div class="flex items-center gap-2">
            <button type="button" class="btn btn-secondary text-sm" :disabled="rulesLoading" @click="loadRules">
              <Icon name="refresh" size="sm" :class="rulesLoading ? 'animate-spin' : ''" />
              {{ t('admin.governance.refresh') }}
            </button>
            <button type="button" class="btn btn-primary text-sm" @click="openRuleDialog()">
              <Icon name="plus" size="sm" />
              {{ t('admin.governance.rules.create') }}
            </button>
          </div>
        </div>

        <div class="card overflow-hidden">
          <DataTable :columns="ruleColumns" :data="rules" :loading="rulesLoading" :actions-count="2">
            <template #cell-rule_type="{ row }">
              <code class="rounded bg-blue-100 px-1.5 py-0.5 text-xs font-medium text-blue-700 dark:bg-blue-900/30 dark:text-blue-300">
                {{ (row as ModerationRule).rule_type }}
              </code>
            </template>
            <template #cell-action="{ row }">
              <span
                class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium"
                :class="ruleActionBadgeClass((row as ModerationRule).action)"
              >
                {{ (row as ModerationRule).action }}
              </span>
            </template>
            <template #cell-enabled="{ row }">
              <span
                class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium"
                :class="(row as ModerationRule).enabled
                  ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300'
                  : 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-gray-400'"
              >
                {{ (row as ModerationRule).enabled ? t('admin.governance.rules.enabled') : t('admin.governance.rules.disabled') }}
              </span>
            </template>
            <template #cell-rule_pattern="{ row }">
              <code class="block max-w-xs truncate text-xs" :title="(row as ModerationRule).rule_pattern">
                {{ (row as ModerationRule).rule_pattern }}
              </code>
            </template>
            <template #cell-actions="{ row }">
              <div class="flex items-center gap-2">
                <button
                  type="button"
                  class="text-xs font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400"
                  @click="openRuleDialog(row as ModerationRule)"
                >
                  {{ t('common.edit') }}
                </button>
                <button
                  type="button"
                  class="text-xs font-medium text-red-600 hover:text-red-700 dark:text-red-400"
                  @click="deleteRule(row as ModerationRule)"
                >
                  {{ t('common.delete') }}
                </button>
              </div>
            </template>
          </DataTable>
        </div>
      </section>

      <!-- 跨法域合规映射 -->
      <section v-show="activeTab === 'jurisdiction'" class="space-y-4">
        <div class="card p-4">
          <div class="grid gap-4 sm:grid-cols-3">
            <div>
              <label class="mb-1 block text-xs font-medium text-gray-500 dark:text-gray-400">
                {{ t('admin.governance.jurisdiction.region') }}
              </label>
              <select v-model="jurisdiction.region" class="input w-full text-sm">
                <option value="EU">EU</option>
                <option value="China">China</option>
                <option value="US-California">US-California</option>
              </select>
            </div>
            <div>
              <label class="mb-1 block text-xs font-medium text-gray-500 dark:text-gray-400">
                {{ t('admin.governance.jurisdiction.industry') }}
              </label>
              <input
                v-model="jurisdiction.industry"
                type="text"
                class="input w-full text-sm"
                :placeholder="t('admin.governance.jurisdiction.industryPlaceholder')"
              />
            </div>
            <div>
              <label class="mb-1 block text-xs font-medium text-gray-500 dark:text-gray-400">
                {{ t('admin.governance.jurisdiction.serviceType') }}
              </label>
              <select v-model="jurisdiction.serviceType" class="input w-full text-sm">
                <option value="">--</option>
                <option value="ai_chatbot">ai_chatbot</option>
                <option value="ai_analysis">ai_analysis</option>
                <option value="ai_recommendation">ai_recommendation</option>
              </select>
            </div>
          </div>
          <div class="mt-4">
            <button type="button" class="btn btn-primary text-sm" :disabled="jurisdiction.loading" @click="runJurisdictionMapping">
              <Icon name="refresh" size="sm" :class="jurisdiction.loading ? 'animate-spin' : ''" />
              {{ t('admin.governance.jurisdiction.map') }}
            </button>
          </div>
          <div class="mt-4 rounded-lg bg-gray-50 p-3 text-xs text-gray-600 dark:bg-dark-900 dark:text-gray-400">
            <h4 class="mb-2 font-semibold">{{ t('admin.governance.jurisdiction.fieldHelp') }}</h4>
            <ul class="space-y-1.5">
              <li><span class="font-medium">Region:</span> {{ t('admin.governance.jurisdiction.fieldHelpRegion') }}</li>
              <li><span class="font-medium">Industry:</span> {{ t('admin.governance.jurisdiction.fieldHelpIndustry') }}</li>
              <li><span class="font-medium">Service Type:</span> {{ t('admin.governance.jurisdiction.fieldHelpServiceType') }}</li>
            </ul>
          </div>
        </div>

        <div v-if="jurisdiction.result" class="card space-y-4 p-4">
          <div class="flex items-center gap-2">
            <span class="text-sm font-medium text-gray-700 dark:text-gray-200">
              {{ t('admin.governance.jurisdiction.riskLevel') }}:
            </span>
            <span
              class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium"
              :class="jurisdictionRiskBadgeClass(jurisdiction.result.risk_level)"
            >
              {{ jurisdiction.result.risk_level }}
            </span>
          </div>
          <div>
            <h4 class="mb-1 text-sm font-semibold text-gray-700 dark:text-gray-200">
              {{ t('admin.governance.jurisdiction.regulations') }}
            </h4>
            <ul class="list-inside list-disc space-y-0.5 text-sm text-gray-600 dark:text-gray-300">
              <li v-for="item in jurisdiction.result.applicable_regulations" :key="item">{{ item }}</li>
            </ul>
          </div>
          <div>
            <h4 class="mb-1 text-sm font-semibold text-gray-700 dark:text-gray-200">
              {{ t('admin.governance.jurisdiction.measures') }}
            </h4>
            <ul class="list-inside list-disc space-y-0.5 text-sm text-gray-600 dark:text-gray-300">
              <li v-for="item in jurisdiction.result.required_measures" :key="item">{{ item }}</li>
            </ul>
          </div>
          <div>
            <h4 class="mb-1 text-sm font-semibold text-gray-700 dark:text-gray-200">
              {{ t('admin.governance.jurisdiction.actions') }}
            </h4>
            <ul class="list-inside list-disc space-y-0.5 text-sm text-gray-600 dark:text-gray-300">
              <li v-for="item in jurisdiction.result.recommended_actions" :key="item">{{ item }}</li>
            </ul>
          </div>
        </div>
      </section>

      <!-- DPA 生成 -->
      <section v-show="activeTab === 'dpa'" class="space-y-4">
        <div class="card p-4">
          <h3 class="mb-4 text-sm font-semibold text-gray-700 dark:text-gray-200">
            {{ t('admin.governance.dpa.title') }}
          </h3>
          <div class="grid gap-4 sm:grid-cols-2">
            <div>
              <label class="mb-1 block text-xs font-medium text-gray-500 dark:text-gray-400">
                {{ t('admin.governance.dpa.controllerName') }}
                <span class="text-red-500">*</span>
              </label>
              <input
                v-model="dpaForm.controllerName"
                type="text"
                class="input w-full text-sm"
                :placeholder="t('admin.governance.dpa.controllerNamePlaceholder')"
              />
            </div>
            <div>
              <label class="mb-1 block text-xs font-medium text-gray-500 dark:text-gray-400">
                {{ t('admin.governance.dpa.controllerContact') }}
                <span class="text-red-500">*</span>
              </label>
              <input
                v-model="dpaForm.controllerContact"
                type="text"
                class="input w-full text-sm"
                :placeholder="t('admin.governance.dpa.controllerContactPlaceholder')"
              />
            </div>
          </div>
          <div class="mt-4">
            <button type="button" class="btn btn-primary text-sm" :disabled="dpaForm.loading" @click="submitDPA">
              <Icon name="download" size="sm" :class="dpaForm.loading ? 'animate-spin' : ''" />
              {{ t('admin.governance.dpa.generate') }}
            </button>
          </div>
        </div>
      </section>

      <!-- 合规凭证 -->
      <section v-show="activeTab === 'credentials'" class="space-y-4">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div class="flex items-center gap-2">
            <label class="text-xs font-medium text-gray-500 dark:text-gray-400">
              {{ t('admin.governance.credentials.status') }}
            </label>
            <select v-model="credentialsStatus" class="input w-32 text-sm" @change="loadCredentials">
              <option value="">--</option>
              <option value="active">active</option>
              <option value="revoked">revoked</option>
            </select>
          </div>
          <div class="flex items-center gap-2">
            <button type="button" class="btn btn-secondary text-sm" :disabled="credentialsLoading" @click="loadCredentials">
              <Icon name="refresh" size="sm" :class="credentialsLoading ? 'animate-spin' : ''" />
              {{ t('admin.governance.refresh') }}
            </button>
            <button type="button" class="btn btn-primary text-sm" @click="openCredentialDialog()">
              <Icon name="plus" size="sm" />
              {{ t('admin.governance.credentials.create') }}
            </button>
          </div>
        </div>

        <div class="card overflow-hidden">
          <DataTable :columns="credentialColumns" :data="credentials" :loading="credentialsLoading" :actions-count="2">
            <template #cell-status="{ row }">
              <span
                class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium"
                :class="(row as ComplianceCredential).status === 'active'
                  ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300'
                  : 'bg-gray-100 text-gray-700 dark:bg-gray-900/30 dark:text-gray-300'"
              >
                {{ (row as ComplianceCredential).status }}
              </span>
            </template>
            <template #cell-created_at="{ row }">
              {{ formatDateTime((row as ComplianceCredential).created_at) }}
            </template>
            <template #cell-valid_until="{ row }">
              {{ formatDateTime((row as ComplianceCredential).valid_until) }}
            </template>
            <template #cell-actions="{ row }">
              <button
                v-if="(row as ComplianceCredential).status === 'active'"
                type="button"
                class="btn btn-xs btn-secondary"
                @click="revokeCredential((row as ComplianceCredential).id)"
              >
                {{ t('admin.governance.credentials.revoke') }}
              </button>
              <button
                v-else
                type="button"
                class="btn btn-xs btn-success"
                @click="activateCredential((row as ComplianceCredential).id)"
              >
                {{ t('admin.governance.credentials.activate') }}
              </button>
              <button
                type="button"
                class="btn btn-xs btn-danger"
                @click="deleteCredentialConfirm((row as ComplianceCredential).id)"
              >
                {{ t('common.delete') }}
              </button>
            </template>
          </DataTable>
        </div>
      </section>
    </div>

    <!-- 删除请求处理对话框 -->
    <BaseDialog
      :show="processDialog.show"
      :title="
        processDialog.approved
          ? t('admin.governance.erasure.approveTitle')
          : t('admin.governance.erasure.rejectTitle')
      "
      width="normal"
      @close="processDialog.show = false"
    >
      <div class="space-y-4">
        <p class="text-sm text-gray-600 dark:text-gray-300">
          {{ t('admin.governance.erasure.confirmHint', { id: processDialog.id }) }}
        </p>
        <div v-if="!processDialog.approved">
          <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.governance.erasure.reason') }}
            <span class="text-red-500">*</span>
          </label>
          <textarea
            v-model="processDialog.reason"
            rows="3"
            class="input w-full text-sm"
            :placeholder="t('admin.governance.erasure.reasonPlaceholder')"
          ></textarea>
        </div>
      </div>
      <template #footer>
        <button type="button" class="btn btn-secondary" @click="processDialog.show = false">
          {{ t('common.cancel') }}
        </button>
        <button
          type="button"
          class="btn btn-primary"
          :disabled="processDialog.submitting"
          @click="submitProcess"
        >
          {{ t('common.confirm') }}
        </button>
      </template>
    </BaseDialog>

    <!-- 创建自定义模板对话框 -->
    <BaseDialog
      :show="templateDialog.show"
      :title="t('admin.governance.templates.createTitle')"
      width="normal"
      @close="templateDialog.show = false"
    >
      <div class="space-y-4">
        <div>
          <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.governance.templates.fieldCode') }} <span class="text-red-500">*</span>
          </label>
          <input v-model="templateDialog.template_code" type="text" class="input w-full text-sm" />
        </div>
        <div>
          <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.governance.templates.fieldIndustry') }} <span class="text-red-500">*</span>
          </label>
          <input v-model="templateDialog.industry" type="text" class="input w-full text-sm" />
        </div>
        <div>
          <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.governance.templates.fieldDescription') }}
          </label>
          <textarea v-model="templateDialog.description" rows="2" class="input w-full text-sm"></textarea>
        </div>
        <div>
          <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.governance.templates.fieldRules') }}
          </label>
          <textarea
            v-model="templateDialog.rulesJSON"
            rows="4"
            class="input w-full font-mono text-xs"
            :placeholder="'[{&quot;name&quot;: &quot;DATA_RETENTION_LIMIT&quot;, &quot;value&quot;: &quot;90_days&quot;}]'"
          ></textarea>
        </div>
        <div>
          <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.governance.templates.fieldRiskTags') }}
          </label>
          <input
            v-model="templateDialog.riskTagsText"
            type="text"
            class="input w-full text-sm"
            :placeholder="t('admin.governance.templates.riskTagsPlaceholder')"
          />
        </div>
      </div>
      <template #footer>
        <button type="button" class="btn btn-secondary" @click="templateDialog.show = false">
          {{ t('common.cancel') }}
        </button>
        <button
          type="button"
          class="btn btn-primary"
          :disabled="templateDialog.submitting"
          @click="submitCreateTemplate"
        >
          {{ t('common.confirm') }}
        </button>
      </template>
    </BaseDialog>

    <!-- 审核规则编辑对话框 -->
    <BaseDialog
      :show="ruleDialog.show"
      :title="ruleDialog.editing ? t('admin.governance.rules.editTitle') : t('admin.governance.rules.createTitle')"
      width="normal"
      @close="ruleDialog.show = false"
    >
      <div class="space-y-4">
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.governance.rules.ruleId') }} <span class="text-red-500">*</span>
            </label>
            <input
              v-model="ruleDialog.rule_id"
              type="text"
              class="input w-full text-sm"
              :disabled="ruleDialog.editing"
            />
          </div>
          <div>
            <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.governance.rules.ruleName') }} <span class="text-red-500">*</span>
            </label>
            <input v-model="ruleDialog.rule_name" type="text" class="input w-full text-sm" />
          </div>
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.governance.rules.ruleType') }}
            </label>
            <select v-model="ruleDialog.rule_type" class="input w-full text-sm">
              <option value="KEYWORD">KEYWORD</option>
              <option value="REGEX">REGEX</option>
              <option value="PATTERN">PATTERN</option>
            </select>
          </div>
          <div>
            <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.governance.rules.action') }}
            </label>
            <select v-model="ruleDialog.action" class="input w-full text-sm">
              <option value="BLOCK">BLOCK</option>
              <option value="REVIEW">REVIEW</option>
              <option value="ALLOW">ALLOW</option>
            </select>
          </div>
        </div>
        <div>
          <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.governance.rules.rulePattern') }} <span class="text-red-500">*</span>
          </label>
          <input
            v-model="ruleDialog.rule_pattern"
            type="text"
            class="input w-full font-mono text-sm"
            :placeholder="t('admin.governance.rules.patternPlaceholder')"
          />
        </div>
        <div class="grid grid-cols-3 gap-3">
          <div>
            <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.governance.rules.threshold') }}
            </label>
            <input v-model.number="ruleDialog.threshold" type="number" step="0.1" min="0" max="1" class="input w-full text-sm" />
          </div>
          <div>
            <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.governance.rules.priority') }}
            </label>
            <input v-model.number="ruleDialog.priority" type="number" min="1" class="input w-full text-sm" />
          </div>
          <div>
            <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.governance.rules.riskCategory') }}
            </label>
            <input v-model="ruleDialog.risk_category" type="text" class="input w-full text-sm" />
          </div>
        </div>
        <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
          <input v-model="ruleDialog.enabled" type="checkbox" class="rounded" />
          {{ t('admin.governance.rules.enabledLabel') }}
        </label>
      </div>
      <template #footer>
        <button type="button" class="btn btn-secondary" @click="ruleDialog.show = false">
          {{ t('common.cancel') }}
        </button>
        <button
          type="button"
          class="btn btn-primary"
          :disabled="ruleDialog.submitting"
          @click="submitRule"
        >
          {{ t('common.confirm') }}
        </button>
      </template>
    </BaseDialog>

    <!-- 创建合规凭证对话框 -->
    <BaseDialog
      :show="credentialDialog.show"
      :title="t('admin.governance.credentials.create')"
      width="normal"
      @close="credentialDialog.show = false"
    >
      <div class="space-y-4">
        <div>
          <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.governance.credentials.credentialId') }} <span class="text-red-500">*</span>
          </label>
          <input v-model="credentialDialog.credential_id" type="text" class="input w-full text-sm" />
        </div>
        <div>
          <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.governance.credentials.type') }} <span class="text-red-500">*</span>
          </label>
          <input v-model="credentialDialog.credential_type" type="text" class="input w-full text-sm" />
        </div>
        <div>
          <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.governance.credentials.issuer') }} <span class="text-red-500">*</span>
          </label>
          <input v-model="credentialDialog.issuer" type="text" class="input w-full text-sm" />
        </div>
        <div>
          <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.governance.credentials.validUntil') }}
          </label>
          <input v-model="credentialDialog.valid_until" type="datetime-local" class="input w-full text-sm" />
        </div>
      </div>
      <template #footer>
        <button type="button" class="btn btn-secondary" @click="credentialDialog.show = false">
          {{ t('common.cancel') }}
        </button>
        <button
          type="button"
          class="btn btn-primary"
          :disabled="credentialDialog.submitting"
          @click="submitCredential"
        >
          {{ t('common.confirm') }}
        </button>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import type { Column } from '@/components/common/types'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime as formatDateTimeValue } from '@/utils/format'
import {
  governanceAPI,
  type GovernanceStatus,
  type ComplianceAuditLog,
  type RiskTagsCatalog,
  type EUAIActAssessment,
  type DataProcessingRecord,
  type DataErasureRequest,
  type CompliancePolicyTemplate,
  type ModerationRule,
  type JurisdictionMappingResult,
  type ComplianceCredential,
} from '@/api/admin/governance'

const { t } = useI18n()
const appStore = useAppStore()

const getRiskTagLabel = (tagName: string) => {
  return t(`compliance.risk.tags.${tagName}.label`)
}

const getRiskTagDescription = (tagName: string) => {
  return t(`compliance.risk.tags.${tagName}.description`)
}

type TabKey = 'audit' | 'risk' | 'euaiact' | 'ropa' | 'erasure' | 'templates' | 'rules' | 'jurisdiction' | 'dpa' | 'credentials'
const activeTab = ref<TabKey>('audit')

const tabs = computed<{ key: TabKey; label: string }[]>(() => [
  { key: 'audit', label: t('admin.governance.tabs.audit') },
  { key: 'risk', label: t('admin.governance.tabs.risk') },
  { key: 'euaiact', label: t('admin.governance.tabs.euAiAct') },
  { key: 'ropa', label: t('admin.governance.tabs.ropa') },
  { key: 'erasure', label: t('admin.governance.tabs.erasure') },
  { key: 'templates', label: t('admin.governance.tabs.templates') },
  { key: 'rules', label: t('admin.governance.tabs.rules') },
  { key: 'jurisdiction', label: t('admin.governance.tabs.jurisdiction') },
  { key: 'dpa', label: t('admin.governance.tabs.dpa') },
  { key: 'credentials', label: t('admin.governance.tabs.credentials') },
])

function formatDateTime(value?: string): string {
  if (!value) return '-'
  return formatDateTimeValue(value)
}

function prettyJSON(value: unknown): string {
  return JSON.stringify(value, null, 2)
}

// ==================== 模块状态 ====================
const status = ref<GovernanceStatus | null>(null)
const statusLoading = ref(false)

const enabledCapabilities = computed<string[]>(() => {
  if (!status.value) return []
  return Object.entries(status.value.capabilities)
    .filter(([, v]) => v)
    .map(([k]) => k)
})

async function loadStatus() {
  statusLoading.value = true
  try {
    status.value = await governanceAPI.getStatus()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.governance.loadFailed')))
  } finally {
    statusLoading.value = false
  }
}

// ==================== 审计日志 ====================
const auditColumns = computed<Column[]>(() => [
  { key: 'id', label: 'ID' },
  { key: 'compliance_type', label: t('admin.governance.audit.complianceType') },
  { key: 'subject_type', label: t('admin.governance.audit.subjectType') },
  { key: 'subject_id', label: t('admin.governance.audit.subjectId') },
  { key: 'details', label: t('admin.governance.audit.details') },
  { key: 'operator', label: t('admin.governance.audit.operator') },
  { key: 'created_at', label: t('admin.governance.audit.createdAt') },
])
const auditLogs = ref<ComplianceAuditLog[]>([])
const auditLoading = ref(false)
const auditTotal = ref(0)
const auditPage = ref(1)
const auditPageSize = ref(20)
const auditFilters = reactive({ compliance_type: '', subject_type: '' })

async function loadAuditLogs(page = auditPage.value) {
  auditLoading.value = true
  try {
    const result = await governanceAPI.listAuditLogs({
      page,
      page_size: auditPageSize.value,
      compliance_type: auditFilters.compliance_type || undefined,
      subject_type: auditFilters.subject_type || undefined,
    })
    auditLogs.value = result.items ?? []
    auditTotal.value = result.total
    auditPage.value = result.page
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.governance.loadFailed')))
  } finally {
    auditLoading.value = false
  }
}

function onAuditPageSizeChange(size: number) {
  auditPageSize.value = size
  loadAuditLogs(1)
}

function resetAuditFilters() {
  auditFilters.compliance_type = ''
  auditFilters.subject_type = ''
  loadAuditLogs(1)
}

// ==================== 风险分析 ====================
const riskCatalog = ref<RiskTagsCatalog | null>(null)
const riskLoading = ref(false)

async function loadRiskTags() {
  if (riskCatalog.value) return
  riskLoading.value = true
  try {
    riskCatalog.value = await governanceAPI.getRiskTags()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.governance.loadFailed')))
  } finally {
    riskLoading.value = false
  }
}

// ==================== EU AI Act ====================
const euReport = ref<EUAIActAssessment | null>(null)
const euLoading = ref(false)
const euExporting = ref(false)

async function loadEUAIAct() {
  euLoading.value = true
  try {
    euReport.value = await governanceAPI.getEUAIActAssessment()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.governance.loadFailed')))
  } finally {
    euLoading.value = false
  }
}

async function exportEUAIAct() {
  euExporting.value = true
  try {
    const blob = await governanceAPI.exportEUAIActAssessment()
    downloadBlob(blob, `eu-ai-act-assessment-${Date.now()}.json`)
    appStore.showSuccess(t('admin.governance.euAiAct.exportSuccess'))
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.governance.euAiAct.exportFailed')))
  } finally {
    euExporting.value = false
  }
}

function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}

// ==================== GDPR ROPA ====================
const ropaRecord = ref<DataProcessingRecord | null>(null)
const ropaLoading = ref(false)

async function loadROPA() {
  ropaLoading.value = true
  try {
    ropaRecord.value = await governanceAPI.getDataProcessingRecord()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.governance.loadFailed')))
  } finally {
    ropaLoading.value = false
  }
}

// ==================== GDPR 删除请求 ====================
const erasureColumns = computed<Column[]>(() => [
  { key: 'id', label: 'ID' },
  { key: 'user_id', label: t('admin.governance.erasure.userId') },
  { key: 'request_type', label: t('admin.governance.erasure.requestType') },
  { key: 'status', label: t('admin.governance.erasure.status') },
  { key: 'requested_at', label: t('admin.governance.erasure.requestedAt') },
  { key: 'actions', label: t('admin.governance.erasure.actions') },
])
const erasureRequests = ref<DataErasureRequest[]>([])
const erasureLoading = ref(false)
const erasureTotal = ref(0)
const erasurePage = ref(1)
const erasurePageSize = ref(20)
const erasureFilters = reactive({ status: '' })

async function loadErasureRequests(page = erasurePage.value) {
  erasureLoading.value = true
  try {
    const result = await governanceAPI.listErasureRequests({
      page,
      page_size: erasurePageSize.value,
      status: erasureFilters.status || undefined,
    })
    erasureRequests.value = result.items ?? []
    erasureTotal.value = result.total
    erasurePage.value = result.page
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.governance.loadFailed')))
  } finally {
    erasureLoading.value = false
  }
}

function onErasurePageSizeChange(size: number) {
  erasurePageSize.value = size
  loadErasureRequests(1)
}

function statusBadgeClass(s: string): string {
  switch (s) {
    case 'pending':
      return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
    case 'approved':
    case 'completed':
      return 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300'
    case 'rejected':
      return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
    default:
      return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
  }
}

const processDialog = reactive({
  show: false,
  id: 0,
  approved: true,
  reason: '',
  submitting: false,
})

function openProcessDialog(row: DataErasureRequest, approved: boolean) {
  processDialog.id = row.id
  processDialog.approved = approved
  processDialog.reason = ''
  processDialog.show = true
}

async function submitProcess() {
  if (!processDialog.approved && !processDialog.reason.trim()) {
    appStore.showError(t('admin.governance.erasure.reasonRequired'))
    return
  }
  processDialog.submitting = true
  try {
    await governanceAPI.processErasureRequest(processDialog.id, {
      approved: processDialog.approved,
      reason: processDialog.reason.trim() || undefined,
    })
    appStore.showSuccess(t('admin.governance.erasure.processSuccess'))
    processDialog.show = false
    loadErasureRequests(erasurePage.value)
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.governance.erasure.processFailed')))
  } finally {
    processDialog.submitting = false
  }
}

// ==================== 合规模板 ====================
const templates = ref<CompliancePolicyTemplate[]>([])
const templatesLoading = ref(false)
const applyingCode = ref('')
const activeTemplateCode = ref('')

async function loadTemplates() {
  templatesLoading.value = true
  try {
    const result = await governanceAPI.getComplianceTemplates()
    templates.value = result.items
    activeTemplateCode.value = result.active_template_code
    if (activeTemplateCode.value && !jurisdiction.industry) {
      const activeTemplate = templates.value.find(t => t.template_code === activeTemplateCode.value)
      if (activeTemplate && activeTemplate.industry) {
        jurisdiction.industry = activeTemplate.industry
      }
    }
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.governance.loadFailed')))
  } finally {
    templatesLoading.value = false
  }
}

async function applyTemplate(tpl: CompliancePolicyTemplate) {
  applyingCode.value = tpl.template_code
  try {
    await governanceAPI.applyComplianceTemplate(tpl.template_code)
    appStore.showSuccess(t('admin.governance.templates.applySuccess'))
    activeTemplateCode.value = tpl.template_code
    jurisdiction.industry = tpl.industry
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.governance.templates.applyFailed')))
  } finally {
    applyingCode.value = ''
  }
}

const templateDialog = reactive({
  show: false,
  template_code: '',
  industry: '',
  description: '',
  rulesJSON: '',
  riskTagsText: '',
  submitting: false,
})

function openCreateTemplateDialog() {
  templateDialog.template_code = ''
  templateDialog.industry = ''
  templateDialog.description = ''
  templateDialog.rulesJSON = ''
  templateDialog.riskTagsText = ''
  templateDialog.show = true
}

async function submitCreateTemplate() {
  if (!templateDialog.template_code.trim() || !templateDialog.industry.trim()) {
    appStore.showError(t('admin.governance.templates.requiredFields'))
    return
  }
  let rules: Array<Record<string, unknown>> = []
  if (templateDialog.rulesJSON.trim()) {
    try {
      const parsed = JSON.parse(templateDialog.rulesJSON)
      if (!Array.isArray(parsed)) throw new Error('not array')
      rules = parsed
    } catch {
      appStore.showError(t('admin.governance.templates.invalidRules'))
      return
    }
  }
  const riskTags = templateDialog.riskTagsText
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean)
  templateDialog.submitting = true
  try {
    await governanceAPI.createCustomTemplate({
      template_code: templateDialog.template_code.trim(),
      industry: templateDialog.industry.trim(),
      description: templateDialog.description.trim() || undefined,
      rules,
      risk_tags: riskTags,
    })
    appStore.showSuccess(t('admin.governance.templates.createSuccess'))
    templateDialog.show = false
    loadTemplates()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.governance.templates.createFailed')))
  } finally {
    templateDialog.submitting = false
  }
}

// ==================== 内容审核规则 ====================
const ruleColumns = computed<Column[]>(() => [
  { key: 'priority', label: t('admin.governance.rules.priority') },
  { key: 'rule_id', label: t('admin.governance.rules.ruleId') },
  { key: 'rule_name', label: t('admin.governance.rules.ruleName') },
  { key: 'rule_type', label: t('admin.governance.rules.ruleType') },
  { key: 'rule_pattern', label: t('admin.governance.rules.rulePattern') },
  { key: 'action', label: t('admin.governance.rules.action') },
  { key: 'enabled', label: t('admin.governance.rules.enabledLabel') },
  { key: 'actions', label: t('admin.governance.erasure.actions') },
])
const rules = ref<ModerationRule[]>([])
const rulesLoading = ref(false)
const ruleStrategy = ref('OR')

async function loadRules() {
  rulesLoading.value = true
  try {
    rules.value = await governanceAPI.listModerationRules()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.governance.loadFailed')))
  } finally {
    rulesLoading.value = false
  }
}

async function onStrategyChange() {
  try {
    await governanceAPI.setModerationStrategy(ruleStrategy.value)
    appStore.showSuccess(t('admin.governance.rules.strategySuccess'))
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.governance.rules.strategyFailed')))
  }
}

function ruleActionBadgeClass(action: string): string {
  switch (action) {
    case 'BLOCK':
      return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
    case 'REVIEW':
      return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
    default:
      return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
  }
}

const ruleDialog = reactive({
  show: false,
  editing: false,
  rule_id: '',
  rule_name: '',
  rule_type: 'KEYWORD',
  rule_pattern: '',
  threshold: 0.8,
  action: 'BLOCK',
  risk_category: '',
  enabled: true,
  priority: 100,
  submitting: false,
})

function openRuleDialog(rule?: ModerationRule) {
  if (rule) {
    ruleDialog.editing = true
    ruleDialog.rule_id = rule.rule_id
    ruleDialog.rule_name = rule.rule_name
    ruleDialog.rule_type = rule.rule_type
    ruleDialog.rule_pattern = rule.rule_pattern
    ruleDialog.threshold = rule.threshold
    ruleDialog.action = rule.action
    ruleDialog.risk_category = rule.risk_category
    ruleDialog.enabled = rule.enabled
    ruleDialog.priority = rule.priority
  } else {
    ruleDialog.editing = false
    ruleDialog.rule_id = ''
    ruleDialog.rule_name = ''
    ruleDialog.rule_type = 'KEYWORD'
    ruleDialog.rule_pattern = ''
    ruleDialog.threshold = 0.8
    ruleDialog.action = 'BLOCK'
    ruleDialog.risk_category = ''
    ruleDialog.enabled = true
    ruleDialog.priority = 100
  }
  ruleDialog.show = true
}

async function submitRule() {
  if (!ruleDialog.rule_id.trim() || !ruleDialog.rule_name.trim() || !ruleDialog.rule_pattern.trim()) {
    appStore.showError(t('admin.governance.rules.requiredFields'))
    return
  }
  ruleDialog.submitting = true
  try {
    if (ruleDialog.editing) {
      await governanceAPI.updateModerationRule(ruleDialog.rule_id, {
        rule_name: ruleDialog.rule_name.trim(),
        rule_type: ruleDialog.rule_type,
        rule_pattern: ruleDialog.rule_pattern,
        threshold: ruleDialog.threshold,
        action: ruleDialog.action,
        risk_category: ruleDialog.risk_category.trim(),
        enabled: ruleDialog.enabled,
        priority: ruleDialog.priority,
      })
    } else {
      await governanceAPI.createModerationRule({
        rule_id: ruleDialog.rule_id.trim(),
        rule_name: ruleDialog.rule_name.trim(),
        rule_type: ruleDialog.rule_type,
        rule_pattern: ruleDialog.rule_pattern,
        threshold: ruleDialog.threshold,
        action: ruleDialog.action,
        risk_category: ruleDialog.risk_category.trim(),
        enabled: ruleDialog.enabled,
        priority: ruleDialog.priority,
      })
    }
    appStore.showSuccess(t('admin.governance.rules.saveSuccess'))
    ruleDialog.show = false
    loadRules()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.governance.rules.saveFailed')))
  } finally {
    ruleDialog.submitting = false
  }
}

async function deleteRule(rule: ModerationRule) {
  try {
    await governanceAPI.deleteModerationRule(rule.rule_id)
    appStore.showSuccess(t('admin.governance.rules.deleteSuccess'))
    loadRules()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.governance.rules.deleteFailed')))
  }
}

// ==================== 跨法域合规映射 ====================
const jurisdiction = reactive<{
  region: string
  industry: string
  serviceType: string
  loading: boolean
  result: JurisdictionMappingResult | null
}>({
  region: 'EU',
  industry: '',
  serviceType: '',
  loading: false,
  result: null,
})

async function runJurisdictionMapping() {
  jurisdiction.loading = true
  try {
    jurisdiction.result = await governanceAPI.getJurisdictionMapping({
      company_region: jurisdiction.region,
      industry: jurisdiction.industry.trim() || undefined,
      service_type: jurisdiction.serviceType || undefined,
    })
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.governance.jurisdiction.mapFailed')))
  } finally {
    jurisdiction.loading = false
  }
}

function jurisdictionRiskBadgeClass(level: string): string {
  switch (level) {
    case 'high':
      return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
    case 'medium':
      return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
    default:
      return 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300'
  }
}

// ==================== Tab 切换懒加载 ====================
function switchTab(tab: TabKey) {
  activeTab.value = tab
  switch (tab) {
    case 'audit':
      if (auditLogs.value.length === 0) loadAuditLogs(1)
      break
    case 'risk':
      loadRiskTags()
      break
    case 'euaiact':
      if (!euReport.value) loadEUAIAct()
      break
    case 'ropa':
      if (!ropaRecord.value) loadROPA()
      break
    case 'erasure':
      if (erasureRequests.value.length === 0) loadErasureRequests(1)
      break
    case 'templates':
      if (templates.value.length === 0) loadTemplates()
      break
    case 'rules':
      if (rules.value.length === 0) loadRules()
      break
    case 'jurisdiction':
      if (!jurisdiction.result) runJurisdictionMapping()
      break
    case 'dpa':
      break
    case 'credentials':
      if (credentials.value.length === 0) loadCredentials()
      break
  }
}

// ==================== DPA 生成 ====================
const dpaForm = reactive<{
  controllerName: string
  controllerContact: string
  loading: boolean
}>({
  controllerName: '',
  controllerContact: '',
  loading: false,
})

async function submitDPA() {
  if (!dpaForm.controllerName.trim() || !dpaForm.controllerContact.trim()) {
    appStore.showError(t('admin.governance.dpa.requiredFields'))
    return
  }
  dpaForm.loading = true
  try {
    await governanceAPI.generateDPA({
      controller_name: dpaForm.controllerName.trim(),
      controller_contact: dpaForm.controllerContact.trim(),
    })
    appStore.showSuccess(t('admin.governance.dpa.generateSuccess'))
    dpaForm.controllerName = ''
    dpaForm.controllerContact = ''
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.governance.dpa.generateFailed')))
  } finally {
    dpaForm.loading = false
  }
}

// ==================== 合规凭证 ====================
const credentials = ref<ComplianceCredential[]>([])
const credentialsLoading = ref(false)
const credentialsStatus = ref('')

const credentialDialog = reactive<{
  show: boolean
  credential_id: string
  credential_type: string
  issuer: string
  valid_until: string
  submitting: boolean
}>({
  show: false,
  credential_id: '',
  credential_type: '',
  issuer: '',
  valid_until: '',
  submitting: false,
})

function openCredentialDialog() {
  credentialDialog.show = true
  credentialDialog.credential_id = ''
  credentialDialog.credential_type = ''
  credentialDialog.issuer = ''
  credentialDialog.valid_until = ''
  credentialDialog.submitting = false
}

async function submitCredential() {
  if (!credentialDialog.credential_id.trim() || !credentialDialog.credential_type.trim() || !credentialDialog.issuer.trim()) {
    appStore.showError(t('admin.governance.credentials.requiredFields'))
    return
  }
  credentialDialog.submitting = true
  try {
    await governanceAPI.createCredential({
      credential_id: credentialDialog.credential_id.trim(),
      credential_type: credentialDialog.credential_type.trim(),
      issuer: credentialDialog.issuer.trim(),
      valid_until: credentialDialog.valid_until || undefined,
    })
    appStore.showSuccess(t('admin.governance.credentials.createSuccess'))
    credentialDialog.show = false
    loadCredentials()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.governance.credentials.createFailed')))
  } finally {
    credentialDialog.submitting = false
  }
}

const credentialColumns = computed<Column[]>(() => [
  { key: 'credential_id', label: t('admin.governance.credentials.credentialId'), sortable: true },
  { key: 'credential_type', label: t('admin.governance.credentials.type'), sortable: true },
  { key: 'issuer', label: t('admin.governance.credentials.issuer'), sortable: true },
  { key: 'status', label: t('admin.governance.credentials.status'), sortable: true },
  { key: 'valid_until', label: t('admin.governance.credentials.validUntil'), sortable: true },
  { key: 'created_at', label: t('admin.governance.credentials.createdAt'), sortable: true },
  { key: 'actions', label: t('common.actions') },
])

async function loadCredentials() {
  credentialsLoading.value = true
  try {
    credentials.value = await governanceAPI.listCredentials(undefined, credentialsStatus.value || undefined)
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.governance.credentials.loadFailed')))
  } finally {
    credentialsLoading.value = false
  }
}

async function revokeCredential(id: number) {
  try {
    await governanceAPI.revokeCredential(id)
    appStore.showSuccess(t('admin.governance.credentials.revokeSuccess'))
    loadCredentials()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.governance.credentials.revokeFailed')))
  }
}

async function activateCredential(id: number) {
  try {
    await governanceAPI.activateCredential(id)
    appStore.showSuccess(t('admin.governance.credentials.activateSuccess'))
    loadCredentials()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.governance.credentials.activateFailed')))
  }
}

async function deleteCredentialConfirm(id: number) {
  try {
    await governanceAPI.deleteCredential(id)
    appStore.showSuccess(t('admin.governance.credentials.deleteSuccess'))
    loadCredentials()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.governance.credentials.deleteFailed')))
  }
}

onMounted(() => {
  loadStatus()
  loadAuditLogs(1)
})
</script>
