import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import LoginView from '../LoginView.vue'

const { route, login, push, replace, exchangeCompletion } = vi.hoisted(() => ({
  route: { path: '/login', query: {} as Record<string, unknown> },
  login: vi.fn(), push: vi.fn(), replace: vi.fn(), exchangeCompletion: vi.fn()
}))
vi.mock('vue-router', () => ({ useRoute: () => route, useRouter: () => ({ push, replace, currentRoute: { value: route } }) }))
vi.mock('vue-i18n', () => ({
  createI18n: () => ({ global: { t: (key: string) => key } }),
  useI18n: () => ({ t: (key: string) => key, locale: { value: 'en' } })
}))
vi.mock('@/stores', () => ({
  useAuthStore: () => ({ login, loginWithPasskey: vi.fn(), login2FA: vi.fn(), setToken: vi.fn() }),
  useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn(), showWarning: vi.fn(), cachedPublicSettings: {}, publicSettingsLoaded: true })
}))
vi.mock('@/api/client', () => ({ apiClient: { post: vi.fn() } }))
vi.mock('@/api/auth', async () => {
  const actual = await vi.importActual<typeof import('@/api/auth')>('@/api/auth')
  return {
    ...actual,
    getPublicSettings: vi.fn().mockResolvedValue({ registration_enabled: false }),
    isTotp2FARequired: () => false,
    exchangePendingOAuthCompletion: exchangeCompletion,
    persistOAuthTokenContext: vi.fn()
  }
})
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copyToClipboard: vi.fn() }) }))

const authorizationPath = '/oauth/mcp/authorize?' + new URLSearchParams({
  response_type: 'code', client_id: 'desktop', redirect_uri: 'http://127.0.0.1:54321/callback',
  resource: 'https://api.example.com/api/v1/mcp', state: 'opaque+a&b=c',
  code_challenge: 'a'.repeat(43), code_challenge_method: 'S256'
}).toString()
const mounted: ReturnType<typeof mount>[] = []
const originalLocation = window.location
beforeEach(() => {
  vi.clearAllMocks()
  route.path = '/login'
  route.query = { redirect: authorizationPath }
  login.mockResolvedValue({})
  Object.defineProperty(window, 'location', { configurable: true, value: { ...originalLocation, href: 'https://platform.example.com/auth/callback', hash: '' } })
})
afterEach(() => {
  mounted.splice(0).forEach(wrapper => wrapper.unmount())
  Object.defineProperty(window, 'location', { configurable: true, value: originalLocation })
})

describe('MCP authorization through existing sign-in flows', () => {
  it('returns to the original OAuth query after password login', async () => {
    const wrapper = mount(LoginView, {
      global: { stubs: {
        AuthLayout: { template: '<div><slot /><slot name="footer" /></div>' },
        DingTalkOAuthSection: true, EmailOAuthButtons: true, Icon: true, LinuxDoOAuthSection: true,
        LoginAgreementPrompt: true, OidcOAuthSection: true, RouterLink: true, TotpLoginModal: true,
        TurnstileWidget: true, WechatOAuthSection: true
      } }
    })
    mounted.push(wrapper)
    await flushPromises()
    await wrapper.get('#email').setValue('user@example.com')
    await wrapper.get('#password').setValue('local-test-password')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(login).toHaveBeenCalledTimes(1)
    expect(push).toHaveBeenCalledWith(authorizationPath)
  })

})
