import { beforeEach, describe, expect, it, vi } from 'vitest'

const post = vi.hoisted(() => vi.fn())

vi.mock('@/api/client', () => ({
  apiClient: { post }
}))

import { register } from '@/api/auth'

const credentials = { email: 'user@example.com', password: 'secret-123' }

describe('register() response integrity', () => {
  beforeEach(() => {
    post.mockReset()
    localStorage.clear()
  })

  // 后端蜜罐命中时返回 200/code=0 的「验证码已发送」响应体（未建号、未签发 token）。
  // 旧实现会把 undefined 写进 localStorage 并让前端显示「注册成功」。
  it('rejects a honeypot fake-success response without writing auth state', async () => {
    post.mockResolvedValue({
      data: { message: 'Verification code sent successfully', countdown: 60 }
    })

    await expect(register(credentials)).rejects.toMatchObject({
      reason: 'REGISTRATION_RESPONSE_INVALID'
    })
    expect(localStorage.getItem('auth_token')).toBeNull()
    expect(localStorage.getItem('auth_user')).toBeNull()
  })

  it('rejects a response with an empty access token', async () => {
    post.mockResolvedValue({
      data: { access_token: '', user: { id: 1 } }
    })

    await expect(register(credentials)).rejects.toMatchObject({
      reason: 'REGISTRATION_RESPONSE_INVALID'
    })
    expect(localStorage.getItem('auth_token')).toBeNull()
  })

  it('stores tokens for a genuine registration response', async () => {
    const payload = {
      access_token: 'jwt-token',
      refresh_token: 'refresh-token',
      expires_in: 3600,
      token_type: 'Bearer',
      user: { id: 1, email: 'user@example.com' }
    }
    post.mockResolvedValue({ data: payload })

    await expect(register(credentials)).resolves.toEqual(payload)
    expect(localStorage.getItem('auth_token')).toBe('jwt-token')
    expect(localStorage.getItem('refresh_token')).toBe('refresh-token')
  })
})
