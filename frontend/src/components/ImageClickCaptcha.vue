<template>
  <div class="w-full">
    <p class="mb-2 text-center text-sm text-gray-600 dark:text-dark-300">
      {{ t('auth.clickCaptcha.imageClickPrompt') }}
      <template v-for="(hint, i) in challenge.prompt" :key="`${hint.color}-${hint.shape}-${i}`">
        <span class="inline-flex items-center gap-1 font-medium text-gray-800 dark:text-dark-100">
          <span
            class="inline-block h-3 w-3 rounded-full border border-black/10 align-middle"
            :style="{ backgroundColor: colorHex(hint.color) }"
          />
          {{ colorLabel(hint.color) }}{{ t('auth.clickCaptcha.hintSeparator') }}{{ shapeLabel(hint.shape) }}
        </span>
        <span v-if="i < challenge.prompt.length - 1">{{ t('auth.clickCaptcha.promptSeparator') }}</span>
      </template>
    </p>

    <div
      class="relative mx-auto w-full overflow-hidden rounded-xl border border-gray-300 dark:border-dark-600"
      :style="{ aspectRatio: `${challenge.width} / ${challenge.height}` }"
    >
      <img
        ref="imageRef"
        :src="imageUrl"
        alt=""
        class="block h-full w-full cursor-crosshair select-none"
        draggable="false"
        @click="handleClick"
      />
      <span
        v-for="(point, i) in clicks"
        :key="i"
        class="pointer-events-none absolute flex h-5 w-5 -translate-x-1/2 -translate-y-1/2 items-center justify-center rounded-full bg-primary-500 text-[11px] font-semibold text-white shadow"
        :style="{ left: `${(point.x / challenge.width) * 100}%`, top: `${(point.y / challenge.height) * 100}%` }"
      >{{ i + 1 }}</span>
    </div>

    <div class="mt-2 flex items-center justify-between">
      <button
        type="button"
        class="text-sm text-primary-600 hover:underline disabled:opacity-50 dark:text-primary-400"
        :disabled="submitting || refreshing"
        @click="emit('refresh')"
      >
        {{ refreshing ? t('auth.clickCaptcha.refreshing') : t('auth.clickCaptcha.refresh') }}
      </button>
      <span class="text-xs text-gray-400">{{ clicks.length }}/{{ challenge.click_count }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ClickPoint, ImageClickChallenge } from '@/types'

const props = defineProps<{
  challenge: ImageClickChallenge
  submitting?: boolean
  refreshing?: boolean
}>()

const emit = defineEmits<{
  (e: 'submit', clicks: ClickPoint[]): void
  (e: 'refresh'): void
}>()

const { t } = useI18n()

const imageRef = ref<HTMLImageElement | null>(null)
const clicks = ref<ClickPoint[]>([])

const imageUrl = computed(() => `data:image/png;base64,${props.challenge.image}`)

// 新题面到达时清空已选点
watch(
  () => props.challenge,
  () => {
    clicks.value = []
  },
)

const colorMap: Record<string, string> = {
  red: '#ef4444',
  blue: '#3b82f6',
  green: '#22c55e',
  yellow: '#eab308',
  purple: '#a855f7',
}

function colorHex(color: string): string {
  return colorMap[color] ?? '#9ca3af'
}

function shapeLabel(shape: string): string {
  return t(`auth.clickCaptcha.shapes.${shape}`)
}

function colorLabel(color: string): string {
  return t(`auth.clickCaptcha.colors.${color}`)
}

// 把浏览器视口坐标换算为图片像素坐标（题面宽高为服务端权威尺寸）
function handleClick(event: MouseEvent) {
  if (props.submitting || !imageRef.value) return
  const rect = imageRef.value.getBoundingClientRect()
  if (rect.width <= 0 || rect.height <= 0) return
  const x = ((event.clientX - rect.left) * props.challenge.width) / rect.width
  const y = ((event.clientY - rect.top) * props.challenge.height) / rect.height
  if (x < 0 || y < 0 || x > props.challenge.width || y > props.challenge.height) return

  clicks.value = [...clicks.value, { x, y }]
  if (clicks.value.length >= props.challenge.click_count) {
    emit('submit', clicks.value)
  }
}

defineExpose({
  reset: () => {
    clicks.value = []
  },
})
</script>
