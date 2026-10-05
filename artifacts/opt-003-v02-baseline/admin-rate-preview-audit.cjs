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
  const scanAll = process.env.AUDIT_ALL_GROUPS === '1'
  const output = path.join(__dirname, scanAll ? 'admin-rate-preview-all-groups.json' : 'admin-rate-preview-audit.json')
  const browser = await chromium.launch({ headless: true, executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' })
  const report = { observedAt: new Date().toISOString(), method: scanAll ? 'open each group separately; preview input only; no apply or save' : 'open first group modal only; no apply or save', results: [] }
  try {
    const account = credentials()
    for (const [width, height] of [[1280, 900], [390, 844]]) {
      const context = await browser.newContext({ viewport: { width, height } })
      const page = await context.newPage()
      await page.goto('http://49.51.203.200/login', { waitUntil: 'domcontentloaded' })
      const emailField = page.getByRole('textbox', { name: '邮箱' })
      try {
        await emailField.waitFor({ state: 'visible', timeout: 15000 })
      } catch {
        await page.goto('http://49.51.203.200/login', { waitUntil: 'domcontentloaded' })
        await emailField.waitFor({ state: 'visible', timeout: 15000 })
      }
      await emailField.fill(account.email)
      await page.getByRole('textbox', { name: '密码' }).fill(account.password)
      await page.getByRole('button', { name: '登录', exact: true }).click()
      await page.waitForURL(/\/dashboard/, { timeout: 15000 })
      await page.goto('http://49.51.203.200/admin/groups')
      const triggers = page.getByTestId('group-rate-multipliers')
      await triggers.first().waitFor({ state: 'visible', timeout: 15000 }).catch(() => {})
      const result = { viewport: `${width}x${height}`, path: new URL(page.url()).pathname, groupTriggerCount: await triggers.count(), inspectedCount: 0, entryCount: 0, previewValueHasSuffix: null, loadingUnresolved: false }
      if (scanAll) result.groups = []
      report.results.push(result)
      for (let index = 0; index < (scanAll ? result.groupTriggerCount : Math.min(result.groupTriggerCount, 1)); index++) {
        if (index > 0) {
          await page.goto('http://49.51.203.200/admin/groups')
          await triggers.nth(index).waitFor({ state: 'visible', timeout: 15000 })
        }
        if (await page.locator('.driver-overlay').count()) await page.keyboard.press('Escape')
        const group = { index, entryCount: 0, previewValueHasSuffix: null, loadingUnresolved: false, guideBlocked: await page.locator('.driver-overlay').count() > 0 }
        if (group.guideBlocked) {
          if (scanAll) result.groups.push(group)
          if (scanAll) fs.writeFileSync(output, JSON.stringify(report, null, 2) + '\n')
          continue
        }
        await triggers.nth(index).click({ timeout: 5000 })
        const dialog = page.locator('.modal-overlay[role="dialog"]').last()
        await dialog.waitFor({ state: 'visible', timeout: 10000 })
        await dialog.getByText(/倍率/).first().waitFor({ state: 'visible' })
        try {
          await dialog.locator('h4').nth(1).waitFor({ state: 'visible', timeout: 5000 })
        } catch {
          group.loadingUnresolved = true
        }
        if (!group.loadingUnresolved) {
          result.inspectedCount++
          const table = dialog.locator('table').first()
          group.entryCount = await table.locator('tbody tr').count()
          result.entryCount += group.entryCount
          if (group.entryCount) {
            const factor = dialog.locator('input[placeholder="0.5"]')
            await factor.fill('1.5')
            const finalRateIndex = await table.locator('thead th').evaluateAll(ths => ths.findIndex(th => th.textContent?.includes('最终倍率')))
            if (finalRateIndex >= 0) {
              const preview = table.locator('tbody tr').first().locator('td').nth(finalRateIndex)
              group.previewValueHasSuffix = /x倍率$/.test((await preview.textContent() || '').trim())
              result.previewValueHasSuffix = result.previewValueHasSuffix === false ? false : group.previewValueHasSuffix
            }
          }
        }
        result.loadingUnresolved ||= group.loadingUnresolved
        if (scanAll) result.groups.push(group)
        if (scanAll) fs.writeFileSync(output, JSON.stringify(report, null, 2) + '\n')
      }
      await context.close()
    }
  } finally {
    await browser.close()
  }
  fs.writeFileSync(output, JSON.stringify(report, null, 2) + '\n')
  console.log(JSON.stringify(report.results))
}

main().catch(error => { console.error(error.message); process.exitCode = 1 })
