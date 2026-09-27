const { spawnSync } = require('node:child_process')
const fs = require('node:fs')
const path = require('node:path')
const { chromium } = require('playwright')

function credentials() {
  const cmd = "sudo -n bash -c 'set -a; . /opt/sub2api-test-station/.env; printf \"%s\\n%s\\n\" \"$ADMIN_LAB_ADMIN_EMAIL\" \"$ADMIN_LAB_ADMIN_PASSWORD\"'"
  const result = spawnSync('ssh', ['-T', '-o', 'BatchMode=yes', '-o', 'StrictHostKeyChecking=yes', 'sub2api-test-station', cmd], { encoding: 'utf8' })
  if (result.status !== 0) throw new Error('Protected credential read failed')
  const [email, password] = result.stdout.trimEnd().split('\n')
  if (!email || !password) throw new Error('Protected credentials unavailable')
  return { email, password }
}

function category(request) {
  const pathname = new URL(request.url()).pathname
  if (/\/api\/v1\/admin\/usage$/.test(pathname)) return 'usage-list'
  if (/\/api\/v1\/admin\/usage\/stats$/.test(pathname)) return 'usage-stats'
  if (pathname.startsWith('/api/v1/admin/usage/')) return 'usage-other'
  return 'other'
}

async function main() {
  const browser = await chromium.launch({ headless: true, executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' })
  const report = { observedAt: new Date().toISOString(), method: 'admin GET request outcome and rendered row counts; no URLs, params, response items or text', results: [] }
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
        const requests = []
        const responses = []
        const failures = []
        page.on('request', request => {
          if (request.resourceType() === 'xhr' || request.resourceType() === 'fetch') requests.push(category(request))
        })
        page.on('requestfailed', request => {
          if (request.resourceType() === 'xhr' || request.resourceType() === 'fetch') failures.push({ category: category(request), reason: request.failure()?.errorText || 'unknown' })
        })
        page.on('response', response => {
          const request = response.request()
          if ((request.resourceType() !== 'xhr' && request.resourceType() !== 'fetch') || category(request) !== 'usage-list') return
          const entry = { category: 'usage-list', status: response.status(), itemCount: null, totalPresent: false }
          responses.push(entry)
          if (response.ok()) void response.json().then(body => {
            const data = body?.data ?? body
            entry.itemCount = Array.isArray(data?.items) ? data.items.length : null
            entry.totalPresent = typeof data?.total === 'number'
          }).catch(() => {})
        })
        await page.goto('http://49.51.203.200/admin/usage', { waitUntil: 'domcontentloaded' })
        await page.waitForFunction(() => !!document.querySelector('main [data-testid="usage-detail-tab"]'), null, { timeout: 15000 }).catch(() => {})
        await page.waitForTimeout(8000)
        const dom = await page.evaluate(() => ({
          landed: location.pathname,
          tabCount: document.querySelectorAll('main [data-testid="usage-detail-tab"]').length,
          desktopCostTriggerCount: document.querySelectorAll('main table tbody tr span.text-green-600 ~ div.group.relative').length,
          mobileFieldCount: document.querySelectorAll('main [data-field]').length,
          mobileCostTriggerCount: document.querySelectorAll('main [data-field="cost"] div.group.relative').length,
        }))
        report.results.push({ viewport: `${width}x${height}`, ...dom, requestCategories: requests, usageListResponses: responses, failedRequests: failures })
      } finally {
        await context.close()
      }
    }
  } finally {
    await browser.close()
  }
  fs.writeFileSync(path.join(__dirname, 'admin-usage-request-outcome-audit.json'), JSON.stringify(report, null, 2) + '\n')
  console.log(JSON.stringify(report.results))
}

main().catch(error => { console.error(error.message); process.exitCode = 1 })
