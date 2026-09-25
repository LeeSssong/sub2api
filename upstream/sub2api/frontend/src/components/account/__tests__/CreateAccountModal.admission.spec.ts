import { defineComponent } from 'vue'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

enableAutoUnmount(afterEach)

const {
  createAccountMock,
  probeUpstreamBillingMock,
  syncUpstreamModelsMock,
  showWarningMock,
  showErrorMock,
  generateAuthUrlMock,
  exchangeCodeMock,
  refreshTokenMock,
  grokSSOCreateMock,
  importCodexSessionMock,
  createOpenAICodexPATMock,
  authIsSimpleMode,
} = vi.hoisted(() => ({
  createAccountMock: vi.fn(),
  probeUpstreamBillingMock: vi.fn(),
  syncUpstreamModelsMock: vi.fn(),
  showWarningMock: vi.fn(),
  showErrorMock: vi.fn(),
  generateAuthUrlMock: vi.fn(),
  exchangeCodeMock: vi.fn(),
  refreshTokenMock: vi.fn(),
  grokSSOCreateMock: vi.fn(),
  importCodexSessionMock: vi.fn(),
  createOpenAICodexPATMock: vi.fn(),
  authIsSimpleMode: { value: true },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: showErrorMock,
    showSuccess: vi.fn(),
    showWarning: showWarningMock,
  }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    get isSimpleMode() {
      return authIsSimpleMode.value
    },
  }),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      create: createAccountMock,
      generateAuthUrl: generateAuthUrlMock,
      exchangeCode: exchangeCodeMock,
      refreshOpenAIToken: refreshTokenMock,
      probeUpstreamBilling: probeUpstreamBillingMock,
      syncUpstreamModels: syncUpstreamModelsMock,
      checkMixedChannelRisk: vi.fn().mockResolvedValue({ has_risk: false }),
      importCodexSession: importCodexSessionMock,
      createOpenAICodexPAT: createOpenAICodexPATMock,
    },
    gemini: { generateAuthUrl: generateAuthUrlMock, exchangeCode: exchangeCodeMock, getCapabilities: vi.fn().mockResolvedValue({}) },
    antigravity: { generateAuthUrl: generateAuthUrlMock, exchangeCode: exchangeCodeMock, refreshAntigravityToken: refreshTokenMock },
    grok: { generateAuthUrl: generateAuthUrlMock, exchangeCode: exchangeCodeMock, refreshGrokToken: refreshTokenMock, createFromSSO: grokSSOCreateMock },
    settings: {
      getWebSearchEmulationConfig: vi.fn().mockResolvedValue({ enabled: false, providers: [] }),
      getSettings: vi.fn().mockResolvedValue({}),
    },
    tlsFingerprintProfiles: {
      list: vi.fn().mockResolvedValue([]),
    },
  },
}))

vi.mock('@/api/admin/accounts', () => ({
  getAntigravityDefaultModelMapping: vi.fn().mockResolvedValue([]),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

import CreateAccountModal from '../CreateAccountModal.vue'


const BaseDialogStub = defineComponent({
  name: 'BaseDialog',
  props: { show: { type: Boolean, default: false } },
  template: '<div v-if="show"><slot /><slot name="footer" /></div>',
})

const OAuthAuthorizationFlowStub = defineComponent({
  name: 'OAuthAuthorizationFlow',
  props: {
    showManualOption: Boolean,
    showCodexSessionImportOption: Boolean,
    showAgentIdentityOption: Boolean,
    showCodexPatOption: Boolean,
    initialInputMethod: String,
  },
  data: () => ({ inputMethod: 'manual', authCode: 'one-time-code', oauthState: 'state' }),
  methods: { reset() {} },
  emits: ['import-codex-session', 'import-codex-pat', 'generate-url', 'import-sso', 'validate-refresh-token', 'cookie-auth'],
  template: `
    <div>
      <button data-testid="import-codex-session" @click="$emit('import-codex-session', 'session-json')">session</button>
      <button data-testid="import-codex-pat" @click="$emit('import-codex-pat', 'pat-token')">pat</button>
    </div>
  `,
})

const GroupSelectorStub = defineComponent({
  name: 'GroupSelector',
  props: {
    modelValue: {
      type: Array,
      default: () => [],
    },
  },
  emits: ['update:modelValue'],
  template: `
    <button
      type="button"
      data-testid="select-pricing-groups"
      @click="$emit('update:modelValue', [1, 2])"
    >
      groups
    </button>
  `,
})

const ModelWhitelistSelectorStub = defineComponent({
  name: 'ModelWhitelistSelector',
  props: {
    modelValue: {
      type: Array,
      default: () => [],
    },
    platform: String,
    syncCredentials: Object,
  },
  emits: ['update:modelValue', 'upstream-synced'],
  template: `<button
    type="button"
    data-testid="model-whitelist-selector"
    @click="$emit('update:modelValue', ['public-glm']); $emit('upstream-synced')"
  >models</button>`,
})

function mountModal(groups: any[] = []) {
  return mount(CreateAccountModal, {
    props: { show: true, proxies: [], groups },
    global: {
      stubs: {
        BaseDialog: BaseDialogStub,
        OAuthAuthorizationFlow: OAuthAuthorizationFlowStub,
        ConfirmDialog: true,
        Select: true,
        Icon: true,
        PlatformIcon: true,
        ProxySelector: true,
        ProxyAdBanner: true,
        GroupSelector: GroupSelectorStub,
        ModelWhitelistSelector: ModelWhitelistSelectorStub,
        QuotaLimitCard: true,
      },
    },
  })
}

async function selectButtonByText(wrapper: ReturnType<typeof mountModal>, text: string) {
  const button = wrapper.findAll('button').find((candidate) => candidate.text().includes(text))
  expect(button).toBeDefined()
  await button?.trigger('click')
  await flushPromises()
}

describe('CreateAccountModal admission', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    authIsSimpleMode.value = true
    createAccountMock.mockResolvedValue({ id: 42, platform: 'openai', type: 'oauth' })
    importCodexSessionMock.mockResolvedValue({ created: 1, updated: 0, skipped: 0, failed: 0 })
    createOpenAICodexPATMock.mockResolvedValue({ id: 42 })
    generateAuthUrlMock.mockResolvedValue({ auth_url: 'https://oauth.example/?state=state', session_id: 'session', state: 'state' })
    exchangeCodeMock.mockResolvedValue({ access_token: 'token', refresh_token: 'refresh' })
    refreshTokenMock.mockResolvedValue({ access_token: 'token', refresh_token: 'refresh' })
    grokSSOCreateMock.mockResolvedValue({ created: [{ index: 0, account: { id: 42 } }], failed: [] })
  })

  const admissionGroups = [
    { id: 1, name: 'Formal A', platform: 'openai', status: 'active' },
    { id: 2, name: 'Formal B', platform: 'openai', status: 'active' },
    { id: 3, name: 'Temporary', platform: 'openai', status: 'active' },
    { id: 4, name: 'Claude temporary', platform: 'anthropic', status: 'active' },
    { id: 5, name: 'Disabled', platform: 'openai', status: 'inactive' },
  ]

  async function prepareAdmission(platform = 'openai') {
    const wrapper = mountModal(admissionGroups.map(group => group.id <= 3 ? { ...group, platform } : group))
    const buttonText = { openai: 'OpenAI', anthropic: 'Anthropic', gemini: 'Gemini', antigravity: 'Antigravity', grok: 'Grok' }[platform]!
    await selectButtonByText(wrapper, buttonText)
    await wrapper.get('form#create-account-form input[type="text"]').setValue('Admission account')
    await wrapper.get('[data-testid="select-pricing-groups"]').trigger('click')
    await wrapper.get('[data-testid="account-admission-enabled"]').setValue(true)
    return wrapper
  }

  it.each(['Claude', 'OpenAI', 'Gemini', 'Antigravity', 'Grok'])('keeps admission optional on the native %s authorization form', async (platform) => {
    const wrapper = mountModal(admissionGroups)
    if (platform !== 'Claude') await selectButtonByText(wrapper, platform)
    expect(wrapper.get<HTMLInputElement>('[data-testid="account-admission-enabled"]').element.checked).toBe(false)
    expect(wrapper.find('[data-testid="account-admission-test-group"]').exists()).toBe(false)
    expect(wrapper.findAllComponents(GroupSelectorStub)).toHaveLength(1)
    await wrapper.get('[data-testid="account-admission-enabled"]').setValue(true)
    expect(wrapper.find('[data-testid="account-admission-test-group"]').exists()).toBe(true)
    expect(wrapper.findAllComponents(GroupSelectorStub)).toHaveLength(1)
  })

  it('requires a temporary group before entering authorization', async () => {
    const wrapper = await prepareAdmission()
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    expect(wrapper.findComponent(OAuthAuthorizationFlowStub).exists()).toBe(false)
    expect(showErrorMock).toHaveBeenCalledWith('admin.accounts.admission.testGroupRequired')
    expect(exchangeCodeMock).not.toHaveBeenCalled()
  })

  it('filters temporary groups by platform and excludes inactive groups', async () => {
    const wrapper = await prepareAdmission()
    const select = wrapper.getComponent('[data-testid="account-admission-test-group"]')
    expect((select.props('options') as Array<{ value: number }>).map(item => item.value)).toEqual([1, 2, 3])
  })

  it.each(['openai', 'anthropic', 'gemini', 'antigravity', 'grok'])('preserves native formal groups through the existing two-step %s OAuth creation', async (platform) => {
    const wrapper = await prepareAdmission(platform)
    wrapper.getComponent('[data-testid="account-admission-test-group"]').vm.$emit('update:modelValue', 3)
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()
    const flow = wrapper.getComponent(OAuthAuthorizationFlowStub)
    expect(wrapper.find('form#create-account-form').exists()).toBe(false)
    expect(wrapper.find('[data-testid="account-admission-enabled"]').exists()).toBe(false)
    flow.vm.$emit('generate-url')
    await flushPromises()
    await selectButtonByText(wrapper, 'admin.accounts.oauth.completeAuth')
    expect(exchangeCodeMock).toHaveBeenCalledTimes(1)
    expect(createAccountMock).toHaveBeenCalledWith(expect.objectContaining({ platform, group_ids: [1, 2], admission: { enabled: true, test_group_id: 3 } }))
    expect(wrapper.emitted('created')).toHaveLength(1)
  })

  it('rejects overlapping temporary and formal groups before authorization', async () => {
    const wrapper = await prepareAdmission()
    wrapper.getComponent('[data-testid="account-admission-test-group"]').vm.$emit('update:modelValue', 1)
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    expect(wrapper.findComponent(OAuthAuthorizationFlowStub).exists()).toBe(false)
    expect(showErrorMock).toHaveBeenCalledWith('admin.accounts.admission.groupsOverlap')
  })

  it('requires a formal group when detection is enabled', async () => {
    const wrapper = await prepareAdmission()
    wrapper.getComponent(GroupSelectorStub).vm.$emit('update:modelValue', [])
    wrapper.getComponent('[data-testid="account-admission-test-group"]').vm.$emit('update:modelValue', 3)
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    expect(showErrorMock).toHaveBeenCalledWith('admin.accounts.admission.targetGroupsRequired')
    expect(wrapper.findComponent(OAuthAuthorizationFlowStub).exists()).toBe(false)
  })

  it('rechecks group validity before consuming a one-time authorization code', async () => {
    const wrapper = await prepareAdmission()
    wrapper.getComponent('[data-testid="account-admission-test-group"]').vm.$emit('update:modelValue', 3)
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()
    wrapper.getComponent(OAuthAuthorizationFlowStub).vm.$emit('generate-url')
    await flushPromises()
    await wrapper.setProps({ groups: admissionGroups.filter(group => group.id !== 3) })
    await selectButtonByText(wrapper, 'admin.accounts.oauth.completeAuth')
    expect(exchangeCodeMock).not.toHaveBeenCalled()
    expect(createAccountMock).not.toHaveBeenCalled()
  })

  it.each(['import-codex-session', 'import-codex-pat'])('passes admission and original groups through %s', async (action) => {
    const wrapper = await prepareAdmission()
    wrapper.getComponent('[data-testid="account-admission-test-group"]').vm.$emit('update:modelValue', 3)
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await wrapper.get('[data-testid="' + action + '"]').trigger('click')
    await flushPromises()
    const api = action === 'import-codex-session' ? importCodexSessionMock : createOpenAICodexPATMock
    expect(api).toHaveBeenCalledWith(expect.objectContaining({ group_ids: [1, 2], admission: { enabled: true, test_group_id: 3 } }))
  })

  it.each(['openai', 'antigravity', 'grok'])('preserves admission during %s refresh-token import', async (platform) => {
    const wrapper = await prepareAdmission(platform)
    wrapper.getComponent('[data-testid="account-admission-test-group"]').vm.$emit('update:modelValue', 3)
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()
    wrapper.getComponent(OAuthAuthorizationFlowStub).vm.$emit('validate-refresh-token', 'fixture-refresh')
    await flushPromises()
    expect(refreshTokenMock).toHaveBeenCalledTimes(1)
    expect(createAccountMock).toHaveBeenCalledWith(expect.objectContaining({ platform, group_ids: [1, 2], admission: { enabled: true, test_group_id: 3 } }))
  })

  it('passes admission through the native Grok SSO batch create request', async () => {
    const wrapper = await prepareAdmission('grok')
    wrapper.getComponent('[data-testid="account-admission-test-group"]').vm.$emit('update:modelValue', 3)
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()
    wrapper.getComponent(OAuthAuthorizationFlowStub).vm.$emit('import-sso', 'fixture-sso')
    await flushPromises()
    expect(grokSSOCreateMock).toHaveBeenCalledWith(expect.objectContaining({ sso_tokens: ['fixture-sso'], group_ids: [1, 2], admission: { enabled: true, test_group_id: 3 } }))
  })

  it('passes admission through Claude cookie authorization creation', async () => {
    const wrapper = await prepareAdmission('anthropic')
    wrapper.getComponent('[data-testid="account-admission-test-group"]').vm.$emit('update:modelValue', 3)
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()
    wrapper.getComponent(OAuthAuthorizationFlowStub).vm.$emit('cookie-auth', 'fixture-cookie')
    await flushPromises()
    expect(exchangeCodeMock).toHaveBeenCalledTimes(1)
    expect(createAccountMock).toHaveBeenCalledWith(expect.objectContaining({ platform: 'anthropic', group_ids: [1, 2], admission: { enabled: true, test_group_id: 3 } }))
  })

  it('omits admission after turning detection back off', async () => {
    const wrapper = await prepareAdmission()
    await wrapper.get('[data-testid="account-admission-enabled"]').setValue(false)
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await wrapper.get('[data-testid="import-codex-session"]').trigger('click')
    await flushPromises()
    expect(importCodexSessionMock.mock.calls[0][0]).not.toHaveProperty('admission')
    expect(importCodexSessionMock.mock.calls[0][0].group_ids).toEqual([1, 2])
  })
})
