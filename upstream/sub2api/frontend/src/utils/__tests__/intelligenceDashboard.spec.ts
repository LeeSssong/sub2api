import { describe, expect, it } from "vitest";
import {
  intelligenceStats,
  intelligenceGroupStatus,
  intelligenceTimeline,
} from "../intelligenceDashboard";
import type { IntelligenceResult } from "@/api/intelligenceTests";
const sample = (
  verdict: IntelligenceResult["verdict"],
  id: number,
): IntelligenceResult => ({
  id,
  group_id: 1,
  model_id: "m",
  kind: "candy",
  verdict,
  latency_ms: 1000,
  attempts: 1,
  started_at: `2026-10-04T0${id}:00:00Z`,
  reasoning_effort: "medium",
});
describe("intelligence dashboard", () => {
  it("keeps request failures in the denominator and distinguishes wrong answers", () => {
    expect(
      intelligenceStats([
        sample("passed", 1),
        sample("incorrect", 2),
        sample("abnormal", 3),
      ]),
    ).toMatchObject({
      passed: 1,
      total: 3,
      abnormal: 1,
      incorrect: 1,
      percentage: "33.3",
      average: 1000,
    });
  });
  it("does not invent a healthy result for empty or abnormal groups", () => {
    expect(intelligenceGroupStatus([])).toBe("empty");
    expect(intelligenceGroupStatus([sample("abnormal", 1)])).toBe("abnormal");
  });
  it("sorts retained real samples chronologically without fake green fillers", () => {
    expect(
      intelligenceTimeline(
        [sample("passed", 3), sample("incorrect", 1)],
        "candy",
      ).map((r) => r.id),
    ).toEqual([1, 3]);
  });
});
