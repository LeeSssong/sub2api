<template>
  <section class="card min-w-0 overflow-hidden" aria-labelledby="storefront-title">
    <div class="flex flex-wrap items-start justify-between gap-3 border-b border-[var(--xq-border)] px-5 py-4">
      <div class="min-w-0">
        <h2 id="storefront-title" class="text-base font-semibold text-[var(--xq-text)]">{{ t('redeem.storefrontTitle') }}</h2>
        <p class="mt-1 text-sm leading-6 text-[var(--xq-secondary)]">{{ t('redeem.storefrontInstructions') }}</p>
      </div>
      <a data-test="storefront-open" :href="url" target="_blank" rel="noopener noreferrer" class="btn btn-secondary min-h-[44px] shrink-0">
        <Icon name="externalLink" size="sm" aria-hidden="true" />
        {{ t('customPage.openInNewTab') }}
      </a>
    </div>
    <iframe :src="url" :title="t('redeem.storefrontTitle')" class="storefront-frame" allowfullscreen></iframe>
    <p class="border-t border-[var(--xq-border)] px-5 py-3 text-xs leading-5 text-[var(--xq-secondary)]">
      {{ t('redeem.storefrontFallback') }}
      <a :href="url" target="_blank" rel="noopener noreferrer" class="font-medium text-[var(--xq-accent)] underline underline-offset-4">{{ t('customPage.openInNewTab') }}</a>
    </p>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

defineProps<{ url: string }>()
const { t } = useI18n()
</script>

<style scoped>
.storefront-frame {
  display: block;
  width: 100%;
  height: clamp(540px, 68vh, 760px);
  border: 0;
  background: var(--xq-surface);
}
</style>
