import { describe, expect, it } from "vitest";
import {
  intelligenceRuleDefaults,
  intelligenceRuleFromPlans,
} from "../intelligenceRules";
import type { PelicanGroupTestPlan } from "@/api/admin/pelicanTests";
describe("intelligence rules", () => {
  it("defaults to the existing candy and exact approved drawing template", () => {
    const input = intelligenceRuleDefaults();
    expect(input.expected_answer).toBe("21");
    expect(input.candy_prompt).toContain("圆形 7 9 8");
    expect(input.drawing_prompt).toContain("山姆奥特曼{动作}在{场景}");
    expect(input.group_ids).toEqual([]);
  });
  it("edits all groups of a rule without mutating saved settings", () => {
    const config = {
      prompt: "draw {动作} {场景}",
      reasoning_effort: "medium",
      parallel_count: 1,
      intelligence: {
        id: "r",
        name: "Rule",
        actions: ["a"],
        scenes: ["s"],
        candy: {
          prompt: "question",
          reasoning_effort: "medium",
          parallel_count: 1,
          quality: {
            expected_answer: "42",
            action: "observe_only",
            judge: { group_id: 4, model_id: "judge", prompt: "compare" },
          },
        },
      },
    };
    const plans = [
      {
        group_id: 1,
        model_id: "model",
        cron_expression: "*/30 * * * *",
        enabled: true,
        pelican_config: config,
      },
      { group_id: 2, pelican_config: config },
    ] as PelicanGroupTestPlan[];
    const draft = intelligenceRuleFromPlans(plans);
    expect(draft.group_ids).toEqual([1, 2]);
    expect(draft.expected_answer).toBe("42");
    draft.actions.push("b");
    expect(config.intelligence.actions).toEqual(["a"]);
  });
});
