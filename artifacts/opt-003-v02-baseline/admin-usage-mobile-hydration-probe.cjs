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
  const report = { observedAt: new Date().toISOString(), method: '390x844 authenticated hydration timings; structural counts and HTTP status only', samples: [], networkFailures: [] }
  try {
    const context = await browser.newContext({ viewport: { width: 390, height: 844 } })
    try {
      const page = await context.newPage()
      await page.goto('http://49.51.203.200/login', { waitUntil: 'domcontentloaded' })
      const account = credentials()
      await page.getByRole('textbox', { name: '邮箱' }).fill(account.email)
      await page.getByRole('textbox', { name: '密码' }).fill(account.password)
      await page.getByRole('button', { name: '登录', exact: true }).click()
      await page.waitForURL(/\/dashboard/, { timeout: 15000 })
      page.on('response', response => {
        if (response.status() >= 400) report.networkFailures.push({ status: response.status(), resourceType: response.request().resourceType() })
      })
      page.on('requestfailed', request => report.networkFailures.push({ status: 'network-failed', resourceType: request.resourceType() }))
      await page.goto('http://49.51.203.200/admin/usage', { waitUntil: 'domcontentloaded' })
      let elapsed = 0
      for (const target of [1, 5, 15]) {
        await page.waitForTimeout((target - elapsed) * 1000)
        elapsed = target
        report.samples.push({ seconds: target, ...await page.evaluate(() => {
          const main = document.querySelector('main')
          return {
            landed: location.pathname,
            mainPresent: !!main,
            mainFirstChildTag: main?.firstElementChild?.tagName ?? null,
            mainFirstChildClass: typeof main?.firstElementChild?.className === 'string' ? main.firstElementChild.className.slice(0, 80) : null,
            detailTabCount: main?.querySelectorAll('[data-testid="usage-detail-tab"]').length ?? 0,
            tableCount: main?.querySelectorAll('table').length ?? 0,
            mobileFieldCount: main?.querySelectorAll('[data-field]').length ?? 0,
            desktopTableWrapperCount: main?.querySelectorAll('.table-wrapper').length ?? 0,
            tbodyRowCount: main?.querySelectorAll('table tbody tr').length ?? 0,
            costIconCount: main?.querySelectorAll('table tbody tr div.group.relative').length ?? 0,
            spinnerCount: main?.querySelectorAll('.animate-spin').length ?? 0,
          }
        }) })
      }
    } finally {
      await context.close()
    }
  } finally {
    await browser.close()
  }
  fs.writeFileSync(path.join(__dirname, 'admin-usage-mobile-hydration-probe.json'), JSON.stringify(report, null, 2) + '\n')
  console.log(JSON.stringify(report))
}

main().catch(error => { console.error(error.message); process.exitCode = 1 })
