import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, shallowMount } from '@vue/test-utils'

import OpsErrorDetailModal from '../OpsErrorDetailModal.vue'
import { observerUsageContext } from '@/components/admin/usage/observerUsageContext'

const { getRequestErrorDetail, getUpstreamErrorDetail, listRequestErrorUpstreamErrors, own } = vi.hoisted(() => ({
  getRequestErrorDetail: vi.fn(),
  getUpstreamErrorDetail: vi.fn(),
  listRequestErrorUpstreamErrors: vi.fn(),
  own: vi.fn(),
}))

vi.mock('@/api/observerUsage', () => ({ observerUsageAPI: { getErrorDetail: own } }))
vi.mock('@/api/admin/ops', () => ({
  opsAPI: { getRequestErrorDetail, getUpstreamErrorDetail, listRequestErrorUpstreamErrors },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({ showError: vi.fn() }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
    t: (key: string) => ({
      'admin.ops.errorDetail.adminDiagnosis': '管理员诊断',
      'admin.ops.errorDetail.noUpstreamSelected': '未选择上游',
      'admin.ops.errorDetail.selectedUpstream': '已选择上游',
      'admin.ops.errorDetail.diagnosisClass': '分类/代码',
      'admin.ops.errorDetail.diagnosisStage': '阶段',
      'admin.ops.errorDetail.diagnosisOwner': '归属',
      'admin.ops.errorDetail.originalUpstreamStatus': '原始上游状态',
      'admin.ops.errorDetail.originalUpstreamMessage': '原始上游消息',
      'admin.ops.errorDetail.originalUpstreamDetail': '原始上游详情',
    })[key] ?? key,
    }),
  }
})

const BaseDialogStub = {
  name: 'BaseDialog',
  props: ['show', 'title'],
  emits: ['close'],
  template: '<div v-if="show"><slot /></div>',
}

function makeDetail(selected: boolean) {
  return {
    id: 12,
    created_at: '2026-08-12T00:00:00Z',
    phase: selected ? 'upstream' : 'request',
    type: selected ? 'upstream_error' : 'invalid_request_error',
    error_owner: selected ? 'provider' : 'client',
    error_source: selected ? 'upstream_http' : 'client_request',
    severity: 'P1',
    status_code: selected ? 503 : 400,
    platform: 'openai',
    model: 'gpt-5',
    resolved: false,
    client_request_id: '',
    request_id: 'req-12',
    message: selected ? 'Upstream request failed' : 'Failed to read request body',
    user_email: 'user@example.com',
    account_id: selected ? 7 : null,
    account_name: selected ? 'provider-a' : '',
    group_id: selected ? 9 : null,
    group_name: selected ? 'paid' : '',
    error_body: '',
    is_business_limited: false,
    diagnosis: {
      class: selected ? 'upstream_failed' : 'upload_interrupted',
      code: selected ? 'UPSTREAM_FAILED' : 'UPLOAD_INTERRUPTED',
      stage: selected ? 'upstream' : 'request',
      ownership: selected ? 'provider' : 'client',
      upstream_account_selected: selected,
      selected_account_id: selected ? 7 : undefined,
      selected_account_name: selected ? 'provider-a' : undefined,
      group_id: selected ? 9 : undefined,
      group_name: selected ? 'paid' : undefined,
      original_upstream_status: selected ? 503 : undefined,
      original_upstream_message: selected ? 'provider unavailable' : undefined,
      original_upstream_detail: selected ? '{"message":"maintenance"}' : undefined,
    },
  }
}

function mountModal(errorType: 'request' | 'upstream') {
  return mount(OpsErrorDetailModal, {
    props: { show: true, errorId: 12, errorType },
    global: { stubs: { BaseDialog: BaseDialogStub, Icon: true } },
  })
}

describe('OpsErrorDetailModal diagnosis', () => {
  beforeEach(() => {
    getRequestErrorDetail.mockReset()
    getUpstreamErrorDetail.mockReset()
    listRequestErrorUpstreamErrors.mockReset()
    listRequestErrorUpstreamErrors.mockResolvedValue({ items: [] })
  })

  it('renders not-selected upload diagnosis in the existing request detail modal', async () => {
    getRequestErrorDetail.mockResolvedValue(makeDetail(false))
    const wrapper = mountModal('request')
    await flushPromises()

    const diagnosis = wrapper.get('[data-testid="admin-error-diagnosis"]')
    expect(diagnosis.text()).toContain('管理员诊断')
    expect(diagnosis.text()).toContain('upload_interrupted')
    expect(diagnosis.text()).toContain('UPLOAD_INTERRUPTED')
    expect(diagnosis.text()).toContain('未选择上游')
  })

  it('shows administrator evidence separately from the projected client response', async () => {
    const rawDetail = makeDetail(true)
    rawDetail.error_body = '{"error":{"message":"服务暂时异常，请稍后重试。"}}'
    rawDetail.upstream_error_detail = 'X-Goog-Api-Key: raw-upstream-secret'
    getUpstreamErrorDetail.mockResolvedValue(rawDetail)
    const wrapper = mountModal('upstream')
    await flushPromises()

    const diagnosis = wrapper.get('[data-testid="admin-error-diagnosis"]')
    expect(diagnosis.text()).toContain('已选择上游')
    expect(diagnosis.text()).toContain('provider-a')
    expect(diagnosis.text()).toContain('paid')
    expect(diagnosis.text()).toContain('provider unavailable')
    expect(diagnosis.text()).toContain('maintenance')
    expect(wrapper.text()).toContain('服务暂时异常，请稍后重试。')
    expect(wrapper.text()).toContain('raw-upstream-secret')
  })
  it('shows the upstream payload verbatim without JSON reformatting', async () => {
    const payload = ' {\n  "error":{"message":"Encrypted output cannot be decoded","code":"thinking_signature_invalid"}\n}\n'
    getUpstreamErrorDetail.mockResolvedValue({ ...makeDetail(true), upstream_error_detail: payload })
    const wrapper = mountModal('upstream')
    await flushPromises()
    expect(wrapper.findAll('pre code').some(node => node.element.textContent === payload)).toBe(true)
  })

})

it('loads only the owned observer error and never requests correlated admin details', async () => {
  vi.clearAllMocks()
  own.mockResolvedValue({ id: 42, status_code: 502, message: 'own error' })
  const wrapper = shallowMount(OpsErrorDetailModal, {
    props: { show: true, errorId: 42, errorType: 'request' },
    global: { provide: { [observerUsageContext as symbol]: true } },
  })
  await flushPromises()
  expect(own).toHaveBeenCalledWith(42)
  expect(getRequestErrorDetail).not.toHaveBeenCalled()
  expect(listRequestErrorUpstreamErrors).not.toHaveBeenCalled()
  wrapper.unmount()
})

it.each(['user', 'upstream'])('explains %s balance failures and preserves diagnostics', async source => {
  vi.clearAllMocks()
  mocks.listRequestErrorUpstreamErrors.mockResolvedValue({ items: [] })
  mocks.getRequestErrorDetail.mockResolvedValue({
    id: 1, status_code: 403, phase: 'request',
    error_owner: source === 'user' ? 'client' : 'provider',
    error_source: source === 'user' ? 'client_request' : 'upstream_http',
    user_id: 7, account_id: source === 'upstream' ? 9 : null,
    message: 'insufficient balance', error_body: '{"error":{"message":"insufficient balance"}}',
  })
  const wrapper = shallowMount(OpsErrorDetailModal, {
    props: { show: true, errorId: 1, errorType: 'request' },
    global: { stubs: { BaseDialog: { template: '<div><slot /></div>' }, Icon: true } },
  })
  await flushPromises()
  expect(wrapper.get('[data-testid="balance-source"]').text()).toContain(`admin.ops.balanceError.${source}Hint`)
  expect(wrapper.find('pre').text()).toContain('insufficient balance')
  wrapper.unmount()
})
