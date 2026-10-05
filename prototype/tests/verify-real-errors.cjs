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

async function fresh(page, route = 'usage') {
  await page.goto(`${baseUrl}#${route}`);
  await page.evaluate(() => localStorage.clear());
  await page.reload();
  await page.locator('main, .main').first().waitFor();
  await settle(page);
}

async function demoState(page, label) {
  await page.getByRole('button', { name: '演示设置', exact: true }).click();
  await page.getByRole('button', { name: label, exact: true }).click();
  await settle(page);
}

async function shot(page, name) {
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
    await demoState(page, '使用统计加载失败');
    assert.equal(await page.locator('[data-stat="总请求数"]').count(), 0, '统计失败时仍显示统计数字');
    assert.match(await page.locator('.request-error').innerText(), /用量统计加载失败/);
    assert.equal(await page.getByRole('heading', { name: '请求明细', exact: true }).count(), 1, '统计失败时未保留请求明细');
    await shot(page, 'usage-stats-error.png');
    await page.getByRole('button', { name: '重试', exact: true }).click();
    assert.equal(await page.locator('[data-stat="总请求数"]').count(), 1, '统计重试后未恢复');

    await demoState(page, '使用明细加载失败');
    assert.equal(await page.locator('[data-stat="总请求数"]').count(), 1, '明细失败时未保留统计工作面');
    assert.equal(await page.locator('.usage-table').count(), 0, '明细失败时仍显示表格');
    assert.equal(await page.getByText('暂无数据', { exact: true }).count(), 0, '明细失败被伪装成空态');
    assert.match(await page.locator('.request-error').innerText(), /使用记录加载失败/);
    await page.locator('section.panel').last().scrollIntoViewIfNeeded();
    await shot(page, 'usage-list-error.png');
    await page.getByRole('button', { name: '重试', exact: true }).click();
    assert.equal(await page.locator('.usage-table').count(), 1, '明细重试后未恢复');

    await demoState(page, '下一次地区查询失败');
    await page.getByRole('button', { name: '批量获取地区', exact: true }).click();
    assert.match(await page.locator('.usage-table').innerText(), /查询中…/);
    assert.match(await page.locator('.usage-table').innerText(), /内网地址/);
    assert.equal(await page.getByRole('button', { name: '地区查询中…', exact: true }).isDisabled(), true);
    await page.locator('section.panel').last().scrollIntoViewIfNeeded();
    await shot(page, 'usage-region-loading.png');
    await page.clock.runFor(500);
    assert.match(await page.locator('.usage-table').innerText(), /查询失败/);
    assert.match(await page.locator('#toast').innerText(), /地区查询失败，请稍后重试/);
    await shot(page, 'usage-region-error.png');
    await page.getByRole('button', { name: '批量获取地区', exact: true }).click();
    await page.clock.runFor(500);
    const regionSuccess = await page.locator('.usage-table').innerText();
    assert.match(regionSuccess, /美国（示例）|新加坡（示例）/);
    assert.match(regionSuccess, /内网地址/);
    assert.doesNotMatch(regionSuccess, /查询失败|查询中…/);
    await shot(page, 'usage-region-success.png');

    await demoState(page, '下一次 CSV 导出失败');
    await page.getByRole('button', { name: '导出 CSV', exact: true }).click();
    assert.match(await page.locator('#toast').innerText(), /导出失败，请稍后重试/);

    await fresh(page, 'ai');
    await demoState(page, '线路统计读取失败');
    const rateColumn = page.locator('.route-row').filter({ has: page.locator('.danger', { hasText: '暂不可用' }) });
    assert.ok(await rateColumn.count() > 0, '线路统计失败未显示暂不可用');
    assert.equal(await page.locator('.route-row .rate').count(), 0, '线路统计失败仍显示伪造成功率');
    await shot(page, 'ai-stats-error.png');
    await page.getByRole('button', { name: '检查线路', exact: true }).click();
    assert.ok(await page.locator('.route-row .rate').count() > 0, '线路统计刷新后未恢复');

    assert.deepEqual(errors, [], '页面出现运行错误');
    console.log(JSON.stringify({
      browser: browser.version(),
      screenshots: 6,
      assertions: '统计与明细失败不伪装零数据或空态；重试恢复；地区查询区分查询中、失败和内网；CSV 与线路统计失败反馈明确；无 pageerror',
    }, null, 2));
  } finally {
    await browser.close();
  }
})().catch(error => {
  console.error(error);
  process.exitCode = 1;
});
