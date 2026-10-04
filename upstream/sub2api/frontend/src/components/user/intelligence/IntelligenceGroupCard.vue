<template>
  <article class="iq-group">
    <header class="iq-heading">
      <h2 class="iq-group-name">{{ group.name }}</h2>
      <span class="iq-model iq-mono">{{ group.model_id }}</span
      ><span class="iq-effort iq-mono"
        >推理 {{ group.reasoning_effort.toUpperCase() }}</span
      ><span class="iq-multiplier iq-mono">×{{ group.rate_multiplier }}</span
      ><span class="iq-status" :class="status">● {{ statusLabel }}</span>
    </header>
    <p class="iq-description">{{ group.description }}</p>
    <div class="iq-group-grid">
      <div>
        <IntelligenceTimeline
          kind="candy"
          :samples="candy"
          :expected-answer="group.expected_answer"
          :selected="selected?.id"
          :now="now"
          :hours="hours"
          :running-started-at="group.running_started_at"
          @select="select"
        /><IntelligenceTimeline
          kind="pelican"
          :samples="drawing"
          expected-answer=""
          :selected="selected?.id || artwork?.id"
          :now="now"
          :hours="hours"
          :running-started-at="group.running_started_at"
          @select="select"
        />
      </div>
      <aside>
        <div class="iq-art-heading">
          <strong
            >{{
              artwork
                ? `${intelligenceDate(artwork.started_at)} 的画作`
                : "最新画作"
            }}<template v-if="artwork?.action">
              · {{ artwork.action }} · {{ artwork.scene }}</template
            ></strong
          ><span class="iq-mono">{{
            intelligenceDuration(artwork?.latency_ms)
          }}</span>
        </div>
        <div class="iq-art">
          <PelicanArtworkPreview
            v-if="html"
            :html="html"
            :title="`${group.name} 检测画作`"
            :interactive="false"
          />
          <p v-else role="status">
            {{ artError || (artwork ? "正在加载画作…" : "暂无成功画作") }}
          </p>
          <button
            v-if="html && artwork"
            class="iq-zoom"
            @click="select(artwork)"
          >
            <Icon name="externalLink" size="sm" /> 放大
          </button>
        </div>
        <p class="iq-art-hint">
          点击任意色块可查看该次检测：题目、判定结果与模型回复。
        </p>
      </aside>
    </div>
  </article>
</template>
<script setup lang="ts">
import { computed, ref, watch } from "vue";
import Icon from "@/components/icons/Icon.vue";
import PelicanArtworkPreview from "@/components/user/pelican/PelicanArtworkPreview.vue";
import IntelligenceTimeline from "./IntelligenceTimeline.vue";
import type {
  IntelligenceGroup,
  IntelligenceResult,
} from "@/api/intelligenceTests";
import { intelligenceTestsAPI } from "@/api/intelligenceTests";
import {
  intelligenceTimeline,
  intelligenceGroupStatus,
  intelligenceDate,
  intelligenceDuration,
} from "@/utils/intelligenceDashboard";
import { extractPelicanHtml } from "@/utils/pelicanHtml";
const props = defineProps<{
  group: IntelligenceGroup;
  now: number;
  hours: 24 | 72;
}>();
const emit = defineEmits<{ select: [result: IntelligenceResult] }>();
const candy = computed(() =>
  intelligenceTimeline(props.group.results, "candy"),
);
const drawing = computed(() =>
  intelligenceTimeline(props.group.results, "pelican"),
);
const status = computed(() => intelligenceGroupStatus(props.group.results));
const statusLabel = computed(
  () =>
    ({
      empty: "尚未检测",
      partial: "检测未齐",
      passed: "智力正常",
      incorrect: "答题错误",
      abnormal: "检测异常",
    })[status.value],
);
const selected = ref<IntelligenceResult | null>(null);
watch(
  () => props.group.results,
  (results) => {
    if (selected.value && !results.some((r) => r.id === selected.value?.id))
      selected.value = null;
  },
);
const artwork = computed(() =>
  selected.value?.kind === "pelican" && selected.value.verdict === "passed"
    ? selected.value
    : drawing.value.filter((r) => r.verdict === "passed").at(-1),
);
const html = ref("");
const artError = ref("");
let generation = 0;
watch(
  () => artwork.value?.id,
  async (id) => {
    const seq = ++generation;
    html.value = "";
    artError.value = "";
    if (!id) return;
    try {
      const r = await intelligenceTestsAPI.result(id);
      if (seq === generation) {
        html.value = extractPelicanHtml(r.response_text || "");
        if (!html.value) artError.value = "画作内容无法预览";
      }
    } catch {
      if (seq === generation) artError.value = "画作加载失败，请刷新重试";
    }
  },
  { immediate: true },
);
function select(result: IntelligenceResult) {
  selected.value = result;
  emit("select", result);
}
</script>
