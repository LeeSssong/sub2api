const { spawnSync } = require('node:child_process')
const fs = require('node:fs')
const path = require('node:path')
const { chromium } = require('playwright')

const routes = [
  '/admin/dashboard', '/admin/ops', '/admin/operations/account-profitability',
  '/admin/operations/business-overview', '/admin/audit-logs', '/admin/users',
  '/admin/groups', '/admin/channels', '/admin/channels/pricing',
  '/admin/channels/monitor', '/monitor', '/admin/subscriptions',
  '/admin/accounts', '/admin/accounts/monitor', '/admin/announcements',
  '/admin/proxies', '/admin/redeem', '/admin/promo-codes',
  '/admin/scheduler-logs', '/admin/settings', '/admin/risk-control',
  '/admin/prompt-audit', '/admin/usage', '/admin/affiliates/invites',
  '/admin/affiliates/rebates', '/admin/affiliates/transfers',
  '/admin/orders/dashboard', '/admin/orders', '/admin/orders/plans',
]
const selectedRoutes = process.env.ADMIN_AUDIT_ROUTE ? routes.filter(route => route === process.env.ADMIN_AUDIT_ROUTE) : routes

function credentials() {
  const cmd = "sudo -n bash -c 'set -a; . /opt/sub2api-test-station/.env; printf \"%s\\n%s\\n\" \"$ADMIN_LAB_ADMIN_EMAIL\" \"$ADMIN_LAB_ADMIN_PASSWORD\"'"
  const result = spawnSync('ssh', ['-T', '-o', 'BatchMode=yes', '-o', 'StrictHostKeyChecking=yes', 'sub2api-test-station', cmd], { encoding: 'utf8' })
  if (result.status !== 0) throw new Error('Protected credential read failed')
  const [email, password] = result.stdout.trimEnd().split('\n')
  if (!email || !password) throw new Error('Protected credentials unavailable')
  return { email, password }
}

async function main() {
  const browser = await chromium.launch({ headless: true, executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' })
  const report = { observedAt: new Date().toISOString(), method: 'admin session; initial route states, structure only; no text, screenshots or business mutation', results: [] }
  try {
    const account = credentials()
    for (const [width, height] of [[1280, 900], [390, 844]]) {
      const context = await browser.newContext({ viewport: { width, height } })
      try {
        const page = await context.newPage()
        await page.goto('http://49.51.203.200/login', { waitUntil: 'domcontentloaded' })
        await page.getByRole('textbox', { name: '邮箱' }).fill(account.email)
        await page.getByRole('textbox', { name: '密码' }).fill(account.password)
        await page.getByRole('button', { name: '登录', exact: true }).click()
        await page.waitForURL(/\/dashboard/, { timeout: 15000 })
        for (const requested of selectedRoutes) {
          try {
            await page.goto(`http://49.51.203.200${requested}`, { waitUntil: 'domcontentloaded', timeout: 25000 })
            await page.waitForFunction(() => !!document.querySelector('main'), null, { timeout: 10000 }).catch(() => {})
            await page.waitForTimeout(1500)
            const result = await page.evaluate(() => {
              const visible = el => !!el && el.getBoundingClientRect().width > 0 && el.getBoundingClientRect().height > 0 && getComputedStyle(el).visibility !== 'hidden'
              const header = document.querySelector('header.glass.sticky')
              const headerTitle = header?.querySelector('h1')
              const main = document.querySelector('main')
              const mainTop = main?.getBoundingClientRect().top ?? 0
              const headings = [...(main?.querySelectorAll('h1,h2') || [])].filter(visible).slice(0, 8).map(el => ({
                tag: el.tagName,
                id: el.id || null,
                distanceFromMainTop: Math.round(el.getBoundingClientRect().top - mainTop),
                nextParagraph: el.nextElementSibling?.tagName === 'P' && visible(el.nextElementSibling),
                precedingParagraph: el.previousElementSibling?.tagName === 'P' && visible(el.previousElementSibling),
                parentParagraphCount: [...(el.parentElement?.children || [])].filter(child => child.tagName === 'P' && visible(child)).length,
              }))
              return {
                landed: location.pathname,
                appHeaderPresent: visible(header),
                appTitleVisible: visible(headerTitle),
                appHeaderParagraphs: header ? [...header.querySelectorAll('p')].filter(visible).length : 0,
                mainPresent: visible(main),
                headings,
                horizontalOverflow: document.documentElement.scrollWidth > innerWidth,
              }
            })
            report.results.push({ viewport: `${width}x${height}`, requested, ...result })
          } catch (error) {
            report.results.push({ viewport: `${width}x${height}`, requested, collectionError: error.name })
          }
        }
      } finally {
        await context.close()
      }
    }
  } finally {
    await browser.close()
  }
  fs.writeFileSync(path.join(__dirname, process.env.ADMIN_AUDIT_ROUTE ? 'admin-page-header-targeted.json' : 'admin-page-header-audit.json'), JSON.stringify(report, null, 2) + '\n')
  console.log(JSON.stringify({ count: report.results.length, errors: report.results.filter(r => r.collectionError), redirects: report.results.filter(r => r.landed !== r.requested).map(r => [r.viewport, r.requested, r.landed]), candidates: report.results.filter(r => r.headings?.some(h => h.nextParagraph || h.precedingParagraph)).map(r => [r.viewport, r.requested, r.headings.filter(h => h.nextParagraph || h.precedingParagraph)]) }))
}

main().catch(error => { console.error(error.message); process.exitCode = 1 })
