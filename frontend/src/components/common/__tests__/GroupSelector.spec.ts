import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import GroupSelector from '../GroupSelector.vue'

const authState = { isSimpleMode: false }

vi.mock('@/stores', () => ({ useAuthStore: () => authState }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const groups = [
  { id: 1, name: 'Basic', platform: 'anthropic', status: 'active' },
  { id: 2, name: 'Composite', platform: 'composite', status: 'active' }
] as any

const mountSelector = (modelValue: number[] = []) => mount(GroupSelector, {
  props: { modelValue, groups },
  global: { stubs: { GroupBadge: { props: ['name'], template: '<span>{{ name }}</span>' }, Icon: true } }
})

describe('GroupSelector simple-mode binding policy', () => {
  beforeEach(() => { authState.isSimpleMode = false })

  it('includes composite groups in simple mode', () => {
    authState.isSimpleMode = true
    const wrapper = mountSelector()
    expect(wrapper.text()).toContain('Basic')
    expect(wrapper.text()).toContain('Composite')
  })

  it('keeps composite groups available in advanced mode', () => {
    const wrapper = mountSelector()
    expect(wrapper.text()).toContain('Composite')
  })

  it('only offers composite groups that allow the account platform', () => {
    const wrapper = mount(GroupSelector, {
      props: {
        modelValue: [], platform: 'gemini', groups: [
          { ...groups[1], id: 3, name: 'Restricted', platforms: ['openai', 'grok'] },
          { ...groups[1], id: 4, name: 'Allowed', platforms: ['openai', 'gemini'] },
          { ...groups[1], id: 5, name: 'Legacy', platforms: [] },
        ],
      },
      global: { stubs: { GroupBadge: { props: ['name'], template: '<span>{{ name }}</span>' }, Icon: true } },
    })
    expect(wrapper.text()).not.toContain('Restricted')
    expect(wrapper.text()).toContain('Allowed')
    expect(wrapper.text()).toContain('Legacy')
  })

  it('preserves existing composite selections in simple mode', () => {
    authState.isSimpleMode = true
    const wrapper = mountSelector([1, 2])
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })
})
