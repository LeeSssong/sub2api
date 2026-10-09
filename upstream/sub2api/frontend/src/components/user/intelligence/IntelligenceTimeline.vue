<template>
  <section class="iq-test" :class="{ 'iq-test-compact': compact }">
    <div class="iq-test-heading">
      <span v-if="!compact" class="iq-test-name"
        ><Icon :name="kind === 'candy' ? 'grid' : 'sparkles'" size="md" />{{
          kind === "candy" ? "逻辑题测试" : "绘图测试"
        }}</span
      >
      <span v-if="latest?.model_id || modelId" class="iq-model iq-mono">{{ latest?.model_id || modelId }}</span>
      <span v-if="latest?.reasoning_effort" class="iq-effort iq-mono">推理 {{ latest.reasoning_effort.toUpperCase() }}</span>
      <span v-if="!compact" class="iq-muted">{{
        kind === "candy"
          ? `答案应为 ${expectedAnswer || "—"}`
          : "SVG 山姆奥特曼 + 随机动作与场景"
      }}</span>
      <div v-if="compact" class="iq-stats">
      <strong :class="{ 'iq-muted': !stats.total }"
        >{{ stats.percentage }}<template v-if="stats.total">%</template></strong
      >
      <span
        >{{ stats.passed }}/{{ stats.total }}
        {{ kind === "candy" ? "答对" : "画出" }}</span
      >
      <span v-if="stats.abnormal" class="abnormal"
        >{{ stats.abnormal }} 次异常</span
      >
      <span class="iq-muted"
        >平均
        <span class="iq-mono">{{
          intelligenceDuration(stats.average)
        }}</span></span
      >
    </div>
      <span v-if="isRunning" class="iq-latest running">● 检测中</span>
      <span v-else-if="latest && isStale" class="iq-latest iq-muted">● 结果已过期 · {{ ago }}</span>
      <span v-else-if="latest" class="iq-latest" :class="latest.verdict"
        >● {{ intelligenceVerdictLabel[latest.verdict]
        }}<span class="iq-muted"> · {{ ago }}</span></span
      >
    </div>
    <div v-if="!compact" class="iq-stats">
      <strong :class="{ 'iq-muted': !stats.total }"
        >{{ stats.percentage }}<template v-if="stats.total">%</template></strong
      >
      <span
        >{{ stats.passed }}/{{ stats.total }}
        {{ kind === "candy" ? "答对" : "画出" }}</span
      >
      <span v-if="stats.abnormal" class="abnormal"
        >{{ stats.abnormal }} 次异常</span
      >
      <span class="iq-muted"
        >平均
        <span class="iq-mono">{{
          intelligenceDuration(stats.average)
        }}</span></span
      >
    </div>
    <div
      class="iq-bars"
      :class="{ 'iq-bars-three-days': hours === 72 }"
      :style="{ '--iq-slot-count': slots.length }"
    >
      <template v-for="slot in slots" :key="slot.timestamp">
        <button
          v-if="slot.result"
          class="iq-bar"
          :class="[slot.status, { selected: slot.result.id === selected }]"
          :aria-label="`${slotTime(slot)} ${intelligenceVerdictLabel[slot.result.verdict]}`"
          :aria-pressed="slot.result.id === selected"
          :title="`${slotTime(slot)} · ${slotSamples(slot).length} 次检测 · 最近一次：${intelligenceVerdictLabel[slot.result.verdict]}`"
          @click="selectSlot(slot)"
        />
        <span
          v-else
          class="iq-bar"
          :class="slot.status"
          :title="`${slotTime(slot)} · ${slot.status === 'running' ? '检测中' : '暂无数据'}`"
          :aria-label="`${slotTime(slot)} ${slot.status === 'running' ? '检测中' : '暂无数据'}`"
        />
      </template>
    </div>
    <div v-if="expandedSamples.length > 1" class="iq-slot-results" aria-label="时段内全部检测">
      <button v-for="r in expandedSamples" :key="r.id" class="iq-slot-result" :class="r.verdict" @click="emit('select', r)">{{ intelligenceDate(r.started_at) }} · {{ intelligenceVerdictLabel[r.verdict] }}</button>
    </div>
    <div class="iq-axis">
      <span>{{ slots[0] ? slotTime(slots[0]) : "—" }}</span
      ><span>现在</span>
    </div>
  </section>
</template>
<script setup lang="ts">
import { computed, ref } from "vue";
import Icon from "@/components/icons/Icon.vue";
import type { IntelligenceResult } from "@/api/intelligenceTests";
import {
  intelligenceSlots,
  intelligenceWindow,
  intelligenceTimeline,
  intelligenceResultStale,
  intelligenceStats,
  intelligenceDate,
  intelligenceDuration,
  intelligenceVerdictLabel,
  type IntelligenceSlot,
} from "@/utils/intelligenceDashboard";
const props = withDefaults(
  defineProps<{
    kind: "candy" | "pelican";
    samples: IntelligenceResult[];
    expectedAnswer: string;
    selected?: number;
    now: number;
    hours?: 24 | 72;
    runningStartedAt?: string;
    modelId?: string;
    compact?: boolean;
  }>(),
  { hours: 24 },
);
const emit = defineEmits<{ select: [result: IntelligenceResult] }>();
const slots = computed(() =>
  intelligenceSlots(
    props.samples,
    props.kind,
    props.now,
    props.hours,
    props.runningStartedAt,
  ),
);
const completed = computed(() =>
  intelligenceTimeline(intelligenceWindow(props.samples, props.now, props.hours), props.kind),
);
const expandedSlot = ref<number | null>(null);
function slotSamples(slot: IntelligenceSlot) {
  return completed.value.filter(r => Date.parse(r.started_at) >= slot.timestamp && Date.parse(r.started_at) < slot.timestamp + 30 * 60 * 1000);
}
const expandedSamples = computed(() => {
  const slot = slots.value.find(s => s.timestamp === expandedSlot.value);
  return slot ? slotSamples(slot) : [];
});
function selectSlot(slot: IntelligenceSlot) {
  if (slotSamples(slot).length > 1) expandedSlot.value = expandedSlot.value === slot.timestamp ? null : slot.timestamp;
  else if (slot.result) { expandedSlot.value = null; emit('select', slot.result); }
}
const stats = computed(() => intelligenceStats(completed.value));
const latest = computed(() => completed.value.at(-1));
const isStale = computed(() => !!latest.value && intelligenceResultStale(latest.value, props.now));
const isRunning = computed(() =>
  slots.value.some((s) => s.status === "running"),
);
const slotTime = (slot: IntelligenceSlot) =>
  intelligenceDate(new Date(slot.timestamp).toISOString());
const ago = computed(() => {
  const m = Math.max(
    0,
    Math.floor(
      (props.now - Date.parse(latest.value?.started_at || "")) / 60000,
    ),
  );
  return m < 1
    ? "刚刚"
    : m < 60
      ? `${m} 分钟前`
      : `${Math.floor(m / 60)} 小时前`;
});
</script>

<style scoped>
.iq-slot-results { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 8px; }
.iq-slot-result { padding: 5px 8px; border: 1px solid var(--xq-border); border-radius: 8px; font-size: 12px; }
</style>
