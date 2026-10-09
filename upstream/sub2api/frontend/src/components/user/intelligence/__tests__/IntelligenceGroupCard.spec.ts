import { mount } from '@vue/test-utils';
import { describe, it, expect, vi } from 'vitest';
import Card from '../IntelligenceGroupCard.vue';
import Timeline from '../IntelligenceTimeline.vue';
import type { IntelligenceGroup, IntelligenceResult } from '@/api/intelligenceTests';
vi.mock('@/api/intelligenceTests', () => ({ intelligenceTestsAPI: { result: vi.fn().mockResolvedValue({ response_text: '' }) } }));
const now = Date.parse('2026-10-09T12:20:00Z');
const result = (id: number, model_id: string, kind: 'candy' | 'pelican'): IntelligenceResult => ({ id, group_id: 1, model_id, kind, verdict: 'passed', latency_ms: 1000, attempts: 1, reasoning_effort: model_id === 'gpt-6.1-sol' ? 'high' : 'medium', started_at: '2026-10-09T12:00:00Z' });
const group: IntelligenceGroup = { id: 1, name: 'Test group', description: '', platform: 'openai', rate_multiplier: 1, model_id: 'draw-model', candy_model_ids: ['gpt-6-astra', 'gpt-6.1-sol'], quality_template_id: 9, reasoning_effort: 'medium', expected_answer: '', results: [result(1, 'gpt-6-astra', 'candy'), result(2, 'gpt-6.1-sol', 'candy'), result(3, 'draw-model', 'pelican')] };
describe('quality reuse group layout', () => {
  it('keeps isolated compact model rows under one logic heading and artwork inside drawing region', () => {
    const wrapper = mount(Card, { props: { group, now, hours: 24 } });
    expect(wrapper.findAll('.iq-logic-region .iq-test-name')).toHaveLength(1);
    const rows = wrapper.find('.iq-logic-region').findAllComponents(Timeline);
    expect(rows).toHaveLength(2);
    expect(rows.map(row => row.props('samples').map((r: IntelligenceResult) => r.model_id))).toEqual([['gpt-6-astra'], ['gpt-6.1-sol']]);
    expect(wrapper.find('.iq-drawing-region aside .iq-art').exists()).toBe(true);
    expect(wrapper.find('.iq-logic-region aside').exists()).toBe(false);
    expect(wrapper.find('.iq-heading').text()).not.toContain('draw-model');
    expect(wrapper.text()).toContain('HIGH');
    wrapper.unmount();
  });
});
