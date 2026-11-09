import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import UsageIpUsersView from '../UsageIpUsersView.vue'

const { listUsersByIP, toggleStatus, showSuccess, showError } = vi.hoisted(() => ({
  listUsersByIP: vi.fn(),
  toggleStatus: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api/admin/usage', () => ({
  adminUsageAPI: { listUsersByIP: (...args: unknown[]) => listUsersByIP(...args) },
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: {
      toggleStatus: (...args: unknown[]) => toggleStatus(...args),
      getById: vi.fn(),
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess, showError }),
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: { ip: '1.2.3.4' } }),
  useRouter: () => ({ back: vi.fn(), push: vi.fn() }),
}))

vi.mock('@/composables/usePersistedPageSize', () => ({
  getPersistedPageSize: () => 20,
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const row = (overrides: Record<string, unknown> = {}) => ({
  user_id: 7,
  email: 'u7@test.com',
  username: '',
  created_at: '2026-11-01T10:00:00Z',
  last_used_at: '2026-11-09T10:00:00Z',
  deleted: false,
  recharged: false,
  looked_count: 0,
  ticket_flagged: false,
  balance: 12.345,
  status: 'active',
  role: 'user',
  ...overrides,
})

const mountView = () =>
  mount(UsageIpUsersView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Pagination: true,
        UserBalanceHistoryModal: true,
        UserBalanceModal: true,
      },
    },
  })

describe('UsageIpUsersView', () => {
  beforeEach(() => {
    listUsersByIP.mockReset()
    toggleStatus.mockReset()
    showSuccess.mockReset()
    showError.mockReset()
    listUsersByIP.mockResolvedValue({ items: [row()], total: 1, page: 1, page_size: 20, pages: 1 })
    toggleStatus.mockResolvedValue({ id: 7, status: 'disabled' })
  })

  it('渲染当前余额列（两位小数）', async () => {
    const wrapper = mountView()
    await flushPromises()

    const headers = wrapper.findAll('th').map((th) => th.text())
    expect(headers).toContain('admin.usage.ipUsers.columns.balance')
    expect(wrapper.text()).toContain('12.35')
  })

  it('对普通用户点击「禁止」调用状态切换接口并刷新列表', async () => {
    const wrapper = mountView()
    await flushPromises()

    const button = wrapper
      .findAll('button')
      .find((b) => b.text().includes('admin.users.disable'))
    expect(button).toBeTruthy()

    await button!.trigger('click')
    await flushPromises()

    expect(toggleStatus).toHaveBeenCalledWith(7, 'disabled')
    expect(showSuccess).toHaveBeenCalled()
    // 切换后重新拉列表：首次加载 1 次 + 刷新 1 次
    expect(listUsersByIP).toHaveBeenCalledTimes(2)
  })

  it('已禁用用户显示「启用」，点击后切回 active', async () => {
    listUsersByIP.mockResolvedValue({
      items: [row({ status: 'disabled' })],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    toggleStatus.mockResolvedValue({ id: 7, status: 'active' })

    const wrapper = mountView()
    await flushPromises()

    const button = wrapper
      .findAll('button')
      .find((b) => b.text().includes('admin.users.enable'))
    expect(button).toBeTruthy()

    await button!.trigger('click')
    await flushPromises()

    expect(toggleStatus).toHaveBeenCalledWith(7, 'active')
  })

  it('管理员账号不提供禁止按钮', async () => {
    listUsersByIP.mockResolvedValue({
      items: [row({ user_id: 1, role: 'admin', email: 'admin@test.com' })],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).not.toContain('admin.users.disable')
    expect(wrapper.text()).not.toContain('admin.users.enable')
  })
})
