const assert = require('node:assert/strict');
const { chromium } = require('/Users/awen/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright');

(async () => {
  const browser = await chromium.launch({ executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', headless: true });
  try {
    for (const width of [1730, 390]) {
      const page = await browser.newPage({ viewport: { width, height: 1000 } });
      const errors = [];
      page.on('pageerror', error => errors.push(error.message));
      await page.goto('http://127.0.0.1:4173/#keys');
      await page.evaluate(async () => {
        const M = await import('./model.mjs');
        const state = M.createSeed();
        state.keys[0].limit = 10;
        state.keys[0].rate5h = 10;
        state.keys[0].usage5h = 2;
        state.columns.keys = [];
        localStorage.setItem(M.STORAGE_KEY, JSON.stringify(state));
      });
      await page.reload();
      assert.equal(await page.locator('.key-table thead th').count(), 7, '旧版已保存列偏好应沿用七列默认布局');
      await page.locator('[data-action="columns"]').click();
      for (const column of [7, 8, 9, 10, 11, 12, 13]) await page.locator(`[name="column"][value="${column}"]`).check();
      await page.locator('#columns-form [type="submit"]').click();
      assert.equal(await page.locator('.key-table thead th').count(), 14);
      assert.equal(await page.locator('.key-table tbody tr:first-child td').count(), 14);
      await page.locator('[data-action="reset-key-quota"][data-id="key-prod"]').click();
      assert.match(await page.locator('#modal').innerText(), /确认重置额度用量/);
      await page.locator('#reset-key-usage-form [type="submit"]').click();
      const result = await page.evaluate(async () => {
        const M = await import('./model.mjs');
        const state = JSON.parse(localStorage.getItem(M.STORAGE_KEY));
        return { key: state.keys.find(key => key.id === 'key-prod'), usage: state.usage.length, overflow: document.documentElement.scrollWidth - innerWidth };
      });
      assert.ok(result.key.quotaResetAt > 0);
      assert.equal(result.usage, 60);
      assert.ok(result.overflow <= 1, `${width}px 页面横向溢出 ${result.overflow}px`);
      await page.locator('[data-action="reset-key-rate"][data-id="key-prod"]').click();
      assert.match(await page.locator('#modal').innerText(), /确认重置速率用量/);
      await page.locator('#reset-key-usage-form [type="submit"]').click();
      const resetState = await page.evaluate(async () => {
        const M = await import('./model.mjs');
        return JSON.parse(localStorage.getItem(M.STORAGE_KEY)).keys.find(key => key.id === 'key-prod');
      });
      assert.deepEqual([resetState.usage5h, resetState.usage1d, resetState.usage7d], [0, 0, 0]);
      await page.locator('[data-action="import-key"][data-id="key-prod"]').click();
      assert.match(await page.locator('#modal').innerText(), /不会自动写入外部应用/);
      assert.equal(await page.locator('[data-action="simulate-import"]').count(), 0);
      await page.locator('#modal .close-btn').click();
      await page.locator('[data-action="edit-key"][data-id="key-prod"]').click();
      await page.locator('#key-form [name="ipOn"]').check();
      await page.locator('#key-form [name="ipWhitelist"]').fill('192.0.2.1');
      await page.locator('#key-form [name="ipBlacklist"]').fill('198.51.100.1');
      await page.locator('#key-form [name="rate5h"]').fill('12.5');
      await page.locator('#key-form [name="expiresOn"]').check();
      await page.locator('#key-form [data-action="key-expires-preset"][data-days="7"]').click();
      assert.ok(await page.locator('#key-form [name="expires"]').inputValue());
      await page.locator('#key-form [type="submit"]').click();
      await page.locator('#modal').waitFor({ state: 'hidden' });
      const edited = await page.evaluate(async () => {
        const M = await import('./model.mjs');
        return JSON.parse(localStorage.getItem(M.STORAGE_KEY)).keys.find(key => key.id === 'key-prod');
      });
      assert.equal(edited.ipWhitelist, '192.0.2.1');
      assert.equal(edited.ipBlacklist, '198.51.100.1');
      assert.equal(edited.rate5h, 12.5);
      assert.ok(edited.expires);
      assert.deepEqual(errors, []);
      await page.close();
    }
    const short = await browser.newPage({ viewport: { width: 1395, height: 675 } });
    await short.goto('http://127.0.0.1:4173/#keys');
    await short.locator('[data-action="create-key"]').first().click();
    await short.locator('#key-form [name="name"]').fill('短视口密钥');
    await short.locator('#key-form .select-trigger').click();
    await short.locator('#key-form [data-action="choose-route"][data-id="gpt-pro"]').click();
    await short.locator('#key-form [name="ipOn"]').check();
    await short.locator('#key-form [name="ipWhitelist"]').fill('192.0.2.2');
    await short.locator('#key-form [name="rateOn"]').check();
    await short.locator('#key-form [name="rate1d"]').fill('20');
    await short.locator('#key-form [name="expiresOn"]').check();
    await short.locator('#key-form [data-action="key-expires-preset"][data-days="30"]').click();
    await short.locator('#key-form [type="submit"]').scrollIntoViewIfNeeded();
    const footer = await short.locator('#key-form [type="submit"]').boundingBox();
    assert.ok(footer && footer.y >= 0 && footer.y + footer.height <= 675, '短视口高级项展开后保存按钮不可达');
    await short.locator('#key-form [type="submit"]').click();
    await short.locator('#modal').waitFor({ state: 'hidden' });
    const created = await short.evaluate(async () => {
      const M = await import('./model.mjs');
      return JSON.parse(localStorage.getItem(M.STORAGE_KEY)).keys.find(key => key.name === '短视口密钥');
    });
    assert.equal(created.routeId, 'gpt-pro');
    assert.equal(created.ipWhitelist, '192.0.2.2');
    assert.equal(created.rate1d, 20);
    await short.close();
    console.log('密钥可选列、额度/速率重置及 CCS 演示边界通过');
  } finally {
    await browser.close();
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
