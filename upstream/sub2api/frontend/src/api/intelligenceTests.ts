import { apiClient } from "./client";
export interface IntelligenceResult {
  id: number;
  group_id: number;
  model_id: string;
  kind: "candy" | "pelican";
  verdict: "passed" | "incorrect" | "abnormal";
  error_kind?: string;
  prompt?: string;
  expected_answer?: string;
  response_text?: string;
  reasoning_effort: string;
  latency_ms: number;
  attempts: number;
  started_at: string;
  action?: string;
  scene?: string;
  first_token_ms?: number;
  input_tokens?: number;
  output_tokens?: number;
}
export interface IntelligenceGroup {
  id: number;
  name: string;
  description: string;
  platform: string;
  rate_multiplier: number;
  model_id: string;
  reasoning_effort: string;
  expected_answer: string;
  next_run_at?: string;
  running_started_at?: string;
  results: IntelligenceResult[];
}
export interface IntelligenceDashboard {
  generated_at?: string;
  enabled: boolean;
  groups: IntelligenceGroup[];
}
export const intelligenceTestsAPI = {
  async dashboard(signal?: AbortSignal) {
    return (
      await apiClient.get<IntelligenceDashboard>("/intelligence-tests", {
        signal,
      })
    ).data;
  },
  async result(id: number) {
    return (
      await apiClient.get<IntelligenceResult>(
        `/intelligence-tests/results/${id}`,
      )
    ).data;
  },
};
