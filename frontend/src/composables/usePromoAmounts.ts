import { computed } from 'vue'
import { useAppStore } from '@/stores'
import { formatPromoAmount, formatPromoTokens } from '@/utils/promo'

/**
 * 营销文案里的「免费额度 / 邀请奖励」金额，统一取自公开设置。
 *
 * - new_user_credits  ← 后台 default_balance（新用户注册赠送）
 * - invite_register_reward ← 后台 affiliate_register_reward（邀请注册奖励）
 *
 * 值为 0（未配置 / 不发放）时对应文案应当隐藏，因此这里同时给出布尔判断用的数值。
 */
export function usePromoAmounts() {
  const appStore = useAppStore()

  const newUserCredits = computed(() => {
    const value = Number(appStore.cachedPublicSettings?.new_user_credits ?? 0)
    return Number.isFinite(value) && value > 0 ? value : 0
  })

  const inviteReward = computed(() => {
    // 邀请返利总开关关闭时不展示邀请奖励文案。
    if (appStore.cachedPublicSettings?.affiliate_enabled === false) return 0
    const value = Number(appStore.cachedPublicSettings?.invite_register_reward ?? 0)
    return Number.isFinite(value) && value > 0 ? value : 0
  })

  const newUserAmount = computed(() => formatPromoAmount(newUserCredits.value))
  const newUserTokens = computed(() => formatPromoTokens(newUserCredits.value))
  const inviteAmount = computed(() => formatPromoAmount(inviteReward.value))

  return { newUserCredits, inviteReward, newUserAmount, newUserTokens, inviteAmount }
}
