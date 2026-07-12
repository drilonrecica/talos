import { expect, test, type Page } from '@playwright/test';

const session = {
  user: { id: 'admin', username: 'admin' },
  expiresAt: '2026-07-11T13:00:00Z',
  absoluteExpiresAt: '2026-07-11T14:00:00Z',
};

async function mockBrowserSession(page: Page) {
  await page.route('**/api/v1/auth/session', (route) =>
    route.fulfill({ json: session }),
  );
  await page.route('**/api/v1/onboarding', (route) =>
    route.fulfill({
      json: {
        checklistDismissed: true,
        completedAt: '2026-07-11T11:00:00Z',
      },
    }),
  );
}

const snapshot = {
  seq: 1,
  ts: '2026-07-11T12:00:00Z',
  bootIdentity: 'boot',
  host: {
    cpuPct: 10,
    memoryUsedBytes: 1024,
    memoryTotalBytes: 2048,
    diskUsedBytes: 100,
    diskTotalBytes: 200,
    load1: 0.1,
    networkRxBps: 2,
    networkTxBps: 3,
  },
  resources: [
    {
      id: 'res1',
      name: 'web-app',
      status: 'healthy',
      cpuHostPct: 5,
      memoryBytes: 512,
      category: 'applications',
      components: [{ id: 'c1', name: 'web-app-1', status: 'healthy' }],
    },
    {
      id: 'infra1',
      name: 'proxy',
      status: 'healthy',
      cpuHostPct: 1,
      memoryBytes: 128,
      category: 'infrastructure',
      infrastructure: true,
    },
  ],
  collectors: { host: { state: 'healthy' }, docker: { state: 'healthy' } },
};

const liveBody = `event: snapshot\nid: 1\ndata: ${JSON.stringify(snapshot)}\n\n`;

async function mockLive(page: Page) {
  await page.route('**/api/v1/live', (route) =>
    route.fulfill({
      status: 200,
      contentType: 'text/event-stream',
      body: liveBody,
    }),
  );
}

async function mockHistoryApis(page: Page) {
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
              {
                at: '2026-07-11T11:00:00Z',
                min: 1,
                avg: 2,
                max: 3,
                count: 1,
              },
            ],
          },
        ],
        gaps: [],
      },
    }),
  );
  await page.route('**/api/v1/events?*', (route) =>
    route.fulfill({ json: [] }),
  );
}

async function mockSettings(page: Page) {
  const values: Record<
    string,
    { value: string; source: string; applyMode: string }
  > = {
    'collection.host_interval': {
      value: '10s',
      source: 'Default',
      applyMode: 'live',
    },
    'collection.container_interval': {
      value: '10s',
      source: 'Default',
      applyMode: 'live',
    },
    'persistence.raw_interval': {
      value: '10s',
      source: 'Default',
      applyMode: 'live',
    },
    'retention.preset': {
      value: 'balanced',
      source: 'Default',
      applyMode: 'live',
    },
    'retention.raw': { value: '24h', source: 'Default', applyMode: 'live' },
    'retention.one_minute': {
      value: '7d',
      source: 'Default',
      applyMode: 'live',
    },
    'retention.fifteen_minute': {
      value: '30d',
      source: 'Default',
      applyMode: 'live',
    },
    'retention.one_hour': {
      value: '365d',
      source: 'Default',
      applyMode: 'live',
    },
    'database.target_budget_bytes': {
      value: '1073741824',
      source: 'Default',
      applyMode: 'live',
    },
    'sessions.idle_timeout': {
      value: '15m',
      source: 'Default',
      applyMode: 'live',
    },
    'sessions.absolute_lifetime': {
      value: '24h',
      source: 'Default',
      applyMode: 'live',
    },
    'http.listen_address': {
      value: ':8080',
      source: 'Default',
      applyMode: 'restart_required',
    },
    'docker.socket_path': {
      value: '/var/run/docker.sock',
      source: 'Default',
      applyMode: 'restart_required',
    },
    'paths.host_proc': {
      value: '/host/proc',
      source: 'Default',
      applyMode: 'restart_required',
    },
    'paths.host_sys': {
      value: '/host/sys',
      source: 'Default',
      applyMode: 'restart_required',
    },
    'paths.data_dir': {
      value: '/var/lib/binnacle',
      source: 'Default',
      applyMode: 'restart_required',
    },
  };
  await page.route('**/api/v1/settings', (route) =>
    route.fulfill({ json: { revision: 1, values } }),
  );
}

async function expectedTheme(page: Page) {
  return page.evaluate(() => document.documentElement.dataset.theme ?? '');
}

async function viewportWidth(page: Page) {
  return page.viewportSize()?.width ?? 0;
}

test('watch renders host instruments and the resource roster', async ({
  page,
}) => {
  await mockBrowserSession(page);
  await mockLive(page);
  await page.goto('/watch');
  await expect(page.getByRole('heading', { name: 'Watch' })).toBeVisible();
  await expect(page.getByRole('navigation')).toBeVisible();
  const brandMark = page.locator('.app-brand img:visible');
  await expect(brandMark).toBeVisible();
  expect(
    await brandMark.evaluate((image) => image.naturalWidth),
  ).toBeGreaterThan(0);
  await expect(
    page.getByRole('link', { name: 'Server', exact: true }),
  ).toBeVisible();
  const box = await page.locator('.host-band').boundingBox();
  expect(box?.width).toBeLessThanOrEqual(await viewportWidth(page));
});

test('login renders the dark Binnacle access gate', async ({ page }) => {
  await page.route('**/api/v1/auth/session', (route) =>
    route.fulfill({ status: 401 }),
  );
  await page.route('**/api/v1/setup', (route) =>
    route.fulfill({ json: { available: false } }),
  );
  await page.goto('/login');
  const mark = page.locator('.access-brand img');
  await expect(mark).toBeVisible();
  expect(await mark.evaluate((image) => image.naturalWidth)).toBeGreaterThan(0);
});

test('server renders telemetry and historical charts', async ({ page }) => {
  await mockBrowserSession(page);
  await mockLive(page);
  await mockHistoryApis(page);
  await page.goto('/server');
  await expect(
    page.getByRole('heading', { name: 'Instrumentation sheet' }),
  ).toBeVisible();
  await expect(page.getByText('CPU / CURRENT', { exact: true })).toBeVisible();
  await expect(
    page.getByRole('heading', { name: 'Historical telemetry' }),
  ).toBeVisible();
});

test('resource inspector opens from watch and links to the full record', async ({
  page,
}) => {
  await mockBrowserSession(page);
  await mockLive(page);
  await mockHistoryApis(page);
  await page.goto('/watch');
  const link = page.locator('.resource-roster tbody a').first();
  await expect(link).toBeVisible();
  const name = (await link.textContent()) ?? 'Resource';
  await link.click();
  await expect(page).toHaveURL(/\/watch\?inspect=res1/);
  await expect(page.getByRole('heading', { name, exact: true })).toBeVisible();
  await expect(
    page.getByRole('link', { name: 'Open full record' }),
  ).toBeVisible();
  await page.goBack();
  await expect(page).toHaveURL(/\/watch$/);
});

test('events page renders', async ({ page }) => {
  await mockBrowserSession(page);
  await mockLive(page);
  await mockHistoryApis(page);
  await page.goto('/events');
  await expect(
    page.getByRole('heading', { name: 'Event logbook' }),
  ).toBeVisible();
});

test('settings page renders all sections', async ({ page }) => {
  await mockBrowserSession(page);
  await mockLive(page);
  await mockSettings(page);
  await page.goto('/settings');
  await expect(page.getByRole('heading', { name: 'Collection' })).toBeVisible();
  await expect(
    page.getByRole('heading', { name: 'Retention & storage' }),
  ).toBeVisible();
  await expect(
    page.getByRole('heading', { name: 'Authentication' }),
  ).toBeVisible();
});

test('first-time users receive the dark signature theme', async ({ page }) => {
  await mockBrowserSession(page);
  await mockLive(page);
  await page.goto('/watch');
  const theme = await expectedTheme(page);
  expect(theme).toBe('dark');
});

test('mobile layout keeps content inside the viewport', async ({ page }) => {
  await mockBrowserSession(page);
  await mockLive(page);
  await page.goto('/watch');
  const heading = page.getByRole('heading', { name: 'Watch' });
  const box = await heading.boundingBox();
  expect(box?.width).toBeLessThanOrEqual(await viewportWidth(page));
});
