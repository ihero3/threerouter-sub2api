/**
 * 自建注册验证码 API（图片点选 / 拼图滑块，单端点判别联合）
 */
import { apiClient } from './client'
import type {
  ClickCaptchaVerifyResponse,
  ClickPoint,
  RegistrationCaptchaChallenge,
  TrajectoryPoint,
} from '@/types'

// 客户端回传的原始输入：按题目模式三选一携带点选坐标 / 滑块落点 / 旋转角度+轨迹。
export interface CaptchaVerifyProof {
  clicks?: ClickPoint[]
  drop_x?: number
  angle?: number
  trajectory?: TrajectoryPoint[]
}

export async function createRegistrationCaptchaChallenge(): Promise<RegistrationCaptchaChallenge> {
  const { data } = await apiClient.post<RegistrationCaptchaChallenge>('/auth/captcha/challenge')
  return data
}

export async function verifyRegistrationCaptcha(
  challengeId: string,
  proof: CaptchaVerifyProof,
): Promise<ClickCaptchaVerifyResponse> {
  const { data } = await apiClient.post<ClickCaptchaVerifyResponse>('/auth/captcha/verify', {
    challenge_id: challengeId,
    ...proof,
  })
  return data
}
