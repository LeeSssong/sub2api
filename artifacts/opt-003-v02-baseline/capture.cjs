const { spawnSync } = require('node:child_process')
const { createHash } = require('node:crypto')
const fs = require('node:fs')
const path = require('node:path')
const { chromium } = require('playwright')

const out = __dirname
const prototype = '/Users/awen/Documents/ChatGPT/星桥ui设计/prototype'
const files = ['index.html', 'styles.css', 'app.mjs', 'pages.mjs', 'dialogs.mjs', 'ui.mjs', 'model.mjs', 'key-columns.mjs', 'server.mjs']
const pages = [
  ['ai', '/dashboard'], ['usage', '/usage'], ['keys', '/keys'],
  ['recharge', '/redeem'], ['orders', '/orders'], ['profile', '/profile'],
]
const sizes = [[1280, 900], [390, 844]]

function credentials() {
  const cmd = "sudo -n bash -c 'set -a; . /opt/sub2api-test-station/.env; printf \"%s\\n%s\\n\" \"$ADMIN_LAB_ADMIN_EMAIL\" \"$ADMIN_LAB_ADMIN_PASSWORD\"'"
  const result = spawnSync('ssh', ['-T', '-o', 'BatchMode=yes', '-o', 'StrictHostKeyChecking=yes', 'sub2api-test-station', cmd], { encoding: 'utf8' })
  if (result.status !== 0) throw new Error(`Protected credential read failed (${result.status})`)
  const [email, password] = result.stdout.trimEnd().split('\n')
  if (!email || !password) throw new Error('Protected credentials not configured')
  return { email, password }
}

async function summary(page) {
  return page.evaluate(() => {
    const main = document.querySelector('main')
    const rateLabels = [...new Set((main?.innerText || '').match(/\d+(?:\.\d+)?x倍率/g) || [])]
    const publicSections = new Set(['我的 AI 线路', '账户权益', '最近活动', '使用分析', '请求明细', '订单列表', '账户信息与权益', '登录邮箱', '账户安全', '额度提醒', '客服支持', '修改密码', '双因素认证 (2FA)', 'Passkey'])
    return {
      path: location.pathname + location.hash,
      title: main?.querySelector('h1')?.textContent?.trim() || null,
      pageHeaderDescription: main?.querySelector('.page-head p, [data-test="page-description"]')?.textContent?.trim() || null,
      sectionTitles: [...(main?.querySelectorAll('h2') || [])].map(el => el.textContent.trim()).filter(text => publicSections.has(text)).slice(0, 8),
      alertCount: main?.querySelectorAll('[role="alert"]').length || 0,
      rateLabels,
      horizontalOverflow: document.documentElement.scrollWidth > innerWidth,
      width: innerWidth,
    }
  })
}

async function main() {
  const hashes = Object.fromEntries(files.map(name => [name, createHash('sha256').update(fs.readFileSync(path.join(prototype, name))).digest('hex')]))
  const browser = await chromium.launch({ headless: true, executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' })
  const report = { observedAt: new Date().toISOString(), prototypeHashes: hashes, results: [] }
  try {
    const { email, password } = credentials()
    for (const [width, height] of sizes) {
      const context = await browser.newContext({ viewport: { width, height }, deviceScaleFactor: 1 })
      const live = await context.newPage()
      await live.goto('http://49.51.203.200/login')
      await live.getByRole('textbox', { name: '邮箱' }).fill(email)
      await live.getByRole('textbox', { name: '密码' }).fill(password)
      await live.getByRole('button', { name: '登录', exact: true }).click()
      await live.waitForURL(/\/dashboard/, { timeout: 15000 })
      const demo = await context.newPage()
      for (const [name, route] of pages) {
        await demo.goto(`http://127.0.0.1:4183/?baseline=opt003-20260926#${name}`)
        await demo.locator('main h1').waitFor()
        const reference = await summary(demo)
        await demo.screenshot({ path: path.join(out, `prototype-${name}-${width}.png`), fullPage: true })
        await live.goto(`http://49.51.203.200${route}`)
        await live.waitForTimeout(1500)
        const actual = await summary(live)
        report.results.push({ page: name, viewport: `${width}x${height}`, prototype: reference, testStation: actual })
      }
      await context.close()
    }
  } finally {
    await browser.close()
  }
  fs.writeFileSync(path.join(out, 'capture.json'), JSON.stringify(report, null, 2) + '\n')
  console.log(JSON.stringify(report.results.map(({ page, viewport, prototype: p, testStation: t }) => ({ page, viewport, reference: p.title, actual: t.title, route: t.path, overflow: t.horizontalOverflow }))))
}

main().catch(error => { console.error(error.message); process.exitCode = 1 })
