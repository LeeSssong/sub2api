import type { PelicanShowcaseView, PelicanShowcaseItem } from '../../src/api/pelicanShowcase'

const fixtureTime = Date.now() - 120000
const now = new Date(fixtureTime).toISOString()
const item = (id: number, group_id: number): PelicanShowcaseItem => ({ id, group_id, model_id: 'gpt-6-astra', reasoning_effort: 'medium', latency_ms: 153100, generated_at: now })
const stats = (success_count: number, total_count: number) => ({ success_count, total_count, success_rate: total_count ? success_count / total_count * 100 : null })
const base: PelicanShowcaseView = {
  enabled: true, max_items: 20, retention_days: 7,
  stats: stats(18, 24),
  stats_window: { from: new Date(fixtureTime - 86400000).toISOString(), to: now, coverage_started_at: new Date(fixtureTime - 172800000).toISOString(), complete: true },
  groups: [
    { id: 1, name: 'GPT-Pro5x', platform: 'openai', stats: stats(15, 20), items: [item(1, 1), item(2, 1), item(3, 1), item(4, 1), item(5, 1), item(6, 1)] },
    { id: 2, name: 'GPT-Pro20x', platform: 'openai', stats: stats(7, 9), items: [] },
  ],
}
const scene = (width: number, height: number) => `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${width} ${height}" width="${width}" height="${height}"><rect width="100%" height="100%" fill="#f5efdf"/><rect x="24" y="24" width="${width - 48}" height="${height - 48}" rx="22" fill="#c9e8df"/><text x="48" y="80" fill="#25575b" font-size="32">海风慢骑 · 完整画布</text><circle cx="${width - 110}" cy="155" r="50" fill="#ffe594"/><path d="M24 ${height * .55} Q${width * .4} ${height * .43} ${width - 24} ${height * .6} V${height - 70} H24Z" fill="#8fc6c2"/><g stroke="#315d63" stroke-width="8" fill="none"><circle cx="${width * .33}" cy="${height * .65}" r="${width * .12}"/><circle cx="${width * .7}" cy="${height * .65}" r="${width * .12}"/><path d="M${width * .33} ${height * .65} L${width * .45} ${height * .46} L${width * .56} ${height * .65} Z L${width * .64} ${height * .43} L${width * .7} ${height * .65}" stroke="#d98559"/></g><text x="48" y="${height - 46}" fill="#25575b" font-size="24">底部边界 · ${width} × ${height}</text></svg>`
const longHtml = `<!doctype html><html><head><style>html,body{margin:0;background:#f5efdf;font-family:sans-serif}.poster{width:1200px;height:1600px;display:flex;flex-direction:column;justify-content:space-between;padding:60px;box-sizing:border-box;color:#25575b}.poster h1{font-size:58px}.poster p{font-size:32px}.poster svg{width:100%;height:850px}</style></head><body><main class="poster"><header><p>THE SLOW COAST CLUB</p><h1>顺着海风，慢慢骑。</h1></header>${scene(1024, 768)}<footer><p>长幅海报 · 底部完整可见</p><p>1200 × 1600 · 超出默认逻辑视口</p></footer></main></body></html>`
const delayedHtml = `<!doctype html><html><head><style>html,body{margin:0;background:#f5efdf}main{height:100vh;box-sizing:border-box;padding:30px;color:#25575b;font:28px sans-serif}svg{width:100%;height:500px}</style></head><body><main><h1>延迟内容 + 100vh</h1>${scene(1024, 768)}</main><script>setTimeout(()=>{const footer=document.createElement('footer');footer.style.cssText='height:300px;background:#b9ddd0;padding:30px;box-sizing:border-box;font:32px sans-serif';footer.textContent='延迟出现的底部：整幅仍应可见';document.body.append(footer)},300);</script></body></html>`
const responsiveHtml = `<!doctype html><html><head><style>html,body{margin:0}.poster{width:1600px;height:800px;box-sizing:border-box;padding:60px;background:#c9e8df;color:#25575b;font:40px sans-serif}@media(min-width:1200px){.poster{background:#ffb1a7}.poster::after{content:'媒体查询随外框改变了原始画布'}}</style></head><body><main class="poster"><h1>固定逻辑视口 · 1024 × 768</h1><p>这张1600像素宽的作品应保持海绿色。</p><p id="viewport"></p></main><script>function report(){document.getElementById('viewport').textContent='JS viewport: '+innerWidth+' × '+innerHeight+'; min-width1200: '+matchMedia('(min-width:1200px)').matches}report();addEventListener('resize',report);</script></body></html>`
const bodies: Record<number, string> = { 1: longHtml, 2: scene(1024, 768), 3: scene(640, 1200), 4: delayedHtml, 5: scene(1024, 768).replace(' width="1024" height="768"', ''), 6: responsiveHtml }

export async function getShowcase(): Promise<PelicanShowcaseView> {
  const view = structuredClone(base)
  const scenario = new URLSearchParams(location.search).get('scenario')
  if (scenario === 'partial') Object.assign(view.stats_window!, { coverage_started_at: new Date(fixtureTime - 7200000).toISOString(), complete: false })
  if (scenario === 'empty') { view.stats = stats(0, 0); view.groups.forEach(group => { group.stats = stats(0, 0) }) }
  if (scenario === 'unavailable') { view.stats = null; view.stats_window = null; view.groups.forEach(group => { group.stats = null }) }
  return view
}
export async function getShowcaseItem(id: number) { return { ...item(id, 1), response_text: bodies[id] } }
export async function removeShowcaseItem(id: number) { base.groups.forEach(group => { group.items = group.items.filter(artwork => artwork.id !== id) }) }
