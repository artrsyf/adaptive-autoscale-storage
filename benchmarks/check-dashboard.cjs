const endpoints = { prometheus: process.argv[3] || 'http://127.0.0.1:9090', grafana: process.argv[4] || 'http://127.0.0.1:3000' };
// Optional visual QA: requires Node.js, Playwright, and an installed Edge browser.
// NODE_PATH can point to an existing Playwright installation.
const fs = require('node:fs');
const path = require('node:path');
const { chromium } = require('playwright');

(async () => {
  const output = process.argv[2] || 'results/dashboard-check';
  fs.mkdirSync(output, { recursive: true });
  const dashboard = JSON.parse(fs.readFileSync('deploy/grafana/dashboards/storage.json', 'utf8'));
  const dashboardUrl = process.env.DASHBOARD_URL || `${endpoints.grafana}/d/storage-baseline?from=now-10m&to=now`;
  const rangeEnd = new URL(dashboardUrl).searchParams.get('to');
  // Archived runs remove their generator; evaluate queries inside the saved interval.
  const queryTime = rangeEnd && /^\d+$/.test(rangeEnd) ? `&time=${Number(rangeEnd) / 1000 - 5}` : '';
  const checks = [];
  for (const panel of dashboard.panels) {
    for (const target of panel.targets || []) {
      const query = target.expr.replaceAll('$window', '30s').replaceAll('$node', '.*');
      const response = await fetch(`${endpoints.prometheus}/api/v1/query?query=${encodeURIComponent(query)}${queryTime}`);
      const result = await response.json();
      checks.push({ panel: panel.title, query, status: result.status, series: result.data?.result?.length ?? 0, error: result.error });
      if (result.status !== 'success') throw new Error(`${panel.title}: ${result.error}`);
    }
  }
  fs.writeFileSync(path.join(output, 'queries.json'), JSON.stringify(checks, null, 2));
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  try {
    const page = await browser.newPage({ viewport: { width: 1600, height: 1050 }, deviceScaleFactor: 1 });
    const errors = [];
    page.on('pageerror', error => errors.push(error.message));
    await page.goto(dashboardUrl, { waitUntil: 'domcontentloaded' });
    await page.getByText('Configured processing units', { exact: true }).waitFor({ timeout: 60000 });
    await page.waitForTimeout(5000);
    await page.screenshot({ path: path.join(output, 'overview.png') });
    for (const [title, file] of [['02 / Load distribution', 'distribution.png'], ['03 / Processing units and connection pools', 'processing-units.png'], ['04 / PostgreSQL bottleneck evidence', 'postgres.png'], ['05 / Container resources and telemetry health', 'resources.png']]) {
      // Grafana virtualizes rows; scroll until the requested row enters the DOM.
      const heading = page.getByText(title, { exact: true });
      for (let step = 0; step < 30 && await heading.count() === 0; step++) {
        await page.mouse.move(1200, 800);
        await page.mouse.wheel(0, 650);
        await page.waitForTimeout(250);
      }
      await heading.scrollIntoViewIfNeeded();
      await page.waitForTimeout(2500);
      await page.screenshot({ path: path.join(output, file) });
    }
    fs.writeFileSync(path.join(output, 'browser-errors.json'), JSON.stringify(errors, null, 2));
    if (errors.length) throw new Error(errors.join('\n'));
    console.log(JSON.stringify({ checkedQueries: checks.length, empty: checks.filter(check => check.series === 0).map(check => check.panel), output }));
  } finally {
    await browser.close();
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
