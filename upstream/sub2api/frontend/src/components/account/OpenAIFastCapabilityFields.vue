<template>
  <div class="border-t border-gray-200 pt-4 dark:border-dark-600 space-y-3">
    <div class="flex items-center justify-between gap-4">
      <div>
        <label :id="`${id}-label`" class="input-label mb-0">{{ t('admin.accounts.openai.fastSupported') }}</label>
        <p :id="`${id}-hint`" class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accounts.openai.fastSupportedDesc') }}</p>
      </div>
      <button
        type="button"
        role="switch"
        data-testid="openai-fast-supported"
        :aria-labelledby="`${id}-label`"
        :aria-describedby="`${id}-hint`"
        :aria-checked="supported"
        :class="[
          'relative inline-flex h-6 w-11 shrink-0 rounded-full border-2 border-transparent transition-colors focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2',
          supported ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
        ]"
        @click="$emit('update:supported', !supported)"
      >
        <span :class="['pointer-events-none inline-block h-5 w-5 rounded-full bg-white transition-transform', supported ? 'translate-x-5' : 'translate-x-0']" />
      </button>
    </div>
    <div v-if="supported">
      <label :for="`${id}-models`" class="input-label">{{ t('admin.accounts.openai.fastModels') }}</label>
      <textarea
        :id="`${id}-models`"
        :value="models"
        :aria-describedby="`${id}-models-hint`"
        rows="3"
        class="input"
        data-testid="openai-fast-models"
        :placeholder="t('admin.accounts.openai.fastModelsPlaceholder')"
        @input="$emit('update:models', ($event.target as HTMLTextAreaElement).value)"
      />
      <p :id="`${id}-models-hint`" class="input-hint">{{ t('admin.accounts.openai.fastModelsHint') }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

defineProps<{ id: string; supported: boolean; models: string }>()
defineEmits<{ 'update:supported': [value: boolean]; 'update:models': [value: string] }>()
const { t } = useI18n()
</script>
