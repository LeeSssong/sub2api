import { mount, flushPromises, type VueWrapper } from '@vue/test-utils';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { createI18n } from 'vue-i18n';
import IntelligenceRulesPanel from '../IntelligenceRulesPanel.vue';
import type { PelicanGroupTestPlan } from '@/api/admin/pelicanTests';
import { intelligenceRulesAPI } from '@/api/admin/intelligenceRules';

vi.mock('@/api', () => ({
  adminAPI: { groups: { getAll: vi.fn().mockResolvedValue([
    { id: 1, name: 'GPT-Pro20x', status: 'active' },
    { id: 2, name: 'GPT-Pro5x', status: 'active' },
  ]) } },
}));
vi.mock('@/api/admin/intelligenceRules', () => ({
  intelligenceRulesAPI: { save: vi.fn().mockResolvedValue({ id: 'rule-1' }), remove: vi.fn() },
}));

function plans(running = false): PelicanGroupTestPlan[] {
  return [1, 2].map((group_id) => ({
    id: group_id,
    group_id,
    group_name: group_id === 1 ? 'GPT-Pro20x' : 'GPT-Pro5x',
    group_platform: 'openai',
    group_status: 'active',
    today_cost_usd: 0,
    total_cost_usd: 0,
    today_cost_incomplete: false,
    total_cost_incomplete: false,
    model_id: 'gpt-6-astra',
    cron_expression: '*/2 * * * *',
    enabled: true,
    pelican_config: {
      prompt: 'draw {动作} {场景}', reasoning_effort: 'medium', parallel_count: 1,
      intelligence: {
        id: 'rule-1', name: '分组智商监测', actions: ['划船'], scenes: ['海边'],
        candy: {
          prompt: 'custom question', reasoning_effort: 'medium', parallel_count: 1,
          quality: {
            expected_answer: '42', action: 'observe_only',
            judge: { group_id: 1, model_id: 'judge', prompt: 'compare' },
          },
        },
      },
    },
    last_run_at: null,
    next_run_at: null,
    running_until: running && group_id === 2 ? new Date(Date.now() + 60_000).toISOString() : null,
    created_at: '2026-10-06T00:00:00Z',
    updated_at: '2026-10-06T00:00:00Z',
  }));
}

let wrapper: VueWrapper;
const global = {
  stubs: { transition: true },
  plugins: [createI18n({ legacy: false, locale: 'en', messages: { en: { common: { cancel: 'Cancel', confirm: 'Confirm' } } } })],
};
afterEach(() => {
  wrapper?.unmount();
  document.body.innerHTML = '';
  vi.clearAllMocks();
});

async function edit() {
  await wrapper.findAll('.rule-actions button').find((b) => b.text() === '编辑')!.trigger('click');
  await flushPromises();
}

describe('intelligence rule editing', () => {
  it('saves edits while a group is running and keeps the current run controls protected', async () => {
    wrapper = mount(IntelligenceRulesPanel, { props: { plans: plans(true) }, global, attachTo: document.body });
    const buttons = wrapper.findAll('.rule-actions button');
    expect((buttons[0].element as HTMLButtonElement).disabled).toBe(true);
    expect((buttons[3].element as HTMLButtonElement).disabled).toBe(true);
    await edit();
    expect(document.querySelector('[role="dialog"]')!.textContent).toContain('编辑检测规则');
    const name = document.querySelector<HTMLInputElement>('[data-testid="intelligence-name"]')!;
    expect(name.value).toBe('分组智商监测');
    expect(document.querySelector<HTMLInputElement>('[data-testid="intelligence-group-1"]')!.checked).toBe(true);
    expect(document.querySelector<HTMLInputElement>('[data-testid="intelligence-group-2"]')!.checked).toBe(true);
    expect(document.querySelector('[role="dialog"] [role="status"]')?.textContent).toContain('下一个周期生效');
    expect(document.querySelector<HTMLButtonElement>('[data-testid="intelligence-save"]')!.disabled).toBe(false);
    name.value = 'Updated rule';
    name.dispatchEvent(new Event('input', { bubbles: true }));
    document.querySelector<HTMLButtonElement>('[data-testid="intelligence-save"]')!.click();
    await flushPromises();
    expect(intelligenceRulesAPI.save).toHaveBeenCalledWith(expect.objectContaining({ name: 'Updated rule', group_ids: [1, 2] }), 'rule-1');
    expect(document.querySelector('[role="dialog"]')).toBeNull();
    expect(wrapper.emitted('changed')).toHaveLength(1);
  });

  it('pauses a running rule without waiting for the current run to finish', async () => {
    wrapper = mount(IntelligenceRulesPanel, { props: { plans: plans(true) }, global, attachTo: document.body });
    const pause = wrapper.findAll('.rule-actions button').find((b) => b.text() === '暂停')!;
    expect((pause.element as HTMLButtonElement).disabled).toBe(false);
    await pause.trigger('click');
    await flushPromises();
    expect(intelligenceRulesAPI.save).toHaveBeenCalledWith(expect.objectContaining({ enabled: false, group_ids: [1, 2] }), 'rule-1');
    expect(wrapper.emitted('changed')).toHaveLength(1);
  });
});
