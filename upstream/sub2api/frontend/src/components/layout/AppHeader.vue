<template>
  <UserHeader :title="pageTitle" show-menu class="dark" />
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import { useAdminSettingsStore } from '@/stores/adminSettings'
import UserHeader from './UserHeader.vue'

const route = useRoute()
const { t } = useI18n()
const appStore = useAppStore()
const adminSettingsStore = useAdminSettingsStore()
const pageTitle = computed(() => {
  if (route.name === 'CustomPage') {
    const id = route.params.id as string
    const item = appStore.cachedPublicSettings?.custom_menu_items?.find(item => item.id === id)
      ?? adminSettingsStore.customMenuItems?.find(item => item.id === id)
    if (item?.label) return item.label
  }
  return route.meta.titleKey ? t(route.meta.titleKey as string) : (route.meta.title as string) || ''
})
</script>
