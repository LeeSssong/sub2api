<template>
  <section class="storefront card min-w-0 overflow-hidden" aria-labelledby="storefront-title">
    <div class="flex flex-wrap items-start justify-between gap-3 border-b border-[var(--xq-border)] px-5 py-4">
      <div class="min-w-0">
        <h2 id="storefront-title" class="text-base font-semibold text-[var(--xq-text)]">{{ t('redeem.storefrontTitle') }}</h2>
        <p class="mt-1 text-sm leading-6 text-[var(--xq-secondary)]">{{ t('redeem.storefrontInstructions') }}</p>
      </div>
      <div class="flex flex-wrap gap-2">
        <button v-if="canPositionProducts && !hasNavigated" data-test="storefront-view-toggle" type="button" class="btn btn-secondary min-h-[44px]" @click="toggleView">
          {{ t(productView ? 'redeem.storefrontShowFull' : 'redeem.storefrontShowProducts') }}
        </button>
        <a data-test="storefront-open" :href="url" target="_blank" rel="noopener noreferrer" class="btn btn-secondary min-h-[44px] shrink-0">
          <Icon name="externalLink" size="sm" aria-hidden="true" />
          {{ t('customPage.openInNewTab') }}
        </a>
      </div>
    </div>
    <p v-if="canPositionProducts" id="storefront-swipe-hint" class="storefront-swipe-hint px-5 py-2 text-xs leading-5 text-[var(--xq-secondary)]">{{ t('redeem.storefrontSwipeHint') }}</p>
    <div ref="viewport" data-test="storefront-viewport" :data-view="productView ? 'products' : 'full'" :class="{ 'storefront-viewport-wide': canPositionProducts }" class="storefront-viewport" role="region" :aria-label="t('redeem.storefrontTitle')" :aria-describedby="canPositionProducts ? 'storefront-swipe-hint' : undefined" tabindex="0">
      <iframe :key="url" :src="url" :title="t('redeem.storefrontTitle')" class="storefront-frame" allowfullscreen @load="onFrameLoad"></iframe>
    </div>
    <p class="border-t border-[var(--xq-border)] px-5 py-3 text-xs leading-5 text-[var(--xq-secondary)]">
      {{ t('redeem.storefrontFallback') }}
      <a :href="url" target="_blank" rel="noopener noreferrer" class="font-medium text-[var(--xq-accent)] underline underline-offset-4">{{ t('customPage.openInNewTab') }}</a>
    </p>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{ url: string }>()
const { t } = useI18n()
const viewport = ref<HTMLDivElement>()
// The public shop currently starts its product heading at ~420px. Only use
// this measured offset for this entry; unrelated shops and order pages stay full.
const productOffset = 400
const canPositionProducts = computed(() => {
  try {
    const entry = new URL(props.url)
    return entry.origin === 'https://catfk.com' && entry.pathname === '/shop/DLK8SNUJ'
  } catch {
    return false
  }
})
const productView = ref(canPositionProducts.value)
const hasNavigated = ref(false)
let loaded = false

async function positionViewport() {
  await nextTick()
  if (!viewport.value) return
  viewport.value.scrollTop = productView.value ? productOffset : 0
  viewport.value.scrollLeft = 0
}

function toggleView() {
  productView.value = !productView.value
  void positionViewport()
}

function onFrameLoad() {
  if (loaded) {
    // A cross-origin navigation may be a checkout or order page. We cannot
    // inspect its URL, so stop applying the shop offset to all later loads.
    productView.value = false
    hasNavigated.value = true
  }
  loaded = true
  void positionViewport()
}

watch(() => props.url, () => {
  loaded = false
  hasNavigated.value = false
  productView.value = canPositionProducts.value
  void positionViewport()
})
onMounted(positionViewport)
</script>

<style scoped>
.storefront {
  container-type: inline-size;
}

.storefront-viewport {
  --storefront-window-height: max(1100px, 100vh);
  height: var(--storefront-window-height);
  overflow: auto;
  overscroll-behavior: contain;
  scroll-behavior: auto;
}

.storefront-viewport:focus-visible {
  outline: 2px solid var(--xq-accent);
  outline-offset: -2px;
}

.storefront-viewport[data-view='products'] {
  --storefront-window-height: max(760px, calc(100vh - 240px));
}

.storefront-frame {
  display: block;
  width: 100%;
  height: var(--storefront-window-height);
  border: 0;
  background: var(--xq-surface);
}

.storefront-viewport-wide .storefront-frame {
  min-width: 720px;
}

.storefront-viewport[data-view='products'] .storefront-frame {
  /* Equal room above and below the initial viewport keeps the shop's fixed,
     centered order modal visible, unlike permanently cropping its header. */
  height: calc(var(--storefront-window-height) + 800px);
}

.storefront-swipe-hint {
  display: none;
}

@container (max-width: 719px) {
  .storefront-swipe-hint {
    display: block;
  }
}
</style>
