import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useAdminSettingsStore, useAppStore, useAuthStore } from '@/stores'

/** Layout follows the current workspace; account roles continue to govern access. */
export function useAppSurface() {
  const route = useRoute()
  const authStore = useAuthStore()
  const appStore = useAppStore()
  const adminSettingsStore = useAdminSettingsStore()

  const isUserSurface = computed(() => {
    if (route.path === '/admin' || route.path.startsWith('/admin/')) return false
    if (route.meta.requiresAdmin || route.meta.requiresAccountManagement) return false
    if (authStore.isObserver && route.path === '/usage') return false

    if (route.path.startsWith('/custom/')) {
      const item = appStore.cachedPublicSettings?.custom_menu_items?.find(item => item.id === route.params.id)
        ?? adminSettingsStore.customMenuItems.find(item => item.id === route.params.id)
      if (item?.visibility === 'admin') return false
    }
    return true
  })

  return { isUserSurface }
}
