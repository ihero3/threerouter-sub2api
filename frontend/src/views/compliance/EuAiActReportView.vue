<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'

const { t } = useI18n()
const router = useRouter()

const reportData = {
  company: {
    name: 'ThreeRouter Technology Ltd.',
    contact: 'privacy@threerouter.com',
    jurisdiction: 'Singapore',
  },
  assessment: {
    date: '2026-07-13',
    version: '1.0',
    scope: 'ThreeRouter AI Gateway Service',
  },
  roles: {
    provider: { status: false, basis: 'Does not develop, modify, or place AI models on the EU market under its own name' },
    deployer: { status: false, basis: 'Does not deploy AI systems for use by third parties' },
    importer: { status: false, basis: 'Does not import AI systems developed outside the EU' },
    distributor: { status: false, basis: 'Does not distribute AI systems to end-users' },
    infrastructure_service_provider: { status: true, basis: 'Provides infrastructure for AI model routing and transmission' },
  },
  highRiskAssessment: {
    isHighRisk: false,
    reasoning: 'The ThreeRouter AI Gateway Service does not fall within any of the high-risk categories defined in EU AI Act Annex III.',
    categories: [
      { name: 'Biometric identification and categorisation systems', applicable: false },
      { name: 'Management and operation of critical infrastructure', applicable: false },
      { name: 'Education and vocational training', applicable: false },
      { name: 'Employment, worker management and access to self-employment', applicable: false },
      { name: 'Essential private and public services', applicable: false },
      { name: 'Law enforcement', applicable: false },
      { name: 'Migration, asylum and border control management', applicable: false },
      { name: 'Administration of justice and democratic processes', applicable: false },
    ],
  },
  transparency: {
    obligations: [
      'Information about the AI system and its intended purpose',
      'Identity of the provider and contact details',
      'Information about training data used',
      'Information about the capabilities and limitations of the AI system',
      'Information about human oversight measures',
    ],
    disclosureMethod: 'Documentation provided via API documentation portal',
  },
  accountability: {
    measures: [
      'Policies and procedures for monitoring AI system performance',
      'Incident reporting and response procedures',
      'Data protection impact assessments (DPIAs)',
      'Human oversight mechanisms',
      'Audit trail and logging',
    ],
  },
}
</script>

<template>
  <div class="min-h-screen bg-gray-50 dark:bg-gray-900">
    <div class="max-w-4xl mx-auto py-8 px-4">
      <div class="bg-white dark:bg-gray-800 rounded-xl shadow-lg overflow-hidden">
        <div class="bg-gradient-to-r from-blue-600 to-indigo-600 px-8 py-6">
          <div class="flex items-center justify-between">
            <div>
              <h1 class="text-2xl font-bold text-white">{{ t('compliance.features.euai.title') }}</h1>
              <p class="text-blue-100 mt-1">{{ t('compliance.features.euai.description') }}</p>
            </div>
            <button
              @click="router.push('/compliance')"
              class="bg-white/20 hover:bg-white/30 text-white px-4 py-2 rounded-lg transition-colors"
            >
              {{ t('whitepaper.back') }}
            </button>
          </div>
        </div>

        <div class="p-8">
          <div class="grid grid-cols-1 md:grid-cols-3 gap-6 mb-8">
            <div class="bg-gray-50 dark:bg-gray-700 rounded-lg p-4">
              <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('whitepaper.toc') }}</p>
              <p class="font-medium text-gray-900 dark:text-white mt-1">{{ reportData.company.name }}</p>
            </div>
            <div class="bg-gray-50 dark:bg-gray-700 rounded-lg p-4">
              <p class="text-sm text-gray-500 dark:text-gray-400">Assessment Date</p>
              <p class="font-medium text-gray-900 dark:text-white mt-1">{{ reportData.assessment.date }}</p>
            </div>
            <div class="bg-gray-50 dark:bg-gray-700 rounded-lg p-4">
              <p class="text-sm text-gray-500 dark:text-gray-400">Version</p>
              <p class="font-medium text-gray-900 dark:text-white mt-1">{{ reportData.assessment.version }}</p>
            </div>
          </div>

          <div class="mb-8">
            <h2 class="text-xl font-semibold text-gray-900 dark:text-white mb-4 flex items-center">
              <span class="w-8 h-8 bg-blue-100 dark:bg-blue-900 rounded-full flex items-center justify-center text-blue-600 dark:text-blue-300 mr-3 text-sm font-bold">1</span>
              AI System Role Identification
            </h2>
            <div class="overflow-x-auto">
              <table class="w-full text-sm">
                <thead>
                  <tr class="border-b border-gray-200 dark:border-gray-700">
                    <th class="text-left py-3 px-4 font-medium text-gray-500 dark:text-gray-400">Role</th>
                    <th class="text-center py-3 px-4 font-medium text-gray-500 dark:text-gray-400">Applicable</th>
                    <th class="text-left py-3 px-4 font-medium text-gray-500 dark:text-gray-400">Basis</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="(value, key) in reportData.roles" :key="key" class="border-b border-gray-100 dark:border-gray-700">
                    <td class="py-3 px-4 text-gray-900 dark:text-white capitalize">{{ key.replace(/_/g, ' ') }}</td>
                    <td class="py-3 px-4 text-center">
                      <span :class="value.status ? 'bg-green-100 dark:bg-green-900 text-green-600 dark:text-green-300' : 'bg-red-100 dark:bg-red-900 text-red-600 dark:text-red-300'" class="px-2 py-1 rounded text-xs font-medium">
                        {{ value.status ? 'Yes' : 'No' }}
                      </span>
                    </td>
                    <td class="py-3 px-4 text-gray-600 dark:text-gray-400">{{ value.basis }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          <div class="mb-8">
            <h2 class="text-xl font-semibold text-gray-900 dark:text-white mb-4 flex items-center">
              <span class="w-8 h-8 bg-blue-100 dark:bg-blue-900 rounded-full flex items-center justify-center text-blue-600 dark:text-blue-300 mr-3 text-sm font-bold">2</span>
              High-Risk Scenario Assessment
            </h2>
            <div class="bg-yellow-50 dark:bg-yellow-900/30 border border-yellow-200 dark:border-yellow-700 rounded-lg p-4 mb-4">
              <div class="flex items-start">
                <span class="w-6 h-6 bg-yellow-500 rounded-full flex items-center justify-center text-white text-xs font-bold mr-3 mt-0.5">!</span>
                <div>
                  <p class="font-medium text-yellow-800 dark:text-yellow-300">High-Risk Classification</p>
                  <p class="text-yellow-700 dark:text-yellow-400 mt-1">{{ reportData.highRiskAssessment.isHighRisk ? 'This AI system is classified as HIGH RISK' : 'This AI system is NOT classified as high risk' }}</p>
                </div>
              </div>
            </div>
            <p class="text-gray-600 dark:text-gray-400 mb-4">{{ reportData.highRiskAssessment.reasoning }}</p>
            <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
              <div v-for="category in reportData.highRiskAssessment.categories" :key="category.name" :class="category.applicable ? 'bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-700' : 'bg-green-50 dark:bg-green-900/30 border border-green-200 dark:border-green-700'" class="rounded-lg p-3">
                <div class="flex items-center justify-between">
                  <span class="text-sm text-gray-700 dark:text-gray-300">{{ category.name }}</span>
                  <span :class="category.applicable ? 'text-red-600 dark:text-red-400' : 'text-green-600 dark:text-green-400'" class="text-xs font-medium">{{ category.applicable ? 'APPLICABLE' : 'NOT APPLICABLE' }}</span>
                </div>
              </div>
            </div>
          </div>

          <div class="mb-8">
            <h2 class="text-xl font-semibold text-gray-900 dark:text-white mb-4 flex items-center">
              <span class="w-8 h-8 bg-blue-100 dark:bg-blue-900 rounded-full flex items-center justify-center text-blue-600 dark:text-blue-300 mr-3 text-sm font-bold">3</span>
              Transparency Obligations (Article 50)
            </h2>
            <div class="bg-blue-50 dark:bg-blue-900/30 border border-blue-200 dark:border-blue-700 rounded-lg p-6">
              <p class="text-sm text-blue-800 dark:text-blue-300 mb-4">Disclosure Method: {{ reportData.transparency.disclosureMethod }}</p>
              <ul class="space-y-2">
                <li v-for="(item, index) in reportData.transparency.obligations" :key="index" class="flex items-start">
                  <span class="w-5 h-5 bg-blue-500 rounded-full flex items-center justify-center text-white text-xs mr-3 mt-0.5">✓</span>
                  <span class="text-gray-700 dark:text-gray-300">{{ item }}</span>
                </li>
              </ul>
            </div>
          </div>

          <div class="mb-8">
            <h2 class="text-xl font-semibold text-gray-900 dark:text-white mb-4 flex items-center">
              <span class="w-8 h-8 bg-blue-100 dark:bg-blue-900 rounded-full flex items-center justify-center text-blue-600 dark:text-blue-300 mr-3 text-sm font-bold">4</span>
              Accountability Measures
            </h2>
            <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
              <div v-for="(measure, index) in reportData.accountability.measures" :key="index" class="bg-gray-50 dark:bg-gray-700 rounded-lg p-3">
                <span class="text-gray-500 dark:text-gray-400 text-xs font-medium">Measure {{ index + 1 }}</span>
                <p class="text-gray-900 dark:text-white mt-1">{{ measure }}</p>
              </div>
            </div>
          </div>

          <div class="border-t border-gray-200 dark:border-gray-700 pt-6 mt-8">
            <div class="flex items-center justify-between">
              <p class="text-sm text-gray-500 dark:text-gray-400">Generated by ThreeRouter Compliance System</p>
              <p class="text-sm text-gray-500 dark:text-gray-400">{{ reportData.assessment.date }}</p>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
