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

async function main() {
  const browser = await chromium.launch({ headless: true, executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' })
  const report = { observedAt: new Date().toISOString(), method: 'admin session; current page only; hover without actions; suffix metadata only', results: [] }
  try {
    const account = credentials()
    for (const [width, height] of [[1280, 900], [390, 844]]) {
      const context = await browser.newContext({ viewport: { width, height } })
      try {
        const page = await context.newPage()
        const pageErrorNames = []
        page.on('pageerror', error => pageErrorNames.push(error.name))
        await page.goto('http://49.51.203.200/login', { waitUntil: 'domcontentloaded' })
        await page.getByRole('textbox', { name: '邮箱' }).fill(account.email)
        await page.getByRole('textbox', { name: '密码' }).fill(account.password)
        await page.getByRole('button', { name: '登录', exact: true }).click()
        await page.waitForURL(/\/dashboard/, { timeout: 15000 })
        await page.goto('http://49.51.203.200/admin/usage', { waitUntil: 'domcontentloaded' })
        await page.waitForFunction(() => !!document.querySelector('main table tbody'), null, { timeout: 12000 }).catch(() => {})
        await page.waitForTimeout(5000)
        const costTriggers = page.locator('main table tbody tr span.text-green-600 ~ div.group.relative')
        const triggerCount = await costTriggers.count()
        if (triggerCount) await costTriggers.first().hover()
        if (triggerCount) await page.waitForTimeout(300)
        const result = await page.evaluate(() => {
          const visible = el => !!el && el.getBoundingClientRect().width > 0 && el.getBoundingClientRect().height > 0
          const values = [...document.querySelectorAll('body > div.fixed .text-blue-400')].filter(visible).map(el => (el.textContent || '').trim())
          return {
            landed: location.pathname,
            mainPresent: !!document.querySelector('main'),
            mainChildCount: document.querySelector('main')?.children.length ?? 0,
            appHeaderPresent: !!document.querySelector('header.glass.sticky'),
            tabCount: document.querySelectorAll('main [role="tab"]').length,
            busyCount: document.querySelectorAll('main [aria-busy="true"]').length,
            tableCount: document.querySelectorAll('main table').length,
            tbodyRowCount: document.querySelectorAll('main table tbody tr').length,
            genericCostIconCount: document.querySelectorAll('main table tbody tr div.group.relative').length,
            costValueCount: document.querySelectorAll('main table tbody tr span.text-green-600').length,
            multiplierValueCount: values.length,
            formattedCount: values.filter(value => /^\d+(?:\.\d+)?x倍率$/.test(value)).length,
            unavailableCount: values.filter(value => value === '倍率暂不可用').length,
            invalidCount: values.filter(value => !/^\d+(?:\.\d+)?x倍率$/.test(value) && value !== '倍率暂不可用').length,
            horizontalOverflow: document.documentElement.scrollWidth > innerWidth,
          }
        })
        report.results.push({ viewport: `${width}x${height}`, triggerCount, ...result, pageErrorNames })
      } finally {
        await context.close()
      }
    }
  } finally {
    await browser.close()
  }
  fs.writeFileSync(path.join(__dirname, 'admin-usage-rate-tooltip-audit.json'), JSON.stringify(report, null, 2) + '\n')
  console.log(JSON.stringify(report.results))
}

main().catch(error => { console.error(error.message); process.exitCode = 1 })
