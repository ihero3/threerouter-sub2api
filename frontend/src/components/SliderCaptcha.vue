<template>
  <div class="w-full">
    <p class="mb-2 text-center text-sm text-gray-600 dark:text-dark-300">
      {{ t('auth.clickCaptcha.sliderPrompt') }}
    </p>

    <div
      ref="boxRef"
      class="relative mx-auto w-full cursor-grab touch-none select-none overflow-hidden rounded-xl border border-gray-300 active:cursor-grabbing dark:border-dark-600"
      :style="{ aspectRatio: `${challenge.width} / ${challenge.height}` }"
      @pointerdown="onPointerDown"
      @pointermove="onPointerMove"
      @pointerup="onPointerUp"
      @pointercancel="onPointerUp"
    >
      <img
        :src="bgUrl"
        alt=""
        class="pointer-events-none block h-full w-full select-none"
        draggable="false"
      />
      <img
        ref="pieceRef"
        :src="pieceUrl"
        alt=""
        class="pointer-events-none absolute select-none drop-shadow-md"
        :style="pieceStyle"
        draggable="false"
        @load="onPieceLoad"
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
      <span class="text-xs text-gray-400">{{ t('auth.clickCaptcha.sliderHint') }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { SliderChallenge, TrajectoryPoint } from '@/types'

const props = defineProps<{
  challenge: SliderChallenge
  submitting?: boolean
  refreshing?: boolean
}>()

const emit = defineEmits<{
  (e: 'submit', payload: { drop_x: number; trajectory: TrajectoryPoint[] }): void
  (e: 'refresh'): void
}>()

const { t } = useI18n()

const boxRef = ref<HTMLDivElement | null>(null)
const pieceRef = ref<HTMLImageElement | null>(null)

// 拼图块左上角在背景图坐标系中的位置（像素）
const pieceLeft = ref(0)
const pieceTop = ref(0)
const pieceNaturalWidth = ref(0)

const dragging = ref(false)
const trajectory = ref<TrajectoryPoint[]>([])
let dragStartMs = 0
let grabOffsetX = 0
let grabOffsetY = 0

const bgUrl = computed(() => `data:image/png;base64,${props.challenge.bg_image}`)
const pieceUrl = computed(() => `data:image/png;base64,${props.challenge.piece_image}`)

// 拼图块渲染尺寸：与服务端掩码一致，主体 size，右侧凸起 size/5+2
const pieceWidth = computed(() => {
  const size = props.challenge.piece_size
  return pieceNaturalWidth.value > 0 ? pieceNaturalWidth.value : size + Math.floor(size / 5) + 2
})

const pieceStyle = computed(() => ({
  left: `${(pieceLeft.value / props.challenge.width) * 100}%`,
  top: `${(pieceTop.value / props.challenge.height) * 100}%`,
  width: `${(pieceWidth.value / props.challenge.width) * 100}%`,
  height: `${(props.challenge.piece_size / props.challenge.height) * 100}%`,
}))

watch(
  () => props.challenge,
  (next) => {
    pieceNaturalWidth.value = 0
    pieceLeft.value = 0
    pieceTop.value = next.piece_y
    dragging.value = false
    trajectory.value = []
  },
  { immediate: true },
)

function onPieceLoad() {
  const el = pieceRef.value
  if (el && el.naturalWidth > 0) {
    pieceNaturalWidth.value = el.naturalWidth
  }
}

// 把浏览器视口坐标换算为背景图像素坐标
function toImageCoords(clientX: number, clientY: number) {
  const box = boxRef.value
  if (!box) return null
  const rect = box.getBoundingClientRect()
  if (rect.width <= 0 || rect.height <= 0) return null
  return {
    x: ((clientX - rect.left) * props.challenge.width) / rect.width,
    y: ((clientY - rect.top) * props.challenge.height) / rect.height,
  }
}

function clamp(v: number, min: number, max: number) {
  return Math.min(Math.max(v, min), max)
}

function onPointerDown(event: PointerEvent) {
  if (props.submitting || dragging.value) return
  const point = toImageCoords(event.clientX, event.clientY)
  if (!point) return
  dragging.value = true
  dragStartMs = performance.now()
  grabOffsetX = point.x - pieceLeft.value
  grabOffsetY = point.y - pieceTop.value
  trajectory.value = [{ x: pieceLeft.value, y: pieceTop.value, t: 0 }]
  boxRef.value?.setPointerCapture?.(event.pointerId)
  event.preventDefault()
}

function onPointerMove(event: PointerEvent) {
  if (!dragging.value || props.submitting) return
  const point = toImageCoords(event.clientX, event.clientY)
  if (!point) return
  const maxLeft = props.challenge.width - props.challenge.piece_size
  const maxTop = props.challenge.height - props.challenge.piece_size
  pieceLeft.value = clamp(point.x - grabOffsetX, 0, maxLeft)
  pieceTop.value = clamp(point.y - grabOffsetY, 0, maxTop)
  trajectory.value = [
    ...trajectory.value,
    { x: pieceLeft.value, y: pieceTop.value, t: Math.round(performance.now() - dragStartMs) },
  ]
}

function onPointerUp(event: PointerEvent) {
  if (!dragging.value) return
  dragging.value = false
  boxRef.value?.releasePointerCapture?.(event.pointerId)
  const points = [...trajectory.value]
  // 收尾：补一个终态采样点，保证末端吸附特征可被服务端评估
  const last = points[points.length - 1]
  if (last && (last.x !== pieceLeft.value || last.y !== pieceTop.value)) {
    points.push({ x: pieceLeft.value, y: pieceTop.value, t: Math.round(performance.now() - dragStartMs) })
  }
  if (points.length > 1) {
    emit('submit', { drop_x: pieceLeft.value, trajectory: points })
  }
}

defineExpose({
  reset: () => {
    pieceLeft.value = 0
    pieceTop.value = props.challenge.piece_y
    trajectory.value = []
    dragging.value = false
  },
})
</script>
