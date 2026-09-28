<template>
  <Select
    class="xq-line-select"
    dropdown-class="xq-line-select-dropdown"
    :model-value="modelValue"
    :options="options"
    :placeholder="placeholder"
    :search-placeholder="searchPlaceholder"
    :empty-text="emptyText"
    :aria-label="ariaLabel"
    searchable
    brand
    @update:model-value="emit('update:modelValue', $event as number | null)"
  >
    <template #selected="{ option }">
      <span v-if="option" class="line-selected">
        <img :src="providerIcon(line(option).platform)" alt="" />
        <span>{{ line(option).label }}</span>
        <small>{{ line(option).rateLabel }}</small>
      </span>
      <span v-else>{{ placeholder }}</span>
    </template>
    <template #option="{ option }">
      <div class="line-option">
        <div class="line-option-head">
          <img :src="providerIcon(line(option).platform)" alt="" />
          <strong>{{ line(option).label }}</strong>
          <span class="line-rate">{{ line(option).rateLabel }}</span>
          <span class="line-status" :data-status="line(option).group.status">
            <i aria-hidden="true" />{{ line(option).group.status === 'active' ? '可用' : line(option).statusLabel }}
          </span>
        </div>
        <small class="line-option-meta">关联密钥 {{ line(option).linkedCount == null ? '暂不可用' : `${line(option).linkedCount} 把` }}</small>
        <small class="line-option-meta">近 1 小时稳定性 <span class="line-success-rate" :data-tone="line(option).successTone">{{ line(option).successLabel }}</span> · 首字 {{ line(option).ttftLabel }}</small>
      </div>
    </template>
  </Select>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import Select from '@/components/common/Select.vue'
import { providerIcon } from '@/features/ai-tools/model'
import type { Group } from '@/types'
import type { MonitorV4Group } from '@/features/monitor-v4/types'
import { buildLineOptions, type LineOption } from './lineOptions'

const props = withDefaults(defineProps<{
  modelValue: number | null
  groups: Group[]
  rates: Record<number, number>
  metrics: Map<number, MonitorV4Group>
  linkedCounts: Map<number, number> | null
  toolId?: string
  placeholder?: string
  searchPlaceholder?: string
  emptyText?: string
  ariaLabel?: string
}>(), {
  placeholder: '选择线路',
  searchPlaceholder: '搜索线路',
  emptyText: '暂无可配置线路',
  ariaLabel: '选择线路'
})
const emit = defineEmits<{ 'update:modelValue': [value: number | null] }>()
const options = computed(() => buildLineOptions(props.groups, props.rates, props.metrics, props.linkedCounts, props.toolId))
const line = (value: unknown) => value as LineOption
</script>

<style scoped>
.line-selected,.line-option-head{display:flex;align-items:center;gap:8px;min-width:0}
.line-selected img,.line-option-head img{width:20px;height:20px;object-fit:contain;flex:none}
.line-selected>span,.line-option-head strong{min-width:0;overflow-wrap:anywhere}
.line-selected small{margin-left:auto;color:var(--xq-secondary)}
.line-option{min-width:0;width:100%;display:grid;gap:7px;color:var(--xq-text)}
.line-option-head strong{flex:1;font-size:14px;font-weight:600}
.line-rate{flex:none;padding:3px 8px;border-radius:999px;background:rgba(97,201,217,.12);color:var(--xq-accent);font-size:11px;font-weight:600}
.line-status{display:inline-flex;align-items:center;gap:5px;flex:none;color:var(--xq-secondary);font-size:11px}
.line-status i{width:6px;height:6px;border-radius:50%;background:var(--xq-success);box-shadow:0 0 0 3px rgba(140,223,196,.08)}
.line-status[data-status="disabled"] i{background:var(--xq-danger);box-shadow:0 0 0 3px rgba(212,126,126,.08)}
.line-status[data-status="disabled"]{color:var(--xq-danger)}
.line-option-meta{font-size:11px;color:var(--xq-secondary);white-space:normal}
.line-success-rate{color:var(--xq-muted)}
.line-success-rate[data-tone="green"]{color:var(--xq-success)}
.line-success-rate[data-tone="amber"]{color:var(--xq-warning)}
.line-success-rate[data-tone="red"]{color:var(--xq-danger)}

:deep(.xq-line-select-dropdown){
  min-width:420px !important;
  max-width:min(480px, calc(100vw - 16px));
  padding:8px;
  border-radius:14px;
  border-color:#24586b;
  background:#071b2b;
  box-shadow:0 18px 45px rgba(0,0,0,.35);
}
:deep(.xq-line-select-dropdown .select-search){
  margin-bottom:8px;
  padding:10px 12px;
  border:1px solid #31596c;
  border-radius:10px;
  background:#0a2134;
}
:deep(.xq-line-select-dropdown .select-options){max-height:min(480px, calc(100vh - 150px));padding:0;}
:deep(.xq-line-select-dropdown .select-option){
  align-items:stretch;
  margin:4px 0;
  padding:12px 14px;
  border:1px solid transparent;
  border-radius:10px;
  background:#102c43;
}
:deep(.xq-line-select-dropdown .select-option:hover),
:deep(.xq-line-select-dropdown .select-option-focused){
  background:#153a53;
  border-color:#28667a;
}
:deep(.xq-line-select-dropdown .select-option-selected){
  background:#123c4c;
  border-color:var(--xq-accent);
}
:deep(.xq-line-select-dropdown .select-option > svg){flex:none;margin-top:2px;color:var(--xq-accent)}
:deep(.xq-line-select-dropdown .select-empty){padding:28px 16px;color:var(--xq-secondary)}
@media (max-width:700px){
  :deep(.xq-line-select-dropdown){min-width:0 !important;width:calc(100vw - 32px);max-width:calc(100vw - 32px)}
  .line-option-head{gap:6px;flex-wrap:wrap}
  .line-option-head strong{flex-basis:calc(100% - 32px)}
  .line-status{margin-left:28px}
}
</style>
