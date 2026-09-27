const { spawnSync } = require('node:child_process')
const { chromium } = require('playwright')

const cmd = "sudo -n bash -c 'set -a; . /opt/sub2api-test-station/.env; printf \"%s\\n%s\\n\" \"$ADMIN_LAB_ADMIN_EMAIL\" \"$ADMIN_LAB_ADMIN_PASSWORD\"'"
const credentials = spawnSync('ssh', ['-T', '-o', 'BatchMode=yes', '-o', 'StrictHostKeyChecking=yes', 'sub2api-test-station', cmd], { encoding: 'utf8' })
if (credentials.status !== 0) throw new Error('Protected credential read failed')
const [email, password] = credentials.stdout.trimEnd().split('\n')
if (!email || !password) throw new Error('Protected credentials not configured')

async function main() {
  const browser = await chromium.launch({ headless: true, executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' })
  try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 900 } })
  await page.goto('http://49.51.203.200/login')
  await page.getByRole('textbox', { name: '邮箱' }).fill(email)
  await page.getByRole('textbox', { name: '密码' }).fill(password)
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await page.waitForURL(/\/dashboard/, { timeout: 15000 })
  for (const route of ['/admin/users', '/usage', '/admin/dashboard']) {
    const response = await page.goto(`http://49.51.203.200${route}`, { waitUntil: 'domcontentloaded' })
    for (const elapsed of [0, 450, 1500, 3000]) {
      if (elapsed) await page.waitForTimeout(elapsed === 450 ? 450 : elapsed === 1500 ? 1050 : 1500)
      console.log(JSON.stringify({ route, elapsed, httpStatus: response?.status(), ...await page.evaluate(() => ({ path: location.pathname, readyState: document.readyState, appChildren: document.querySelector('#app')?.children.length || 0, mainCount: document.querySelectorAll('main').length, appHeaderCount: document.querySelectorAll('header.glass.sticky').length })) }))
    }
  }
  } finally {
    await browser.close()
  }
}

main().catch(error => { console.error(error.message); process.exitCode = 1 })
