<template>
  <div class="w-full">
    <template v-if="verified">
      <div class="rounded-lg bg-green-50 px-3 py-2 text-sm text-green-700 dark:bg-green-900/20 dark:text-green-400">
        ✓ {{ t('auth.clickCaptcha.passed') }}
      </div>
    </template>
    <template v-else>
      <button
        v-if="!challenge"
        type="button"
        class="btn btn-secondary w-full"
        :disabled="loading"
        @click="startChallenge"
      >
        {{ loading ? t('auth.clickCaptcha.verifying') : t('auth.clickCaptcha.start') }}
      </button>

      <div v-else>
        <ImageClickCaptcha
          v-if="challenge.mode === 'image_click' && challenge.image_click"
          :challenge="challenge.image_click"
          :submitting="submitting"
          :refreshing="refreshing"
          @submit="onImageClickSubmit"
          @refresh="onUserRefresh"
        />
        <SliderCaptcha
          v-else-if="challenge.mode === 'slider' && challenge.slider"
          :challenge="challenge.slider"
          :submitting="submitting"
          :refreshing="refreshing"
          @submit="onSliderSubmit"
          @refresh="onUserRefresh"
        />
        <!-- 后端返回了无法识别的题面（如后端未重启/版本不匹配）时，给出可见错误而不是静默空白 -->
        <p v-else class="mt-2 text-center text-sm text-red-500">
          {{ t('auth.clickCaptcha.initFailed') }}
        </p>
        <p v-if="error" class="mt-2 text-center text-sm text-red-500">{{ error }}</p>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import ImageClickCaptcha from '@/components/ImageClickCaptcha.vue'
import SliderCaptcha from '@/components/SliderCaptcha.vue'
import {
  createRegistrationCaptchaChallenge,
  verifyRegistrationCaptcha,
  type CaptchaVerifyProof,
} from '@/api/registrationRisk'
import type { ClickPoint, RegistrationCaptchaChallenge, TrajectoryPoint } from '@/types'

const emit = defineEmits<{
  (e: 'verified', token: string): void
  (e: 'expired'): void
}>()

const { t } = useI18n()

const challenge = ref<RegistrationCaptchaChallenge | null>(null)
const captchaToken = ref('')
const verified = ref(false)
const loading = ref(false)
const refreshing = ref(false)
const submitting = ref(false)
const error = ref('')

// 拉取一道新题；不处理 UI 状态，交由调用方决定。
async function fetchChallenge(): Promise<boolean> {
  try {
    challenge.value = await createRegistrationCaptchaChallenge()
    return true
  } catch {
    return false
  }
}

async function startChallenge() {
  if (loading.value) return
  loading.value = true
  error.value = ''
  captchaToken.value = ''
  verified.value = false
  const ok = await fetchChallenge()
  if (!ok) error.value = t('auth.clickCaptcha.initFailed')
  loading.value = false
}

async function refreshChallenge(clearError: boolean) {
  if (refreshing.value) return
  refreshing.value = true
  const ok = await fetchChallenge()
  if (!ok) {
    error.value = t('auth.clickCaptcha.initFailed')
  } else if (clearError) {
    error.value = ''
  }
  refreshing.value = false
}

function onUserRefresh() {
  void refreshChallenge(true)
}

async function submitProof(proof: CaptchaVerifyProof) {
  if (!challenge.value || submitting.value) return
  submitting.value = true
  error.value = ''
  try {
    const result = await verifyRegistrationCaptcha(challenge.value.challenge_id, proof)
    captchaToken.value = result.captcha_token
    verified.value = true
    emit('verified', captchaToken.value)
  } catch {
    error.value = t('auth.clickCaptcha.invalid')
    // 答错自动换新题，保留错误提示供用户理解
    await refreshChallenge(false)
  } finally {
    submitting.value = false
  }
}

function onImageClickSubmit(clicks: ClickPoint[]) {
  void submitProof({ clicks })
}

function onSliderSubmit(payload: { drop_x: number; trajectory: TrajectoryPoint[] }) {
  void submitProof({ drop_x: payload.drop_x, trajectory: payload.trajectory })
}
</script>
