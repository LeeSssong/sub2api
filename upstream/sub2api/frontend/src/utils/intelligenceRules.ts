import { CANDY_PROMPT } from "./intelligenceTest";
import type { IntelligenceRuleInput } from "@/api/admin/intelligenceRules";
import type { PelicanGroupTestPlan } from "@/api/admin/pelicanTests";
export const INTELLIGENCE_DRAWING_PROMPT =
  "请直接返回完整的单文件HTML代码：使用内联SVG绘制山姆奥特曼{动作}在{场景}的2D动画。SVG必须设置viewBox，并用SVG或CSS实现动画；不要使用canvas、JavaScript、外部资源或Markdown代码块。你不需要任何测试";
export function intelligenceRuleDefaults(): IntelligenceRuleInput {
  return {
    name: "分组智商监测",
    group_ids: [],
    model_id: "",
    reasoning_effort: "medium",
    cron_expression: "*/30 * * * *",
    enabled: true,
    candy_prompt: CANDY_PROMPT,
    expected_answer: "21",
    drawing_prompt: INTELLIGENCE_DRAWING_PROMPT,
    actions: ["坐着雪橇", "骑着摩托车", "乘着热气球", "划着小船", "乘着巴士"],
    scenes: ["游乐园里", "竹林里", "沙漠里", "彩虹下", "海边"],
  };
}
export function intelligenceRuleFromPlans(
  plans: PelicanGroupTestPlan[],
): IntelligenceRuleInput {
  const first = plans[0];
  const rule = first.pelican_config.intelligence;
  if (!rule) throw new Error("不是双题型规则");
  return {
    name: rule.name,
    group_ids: plans.map((p) => p.group_id),
    model_id: first.model_id,
    reasoning_effort: first.pelican_config.reasoning_effort,
    cron_expression: first.cron_expression,
    enabled: first.enabled,
    candy_prompt: rule.candy.prompt,
    expected_answer: rule.candy.quality?.expected_answer || "21",
    judge: rule.candy.quality?.judge
      ? { ...rule.candy.quality.judge }
      : undefined,
    drawing_prompt: first.pelican_config.prompt,
    actions: [...rule.actions],
    scenes: [...rule.scenes],
  };
}
