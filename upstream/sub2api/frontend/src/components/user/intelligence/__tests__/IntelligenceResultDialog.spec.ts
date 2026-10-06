import { mount } from "@vue/test-utils";
import { describe, it, expect, vi } from "vitest";
import Dialog from "../IntelligenceResultDialog.vue";
import type {
  IntelligenceGroup,
  IntelligenceResult,
} from "@/api/intelligenceTests";
Object.defineProperty(HTMLDialogElement.prototype, "showModal", {
  configurable: true,
  value: vi.fn(),
});
const result: IntelligenceResult = {
  id: 1,
  group_id: 4,
  model_id: "m",
  kind: "candy",
  verdict: "incorrect",
  latency_ms: 29000,
  attempts: 1,
  started_at: "2026-10-04T12:00:00Z",
  reasoning_effort: "medium",
  expected_answer: "21",
  response_text:
    '**29**<img src="https://untrusted.example/track"><script>alert(1)</script>',
};
const group: IntelligenceGroup = {
  id: 4,
  name: "group",
  description: "",
  platform: "openai",
  rate_multiplier: 1,
  model_id: "m",
  reasoning_effort: "medium",
  expected_answer: "21",
  results: [result],
};
describe("intelligence detail", () => {
  it("renders model text without executable HTML or automatic external media requests", () => {
    const wrapper = mount(Dialog, {
      props: { group, summary: result, result, loading: false, error: "" },
    });
    expect(wrapper.find(".iq-answer strong").text()).toBe("29");
    expect(wrapper.find(".iq-answer script").exists()).toBe(false);
    expect(wrapper.find(".iq-answer img").exists()).toBe(false);
    expect(wrapper.text()).toContain("失败 · 答案错误");
    expect(wrapper.text()).toContain("— / —");
    wrapper.unmount();
  });
});
