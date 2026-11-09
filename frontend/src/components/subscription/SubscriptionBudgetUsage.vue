<template>
  <div v-if="subscription.budget" class="space-y-2 text-sm">
    <div class="flex justify-between gap-3">
      <span>{{ t('subscriptionBudget.title') }}</span>
      <span>${{ (subscription.budget_usage_usd || 0).toFixed(4) }} / ${{ subscription.budget.amount_usd.toFixed(2) }}</span>
    </div>
    <div class="h-1.5 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
      <div class="h-full bg-primary-500" :style="{ width: `${Math.min(100, (subscription.budget_usage_usd || 0) / subscription.budget.amount_usd * 100)}%` }" />
    </div>
    <div>{{ t('subscriptionBudget.remaining', { amount: (subscription.budget_remaining_usd ?? 0).toFixed(4) }) }}</div>
    <div v-if="subscription.budget_reset_at" class="text-xs text-gray-500">{{ t('subscriptionBudget.nextReset', { time: new Date(subscription.budget_reset_at).toLocaleString(locale) }) }}</div>
  </div>
</template>
<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { UserSubscription } from '@/types'
defineProps<{ subscription: UserSubscription }>()
const { t, locale } = useI18n()
</script>
