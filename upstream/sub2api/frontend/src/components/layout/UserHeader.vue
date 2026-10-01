<template>
  <header class="user-topbar">
    <div class="user-topbar-title">
      <button v-if="showMenu" type="button" class="shell-menu-toggle" :aria-label="t('common.toggleMenu')" :aria-expanded="appStore.mobileOpen" @click="appStore.toggleMobileSidebar()">
        <Icon name="menu" size="sm" />
      </button>
      <h1 :title="title || pageTitle">{{ title || pageTitle }}</h1>
    </div>
    <nav class="user-topbar-actions brand-header-actions" :aria-label="t('nav.docs')">
      <AnnouncementBell show-label />
      <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer" class="user-doc-link">
        <Icon name="book" size="sm" /><span>{{ t('nav.docs') }}</span>
      </a>
      <button v-else type="button" disabled class="user-doc-link" :title="t('common.docsNotConfigured')">
        <Icon name="book" size="sm" /><span>{{ t('nav.docs') }}</span>
      </button>
      <LocaleSwitcher compact />
    </nav>
  </header>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import AnnouncementBell from '@/components/common/AnnouncementBell.vue'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import { sanitizeUrl } from '@/utils/url'

defineProps<{ title?: string; showMenu?: boolean }>()

const route = useRoute()
const { t } = useI18n()
const appStore = useAppStore()
const docUrl = computed(() => sanitizeUrl(appStore.docUrl))
const pageTitle = computed(() => {
  if (route.name === 'CustomPage') {
    const item = appStore.cachedPublicSettings?.custom_menu_items?.find(item => item.id === route.params.id)
    if (item) return item.label
  }
  return route.meta.titleKey ? t(route.meta.titleKey as string) : (route.meta.title as string) || ''
})
</script>
