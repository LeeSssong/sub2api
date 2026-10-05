const { spawnSync } = require('node:child_process')
const fs = require('node:fs')
const path = require('node:path')
const { chromium } = require('playwright')

const routes = ['/batch-image', '/affiliate', '/available-channels', '/subscriptions']

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
  const report = { observedAt: new Date().toISOString(), method: 'admin session, structural metadata only; no account text, screenshots, or form submission after login', results: [] }
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
        for (const requested of routes) {
          await page.goto(`http://49.51.203.200${requested}`, { waitUntil: 'domcontentloaded' })
          await page.waitForFunction(() => !!document.querySelector('main, .card-glass'), null, { timeout: 12000 }).catch(() => {})
          await page.waitForTimeout(1500)
          const result = await page.evaluate(() => {
            const visible = el => !!el && el.getBoundingClientRect().width > 0 && el.getBoundingClientRect().height > 0 && getComputedStyle(el).visibility !== 'hidden'
            const header = document.querySelector('header.glass.sticky')
            const title = header?.querySelector('h1')
            const main = document.querySelector('main')
            const headings = [...(main?.querySelectorAll('h1,h2,h3') || [])].filter(visible).slice(0, 5).map(el => ({
              tag: el.tagName,
              nextParagraph: el.nextElementSibling?.tagName === 'P' && visible(el.nextElementSibling),
              parentClass: String(el.parentElement?.className || '').slice(0, 70),
            }))
            return {
              landed: location.pathname,
              appHeaderPresent: visible(header),
              appTitleVisible: visible(title),
              appHeaderParagraphs: header ? [...header.querySelectorAll('p')].filter(visible).length : 0,
              mainPresent: visible(main),
              mainHeadings: headings,
              horizontalOverflow: document.documentElement.scrollWidth > innerWidth,
            }
          })
          report.results.push({ viewport: `${width}x${height}`, requested, ...result })
        }
      } finally {
        await context.close()
      }
    }
  } finally {
    await browser.close()
  }
  fs.writeFileSync(path.join(__dirname, 'authenticated-user-headers-audit.json'), JSON.stringify(report, null, 2) + '\n')
  console.log(JSON.stringify(report.results))
}

main().catch(error => { console.error(error.message); process.exitCode = 1 })
