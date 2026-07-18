<template>
  <AppLayout>
    <div class="mx-auto max-w-4xl space-y-6">
      <!-- 页面标题 -->
      <div class="flex items-center gap-3">
        <div class="rounded-xl bg-primary-100 p-3 text-primary-600 dark:bg-primary-900/30">
          <Icon name="shield" size="lg" />
        </div>
        <div>
          <h1 class="text-2xl font-bold text-gray-900 dark:text-white">
            {{ t('governance.title') }}
          </h1>
          <p class="text-sm text-gray-500 dark:text-gray-400">
            {{ t('governance.description') }}
          </p>
        </div>
      </div>

      <!-- Tab 切换 -->
      <div class="flex border-b border-gray-200 dark:border-dark-700">
        <button
          v-for="tab in tabs"
          :key="tab.key"
          class="relative px-4 py-3 text-sm font-medium transition-colors"
          :class="activeTab === tab.key ? 'text-primary-600 dark:text-primary-400' : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300'"
          @click="activeTab = tab.key"
        >
          {{ tab.label }}
          <span
            v-if="activeTab === tab.key"
            class="absolute bottom-0 left-0 right-0 h-0.5 bg-primary-600 dark:bg-primary-400"
          />
        </button>
      </div>

      <!-- Tab 内容 -->
      <div v-show="activeTab === 'compliance'">
        <!-- Account 合规配置 -->
        <div class="card p-6">
        <div class="flex items-start gap-3">
          <div class="rounded-xl bg-purple-100 p-3 text-purple-600 dark:bg-purple-900/30">
            <Icon name="shield" size="lg" />
          </div>
          <div class="flex-1">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('compliance.title') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('compliance.description') }}
            </p>

            <div v-if="complianceLoading" class="mt-4 flex items-center justify-center py-8">
              <span class="inline-block h-6 w-6 animate-spin rounded-full border-2 border-primary-600 border-t-transparent" />
            </div>

            <div v-else class="mt-4 space-y-6">
              <!-- 合规状态概览 -->
              <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-700 bg-blue-50/50 dark:bg-blue-900/10">
                <h3 class="text-base font-semibold text-gray-900 dark:text-white flex items-center gap-2">
                  <Icon name="checkCircle" size="md" class="text-green-600 dark:text-green-400" />
                  {{ t('compliance.status.title') }}
                </h3>
                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('compliance.status.description') }}</p>
                
                <div class="mt-4 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                  <div class="flex items-center gap-3 rounded-lg bg-white p-3 shadow-sm dark:bg-dark-700">
                    <div class="rounded-full bg-green-100 p-2 text-green-600 dark:bg-green-900/30 dark:text-green-400">
                      <Icon name="clipboard" size="sm" />
                    </div>
                    <div>
                      <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('compliance.template.title') }}</p>
                      <p class="text-sm font-medium text-gray-900 dark:text-white">
                        {{ templatesResult?.active_template_code ? t('compliance.template.industries.' + templatesResult.active_template_code + '.label') : t('compliance.template.notApplied') }}
                      </p>
                    </div>
                  </div>
                  
                  <div class="flex items-center gap-3 rounded-lg bg-white p-3 shadow-sm dark:bg-dark-700">
                    <div class="rounded-full bg-blue-100 p-2 text-blue-600 dark:bg-blue-900/30 dark:text-blue-400">
                      <Icon name="badge" size="sm" />
                    </div>
                    <div>
                      <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('compliance.zdr.title') }}</p>
                      <p class="text-sm font-medium text-gray-900 dark:text-white">
                        {{ t('compliance.zdr.' + profileForm.zdr_mode) }}
                      </p>
                    </div>
                  </div>
                  
                  <div class="flex items-center gap-3 rounded-lg bg-white p-3 shadow-sm dark:bg-dark-700">
                    <div class="rounded-full bg-purple-100 p-2 text-purple-600 dark:bg-purple-900/30 dark:text-purple-400">
                      <Icon name="shield" size="sm" />
                    </div>
                    <div>
                      <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('compliance.frameworks.title') }}</p>
                      <p class="text-sm font-medium text-gray-900 dark:text-white">
                        {{ profileForm.compliance_frameworks.length }} {{ t('compliance.frameworks.active') }}
                      </p>
                    </div>
                  </div>
                  
                  <div class="flex items-center gap-3 rounded-lg bg-white p-3 shadow-sm dark:bg-dark-700">
                    <div class="rounded-full bg-red-100 p-2 text-red-600 dark:bg-red-900/30 dark:text-red-400">
                      <Icon name="exclamationTriangle" size="sm" />
                    </div>
                    <div>
                      <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('compliance.moderation.title') }}</p>
                      <p class="text-sm font-medium text-gray-900 dark:text-white">
                        {{ profileForm.enabled_rule_ids.length }} {{ t('compliance.moderation.enabledRules') }}
                      </p>
                    </div>
                  </div>
                  
                  <div class="flex items-center gap-3 rounded-lg bg-white p-3 shadow-sm dark:bg-dark-700">
                    <div class="rounded-full bg-indigo-100 p-2 text-indigo-600 dark:bg-indigo-900/30 dark:text-indigo-400">
                      <Icon name="globe" size="sm" />
                    </div>
                    <div>
                      <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('compliance.jurisdiction.title') }}</p>
                      <p class="text-sm font-medium text-gray-900 dark:text-white">
                        {{ savedJurisdictionMapping && savedJurisdictionMapping.applied_rules?.length ? t('compliance.jurisdiction.applied') : t('compliance.jurisdiction.notApplied') }}
                      </p>
                    </div>
                  </div>
                  
                  <div class="flex items-center gap-3 rounded-lg bg-white p-3 shadow-sm dark:bg-dark-700">
                    <div class="rounded-full bg-amber-100 p-2 text-amber-600 dark:bg-amber-900/30 dark:text-amber-400">
                      <Icon name="checkCircle" size="sm" />
                    </div>
                    <div>
                      <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('dataRights.consent.title') }}</p>
                      <p class="text-sm font-medium text-gray-900 dark:text-white">
                        {{ grantedConsents.length }}/{{ consents.length }} {{ t('dataRights.consent.granted') }}
                      </p>
                    </div>
                  </div>
                </div>
              </div>

              <!-- 3.1 行业模板卡片 -->
              <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
                <h3 class="text-base font-semibold text-gray-900 dark:text-white">
                  {{ t('compliance.template.title') }}
                </h3>
                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                  {{ t('compliance.template.description') }}
                </p>

                <div v-if="currentTemplate" class="mt-3 rounded-lg bg-green-50 p-3 dark:bg-green-900/20">
                  <p class="text-sm text-green-700 dark:text-green-300">
                    {{ t('compliance.template.current') }}: <strong>{{ t('compliance.template.industries.' + currentTemplate.code + '.label') }}</strong>
                    <span class="ml-2 text-xs text-green-600 dark:text-green-400">{{ t('compliance.template.industries.' + currentTemplate.code + '.description') }}</span>
                  </p>
                </div>

                <div v-if="templatesLoading" class="mt-4 flex items-center justify-center py-4">
                  <span class="inline-block h-5 w-5 animate-spin rounded-full border-2 border-primary-600 border-t-transparent" />
                </div>

                <div v-else class="mt-3 space-y-2">
                  <div
                    v-for="template in availableTemplates"
                    :key="template.code"
                    class="flex items-center justify-between rounded-lg border border-gray-200 p-3 dark:border-dark-700"
                    :class="template.code === templatesResult?.active_template_code ? 'bg-primary-50 dark:bg-primary-900/20' : ''"
                  >
                    <div>
                      <p class="font-medium text-gray-900 dark:text-white">{{ t('compliance.template.industries.' + template.code + '.label') }}</p>
                      <p class="text-xs text-gray-500">{{ t('compliance.template.industries.' + template.code + '.description') }}</p>
                    </div>
                    <button
                      type="button"
                      class="btn btn-primary btn-sm"
                      :disabled="templatesApplying || template.code === templatesResult?.active_template_code"
                      @click="handleApplyTemplate(template.code)"
                    >
                      <span v-if="templatesApplying" class="mr-2 inline-block h-3 w-3 animate-spin rounded-full border-2 border-white border-t-transparent" />
                      {{ t('compliance.template.apply') }}
                    </button>
                  </div>
                </div>
              </div>

              <!-- 3.2 ZDR 设置卡片 -->
              <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
                <h3 class="text-base font-semibold text-gray-900 dark:text-white">
                  {{ t('compliance.zdr.title') }}
                </h3>
                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                  {{ t('compliance.zdr.description') }}
                </p>

                <div class="mt-3 space-y-3">
                  <div>
                    <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
                      {{ t('compliance.zdr.mode') }}
                    </label>
                    <div class="flex items-center gap-4">
                      <label class="flex items-center gap-2">
                        <input
                          v-model="profileForm.zdr_mode"
                          type="radio"
                          value="aggregate_only"
                          class="h-4 w-4 text-primary-600 focus:ring-primary-500"
                        />
                        <span class="text-sm text-gray-700 dark:text-gray-300">{{ t('compliance.zdr.aggregate_only') }}</span>
                      </label>
                      <label class="flex items-center gap-2">
                        <input
                          v-model="profileForm.zdr_mode"
                          type="radio"
                          value="audit"
                          class="h-4 w-4 text-primary-600 focus:ring-primary-500"
                        />
                        <span class="text-sm text-gray-700 dark:text-gray-300">{{ t('compliance.zdr.audit') }}</span>
                      </label>
                    </div>
                  </div>

                  <div v-if="isAuditMode">
                    <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
                      {{ t('compliance.zdr.retention_days') }}
                    </label>
                    <input
                      v-model.number="profileForm.detail_retention_days"
                      type="number"
                      min="1"
                      max="365"
                      class="input w-full max-w-xs"
                    />
                  </div>

                  <button
                    type="button"
                    class="btn btn-primary"
                    :disabled="complianceSaving"
                    @click="handleSaveZdr"
                  >
                    <span v-if="complianceSaving" class="mr-2 inline-block h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent" />
                    {{ t('compliance.template.apply') }}
                  </button>
                </div>
              </div>

              <!-- 3.3 合规框架卡片 -->
              <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
                <h3 class="text-base font-semibold text-gray-900 dark:text-white">
                  {{ t('compliance.frameworks.title') }}
                </h3>
                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                  {{ t('compliance.frameworks.description') }}
                </p>

                <div class="mt-3 space-y-2">
                  <label
                    v-for="fw in frameworkOptions"
                    :key="fw.key"
                    class="flex items-center gap-2 rounded-lg border border-gray-200 p-3 dark:border-dark-700 cursor-pointer"
                  >
                    <input
                      type="checkbox"
                      :checked="profileForm.compliance_frameworks.includes(fw.key)"
                      class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                      @change="toggleFramework(fw.key)"
                    />
                    <span class="text-sm text-gray-700 dark:text-gray-300">{{ t(fw.label) }}</span>
                  </label>
                </div>

                <button
                  type="button"
                  class="btn btn-primary mt-4"
                  :disabled="complianceSaving"
                  @click="handleSaveFrameworks"
                >
                  <span v-if="complianceSaving" class="mr-2 inline-block h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent" />
                  {{ t('compliance.template.apply') }}
                </button>
              </div>

              <!-- 3.4 内容审核策略卡片 -->
              <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
                <h3 class="text-base font-semibold text-gray-900 dark:text-white">
                  {{ t('compliance.moderation.title') }}
                </h3>
                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                  {{ t('compliance.moderation.description') }}
                </p>

                <div v-if="rulesLoading" class="mt-4 flex items-center justify-center py-4">
                  <span class="inline-block h-5 w-5 animate-spin rounded-full border-2 border-primary-600 border-t-transparent" />
                </div>

                <div v-else-if="moderationRules.length === 0" class="mt-4 rounded-lg bg-gray-50 py-4 text-center dark:bg-dark-900">
                  <p class="text-sm text-gray-500">{{ t('compliance.moderation.enabled') }}</p>
                </div>

                <div v-else class="mt-3 space-y-2">
                  <label
                    v-for="rule in moderationRules"
                    :key="rule.rule_id"
                    class="flex items-center justify-between rounded-lg border border-gray-200 p-3 dark:border-dark-700 cursor-pointer"
                  >
                    <div>
                      <p class="font-medium text-gray-900 dark:text-white">{{ rule.rule_name }}</p>
                      <p class="text-xs text-gray-500">{{ rule.rule_type }} | {{ rule.action }}</p>
                    </div>
                    <input
                      type="checkbox"
                      :checked="profileForm.enabled_rule_ids.length === 0 || profileForm.enabled_rule_ids.includes(rule.rule_id)"
                      class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                      @change="toggleModerationRule(rule.rule_id)"
                    />
                  </label>
                </div>

                <button
                  type="button"
                  class="btn btn-primary mt-4"
                  :disabled="complianceSaving"
                  @click="handleSaveModeration"
                >
                  <span v-if="complianceSaving" class="mr-2 inline-block h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent" />
                  {{ t('compliance.template.apply') }}
                </button>
              </div>

              <!-- 3.5 用户自定义规则 -->
              <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
                <h3 class="text-base font-semibold text-gray-900 dark:text-white">
                  {{ t('compliance.customRules.title') }}
                </h3>
                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                  {{ t('compliance.customRules.description') }}
                </p>

                <div v-if="userRulesLoading" class="mt-4 flex items-center justify-center py-4">
                  <span class="inline-block h-5 w-5 animate-spin rounded-full border-2 border-primary-600 border-t-transparent" />
                </div>

                <div v-else class="mt-3 space-y-2">
                  <div
                    v-for="userRule in userModerationRules"
                    :key="userRule.rule_id"
                    class="flex items-center justify-between rounded-lg border border-gray-200 p-3 dark:border-dark-700"
                  >
                    <div class="flex-1">
                      <div class="flex items-center gap-2">
                        <p class="font-medium text-gray-900 dark:text-white">{{ userRule.rule_name }}</p>
                        <span
                          class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium"
                          :class="userRule.enabled ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400' : 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400'"
                        >
                          {{ userRule.enabled ? t('compliance.customRules.enabled') : t('compliance.customRules.disabled') }}
                        </span>
                      </div>
                      <p class="text-xs text-gray-500">{{ userRule.rule_type }} | {{ userRule.action }} | {{ userRule.rule_pattern }}</p>
                    </div>
                    <div class="flex items-center gap-2">
                      <button
                        type="button"
                        class="rounded p-1 text-gray-400 hover:text-primary-600 hover:bg-gray-100 dark:hover:bg-dark-700"
                        :title="t('compliance.customRules.edit')"
                        @click="openEditUserRuleDialog(userRule)"
                      >
                        <Icon name="edit" size="sm" />
                      </button>
                      <button
                        type="button"
                        class="rounded p-1 text-gray-400 hover:text-red-600 hover:bg-gray-100 dark:hover:bg-dark-700"
                        :title="t('compliance.customRules.delete')"
                        @click="handleDeleteUserRule(userRule.rule_id)"
                      >
                        <Icon name="trash" size="sm" />
                      </button>
                    </div>
                  </div>
                  <div v-if="userModerationRules.length === 0" class="rounded-lg bg-gray-50 py-4 text-center dark:bg-dark-900">
                    <p class="text-sm text-gray-500">{{ t('compliance.customRules.empty') }}</p>
                  </div>
                </div>

                <button
                  type="button"
                  class="btn btn-outline mt-4"
                  @click="showUserRuleDialog = true"
                >
                  + {{ t('compliance.customRules.create') }}
                </button>

                <!-- 新建/编辑自定义规则对话框 -->
                <div v-if="showUserRuleDialog" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50" @click.self="closeUserRuleDialog">
                  <div class="w-full max-w-md rounded-xl bg-white p-6 shadow-xl dark:bg-dark-800">
                    <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
                      {{ editingUserRule ? t('compliance.customRules.editTitle') : t('compliance.customRules.createTitle') }}
                    </h3>
                    <div class="mt-4 space-y-3">
                      <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('compliance.customRules.ruleName') }}</label>
                        <input
                          v-model="userRuleForm.rule_name"
                          type="text"
                          class="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-700 dark:text-white"
                          :placeholder="t('compliance.customRules.ruleNamePlaceholder')"
                        />
                      </div>
                      <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('compliance.customRules.ruleType') }}</label>
                        <select
                          v-model="userRuleForm.rule_type"
                          class="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-700 dark:text-white"
                        >
                          <option value="KEYWORD">KEYWORD</option>
                          <option value="REGEX">REGEX</option>
                          <option value="PATTERN">PATTERN</option>
                        </select>
                      </div>
                      <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('compliance.customRules.rulePattern') }}</label>
                        <input
                          v-model="userRuleForm.rule_pattern"
                          type="text"
                          class="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-700 dark:text-white"
                          :placeholder="t('compliance.customRules.patternPlaceholder')"
                        />
                      </div>
                      <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('compliance.customRules.action') }}</label>
                        <select
                          v-model="userRuleForm.action"
                          class="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-700 dark:text-white"
                        >
                          <option value="BLOCK">BLOCK</option>
                          <option value="REVIEW">REVIEW</option>
                          <option value="ALLOW">ALLOW</option>
                        </select>
                      </div>
                      <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('compliance.customRules.riskCategory') }}</label>
                        <input
                          v-model="userRuleForm.risk_category"
                          type="text"
                          class="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-700 dark:text-white"
                          :placeholder="t('compliance.customRules.riskCategoryPlaceholder')"
                        />
                      </div>
                      <label class="flex items-center gap-2">
                        <input
                          v-model="userRuleForm.enabled"
                          type="checkbox"
                          class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                        />
                        <span class="text-sm text-gray-700 dark:text-gray-300">{{ t('compliance.customRules.enableRule') }}</span>
                      </label>
                    </div>
                    <div class="mt-6 flex justify-end gap-3">
                      <button type="button" class="btn btn-outline" @click="closeUserRuleDialog">
                        {{ t('common.cancel') }}
                      </button>
                      <button
                        type="button"
                        class="btn btn-primary"
                        :disabled="userRuleSaving"
                        @click="editingUserRule ? handleUpdateUserRule() : handleCreateUserRule()"
                      >
                        <span v-if="userRuleSaving" class="mr-2 inline-block h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent" />
                        {{ editingUserRule ? t('compliance.customRules.update') : t('compliance.customRules.create') }}
                      </button>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
      </div>

      <!-- 数据权利 Tab -->
      <div v-show="activeTab === 'dataRights'">
      <!-- 数据导出 -->
      <div class="card p-6">
        <div class="flex items-start justify-between gap-4">
          <div class="flex items-start gap-3">
            <div class="rounded-xl bg-blue-100 p-3 text-blue-600 dark:bg-blue-900/30">
              <Icon name="download" size="lg" />
            </div>
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('dataRights.export.title') }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t('dataRights.export.description') }}
              </p>
            </div>
          </div>
          <button
            type="button"
            class="btn btn-primary"
            :disabled="exporting"
            @click="handleExport"
          >
            <span v-if="exporting" class="mr-2 inline-block h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent" />
            {{ exporting ? t('dataRights.export.processing') : t('dataRights.export.button') }}
          </button>
        </div>
      </div>

      <!-- 数据删除 -->
      <div class="card p-6">
        <div class="flex items-start gap-3">
          <div class="rounded-xl bg-red-100 p-3 text-red-600 dark:bg-red-900/30">
            <Icon name="trash" size="lg" />
          </div>
          <div class="flex-1">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('dataRights.erasure.title') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('dataRights.erasure.description') }}
            </p>

            <div class="mt-4 space-y-3">
              <div>
                <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('dataRights.erasure.reasonLabel') }}
                </label>
                <textarea
                  v-model="erasureReason"
                  rows="3"
                  class="input w-full"
                  :placeholder="t('dataRights.erasure.reasonPlaceholder')"
                />
              </div>
              <div>
                <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('dataRights.erasure.confirmLabel') }}
                </label>
                <input
                  v-model="erasureConfirm"
                  type="text"
                  class="input w-full"
                  :placeholder="t('dataRights.erasure.confirmPlaceholder')"
                />
                <p class="mt-1 text-xs text-gray-500">
                  {{ t('dataRights.erasure.confirmHint') }}
                </p>
              </div>
              <button
                type="button"
                class="btn btn-danger"
                :disabled="erasureSubmitting || !canSubmitErasure"
                @click="handleErasureRequest"
              >
                <span v-if="erasureSubmitting" class="mr-2 inline-block h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent" />
                {{ erasureSubmitting ? t('dataRights.erasure.submitting') : t('dataRights.erasure.submit') }}
              </button>
            </div>

            <div v-if="erasureResult" class="mt-4 rounded-lg bg-green-50 p-4 dark:bg-green-900/20">
              <p class="text-sm text-green-700 dark:text-green-300">
                {{ t('dataRights.erasure.success', { id: erasureResult.id }) }}
              </p>
            </div>
          </div>
        </div>
      </div>

      <!-- 数据删除请求历史 -->
      <div class="card p-6">
        <div class="flex items-start gap-3">
          <div class="rounded-xl bg-orange-100 p-3 text-orange-600 dark:bg-orange-900/30">
              <Icon name="clock" size="lg" />
            </div>
          <div class="flex-1">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('dataRights.erasure.history') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('dataRights.erasure.historyDescription') }}
            </p>

            <div v-if="erasureRequestsLoading" class="mt-4 flex items-center justify-center py-4">
              <span class="inline-block h-5 w-5 animate-spin rounded-full border-2 border-primary-600 border-t-transparent" />
            </div>

            <div v-else-if="erasureRequests.length === 0" class="mt-4 rounded-lg bg-gray-50 py-4 text-center dark:bg-dark-900">
              <p class="text-sm text-gray-500">{{ t('dataRights.erasure.noRequests') }}</p>
            </div>

            <div v-else class="mt-4 space-y-2">
              <div
                v-for="req in erasureRequests"
                :key="req.id"
                class="flex items-center justify-between rounded-lg border border-gray-200 p-3 dark:border-dark-700"
              >
                <div>
                  <p class="font-medium text-gray-900 dark:text-white">{{ t('dataRights.erasure.request') }} #{{ req.id }}</p>
                  <p class="text-xs text-gray-500">
                    {{ t('dataRights.erasure.requestType') }}: {{ req.request_type }} |
                    {{ t('dataRights.consent.status') }}:
                    <span :class="getStatusClass(req.status)">
                      {{ getStatusText(req.status) }}
                    </span>
                  </p>
                  <p v-if="req.reason" class="text-xs text-gray-400 mt-1">{{ t('dataRights.erasure.reason') }}: {{ req.reason }}</p>
                  <p class="text-xs text-gray-400">{{ t('dataRights.consent.createdAt') }}: {{ formatDate(req.requested_at) }}</p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
      </div>

      <!-- 同意管理 Tab -->
      <div v-show="activeTab === 'consent'">
      <!-- 同意记录 -->
      <div class="card p-6">
        <div class="flex items-start gap-3">
          <div class="rounded-xl bg-amber-100 p-3 text-amber-600 dark:bg-amber-900/30">
            <Icon name="checkCircle" size="lg" />
          </div>
          <div class="flex-1">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('dataRights.consent.title') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('dataRights.consent.description') }}
            </p>

            <div v-if="consentLoading" class="mt-4 flex items-center justify-center py-8">
              <span class="inline-block h-6 w-6 animate-spin rounded-full border-2 border-primary-600 border-t-transparent" />
            </div>
            <div v-else class="mt-4 space-y-3">
              <div
                v-for="consent in consents"
                :key="consent.consent_type"
                class="flex items-center justify-between rounded-lg border border-gray-200 p-4 dark:border-dark-700"
              >
                <div>
                  <p class="font-medium text-gray-900 dark:text-white">{{ t('dataRights.consent.types.' + consent.consent_type + '.label') }}</p>
                  <p class="text-sm text-gray-500 mt-1">{{ t('dataRights.consent.types.' + consent.consent_type + '.description') }}</p>
                  <p class="text-xs text-gray-500 mt-2">
                    {{ t('dataRights.consent.version') }}: {{ consent.version }} |
                    {{ t('dataRights.consent.status') }}:
                    <span :class="consent.status === 'granted' ? 'text-green-600' : 'text-red-600'">
                      {{ consent.status === 'granted' ? t('dataRights.consent.granted') : t('dataRights.consent.revoked') }}
                    </span>
                  </p>
                  <p v-if="consent.granted_at" class="text-xs text-gray-400 mt-1">
                    {{ t('dataRights.consent.grantedAt') }}: {{ formatDate(consent.granted_at) }}
                  </p>
                </div>
                <label class="relative inline-flex cursor-pointer items-center">
                  <input
                    type="checkbox"
                    :checked="consent.status === 'granted'"
                    class="peer sr-only"
                    @change="toggleConsent(consent)"
                  >
                  <div class="peer h-6 w-11 rounded-full bg-gray-200 after:absolute after:left-[2px] after:top-[2px] after:h-5 after:w-5 after:rounded-full after:border after:border-gray-300 after:bg-white after:transition-all after:content-[''] peer-checked:bg-primary-600 peer-checked:after:translate-x-full peer-checked:after:border-white peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-primary-300 dark:bg-gray-700 dark:peer-focus:ring-primary-800" />
                </label>
              </div>
            </div>
          </div>
        </div>
      </div>
      </div>

      <!-- 跨法域映射 Tab -->
      <div v-show="activeTab === 'jurisdiction'">
        <div class="card p-6">
          <div class="flex items-start gap-3">
            <div class="rounded-xl bg-indigo-100 p-3 text-indigo-600 dark:bg-indigo-900/30">
              <Icon name="globe" size="lg" />
            </div>
            <div class="flex-1">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('compliance.jurisdiction.title') }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t('compliance.jurisdiction.description') }}
              </p>

              <div v-if="jurisdictionLoading" class="mt-4 flex items-center justify-center py-4">
                <span class="inline-block h-5 w-5 animate-spin rounded-full border-2 border-primary-600 border-t-transparent" />
              </div>

              <div v-else class="mt-4 space-y-4">
                <div class="grid gap-4 sm:grid-cols-3">
                  <div>
                    <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
                      {{ t('compliance.jurisdiction.region') }}
                    </label>
                    <select
                      v-model="jurisdictionForm.region"
                      class="input w-full text-sm"
                      @change="onJurisdictionRegionChange"
                    >
                      <option value="">--</option>
                      <option v-for="region in supportedJurisdictions" :key="region" :value="region">
                        {{ region }}
                      </option>
                    </select>
                  </div>
                  <div>
                    <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
                      {{ t('compliance.jurisdiction.industry') }}
                    </label>
                    <input
                      v-model="jurisdictionForm.industry"
                      type="text"
                      class="input w-full text-sm"
                      :placeholder="t('compliance.jurisdiction.industryPlaceholder')"
                    />
                  </div>
                  <div>
                    <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
                      {{ t('compliance.jurisdiction.serviceType') }}
                    </label>
                    <select v-model="jurisdictionForm.serviceType" class="input w-full text-sm">
                      <option value="">--</option>
                      <option value="ai_chatbot">AI Chatbot</option>
                      <option value="ai_analysis">AI Analysis</option>
                      <option value="ai_recommendation">AI Recommendation</option>
                    </select>
                  </div>
                </div>
                <button
                  type="button"
                  class="btn btn-primary"
                  :disabled="!jurisdictionForm.region || jurisdictionMappingLoading"
                  @click="handleJurisdictionMapping"
                >
                  <span v-if="jurisdictionMappingLoading" class="mr-2 inline-block h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent" />
                  {{ t('compliance.jurisdiction.map') }}
                </button>

                <div class="rounded-lg bg-gray-50 p-4 text-xs text-gray-600 dark:bg-dark-900 dark:text-gray-400">
                  <h4 class="mb-2 font-semibold">{{ t('compliance.jurisdiction.fieldHelp') }}</h4>
                  <ul class="space-y-1.5">
                    <li><span class="font-medium">Region:</span> {{ t('compliance.jurisdiction.fieldHelpRegion') }}</li>
                    <li><span class="font-medium">Industry:</span> {{ t('compliance.jurisdiction.fieldHelpIndustry') }}</li>
                    <li><span class="font-medium">Service Type:</span> {{ t('compliance.jurisdiction.fieldHelpServiceType') }}</li>
                  </ul>
                </div>

                <div v-if="jurisdictionResult" class="mt-4 space-y-4 rounded-lg border border-gray-200 p-4 dark:border-dark-700">
                  <div class="flex items-center gap-2">
                    <span class="text-sm font-medium text-gray-700 dark:text-gray-200">
                      {{ t('compliance.jurisdiction.riskLevel') }}:
                    </span>
                    <span
                      class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium"
                      :class="jurisdictionRiskBadgeClass(jurisdictionResult.risk_level)"
                    >
                      {{ jurisdictionResult.risk_level }}
                    </span>
                  </div>
                  <div>
                    <h4 class="mb-1 text-sm font-semibold text-gray-700 dark:text-gray-200">
                      {{ t('compliance.jurisdiction.regulations') }}
                    </h4>
                    <ul class="list-inside list-disc space-y-0.5 text-sm text-gray-600 dark:text-gray-300">
                      <li v-for="item in jurisdictionResult.applicable_regulations" :key="item">{{ item }}</li>
                    </ul>
                  </div>
                  <div>
                    <h4 class="mb-1 text-sm font-semibold text-gray-700 dark:text-gray-200">
                      {{ t('compliance.jurisdiction.measures') }}
                    </h4>
                    <ul class="list-inside list-disc space-y-0.5 text-sm text-gray-600 dark:text-gray-300">
                      <li v-for="item in jurisdictionResult.required_measures" :key="item">{{ item }}</li>
                    </ul>
                  </div>
                  <div>
                    <h4 class="mb-1 text-sm font-semibold text-gray-700 dark:text-gray-200">
                      {{ t('compliance.jurisdiction.actions') }}
                    </h4>
                    <ul class="list-inside list-disc space-y-0.5 text-sm text-gray-600 dark:text-gray-300">
                      <li v-for="item in jurisdictionResult.recommended_actions" :key="item">{{ item }}</li>
                    </ul>
                  </div>
                  <div class="mt-4 flex flex-wrap items-center gap-3 border-t border-gray-200 pt-4 dark:border-dark-700">
                    <label class="flex items-center gap-2 cursor-pointer">
                      <input v-model="jurisdictionSaveApplyRules" type="checkbox" class="rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
                      <span class="text-sm text-gray-600 dark:text-gray-300">{{ t('compliance.jurisdiction.applyRules') }}</span>
                    </label>
                    <button
                      type="button"
                      class="btn btn-success"
                      :disabled="jurisdictionSaving"
                      @click="handleSaveJurisdictionMapping"
                    >
                      <span v-if="jurisdictionSaving" class="mr-2 inline-block h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent" />
                      {{ t('compliance.jurisdiction.save') }}
                    </button>
                  </div>
                  <div v-if="savedJurisdictionMapping" class="mt-2 rounded-lg bg-green-50 p-3 text-sm text-green-700 dark:bg-green-900/20 dark:text-green-400">
                    {{ t('compliance.jurisdiction.saved') }}
                    <span v-if="savedJurisdictionMapping.applied_rules && savedJurisdictionMapping.applied_rules.length">
                      {{ t('compliance.jurisdiction.appliedRules') }}: {{ savedJurisdictionMapping.applied_rules.join(', ') }}
                    </span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- DPA 生成 Tab -->
      <div v-show="activeTab === 'dpa'">
        <div class="card p-6">
          <div class="flex items-start gap-3">
            <div class="rounded-xl bg-blue-100 p-3 text-blue-600 dark:bg-blue-900/30">
              <Icon name="document" size="lg" />
            </div>
            <div class="flex-1">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('compliance.dpa.title') }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t('compliance.dpa.description') }}
              </p>

              <div class="mt-4 space-y-4">
                <div class="grid gap-4 sm:grid-cols-2">
                  <div>
                    <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
                      {{ t('compliance.dpa.controllerName') }}
                      <span class="text-red-500">*</span>
                    </label>
                    <input
                      v-model="dpaForm.controllerName"
                      type="text"
                      class="input w-full text-sm"
                      :placeholder="t('compliance.dpa.controllerNamePlaceholder')"
                    />
                  </div>
                  <div>
                    <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
                      {{ t('compliance.dpa.controllerContact') }}
                      <span class="text-red-500">*</span>
                    </label>
                    <input
                      v-model="dpaForm.controllerContact"
                      type="text"
                      class="input w-full text-sm"
                      :placeholder="t('compliance.dpa.controllerContactPlaceholder')"
                    />
                  </div>
                </div>
                <button
                  type="button"
                  class="btn btn-primary"
                  :disabled="!dpaForm.controllerName || !dpaForm.controllerContact || dpaGenerating"
                  @click="handleGenerateDPA"
                >
                  <span v-if="dpaGenerating" class="mr-2 inline-block h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent" />
                  {{ t('compliance.dpa.generate') }}
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- 合规凭证 Tab -->
      <div v-show="activeTab === 'credentials'">
        <div class="card p-6">
          <div class="flex items-start gap-3">
            <div class="rounded-xl bg-green-100 p-3 text-green-600 dark:bg-green-900/30">
              <Icon name="badge" size="lg" />
            </div>
            <div class="flex-1">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('compliance.credentials.title') }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t('compliance.credentials.description') }}
              </p>

              <div v-if="credentialsLoading" class="mt-4 flex items-center justify-center py-8">
                <span class="inline-block h-6 w-6 animate-spin rounded-full border-2 border-primary-600 border-t-transparent" />
              </div>

              <div v-else-if="credentials.length === 0" class="mt-4 rounded-lg bg-gray-50 py-8 text-center dark:bg-dark-900">
                <p class="text-sm text-gray-500">{{ t('compliance.credentials.empty') }}</p>
              </div>

              <div v-else class="mt-4 space-y-3">
                <div
                  v-for="credential in credentials"
                  :key="credential.id"
                  class="rounded-lg border border-gray-200 p-4 dark:border-dark-700"
                >
                  <div class="flex items-start justify-between gap-4">
                    <div class="flex-1">
                      <div class="flex items-center gap-2">
                        <p class="font-medium text-gray-900 dark:text-white">{{ credential.credential_id }}</p>
                        <span
                          class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium"
                          :class="credential.status === 'active'
                            ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300'
                            : 'bg-gray-100 text-gray-700 dark:bg-gray-900/30 dark:text-gray-300'"
                        >
                          {{ credential.status }}
                        </span>
                      </div>
                      <p class="text-sm font-medium text-gray-700 dark:text-gray-300 mt-1">
                        {{ t('compliance.credentials.credentialTypes.' + credential.credential_type) || credential.credential_type }}
                      </p>
                      <p class="text-xs text-gray-500 mt-1">{{ credential.issuer }} | {{ t('compliance.credentials.issuerTypes.' + credential.issuer_type) || credential.issuer_type }}</p>
                      <p v-if="credential.scope" class="text-xs text-gray-500 mt-1">
                        <span class="font-medium">{{ t('compliance.credentials.scope') }}:</span> {{ credential.scope }}
                      </p>
                      <div v-if="credential.metadata && Object.keys(credential.metadata).length > 0" class="mt-3 rounded-lg bg-gray-50 p-3 dark:bg-dark-900">
                        <p class="text-xs font-medium text-gray-500 mb-2">{{ t('compliance.credentials.metadata.title') || '详细信息' }}</p>
                        <div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
                          <div
                            v-for="(value, key) in credential.metadata"
                            :key="key"
                            class="flex items-start justify-between"
                          >
                            <span class="text-xs text-gray-500">{{ t('compliance.credentials.metadata.' + key) || key }}</span>
                            <span class="text-xs text-gray-700 dark:text-gray-300 break-all">
                              {{ Array.isArray(value) ? value.join(', ') : (typeof value === 'boolean' ? (value ? t('common.yes') : t('common.no')) : value) }}
                            </span>
                          </div>
                        </div>
                      </div>
                      <div class="mt-3 flex flex-wrap gap-4 text-xs text-gray-400">
                        <span>{{ t('compliance.credentials.validFrom') }}: {{ formatDate(credential.valid_from) }}</span>
                        <span>{{ t('compliance.credentials.validUntil') }}: {{ formatDate(credential.valid_until) }}</span>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- 审计日志 Tab -->
      <div v-show="activeTab === 'audit'">
        <div class="card p-6">
          <div class="flex items-start justify-between gap-4">
            <div class="flex items-start gap-3">
              <div class="rounded-xl bg-blue-100 p-3 text-blue-600 dark:bg-blue-900/30">
                <Icon name="clipboard" size="lg" />
              </div>
              <div>
                <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                  {{ t('compliance.audit.title') }}
                </h2>
                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                  {{ t('compliance.audit.description') }}
                </p>
              </div>
            </div>
          </div>

          <div v-if="auditLogsLoading" class="mt-4 flex items-center justify-center py-8">
            <span class="inline-block h-6 w-6 animate-spin rounded-full border-2 border-primary-600 border-t-transparent" />
          </div>

          <div v-else-if="auditLogs.items.length === 0" class="mt-4 rounded-lg bg-gray-50 py-8 text-center dark:bg-dark-900">
            <p class="text-sm text-gray-500">{{ t('compliance.audit.empty') }}</p>
          </div>

          <div v-else class="mt-4">
            <div class="space-y-3">
              <div
                v-for="log in auditLogs.items"
                :key="log.id"
                class="rounded-lg border border-gray-200 p-4 dark:border-dark-700"
              >
                <div class="flex items-start justify-between gap-4">
                  <div>
                    <div class="flex items-center gap-2">
                      <span
                        class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300"
                      >
                        {{ log.compliance_type }}
                      </span>
                      <span
                        class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium bg-gray-100 text-gray-700 dark:bg-gray-900/30 dark:text-gray-300"
                      >
                        {{ log.subject_type }}
                      </span>
                    </div>
                    <p class="text-sm text-gray-900 dark:text-white mt-2">{{ log.details }}</p>
                    <p class="text-xs text-gray-400 mt-1">
                      {{ t('compliance.audit.operator') }}: {{ log.operator }} | {{ formatDate(log.created_at) }}
                    </p>
                  </div>
                </div>
              </div>
            </div>

            <div v-if="auditLogs.total > auditLogs.page_size" class="mt-4 flex items-center justify-between">
              <button
                type="button"
                class="btn btn-outline"
                :disabled="auditLogs.page <= 1"
                @click="loadAuditLogs(auditLogs.page - 1)"
              >
                {{ t('compliance.audit.prev') }}
              </button>
              <span class="text-sm text-gray-500">
                {{ t('compliance.audit.page') }} {{ auditLogs.page }} / {{ Math.ceil(auditLogs.total / auditLogs.page_size) }}
              </span>
              <button
                type="button"
                class="btn btn-outline"
                :disabled="auditLogs.page >= Math.ceil(auditLogs.total / auditLogs.page_size)"
                @click="loadAuditLogs(auditLogs.page + 1)"
              >
                {{ t('compliance.audit.next') }}
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- 风险分析 Tab -->
      <div v-show="activeTab === 'risk'">
        <div class="card p-6">
          <div class="flex items-start gap-3">
            <div class="rounded-xl bg-orange-100 p-3 text-orange-600 dark:bg-orange-900/30">
              <Icon name="exclamationTriangle" size="lg" />
            </div>
            <div class="flex-1">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('compliance.risk.title') }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t('compliance.risk.description') }}
              </p>

              <div v-if="riskTagsLoading" class="mt-4 flex items-center justify-center py-8">
                <span class="inline-block h-6 w-6 animate-spin rounded-full border-2 border-primary-600 border-t-transparent" />
              </div>

              <div v-else class="mt-4 space-y-6">
                <div>
                  <h3 class="text-sm font-semibold text-gray-700 dark:text-gray-200 mb-3">
                    {{ t('compliance.risk.modelTags') }}
                  </h3>
                  <div class="grid gap-3 sm:grid-cols-2">
                    <div
                      v-for="tag in riskTags.model_tags"
                      :key="tag.tag"
                      class="rounded-lg border border-gray-200 p-3 dark:border-dark-700"
                    >
                      <div class="flex items-center gap-2">
                        <span
                          class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-300"
                        >
                          {{ getRiskTagLabel(tag.tag) }}
                        </span>
                      </div>
                      <p class="text-xs text-gray-500 mt-1">{{ getRiskTagDescription(tag.tag) }}</p>
                    </div>
                  </div>
                </div>

                <div>
                  <h3 class="text-sm font-semibold text-gray-700 dark:text-gray-200 mb-3">
                    {{ t('compliance.risk.riskTags') }}
                  </h3>
                  <div class="grid gap-3 sm:grid-cols-2">
                    <div
                      v-for="tag in riskTags.risk_tags"
                      :key="tag.tag"
                      class="rounded-lg border border-gray-200 p-3 dark:border-dark-700"
                    >
                      <div class="flex items-center gap-2">
                        <span
                          class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300"
                        >
                          {{ getRiskTagLabel(tag.tag) }}
                        </span>
                      </div>
                      <p class="text-xs text-gray-500 mt-1">{{ getRiskTagDescription(tag.tag) }}</p>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- EU AI Act 评估 Tab -->
      <div v-show="activeTab === 'euaiact'">
        <div class="card p-6">
          <div class="flex items-start justify-between gap-4">
            <div class="flex items-start gap-3">
              <div class="rounded-xl bg-indigo-100 p-3 text-indigo-600 dark:bg-indigo-900/30">
                <Icon name="shield" size="lg" />
              </div>
              <div>
                <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                  {{ t('compliance.euAiAct.title') }}
                </h2>
                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                  {{ t('compliance.euAiAct.description') }}
                </p>
              </div>
            </div>
            <button
              type="button"
              class="btn btn-primary"
              :disabled="euaiExporting"
              @click="handleExportEUAIAct"
            >
              <span v-if="euaiExporting" class="mr-2 inline-block h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent" />
              {{ t('compliance.euAiAct.export') }}
            </button>
          </div>

          <div v-if="euaiAssessmentLoading" class="mt-4 flex items-center justify-center py-8">
            <span class="inline-block h-6 w-6 animate-spin rounded-full border-2 border-primary-600 border-t-transparent" />
          </div>

          <div v-else class="mt-4">
            <pre class="rounded-lg border border-gray-200 bg-gray-50 p-4 text-sm text-gray-700 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-300 overflow-x-auto">{{ JSON.stringify(euaiAssessment, null, 2) }}</pre>
          </div>
        </div>
      </div>

      <!-- GDPR 处理记录 Tab -->
      <div v-show="activeTab === 'ropa'">
        <div class="card p-6">
          <div class="flex items-start gap-3">
            <div class="rounded-xl bg-green-100 p-3 text-green-600 dark:bg-green-900/30">
              <Icon name="checkCircle" size="lg" />
            </div>
            <div class="flex-1">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('compliance.ropa.title') }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t('compliance.ropa.description') }}
              </p>

              <div v-if="ropaLoading" class="mt-4 flex items-center justify-center py-8">
                <span class="inline-block h-6 w-6 animate-spin rounded-full border-2 border-primary-600 border-t-transparent" />
              </div>

              <div v-else class="mt-4">
                <pre class="rounded-lg border border-gray-200 bg-gray-50 p-4 text-sm text-gray-700 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-300 overflow-x-auto">{{ JSON.stringify(ropaRecord, null, 2) }}</pre>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, reactive } from 'vue'
import { useI18n } from 'vue-i18n'
import { Icon } from '@/components/icons'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import {
  requestDataErasure,
  listDataErasureRequests,
  exportUserData,
  getUserConsents,
  setConsent,
  getComplianceProfile,
  updateComplianceProfile,
  listComplianceTemplates,
  applyComplianceTemplate,
  listModerationRules,
  listUserModerationRules,
  createUserModerationRule,
  updateUserModerationRule,
  deleteUserModerationRule,
  getSupportedJurisdictions,
  getJurisdictionMapping,
  getUserJurisdictionMapping,
  saveJurisdictionMapping,
  generateDPA,
  listCredentials,
  listAuditLogs,
  getRiskTags,
  getEUAIActAssessment,
  exportEUAIActAssessment,
  getDataProcessingRecord,
  type DataErasureRequest,
  type ConsentRecord,
  type TemplateItem,
  type TemplateListResult,
  type ModerationRuleItem,
  type UserModerationRule,
  type JurisdictionMappingResult,
  type ComplianceCredential,
  type ComplianceAuditLog,
  type PaginatedResult,
  type RiskTagsCatalog,
} from '@/api/governance'

const { t } = useI18n()
const appStore = useAppStore()

const getRiskTagLabel = (tagName: string) => {
  return t(`compliance.risk.tags.${tagName}.label`)
}

const getRiskTagDescription = (tagName: string) => {
  return t(`compliance.risk.tags.${tagName}.description`)
}

// Tab 切换
const activeTab = ref('compliance')
const tabs = computed(() => [
  { key: 'compliance', label: t('compliance.title') },
  { key: 'jurisdiction', label: t('compliance.jurisdiction.title') },
  { key: 'dpa', label: t('compliance.dpa.title') },
  { key: 'credentials', label: t('compliance.credentials.title') },
  { key: 'audit', label: t('compliance.audit.title') },
  { key: 'risk', label: t('compliance.risk.title') },
  { key: 'euaiact', label: t('compliance.euAiAct.title') },
  { key: 'ropa', label: t('compliance.ropa.title') },
  { key: 'dataRights', label: t('dataRights.title') },
  { key: 'consent', label: t('dataRights.consent.title') },
])

// 数据导出
const exporting = ref(false)

async function handleExport() {
  exporting.value = true
  try {
    await exportUserData()
    appStore.showSuccess(t('dataRights.export.successMessage'))
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('dataRights.export.error')))
  } finally {
    exporting.value = false
  }
}

// 数据删除
const erasureReason = ref('')
const erasureConfirm = ref('')
const erasureSubmitting = ref(false)
const erasureResult = ref<DataErasureRequest | null>(null)

const canSubmitErasure = computed(() => {
  return erasureReason.value.trim().length > 0 && erasureConfirm.value.trim().length > 0
})

async function handleErasureRequest() {
  erasureSubmitting.value = true
  erasureResult.value = null
  try {
    erasureResult.value = await requestDataErasure({
      reason: erasureReason.value,
      confirmation_text: erasureConfirm.value,
      request_type: 'FULL_ERASURE',
    })
    appStore.showSuccess(t('dataRights.erasure.successMessage'))
    erasureReason.value = ''
    erasureConfirm.value = ''
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('dataRights.erasure.error')))
  } finally {
    erasureSubmitting.value = false
  }
}

// 数据删除请求历史
const erasureRequests = ref<DataErasureRequest[]>([])
const erasureRequestsLoading = ref(false)

async function loadErasureRequests() {
  erasureRequestsLoading.value = true
  try {
    erasureRequests.value = await listDataErasureRequests()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('dataRights.erasure.error')))
  } finally {
    erasureRequestsLoading.value = false
  }
}

function getStatusText(status: string): string {
  const map: Record<string, string> = {
    pending: t('dataRights.erasure.status.pending'),
    approved: t('dataRights.erasure.status.approved'),
    rejected: t('dataRights.erasure.status.rejected'),
    completed: t('dataRights.erasure.status.completed'),
  }
  return map[status] || status
}

function getStatusClass(status: string): string {
  const map: Record<string, string> = {
    pending: 'text-yellow-600',
    approved: 'text-blue-600',
    rejected: 'text-red-600',
    completed: 'text-green-600',
  }
  return map[status] || 'text-gray-600'
}

// 同意记录
const consents = ref<ConsentRecord[]>([])
const consentLoading = ref(false)
const grantedConsents = computed(() => consents.value.filter(c => c.status === 'granted'))

const consentTypeKeys = ['terms_of_service', 'gdpr_data_processing', 'detailed_logging', 'cross_border_transfer', 'marketing', 'model_training']

async function loadConsents() {
  consentLoading.value = true
  try {
    const existingConsents = await getUserConsents()
    
    const consentMap = new Map<string, ConsentRecord>()
    existingConsents.forEach(c => consentMap.set(c.consent_type, c))
    
    consents.value = consentTypeKeys.map(type => {
      const existing = consentMap.get(type)
      if (existing) {
        return existing
      }
      return {
        id: '',
        consent_type: type,
        version: '1.0',
        status: 'granted',
        granted: true,
        granted_at: '',
        revoked_at: '',
        ip_address: '',
        user_agent: '',
        created_at: '',
      }
    })
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('dataRights.consent.loadError')))
  } finally {
    consentLoading.value = false
  }
}

async function toggleConsent(consent: ConsentRecord) {
  try {
    const newStatus = consent.status === 'granted' ? false : true
    await setConsent(consent.consent_type, newStatus)
    await loadConsents()
    appStore.showSuccess(t('dataRights.consent.updateSuccess'))
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('dataRights.consent.updateError')))
  }
}

function formatDate(dateStr: string): string {
  if (!dateStr) return ''
  const date = new Date(dateStr)
  if (isNaN(date.getTime())) {
    return dateStr
  }
  return date.toLocaleString()
}

// ==================== Account 合规配置 ====================

const complianceLoading = ref(false)
const complianceSaving = ref(false)
const templatesLoading = ref(false)
const templatesApplying = ref(false)
const rulesLoading = ref(false)
const templatesResult = ref<TemplateListResult | null>(null)
const moderationRules = ref<ModerationRuleItem[]>([])

const profileForm = reactive<{
  active_template_code: string
  zdr_mode: 'aggregate_only' | 'audit'
  detail_retention_days: number
  compliance_frameworks: string[]
  enabled_rule_ids: string[]
}>({
  active_template_code: '',
  zdr_mode: 'aggregate_only',
  detail_retention_days: 30,
  compliance_frameworks: [],
  enabled_rule_ids: [],
})

const availableTemplates = computed<TemplateItem[]>(() => {
  return templatesResult.value?.items ?? []
})

const currentTemplate = computed<TemplateItem | null>(() => {
  if (!templatesResult.value?.active_template_code) return null
  return (
    templatesResult.value.items.find(
      (item) => item.code === templatesResult.value!.active_template_code
    ) ?? null
  )
})

const isAuditMode = computed(() => profileForm.zdr_mode === 'audit')

const frameworkOptions = [
  { key: 'gdpr', label: 'compliance.frameworks.gdpr' },
  { key: 'eu_ai_act', label: 'compliance.frameworks.eu_ai_act' },
  { key: 'hipaa', label: 'compliance.frameworks.hipaa' },
]

async function loadComplianceProfile() {
  complianceLoading.value = true
  try {
    const profile = await getComplianceProfile()
    profileForm.active_template_code = profile.active_template_code ?? ''
    profileForm.zdr_mode = profile.zdr_mode
    profileForm.detail_retention_days = profile.detail_retention_days
    profileForm.compliance_frameworks = [...profile.compliance_frameworks]
    profileForm.enabled_rule_ids = [...(profile.moderation_policy?.enabled_rule_ids ?? [])]
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('compliance.status.title')))
  } finally {
    complianceLoading.value = false
  }
}

async function loadTemplates() {
  templatesLoading.value = true
  try {
    templatesResult.value = await listComplianceTemplates()
    if (currentTemplate.value && !jurisdictionForm.industry) {
      jurisdictionForm.industry = t(`compliance.template.industries.${currentTemplate.value.code}.label`)
    }
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('compliance.template.title')))
  } finally {
    templatesLoading.value = false
  }
}

async function loadModerationRules() {
  rulesLoading.value = true
  try {
    moderationRules.value = await listModerationRules()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('compliance.moderation.title')))
  } finally {
    rulesLoading.value = false
  }
}

async function handleApplyTemplate(templateCode: string) {
  templatesApplying.value = true
  try {
    await applyComplianceTemplate(templateCode)
    appStore.showSuccess(t('compliance.template.apply'))
    await loadTemplates()
    await loadComplianceProfile()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('compliance.template.title')))
  } finally {
    templatesApplying.value = false
  }
}

async function handleSaveZdr() {
  complianceSaving.value = true
  try {
    await updateComplianceProfile({
      zdr_mode: profileForm.zdr_mode,
      detail_retention_days: profileForm.detail_retention_days,
    })
    appStore.showSuccess(t('compliance.status.title'))
    await loadComplianceProfile()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('compliance.zdr.title')))
  } finally {
    complianceSaving.value = false
  }
}

async function handleSaveFrameworks() {
  complianceSaving.value = true
  try {
    await updateComplianceProfile({
      compliance_frameworks: profileForm.compliance_frameworks,
    })
    appStore.showSuccess(t('compliance.status.title'))
    await loadComplianceProfile()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('compliance.frameworks.title')))
  } finally {
    complianceSaving.value = false
  }
}

async function handleSaveModeration() {
  complianceSaving.value = true
  try {
    await updateComplianceProfile({
      moderation_policy: { enabled_rule_ids: profileForm.enabled_rule_ids },
    })
    appStore.showSuccess(t('compliance.status.title'))
    await loadComplianceProfile()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('compliance.moderation.title')))
  } finally {
    complianceSaving.value = false
  }
}

function toggleFramework(key: string) {
  const idx = profileForm.compliance_frameworks.indexOf(key)
  if (idx > -1) {
    profileForm.compliance_frameworks.splice(idx, 1)
  } else {
    profileForm.compliance_frameworks.push(key)
  }
}

function toggleModerationRule(ruleId: string) {
  const idx = profileForm.enabled_rule_ids.indexOf(ruleId)
  if (idx > -1) {
    // Rule is in the list → remove it
    profileForm.enabled_rule_ids.splice(idx, 1)
  } else if (profileForm.enabled_rule_ids.length === 0) {
    // Currently "all enabled" (empty list) → user is unchecking this rule
    // Populate with all rules, then remove the unchecked one
    profileForm.enabled_rule_ids = moderationRules.value
      .map(r => r.rule_id)
      .filter(id => id !== ruleId)
  } else {
    // Rule is not in the list → add it
    profileForm.enabled_rule_ids.push(ruleId)
  }
}

// ==================== 用户自定义规则 ====================

const userRulesLoading = ref(false)
const userRuleSaving = ref(false)
const showUserRuleDialog = ref(false)
const editingUserRule = ref<UserModerationRule | null>(null)
const userModerationRules = ref<UserModerationRule[]>([])

const userRuleForm = reactive({
  rule_name: '',
  rule_type: 'KEYWORD',
  rule_pattern: '',
  action: 'BLOCK',
  risk_category: '',
  enabled: true,
})

async function loadUserModerationRules() {
  userRulesLoading.value = true
  try {
    userModerationRules.value = await listUserModerationRules()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('compliance.customRules.title')))
  } finally {
    userRulesLoading.value = false
  }
}

function openEditUserRuleDialog(rule: UserModerationRule) {
  editingUserRule.value = rule
  userRuleForm.rule_name = rule.rule_name
  userRuleForm.rule_type = rule.rule_type
  userRuleForm.rule_pattern = rule.rule_pattern
  userRuleForm.action = rule.action
  userRuleForm.risk_category = rule.risk_category
  userRuleForm.enabled = rule.enabled
  showUserRuleDialog.value = true
}

function closeUserRuleDialog() {
  showUserRuleDialog.value = false
  editingUserRule.value = null
  userRuleForm.rule_name = ''
  userRuleForm.rule_type = 'KEYWORD'
  userRuleForm.rule_pattern = ''
  userRuleForm.action = 'BLOCK'
  userRuleForm.risk_category = ''
  userRuleForm.enabled = true
}

async function handleCreateUserRule() {
  userRuleSaving.value = true
  try {
    await createUserModerationRule({
      rule_name: userRuleForm.rule_name,
      rule_type: userRuleForm.rule_type,
      rule_pattern: userRuleForm.rule_pattern,
      action: userRuleForm.action,
      risk_category: userRuleForm.risk_category || undefined,
      enabled: userRuleForm.enabled,
    })
    appStore.showSuccess(t('compliance.customRules.createSuccess'))
    closeUserRuleDialog()
    await loadUserModerationRules()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('compliance.customRules.title')))
  } finally {
    userRuleSaving.value = false
  }
}

async function handleUpdateUserRule() {
  if (!editingUserRule.value) return
  userRuleSaving.value = true
  try {
    await updateUserModerationRule(editingUserRule.value.rule_id, {
      rule_name: userRuleForm.rule_name,
      rule_type: userRuleForm.rule_type,
      rule_pattern: userRuleForm.rule_pattern,
      action: userRuleForm.action,
      risk_category: userRuleForm.risk_category || undefined,
      enabled: userRuleForm.enabled,
    })
    appStore.showSuccess(t('compliance.customRules.updateSuccess'))
    closeUserRuleDialog()
    await loadUserModerationRules()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('compliance.customRules.title')))
  } finally {
    userRuleSaving.value = false
  }
}

async function handleDeleteUserRule(ruleId: string) {
  if (!confirm(t('compliance.customRules.deleteConfirm'))) return
  userRuleSaving.value = true
  try {
    await deleteUserModerationRule(ruleId)
    appStore.showSuccess(t('compliance.customRules.deleteSuccess'))
    await loadUserModerationRules()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('compliance.customRules.title')))
  } finally {
    userRuleSaving.value = false
  }
}

// ==================== 跨法域映射 ====================

const jurisdictionLoading = ref(false)
const jurisdictionMappingLoading = ref(false)
const supportedJurisdictions = ref<string[]>([])
const jurisdictionResult = ref<JurisdictionMappingResult | null>(null)

const jurisdictionForm = reactive<{
  region: string
  industry: string
  serviceType: string
}>({
  region: '',
  industry: '',
  serviceType: '',
})

const jurisdictionSaving = ref(false)
const jurisdictionSaveApplyRules = ref(false)
const savedJurisdictionMapping = ref<{ applied_rules: string[] } | null>(null)

async function loadSupportedJurisdictions() {
  jurisdictionLoading.value = true
  try {
    supportedJurisdictions.value = await getSupportedJurisdictions()
    await loadUserJurisdictionMapping()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('compliance.jurisdiction.title')))
  } finally {
    jurisdictionLoading.value = false
  }
}

async function loadUserJurisdictionMapping() {
  try {
    const mapping = await getUserJurisdictionMapping()
    if (mapping) {
      jurisdictionForm.region = mapping.company_region
      jurisdictionForm.industry = mapping.industry || ''
      jurisdictionForm.serviceType = mapping.service_type || ''
      jurisdictionResult.value = {
        company_region: mapping.company_region,
        industry: mapping.industry || '',
        service_type: mapping.service_type || '',
        applicable_regulations: mapping.applicable_regulations,
        required_measures: mapping.required_measures,
        risk_level: mapping.risk_level,
        recommended_actions: mapping.recommended_actions,
      }
      savedJurisdictionMapping.value = { applied_rules: mapping.applied_rules || [] }
      jurisdictionSaveApplyRules.value = mapping.applied_rules && mapping.applied_rules.length > 0
    }
  } catch (err) {
    console.warn('Failed to load user jurisdiction mapping:', err)
  }
}

async function handleJurisdictionMapping() {
  if (!jurisdictionForm.region) return
  jurisdictionMappingLoading.value = true
  jurisdictionResult.value = null
  savedJurisdictionMapping.value = null
  try {
    jurisdictionResult.value = await getJurisdictionMapping({
      company_region: jurisdictionForm.region,
      industry: jurisdictionForm.industry || undefined,
      service_type: jurisdictionForm.serviceType || undefined,
    })
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('compliance.jurisdiction.title')))
  } finally {
    jurisdictionMappingLoading.value = false
  }
}

async function handleSaveJurisdictionMapping() {
  if (!jurisdictionResult.value) return
  jurisdictionSaving.value = true
  try {
    const result = await saveJurisdictionMapping({
      company_region: jurisdictionResult.value.company_region,
      industry: jurisdictionResult.value.industry || undefined,
      service_type: jurisdictionResult.value.service_type || undefined,
      apply_rules: jurisdictionSaveApplyRules.value,
    })
    savedJurisdictionMapping.value = { applied_rules: result.applied_rules || [] }
    appStore.showSuccess(t('compliance.jurisdiction.saveSuccess'))
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('compliance.jurisdiction.title')))
  } finally {
    jurisdictionSaving.value = false
  }
}

function onJurisdictionRegionChange() {
  jurisdictionResult.value = null
  savedJurisdictionMapping.value = null
}

function jurisdictionRiskBadgeClass(riskLevel: string): string {
  const map: Record<string, string> = {
    LOW: 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300',
    MEDIUM: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-300',
    HIGH: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300',
  }
  return map[riskLevel] || 'bg-gray-100 text-gray-700 dark:bg-gray-900/30 dark:text-gray-300'
}

// ==================== DPA 生成 ====================

const dpaGenerating = ref(false)

const dpaForm = reactive<{
  controllerName: string
  controllerContact: string
}>({
  controllerName: '',
  controllerContact: '',
})

async function handleGenerateDPA() {
  if (!dpaForm.controllerName || !dpaForm.controllerContact) return
  dpaGenerating.value = true
  try {
    await generateDPA({
      controller_name: dpaForm.controllerName,
      controller_contact: dpaForm.controllerContact,
    })
    appStore.showSuccess(t('compliance.dpa.success'))
    dpaForm.controllerName = ''
    dpaForm.controllerContact = ''
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('compliance.dpa.title')))
  } finally {
    dpaGenerating.value = false
  }
}

// ==================== 合规凭证 ====================

const credentialsLoading = ref(false)
const credentials = ref<ComplianceCredential[]>([])

async function loadCredentials() {
  credentialsLoading.value = true
  try {
    credentials.value = await listCredentials()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('compliance.credentials.title')))
  } finally {
    credentialsLoading.value = false
  }
}

// ==================== 审计日志 ====================

const auditLogsLoading = ref(false)
const auditLogs = ref<PaginatedResult<ComplianceAuditLog>>({
  items: [],
  total: 0,
  page: 1,
  page_size: 10,
  total_pages: 0,
})

async function loadAuditLogs(page: number = 1) {
  auditLogsLoading.value = true
  try {
    auditLogs.value = await listAuditLogs({ page, page_size: 10 })
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('compliance.audit.title')))
  } finally {
    auditLogsLoading.value = false
  }
}

// ==================== 风险分析 ====================

const riskTagsLoading = ref(false)
const riskTags = ref<RiskTagsCatalog>({
  model_tags: [],
  risk_tags: [],
})

async function loadRiskTags() {
  riskTagsLoading.value = true
  try {
    riskTags.value = await getRiskTags()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('compliance.risk.title')))
  } finally {
    riskTagsLoading.value = false
  }
}

// ==================== EU AI Act 评估 ====================

const euaiAssessmentLoading = ref(false)
const euaiExporting = ref(false)
const euaiAssessment = ref<Record<string, unknown>>({})

async function loadEUAIActAssessment() {
  euaiAssessmentLoading.value = true
  try {
    euaiAssessment.value = await getEUAIActAssessment()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('compliance.euAiAct.title')))
  } finally {
    euaiAssessmentLoading.value = false
  }
}

async function handleExportEUAIAct() {
  euaiExporting.value = true
  try {
    await exportEUAIActAssessment()
    appStore.showSuccess(t('compliance.euAiAct.exportSuccess'))
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('compliance.euAiAct.title')))
  } finally {
    euaiExporting.value = false
  }
}

// ==================== GDPR 处理记录 (ROPA) ====================

const ropaLoading = ref(false)
const ropaRecord = ref<Record<string, unknown>>({})

async function loadDataProcessingRecord() {
  ropaLoading.value = true
  try {
    ropaRecord.value = await getDataProcessingRecord()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('compliance.ropa.title')))
  } finally {
    ropaLoading.value = false
  }
}

onMounted(() => {
  loadConsents()
  loadComplianceProfile()
  loadTemplates()
  loadModerationRules()
  loadUserModerationRules()
  loadErasureRequests()
  loadSupportedJurisdictions()
  loadCredentials()
  loadAuditLogs()
  loadRiskTags()
  loadEUAIActAssessment()
  loadDataProcessingRecord()
})
</script>
