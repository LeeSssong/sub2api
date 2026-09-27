const path = require('node:path')
const { chromium } = require('playwright')

const base = 'http://127.0.0.1:4183/?baseline=opt003-20260926#'
const states = [
  ['ai-detail', 'ai', '[data-action="route-details"][data-tool="Codex"]'],
  ['keys-create', 'keys', '[data-action="create-key"]'],
  ['profile-support', 'profile', '[data-action="support"]'],
  ['usage-stats-error', 'usage', '[data-action="fail-usage-stats"]'],
  ['usage-list-error', 'usage', '[data-action="fail-usage-list"]'],
  ['keys-empty', 'keys', '[data-action="empty-demo"]'],
]

async function main() {
  const browser = await chromium.launch({ headless: true, executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' })
  try {
    for (const [width, height] of [[1280, 900], [390, 844]]) {
      for (const [state, route, action] of states) {
        const context = await browser.newContext({ viewport: { width, height }, deviceScaleFactor: 1 })
        const page = await context.newPage()
        await page.goto(`${base}${route}`)
        if (state.endsWith('-error') || state === 'keys-empty') {
          await page.locator('[data-action="demo"]').click()
        }
        await page.locator(action).first().click()
        if (state === 'keys-empty') await page.locator('[data-page="keys"]').first().click()
        await page.screenshot({ path: path.join(__dirname, `prototype-state-${state}-${width}.png`), fullPage: true })
        await context.close()
      }
    }
  } finally {
    await browser.close()
  }
  console.log('12 prototype interaction-state screenshots captured')
}

main().catch(error => { console.error(error.message); process.exitCode = 1 })
