<template>
  <div class="w-full">
    <p class="mb-2 text-center text-sm text-gray-600 dark:text-dark-300">
      {{ t('auth.clickCaptcha.rotationPrompt') }}
    </p>

    <div
      ref="boxRef"
      class="relative mx-auto cursor-grab touch-none select-none active:cursor-grabbing"
      :style="boxStyle"
      @pointerdown="onPointerDown"
      @pointermove="onPointerMove"
      @pointerup="onPointerUp"
      @pointercancel="onPointerUp"
    >
      <!-- 顶部参考标记：用户需把图片内的箭头旋到与该标记对齐 -->
      <div class="pointer-events-none absolute left-1/2 top-0 z-10 -translate-x-1/2 -translate-y-1">
        <svg width="20" height="14" viewBox="0 0 20 14" aria-hidden="true">
          <path d="M10 14 L0 0 L20 0 Z" :fill="markerColor" />
        </svg>
      </div>

      <img
        :src="imageUrl"
        alt=""
        class="pointer-events-none block select-none rounded-full shadow-lg ring-1 ring-black/10"
        :style="imageStyle"
        draggable="false"
      />
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
      <span class="text-xs text-gray-400">{{ t('auth.clickCaptcha.rotationHint') }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { RotationChallenge, TrajectoryPoint } from '@/types'

const props = defineProps<{
  challenge: RotationChallenge
  submitting?: boolean
  refreshing?: boolean
}>()

const emit = defineEmits<{
  (e: 'submit', payload: { angle: number; trajectory: TrajectoryPoint[] }): void
  (e: 'refresh'): void
}>()

const { t } = useI18n()

const boxRef = ref<HTMLDivElement | null>(null)

// 当前累计旋转角度（度，CSS 顺时针为正，与服务端约定一致）
const currentAngle = ref(0)
const dragging = ref(false)
const trajectory = ref<TrajectoryPoint[]>([])
let dragStartX = 0
let dragStartAngle = 0
let dragStartMs = 0

const imageUrl = computed(() => `data:image/png;base64,${props.challenge.image}`)

const boxStyle = computed(() => ({
  width: `${props.challenge.width}px`,
  maxWidth: '100%',
  aspectRatio: '1 / 1',
}))

const imageStyle = computed(() => ({
  width: '100%',
  height: '100%',
  transform: `rotate(${currentAngle.value}deg)`,
  transition: dragging.value ? 'none' : 'transform 0.15s ease-out',
}))

const markerColor = '#dc2626'

watch(
  () => props.challenge,
  () => {
    currentAngle.value = 0
    dragging.value = false
    trajectory.value = []
  },
  { immediate: true },
)

function toBoxX(clientX: number) {
  const box = boxRef.value
  if (!box) return null
  const rect = box.getBoundingClientRect()
  if (rect.width <= 0) return null
  return ((clientX - rect.left) / rect.width) * props.challenge.width
}

function onPointerDown(event: PointerEvent) {
  if (props.submitting || dragging.value) return
  const x = toBoxX(event.clientX)
  if (x === null) return
  dragging.value = true
  dragStartX = x
  dragStartAngle = currentAngle.value
  dragStartMs = performance.now()
  trajectory.value = [{ x: currentAngle.value, y: 0, t: 0 }]
  boxRef.value?.setPointerCapture?.(event.pointerId)
  event.preventDefault()
}

function onPointerMove(event: PointerEvent) {
  if (!dragging.value || props.submitting) return
  const x = toBoxX(event.clientX)
  if (x === null) return
  // 横向拖拽满一个画布宽度 = 旋转 360°
  const deltaX = x - dragStartX
  const angle = dragStartAngle + (deltaX / props.challenge.width) * 360
  currentAngle.value = angle
  trajectory.value = [
    ...trajectory.value,
    { x: angle, y: 0, t: Math.round(performance.now() - dragStartMs) },
  ]
}

function onPointerUp(event: PointerEvent) {
  if (!dragging.value) return
  dragging.value = false
  boxRef.value?.releasePointerCapture?.(event.pointerId)
  const points = [...trajectory.value]
  const last = points[points.length - 1]
  if (last && last.x !== currentAngle.value) {
    points.push({ x: currentAngle.value, y: 0, t: Math.round(performance.now() - dragStartMs) })
  }
  if (points.length > 1) {
    emit('submit', { angle: currentAngle.value, trajectory: points })
  }
}

defineExpose({
  reset: () => {
    currentAngle.value = 0
    trajectory.value = []
    dragging.value = false
  },
})
</script>
