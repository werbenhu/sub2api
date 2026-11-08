import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import EmailOAuthButtons from '@/components/auth/EmailOAuthButtons.vue'

const routeState = vi.hoisted(() => ({ query: {} as Record<string, unknown> }))
vi.mock('vue-router', () => ({ useRoute: () => routeState }))
vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, string>) => key === 'auth.emailOAuth.signIn'
      ? `使用 ${params?.providerName ?? ''} 登录`
      : key,
  }),
}))

describe('EmailOAuthButtons', () => {
  beforeEach(() => {
    routeState.query = { redirect: '/billing?plan=pro' }
    window.sessionStorage.clear()
  })

  it.each(['github', 'google'] as const)('emits %s with only the redirect parameter', async (provider) => {
    const wrapper = mount(EmailOAuthButtons, {
      props: { githubEnabled: provider === 'github', googleEnabled: provider === 'google' },
      global: { stubs: { GitHubMark: true, GoogleMark: true } },
    })

    await wrapper.get('button').trigger('click')

    expect(wrapper.emitted('start')?.[0]?.[0]).toEqual({
      provider,
      params: { redirect: '/billing?plan=pro' },
    })
    expect(window.sessionStorage.getItem('email_oauth_pending_provider')).toBe(provider)
  })

  it('uses a full-width descriptive button when only GitHub is enabled', () => {
    const wrapper = mount(EmailOAuthButtons, {
      props: { githubEnabled: true, googleEnabled: false },
      global: { stubs: { GitHubMark: true, GoogleMark: true } },
    })
    expect(wrapper.find('.grid').classes()).not.toContain('sm:grid-cols-2')
    expect(wrapper.get('button').text()).toContain('使用 GitHub 登录')
  })

  it('uses compact labels and two columns when both providers are enabled', () => {
    const wrapper = mount(EmailOAuthButtons, {
      props: { githubEnabled: true, googleEnabled: true },
      global: { stubs: { GitHubMark: true, GoogleMark: true } },
    })
    expect(wrapper.find('.grid').classes()).toContain('sm:grid-cols-2')
  })
})
