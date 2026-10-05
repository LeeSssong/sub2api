const fs = require('node:fs')
const path = require('node:path')
const { chromium } = require('playwright')

async function main() {
  const browser = await chromium.launch({ headless: true, executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' })
  const results = []
  try {
    for (const [width, height] of [[1280, 900], [390, 844]]) {
      const context = await browser.newContext({ viewport: { width, height } })
      try {
        const page = await context.newPage()
        await page.goto('http://49.51.203.200/purchase', { waitUntil: 'domcontentloaded' })
        await page.waitForTimeout(4500)
        results.push({
          viewport: `${width}x${height}`,
          requested: '/purchase',
          landed: new URL(page.url()).pathname,
          authHeadingPresent: await page.locator('.card-glass h2').count() > 0,
          mainPresent: await page.locator('main').count() > 0,
        })
      } finally {
        await context.close()
      }
    }
  } finally {
    await browser.close()
  }
  const report = { observedAt: new Date().toISOString(), method: 'fresh anonymous context; delayed route guard recheck; no text or form submission', results }
  fs.writeFileSync(path.join(__dirname, 'purchase-anonymous-recheck.json'), JSON.stringify(report, null, 2) + '\n')
  console.log(JSON.stringify(results))
}

main().catch(error => { console.error(error.name); process.exitCode = 1 })
