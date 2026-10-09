import type { IntelligenceResult } from "@/api/intelligenceTests";
export function intelligenceTimeline(
  results: IntelligenceResult[],
  kind: IntelligenceResult["kind"],
  modelId?: string,
) {
  return results
    .filter((r) => r.kind === kind && (!modelId || r.model_id === modelId))
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
export function intelligenceGroupStatus(results: IntelligenceResult[], models?: string[], now?: number, sourceStatus?: string) {
  const required = models?.length ? models : [...new Set(results.filter(r => r.kind === "candy").map(r => r.model_id))];
  const latest = [
    ...required.map(model => intelligenceTimeline(results, "candy", model).at(-1)),
    intelligenceTimeline(results, "pelican").at(-1),
  ];
  const present = latest.filter((r): r is IntelligenceResult => !!r);
  if (!present.length) return "empty";
  // Current or immediately preceding display slot may still be in flight.
  if (now != null && present.some(r => !Number.isFinite(Date.parse(r.started_at)) || now - Date.parse(r.started_at) >= 2 * INTELLIGENCE_SLOT_MS)) return "stale";
  if (present.some(r => r.verdict === "incorrect")) return "incorrect";
  if (present.some(r => r.verdict === "abnormal" || r.verdict === "unknown")) return "abnormal";
  if (sourceStatus === "missing") return "partial";
  return required.length > 0 && present.length === latest.length ? "passed" : "partial";
}
export function intelligenceCandySamples(results: IntelligenceResult[], models: string[], now: number, hours: 24 | 72) {
  return models.flatMap(model => intelligenceSlots(intelligenceTimeline(results, "candy", model), "candy", now, hours).flatMap(s => s.result ? [s.result] : []));
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
  unknown: "判定未知",
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
