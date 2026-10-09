import { describe, it, expect } from "vitest";
import { intelligenceSlots } from "../intelligenceDashboard";
import type { IntelligenceResult } from "@/api/intelligenceTests";
const now = Date.parse("2026-10-05T02:03:00+08:00");
const record: IntelligenceResult = {
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
describe("fixed intelligence time slots", () => {
  it("puts a single failed test in one of 48 slots without widening it", () => {
    const slots = intelligenceSlots([record], "candy", now, 24);
    expect(slots).toHaveLength(48);
    expect(slots.filter((s) => s.status === "empty")).toHaveLength(47);
    expect(slots.at(-1)?.result?.id).toBe(1);
  });
  it("uses 144 half-hour slots for three days and keeps old results outside 24 hours", () => {
    const old = { ...record, started_at: "2026-10-03T03:00:00+08:00" };
    expect(
      intelligenceSlots([old], "candy", now, 24).every(
        (s) => s.status === "empty",
      ),
    ).toBe(true);
    expect(
      intelligenceSlots([old], "candy", now, 72).filter((s) => s.result),
    ).toHaveLength(1);
  });
  it("only marks a running slot from an active run and keeps placeholders out of statistics", () => {
    const slots = intelligenceSlots(
      [],
      "pelican",
      now,
      24,
      "2026-10-05T02:01:00+08:00",
    );
    expect(slots.at(-1)?.status).toBe("running");
    expect(slots.filter((s) => s.result)).toHaveLength(0);
  });
});

it('keeps completed evidence clickable while a later run in the same slot is pending', () => {
 const slots = intelligenceSlots([record], 'candy', now, 24, '2026-10-05T02:02:00+08:00');
 expect(slots.at(-1)?.status).toBe('running');
 expect(slots.at(-1)?.result?.id).toBe(1);
});
