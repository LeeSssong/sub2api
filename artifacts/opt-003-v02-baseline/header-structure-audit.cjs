const { spawnSync } = require('node:child_process')
const fs = require('node:fs')
const path = require('node:path')
const { chromium } = require('playwright')

const base = 'http://49.51.203.200'
const routes = [
  '/dashboard', '/usage', '/keys', '/redeem', '/orders', '/profile', '/purchase',
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
const selectedRoutes = process.env.AUDIT_ROUTE ? routes.filter(route => route === process.env.AUDIT_ROUTE) : routes

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
    const visible = el => {
      const rect = el.getBoundingClientRect()
      return rect.width > 0 && rect.height > 0 && getComputedStyle(el).visibility !== 'hidden'
    }
    const topHeader = document.querySelector('header.glass.sticky')
    const appTitle = topHeader?.querySelector('h1')
    const main = document.querySelector('main')
    const mainTop = main?.getBoundingClientRect().top ?? 0
    const headings = main ? [...main.querySelectorAll('h1,h2')].filter(el => visible(el) && el.getBoundingClientRect().top - mainTop < 180) : []
    const independent = headings.slice(0, 3).map(el => {
      const parent = el.parentElement
      const next = el.nextElementSibling
      return {
        tag: el.tagName,
        parentTag: parent?.tagName || null,
        nextTag: next?.tagName || null,
        nextParagraphVisible: !!next && next.tagName === 'P' && visible(next),
        adjacentParagraphs: parent ? [...parent.children].filter(child => child.tagName === 'P' && visible(child)).length : 0,
      }
    })
    return {
      landed: location.pathname,
      appHeaderPresent: !!topHeader,
      appTitleVisible: !!appTitle && visible(appTitle),
      appHeaderParagraphs: topHeader ? [...topHeader.querySelectorAll('p')].filter(visible).length : 0,
      mainPresent: !!main,
      topHeadings: independent,
    }
  })
}

async function main() {
  const browser = await chromium.launch({ headless: true, executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' })
  const result = { observedAt: new Date().toISOString(), method: 'admin session; structural metadata only, no page text or screenshots', results: [] }
  try {
    const account = credentials()
    for (const [width, height] of [[1280, 900], [390, 844]]) {
      const context = await browser.newContext({ viewport: { width, height } })
      const page = await context.newPage()
      await page.goto(`${base}/login`)
      await page.getByRole('textbox', { name: '邮箱' }).fill(account.email)
      await page.getByRole('textbox', { name: '密码' }).fill(account.password)
      await page.getByRole('button', { name: '登录', exact: true }).click()
      await page.waitForURL(/\/dashboard/, { timeout: 15000 })
      for (const requested of selectedRoutes) {
        let navigationError
        for (let attempt = 0; attempt < 3; attempt++) {
          try {
            const response = await page.goto(`${base}${requested}`, { waitUntil: 'domcontentloaded' })
            if (!response || response.status() >= 500) throw new Error(`HTTP ${response?.status() ?? 'no response'}`)
            navigationError = null
            break
          } catch (error) {
            navigationError = error
            if (attempt < 2) await page.waitForTimeout(750)
          }
        }
        if (navigationError) {
          result.results.push({ viewport: `${width}x${height}`, requested, navigationError: String(navigationError).split('\n')[0] })
          continue
        }
        try {
          await page.waitForFunction(() => !!document.querySelector('main') || (document.querySelector('#app')?.children.length ?? 0) >= 2, null, { timeout: 6000 })
        } catch {
          result.results.push({ viewport: `${width}x${height}`, requested, landed: new URL(page.url()).pathname, renderPending: true })
          continue
        }
        await page.waitForTimeout(200)
        result.results.push({ viewport: `${width}x${height}`, requested, ...await inspect(page) })
      }
      await context.close()
    }
  } finally {
    await browser.close()
  }
  const filename = process.env.AUDIT_ROUTE ? 'header-structure-targeted.json' : 'header-structure-audit.json'
  fs.writeFileSync(path.join(__dirname, filename), JSON.stringify(result, null, 2) + '\n')
  console.log(JSON.stringify({ count: result.results.length, pending: result.results.filter(r => r.renderPending).map(r => [r.viewport, r.requested]), withAdjacentParagraph: result.results.filter(r => r.topHeadings?.some(h => h.nextParagraphVisible || h.adjacentParagraphs)).map(r => [r.viewport, r.requested, r.landed]) }))
}

main().catch(error => { console.error(error.message); process.exitCode = 1 })
