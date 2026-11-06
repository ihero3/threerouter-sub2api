<template>
  <div
    :class="[
      'flex items-center justify-between rounded-xl border p-4 transition-all duration-200',
      'border-gray-200 bg-white dark:border-dark-600 dark:bg-dark-800',
      'hover:border-gray-300 hover:shadow-sm dark:hover:border-dark-500 dark:hover:shadow-black/20'
    ]"
  >
    <!-- Member Info -->
    <div class="flex items-center gap-3">
      <div
        :class="[
          'flex h-10 w-10 items-center justify-center rounded-full text-sm font-medium',
          member.status === 'active'
            ? 'bg-primary-100 text-primary-700 dark:bg-primary-900/30 dark:text-primary-300'
            : 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-gray-400'
        ]"
      >
        {{ getInitials(member.name) }}
      </div>
      <div>
        <div class="flex items-center gap-2">
          <p
            :class="[
              'font-medium',
              member.status === 'active'
                ? 'text-gray-900 dark:text-white'
                : 'text-gray-500 dark:text-gray-400'
            ]"
          >
            {{ member.name }}
          </p>
          <span
            v-if="member.status === 'inactive'"
            class="rounded px-1.5 py-0.5 text-[10px] font-medium uppercase tracking-wider bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-gray-400"
          >
            {{ t('admin.team.members.status.inactive') }}
          </span>
        </div>
        <p class="text-xs text-gray-500 dark:text-dark-400">{{ member.email }}</p>
      </div>
    </div>

    <!-- Role & Actions -->
    <div class="flex items-center gap-3">
      <RoleBadge :role="member.role" size="sm" />

      <!-- Actions Dropdown -->
      <div class="relative" ref="dropdownRef">
        <button
          @click="toggleDropdown"
          :class="[
            'rounded-lg p-1.5 transition-colors',
            'text-gray-500 hover:bg-gray-100 hover:text-gray-700',
            'dark:text-gray-400 dark:hover:bg-dark-700 dark:hover:text-gray-200'
          ]"
          :aria-expanded="dropdownOpen"
          aria-haspopup="true"
        >
          <Icon name="more" size="sm" />
        </button>

        <Transition name="dropdown">
          <div
            v-if="dropdownOpen"
            class="absolute right-0 z-50 mt-1 w-48 rounded-xl border border-gray-200 bg-white py-1 shadow-lg dark:border-dark-700 dark:bg-dark-800 dark:shadow-black/30"
          >
            <button
              @click="handleEditRole"
              class="flex w-full items-center gap-2 px-4 py-2 text-left text-sm text-gray-700 transition-colors hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-dark-700"
            >
              <Icon name="edit" size="sm" class="text-gray-400" />
              {{ t('admin.team.members.editRole') }}
            </button>
            <button
              @click="handleToggleStatus"
              class="flex w-full items-center gap-2 px-4 py-2 text-left text-sm transition-colors hover:bg-gray-50 dark:hover:bg-dark-700"
              :class="member.status === 'active' ? 'text-amber-600 dark:text-amber-400' : 'text-green-600 dark:text-green-400'"
            >
              <Icon :name="member.status === 'active' ? 'ban' : 'checkCircle'" size="sm" />
              {{ member.status === 'active' ? t('common.disabled') : t('common.enabled') }}
            </button>
            <div class="my-1 border-t border-gray-100 dark:border-dark-700"></div>
            <button
              @click="handleRemove"
              class="flex w-full items-center gap-2 px-4 py-2 text-left text-sm text-red-600 transition-colors hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/20"
            >
              <Icon name="trash" size="sm" />
              {{ t('common.remove') }}
            </button>
          </div>
        </Transition>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import RoleBadge from './RoleBadge.vue'

const { t } = useI18n()

type Role = 'owner' | 'admin' | 'member' | 'viewer'
type Status = 'active' | 'inactive'

export interface Member {
  id: number
  name: string
  email: string
  role: Role
  status: Status
}

interface Props {
  member: Member
}

const props = defineProps<Props>()

interface Emits {
  (e: 'editRole', member: Member): void
  (e: 'toggleStatus', member: Member): void
  (e: 'remove', member: Member): void
}

const emit = defineEmits<Emits>()

const dropdownOpen = ref(false)
const dropdownRef = ref<HTMLElement | null>(null)

const getInitials = (name: string): string => {
  return name
    .split(' ')
    .map((n) => n[0])
    .join('')
    .toUpperCase()
    .slice(0, 2)
}

const toggleDropdown = () => {
  dropdownOpen.value = !dropdownOpen.value
}

const closeDropdown = () => {
  dropdownOpen.value = false
}

const handleEditRole = () => {
  closeDropdown()
  emit('editRole', props.member)
}

const handleToggleStatus = () => {
  closeDropdown()
  emit('toggleStatus', props.member)
}

const handleRemove = () => {
  closeDropdown()
  emit('remove', props.member)
}

const handleClickOutside = (event: MouseEvent) => {
  const target = event.target as HTMLElement
  if (dropdownRef.value && !dropdownRef.value.contains(target)) {
    closeDropdown()
  }
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
})
</script>

<style scoped>
.dropdown-enter-active,
.dropdown-leave-active {
  transition: all 0.15s ease;
}

.dropdown-enter-from,
.dropdown-leave-to {
  opacity: 0;
  transform: translateY(-4px) scale(0.98);
}
</style>
