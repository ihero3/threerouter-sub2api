import { describe, expect, it, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

import IpAccountRanking from '../IpAccountRanking.vue'

const listIPAccountRanking = vi.fn()

vi.mock('@/api/admin/usage', () => ({
  listIPAccountRanking: (...args: unknown[]) => listIPAccountRanking(...args),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

const page1 = {
  items: [
    { ip_address: '1.1.1.1', account_count: 9 },
    { ip_address: '2.2.2.2', account_count: 4 },
  ],
  total: 25,
  page: 1,
  page_size: 20,
  pages: 2,
}

const mountRanking = (props: Record<string, unknown> = {}) =>
  mount(IpAccountRanking, {
    props: {
      startDate: '2026-10-20',
      endDate: '2026-10-26',
      ...props,
    },
    global: { stubs: { Pagination: true, LoadingSpinner: true } },
  })

describe('IpAccountRanking', () => {
  beforeEach(() => {
    listIPAccountRanking.mockReset()
    listIPAccountRanking.mockResolvedValue(page1)
  })

  it('loads on mount with the shared date range and emits select-ip on row click', async () => {
    const wrapper = mountRanking()
    await flushPromises()

    expect(listIPAccountRanking).toHaveBeenCalledWith({
      start_date: '2026-10-20',
      end_date: '2026-10-26',
      page: 1,
      page_size: 20,
    })

    const rows = wrapper.findAll('tbody tr')
    expect(rows).toHaveLength(2)
    // 只暴露 IP 与账号数两列
    expect(rows[0].text()).toContain('1.1.1.1')
    expect(rows[0].text()).toContain('9')

    await rows[0].trigger('click')
    expect(wrapper.emitted('select-ip')![0]).toEqual(['1.1.1.1'])
  })

  it('reloads and returns to page 1 when the date range changes', async () => {
    const wrapper = mountRanking()
    await flushPromises()
    expect(listIPAccountRanking).toHaveBeenCalledTimes(1)

    await wrapper.setProps({ startDate: '2026-10-24', endDate: '2026-10-26' })
    await flushPromises()

    expect(listIPAccountRanking).toHaveBeenCalledTimes(2)
    expect(listIPAccountRanking).toHaveBeenLastCalledWith(
      expect.objectContaining({ start_date: '2026-10-24', end_date: '2026-10-26', page: 1 })
    )
  })

  it('flags a clamped window when the range is longer than 7 days', async () => {
    const wrapper = mountRanking({ startDate: '2026-09-01', endDate: '2026-10-26' })
    await flushPromises()

    expect(wrapper.text()).toContain('admin.usage.ipRanking.windowClamped')
  })

  it('does not flag a window of exactly 7 calendar days', async () => {
    const wrapper = mountRanking({ startDate: '2026-10-20', endDate: '2026-10-26' })
    await flushPromises()

    expect(wrapper.text()).not.toContain('admin.usage.ipRanking.windowClamped')
  })
})
