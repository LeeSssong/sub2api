# Pelican report assets

The card follows `design-preview/pelican-report/refined.html`: 448 × 610 CSS
pixels, rendered at device scale 2 to a 896 × 1220 PNG. The logo is the approved
local `xingqiao-mark.png`. This directory is required beside `pelican_renderer.py`.

## Python integration

```python
from pelican_renderer import build_report_html, render_report_png

html = build_report_html(normalized_public_snapshot, thumbnail_png=None)
png_bytes = await render_report_png(normalized_public_snapshot, timeout_seconds=45)
```

`render_report_png` optionally accepts `executable_path` for an existing Chromium.
It does not fetch data, run detectors, call models, write files, or send messages.
The caller owns obtaining the normalized schema-v1 snapshot and storing the PNG.
Dependencies are Pillow, Python Playwright, a locally installed Chromium, and a
Chinese font (`Noto Sans CJK SC` on Linux; PingFang SC on macOS). Local verification
uses Playwright 1.55.0 installed only under `/tmp`, with a preinstalled browser.

## Display semantics

- Counts refer to the selected public group. The continuous bar is the aggregate
  success proportion, explicitly **not** a series of per-attempt results.
- No candy check, price multiplier, assumed 60-run history, or invented average.
- `fetched_at` is labeled data acquisition time; `latest.generated_at` is labeled
  artwork record time. `latency_ms` is the latest record's generation duration.
- Missing, incomplete and stale data remain visible; absent counts never become 0.
- Artwork is fully contained in the preview window. Missing/unsupported artwork
  shows a placeholder while the public statistical section stays available.

## Artwork isolation

The model response is never inserted in the report. Only HTML/CSS and inline SVG
are supported: active tags, scripts, event handlers, navigation and animations
are removed. Script-only canvas output is unsupported and gets a placeholder.
The bounded source is rendered in an opaque sandbox iframe inside a fresh
JavaScript-disabled browser context. A restrictive CSP, offline context and an
abort-all request route prevent remote and file URL loads. The report is then
rendered in a separate fresh offline context using only the resulting PNG.

Source size, DOM node count, content dimensions, operation timeouts and overall
render time are bounded. The default overall deadline is 45 seconds, including
browser startup; context/browser cleanup has separate short bounds. No stored
browser profile, cookies or authenticated browser state is reused.

Run focused static tests with:

```sh
python3 -m unittest discover -s tests -p test_pelican_renderer.py -v
```

Set `PELICAN_TEST_CHROMIUM` to a local Chromium executable and install Playwright
in the test interpreter to enable the real PNG/containment tests.
