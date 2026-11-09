<template>
  <div class="relative" ref="selectRef">
    <button
      type="button"
      class="flex w-full items-center justify-between rounded-lg border bg-white px-3 py-2.5 text-left text-sm text-zinc-900 transition-colors focus:outline-none focus:ring-2 focus:ring-indigo-500/20 dark:bg-zinc-700 dark:text-zinc-100"
      :class="[
        isOpen ? 'border-indigo-500 ring-2 ring-indigo-500/20' : 'border-zinc-300 dark:border-zinc-600',
        error ? 'border-red-500 focus:border-red-500 focus:ring-red-500/20' : ''
      ]"
      @click="isOpen = !isOpen"
    >
      <span :class="selectedName ? 'text-zinc-900 dark:text-zinc-100' : 'text-zinc-400'">
        {{ selectedName || placeholderText }}
      </span>
      <svg
        class="h-4 w-4 text-zinc-400 transition-transform"
        :class="{ 'rotate-180': isOpen }"
        fill="none"
        stroke="currentColor"
        viewBox="0 0 24 24"
      >
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
      </svg>
    </button>

    <div
      v-if="isOpen"
      class="absolute z-50 mt-1 max-h-64 w-full overflow-auto rounded-lg border border-zinc-200 bg-white py-1 shadow-lg dark:border-zinc-600 dark:bg-zinc-800"
    >
      <!-- Clear / No-department option -->
      <div
        v-if="allowClear"
        class="flex cursor-pointer items-center rounded px-2 py-1.5 text-sm transition-colors text-zinc-700 hover:bg-zinc-100 dark:text-zinc-200 dark:hover:bg-zinc-700"
        @click="select(null)"
      >
        <span class="ml-1 text-zinc-500">{{ clearLabelText }}</span>
      </div>

      <template v-if="flattened.length === 0">
        <div class="px-3 py-2 text-sm text-zinc-500 dark:text-zinc-400">
          {{ emptyTextValue }}
        </div>
      </template>
      <template v-else>
        <div
          v-for="node in flattened"
          :key="node.flatId"
          class="flex cursor-pointer items-center rounded px-2 py-1.5 text-sm transition-colors"
          :class="[
            modelValue === node.id
              ? 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-300'
              : 'text-zinc-700 hover:bg-zinc-100 dark:text-zinc-200 dark:hover:bg-zinc-700'
          ]"
          :style="{ paddingLeft: (node.depth * 16 + 8) + 'px' }"
          @click="select(node.id)"
        >
          <span
            v-if="node.hasChildren"
            class="mr-1 inline-block h-4 w-4 flex-shrink-0 text-zinc-400"
            @click.stop="toggleExpand(node.flatId)"
          >
            <svg v-if="expanded.has(node.flatId)" class="h-3 w-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
            </svg>
            <svg v-else class="h-3 w-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
            </svg>
          </span>
          <span v-else class="mr-1 inline-block h-4 w-4 flex-shrink-0" />
          <span class="truncate">{{ node.name }}</span>
        </div>
      </template>
    </div>

    <p v-if="error" class="mt-1 text-xs text-red-500">{{ error }}</p>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import type { DepartmentTreeNode } from '@/api/team'

interface Props {
  modelValue: number | '' | null
  departments: DepartmentTreeNode[]
  placeholder?: string
  allowClear?: boolean
  clearLabel?: string
  emptyText?: string
  error?: string
}

const { t } = useI18n()

const props = withDefaults(defineProps<Props>(), {
  allowClear: false,
  error: '',
})

// defineProps 的默认值在 setup 外被提升，无法引用 t()，
// 因此这里在模板渲染时按需回退到词典文案。
const placeholderText = computed(() => props.placeholder ?? t('team.consumer.selectDepartment'))
const clearLabelText = computed(() => props.clearLabel ?? t('team.consumer.noParentDepartment'))
const emptyTextValue = computed(() => props.emptyText ?? t('team.consumer.noDepartments'))

const emit = defineEmits<{
  'update:modelValue': [value: number | null]
}>()

const selectRef = ref<HTMLElement | null>(null)
const isOpen = ref(false)
const expanded = ref<Set<string>>(new Set())

interface FlatNode {
  id: number
  name: string
  depth: number
  flatId: string
  hasChildren: boolean
}

const flattened = computed<FlatNode[]>(() => {
  const result: FlatNode[] = []
  const walk = (nodes: DepartmentTreeNode[], depth: number, parentPath: string) => {
    for (const node of nodes) {
      const flatId = parentPath ? `${parentPath}-${node.id}` : `${node.id}`
      result.push({
        id: node.id,
        name: node.name,
        depth,
        flatId,
        hasChildren: !!(node.children && node.children.length > 0),
      })
      if (node.children && node.children.length > 0 && expanded.value.has(flatId)) {
        walk(node.children, depth + 1, flatId)
      }
    }
  }
  walk(props.departments, 0, '')
  return result
})

// Auto-expand all nodes with children on initial load / data change
watch(
  () => props.departments,
  (depts) => {
    const next = new Set<string>()
    const walk = (nodes: DepartmentTreeNode[], parentPath: string) => {
      for (const node of nodes) {
        const flatId = parentPath ? `${parentPath}-${node.id}` : `${node.id}`
        if (node.children && node.children.length > 0) {
          next.add(flatId)
          walk(node.children, flatId)
        }
      }
    }
    walk(depts, '')
    expanded.value = next
  },
  { immediate: true }
)

function toggleExpand(flatId: string) {
  const next = new Set(expanded.value)
  if (next.has(flatId)) {
    next.delete(flatId)
  } else {
    next.add(flatId)
  }
  expanded.value = next
}

function select(id: number | null) {
  emit('update:modelValue', id)
  isOpen.value = false
}

const selectedName = computed(() => {
  if (props.modelValue === '' || props.modelValue === null) return ''
  const node = flattened.value.find((n) => n.id === props.modelValue)
  return node ? node.name : ''
})

function handleClickOutside(e: MouseEvent) {
  if (selectRef.value && !selectRef.value.contains(e.target as Node)) {
    isOpen.value = false
  }
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', handleClickOutside)
})
</script>
