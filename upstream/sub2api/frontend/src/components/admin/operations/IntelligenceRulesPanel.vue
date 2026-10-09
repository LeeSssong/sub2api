<template>
  <section class="rules-panel">
    <header>
      <div>
        <h3>检测规则</h3>
        <p>逻辑题抽样复用质量运维结果，绘图每 30 分钟执行一次。</p>
      </div>
      <button
        v-if="showCreate"
        class="btn btn-primary"
        data-testid="intelligence-create"
        @click="open()"
      >
        创建规则
      </button>
    </header>
    <p v-if="!rules.length" class="rules-empty">
      创建第一条规则，选择质量来源、绘图配置和生效分组。
    </p>
    <article v-for="rule in rules" :key="rule.id" class="rule-row">
      <div>
        <strong>{{ rule.name }}</strong
        ><span class="rule-state">{{
          rule.plans[0].enabled ? "已启用" : "已暂停"
        }}</span>
        <p>{{ rule.plans.map((p) => p.group_name).join(" · ") }}</p>
        <small
          >{{ rule.plans[0].model_id }} ·
          {{ rule.plans[0].pelican_config.reasoning_effort }} ·
          {{ rule.plans[0].cron_expression }}</small
        >
      </div>
      <div class="rule-actions">
        <button
          :disabled="busy || rule.plans.some(running)"
          @click="run(rule.plans)"
        >
          立即检测</button
        ><button
          :disabled="busy"
          @click="toggle(rule.plans)"
        >
          {{ rule.plans[0].enabled ? "暂停" : "启用" }}</button
        ><button
          :disabled="busy"
          @click="open(rule.plans)"
        >
          编辑</button
        ><button
          :disabled="busy || rule.plans.some(running)"
          @click="deleting = rule.id"
        >
          删除
        </button>
      </div>
    </article>
    <p v-if="error" class="text-red-500" role="alert">{{ error }}</p>
    <p v-if="notice" class="text-emerald-500" role="status">{{ notice }}</p>
    <BaseDialog
      :show="!!draft"
      :title="editing ? '编辑检测规则' : '创建检测规则'"
      width="extra-wide"
      @close="!busy && (draft = null)"
    >
      <form
        v-if="draft"
        id="intelligence-rule-form"
        class="rule-form"
        @submit.prevent="save"
      >
        <fieldset :disabled="busy">
          <p v-if="editingRunning" class="rule-help" role="status">
            检测正在进行，可以暂停或修改；修改将在下一个周期生效，本轮检测继续使用原配置。
          </p>
          <div class="rule-fields">
            <label
              >规则名称<input
                v-model.trim="draft.name"
                class="input"
                required
                maxlength="100"
                data-testid="intelligence-name" /></label
            ><label
              >绘图模型<input
                v-model.trim="draft.model_id"
                class="input"
                required
                maxlength="100"
                placeholder="gpt-6-astra"
                data-testid="intelligence-model" /></label
            ><label>绘图频率<input class="input" value="每 30 分钟" disabled data-testid="intelligence-cron" /><small class="rule-help">逻辑题按相同时段抽样展示已完成的质量检测。</small></label>
            <label
              >推理强度<select v-model="draft.reasoning_effort" class="input">
                <option
                  v-for="effort in [
                    'minimal',
                    'low',
                    'medium',
                    'high',
                    'xhigh',
                  ]"
                  :key="effort"
                >
                  {{ effort }}
                </option>
              </select></label
            >
          </div>
          <fieldset class="rule-section">
            <legend>逻辑题来源</legend>
            <p class="rule-help">每个展示时段从所选质量规则中抽样，保留原判定和检测时间。缺失结果不会补测。</p>
            <label>逻辑题模型（每行一项）<textarea v-model="candyModels" class="input" rows="2" required data-testid="intelligence-candy-models" /></label>
            <p v-if="templatesLoading" role="status">正在加载质量规则…</p>
            <p v-if="templatesError" role="alert">质量规则加载失败 <button type="button" @click="loadTemplates">重试</button></p>
            <label v-for="id in draft.group_ids" :key="id" class="quality-source-choice">
              {{ groups.find(g => g.id === id)?.name || `分组 #${id}` }} 的质量来源
              <select v-model.number="sourceSelections[id]" class="input" required :data-testid="`intelligence-source-${id}`">
                <option :value="0" disabled>选择质量规则</option>
                <option v-for="template in sourceCandidates(id)" :key="template.id" :value="template.id">规则 #{{ template.id }} · {{ template.cron_expression }}</option>
              </select>
              <small v-if="!sourceCandidates(id).length" class="rule-help">暂无匹配分组且包含所选模型的质量规则，请先在质量运维中配置。</small>
            </label>
            <p v-if="!draft.group_ids.length" class="rule-help">先在下方选择生效分组。</p>
          </fieldset>
          <fieldset class="rule-section">
            <legend>画图配置</legend>
            <div class="flex justify-between">
              <span>提示词模板</span
              ><button
                type="button"
                class="text-primary-500"
                @click="draft.drawing_prompt = INTELLIGENCE_DRAWING_PROMPT"
              >
                恢复默认模板
              </button>
            </div>
            <textarea
              v-model="draft.drawing_prompt"
              class="input"
              required
              rows="4"
              maxlength="32000"
              data-testid="intelligence-drawing"
            />
            <p class="rule-help">
              保留 {动作} 和 {场景}，每次检测随机组合并记录实际题目。
            </p>
            <div class="rule-fields">
              <label
                >动作（每行一项）<textarea
                  v-model="actions"
                  class="input"
                  rows="5"
                  required
                /></label
              ><label
                >场景（每行一项）<textarea
                  v-model="scenes"
                  class="input"
                  rows="5"
                  required
                />
              </label>
            </div>
          </fieldset>
          <fieldset class="rule-section">
            <legend>生效分组</legend>
            <p v-if="groupsLoading">正在加载分组…</p>
            <p v-if="groupsError" role="alert">
              分组加载失败
              <button type="button" @click="loadGroups">重试</button>
            </p>
            <label v-for="g in groups" :key="g.id" class="group-choice"
              ><input
                v-model="draft.group_ids"
                type="checkbox"
                :value="g.id"
                :data-testid="`intelligence-group-${g.id}`"
              />{{ g.name }}</label
            >
            <p v-for="id in missingGroups" :key="id" class="text-amber-600">
              已选分组 #{{ id }} 当前不可用
              <button
                type="button"
                @click="
                  draft.group_ids = draft.group_ids.filter((g) => g !== id)
                "
              >
                移除
              </button>
            </p>
          </fieldset>
          <label class="judge-switch"
            ><input v-model="draft.enabled" type="checkbox" /> 启用规则</label
          >
          <p v-if="formError" role="alert" class="text-red-500">
            {{ formError }}
          </p>
        </fieldset>
      </form>
      <template #footer
        ><button
          class="btn btn-secondary"
          :disabled="busy"
          @click="draft = null"
        >
          取消</button
        ><button
          form="intelligence-rule-form"
          type="submit"
          class="btn btn-primary"
          :disabled="busy || groupsLoading || groupsError || templatesLoading || templatesError"
          data-testid="intelligence-save"
        >
          {{ busy ? "正在保存…" : "保存规则" }}
        </button></template
      >
    </BaseDialog>
    <ConfirmDialog
      :show="!!deleting"
      title="删除检测规则"
      message="删除规则将移除其全部分组计划及检测历史。确定继续？"
      confirm-text="删除"
      @cancel="deleting = null"
      @confirm="remove"
    />
  </section>
</template>
<script setup lang="ts">
import { computed, ref, watch } from "vue";
import BaseDialog from "@/components/common/BaseDialog.vue";
import ConfirmDialog from "@/components/common/ConfirmDialog.vue";
import { adminAPI } from "@/api";
import type { AdminGroup } from "@/types";
import {
  pelicanTestsAPI,
  type PelicanGroupTestPlan,
} from "@/api/admin/pelicanTests";
import {
  intelligenceRulesAPI,
  type IntelligenceRuleInput,
} from "@/api/admin/intelligenceRules";
import {
  intelligenceRuleDefaults,
  intelligenceRuleFromPlans,
  INTELLIGENCE_DRAWING_PROMPT,
} from "@/utils/intelligenceRules";
import { listQualityTemplates, type QualityRuleTemplate } from "@/api/admin/accountQuality";
const props = withDefaults(
  defineProps<{ plans: PelicanGroupTestPlan[]; showCreate?: boolean }>(),
  { showCreate: true },
);
const emit = defineEmits<{ changed: [] }>();
const rules = computed(() => {
  const map = new Map<
    string,
    { id: string; name: string; plans: PelicanGroupTestPlan[] }
  >();
  for (const p of props.plans) {
    const r = p.pelican_config?.intelligence;
    if (!r) continue;
    if (!map.has(r.id)) map.set(r.id, { id: r.id, name: r.name, plans: [] });
    map.get(r.id)!.plans.push(p);
  }
  return [...map.values()];
});
const draft = ref<IntelligenceRuleInput | null>(null);
const editing = ref("");
const actions = ref("");
const scenes = ref("");
const candyModels = ref("");
const templates = ref<QualityRuleTemplate[]>([]);
const templatesLoading = ref(false);
const templatesError = ref(false);
const sourceSelections = ref<Record<number, number>>({});
const modelList = computed(() => [...new Set(candyModels.value.split("\n").map(m => m.trim()).filter(Boolean))]);
function sourceCandidates(groupId: number) {
  return templates.value.filter(t => t.enabled && t.pelican_config.question_kind === "candy" && !!t.pelican_config.quality && t.account_filter.group === String(groupId) && modelList.value.every(m => (t.pelican_config.model_ids?.length ? t.pelican_config.model_ids : [t.model_id]).includes(m)));
}
function syncSources() {
  for (const id of draft.value?.group_ids || []) {
    const candidates = sourceCandidates(id);
    if (!candidates.some(t => t.id === sourceSelections.value[id])) sourceSelections.value[id] = candidates.length === 1 ? candidates[0].id : 0;
  }
}
watch(() => [draft.value?.group_ids.join(","), candyModels.value], syncSources);
async function loadTemplates() {
  templatesLoading.value = true;
  templatesError.value = false;
  try { templates.value = await listQualityTemplates(); syncSources(); }
  catch { templatesError.value = true; }
  finally { templatesLoading.value = false; }
}
const groups = ref<AdminGroup[]>([]);
const groupsLoading = ref(false);
const groupsError = ref(false);
const busy = ref(false);
const error = ref("");
const formError = ref("");
const notice = ref("");
const deleting = ref<string | null>(null);
const editingRunning = computed(() =>
  props.plans.some(
    (p) => p.pelican_config.intelligence?.id === editing.value && running(p),
  ),
);
const missingGroups = computed(
  () =>
    draft.value?.group_ids.filter(
      (id) => !groups.value.some((g) => g.id === id),
    ) || [],
);
function running(p: PelicanGroupTestPlan) {
  return !!p.running_until && Date.parse(p.running_until) > Date.now();
}
async function loadGroups() {
  groupsLoading.value = true;
  groupsError.value = false;
  try {
    groups.value = (await adminAPI.groups.getAll()).filter(
      (g) => g.status === "active",
    );
  } catch {
    groupsError.value = true;
  } finally {
    groupsLoading.value = false;
  }
}
function open(plans?: PelicanGroupTestPlan[]) {
  draft.value = plans
    ? intelligenceRuleFromPlans(plans)
    : intelligenceRuleDefaults();
  editing.value = plans?.[0].pelican_config.intelligence?.id || "";
  actions.value = draft.value.actions.join("\n");
  scenes.value = draft.value.scenes.join("\n");
  candyModels.value = draft.value.candy_models.join("\n");
  sourceSelections.value = Object.fromEntries(draft.value.quality_sources.map(s => [s.group_id, s.template_id]));
  void loadTemplates();
  formError.value = "";
  void loadGroups();
}
defineExpose({ open });
function errorText(e: unknown) {
  const response = (e as { response?: { data?: { message?: string } } })
    ?.response?.data;
  return response?.message || "操作失败，请重试";
}
async function save() {
  if (!draft.value || busy.value) return;
  formError.value = "";
  if (!draft.value.group_ids.length) {
    formError.value = "请选择至少一个生效分组";
    return;
  }
  if (!modelList.value.length) { formError.value = "请选择逻辑题模型"; return; }
  if (templatesLoading.value || templatesError.value || draft.value.group_ids.some(id => !sourceCandidates(id).some(t => t.id === sourceSelections.value[id]))) {
    formError.value = "请为每个生效分组选择有效的质量规则来源"; return;
  }
  if (
    !draft.value.drawing_prompt.includes("{动作}") ||
    !draft.value.drawing_prompt.includes("{场景}")
  ) {
    formError.value = "画图模板必须包含 {动作} 和 {场景}";
    return;
  }
  busy.value = true;
  try {
    await intelligenceRulesAPI.save(
      {
        ...draft.value,
        actions: actions.value
          .split("\n")
          .map((s) => s.trim())
          .filter(Boolean),
        scenes: scenes.value
          .split("\n")
          .map((s) => s.trim())
          .filter(Boolean),
        candy_models: [...modelList.value],
        quality_sources: draft.value.group_ids.map(group_id => ({ group_id, template_id: sourceSelections.value[group_id] })),
        cron_expression: "*/30 * * * *",
      },
      editing.value || undefined,
    );
    draft.value = null;
    notice.value = "检测规则已保存";
    emit("changed");
  } catch (e) {
    formError.value = errorText(e);
  } finally {
    busy.value = false;
  }
}
async function toggle(plans: PelicanGroupTestPlan[]) {
  busy.value = true;
  error.value = "";
  try {
    const input = intelligenceRuleFromPlans(plans);
    input.enabled = !input.enabled;
    await intelligenceRulesAPI.save(
      input,
      plans[0].pelican_config.intelligence!.id,
    );
    emit("changed");
  } catch (e) {
    error.value = errorText(e);
  } finally {
    busy.value = false;
  }
}
async function run(plans: PelicanGroupTestPlan[]) {
  busy.value = true;
  error.value = "";
  const started: string[] = [];
  const failed: string[] = [];
  for (const p of plans) {
    if (running(p)) {
      failed.push(`${p.group_name}（正在运行）`);
      continue;
    }
    try {
      await pelicanTestsAPI.runPlan(p.id);
      started.push(p.group_name);
    } catch (e) {
      failed.push(`${p.group_name}（${errorText(e)}）`);
    }
  }
  notice.value = started.length ? `已启动：${started.join("、")}` : "";
  error.value = failed.length ? `未启动：${failed.join("、")}` : "";
  busy.value = false;
  emit("changed");
}

async function remove() {
  if (!deleting.value) return;
  busy.value = true;
  try {
    await intelligenceRulesAPI.remove(deleting.value);
    deleting.value = null;
    emit("changed");
  } catch (e) {
    error.value = errorText(e);
  } finally {
    busy.value = false;
  }
}
</script>
<style scoped>
.rules-panel {
  border: 1px solid rgb(148 163 184 / 0.22);
  border-radius: 16px;
  padding: 24px;
  background: var(--color-background, #fff);
  margin: 20px 0;
}
.dark .rules-panel {
  background: #111c2d;
}
.rules-panel > header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
}
.rules-panel h3 {
  font-size: 18px;
  font-weight: 600;
}
.rules-panel header p,
.rules-empty,
.rule-help {
  color: #8993a3;
  font-size: 13px;
  margin-top: 8px;
}
.rules-empty {
  padding: 24px 0;
}
.rule-row {
  display: flex;
  justify-content: space-between;
  gap: 20px;
  padding: 20px 0;
  border-bottom: 1px solid rgb(148 163 184 / 0.15);
}
.rule-row p {
  margin-top: 6px;
  color: #8993a3;
}
.rule-state {
  font-size: 12px;
  margin-left: 16px;
  color: #0aa987;
}
.rule-actions {
  display: flex;
  gap: 16px;
  align-items: center;
  flex-wrap: wrap;
  font-size: 13px;
  color: #7185e9;
}
.rule-actions button:disabled {
  opacity: 0.45;
}
.rule-form {
  display: grid;
  gap: 24px;
}
.rule-fields {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 18px;
}
.rule-form label {
  display: block;
  font-size: 14px;
}
.rule-form .input {
  margin-top: 8px;
  width: 100%;
}
.rule-section {
  border: 1px solid rgb(148 163 184 / 0.3);
  border-radius: 10px;
  padding: 20px;
  margin: 24px 0;
}
.rule-section legend {
  font-weight: 600;
  padding: 0 8px;
}
.rule-wide {
  grid-column: 1/-1;
}
.judge-switch {
  margin: 15px 0;
}
.judge-switch input,
.group-choice input {
  margin-right: 10px;
}
.group-choice {
  padding: 8px 0;
}
@media (max-width: 700px) {
  .rule-fields {
    grid-template-columns: 1fr;
  }
  .rule-row {
    flex-direction: column;
  }
  .rules-panel {
    padding: 16px;
  }
}
</style>

<style scoped>
.quality-source-choice { display: grid; gap: 6px; margin-top: 14px; }
</style>
