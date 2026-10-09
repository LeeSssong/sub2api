import { flushPromises, mount, RouterLinkStub } from "@vue/test-utils";
import { beforeEach, afterEach, describe, expect, it, vi } from "vitest";
import View from "../PelicanTestsView.vue";
const api = vi.hoisted(() => ({
  listPlans: vi.fn(),
  listResults: vi.fn(),
  getResult: vi.fn(),
  runPlan: vi.fn(),
}));
const rules = vi.hoisted(() => ({ save: vi.fn(), remove: vi.fn() }));
const groups = vi.hoisted(() => vi.fn());
vi.mock("@/api/admin/pelicanTests", () => ({ pelicanTestsAPI: api }));
vi.mock("@/api/admin/intelligenceRules", () => ({
  intelligenceRulesAPI: rules,
}));
vi.mock("@/api/admin/accountQuality", () => ({
  listQualityTemplates: vi.fn().mockResolvedValue([4, 5].map(group => ({
    id: group + 10, account_filter: { group: String(group) }, model_id: "gpt-6-astra",
    cron_expression: "*/5 * * * *", enabled: true,
    pelican_config: { question_kind: "candy", quality: { expected_answer: "21" }, model_ids: ["gpt-6-astra", "gpt-6.1-sol"] },
  }))),
}));
vi.mock("@/api", () => ({ adminAPI: { groups: { getAll: groups } } }));
vi.mock("vue-i18n", async () => ({
  ...(await vi.importActual<typeof import("vue-i18n")>("vue-i18n")),
  useI18n: () => ({ t: (key: string) => key }),
}));
const record = {
  id: 1,
  plan_id: 1,
  group_id: 4,
  group_name: "default",
  account_id: 0,
  account_name: "",
  attempts: [],
  status: "failed",
  error_message: "no_available_account",
  latency_ms: 10,
  cost_usd: 0,
  cost_incomplete: false,
  pelican_config: {
    question_kind: "candy",
    prompt: "question",
    model_id: "gpt-6-astra",
    reasoning_effort: "medium",
    parallel_count: 1,
  },
  started_at: "2026-10-05T02:00:00+08:00",
  finished_at: "",
  created_at: "",
};
let wrapper: ReturnType<typeof mount>;
const render = () =>
  mount(View, {
    global: {
      stubs: {
        AppLayout: { template: "<div><slot/></div>" },
        SmartOpsNav: true,
        RouterLink: RouterLinkStub,
        Pagination: true,
        PelicanArtworkPreview: true,
        BaseDialog: {
          props: ["show"],
          template: '<div v-if="show"><slot/><slot name="footer"/></div>',
        },
        ConfirmDialog: true,
      },
    },
  });
beforeEach(() => {
  vi.clearAllMocks();
  api.listPlans.mockResolvedValue([]);
  api.listResults.mockResolvedValue({ items: [record], total: 1 });
  groups.mockResolvedValue([
    { id: 4, name: "default", status: "active" },
    { id: 5, name: "second", status: "active" },
  ]);
  rules.save.mockResolvedValue({ id: "r" });
  api.getResult.mockResolvedValue({ ...record, response_text: "" });
});
afterEach(() => wrapper?.unmount());
describe("unified intelligence administration", () => {
  it("has a single create action and no legacy display or plan panels", async () => {
    wrapper = render();
    await flushPromises();
    expect(
      wrapper.findAll("button").filter((b) => b.text() === "创建规则"),
    ).toHaveLength(1);
    expect(wrapper.text()).not.toContain("兼容画图计划");
    expect(wrapper.text()).not.toContain("用户展示");
    expect(wrapper.getComponent(RouterLinkStub).props("to")).toBe(
      "/intelligence-test",
    );
  });
  it("creates a half-hour drawing rule with quality sources for multiple groups", async () => {
    wrapper = render();
    await flushPromises();
    await wrapper.get('[data-testid="pelican-tests-create"]').trigger("click");
    await flushPromises();
    await wrapper
      .get('[data-testid="intelligence-model"]')
      .setValue("gpt-6-astra");
    await wrapper.get('[data-testid="intelligence-group-4"]').setValue(true);
    await wrapper.get('[data-testid="intelligence-group-5"]').setValue(true);
    expect(wrapper.get('[data-testid="intelligence-cron"]').attributes("disabled")).toBeDefined();
    await wrapper.get("#intelligence-rule-form").trigger("submit");
    await flushPromises();
    expect(rules.save).toHaveBeenCalledWith(
      expect.objectContaining({
        group_ids: [4, 5],
        cron_expression: "*/30 * * * *",
        quality_sources: [{ group_id: 4, template_id: 14 }, { group_id: 5, template_id: 15 }],
        candy_models: ["gpt-6-astra", "gpt-6.1-sol"],
        drawing_prompt: expect.stringContaining("{动作}"),
      }),
      undefined,
    );
  });
  it("requires an applicable group before saving", async () => {
    wrapper = render();
    await flushPromises();
    await wrapper.get('[data-testid="pelican-tests-create"]').trigger("click");
    await flushPromises();
    await wrapper.get("#intelligence-rule-form").trigger("submit");
    expect(rules.save).not.toHaveBeenCalled();
    expect(wrapper.text()).toContain("请选择至少一个生效分组");
  });
  it("lets an administrator inspect failed requests without claiming artwork exists", async () => {
    wrapper = render();
    await flushPromises();
    await wrapper.get('[data-testid="pelican-result-view-1"]').trigger("click");
    await flushPromises();
    expect(wrapper.text()).toContain("no_available_account");
    expect(wrapper.text()).toContain("未收到模型回复。");
    expect(wrapper.find(".detail-art").exists()).toBe(false);
  });
});
