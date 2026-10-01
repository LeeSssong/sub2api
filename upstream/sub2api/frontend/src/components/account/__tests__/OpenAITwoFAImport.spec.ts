
vi.mock('@/api/admin/credentialEncryption', () => ({
  getCredentialEncryption: vi.fn().mockResolvedValue({ configured: true, source: 'server_config' }),
  initializeCredentialEncryption: vi.fn(),
}))
import { getCredentialEncryption, initializeCredentialEncryption } from '@/api/admin/credentialEncryption'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import OpenAITwoFAImport from '../OpenAITwoFAImport.vue'
import { deleteTwoFALogin, getTwoFALogin, startTwoFALogin } from '@/api/admin/accountTokenGuard'

vi.mock('vue-i18n', async importOriginal => ({ ...await importOriginal<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/api/admin/accountTokenGuard', async importOriginal => ({
  ...await importOriginal<typeof import('@/api/admin/accountTokenGuard')>(),
  startTwoFALogin: vi.fn(), getTwoFALogin: vi.fn(), deleteTwoFALogin: vi.fn(),
}))
enableAutoUnmount(afterEach)
beforeEach(() => {
  vi.resetAllMocks()
  vi.mocked(getCredentialEncryption).mockResolvedValue({ configured: true, source: 'server_config' })
  vi.mocked(startTwoFALogin).mockResolvedValue({ id: 'job-1', status: 'running' })
  vi.mocked(getTwoFALogin).mockResolvedValue({ id: 'job-1', status: 'succeeded', credential: { access_token: 'mock-access' } })
  vi.mocked(deleteTwoFALogin).mockResolvedValue()
})

async function start(wrapper: ReturnType<typeof mount>, input: string) {
  await flushPromises()
  await wrapper.get('textarea').setValue(input)
  await wrapper.get('button').trigger('click')
  await flushPromises()
}

describe('OpenAI initial 2FA import', () => {
  it('imports each account, skips completed rows, and retries creation without logging in again', async () => {
    const importCredential = vi.fn().mockResolvedValueOnce('created').mockRejectedValueOnce(new Error('do not display credential'))
    const wrapper = mount(OpenAITwoFAImport, { props: { importCredential } })
    await start(wrapper, 'a@example.com----p1----s1\nb@example.com----p2----s2')
    expect(importCredential).toHaveBeenCalledTimes(2)
    expect(startTwoFALogin).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('tokenGuard.twoFA.states.created')
    expect(wrapper.text()).toContain('tokenGuard.twoFA.states.importFailed')
    expect(wrapper.text()).not.toContain('do not display credential')
    expect(wrapper.find('textarea').exists()).toBe(false)
    importCredential.mockResolvedValueOnce('created')
    await wrapper.get('button').trigger('click')
    await flushPromises()
    expect(importCredential).toHaveBeenCalledTimes(3)
    expect(startTwoFALogin).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).not.toContain('tokenGuard.twoFA.retry')
    expect(importCredential.mock.calls[1]?.[2]).toEqual({ email: 'b@example.com', password: 'p2', mfa_secret: 's2' })
    expect(importCredential.mock.calls[2]?.[2]).toEqual({ email: 'b@example.com', password: 'p2', mfa_secret: 's2' })
    expect(wrapper.emitted('busy')).toEqual([[true], [false], [true], [false]])
  })

  it('divides the total purchase cost by all valid rows before any import result is known', async () => {
    const importCredential = vi.fn()
      .mockResolvedValueOnce('created')
      .mockRejectedValueOnce(new Error('import failed'))
      .mockResolvedValueOnce('created')
    const wrapper = mount(OpenAITwoFAImport, { props: { importCredential } })
    await flushPromises()
    await wrapper.get('[data-testid="two-fa-total-cost"]').setValue('3')
    await start(wrapper, 'a@example.com----p1----s1\nb@example.com----p2----s2\nc@example.com----p3----s3')

    expect(importCredential).toHaveBeenCalledTimes(3)
    expect(importCredential.mock.calls.map(call => call[3])).toEqual([1, 1, 1])
  })

  it('passes no purchase cost when the optional field is blank', async () => {
    const importCredential = vi.fn().mockResolvedValue('created')
    const wrapper = mount(OpenAITwoFAImport, { props: { importCredential } })
    await start(wrapper, 'a@example.com----p----s')

    expect(importCredential.mock.calls[0]?.[3]).toBeUndefined()
  })

  it('rejects a negative purchase cost before starting any login', async () => {
    const importCredential = vi.fn()
    const wrapper = mount(OpenAITwoFAImport, { props: { importCredential } })
    await flushPromises()
    await wrapper.get('[data-testid="two-fa-total-cost"]').setValue('-0.01')
    await start(wrapper, 'a@example.com----p----s')

    expect(wrapper.get('[role="alert"]').text()).toBe('tokenGuard.twoFA.purchaseCostInvalid')
    expect(startTwoFALogin).not.toHaveBeenCalled()
    expect(importCredential).not.toHaveBeenCalled()
  })

  it('never imports when login fails, and rejects malformed batch lines before sending anything', async () => {
    const importCredential = vi.fn()
    const wrapper = mount(OpenAITwoFAImport, { props: { importCredential } })
    await start(wrapper, 'a@example.com----p----s\ninvalid')
    expect(startTwoFALogin).not.toHaveBeenCalled()
    expect(wrapper.get('[role="alert"]').text()).toContain('invalid')
    vi.mocked(getTwoFALogin).mockResolvedValue({ id: 'job-1', status: 'failed' })
    await start(wrapper, 'a@example.com----p----s')
    expect(importCredential).not.toHaveBeenCalled()
    expect(deleteTwoFALogin).toHaveBeenCalledWith('job-1')
  })

  it('resumes a job after a polling failure instead of starting another login', async () => {
    const importCredential = vi.fn().mockResolvedValue('created')
    const wrapper = mount(OpenAITwoFAImport, { props: { importCredential } })
    vi.mocked(getTwoFALogin).mockRejectedValueOnce(new Error('network failure'))
    await start(wrapper, 'a@example.com----p----s')
    await wrapper.get('button').trigger('click')
    await flushPromises()
    expect(startTwoFALogin).toHaveBeenCalledTimes(1)
    expect(importCredential).toHaveBeenCalledTimes(1)
  })

  it('stops an active login without importing or starting the next row', async () => {
    vi.useFakeTimers()
    try {
      vi.mocked(getTwoFALogin).mockResolvedValue({ id: 'job-1', status: 'running' })
      const importCredential = vi.fn()
      const wrapper = mount(OpenAITwoFAImport, { props: { importCredential } })
      await start(wrapper, 'a@example.com----p----s\nb@example.com----p----s')
      await wrapper.get('button').trigger('click')
      await vi.advanceTimersByTimeAsync(1500)
      await flushPromises()
      expect(deleteTwoFALogin).toHaveBeenCalledWith('job-1')
      expect(importCredential).not.toHaveBeenCalled()
      expect(startTwoFALogin).toHaveBeenCalledTimes(1)
    } finally {
      vi.useRealTimers()
    }
  })
})


it('blocks 2FA login and account creation until encryption initialization succeeds', async () => {
  vi.mocked(getCredentialEncryption).mockResolvedValue({ configured: false, source: 'unconfigured' })
  vi.mocked(initializeCredentialEncryption).mockResolvedValue({ configured: true, source: 'local_file' })
  const importCredential = vi.fn().mockResolvedValue('created')
  const wrapper = mount(OpenAITwoFAImport, { props: { importCredential } })
  await flushPromises()
  await wrapper.get('textarea').setValue('a@example.com----password----secret')
  expect(wrapper.get('[data-testid="two-fa-start"]').attributes('disabled')).toBeDefined()
  await wrapper.get('[data-testid="two-fa-start"]').trigger('click')
  expect(startTwoFALogin).not.toHaveBeenCalled()
  expect(importCredential).not.toHaveBeenCalled()
  await wrapper.get('[data-testid="initialize-credential-encryption"]').trigger('click')
  await flushPromises()
  expect(wrapper.get('[data-testid="two-fa-start"]').attributes('disabled')).toBeUndefined()
  await wrapper.get('[data-testid="two-fa-start"]').trigger('click')
  await flushPromises()
  expect(importCredential).toHaveBeenCalledTimes(1)
})
