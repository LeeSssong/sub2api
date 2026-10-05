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
  const monitorOnly = process.env.HEADER_ROUTE === '/monitor'
  const browser = await chromium.launch({ headless: true, executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' })
  const report = { observedAt: new Date().toISOString(), method: 'admin session, DOM structure only; no page text or screenshots', results: [] }
  try {
    const account = credentials()
    for (const [width, height] of [[1280, 900], [390, 844]]) {
      const context = await browser.newContext({ viewport: { width, height } })
      try {
        const page = await context.newPage()
        const pageErrors = []
        page.on('pageerror', error => pageErrors.push(error.name))
        await page.goto('http://49.51.203.200/login', { waitUntil: 'domcontentloaded' })
        await page.getByRole('textbox', { name: '邮箱' }).fill(account.email)
        await page.getByRole('textbox', { name: '密码' }).fill(account.password)
        await page.getByRole('button', { name: '登录', exact: true }).click()
        await page.waitForURL(/\/dashboard/, { timeout: 15000 })
        for (const requested of (monitorOnly ? ['/monitor'] : ['/monitor', '/admin/prompt-audit', '/admin/channels/monitor', '/key-usage'])) {
          pageErrors.length = 0
          await page.goto(`http://49.51.203.200${requested}`, { waitUntil: 'domcontentloaded' })
          if (monitorOnly) {
            await page.locator('#hybrid-performance-title').waitFor({ state: 'visible', timeout: 15000 }).catch(() => {})
          } else {
            await page.waitForFunction(() => !!document.querySelector('#monitor-v2-title, #hybrid-performance-title, .page-description, [data-test="tab-panel-config"], .page-header h1, main h1'), null, { timeout: 10000 }).catch(() => {})
          }
          await page.waitForTimeout(400)
          const observed = await page.evaluate(() => {
            const visible = el => !!el && el.getBoundingClientRect().width > 0 && el.getBoundingClientRect().height > 0
            const monitorTitle = document.querySelector('#monitor-v2-title')
            const hybridTitle = document.querySelector('#hybrid-performance-title')
            const promptTitle = document.querySelector('header.mb-6 h1')
            const monitorStatus = monitorTitle?.parentElement?.nextElementSibling
            const channelTitle = document.querySelector('.page-header h1')
            const channelStatus = document.querySelector('.page-description')
            const main = document.querySelector('main')
            return {
              landed: location.pathname,
              ...(location.pathname === '/monitor' ? {
                mainPresent: !!main,
                mainChildren: [...(main?.children || [])].slice(0, 4).map(el => ({ tag: el.tagName, classes: String(el.className).slice(0, 90), test: el.getAttribute('data-test') })),
                hybridChildren: [...(document.querySelector('[data-test="hybrid-performance-view"]')?.children || [])].map(el => ({ tag: el.tagName, test: el.getAttribute('data-test') })),
                hybridPanelPresent: !!document.querySelector('[data-test="hybrid-performance-panel"]'),
                mainHeadings: [...(main?.querySelectorAll('h1,h2') || [])].slice(0, 5).map(el => ({ tag: el.tagName, id: el.id, classes: String(el.className).slice(0, 90) })),
                alertCount: main?.querySelectorAll('[role="alert"]').length ?? 0,
                injectedMonitorMode: window.__APP_CONFIG__?.channel_monitor_mode ?? null,
              } : {}),
              monitorV2: visible(monitorTitle),
              monitorAdjacentTimestamp: visible(monitorStatus) && monitorStatus?.tagName === 'P',
              monitorOverallStatus: visible(monitorTitle?.parentElement?.querySelector('[role="status"]')),
              hybridMonitor: visible(hybridTitle),
              hybridAdjacentTimestamp: visible(hybridTitle?.nextElementSibling) && hybridTitle.nextElementSibling?.tagName === 'P',
              monitorV1Hero: visible(document.querySelector('main > section.py-3')),
              monitorRouteLoading: visible(document.querySelector('section[aria-busy="true"]')),
              channelTitle: visible(channelTitle),
              channelStatus: visible(channelStatus),
              channelStatusContainsSpinner: !!channelStatus?.querySelector('svg'),
              promptTitle: visible(promptTitle),
              promptPrecedingEyebrow: visible(promptTitle?.previousElementSibling) && promptTitle.previousElementSibling?.tagName === 'P',
              keyUsageTitle: visible(document.querySelector('main h1')) && location.pathname === '/key-usage',
            }
          })
          report.results.push({ viewport: `${width}x${height}`, requested, ...observed, pageErrorNames: [...pageErrors] })
        }
      } finally {
        await context.close()
      }
    }
  } finally {
    await browser.close()
  }
  fs.writeFileSync(path.join(__dirname, monitorOnly ? 'header-monitor-mode.json' : 'header-context-audit.json'), JSON.stringify(report, null, 2) + '\n')
  console.log(JSON.stringify(report.results))
}

main().catch(error => { console.error(error.message); process.exitCode = 1 })
