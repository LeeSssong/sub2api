const { spawnSync } = require('node:child_process')
const fs = require('node:fs')
const path = require('node:path')
const { chromium } = require('playwright')

const allowed = ['名称', '线路', '自定义密钥', 'IP 限制', '额度限制', '速率限制', '密钥有效期']

function credentials() {
  const cmd = "sudo -n bash -c 'set -a; . /opt/sub2api-test-station/.env; printf \"%s\\n%s\\n\" \"$ADMIN_LAB_ADMIN_EMAIL\" \"$ADMIN_LAB_ADMIN_PASSWORD\"'"
  const result = spawnSync('ssh', ['-T', '-o', 'BatchMode=yes', '-o', 'StrictHostKeyChecking=yes', 'sub2api-test-station', cmd], { encoding: 'utf8' })
  if (result.status !== 0) throw new Error(`Protected credential read failed (${result.status})`)
  const [email, password] = result.stdout.trimEnd().split('\n')
  if (!email || !password) throw new Error('Protected credentials not configured')
  return { email, password }
}

async function fields(page, formId) {
  return page.evaluate(({ names, formId }) => {
    const form = document.getElementById(formId)
    const dialog = form?.closest('[role="dialog"],dialog')
    if (!dialog || dialog.getBoundingClientRect().height === 0) return { dialogVisible: false, fields: [] }
    const selectors = formId === 'key-form'
      ? ['.field > .label', '.toggle-row > span']
      : ['label.field', '#xq-line-label', '.key-option > label.toggle-row']
    const fields = [...form.querySelectorAll(selectors.join(', '))].map(el => {
      const text = [...el.childNodes].filter(node => node.nodeType === Node.TEXT_NODE).map(node => node.textContent).join('').trim() || el.textContent.trim()
      if (!names.includes(text) || el.getBoundingClientRect().height === 0) return null
      const style = getComputedStyle(el)
      return { name: text, fontSize: style.fontSize, fontWeight: style.fontWeight }
    }).filter(Boolean)
    return { dialogVisible: true, fields }
  }, { names: allowed, formId })
}

async function main() {
  const browser = await chromium.launch({ headless: true, executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' })
  const report = { observedAt: new Date().toISOString(), method: 'open create dialog without submitting', results: [] }
  try {
    const { email, password } = credentials()
    for (const [width, height] of [[1280, 900], [390, 844]]) {
      const context = await browser.newContext({ viewport: { width, height } })
      const reference = await context.newPage()
      await reference.goto('http://127.0.0.1:4183/?baseline=opt003-20260926#keys')
      await reference.locator('[data-action="create-key"]').first().click()
      const prototypeFields = await fields(reference, 'key-form')
      const live = await context.newPage()
      await live.goto('http://49.51.203.200/login')
      await live.getByRole('textbox', { name: '邮箱' }).fill(email)
      await live.getByRole('textbox', { name: '密码' }).fill(password)
      await live.getByRole('button', { name: '登录', exact: true }).click()
      await live.waitForURL(/\/dashboard/, { timeout: 15000 })
      await live.goto('http://49.51.203.200/keys')
      await live.locator('[data-tour="keys-create-btn"]').click()
      await live.waitForTimeout(350)
      const siteFields = await fields(live, 'xq-create-line-key')
      report.results.push({ viewport: `${width}x${height}`, prototype: prototypeFields, testStation: siteFields })
      await context.close()
    }
  } finally {
    await browser.close()
  }
  fs.writeFileSync(path.join(__dirname, 'modal-audit.json'), JSON.stringify(report, null, 2) + '\n')
  console.log(JSON.stringify(report.results))
}

main().catch(error => { console.error(error.message); process.exitCode = 1 })
