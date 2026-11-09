import type { SubscriptionBudget } from '@/types'
export function defaultSubscriptionBudget(): SubscriptionBudget {
  return { amount_usd: 10, reset_period: 'daily', reset_hours: 6, reset_days: 1, reset_time: '00:00', timezone: 'Asia/Shanghai' }
}
