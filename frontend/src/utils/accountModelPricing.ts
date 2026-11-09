import type { ChannelModelPricing } from '@/api/admin/channels'
import type { PricingFormEntry } from '@/components/admin/channel/types'
import { apiIntervalsToForm, formIntervalsToAPI, apiTimePricingToForm, formTimePricingToAPI,
  createDefaultTimePricingForm, formReasoningEffortMultipliersToAPI, mTokToPerToken, perTokenToMTok } from '@/components/admin/channel/types'

export function emptyAccountPricing(platform: string): PricingFormEntry {
  return { platform, models: [], billing_mode: 'token', input_price: null, output_price: null,
    cache_write_price: null, cache_read_price: null, image_input_price: null, image_output_price: null,
    per_request_price: null, intervals: [], time_pricing: createDefaultTimePricingForm() }
}
export function accountPricingFromAPI(entries: ChannelModelPricing[]): PricingFormEntry[] {
  return entries.map(entry => ({ ...entry,
    input_price: perTokenToMTok(entry.input_price), output_price: perTokenToMTok(entry.output_price),
    cache_write_price: perTokenToMTok(entry.cache_write_price), cache_write_1h_price: perTokenToMTok(entry.cache_write_1h_price),
    cache_read_price: perTokenToMTok(entry.cache_read_price), image_input_price: perTokenToMTok(entry.image_input_price),
    image_output_price: perTokenToMTok(entry.image_output_price),
    intervals: apiIntervalsToForm(entry.intervals || []), time_pricing: apiTimePricingToForm(entry.time_pricing) }))
}
export function accountPricingToAPI(entries: PricingFormEntry[], platform: string): ChannelModelPricing[] {
  return entries.filter(entry => entry.models.length > 0).map(entry => ({ ...entry, platform,
    input_price: mTokToPerToken(entry.input_price), output_price: mTokToPerToken(entry.output_price),
    cache_write_price: mTokToPerToken(entry.cache_write_price), cache_write_1h_price: mTokToPerToken(entry.cache_write_1h_price),
    cache_read_price: mTokToPerToken(entry.cache_read_price), image_input_price: mTokToPerToken(entry.image_input_price),
    image_output_price: mTokToPerToken(entry.image_output_price), fast_multiplier: numeric(entry.fast_multiplier),
    flex_multiplier: numeric(entry.flex_multiplier), per_request_price: numeric(entry.per_request_price),
    reasoning_effort_multipliers: formReasoningEffortMultipliersToAPI(entry.reasoning_effort_multipliers),
    intervals: formIntervalsToAPI(entry.intervals), time_pricing: formTimePricingToAPI(entry.time_pricing) }))
}
function numeric(value: number | string | null | undefined): number | null {
  return value === null || value === undefined || value === '' ? null : Number(value)
}
