import { apiClient } from '@/api/client'
import type { PlazaOfficialPricing } from '@/api/modelPlaza'
export interface LineCheck { group_id: number; status: 'success'|'timeout'|'failed'|'disabled'; ttft_ms: number|null; checked_at: string }
export interface GroupModels { group_id: number; supported_models: string[]; official_pricing?: Record<string, PlazaOfficialPricing | null> }
export async function getGroupModels(signal?: AbortSignal, groupIDs?: number[], includePricing = false): Promise<GroupModels[]> {
  const params = { ...(groupIDs ? { group_ids: groupIDs.join(',') } : {}), ...(includePricing ? { include_pricing: true } : {}) }
  const { data } = await apiClient.get<GroupModels[]>('/groups/available-models', { signal, params })
  return data
}
export async function checkLines(groupIDs:number[], signal?:AbortSignal):Promise<LineCheck[]> {
  const {data}=await apiClient.post<{results:LineCheck[]}>('/monitor-v4/check',{group_ids:groupIDs},{signal,timeout:120000})
  return data.results
}

export interface ModelPopularity { model: string; requests: number }
export interface ModelPopularityResponse { models: ModelPopularity[]; start_time: string; end_time: string }
export async function getModelPopularity(signal?: AbortSignal): Promise<ModelPopularityResponse> {
  const { data } = await apiClient.get<ModelPopularityResponse>('/usage/models/popularity', { signal })
  return data
}
