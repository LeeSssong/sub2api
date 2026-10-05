const { spawnSync } = require('node:child_process')
const fs = require('node:fs')
const path = require('node:path')
const { chromium } = require('playwright')

const base = 'http://49.51.203.200'
const scenarios = [
  { name: 'usage-stats', page: '/usage', api: '/api/v1/usage/stats' },
  { name: 'usage-chart', page: '/usage', api: '/api/v1/usage/dashboard/snapshot-v2' },
  { name: 'usage-list', page: '/usage', api: '/api/v1/usage' },
  { name: 'keys-list', page: '/keys', api: '/api/v1/keys' },
]

function credentials() {
  const cmd = "sudo -n bash -c 'set -a; . /opt/sub2api-test-station/.env; printf \"%s\\n%s\\n\" \"$ADMIN_LAB_ADMIN_EMAIL\" \"$ADMIN_LAB_ADMIN_PASSWORD\"'"
  const result = spawnSync('ssh', ['-T', '-o', 'BatchMode=yes', '-o', 'StrictHostKeyChecking=yes', 'sub2api-test-station', cmd], { encoding: 'utf8' })
  if (result.status !== 0) throw new Error(`Protected credential read failed (${result.status})`)
  const [email, password] = result.stdout.trimEnd().split('\n')
  if (!email || !password) throw new Error('Protected credentials not configured')
  return { email, password }
}

async function main() {
  const browser = await chromium.launch({ headless: true, executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' })
  const report = { observedAt: new Date().toISOString(), method: 'browser-local GET abort; no server-side fault', results: [] }
  try {
    const { email, password } = credentials()
    for (const [width, height] of [[1280, 900], [390, 844]]) {
      const context = await browser.newContext({ viewport: { width, height } })
      const page = await context.newPage()
      await page.goto(`${base}/login`)
      await page.getByRole('textbox', { name: '邮箱' }).fill(email)
      await page.getByRole('textbox', { name: '密码' }).fill(password)
      await page.getByRole('button', { name: '登录', exact: true }).click()
      await page.waitForURL(/\/dashboard/, { timeout: 15000 })
      for (const scenario of scenarios) {
        let blocked = 0
        const handler = route => {
          if (route.request().method() === 'GET' && new URL(route.request().url()).pathname === scenario.api) {
            blocked++
            return route.abort('failed')
          }
          return route.continue()
        }
        await page.route('**/api/v1/**', handler)
        await page.goto(`${base}${scenario.page}`)
        await page.waitForTimeout(1300)
        const view = await page.evaluate(() => {
          const main = document.querySelector('main')
          return {
            path: location.pathname,
            alertCount: main?.querySelectorAll('[role="alert"]').length || 0,
            retryButtonCount: [...(main?.querySelectorAll('button') || [])].filter(b => /重试|刷新/.test(b.textContent || '')).length,
            visibleEmptyStateCount: [...(main?.querySelectorAll('*') || [])].filter(e => e.children.length === 0 && /暂无(密钥|记录|数据)/.test(e.textContent || '') && e.getBoundingClientRect().height > 0).length,
            horizontalOverflow: document.documentElement.scrollWidth > innerWidth,
          }
        })
        report.results.push({ viewport: `${width}x${height}`, scenario: scenario.name, blocked, view })
        await page.unroute('**/api/v1/**', handler)
      }
      await context.close()
    }
  } finally {
    await browser.close()
  }
  fs.writeFileSync(path.join(__dirname, 'fault-audit.json'), JSON.stringify(report, null, 2) + '\n')
  console.log(JSON.stringify(report.results))
}

main().catch(error => { console.error(error.message); process.exitCode = 1 })
