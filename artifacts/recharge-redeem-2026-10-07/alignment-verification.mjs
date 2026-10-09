import { chromium } from '/Users/awen/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright/index.mjs'
import { mkdir, writeFile } from 'node:fs/promises'
const out = '/Users/awen/.codex/worktrees/recharge-redeem-storefront/星桥测试服/artifacts/recharge-redeem-2026-10-07'
await mkdir(out, { recursive: true })
const user = { id: 1111, username: '演示账户', email: 'demo@example.test', role: 'user', status: 'active', balance: 0.15, concurrency: 5 }
const settings = { site_name: '星桥 AI Link', site_logo: '/xingqiao/logo-brand.png', payment_enabled: false, channel_monitor_enabled: false, contact_info: '请通过侧栏客服入口联系', custom_menu_items: [{ id: 'xingqiao-storefront', label: '云猫兑换码充值', url: 'https://catfk.com/shop/DLK8SNUJ', visibility: 'user', sort_order: 90, icon_svg: '' }] }
const browser = await chromium.launch({ executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', headless: true })
try {
 const context = await browser.newContext({ viewport: { width: 1600, height: 1000 } })
 await context.addInitScript(({ user, settings }) => {
   window.__APP_CONFIG__ = settings
   localStorage.setItem('auth_token', 'local-browser-demo-token')
   localStorage.setItem('auth_user', JSON.stringify(user))
   localStorage.setItem('locale', 'zh')
   localStorage.setItem('theme', 'dark')
   localStorage.setItem('user_guide_completed', 'true')
 }, { user, settings })
 const errors = []
 const requests = []
 await context.route('**/*', async route => {
   const url = new URL(route.request().url())
   if (url.origin !== 'http://127.0.0.1:43127') return route.continue()
   if (!url.pathname.startsWith('/api/') && url.pathname !== '/setup/status') return route.continue()
   requests.push({ path: url.pathname, method: route.request().method() })
   let data = {}
   let status = 200
   if (url.pathname.endsWith('/auth/me')) data = user
   else if (url.pathname.endsWith('/settings/public')) data = settings
   else if (url.pathname === '/setup/status') data = { needs_setup: false }
   else if (url.pathname.endsWith('/groups/available')) data = []
   else if (url.pathname.endsWith('/keys')) data = { items: [], total: 0, page: 1, page_size: 20, pages: 0 }
   else if (url.pathname.endsWith('/redeem/history')) data = { items: [], total: 0 }
   else if (url.pathname.endsWith('/redeem')) {
     const code = route.request().postDataJSON().code
     if (code === 'INVALID') { status = 400; data = { detail: '兑换码无效，请检查后重试' } }
     else { user.balance = 20.15; data = { type: 'balance', value: 20, message: '演示兑换成功（本地模拟）', new_balance: 20.15 } }
   } else if (url.pathname.includes('announcement')) data = { items: [], total: 0 }
   else if (url.pathname.includes('subscription')) data = []
   await route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(status === 200 ? { code: 0, data } : data) })
 })
 const page = await context.newPage()
 page.on('pageerror', error => errors.push(error.message))
 await page.goto('http://127.0.0.1:43127/custom/xingqiao-storefront')
 await page.locator('#code').waitFor()
 await page.waitForFunction(() => document.querySelector('.user-topbar-title')?.textContent?.includes('充值') || document.body.textContent.includes('购买后复制兑换码'))
 // Wait only for the third-party initial paint; do not enter checkout or create any orders.
 try { await page.frameLocator('iframe').getByText('店铺公告').waitFor({ timeout: 15000 }) } catch {}
 const measure = async () => page.evaluate(() => {
   const rect = selector => { const el = document.querySelector(selector); if (!el) return null; const r = el.getBoundingClientRect(); return { x: r.x, y: r.y, width: r.width, height: r.height, bottom: r.bottom } }
   return { width: innerWidth, route: location.pathname, title: document.title, form: rect('[data-test="redeem-form-card"]'), shop: rect('[aria-labelledby="storefront-title"]'), grid: getComputedStyle(document.querySelector('.redeem-workspace')).gridTemplateColumns, overflow: document.documentElement.scrollWidth > innerWidth, workspaceOverflow: document.querySelector('.user-workspace').scrollWidth > document.querySelector('.user-workspace').clientWidth, link: document.querySelector('[data-test="storefront-open"]').href, iframe: document.querySelector('iframe').src, balanceLink: document.querySelector('[data-testid="user-sidebar-recharge"]').getAttribute('href'), topbar: rect('.user-topbar'), first: rect('.redeem-page > div'), firstContent: rect('.redeem-page > div > section'), bottom: rect('.redeem-page > .space-y-6 > .card'), actions: rect('.user-sidebar-actions') }
 })
 const alignment = {}
 for (const width of [1600, 375]) {
   await page.setViewportSize({ width, height: width === 1600 ? 1000 : 900 })
   await page.waitForFunction(width => document.querySelector('.user-main-frame').getBoundingClientRect().x === (width === 1600 ? 242 : 76), width)
   await page.locator('.user-workspace').evaluate(el => el.scrollTo({ top: 0, behavior: 'instant' }))
   const position = async selector => page.evaluate(selector => ({ first: document.querySelector(selector).getBoundingClientRect().top, title: document.querySelector('.user-topbar h1').getBoundingClientRect().top }), selector)
   const redeem = await position('.redeem-page > div > section')
   await page.locator('a[href="/keys"]').first().click()
   await page.locator('.keys-work-surface').waitFor()
   const keys = await position('.keys-work-surface')
   alignment[width] = { redeem, keys }
   if (Math.abs(redeem.first - keys.first) > 1 || Math.abs(redeem.title - keys.title) > 1) throw new Error('Keys/recharge top alignment failed')
   await page.locator('[data-testid="user-sidebar-recharge"]').click()
   await page.locator('#code').waitFor()
 }
 await page.setViewportSize({ width: 1600, height: 1000 })
 await page.waitForFunction(() => document.querySelector('.user-main-frame').getBoundingClientRect().x === 242)
 await page.evaluate(() => document.documentElement.classList.remove('dark'))
 await page.waitForFunction(() => getComputedStyle(document.querySelector('[data-test="redeem-form-card"]')).backgroundColor === 'rgb(240, 245, 244)')
 await page.screenshot({ path: `${out}/desktop-light.png` })
 await page.setViewportSize({ width: 375, height: 900 })
 await page.waitForFunction(() => document.querySelector('.user-main-frame').getBoundingClientRect().x === 76)
 await page.screenshot({ path: `${out}/mobile-light.png` })
 if (errors.length) throw new Error(errors.join('; '))
 await writeFile(`${out}/alignment.json`, JSON.stringify({ alignment, errors }, null, 2))
 console.log(JSON.stringify({ alignment, errors }, null, 2))
} finally { await browser.close() }
