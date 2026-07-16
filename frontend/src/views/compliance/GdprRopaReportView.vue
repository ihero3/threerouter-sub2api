<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'

const { t } = useI18n()
const router = useRouter()

const ropaData = {
  controller: {
    name: 'Customer Organization',
    role: 'Service Customer / Data Controller',
    description: 'Business organizations using ThreeRouter AI API services',
  },
  processor: {
    name: 'ThreeRouter Technology Ltd.',
    legalEntity: 'ThreeRouter Technology Ltd.',
    contact: 'privacy@threerouter.com',
    dpoContact: 'dpo@threerouter.com',
    processingOperations: ['Collection', 'Transmission', 'Routing', 'Storage where configured', 'Deletion'],
  },
  processingActivities: [
    {
      id: 'PA-001',
      description: 'AI API Request Processing',
      dataCategories: ['Personal data in prompts', 'User authentication data', 'API usage metadata'],
      legalBasis: 'Contract Performance (Art. 6(1)(b) GDPR)',
      lawfulProcessingCondition: 'Processing is necessary for the performance of a contract to which the data subject is party or in order to take steps at the request of the data subject prior to entering into a contract',
      recipients: ['Third-party AI model providers', 'Cloud infrastructure providers'],
      crossBorderTransfer: {
        mechanism: 'EU Standard Contractual Clauses (SCCs)',
        thirdCountry: 'Singapore',
        adequacyDecision: false,
      },
      retentionPeriod: 'Depends on customer data retention settings (Aggregate: 30 days, Audit: 90 days, Detail: configurable)',
      dataSubjectRights: ['Access', 'Correction', 'Erasure', 'Restriction', 'Data Portability', 'Objection'],
    },
    {
      id: 'PA-002',
      description: 'System Logging and Monitoring',
      dataCategories: ['API request logs', 'Error logs', 'Performance metrics', 'User session data'],
      legalBasis: 'Legitimate Interest (Art. 6(1)(f) GDPR)',
      lawfulProcessingCondition: 'Processing is necessary for the purposes of the legitimate interests pursued by the controller or by a third party, except where such interests are overridden by the interests or fundamental rights and freedoms of the data subject which require protection of personal data, in particular where the data subject is a child',
      recipients: ['Internal security team', 'Cloud infrastructure providers'],
      crossBorderTransfer: {
        mechanism: 'EU Standard Contractual Clauses (SCCs)',
        thirdCountry: 'Singapore',
        adequacyDecision: false,
      },
      retentionPeriod: '90 days for audit logs',
      dataSubjectRights: ['Access', 'Correction', 'Erasure', 'Restriction', 'Data Portability', 'Objection'],
    },
    {
      id: 'PA-003',
      description: 'Billing and Usage Analytics',
      dataCategories: ['Usage statistics', 'Billing information', 'Account details'],
      legalBasis: 'Contract Performance (Art. 6(1)(b) GDPR)',
      lawfulProcessingCondition: 'Processing is necessary for the performance of a contract to which the data subject is party or in order to take steps at the request of the data subject prior to entering into a contract',
      recipients: ['Internal finance team', 'Payment processors'],
      crossBorderTransfer: {
        mechanism: 'EU Standard Contractual Clauses (SCCs)',
        thirdCountry: 'Singapore',
        adequacyDecision: false,
      },
      retentionPeriod: '7 years for financial records',
      dataSubjectRights: ['Access', 'Correction', 'Erasure', 'Restriction', 'Data Portability', 'Objection'],
    },
  ],
  safeguards: [
    'Zero Data Retention (ZDR) by default',
    'Encryption at rest and in transit',
    'Access control and authentication',
    'Regular security audits',
    'Incident response procedures',
    'Data Protection Impact Assessments (DPIAs)',
  ],
}
</script>

<template>
  <div class="min-h-screen bg-gray-50 dark:bg-gray-900">
    <div class="max-w-4xl mx-auto py-8 px-4">
      <div class="bg-white dark:bg-gray-800 rounded-xl shadow-lg overflow-hidden">
        <div class="bg-gradient-to-r from-green-600 to-emerald-600 px-8 py-6">
          <div class="flex items-center justify-between">
            <div>
              <h1 class="text-2xl font-bold text-white">{{ t('compliance.features.gdpr.title') }}</h1>
              <p class="text-green-100 mt-1">{{ t('compliance.features.gdpr.description') }}</p>
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
          <div class="mb-8">
            <h2 class="text-xl font-semibold text-gray-900 dark:text-white mb-4 flex items-center">
              <span class="w-8 h-8 bg-green-100 dark:bg-green-900 rounded-full flex items-center justify-center text-green-600 dark:text-green-300 mr-3 text-sm font-bold">1</span>
              Controller Information
            </h2>
            <div class="bg-gray-50 dark:bg-gray-700 rounded-lg p-4">
              <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div>
                  <p class="text-sm text-gray-500 dark:text-gray-400">Name</p>
                  <p class="font-medium text-gray-900 dark:text-white">{{ ropaData.controller.name }}</p>
                </div>
                <div>
                  <p class="text-sm text-gray-500 dark:text-gray-400">Role</p>
                  <p class="font-medium text-gray-900 dark:text-white">{{ ropaData.controller.role }}</p>
                </div>
              </div>
              <div class="mt-4">
                <p class="text-sm text-gray-500 dark:text-gray-400">Description</p>
                <p class="text-gray-900 dark:text-white">{{ ropaData.controller.description }}</p>
              </div>
            </div>
          </div>

          <div class="mb-8">
            <h2 class="text-xl font-semibold text-gray-900 dark:text-white mb-4 flex items-center">
              <span class="w-8 h-8 bg-green-100 dark:bg-green-900 rounded-full flex items-center justify-center text-green-600 dark:text-green-300 mr-3 text-sm font-bold">2</span>
              Processor Information
            </h2>
            <div class="bg-gray-50 dark:bg-gray-700 rounded-lg p-4">
              <div class="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
                <div>
                  <p class="text-sm text-gray-500 dark:text-gray-400">Name</p>
                  <p class="font-medium text-gray-900 dark:text-white">{{ ropaData.processor.name }}</p>
                </div>
                <div>
                  <p class="text-sm text-gray-500 dark:text-gray-400">Legal Entity</p>
                  <p class="font-medium text-gray-900 dark:text-white">{{ ropaData.processor.legalEntity }}</p>
                </div>
                <div>
                  <p class="text-sm text-gray-500 dark:text-gray-400">Contact</p>
                  <p class="font-medium text-gray-900 dark:text-white">{{ ropaData.processor.contact }}</p>
                </div>
                <div>
                  <p class="text-sm text-gray-500 dark:text-gray-400">DPO Contact</p>
                  <p class="font-medium text-gray-900 dark:text-white">{{ ropaData.processor.dpoContact }}</p>
                </div>
              </div>
              <div>
                <p class="text-sm text-gray-500 dark:text-gray-400">Processing Operations</p>
                <div class="flex flex-wrap gap-2 mt-2">
                  <span v-for="operation in ropaData.processor.processingOperations" :key="operation" class="bg-green-100 dark:bg-green-900 text-green-600 dark:text-green-300 px-3 py-1 rounded-full text-sm">
                    {{ operation }}
                  </span>
                </div>
              </div>
            </div>
          </div>

          <div class="mb-8">
            <h2 class="text-xl font-semibold text-gray-900 dark:text-white mb-4 flex items-center">
              <span class="w-8 h-8 bg-green-100 dark:bg-green-900 rounded-full flex items-center justify-center text-green-600 dark:text-green-300 mr-3 text-sm font-bold">3</span>
              Processing Activities (Art. 30 GDPR)
            </h2>
            <div class="space-y-6">
              <div v-for="activity in ropaData.processingActivities" :key="activity.id" class="border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden">
                <div class="bg-gray-50 dark:bg-gray-700 px-4 py-3 flex items-center justify-between">
                  <span class="font-medium text-gray-900 dark:text-white">{{ activity.id }}</span>
                  <span class="text-sm text-gray-500 dark:text-gray-400">{{ activity.description }}</span>
                </div>
                <div class="p-4">
                  <div class="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
                    <div>
                      <p class="text-sm text-gray-500 dark:text-gray-400">Data Categories</p>
                      <div class="flex flex-wrap gap-1 mt-1">
                        <span v-for="category in activity.dataCategories" :key="category" class="bg-blue-100 dark:bg-blue-900 text-blue-600 dark:text-blue-300 px-2 py-0.5 rounded text-xs">
                          {{ category }}
                        </span>
                      </div>
                    </div>
                    <div>
                      <p class="text-sm text-gray-500 dark:text-gray-400">Legal Basis</p>
                      <p class="text-gray-900 dark:text-white text-sm mt-1">{{ activity.legalBasis }}</p>
                    </div>
                  </div>
                  <div class="mb-4">
                    <p class="text-sm text-gray-500 dark:text-gray-400">Lawful Processing Condition</p>
                    <p class="text-gray-600 dark:text-gray-400 text-sm mt-1">{{ activity.lawfulProcessingCondition }}</p>
                  </div>
                  <div class="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
                    <div>
                      <p class="text-sm text-gray-500 dark:text-gray-400">Recipients</p>
                      <ul class="text-sm text-gray-900 dark:text-white mt-1">
                        <li v-for="recipient in activity.recipients" :key="recipient" class="flex items-center">
                          <span class="w-1.5 h-1.5 bg-gray-400 rounded-full mr-2"></span>
                          {{ recipient }}
                        </li>
                      </ul>
                    </div>
                    <div>
                      <p class="text-sm text-gray-500 dark:text-gray-400">Retention Period</p>
                      <p class="text-gray-900 dark:text-white text-sm mt-1">{{ activity.retentionPeriod }}</p>
                    </div>
                  </div>
                  <div class="bg-yellow-50 dark:bg-yellow-900/30 border border-yellow-200 dark:border-yellow-700 rounded-lg p-3 mb-4">
                    <p class="text-sm text-yellow-800 dark:text-yellow-300 font-medium">Cross-Border Transfer</p>
                    <div class="grid grid-cols-1 md:grid-cols-3 gap-2 mt-2 text-sm">
                      <div>
                        <span class="text-gray-500 dark:text-gray-400">Mechanism:</span>
                        <span class="text-gray-900 dark:text-white ml-1">{{ activity.crossBorderTransfer.mechanism }}</span>
                      </div>
                      <div>
                        <span class="text-gray-500 dark:text-gray-400">Third Country:</span>
                        <span class="text-gray-900 dark:text-white ml-1">{{ activity.crossBorderTransfer.thirdCountry }}</span>
                      </div>
                      <div>
                        <span class="text-gray-500 dark:text-gray-400">Adequacy:</span>
                        <span :class="activity.crossBorderTransfer.adequacyDecision ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'" class="ml-1">{{ activity.crossBorderTransfer.adequacyDecision ? 'Yes' : 'No' }}</span>
                      </div>
                    </div>
                  </div>
                  <div>
                    <p class="text-sm text-gray-500 dark:text-gray-400">Data Subject Rights</p>
                    <div class="flex flex-wrap gap-2 mt-1">
                      <span v-for="right in activity.dataSubjectRights" :key="right" class="bg-purple-100 dark:bg-purple-900 text-purple-600 dark:text-purple-300 px-2 py-0.5 rounded text-xs">
                        {{ right }}
                      </span>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div class="mb-8">
            <h2 class="text-xl font-semibold text-gray-900 dark:text-white mb-4 flex items-center">
              <span class="w-8 h-8 bg-green-100 dark:bg-green-900 rounded-full flex items-center justify-center text-green-600 dark:text-green-300 mr-3 text-sm font-bold">4</span>
              Technical and Organizational Safeguards
            </h2>
            <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
              <div v-for="(safeguard, index) in ropaData.safeguards" :key="index" class="bg-gray-50 dark:bg-gray-700 rounded-lg p-3">
                <span class="w-6 h-6 bg-green-500 rounded-full flex items-center justify-center text-white text-xs font-bold mr-3">✓</span>
                <span class="text-gray-900 dark:text-white">{{ safeguard }}</span>
              </div>
            </div>
          </div>

          <div class="border-t border-gray-200 dark:border-gray-700 pt-6 mt-8">
            <div class="flex items-center justify-between">
              <p class="text-sm text-gray-500 dark:text-gray-400">Generated by ThreeRouter Compliance System - GDPR Art. 30 ROPA</p>
              <p class="text-sm text-gray-500 dark:text-gray-400">{{ new Date().toISOString().split('T')[0] }}</p>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
