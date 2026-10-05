const { spawnSync } = require('node:child_process')
const fs = require('node:fs')
const path = require('node:path')
const { chromium } = require('playwright')

const base = 'http://49.51.203.200'
const prototype = 'http://127.0.0.1:4183/?baseline=opt003-20260926#'
const business = [
  ['ai', '/dashboard'], ['usage', '/usage'], ['keys', '/keys'],
  ['recharge', '/redeem'], ['orders', '/orders'], ['profile', '/profile'],
]
const additional = [
  '/batch-image', '/affiliate', '/available-channels', '/subscriptions', '/key-usage', '/model-plaza',
  '/admin/dashboard', '/admin/ops', '/admin/operations/account-profitability',
  '/admin/operations/business-overview', '/admin/audit-logs', '/admin/users', '/admin/groups',
  '/admin/channels', '/admin/channels/pricing', '/admin/channels/monitor', '/monitor',
  '/admin/subscriptions', '/admin/accounts', '/admin/accounts/monitor', '/admin/announcements',
  '/admin/proxies', '/admin/redeem', '/admin/promo-codes', '/admin/scheduler-logs',
  '/admin/settings', '/admin/risk-control', '/admin/prompt-audit', '/admin/usage',
  '/admin/affiliates/invites', '/admin/affiliates/rebates', '/admin/affiliates/transfers',
  '/admin/orders/dashboard', '/admin/orders', '/admin/orders/plans',
  '/home', '/login', '/register', '/forgot-password', '/reset-password',
]

function credentials() {
  const cmd = "sudo -n bash -c 'set -a; . /opt/sub2api-test-station/.env; printf \"%s\\n%s\\n\" \"$ADMIN_LAB_ADMIN_EMAIL\" \"$ADMIN_LAB_ADMIN_PASSWORD\"'"
  const result = spawnSync('ssh', ['-T', '-o', 'BatchMode=yes', '-o', 'StrictHostKeyChecking=yes', 'sub2api-test-station', cmd], { encoding: 'utf8' })
  if (result.status !== 0) throw new Error(`Protected credential read failed (${result.status})`)
  const [email, password] = result.stdout.trimEnd().split('\n')
  if (!email || !password) throw new Error('Protected credentials not configured')
  return { email, password }
}

async function inspect(page) {
  return page.evaluate(() => {
    const root = document.querySelector('main') || document.body
    const title = root.querySelector('h1') || root.querySelector('h2')
    const titleRect = title?.getBoundingClientRect()
    const near = title && [...title.parentElement.querySelectorAll('p')].filter(p => {
      const r = p.getBoundingClientRect()
      return r.top >= titleRect.bottom - 4 && r.top - titleRect.bottom < 85 && r.width > 0 && r.height > 0
    })
    const candidates = (near || []).map(p => ({ length: p.textContent.trim().length, className: String(p.className).slice(0, 100) }))
    return {
      path: location.pathname,
      titlePresent: !!title,
      titleTag: title?.tagName || null,
      titleClass: title ? String(title.className).slice(0, 120) : null,
      titleDescriptionCandidates: candidates,
      horizontalOverflow: document.documentElement.scrollWidth > innerWidth,
      dialogCount: document.querySelectorAll('[role="dialog"],dialog[open]').length,
    }
  })
}

async function login(page, { email, password }) {
  await page.goto(`${base}/login`)
  await page.getByRole('textbox', { name: '邮箱' }).fill(email)
  await page.getByRole('textbox', { name: '密码' }).fill(password)
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await page.waitForURL(/\/dashboard/, { timeout: 15000 })
}

async function clickProbe(page, selector, name) {
  const item = page.locator(selector).first()
  if (!await item.isVisible().catch(() => false)) return { name, available: false }
  await item.click()
  await page.waitForTimeout(200)
  const state = await inspect(page)
  const result = { name, available: true, dialogCount: state.dialogCount, route: state.path }
  await page.keyboard.press('Escape')
  return result
}

async function main() {
  const browser = await chromium.launch({ headless: true, executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' })
  const result = { observedAt: new Date().toISOString(), routes: [], interactions: [] }
  try {
    const account = credentials()
    for (const [width, height] of [[1280, 900], [390, 844]]) {
      const context = await browser.newContext({ viewport: { width, height }, deviceScaleFactor: 1 })
      const live = await context.newPage()
      await login(live, account)
      for (const route of additional) {
        await live.goto(`${base}${route}`, { waitUntil: 'domcontentloaded' })
        await live.waitForTimeout(450)
        result.routes.push({ viewport: `${width}x${height}`, requested: route, observed: await inspect(live) })
      }
      const demo = await context.newPage()
      for (const [name, route] of business) {
        await demo.goto(`${prototype}${name}`)
        await live.goto(`${base}${route}`)
        await live.waitForTimeout(600)
        if (name === 'ai') {
          result.interactions.push({ viewport: `${width}x${height}`, page: name, prototype: await clickProbe(demo, '[data-action="route-details"]', 'route-detail'), testStation: await clickProbe(live, 'button[aria-label*="线路详情"]', 'route-detail') })
        } else if (name === 'keys') {
          result.interactions.push({ viewport: `${width}x${height}`, page: name, prototype: await clickProbe(demo, '[data-action="create-key"]', 'create-modal'), testStation: await clickProbe(live, '[data-tour="keys-create-btn"]', 'create-modal') })
        } else if (name === 'profile') {
          result.interactions.push({ viewport: `${width}x${height}`, page: name, prototype: await clickProbe(demo, '[data-action="support"]', 'support-modal'), testStation: await clickProbe(live, '[data-testid="user-sidebar-support"]', 'support-modal') })
        } else if (name === 'recharge') {
          result.interactions.push({ viewport: `${width}x${height}`, page: name, prototype: { rechargeTab: await demo.getByRole('tab', { name: '充值' }).isEnabled().catch(() => null) }, testStation: { rechargeTab: await live.getByRole('tab', { name: '充值' }).isEnabled().catch(() => null), orderLinkCount: await live.locator('a[href="/orders"]').count() } })
        }
      }
      await context.close()
    }
  } finally {
    await browser.close()
  }
  fs.writeFileSync(path.join(__dirname, 'continue-audit.json'), JSON.stringify(result, null, 2) + '\n')
  console.log(JSON.stringify({ routeCount: result.routes.length, interactions: result.interactions, candidateRoutes: result.routes.filter(r => r.observed.titleDescriptionCandidates.length).map(r => [r.viewport, r.requested, r.observed.path]) }))
}

main().catch(error => { console.error(error.message); process.exitCode = 1 })
