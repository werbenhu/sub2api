import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import SubscriptionBudgetEditor from '../SubscriptionBudgetEditor.vue'
import { defaultSubscriptionBudget } from '@/utils/subscriptionBudget'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
describe('SubscriptionBudgetEditor', () => {
  it('keeps a custom hourly interval when changing the amount', async () => {
    const budget = { ...defaultSubscriptionBudget(), reset_period: 'hours' as const, reset_hours: 12 }
    const wrapper = mount(SubscriptionBudgetEditor, { props: { modelValue: budget } })
    await wrapper.find('input[type="number"]').setValue('25')
    expect(wrapper.emitted('update:modelValue')?.[0]?.[0]).toMatchObject({ amount_usd: 25, reset_hours: 12 })
    expect(wrapper.find('input[type="time"]').exists()).toBe(false)
  })
  it('keeps calendar reset times separate from the subscription expiry', () => {
    const wrapper = mount(SubscriptionBudgetEditor, { props: { modelValue: defaultSubscriptionBudget() } })
    expect(wrapper.find('input[type="time"]').element.value).toBe('00:00')
    expect(wrapper.find('input[type="date"]').exists()).toBe(false)
  })
})
