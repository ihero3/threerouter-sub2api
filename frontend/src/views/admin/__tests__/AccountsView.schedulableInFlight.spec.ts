import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AccountsView from '../AccountsView.vue'

const { listAccounts, listWithEtag, getBatchTodayStats, getAllProxies, getAllGroups, setSchedulable, showInfo } =
  vi.hoisted(() => ({
    listAccounts: vi.fn(),
    listWithEtag: vi.fn(),
    getBatchTodayStats: vi.fn(),
    getAllProxies: vi.fn(),
    getAllGroups: vi.fn(),
    setSchedulable: vi.fn(),
    showInfo: vi.fn()
  }))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      list: listAccounts,
      listWithEtag,
      getBatchTodayStats,
      getUpstreamBillingProbeSettings: vi.fn().mockResolvedValue({ enabled: true, interval_minutes: 30 }),
      delete: vi.fn(),
      batchClearError: vi.fn(),
      batchRefresh: vi.fn(),
      toggleSchedulable: vi.fn(),
      setSchedulable
    },
    proxies: { getAll: getAllProxies },
    groups: { getAll: getAllGroups }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
    showInfo
  })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ token: 'test-token' })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        params?.count === undefined ? key : `${key}:${params.count}`
    })
  }
})

// 渲染 schedulable 单元格，才能点到那一行的开关。
const DataTableStub = {
  props: ['columns', 'data'],
  template: `
    <div data-test="data-table">
      <div v-for="row in data" :key="row.id">
        <slot name="cell-schedulable" :row="row" />
      </div>
    </div>
  `
}

function mountView() {
  return mount(AccountsView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TablePageLayout: {
          template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
        },
        DataTable: DataTableStub,
        HelpTooltip: true,
        Pagination: true,
        ConfirmDialog: true,
        AccountTableActions: { template: '<div><slot name="beforeCreate" /><slot name="after" /></div>' },
        AccountTableFilters: { template: '<div></div>' },
        AccountBulkActionsBar: true,
        AccountActionMenu: true,
        ImportDataModal: true,
        ReAuthAccountModal: true,
        AccountTestModal: true,
        AccountStatsModal: true,
        ScheduledTestsPanel: true,
        SyncFromCrsModal: true,
        TempUnschedStatusModal: true,
        ErrorPassthroughRulesModal: true,
        TLSFingerprintProfilesModal: true,
        CreateAccountModal: true,
        EditAccountModal: true,
        BulkEditAccountModal: true,
        PlatformTypeBadge: true,
        AccountCapacityCell: true,
        AccountStatusIndicator: true,
        AccountTodayStatsCell: true,
        AccountGroupsCell: true,
        AccountUsageCell: true,
        Icon: true
      }
    }
  })
}

const baseAccount = {
  platform: 'openai',
  type: 'apikey',
  status: 'active',
  schedulable: true,
  concurrency: 1,
  priority: 0,
  error_message: null,
  last_used_at: null,
  expires_at: null,
  auto_pause_on_expired: false,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z'
}

describe('admin AccountsView 关闭调度时的在途请求提示', () => {
  beforeEach(() => {
    localStorage.clear()
    listAccounts.mockReset()
    listWithEtag.mockReset()
    getBatchTodayStats.mockReset()
    getAllProxies.mockReset()
    getAllGroups.mockReset()
    setSchedulable.mockReset()
    showInfo.mockReset()

    listAccounts.mockResolvedValue({ items: [{ ...baseAccount, id: 7, name: 'acct-in-flight' }] })
    listWithEtag.mockResolvedValue({ notModified: true, items: [] })
    getBatchTodayStats.mockResolvedValue({ stats: {} })
    getAllProxies.mockResolvedValue([])
    getAllGroups.mockResolvedValue([])
  })

  async function toggleOff(wrapper: ReturnType<typeof mountView>) {
    const button = wrapper.find('[title="admin.accounts.schedulableEnabled"]')
    expect(button.exists()).toBe(true)
    await button.trigger('click')
    await flushPromises()
  }

  // 关闭调度只停止把新请求路由到该账号，后端不会中断在途请求。运维需要看到
  // 这句话，否则会以为正在生成的图片/视频被打断了。
  it('有关键在途请求时提示"会先跑完"', async () => {
    setSchedulable.mockResolvedValue({ ...baseAccount, id: 7, schedulable: false, current_concurrency: 3 })
    const wrapper = mountView()
    await flushPromises()

    await toggleOff(wrapper)

    expect(setSchedulable).toHaveBeenCalledWith(7, false)
    expect(showInfo).toHaveBeenCalledTimes(1)
    expect(showInfo.mock.calls[0][0]).toBe('admin.accounts.schedulableDisabledInFlight:3')
  })

  it('账号当前没有在途请求时不打扰用户', async () => {
    setSchedulable.mockResolvedValue({ ...baseAccount, id: 7, schedulable: false, current_concurrency: 0 })
    const wrapper = mountView()
    await flushPromises()

    await toggleOff(wrapper)

    expect(setSchedulable).toHaveBeenCalledWith(7, false)
    expect(showInfo).not.toHaveBeenCalled()
  })

  it('打开调度时不做任何额外提示', async () => {
    setSchedulable.mockResolvedValue({ ...baseAccount, id: 7, schedulable: true, current_concurrency: 2 })
    listAccounts.mockResolvedValue({
      items: [{ ...baseAccount, id: 7, name: 'acct-off', schedulable: false, current_concurrency: 2 }]
    })
    const wrapper = mountView()
    await flushPromises()

    const button = wrapper.find('[title="admin.accounts.schedulableDisabled"]')
    expect(button.exists()).toBe(true)
    await button.trigger('click')
    await flushPromises()

    expect(setSchedulable).toHaveBeenCalledWith(7, true)
    expect(showInfo).not.toHaveBeenCalled()
  })
})
