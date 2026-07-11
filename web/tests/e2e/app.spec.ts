import { expect, test } from '@playwright/test';

test('renders the TALOS application shell', async ({ page }) => {
  await page.goto('/');

  await expect(page).toHaveTitle('TALOS');
  await expect(page.getByRole('heading', { name: 'TALOS' })).toBeVisible();
});

test('switches every historical range without hiding gaps', async ({
  page,
}) => {
  await page.route('**/api/v1/session', (route) =>
    route.fulfill({ status: 204 }),
  );
  await page.route('**/api/v1/live', (route) =>
    route.fulfill({
      status: 200,
      contentType: 'text/event-stream',
      body: `event: snapshot\nid: 1\ndata: {"seq":1,"ts":"2026-07-11T12:00:00Z","bootIdentity":"boot","host":{"cpuPct":10,"memoryUsedBytes":1024,"load1":0.1,"networkRxBps":2,"networkTxBps":3},"resources":[],"collectors":{}}\n\n`,
    }),
  );
  await page.route('**/api/v1/events?*', (route) =>
    route.fulfill({
      json: [
        {
          ts: '2026-07-11T11:30:00Z',
          type: 'deployment',
          summary: 'Deployment',
        },
      ],
    }),
  );
  await page.route('**/api/v1/metrics?*', (route) =>
    route.fulfill({
      json: {
        scope: 'host',
        from: '2026-07-11T11:00:00Z',
        to: '2026-07-11T12:00:00Z',
        resolution: '10s',
        series: [
          {
            metric: 'cpu',
            unit: 'percent',
            points: [
              { at: '2026-07-11T11:00:00Z', min: 1, avg: 2, max: 3, count: 1 },
            ],
          },
        ],
        gaps: [
          {
            from: '2026-07-11T11:10:00Z',
            to: '2026-07-11T11:20:00Z',
            reason: 'collector_unavailable',
          },
        ],
      },
    }),
  );
  await page.goto('/');
  await page.getByRole('link', { name: 'server', exact: true }).click();
  await expect(
    page.getByRole('heading', { name: 'Historical telemetry' }),
  ).toBeVisible();
  for (const range of ['1h', '6h', '24h', '7d', '30d'])
    await page.getByRole('button', { name: range, exact: true }).click();
  await expect(page.getByText('1 explicit data gap.')).toBeVisible();
  await expect(page.getByText('1 event annotation')).toBeVisible();
});
