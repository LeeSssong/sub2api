const fs = require('node:fs')
const path = require('node:path')
const { chromium } = require('playwright')

async function main() {
  const browser = await chromium.launch({ headless: true, executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' })
  const report = { observedAt: new Date().toISOString(), method: 'fresh anonymous context for each route; structural metadata only; no form submission or page text', results: [] }
  try {
    for (const [width, height] of [[1280, 900], [390, 844]]) {
      for (const requested of ['/home', '/login', '/register', '/forgot-password', '/reset-password']) {
        const context = await browser.newContext({ viewport: { width, height } })
        try {
          const page = await context.newPage()
          await page.goto(`http://49.51.203.200${requested}`, { waitUntil: 'domcontentloaded' })
          await page.waitForFunction(() => !!document.querySelector('.card-glass h2, .card-glass [role="alert"], .card-glass form, main h1'), null, { timeout: 15000 }).catch(() => {})
          await page.waitForTimeout(300)
          const result = await page.evaluate(() => {
            const visible = el => !!el && el.getBoundingClientRect().width > 0 && el.getBoundingClientRect().height > 0 && getComputedStyle(el).visibility !== 'hidden'
            const formHeading = document.querySelector('.card-glass h2')
            const explanation = formHeading?.nextElementSibling
            const brand = document.querySelector('.card-glass')?.previousElementSibling
            return {
              landed: location.pathname,
              authCardPresent: visible(document.querySelector('.card-glass')),
              formHeadingVisible: visible(formHeading),
              formHeadingNextParagraph: explanation?.tagName === 'P' && visible(explanation),
              brandSubtitleVisible: visible(brand?.querySelector('h1 + p')),
              registrationDisabledNotice: visible(document.querySelector('.card-glass .border-amber-200')),
              pageHorizontalOverflow: document.documentElement.scrollWidth > window.innerWidth,
            }
          })
          report.results.push({ viewport: `${width}x${height}`, requested, ...result })
        } finally {
          await context.close()
        }
      }
    }
  } finally {
    await browser.close()
  }
  fs.writeFileSync(path.join(__dirname, 'anonymous-headers-audit.json'), JSON.stringify(report, null, 2) + '\n')
  console.log(JSON.stringify(report.results))
}

main().catch(error => { console.error(error.message); process.exitCode = 1 })
