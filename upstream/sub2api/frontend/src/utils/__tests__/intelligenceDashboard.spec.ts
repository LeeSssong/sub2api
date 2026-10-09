import { describe, expect, it } from "vitest";
import {
  intelligenceStats,
  intelligenceGroupStatus,
  intelligenceTimeline,
  intelligenceCandySamples,
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

const now = Date.parse("2026-10-09T12:20:00Z");
const models = ["gpt-6-astra", "gpt-6.1-sol"];
const fresh = (id: number, model: string, kind: "candy" | "pelican" = "candy") => ({
  ...sample("passed", id), model_id: model, kind, started_at: "2026-10-09T12:00:00Z",
});
describe("quality source status", () => {
  it("requires both configured logic models and drawing", () => {
    expect(intelligenceGroupStatus([fresh(1, models[0]), fresh(3, "draw", "pelican")], models, now)).toBe("partial");
    expect(intelligenceGroupStatus([fresh(1, models[0]), fresh(2, models[1]), fresh(3, "draw", "pelican")], models, now)).toBe("passed");
  });
  it("does not report retained old snapshots as current health", () => {
    const results = [fresh(1, models[0]), fresh(2, models[1]), fresh(3, "draw", "pelican")];
    expect(intelligenceGroupStatus(results, models, now + 60 * 60 * 1000)).toBe("stale");
  });
  it("counts each displayed model sample in the combined rate", () => {
    const results = [fresh(1, models[0]), {...fresh(2, models[1]), verdict: "incorrect" as const}, fresh(3, "other"), fresh(4, "draw", "pelican")];
    expect(intelligenceStats(intelligenceCandySamples(results, models, now, 24))).toMatchObject({ total: 2, passed: 1, percentage: "50" });
  });
  it("keeps unknown source judgments distinct from success", () => {
    expect(intelligenceGroupStatus([{...fresh(1, models[0]), verdict: "unknown" as const}, fresh(2, models[1]), fresh(3, "draw", "pelican")], models, now)).toBe("abnormal");
  });
  it("does not hide unavailable source configuration behind retained results", () => {
    expect(intelligenceGroupStatus([fresh(1, models[0]), fresh(2, models[1]), fresh(3, "draw", "pelican")], models, now, "missing")).toBe("partial");
  });
  it("isolates model timelines", () => {
    expect(intelligenceTimeline([fresh(1, models[0]), fresh(2, models[1])], "candy", models[1]).map(r => r.id)).toEqual([2]);
  });
});

describe('live candy monitoring', () => {
  it('counts every real test even within one half-hour slot', () => {
    const results = [fresh(1, models[0]), {...fresh(2, models[0]), started_at: '2026-10-09T12:05:00Z', verdict: 'incorrect' as const}];
    expect(intelligenceStats(intelligenceCandySamples(results, models, now, 24))).toMatchObject({total: 2, passed: 1});
  });
  it('keeps a daily candy result fresh until its next scheduled check', () => {
    const results = [
      {...fresh(1, models[0]), started_at: '2026-10-09T00:00:00Z', valid_until: '2026-10-10T00:10:00Z'},
      {...fresh(2, models[1]), started_at: '2026-10-09T00:00:00Z', valid_until: '2026-10-10T00:10:00Z'},
      fresh(3, 'draw', 'pelican'),
    ];
    expect(intelligenceGroupStatus(results, models, now)).toBe('passed');
  });
});
