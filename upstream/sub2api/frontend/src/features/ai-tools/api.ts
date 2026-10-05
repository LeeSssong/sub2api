import { apiClient } from '@/api/client'
export interface LineCheck { group_id: number; status: 'success'|'timeout'|'failed'|'disabled'; ttft_ms: number|null; checked_at: string }
export interface GroupModels { group_id: number; supported_models: string[] }
export async function getGroupModels(signal?: AbortSignal): Promise<GroupModels[]> {
  const { data } = await apiClient.get<GroupModels[]>('/groups/available-models', { signal })
  return data
}
export async function checkLines(groupIDs:number[], signal?:AbortSignal):Promise<LineCheck[]> {
  const {data}=await apiClient.post<{results:LineCheck[]}>('/monitor-v4/check',{group_ids:groupIDs},{signal,timeout:120000})
  return data.results
}
