import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import MediaTasksView from '../MediaTasksView.vue'

const { listMediaTasks, cancelMediaTask } = vi.hoisted(() => ({
  listMediaTasks: vi.fn(),
  cancelMediaTask: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    mediaTasks: {
      list: listMediaTasks,
      cancel: cancelMediaTask
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn()
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const listResponse = {
  items: [
    {
      id: 1,
      local_id: 'img_abc',
      media_kind: 'image',
      user_id: 2,
      api_key_id: 35,
      public_model: 'qwen-image-3.0',
      upstream_model: 'qwen-image-3.0',
      account_id: 35,
      upstream_task_id: '',
      status: 'succeeded',
      resolution: '1122x1402',
      duration_sec: 0,
      media_url: '',
      thumbnail_url: '',
      error_message: '',
      cost_usd: 0,
      created_at: '2026-09-18T02:00:00Z',
      updated_at: '2026-09-18T02:00:00Z',
      finished_at: ''
    }
  ],
  total: 1,
  page: 1,
  page_size: 20,
  pages: 1
}

function mountView() {
  return mount(MediaTasksView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TablePageLayout: {
          template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
        },
        DataTable: {
          props: ['data'],
          template: '<div><div v-for="row in data" :key="row.id" class="row">{{ row.local_id }}</div></div>'
        },
        Pagination: true,
        ConfirmDialog: true,
        Select: true,
        Icon: true
      }
    }
  })
}

describe('admin MediaTasksView 首次加载', () => {
  beforeEach(() => {
    listMediaTasks.mockReset()
    listMediaTasks.mockResolvedValue(listResponse)
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  // 此前该页面只在点刷新或改筛选时才请求，从左侧菜单进来是一张空表，
  // 必须手动点一次刷新才出数据。
  it('挂载即拉取任务列表，无需手动刷新', async () => {
    const wrapper = mountView()
    expect(listMediaTasks).toHaveBeenCalledTimes(1)

    await flushPromises()

    expect(wrapper.text()).toContain('img_abc')
    const params = listMediaTasks.mock.calls[0][0]
    expect(params.page).toBe(1)
    expect(params.user_id).toBeUndefined()
    expect(params.status).toBeUndefined()
  })

  it('带 AbortSignal 请求，便于组件卸载时取消在途请求', async () => {
    mountView()
    await flushPromises()

    const params = listMediaTasks.mock.calls[0][0]
    expect(params.signal).toBeInstanceOf(AbortSignal)
  })
})
