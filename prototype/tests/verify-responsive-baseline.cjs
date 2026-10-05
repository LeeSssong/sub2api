const { chromium } = require('/Users/awen/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const chromePath = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
const baseUrl = 'http://127.0.0.1:4173/';
const outputRoot = path.resolve('docs/refactorUIUXv0.1/evidence/prototype-baseline');
const fixedTime = new Date('2026-09-19T12:00:00+08:00');
const pages = ['ai', 'usage', 'keys', 'recharge', 'orders', 'profile'];
const viewports = [
  { width: 1280, height: 900 },
  { width: 900, height: 1000 },
  { width: 390, height: 844 },
];

function viewportName(viewport) {
  return `${viewport.width}x${viewport.height}`;
}

async function settle(page) {
  await page.evaluate(async () => {
    await document.fonts.ready;
    await Promise.all([...document.images].map(image => image.decode().catch(() => {})));
  });
}

async function resetAndOpen(page, route) {
  await page.goto(`${baseUrl}#${route}`);
  await page.evaluate(() => localStorage.clear());
  await page.reload();
  await page.locator('main, .main').first().waitFor();
  await settle(page);
}

async function inspectLayout(page) {
  return page.evaluate(() => {
    const rootOverflow = Math.max(
      document.documentElement.scrollWidth,
      document.body.scrollWidth,
    ) - window.innerWidth;
    const clippedControls = [...document.querySelectorAll('button, input, select, textarea, a[href]')]
      .filter(element => {
        const style = getComputedStyle(element);
        if (style.display === 'none' || style.visibility === 'hidden') return false;
        const rect = element.getBoundingClientRect();
        if (!rect.width || !rect.height) return false;
        if (element.closest('.table-scroll')) return false;
        return element.scrollWidth > element.clientWidth + 1 && style.overflowX === 'hidden';
      })
      .map(element => ({
        tag: element.tagName.toLowerCase(),
        text: (element.getAttribute('aria-label') || element.textContent || element.value || '').trim().slice(0, 80),
        clientWidth: element.clientWidth,
        scrollWidth: element.scrollWidth,
      }));
    const tableOverflows = [...document.querySelectorAll('.table-scroll')].map(container => ({
      clientWidth: container.clientWidth,
      scrollWidth: container.scrollWidth,
      contained: container.scrollWidth >= container.clientWidth,
    }));
    return { rootOverflow, clippedControls, tableOverflows };
  });
}

(async () => {
  const browser = await chromium.launch({ executablePath: chromePath, headless: true });
  const results = [];

  try {
    for (const viewport of viewports) {
      const context = await browser.newContext({
        viewport,
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

      const directory = path.join(outputRoot, viewportName(viewport));
      fs.mkdirSync(directory, { recursive: true });

      for (const route of pages) {
        await resetAndOpen(page, route);
        const layout = await inspectLayout(page);
        assert.ok(layout.rootOverflow <= 1, `${viewportName(viewport)} #${route} 页面横向溢出 ${layout.rootOverflow}px`);
        assert.deepEqual(layout.clippedControls, [], `${viewportName(viewport)} #${route} 存在控件文字裁切`);
        await page.screenshot({ path: path.join(directory, `${route}.png`), fullPage: false });
        results.push({ viewport: viewportName(viewport), route, ...layout });
      }

      assert.deepEqual(errors, [], `${viewportName(viewport)} 出现 pageerror`);
      await context.close();
    }

    const viewport = { width: 1395, height: 675 };
    const context = await browser.newContext({
      viewport,
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
    await resetAndOpen(page, 'ai');
    await page.locator('[data-action="create-key"]').first().click();
    const dialog = page.locator('dialog.key-form-modal');
    await dialog.waitFor();
    for (const name of ['customOn', 'ipOn', 'rateOn', 'expiresOn']) {
      await dialog.locator(`input[name="${name}"]`).check();
    }
    await settle(page);
    const dialogMetrics = await dialog.evaluate(element => {
      const rect = element.getBoundingClientRect();
      return {
        top: rect.top,
        bottom: rect.bottom,
        height: rect.height,
        clientHeight: element.clientHeight,
        scrollHeight: element.scrollHeight,
        overflowY: getComputedStyle(element).overflowY,
      };
    });
    assert.ok(dialogMetrics.top >= -1, `创建密钥弹窗顶部超出视口：${dialogMetrics.top}px`);
    assert.ok(dialogMetrics.bottom <= viewport.height + 1, `创建密钥弹窗底部超出视口：${dialogMetrics.bottom}px`);
    assert.ok(
      dialogMetrics.scrollHeight <= dialogMetrics.clientHeight + 1 || dialogMetrics.overflowY === 'auto' || dialogMetrics.overflowY === 'scroll',
      '创建密钥弹窗内容不可滚动到达',
    );
    const scrollState = await dialog.evaluate(element => {
      element.scrollTop = element.scrollHeight;
      const submit = element.querySelector('[type="submit"]');
      const submitRect = submit.getBoundingClientRect();
      return {
        scrollTop: element.scrollTop,
        submitTop: submitRect.top,
        submitBottom: submitRect.bottom,
        viewportHeight: window.innerHeight,
      };
    });
    assert.ok(scrollState.scrollTop > 0, '创建密钥弹窗不能滚动到高级项底部');
    assert.ok(scrollState.submitTop >= 0 && scrollState.submitBottom <= viewport.height, '创建按钮滚动后仍不在视口内');
    assert.deepEqual(errors, [], '创建密钥高级项出现 pageerror');
    await page.screenshot({
      path: path.join(outputRoot, 'interactive', 'ai-create-key-advanced-1395x675.png'),
      fullPage: false,
    });
    results.push({ viewport: viewportName(viewport), route: 'ai-create-key-advanced', dialogMetrics, scrollState });
    await context.close();

    console.log(JSON.stringify({
      browser: browser.version(),
      screenshots: 19,
      assertions: '六页三档视口无页面级横向溢出、无控件文字裁切、无 pageerror；创建密钥高级项弹窗完整且可操作',
      results,
    }, null, 2));
  } finally {
    await browser.close();
  }
})().catch(error => {
  console.error(error);
  process.exitCode = 1;
});
