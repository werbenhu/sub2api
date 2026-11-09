<template>
  <section class="space-y-3 rounded-lg border border-gray-200 p-4 dark:border-dark-700">
    <div class="flex items-center justify-between gap-3">
      <h3 class="font-medium">{{ t('accountPricing.title') }}</h3>
      <button type="button" class="btn btn-secondary btn-sm" @click="add">{{ t('accountPricing.add') }}</button>
    </div>
    <p class="input-hint">{{ t('accountPricing.hint') }}</p>
    <PricingEntryCard v-for="(entry, index) in entries" :key="index" :entry="entry" :platform="platform"
      @update="update(index, $event)" @remove="remove(index)" />
  </section>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ChannelModelPricing } from '@/api/admin/channels'
import PricingEntryCard from '@/components/admin/channel/PricingEntryCard.vue'
import type { PricingFormEntry } from '@/components/admin/channel/types'
import { accountPricingFromAPI, accountPricingToAPI, emptyAccountPricing } from '@/utils/accountModelPricing'

const props = defineProps<{ modelValue: ChannelModelPricing[]; platform: string }>()
const emit = defineEmits<{ 'update:modelValue': [value: ChannelModelPricing[]] }>()
const { t } = useI18n()
const entries = ref<PricingFormEntry[]>([])
let lastEmitted: ChannelModelPricing[] | undefined
watch(() => props.modelValue, value => {
  if (value !== lastEmitted) entries.value = accountPricingFromAPI(value)
}, { immediate: true })
watch(() => props.platform, platform => { entries.value = entries.value.map(entry => ({ ...entry, platform })); publish() })
function publish() { lastEmitted = accountPricingToAPI(entries.value, props.platform); emit('update:modelValue', lastEmitted) }
function add() { entries.value.push(emptyAccountPricing(props.platform)) }
function update(index: number, entry: PricingFormEntry) { entries.value[index] = entry; publish() }
function remove(index: number) { entries.value.splice(index, 1); publish() }
</script>
