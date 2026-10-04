import type { IntelligenceResult } from "@/api/intelligenceTests";
export function intelligenceTimeline(
  results: IntelligenceResult[],
  kind: IntelligenceResult["kind"],
) {
  return results
    .filter((r) => r.kind === kind)
    .sort(
      (a, b) =>
        Date.parse(a.started_at) - Date.parse(b.started_at) || a.id - b.id,
    );
}
export function intelligenceStats(results: IntelligenceResult[]) {
  const passed = results.filter((r) => r.verdict === "passed").length;
  const times = results
    .filter((r) => r.verdict !== "abnormal" && r.latency_ms > 0)
    .map((r) => r.latency_ms);
  const percent = results.length ? (passed / results.length) * 100 : 0;
  return {
    passed,
    total: results.length,
    abnormal: results.filter((r) => r.verdict === "abnormal").length,
    incorrect: results.filter((r) => r.verdict === "incorrect").length,
    percentage: results.length
      ? percent.toFixed(Number.isInteger(percent) ? 0 : 1)
      : "—",
    average: times.length
      ? times.reduce((a, b) => a + b, 0) / times.length
      : null,
  };
}
export function intelligenceGroupStatus(results: IntelligenceResult[]) {
  const latest = (["candy", "pelican"] as const)
    .map((kind) => intelligenceTimeline(results, kind).at(-1))
    .filter((r): r is IntelligenceResult => !!r);
  if (!latest.length) return "empty";
  if (latest.some((r) => r.verdict === "incorrect")) return "incorrect";
  if (latest.some((r) => r.verdict === "abnormal")) return "abnormal";
  return latest.length === 2 ? "passed" : "partial";
}
export function intelligenceDuration(ms?: number | null) {
  if (ms == null || ms <= 0) return "—";
  const s = Math.round(ms / 1000);
  return s >= 60 ? `${Math.floor(s / 60)}m ${s % 60}s` : `${s} s`;
}
export function intelligenceDate(date: string) {
  const d = new Date(date);
  const n = (v: number) => String(v).padStart(2, "0");
  return `${n(d.getMonth() + 1)}–${n(d.getDate())} ${n(d.getHours())}:${n(d.getMinutes())}`;
}
export const intelligenceVerdictLabel = {
  passed: "通过",
  incorrect: "失败 · 答案错误",
  abnormal: "请求异常",
};

export interface IntelligenceSlot {
  timestamp: number;
  status: IntelligenceResult["verdict"] | "empty" | "running";
  result?: IntelligenceResult;
}
export const INTELLIGENCE_SLOT_MS = 30 * 60 * 1000;
export function intelligenceSlots(
  results: IntelligenceResult[],
  kind: IntelligenceResult["kind"],
  now: number,
  hours: 24 | 72,
  runningStartedAt?: string,
): IntelligenceSlot[] {
  const count = hours * 2;
  const last = Math.floor(now / INTELLIGENCE_SLOT_MS) * INTELLIGENCE_SLOT_MS;
  const first = last - (count - 1) * INTELLIGENCE_SLOT_MS;
  const slots: IntelligenceSlot[] = Array.from({ length: count }, (_, i) => ({
    timestamp: first + i * INTELLIGENCE_SLOT_MS,
    status: "empty",
  }));
  for (const result of results) {
    if (result.kind !== kind) continue;
    const index = Math.floor(
      (Date.parse(result.started_at) - first) / INTELLIGENCE_SLOT_MS,
    );
    const slot = slots[index];
    if (!slot) continue;
    if (
      !slot.result ||
      Date.parse(result.started_at) > Date.parse(slot.result.started_at) ||
      (result.started_at === slot.result.started_at &&
        result.id > slot.result.id)
    ) {
      slot.result = result;
      slot.status = result.verdict;
    }
  }
  if (runningStartedAt) {
    const started = Date.parse(runningStartedAt);
    const slot = slots[Math.floor((started - first) / INTELLIGENCE_SLOT_MS)];
    if (
      slot &&
      (!slot.result || Date.parse(slot.result.started_at) < started)
    ) {
      slot.status = "running";
      slot.result = undefined;
    }
  }
  return slots;
}
export function intelligenceWindow(
  results: IntelligenceResult[],
  now: number,
  hours: 24 | 72,
) {
  const first =
    Math.floor(now / INTELLIGENCE_SLOT_MS) * INTELLIGENCE_SLOT_MS -
    (hours * 2 - 1) * INTELLIGENCE_SLOT_MS;
  return results.filter(
    (r) => Date.parse(r.started_at) >= first && Date.parse(r.started_at) <= now,
  );
}
