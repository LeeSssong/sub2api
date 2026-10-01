<template>
  <div class="min-h-screen" :class="isAdmin ? 'brand-admin-shell bg-gray-50 dark:bg-dark-950' : 'user-app-shell'">
    <!-- Background Decoration -->
    <div class="pointer-events-none fixed inset-0 bg-mesh-gradient"></div>

    <!-- Sidebar -->
    <AppSidebar />

    <!-- Main Content Area -->
    <div
      class="relative min-h-screen transition-all duration-300"
      :class="isAdmin ? ['admin-main-frame', { 'admin-main-collapsed': sidebarCollapsed }] : 'user-main-frame'"
    >
      <!-- Header -->
      <AppHeader v-if="isAdmin" />

      <UserHeader v-else />

      <!-- Main Content -->
      <main class="p-4 md:p-6 lg:p-8" :class="{ 'user-workspace': !isAdmin, 'admin-workspace': isAdmin }">
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
import { useOnboardingTour } from '@/composables/useOnboardingTour'
import { useOnboardingStore } from '@/stores/onboarding'
import AppSidebar from './AppSidebar.vue'
import AppHeader from './AppHeader.vue'
import UserHeader from './UserHeader.vue'

const appStore = useAppStore()
const authStore = useAuthStore()
const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)
const isAdmin = computed(() => authStore.user?.role === 'admin')
provide('starbridge-user', computed(() => !isAdmin.value))

const { replayTour } = useOnboardingTour({
  storageKey: isAdmin.value ? 'admin_guide' : 'user_guide',
  autoStart: true
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
