const { spawnSync } = require('node:child_process')
const fs = require('node:fs')
const path = require('node:path')
const { chromium } = require('playwright')

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
  const report = { observedAt: new Date().toISOString(), method: 'admin session; first visible cost editor only; no save or account identifiers recorded', results: [] }
  try {
    const account = credentials()
    for (const [width, height] of [[1280, 900], [390, 844]]) {
      const context = await browser.newContext({ viewport: { width, height } })
      try {
        const page = await context.newPage()
        await page.goto('http://49.51.203.200/login')
        await page.getByRole('textbox', { name: '邮箱' }).fill(account.email)
        await page.getByRole('textbox', { name: '密码' }).fill(account.password)
        await page.getByRole('button', { name: '登录', exact: true }).click()
        await page.waitForURL(/\/dashboard/, { timeout: 15000 })
        await page.goto('http://49.51.203.200/admin/accounts/monitor', { waitUntil: 'domcontentloaded' })
        await page.locator('[data-test="account-monitor-page"]').waitFor({ timeout: 15000 })
        await page.waitForFunction(() => !!document.querySelector('[data-test="account-card-grid"], [data-test="account-empty"], [data-test="account-monitor-error-empty"]'), null, { timeout: 15000 }).catch(() => {})
        const result = { viewport: `${width}x${height}`, landed: new URL(page.url()).pathname, cardCount: await page.locator('[data-test="account-card-grid"] > *').count(), emptyState: await page.locator('[data-test="account-empty"]').count() > 0, visibleEditors: 0, visibleUpstreamEditors: 0, guideBlocked: false, previewKind: null, suffix: null, matchesGlobalLabel: null }
        const editors = page.locator('[data-test="edit-cost"]:visible')
        const upstreamEditors = page.locator('button[title="编辑上游声明倍率"]:visible')
        result.visibleEditors = await editors.count()
        result.visibleUpstreamEditors = await upstreamEditors.count()
        if (result.visibleEditors || result.visibleUpstreamEditors) {
          if (await page.locator('.driver-overlay').count()) await page.keyboard.press('Escape')
          if (await page.locator('.driver-overlay').count()) {
            result.guideBlocked = true
          } else {
            await (result.visibleEditors ? editors : upstreamEditors).first().click({ timeout: 5000 })
            const preview = page.locator('[data-test="derived-multiplier"], [data-test="effective-cost-preview"]').first()
            await preview.waitFor({ state: 'visible', timeout: 10000 })
            const value = (await preview.locator('span').textContent() || '').trim()
            result.previewKind = await preview.getAttribute('data-test')
            result.suffix = value.endsWith('x倍率') ? 'x倍率' : value.endsWith('×') ? '×' : 'other'
            result.matchesGlobalLabel = /^\d+(?:\.\d+)?x倍率$/.test(value)
          }
        }
        report.results.push(result)

        await page.goto('http://49.51.203.200/admin/operations/account-profitability', { waitUntil: 'domcontentloaded' })
        await page.locator('[data-test="account-profitability-page"]').waitFor({ timeout: 15000 })
        await page.waitForFunction(() => !!document.querySelector('[data-test="self-purchased-empty"], [data-test^="self-purchased-card-"]'), null, { timeout: 12000 }).catch(() => {})
        const procurement = { viewport: `${width}x${height}`, landed: new URL(page.url()).pathname, cardCount: await page.locator('[data-test^="self-purchased-card-"]').count(), emptyState: await page.locator('[data-test="self-purchased-empty"]').count() > 0, visibleEditors: 0, previewKind: null, suffix: null, matchesGlobalLabel: null }
        const procurementEditors = page.locator('[data-test^="edit-procurement-"]:visible')
        procurement.visibleEditors = await procurementEditors.count()
        if (procurement.visibleEditors) {
          await procurementEditors.first().click()
          const preview = page.locator('[data-test="derived-multiplier"]')
          await preview.waitFor({ state: 'visible', timeout: 10000 })
          const value = (await preview.locator('span').textContent() || '').trim()
          procurement.previewKind = 'derived-multiplier'
          procurement.suffix = value.endsWith('x倍率') ? 'x倍率' : value.endsWith('×') ? '×' : 'other'
          procurement.matchesGlobalLabel = /^\d+(?:\.\d+)?x倍率$/.test(value)
        }
        report.results.push(procurement)
      } finally {
        await context.close()
      }
    }
  } finally {
    await browser.close()
  }
  fs.writeFileSync(path.join(__dirname, 'account-cost-preview-audit.json'), JSON.stringify(report, null, 2) + '\n')
  console.log(JSON.stringify(report.results))
}

main().catch(error => { console.error(error.message); process.exitCode = 1 })
