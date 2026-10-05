const assert = require('node:assert/strict');
const { chromium } = require('/Users/awen/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright');

const baseUrl = 'http://127.0.0.1:4173/#keys';

(async () => {
  const browser = await chromium.launch({
    executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
    headless: true,
  });
  try {
    const page = await browser.newPage({ viewport: { width: 390, height: 844 } });
    await page.goto(baseUrl);
    await page.locator('.key-route-control').first().click();
    const bounds = await page.evaluate(() => {
      const sidebar = document.querySelector('.sidebar').getBoundingClientRect();
      const menu = document.querySelector('.key-route-popover').getBoundingClientRect();
      return { sidebarRight: sidebar.right, menuLeft: menu.left, menuRight: menu.right, viewport: innerWidth };
    });
    assert.ok(bounds.menuLeft >= bounds.sidebarRight + 12, `线路菜单被侧栏遮挡：${JSON.stringify(bounds)}`);
    assert.ok(bounds.menuRight <= bounds.viewport - 12, `线路菜单超出右侧安全区：${JSON.stringify(bounds)}`);

    await page.setViewportSize({ width: 1280, height: 900 });
    await page.goto('http://127.0.0.1:4173/#usage');
    await page.locator('.usage-overview .brand-dropdown-trigger').click();
    const overview = await page.evaluate(() => {
      const panel = document.querySelector('.usage-overview').getBoundingClientRect();
      const menu = document.querySelector('.usage-overview .brand-dropdown-menu').getBoundingClientRect();
      return { panelRight: panel.right, menuRight: menu.right };
    });
    assert.ok(overview.menuRight <= overview.panelRight, `统计粒度菜单越过概览面板：${JSON.stringify(overview)}`);
    console.log('窄屏线路菜单和统计粒度菜单均在容器边界内');
  } finally {
    await browser.close();
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
