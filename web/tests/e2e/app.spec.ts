import { expect, test } from '@playwright/test';

test('renders the Binnacle application shell', async ({ page }) => {
  await page.goto('/');

  await expect(page).toHaveTitle('Binnacle');
  await expect(page.getByRole('heading', { name: 'Binnacle' })).toBeVisible();
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
  await page.route('**/api/v1/metrics?*', (route) => {
    const query = new URL(route.request().url()).searchParams;
    const span =
      new Date(query.get('to')!).getTime() -
      new Date(query.get('from')!).getTime();
    return route.fulfill({
      json: {
        scope: 'host',
        from: '2026-07-11T11:00:00Z',
        to: '2026-07-11T12:00:00Z',
        resolution: span > 10 * 24 * 60 * 60 * 1000 ? '1h' : '10s',
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
    });
  });
  await page.goto('/');
  await page.getByRole('link', { name: 'server', exact: true }).click();
  await expect(
    page.getByRole('heading', { name: 'Historical telemetry' }),
  ).toBeVisible();
  for (const range of ['1h', '6h', '24h', '7d', '30d'])
    await page.getByRole('button', { name: range, exact: true }).click();
  await expect(page.getByText('Resolution: 1h.')).toBeVisible();
  await expect(page.getByText('1 explicit data gap.')).toBeVisible();
  await expect(page.getByText('1 event annotation')).toBeVisible();
  await page.getByText('1 data gap', { exact: true }).click();
  await expect(page.getByText(/collector unavailable/)).toBeVisible();
  const inspector = page.getByRole('button', {
    name: 'CPU (host-normalized %) chart inspection',
  });
  await inspector.focus();
  await page.keyboard.press('ArrowRight');
  await expect(inspector).toContainText('Selected point');
  await page.getByRole('button', { name: 'Custom' }).click();
  await page
    .getByRole('textbox', { name: 'From', exact: true })
    .fill('2026-07-11T12:00');
  await page
    .getByRole('textbox', { name: 'To', exact: true })
    .fill('2026-07-10T12:00');
  await page.getByRole('button', { name: 'Apply range' }).click();
  await expect(page.getByRole('alert')).toContainText(
    'end time after the start',
  );
  await page.setViewportSize({ width: 390, height: 844 });
  const box = await page
    .getByRole('heading', { name: 'Historical telemetry' })
    .boundingBox();
  expect(box?.width).toBeLessThanOrEqual(390);
});

test('requires typed confirmation for history deletion', async ({ page }) => {
  await page.route('**/api/v1/session', (route) =>
    route.fulfill({ status: 204 }),
  );
  await page.route('**/api/v1/live', (route) =>
    route.fulfill({ status: 200, contentType: 'text/event-stream', body: '' }),
  );
  await page.route('**/api/v1/history/deletion-previews', (route) =>
    route.fulfill({
      json: {
        token: 'preview',
        confirmation: 'RESET ALL HISTORY',
        totalRows: 42,
        expiresAt: '2026-07-11T13:00:00Z',
      },
    }),
  );
  await page.route('**/api/v1/history/deletion-jobs', (route) =>
    route.fulfill({
      status: 202,
      json: { id: 'del_test', state: 'queued', totalRows: 42, deletedRows: 0 },
    }),
  );
  await page.route('**/api/v1/history/deletion-jobs/del_test', (route) =>
    route.fulfill({
      json: {
        id: 'del_test',
        state: 'completed',
        totalRows: 42,
        deletedRows: 42,
      },
    }),
  );
  await page.goto('/');
  await page.getByRole('link', { name: 'settings', exact: true }).click();
  await page.getByLabel('Scope').selectOption('all');
  await page.getByRole('button', { name: 'Preview deletion' }).click();
  const remove = page.getByRole('button', { name: 'Delete history' });
  await expect(remove).toBeDisabled();
  await page.getByLabel('Confirmation').fill('RESET ALL HISTORY');
  await remove.click();
  await expect(
    page.getByText('completed: 42 of 42 rows deleted.'),
  ).toBeVisible();
});
