import { defineComponent } from 'vue'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { AdminGroup } from '@/types'

enableAutoUnmount(afterEach)
const { importData, showError } = vi.hoisted(() => ({ importData: vi.fn(), showError: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: { accounts: { importData } } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess: vi.fn(), showWarning: vi.fn() }) }))
vi.mock('vue-i18n', async () => ({ ...(await vi.importActual<typeof import('vue-i18n')>('vue-i18n')), useI18n: () => ({ t: (key: string) => key }) }))
import ImportDataModal from '../ImportDataModal.vue'

const GroupSelectorStub = defineComponent({
  name: 'GroupSelector',
  props: ['modelValue', 'groups', 'label'],
  emits: ['update:modelValue'],
  template: '<button type="button" data-testid="json-target-groups" @click="$emit(\'update:modelValue\', [1])">Groups</button>',
})
const groups = [
  { id: 1, name: 'Formal', platform: 'openai', status: 'active' },
  { id: 2, name: 'Temporary', platform: 'openai', status: 'active' },
  { id: 3, name: 'Claude group', platform: 'anthropic', status: 'active' },
] as AdminGroup[]
const data = { proxies: [], accounts: [{ name: 'Imported', platform: 'openai', type: 'oauth', credentials: { access_token: 'fixture' } }] }

async function openImport() {
  const wrapper = mount(ImportDataModal, {
    props: { show: true, groups },
    global: { stubs: {
      BaseDialog: defineComponent({ template: '<div><slot /><slot name="footer" /></div>' }),
      Select: true, GroupSelector: GroupSelectorStub,
    } },
  })
  const file = new File([JSON.stringify(data)], 'accounts.json', { type: 'application/json' })
  Object.defineProperty(file, 'text', { value: async () => JSON.stringify(data) })
  Object.defineProperty(wrapper.get('input[type="file"]').element, 'files', { value: [file], configurable: true })
  await wrapper.get('input[type="file"]').trigger('change')
  await flushPromises()
  return wrapper
}

describe('JSON account admission', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    importData.mockResolvedValue({ account_created: 1, account_failed: 0, proxy_created: 0, proxy_failed: 0, proxy_reused: 0 })
  })

  it('keeps the original import payload while detection is disabled', async () => {
    const wrapper = await openImport()
    expect(wrapper.get<HTMLInputElement>('[data-testid="data-admission-enabled"]').element.checked).toBe(false)
    expect(wrapper.find('[data-testid="data-admission-test-group"]').exists()).toBe(false)
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()
    expect(importData).toHaveBeenCalledWith({ data, skip_default_group_bind: true })
  })

  it('filters native JSON groups using the selected file platforms', async () => {
    const wrapper = await openImport()
    await wrapper.get('[data-testid="data-admission-enabled"]').setValue(true)
    const options = wrapper.getComponent('[data-testid="data-admission-test-group"]').props('options') as Array<{ value: number | null }>
    expect(options.map(option => option.value)).toEqual([null, 1, 2])
    expect(wrapper.getComponent(GroupSelectorStub).props('groups').map((group: AdminGroup) => group.id)).toEqual([1, 2])
  })

  it('allows JSON admission with an unassigned temporary group and native formal groups', async () => {
    const wrapper = await openImport()
    await wrapper.get('[data-testid="data-admission-enabled"]').setValue(true)
    expect(wrapper.getComponent('[data-testid="data-admission-test-group"]').props('modelValue')).toBeNull()
    await wrapper.get('[data-testid="json-target-groups"]').trigger('click')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()
    expect(importData).toHaveBeenCalledWith({ data, skip_default_group_bind: true, group_ids: [1], admission: { enabled: true } })
  })

  it('rejects a temporary group also selected as a formal group', async () => {
    const wrapper = await openImport()
    await wrapper.get('[data-testid="data-admission-enabled"]').setValue(true)
    await wrapper.get('[data-testid="json-target-groups"]').trigger('click')
    wrapper.getComponent('[data-testid="data-admission-test-group"]').vm.$emit('update:modelValue', 1)
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()
    expect(importData).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalledWith('admin.accounts.admission.groupsOverlap')
  })

  it('drops admission configuration if detection is switched off again', async () => {
    const wrapper = await openImport()
    await wrapper.get('[data-testid="data-admission-enabled"]').setValue(true)
    await wrapper.get('[data-testid="json-target-groups"]').trigger('click')
    wrapper.getComponent('[data-testid="data-admission-test-group"]').vm.$emit('update:modelValue', 2)
    await wrapper.get('[data-testid="data-admission-enabled"]').setValue(false)
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()
    expect(importData).toHaveBeenCalledWith({ data, skip_default_group_bind: true })
  })

  it('rejects formal groups incompatible with the imported account platform', async () => {
    const wrapper = await openImport()
    await wrapper.setProps({ groups: [{ ...groups[0], platform: 'anthropic' }] as AdminGroup[] })
    await wrapper.get('[data-testid="data-admission-enabled"]').setValue(true)
    await wrapper.get('[data-testid="json-target-groups"]').trigger('click')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()
    expect(importData).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalledWith('admin.accounts.admission.invalidGroups')
  })

  it('requires a formal group when JSON admission is enabled', async () => {
    const wrapper = await openImport()
    await wrapper.get('[data-testid="data-admission-enabled"]').setValue(true)
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()
    expect(importData).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalledWith('admin.accounts.admission.targetGroupsRequired')
  })
})
