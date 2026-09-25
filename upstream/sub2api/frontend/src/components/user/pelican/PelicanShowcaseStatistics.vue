<template>
  <section
    class="rounded-xl border border-gray-200/80 bg-white/70 px-4 py-4 dark:border-dark-700/70 dark:bg-dark-800/60 sm:px-5"
    :aria-label="t('pelicanShowcase.statistics.title')"
    :aria-busy="loading"
    aria-live="polite"
    data-testid="showcase-statistics"
  >
    <div class="mb-3 flex flex-wrap items-center justify-between gap-x-4 gap-y-1 text-xs text-gray-500 dark:text-gray-400">
      <h2>{{ t('pelicanShowcase.statistics.scope', { group: groupName }) }}</h2>
      <span data-testid="showcase-statistics-window">{{ windowLabel }}</span>
    </div>
    <dl class="grid grid-cols-2 gap-4">
      <div>
        <dt class="mb-1 text-xs text-gray-500 dark:text-gray-400">{{ t('pelicanShowcase.statistics.count') }}</dt>
        <dd class="text-2xl font-semibold tabular-nums text-gray-900 dark:text-gray-100 sm:text-3xl" data-testid="showcase-statistics-count">{{ pelicanStatsCount(stats) }}</dd>
      </div>
      <div>
        <dt class="mb-1 text-xs text-gray-500 dark:text-gray-400">{{ t('pelicanShowcase.statistics.rate') }}</dt>
        <dd class="text-2xl font-semibold tabular-nums sm:text-3xl" :class="stats?.total_count ? 'text-primary-700 dark:text-primary-300' : 'text-gray-500 dark:text-gray-400'" data-testid="showcase-statistics-rate">{{ pelicanStatsRate(stats) }}</dd>
      </div>
    </dl>
    <p class="mt-3 text-xs leading-relaxed text-gray-500 dark:text-gray-400">
      {{ !stats ? t('pelicanShowcase.statistics.unavailable') : stats.total_count === 0 ? t('pelicanShowcase.statistics.noTests') : t('pelicanShowcase.statistics.definition') }}
    </p>
    <p
      v-if="stats && statsWindow && !statsWindow.complete"
      class="mt-2 text-xs leading-relaxed text-amber-700 dark:text-amber-300"
      data-testid="showcase-statistics-coverage"
    >{{ t('pelicanShowcase.statistics.partialCoverage', { time: formatDateTimeToMinute(statsWindow.coverage_started_at) }) }}</p>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { PelicanShowcaseStats, PelicanShowcaseStatsWindow } from '@/api/pelicanShowcase'
import { formatDateTimeToMinute } from '@/utils/format'
import { pelicanStatsCount, pelicanStatsRate } from './pelicanStatistics'

const props = defineProps<{
  stats?: PelicanShowcaseStats | null
  statsWindow?: PelicanShowcaseStatsWindow | null
  groupName: string
  loading: boolean
}>()
const { t } = useI18n()
const windowLabel = computed(() => t(
  props.statsWindow?.complete === false
    ? 'pelicanShowcase.statistics.sinceEnabled'
    : 'pelicanShowcase.statistics.last24Hours'
))
</script>
