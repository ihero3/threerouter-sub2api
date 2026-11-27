import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import PromoBanner from '../PromoBanner.vue'

type PromoProfile = { promo?: { enabled?: boolean; badge?: string; title?: string; cta?: string; items?: string[] } }

const { appStore } = vi.hoisted(() => ({
  appStore: {
    cachedPublicSettings: {} as Record<string, unknown>,
    activeSiteProfile: undefined as PromoProfile | undefined,
  },
}))

vi.mock('@/stores', () => ({ useAppStore: () => appStore }))

vi.mock('vue-router', () => ({ useRouter: () => ({ push: vi.fn() }) }))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    // 把插值参数渲染出来，便于断言动态金额。
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        params ? `${key}|${JSON.stringify(params)}` : key,
    }),
  }
})

function mountBanner() {
  return mount(PromoBanner)
}

describe('PromoBanner', () => {
  beforeEach(() => {
    appStore.cachedPublicSettings = {}
    appStore.activeSiteProfile = undefined
  })

  it('renders the new-user credit and invite reward from public settings', () => {
    appStore.cachedPublicSettings = {
      new_user_credits: 6,
      invite_register_reward: 1,
      affiliate_enabled: true,
    }

    const text = mountBanner().text()

    expect(text).toContain('promo.items.newUser|{"amount":"$6"}')
    expect(text).toContain('promo.items.refer|{"amount":"$1"}')
  })

  it('follows the configured amounts instead of hardcoded copy', () => {
    // 后台把新用户赠送改成 $3、邀请奖励改成 $2.5 时，文案必须跟着变。
    appStore.cachedPublicSettings = {
      new_user_credits: 3,
      invite_register_reward: 2.5,
      affiliate_enabled: true,
    }

    const text = mountBanner().text()

    expect(text).toContain('promo.items.newUser|{"amount":"$3"}')
    expect(text).toContain('promo.items.refer|{"amount":"$2.50"}')
  })

  it('hides reward lines that are not configured', () => {
    appStore.cachedPublicSettings = {
      new_user_credits: 0,
      invite_register_reward: 0,
      affiliate_enabled: true,
    }

    const text = mountBanner().text()

    expect(text).not.toContain('promo.items.newUser')
    expect(text).not.toContain('promo.items.refer')
    // 其余活动文案照常展示
    expect(text).toContain('promo.items.topUp10')
  })

  it('hides the invite reward line when the affiliate program is disabled', () => {
    appStore.cachedPublicSettings = {
      new_user_credits: 6,
      invite_register_reward: 1,
      affiliate_enabled: false,
    }

    const text = mountBanner().text()

    expect(text).toContain('promo.items.newUser')
    expect(text).not.toContain('promo.items.refer')
  })

  it('prefers the admin-configured site promo copy when present', () => {
    appStore.activeSiteProfile = { promo: { items: ['自定义活动：送 $9'] } }

    const text = mountBanner().text()

    expect(text).toContain('自定义活动：送 $9')
    expect(text).not.toContain('promo.items.newUser')
  })
})
