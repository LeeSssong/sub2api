<template>
  <div class="min-h-screen" :class="isUserSurface ? 'user-app-shell' : 'brand-admin-shell bg-gray-50 dark:bg-dark-950'">
    <!-- Background Decoration -->
    <div class="pointer-events-none fixed inset-0 bg-mesh-gradient"></div>

    <!-- Sidebar -->
    <AppSidebar />

    <!-- Main Content Area -->
    <div
      class="relative min-h-screen transition-all duration-300"
      :class="isUserSurface ? 'user-main-frame' : ['admin-main-frame', { 'admin-main-collapsed': sidebarCollapsed }]"
    >
      <!-- Header -->
      <AppHeader v-if="!isUserSurface || showDesktopHeader" />

      <UserHeader v-else />

      <!-- Main Content -->
      <main class="p-4 md:p-6 lg:p-8" :class="{ 'user-workspace': isUserSurface, 'admin-workspace': !isUserSurface }">
        <slot />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import '@/styles/onboarding.css'
import '@/styles/xingqiao-brand.css'
import '@/styles/xingqiao-user.css'
import '@/styles/xingqiao-ai.css'
import { computed, onMounted, provide } from 'vue'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import { useAppSurface } from '@/composables/useAppSurface'
import { useOnboardingTour } from '@/composables/useOnboardingTour'
import { useOnboardingStore } from '@/stores/onboarding'
import AppSidebar from './AppSidebar.vue'
import AppHeader from './AppHeader.vue'
import UserHeader from './UserHeader.vue'

withDefaults(defineProps<{ showDesktopHeader?: boolean }>(), { showDesktopHeader: false })

const appStore = useAppStore()
const authStore = useAuthStore()
const { isUserSurface } = useAppSurface()
const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)
provide('starbridge-user', isUserSurface)

const { replayTour } = useOnboardingTour({
  storageKey: authStore.isAdmin ? 'admin_guide' : 'user_guide',
  autoStart: !isUserSurface.value
})

const onboardingStore = useOnboardingStore()

onMounted(() => {
  onboardingStore.setReplayCallback(replayTour)
})

defineExpose({ replayTour })
</script>

<style scoped>
.user-app-shell {
  color: var(--xq-text);
}

.user-workspace {
  min-height: 0;
  background: var(--xq-void);
}
</style>
