const fs = require('node:fs')
const path = require('node:path')
const { chromium } = require('playwright')

const staticRoutes = [
  '/legal/admin-compliance', '/legal/opt003-audit-nonexistent',
  '/model-plaza', '/key-usage',
  '/batch-image', '/affiliate', '/available-channels', '/subscriptions',
  '/payment/qrcode', '/purchase', '/orders',
]

async function main() {
  const browser = await chromium.launch({ headless: true, executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' })
  const report = { observedAt: new Date().toISOString(), method: 'fresh anonymous context; structure only; no text, screenshot, or form submission', results: [] }
  try {
    const response = await fetch('http://49.51.203.200/api/v1/settings/public')
    if (!response.ok) throw new Error(`Public settings HTTP ${response.status}`)
    const settings = await response.json()
    const agreements = settings.data?.login_agreement_documents ?? []
    const routes = [
      ...staticRoutes.map(url => ({ url, label: url, dynamic: false })),
      ...agreements.map((doc, index) => ({
        url: `/legal/${encodeURIComponent(doc.id)}`,
        label: `/legal/configured#${index + 1}`,
        dynamic: true,
      })),
    ]
    report.configuredAgreementCount = agreements.length
    for (const [width, height] of [[1280, 900], [390, 844]]) {
      for (const route of routes) {
        const context = await browser.newContext({ viewport: { width, height } })
        try {
          const page = await context.newPage()
          await page.goto(`http://49.51.203.200${route.url}`, { waitUntil: 'domcontentloaded', timeout: 20000 })
          await page.waitForFunction(() => {
            const main = document.querySelector('main')
            return location.pathname === '/login' ? !!document.querySelector('.card-glass h2') : !!main?.querySelector('h1')
          }, null, { timeout: 15000 }).catch(() => {})
          await page.waitForTimeout(1000)
          const result = await page.evaluate(() => {
            const visible = el => !!el && el.getBoundingClientRect().width > 0 && el.getBoundingClientRect().height > 0 && getComputedStyle(el).visibility !== 'hidden'
            const main = document.querySelector('main')
            const h1 = [...(main?.querySelectorAll('h1') || [])].find(visible)
            const next = h1?.nextElementSibling
            const legalArticle = main?.querySelector('article')
            const plazaDescription = main?.querySelector('.plaza-description')
            return {
              landed: location.pathname,
              mainPresent: visible(main),
              mainH1Visible: visible(h1),
              headingNextTag: next?.tagName || null,
              headingNextParagraphVisible: next?.tagName === 'P' && visible(next),
              legalArticleVisible: visible(legalArticle),
              legalUpdatedAtVisible: !!legalArticle && next?.tagName === 'P' && visible(next),
              legalContentVisible: visible(main?.querySelector('.legal-document-content')),
              legalErrorOrNotFound: !legalArticle && visible(main?.querySelector('section h1')),
              plazaDescriptionVisible: visible(plazaDescription),
              plazaAnonymousHintVisible: visible(main?.querySelector('p.flex.items-center')),
              horizontalOverflow: document.documentElement.scrollWidth > innerWidth,
            }
          })
          report.results.push({
            viewport: `${width}x${height}`,
            requested: route.label,
            ...result,
            landed: route.dynamic ? (result.landed === route.url ? 'configured-legal' : 'other') : result.landed,
          })
        } catch (error) {
          report.results.push({ viewport: `${width}x${height}`, requested: route.label, collectionError: error.name })
        } finally {
          await context.close()
        }
      }
    }
  } finally {
    await browser.close()
  }
  fs.writeFileSync(path.join(__dirname, 'public-route-header-audit.json'), JSON.stringify(report, null, 2) + '\n')
  console.log(JSON.stringify(report.results))
}

main().catch(error => { console.error(error.message); process.exitCode = 1 })
