const assert = require('node:assert/strict');
const { chromium } = require('/Users/awen/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright');

(async () => {
  const browser = await chromium.launch({
    executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
    headless: true,
  });
  try {
    for (const width of [1730, 390]) {
      const page = await browser.newPage({ viewport: { width, height: 1000 } });
      await page.goto('http://127.0.0.1:4173/#keys');
      const layout = await page.evaluate(() => {
        const work = document.querySelector('section[aria-label="我的密钥"]');
        const filters = work.querySelector('.filters');
        const endpoint = work.querySelector('.endpoint');
        const list = work.querySelector('section.panel:not(.endpoint)');
        const border = element => getComputedStyle(element).borderLeftWidth;
        const box = element => element.getBoundingClientRect();
        return {
          workBorder: border(work), filtersBorder: border(filters), endpointBorder: border(endpoint), listBorder: border(list),
          work: { left: box(work).left, right: box(work).right },
          children: [filters, endpoint, list].map(element => ({ left: box(element).left, right: box(element).right })),
          overflow: document.documentElement.scrollWidth - innerWidth,
          endpointLines: box(endpoint.querySelector('code')).height / parseFloat(getComputedStyle(endpoint.querySelector('code')).lineHeight),
          titleLines: box(list.querySelector('h3')).height / parseFloat(getComputedStyle(list.querySelector('h3')).lineHeight),
          workBottom: box(work).bottom, previewTop: box(document.querySelector('.preview-bar')).top,
          clippedRoutes: [...work.querySelectorAll('.key-table .route-identity strong')].filter(name => name.scrollWidth > name.clientWidth + 1).length,
        };
      });
      assert.equal(layout.workBorder, '1px', `${width}px 工作面缺少统一边界`);
      assert.equal(layout.filtersBorder, '0px', `${width}px 筛选区仍是独立卡片`);
      assert.equal(layout.endpointBorder, '0px', `${width}px 端点仍是独立卡片`);
      assert.equal(layout.listBorder, '0px', `${width}px 列表仍是嵌套卡片`);
      assert.ok(layout.children.every(child => child.left >= layout.work.left && child.right <= layout.work.right), `${width}px 内容超出工作面`);
      assert.ok(layout.overflow <= 1, `${width}px 页面横向溢出 ${layout.overflow}px`);
      if (width === 390) {
        assert.ok(layout.endpointLines <= 1.1, `窄屏端点地址孤字换行：${JSON.stringify(layout)}`);
        assert.ok(layout.titleLines <= 1.1, `窄屏列表标题被说明挤压：${JSON.stringify(layout)}`);
        const row = page.locator('.key-table tbody tr').first();
        const first = await row.boundingBox();
        const actions = await row.locator('.key-actions').boundingBox();
        assert.ok(first && actions && actions.x + actions.width <= layout.work.right - 10, '窄屏行操作仍需横向滚动才能使用');
        assert.equal(await row.locator('td[data-label]').count(), await page.locator('.key-table thead th').count(), '窄屏字段缺少标签');
        assert.ok(layout.previewTop >= layout.workBottom, '演示入口覆盖密钥操作');
        assert.equal(layout.clippedRoutes, 0, '窄屏线路名称被截断');
      }
      await page.close();
    }
    console.log('密钥工作面桌面与窄屏布局通过');
  } finally {
    await browser.close();
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
