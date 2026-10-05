const { chromium } = require('/Users/awen/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const chromePath = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
const baseUrl = 'http://127.0.0.1:4173/';
const output = path.resolve('docs/refactorUIUXv0.1/evidence/prototype-baseline/interactive');
const fixedTime = new Date('2026-09-19T12:00:00+08:00');

async function settle(page) {
  await page.evaluate(async () => {
    await document.fonts.ready;
    await Promise.all([...document.images].map(image => image.decode().catch(() => {})));
  });
}

async function fresh(page) {
  await page.goto(`${baseUrl}#recharge`);
  await page.evaluate(() => localStorage.clear());
  await page.reload();
  await page.locator('main, .main').first().waitFor();
  await settle(page);
}

async function shot(page, name) {
  await page.clock.runFor(3500);
  await settle(page);
  await page.screenshot({ path: path.join(output, name), fullPage: false });
}

(async () => {
  fs.mkdirSync(output, { recursive: true });
  const browser = await chromium.launch({ executablePath: chromePath, headless: true });
  const context = await browser.newContext({
    viewport: { width: 1730, height: 1000 },
    deviceScaleFactor: 1,
    locale: 'zh-CN',
    timezoneId: 'Asia/Shanghai',
    reducedMotion: 'reduce',
  });
  const page = await context.newPage();
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  await page.clock.install({ time: fixedTime });
  await page.clock.pauseAt(fixedTime);

  try {
    await fresh(page);
    const orderCountBefore = await page.evaluate(() => JSON.parse(localStorage.getItem('starbridge-interactive-v1')).orders.length);
    await page.getByRole('button', { name: /创建订单/ }).click();
    await page.getByRole('heading', { name: '等待支付', exact: true }).waitFor();
    const handoffText = await page.locator('.native-payment-panel').innerText();
    assert.match(handoffText, /sub 原生支付承接/);
    assert.match(handoffText, /订单编号/);
    assert.match(handoffText, /¥30\.00/);
    assert.equal(await page.getByText(/模拟支付成功|模拟取消支付/).count(), 0, '普通支付工作面仍暴露模拟结算按钮');
    assert.equal(await page.getByRole('heading', { name: '选择充值额度', exact: true }).count(), 0, '支付承接时仍显示金额选择');
    const recovery = await page.evaluate(() => JSON.parse(localStorage.getItem('starbridge-interactive-v1')).paymentRecovery);
    assert.ok(recovery?.orderId && recovery?.orderNumber, '订单创建后未保存恢复快照');
    await shot(page, 'recharge-payment-pending-native.png');

    await page.getByRole('button', { name: '取消订单', exact: true }).click();
    await page.getByRole('heading', { name: '取消待支付订单', exact: true }).waitFor();
    assert.match(await page.locator('dialog').innerText(), new RegExp(recovery.orderNumber));
    await shot(page, 'recharge-payment-cancel-confirm.png');
    await page.getByRole('button', { name: '继续支付', exact: true }).click();

    await page.getByRole('button', { name: '查看我的订单', exact: true }).click();
    const pendingRow = page.locator('.orders-table tbody tr').filter({ hasText: recovery.orderNumber });
    assert.match(await pendingRow.innerText(), /待支付/);
    assert.equal(await pendingRow.getByRole('button', { name: '取消订单', exact: true }).count(), 1);
    for (const value of ['pending', 'completed', 'failed', 'refunded']) {
      assert.equal(await page.locator(`select[aria-label="订单状态"] option[value="${value}"]`).count(), 1, `缺少原生订单筛选 ${value}`);
    }
    await shot(page, 'orders-pending-native.png');
    await pendingRow.getByRole('button', { name: '取消订单', exact: true }).click();
    await page.getByRole('button', { name: '确认取消订单', exact: true }).click();
    assert.match(await pendingRow.innerText(), /已取消/);
    assert.equal(await page.evaluate(() => JSON.parse(localStorage.getItem('starbridge-interactive-v1')).paymentRecovery), undefined, '取消后仍保留支付恢复快照');

    await fresh(page);
    await page.getByRole('button', { name: '演示设置', exact: true }).click();
    await page.getByRole('button', { name: '下一次订单创建失败', exact: true }).click();
    const failedCountBefore = await page.evaluate(() => JSON.parse(localStorage.getItem('starbridge-interactive-v1')).orders.length);
    await page.getByRole('button', { name: /创建订单/ }).click();
    assert.match(await page.locator('.payment-create-error').innerText(), /订单未创建/);
    assert.match(await page.locator('.payment-create-error').innerText(), /支付渠道暂时不可用/);
    assert.equal(await page.getByRole('heading', { name: '选择充值额度', exact: true }).count(), 1, '创建失败后未保留金额选择');
    const failedCountAfter = await page.evaluate(() => JSON.parse(localStorage.getItem('starbridge-interactive-v1')).orders.length);
    assert.equal(failedCountAfter, failedCountBefore, '创建失败仍新增了订单');
    assert.equal(failedCountBefore, orderCountBefore, '重置后的种子订单数量异常');
    await shot(page, 'recharge-payment-create-error.png');

    await page.setViewportSize({ width: 390, height: 844 });
    await fresh(page);
    await page.getByRole('button', { name: /创建订单/ }).click();
    const mobileLayout = await page.evaluate(() => ({
      overflow: Math.max(document.documentElement.scrollWidth, document.body.scrollWidth) - window.innerWidth,
      actions: [...document.querySelectorAll('.payment-handoff-actions button')].map(button => ({
        width: button.getBoundingClientRect().width,
        height: button.getBoundingClientRect().height,
      })),
    }));
    assert.ok(mobileLayout.overflow <= 1, `390×844 支付承接页面横向溢出 ${mobileLayout.overflow}px`);
    assert.equal(mobileLayout.actions.length, 3, '移动支付承接操作数量不完整');
    assert.ok(mobileLayout.actions.every(action => action.width > 0 && action.height >= 38), '移动支付承接操作不可达');

    assert.deepEqual(errors, [], '支付路径出现 pageerror');
    console.log(JSON.stringify({
      browser: browser.version(),
      screenshots: 4,
      assertions: '创建订单进入 sub 原生承接并保存恢复快照；普通流程无模拟结算按钮；待支付订单可确认取消；创建失败不新增订单且保留输入；390×844 无页面级横向溢出且操作可达；无 pageerror',
    }, null, 2));
  } finally {
    await browser.close();
  }
})().catch(error => {
  console.error(error);
  process.exitCode = 1;
});
