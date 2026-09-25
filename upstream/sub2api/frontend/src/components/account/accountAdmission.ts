import type { AccountAdmissionConfig, AccountPlatform, Group } from '@/types'

type AdmissionGroup = Pick<Group, 'id' | 'platform' | 'status'>

export function getAdmissionGroups<T extends AdmissionGroup>(
  groups: T[],
  platforms: AccountPlatform[] = [],
  mixedScheduling = false
): T[] {
  return groups.filter(group => group.status === 'active' && platforms.every(platform =>
    group.platform === platform || group.platform === 'composite' ||
    (platform === 'antigravity' && mixedScheduling && ['anthropic', 'gemini'].includes(group.platform))
  ))
}

export function buildAccountAdmission(enabled: boolean, testGroupId: number | null): AccountAdmissionConfig | undefined {
  if (!enabled) return undefined
  return { enabled: true, ...(testGroupId !== null ? { test_group_id: testGroupId } : {}) }
}

export function getAccountAdmissionError(options: {
  enabled: boolean
  testGroupId: number | null
  targetGroupIds: number[]
  groups: AdmissionGroup[]
  platforms: AccountPlatform[]
  requireTestGroup?: boolean
  mixedScheduling?: boolean
}): string | undefined {
  if (!options.enabled) return undefined
  if (options.requireTestGroup && options.testGroupId === null) return 'admin.accounts.admission.testGroupRequired'
  if (!options.targetGroupIds.length) return 'admin.accounts.admission.targetGroupsRequired'
  if (options.testGroupId !== null && options.targetGroupIds.includes(options.testGroupId)) return 'admin.accounts.admission.groupsOverlap'
  const validIds = new Set(getAdmissionGroups(options.groups, options.platforms, options.mixedScheduling).map(group => group.id))
  if (new Set(options.targetGroupIds).size !== options.targetGroupIds.length ||
      options.targetGroupIds.some(id => !validIds.has(id)) ||
      (options.testGroupId !== null && !validIds.has(options.testGroupId))) {
    return 'admin.accounts.admission.invalidGroups'
  }
  return undefined
}
