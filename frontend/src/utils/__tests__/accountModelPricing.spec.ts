import { describe, expect, it } from 'vitest'
import { accountPricingFromAPI, accountPricingToAPI, emptyAccountPricing } from '../accountModelPricing'

describe('account model pricing', () => {
  it('converts UI prices per million tokens without losing other rates', () => {
    const form = emptyAccountPricing('openai')
    Object.assign(form, { models: ['gpt-test'], input_price: '2.5', output_price: 10,
      cache_read_price: 0, per_request_price: null, reasoning_effort_multipliers: { high: '1.5' } })
    const [api] = accountPricingToAPI([form], 'openai')
    expect(api.input_price).toBe(0.0000025)
    expect(api.output_price).toBe(0.00001)
    expect(api.cache_read_price).toBe(0)
    expect(api.reasoning_effort_multipliers).toEqual({ high: 1.5 })
    const [restored] = accountPricingFromAPI([api])
    expect(restored.input_price).toBe(2.5)
    expect(restored.cache_read_price).toBe(0)
    expect(restored.cache_write_price).toBeNull()
  })
  it('removes blank model cards and permits removing all overrides', () => {
    expect(accountPricingToAPI([emptyAccountPricing('openai')], 'openai')).toEqual([])
    expect(accountPricingFromAPI([])).toEqual([])
  })
})
