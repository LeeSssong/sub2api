<template>
  <section class="iq-test">
    <div class="iq-test-heading">
      <span class="iq-test-name"
        ><Icon :name="kind === 'candy' ? 'grid' : 'sparkles'" size="md" />{{
          kind === "candy" ? "逻辑题测试" : "绘图测试"
        }}</span
      >
      <span class="iq-muted">{{
        kind === "candy"
          ? `答案应为 ${expectedAnswer || "—"}`
          : "SVG 山姆奥特曼 + 随机动作与场景"
      }}</span>
      <span v-if="isRunning" class="iq-latest running">● 检测中</span>
      <span v-else-if="latest" class="iq-latest" :class="latest.verdict"
        >● {{ intelligenceVerdictLabel[latest.verdict]
        }}<span class="iq-muted"> · {{ ago }}</span></span
      >
    </div>
    <div class="iq-stats">
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
          :title="`${slotTime(slot)} · ${intelligenceVerdictLabel[slot.result.verdict]}`"
          @click="$emit('select', slot.result)"
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
    <div class="iq-axis">
      <span>{{ slots[0] ? slotTime(slots[0]) : "—" }}</span
      ><span>现在</span>
    </div>
  </section>
</template>
<script setup lang="ts">
import { computed } from "vue";
import Icon from "@/components/icons/Icon.vue";
import type { IntelligenceResult } from "@/api/intelligenceTests";
import {
  intelligenceSlots,
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
  }>(),
  { hours: 24 },
);
defineEmits<{ select: [result: IntelligenceResult] }>();
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
  slots.value.flatMap((s) => (s.result ? [s.result] : [])),
);
const stats = computed(() => intelligenceStats(completed.value));
const latest = computed(() => completed.value.at(-1));
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
