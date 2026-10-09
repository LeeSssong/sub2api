import { apiClient } from '@/api/client'
import type { MonitorV4Window } from '@/features/monitor-v4/types'
export type RouteTimelinePoint = { group_id:number; start:string; end:string; request_count:number; success_count:number; cache_hit_rate:number|null; ttft_p50_ms:number|null; graded_round_count?:number; suspected_degraded_round_count?:number }
export async function getRouteTimeline(window:MonitorV4Window,signal:AbortSignal,granularity: 'hour'|'day' = 'hour'):Promise<RouteTimelinePoint[]> {
 const {data}=await apiClient.get('/monitor-v4/timeline',{params:{window,granularity},signal})
 if(data?.window!==window || !Array.isArray(data.points) || data.points.length>16800) throw new Error('Invalid timeline')
 for(const p of data.points){
  if(!Number.isSafeInteger(p.group_id)||p.group_id<=0||!Number.isSafeInteger(p.request_count)||!Number.isSafeInteger(p.success_count)||p.success_count<0||p.request_count<p.success_count||!Number.isFinite(Date.parse(p.start))||!(Date.parse(p.end)>Date.parse(p.start)))throw new Error('Invalid timeline point')
  if(p.cache_hit_rate!==null && (!Number.isFinite(p.cache_hit_rate)||p.cache_hit_rate<0||p.cache_hit_rate>1))throw new Error('Invalid cache hit rate')
  if(p.ttft_p50_ms!==null && (!Number.isFinite(p.ttft_p50_ms)||p.ttft_p50_ms<0))throw new Error('Invalid first token latency')
  if(p.graded_round_count!==undefined || p.suspected_degraded_round_count!==undefined){
   if(!Number.isSafeInteger(p.graded_round_count)||p.graded_round_count<0||!Number.isSafeInteger(p.suspected_degraded_round_count)||p.suspected_degraded_round_count<0||p.suspected_degraded_round_count>p.graded_round_count)throw new Error('Invalid graded patrol counts')
  }
 }
 return data.points
}
