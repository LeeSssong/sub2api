import { mount } from "@vue/test-utils";
import { describe, it, expect } from "vitest";
import Timeline from "../IntelligenceTimeline.vue";
import type { IntelligenceResult } from "@/api/intelligenceTests";
const now = Date.parse("2026-10-05T02:03:00+08:00");
const result: IntelligenceResult = {
  id: 1,
  group_id: 1,
  model_id: "m",
  kind: "candy",
  verdict: "abnormal",
  latency_ms: 0,
  attempts: 0,
  reasoning_effort: "medium",
  started_at: "2026-10-05T02:00:00+08:00",
};
describe("intelligence timeline rendering", () => {
  it("renders 47 empty cells and one clickable result in a fixed 48 column grid", async () => {
    const wrapper = mount(Timeline, {
      props: {
        kind: "candy",
        samples: [result],
        expectedAnswer: "21",
        now,
        hours: 24,
      },
      global: { stubs: { Icon: true } },
    });
    expect(wrapper.findAll(".iq-bar")).toHaveLength(48);
    expect(wrapper.findAll(".iq-bar.empty")).toHaveLength(47);
    expect(wrapper.findAll("button.iq-bar")).toHaveLength(1);
    expect(wrapper.get(".iq-bars").attributes("style")).toContain(
      "--iq-slot-count: 48",
    );
    await wrapper.get("button.iq-bar").trigger("click");
    expect(wrapper.emitted("select")?.[0]).toEqual([result]);
    wrapper.unmount();
  });
});
