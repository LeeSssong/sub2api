<template>
  <BaseDialog :show="show" :title="t('keys.bulk.edit')" brand-theme :close-on-escape="!busy" :show-close-button="!busy" @close="close">
    <form id="bulk-key-form" class="bulk-key-form" @submit.prevent="submit">
      <p class="bulk-hint">{{ t('keys.bulk.selected', { count: pending.length }) }} · {{ t('keys.bulk.hint') }}</p>
      <p v-if="result" role="status" class="bulk-hint">{{ result }}</p>
      <p v-if="error" ref="errorRef" tabindex="-1" role="alert" class="bulk-error">{{ error }}</p>
      <fieldset v-for="field in fields" :key="field" :disabled="busy" class="bulk-field">
        <label class="bulk-toggle"><input v-model="draft.enabled[field]" type="checkbox" :data-field="field" /> {{ t(`keys.bulk.${field}`) }} <span v-if="!draft.enabled[field]">{{ t('keys.bulk.unchanged') }}</span></label>
        <div v-if="draft.enabled[field]" class="bulk-inputs">
          <Select v-if="field === 'group'" v-model="draft.groupId" :options="groupOptions" :aria-label="t('keys.bulk.group')" :placeholder="t('keys.selectGroup')" searchable brand />
          <Select v-if="field === 'status'" v-model="draft.status" :options="statusOptions" :aria-label="t('keys.bulk.status')" brand />
          <template v-if="field === 'quota'">
            <input v-model.number="draft.quota" :aria-label="t('keys.bulk.quota')" class="input" type="number" min="0" step="any" required />
            <p class="bulk-hint">{{ t('keys.bulk.unlimited') }}</p>
          </template>
          <template v-if="field === 'expiry'">
            <input v-model="draft.expiry" :aria-label="t('keys.bulk.expiry')" class="input" type="datetime-local" />
            <p class="bulk-hint">{{ t('keys.bulk.unlimited') }}</p>
          </template>
          <div v-if="field === 'rate'" class="bulk-rates">
            <label v-for="key in rateFields" :key="key">{{ t(`keys.bulk.${key}`) }}<input v-model.number="draft[key]" class="input" type="number" min="0" step="any" required /></label>
          </div>
          <template v-if="field === 'ip'">
            <label>{{ t('keys.bulk.whitelist') }}<textarea v-model="draft.whitelist" class="input" rows="3" /></label>
            <label>{{ t('keys.bulk.blacklist') }}<textarea v-model="draft.blacklist" class="input" rows="3" /></label>
            <p class="bulk-hint">{{ t('keys.bulk.ipHint') }}</p>
          </template>
        </div>
      </fieldset>
    </form>
    <template #footer>
      <button type="button" class="btn btn-secondary" :disabled="busy" @click="close">{{ t('common.cancel') }}</button>
      <button type="submit" form="bulk-key-form" class="btn btn-primary" :disabled="busy || !pending.length || !Object.values(draft.enabled).some(Boolean)">{{ busy ? t('keys.bulk.working') : t('keys.bulk.apply', { count: pending.length }) }}</button>
    </template>
  </BaseDialog>
</template>
<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import { keysAPI } from '@/api'
import type { Group } from '@/types'
import { applyBulkPatch, buildBulkPatch, newBulkDraft, type BulkField } from './bulkEdit'
const props = defineProps<{ show: boolean; ids: number[]; groups: Group[] }>()
const emit = defineEmits<{ close: []; finished: [failed: number[]] }>()
const { t } = useI18n()
const fields: BulkField[] = ['group', 'status', 'quota', 'expiry', 'rate', 'ip']
const rateFields = ['rate5h', 'rate1d', 'rate7d'] as const
const draft = ref(newBulkDraft()), busy = ref(false), pending = ref<number[]>([])
const error = ref(''), result = ref('')
const errorRef = ref<HTMLElement | null>(null)
watch(error, async value => { if (value) { await nextTick(); errorRef.value?.focus() } })
const groupOptions = computed(() => props.groups.filter(g => g.status === 'active').map(g => ({ value: g.id, label: g.name })))
const statusOptions = computed(() => [{ value: 'active', label: t('keys.status.active') }, { value: 'inactive', label: t('keys.status.inactive') }])
watch(() => props.show, show => {
  if (show) { draft.value = newBulkDraft(); pending.value = [...props.ids]; error.value = ''; result.value = '' }
}, { immediate: true })
function close() { if (!busy.value) emit('close') }
async function submit() {
  if (busy.value || !pending.value.length) return
  error.value = ''
  let patch
  try { patch = buildBulkPatch(draft.value) } catch (e) { error.value = t(`keys.bulk.${(e as Error).message}`); return }
  busy.value = true
  try {
    const outcome = await applyBulkPatch(pending.value, patch, keysAPI.update)
    pending.value = outcome.failed
    result.value = t('keys.bulk.result', { success: outcome.succeeded.length, failed: outcome.failed.length })
    emit('finished', outcome.failed)
    if (outcome.failed.length) error.value = t('keys.bulk.failed', { ids: outcome.failed.map(id => `#${id}`).join(', ') })
    else emit('close')
  } finally { busy.value = false }
}
</script>
<style scoped>
.bulk-key-form { display: grid; gap: 0; color: var(--xq-text); }
.bulk-hint { color: var(--xq-muted); font-size: 12px; line-height: 1.6; }
.bulk-error { color: var(--xq-danger); font-size: 13px; overflow-wrap: anywhere; }
.bulk-field { min-width: 0; border: 0; border-bottom: 1px solid var(--xq-line); padding: 16px 0; }
.bulk-field:last-child { border-bottom: 0; }
.bulk-toggle { display: flex; align-items: center; gap: 10px; font-size: 14px; cursor: pointer; }
.bulk-toggle input { width: 16px; height: 16px; accent-color: var(--xq-accent); }
.bulk-toggle span { margin-left: auto; color: var(--xq-muted); font-size: 12px; }
.bulk-inputs { display: grid; gap: 10px; padding-top: 12px; }
.bulk-inputs label { font-size: 12px; color: var(--xq-secondary); }
.bulk-rates { display: grid; grid-template-columns: repeat(3,minmax(0,1fr)); gap: 10px; }
.bulk-inputs input,.bulk-inputs textarea { margin-top: 4px; }
@media(max-width:520px) { .bulk-rates { grid-template-columns: 1fr; } }
</style>
