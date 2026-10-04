// Official reference prices confirmed in the approved prototype (2026-10-03).
// USD per million tokens. Missing entries remain unknown; never infer a tier rate.
export type PricingTier = 'standard' | 'batch' | 'flex' | 'fast' | 'ultrafast'
export type PriceValues = Partial<Record<'input' | 'cache' | 'cacheWrite' | 'output' | 'longInput' | 'longCache' | 'longCacheWrite' | 'longOutput', number | null>>
export const pricingTiers: { id: PricingTier; label: string }[] = [
  { id: 'standard', label: 'Standard' }, { id: 'batch', label: 'Batch' },
  { id: 'flex', label: 'Flex' }, { id: 'fast', label: 'Fast' },
  { id: 'ultrafast', label: 'Ultrafast' }
]
export const priceColumns: { key: keyof PriceValues; label: string }[] = [
  { key: 'input', label: '输入' }, { key: 'cache', label: '缓存输入' },
  { key: 'cacheWrite', label: '缓存写入' }, { key: 'output', label: '输出' },
  { key: 'longInput', label: '输入' }, { key: 'longCache', label: '缓存输入' },
  { key: 'longCacheWrite', label: '缓存写入' }, { key: 'longOutput', label: '输出' }
]
const openAiPrices:Record<string,PriceValues & {name:string}>={
  'gpt-6-astra':{name:'GPT-6 Astra',input:10,cache:1,cacheWrite:12.5,output:50,longInput:20,longCache:2,longCacheWrite:25,longOutput:75},
  'gpt-5.6-sol':{name:'GPT-5.6 Sol',input:4,cache:.4,cacheWrite:5,output:20,longInput:8,longCache:.8,longCacheWrite:10,longOutput:30},
  'gpt-5.6-terra':{name:'GPT-5.6 Terra',input:2,cache:.2,cacheWrite:2.5,output:12,longInput:4,longCache:.4,longCacheWrite:5,longOutput:18},
  'gpt-5.6-luna':{name:'GPT-5.6 Luna',input:.2,cache:.02,cacheWrite:.25,output:1.2,longInput:.4,longCache:.04,longCacheWrite:.5,longOutput:1.8},
  'gpt-5.5':{name:'GPT-5.5',input:5,cache:.5,cacheWrite:null,output:30,longInput:10,longCache:1,longCacheWrite:null,longOutput:45},
  'gpt-5.4':{name:'GPT-5.4',input:2.5,cache:.25,cacheWrite:null,output:15,longInput:5,longCache:.5,longCacheWrite:null,longOutput:22.5},
  'gpt-5.2':{name:'GPT-5.2',input:1.75,cache:.175,output:14},
  'gpt-5.2-pro':{name:'GPT-5.2 pro',input:21,cache:null,output:168},
  'gpt-5.4-mini':{name:'GPT-5.4 mini',input:.75,cache:.075,output:4.5}
}
const openAiTierPrices:Record<string,Partial<Record<PricingTier,PriceValues>>>= {
  'gpt-6-astra':{
    batch:{input:5,cache:.5,cacheWrite:6.25,output:25,longInput:10,longCache:1,longCacheWrite:12.5,longOutput:37.5},
    flex:{input:5,cache:.5,cacheWrite:6.25,output:25,longInput:10,longCache:1,longCacheWrite:12.5,longOutput:37.5},
    fast:{input:20,cache:2,cacheWrite:25,output:100,longInput:40,longCache:4,longCacheWrite:50,longOutput:150},
    ultrafast:{input:60,cache:6,cacheWrite:75,output:300,longInput:120,longCache:12,longCacheWrite:150,longOutput:450}},
  'gpt-5.6-sol':{
    batch:{input:2,cache:.2,cacheWrite:2.5,output:10,longInput:4,longCache:.4,longCacheWrite:5,longOutput:15},
    flex:{input:2,cache:.2,cacheWrite:2.5,output:10,longInput:4,longCache:.4,longCacheWrite:5,longOutput:15},
    fast:{input:8,cache:.8,cacheWrite:10,output:40,longInput:16,longCache:1.6,longCacheWrite:20,longOutput:60}},
  'gpt-5.6-terra':{
    batch:{input:1,cache:.1,cacheWrite:1.25,output:6,longInput:2,longCache:.2,longCacheWrite:2.5,longOutput:9},
    flex:{input:1,cache:.1,cacheWrite:1.25,output:6,longInput:2,longCache:.2,longCacheWrite:2.5,longOutput:9},
    fast:{input:4,cache:.4,cacheWrite:5,output:24,longInput:8,longCache:.8,longCacheWrite:10,longOutput:36}},
  'gpt-5.6-luna':{
    batch:{input:.1,cache:.01,cacheWrite:.125,output:.6,longInput:.2,longCache:.02,longCacheWrite:.25,longOutput:.9},
    flex:{input:.1,cache:.01,cacheWrite:.125,output:.6,longInput:.2,longCache:.02,longCacheWrite:.25,longOutput:.9},
    fast:{input:.4,cache:.04,cacheWrite:.5,output:2.4,longInput:.8,longCache:.08,longCacheWrite:1,longOutput:3.6}},
  'gpt-5.5':{batch:{input:2.5,cache:.25,output:15,longInput:5,longCache:.5,longOutput:22.5},flex:{input:2.5,cache:.25,output:15,longInput:5,longCache:.5,longOutput:22.5},fast:{input:12.5,cache:1.25,output:75,longInput:null,longCache:null,longOutput:null}},
  'gpt-5.4':{batch:{input:1.25,cache:.13,output:7.5,longInput:2.5,longCache:.25,longOutput:11.25},flex:{input:1.25,cache:.13,output:7.5,longInput:2.5,longCache:.25,longOutput:11.25},fast:{input:5,cache:.5,output:30,longInput:null,longCache:null,longOutput:null}},
  'gpt-5.4-mini':{batch:{input:.375,cache:.0375,output:2.25},flex:{input:.375,cache:.0375,output:2.25},fast:{input:1.5,cache:.15,output:9}},
  'gpt-5.2':{batch:{input:.875,cache:.0875,output:7},flex:{input:.875,cache:.0875,output:7},fast:{input:3.5,cache:.35,output:28}},
  'gpt-5.2-pro':{batch:{input:10.5,cache:null,output:84}}
}

export function officialPrice(model: string, tier: PricingTier, column: keyof PriceValues): string {
  const value = tier === 'standard' ? openAiPrices[model]?.[column] : openAiTierPrices[model]?.[tier]?.[column]
  if (value === undefined) return '待核对'
  if (value === null) return '—'
  return `$${value.toFixed(Math.max(2, (String(value).split('.')[1] || '').length))}`
}
