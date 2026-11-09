<template>
  <section class="space-y-3 rounded-lg border border-gray-200 p-4 dark:border-dark-700">
    <label class="flex items-center gap-2 font-medium">
      <input type="checkbox" :checked="!!modelValue" @change="toggle(($event.target as HTMLInputElement).checked)" />
      {{ t('subscriptionBudget.title') }}
    </label>
    <p class="input-hint">{{ t(modelValue ? 'subscriptionBudget.hint' : 'subscriptionBudget.legacy') }}</p>
    <div v-if="modelValue" class="grid gap-3 sm:grid-cols-2">
      <label class="input-label">{{ t('subscriptionBudget.amount') }}
        <input :value="modelValue.amount_usd" type="number" required min="0.000001" step="any" class="input" @input="patch({ amount_usd: Number(($event.target as HTMLInputElement).value) })" />
      </label>
      <label class="input-label">{{ t('subscriptionBudget.period') }}
        <Select :model-value="modelValue.reset_period" :options="periodOptions"
          @update:model-value="patch({ reset_period: $event as SubscriptionBudget['reset_period'] })" />
      </label>
      <label v-if="modelValue.reset_period === 'hours'" class="input-label">{{ t('subscriptionBudget.hoursLabel') }}
        <input :value="modelValue.reset_hours || 6" type="number" min="1" max="876000" required class="input" @input="patch({ reset_hours: Number(($event.target as HTMLInputElement).value) })" />
      </label>
      <label v-if="modelValue.reset_period === 'custom'" class="input-label">{{ t('subscriptionBudget.daysLabel') }}
        <input :value="modelValue.reset_days || 1" type="number" min="1" max="36500" required class="input" @input="patch({ reset_days: Number(($event.target as HTMLInputElement).value) })" />
      </label>
      <template v-if="modelValue.reset_period !== 'hours' && modelValue.reset_period !== 'none'">
        <label class="input-label">{{ t('subscriptionBudget.time') }}
          <input :value="modelValue.reset_time" type="time" required class="input" @input="patch({ reset_time: ($event.target as HTMLInputElement).value })" />
        </label>
        <label class="input-label">{{ t('subscriptionBudget.timezone') }}
          <input :value="modelValue.timezone" required class="input" list="budget-timezones" @input="patch({ timezone: ($event.target as HTMLInputElement).value })" />
          <datalist id="budget-timezones"><option v-for="zone in COMMON_TIMEZONES" :key="zone" :value="zone" /></datalist>
        </label>
      </template>
      <p class="input-hint sm:col-span-2">{{ t(`subscriptionBudget.${modelValue.reset_period}Hint`) }}</p>
    </div>
  </section>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import Select from '@/components/common/Select.vue'
import { useI18n } from 'vue-i18n'
import type { SubscriptionBudget } from '@/types'
import { COMMON_TIMEZONES } from '@/components/admin/channel/types'
import { defaultSubscriptionBudget } from '@/utils/subscriptionBudget'
const props = defineProps<{ modelValue: SubscriptionBudget | null }>()
const emit = defineEmits<{ 'update:modelValue': [value: SubscriptionBudget | null] }>()
const { t } = useI18n()
const periods = ['hours', 'daily', 'weekly', 'monthly', 'custom', 'none'] as const
const periodOptions = computed(() => periods.map(value => ({ value, label: t(`subscriptionBudget.${value}`) })))
function toggle(enabled: boolean) { emit('update:modelValue', enabled ? defaultSubscriptionBudget() : null) }
function patch(value: Partial<SubscriptionBudget>) {
  if (props.modelValue) emit('update:modelValue', { reset_hours: 6, reset_days: 1, ...props.modelValue, ...value })
}
</script>
