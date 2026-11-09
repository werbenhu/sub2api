<template>
  <BaseDialog :show="show" :title="t('admin.subscriptions.planManagement')" width="wide" @close="emit('close')">
    <div class="space-y-5">
      <div class="flex justify-end">
        <button type="button" class="btn btn-secondary" @click="startCreate">{{ t('admin.subscriptions.createPlan') }}</button>
      </div>
      <div v-if="loading" class="py-8 text-center text-gray-500">{{ t('common.loading') }}</div>
      <div v-else class="space-y-2">
        <div v-if="plans.length === 0" class="py-6 text-center text-gray-500">{{ t('admin.subscriptions.noPlans') }}</div>
        <div v-for="plan in plans" :key="plan.id" class="flex items-center justify-between gap-3 rounded-lg border border-gray-200 p-3 dark:border-dark-700">
          <div>
            <p class="font-medium text-gray-900 dark:text-white">{{ plan.name }}</p>
            <p class="text-sm text-gray-500">{{ groupName(plan.group_id) }} · {{ plan.validity_days }} {{ t('admin.subscriptions.days') }}</p>
            <p v-if="plan.budget" class="text-sm text-gray-500">${{ plan.budget.amount_usd }} · {{ t(`subscriptionBudget.${plan.budget.reset_period}`) }}</p>
          </div>
          <div class="flex gap-2">
            <button type="button" class="btn btn-secondary btn-sm" @click="startEdit(plan)">{{ t('common.edit') }}</button>
            <button type="button" class="btn btn-danger btn-sm" @click="removePlan(plan)">{{ t('common.delete') }}</button>
          </div>
        </div>
      </div>
      <form v-if="editing" class="space-y-4 rounded-lg border border-gray-200 p-4 dark:border-dark-700" @submit.prevent="savePlan">
        <div class="grid gap-4 sm:grid-cols-2">
          <div>
            <label class="input-label">{{ t('admin.subscriptions.planName') }}</label>
            <input v-model.trim="form.name" required class="input" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.subscriptions.form.group') }}</label>
            <Select v-model="form.group_id" :options="groupOptions" required />
          </div>
          <div>
            <label class="input-label">{{ t('admin.subscriptions.form.validityDays') }}</label>
            <input v-model.number="form.validity_days" type="number" min="1" max="36500" required class="input" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.subscriptions.sortOrder') }}</label>
            <input v-model.number="form.sort_order" type="number" class="input" />
          </div>
        </div>
        <SubscriptionBudgetEditor v-model="form.budget" />
        <textarea v-model.trim="form.description" class="input" rows="2" :placeholder="t('admin.groups.optionalDescription')" />
        <div class="flex justify-end gap-2">
          <button type="button" class="btn btn-secondary" @click="editing = false">{{ t('common.cancel') }}</button>
          <button type="submit" class="btn btn-primary" :disabled="saving">{{ t('common.save') }}</button>
        </div>
      </form>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { SubscriptionPlan } from '@/api/admin/subscriptions'
import type { Group, SubscriptionBudget } from '@/types'
import SubscriptionBudgetEditor from './SubscriptionBudgetEditor.vue'
import { defaultSubscriptionBudget } from '@/utils/subscriptionBudget'
import { useAppStore } from '@/stores'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'

const props = defineProps<{ show: boolean; groups: Group[] }>()
const emit = defineEmits<{ close: []; changed: [] }>()
const { t } = useI18n()
const appStore = useAppStore()
const plans = ref<SubscriptionPlan[]>([])
const loading = ref(false)
const saving = ref(false)
const editing = ref(false)
const editingID = ref<number | null>(null)
const form = reactive({ name: '', description: '', group_id: null as number | null, validity_days: 30, sort_order: 0, budget: null as SubscriptionBudget | null })
const groupOptions = computed(() => props.groups.map(group => ({ value: group.id, label: group.name })))

function groupName(id: number): string {
  return props.groups.find(group => group.id === id)?.name || `#${id}`
}

async function loadPlans() {
  loading.value = true
  try {
    plans.value = await adminAPI.subscriptions.listPlans()
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.subscriptions.failedToLoad'))
  } finally {
    loading.value = false
  }
}

function startCreate() {
  editingID.value = null
  Object.assign(form, { name: '', description: '', group_id: props.groups[0]?.id ?? null, validity_days: 30, sort_order: plans.value.length, budget: defaultSubscriptionBudget() })
  editing.value = true
}

function startEdit(plan: SubscriptionPlan) {
  editingID.value = plan.id
  Object.assign(form, { name: plan.name, description: plan.description, group_id: plan.group_id, validity_days: plan.validity_days, sort_order: plan.sort_order, budget: plan.budget ? { ...plan.budget } : null })
  editing.value = true
}

async function savePlan() {
  if (!form.group_id || saving.value) return
  saving.value = true
  try {
    if (editingID.value) {
      await adminAPI.subscriptions.updatePlan(editingID.value, { ...form, group_id: form.group_id })
    } else {
      await adminAPI.subscriptions.createPlan({ ...form, group_id: form.group_id })
    }
    editing.value = false
    await loadPlans()
    emit('changed')
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.subscriptions.failedToSave'))
  } finally {
    saving.value = false
  }
}

async function removePlan(plan: SubscriptionPlan) {
  if (!window.confirm(t('admin.subscriptions.deletePlanConfirm', { name: plan.name }))) return
  try {
    await adminAPI.subscriptions.deletePlan(plan.id)
    await loadPlans()
    emit('changed')
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.subscriptions.failedToDelete'))
  }
}

watch(() => props.show, show => { if (show) void loadPlans() })
</script>
