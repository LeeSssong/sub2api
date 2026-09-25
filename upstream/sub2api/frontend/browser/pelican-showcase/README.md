# Pelican showcase browser fixture

Run from `frontend`: `pnpm exec vite --config vite.pelican-browser.config.ts`. Open `/browser/pelican-showcase/` on the localhost port printed by Vite (default 8771). This is a development-only entry, excluded from the production build.

The real gallery, native dialogs, statistics panel and sandbox preview renderers run against synthetic API results. The app shell and stores are fixtures; no login, production data or remote API is used. To verify serialized measurement code after minification, build with `pnpm exec vite build --config vite.pelican-browser.config.ts --outDir /tmp/pelican-showcase-fixture-dist`, then serve it with `pnpm exec vite preview --config vite.pelican-browser.config.ts --outDir /tmp/pelican-showcase-fixture-dist --host 127.0.0.1 --port 8772`.

Examples include a 1200×1600 HTML poster, 1024×768 SVG, 640×1200 portrait SVG, an HTML page whose footer appears after a delay, viewBox-only SVG, and a 1600px responsive document. Top and bottom markers make cropping visible. The responsive case must retain its mint color, report a 1024×768 JS viewport and keep its 1200px media query false. The group counts overlap, so the page total must remain the supplied deduplicated total.

Query variants: `?scenario=partial`, `?scenario=empty`, `?scenario=unavailable`, `?theme=light`.

Verify at desktop, short landscape and mobile viewport sizes:
- Each thumbnail includes the whole artwork; card and modal share the same logical layout.
- The modal opens in Fit mode. Its stage uses remaining viewport height, and its footer stays visible without document scrolling.
- 100% explicitly permits scrolling inside the stage; reopening resets to Fit.
- Delayed content becomes visible without continuously growing the preview.
- Filtering groups changes the summary; artwork removal does not change the recorded counts.
- Zero results, unavailable statistics and partial coverage remain distinguishable.

The HTML is rendered in the production sandbox with the production CSP. Use this fixture for local UI validation only.
