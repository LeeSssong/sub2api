# Pelican showcase browser fixture

Run from `frontend`: `pnpm exec vite --config vite.pelican-browser.config.ts`. Open `/browser/pelican-showcase/` on the localhost port printed by Vite (default 8771). This is a development-only entry, excluded from the production build.

The real gallery, native dialogs, statistics panel and sandbox preview renderers run against synthetic API results. The app shell and stores are fixtures; no login, production data or remote API is used.

Examples include a 1200×1600 HTML poster, 1024×768 SVG, 640×1200 portrait SVG, and an HTML page whose footer appears after a delay. Top and bottom markers make cropping visible. The group counts overlap, so the page total must remain the supplied deduplicated total.

Query variants: `?scenario=partial`, `?scenario=empty`, `?scenario=unavailable`, `?theme=light`.

Verify at desktop, short landscape and mobile viewport sizes:
- Each thumbnail includes the whole artwork; card and modal share the same logical layout.
- The modal opens in Fit mode. Its stage uses remaining viewport height, and its footer stays visible without document scrolling.
- 100% explicitly permits scrolling inside the stage; reopening resets to Fit.
- Delayed content becomes visible without continuously growing the preview.
- Filtering groups changes the summary; artwork removal does not change the recorded counts.
- Zero results, unavailable statistics and partial coverage remain distinguishable.

The HTML is rendered in the production sandbox with the production CSP. Use this fixture for local UI validation only.
