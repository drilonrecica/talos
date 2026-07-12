import { expect, test, type Page } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';

const session = {
  user: { id: 'admin', username: 'admin' },
  expiresAt: '2026-07-11T13:00:00Z',
  absoluteExpiresAt: '2026-07-11T14:00:00Z',
};

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

async function mockSetupAvailable(page: Page, available: boolean) {
  await page.route('**/api/v1/setup', (route) =>
    route.fulfill({ json: { available } }),
  );
}

async function mockAuthSession(page: Page, status: 'authenticated' | 'guest') {
  await page.route('**/api/v1/auth/session', (route) => {
    if (status === 'guest') return route.fulfill({ status: 401 });
    return route.fulfill({ json: session });
  });
}

async function mockLive(page: Page) {
  await page.route('**/api/v1/live', (route) =>
    route.fulfill({
      status: 200,
      contentType: 'text/event-stream',
      body: liveBody,
    }),
  );
}

async function mockOnboarding(page: Page, completed: boolean) {
  await page.route('**/api/v1/onboarding', (route) =>
    route.fulfill({
      json: {
        checklistDismissed: true,
        completedAt: completed ? '2026-07-11T11:00:00Z' : undefined,
      },
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

async function mockMonitorHealth(page: Page) {
  await page.route('**/api/v1/monitor-health', (route) =>
    route.fulfill({
      json: {
        at: '2026-07-11T12:00:00Z',
        metrics: [
          {
            id: 'rss',
            label: 'Memory',
            value: 42_000_000,
            unit: 'bytes',
            status: 'normal',
            help: 'RSS',
          },
        ],
      },
    }),
  );
}

async function scan(page: Page) {
  const results = await new AxeBuilder({ page })
    .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
    .analyze();
  expect(results.violations).toEqual([]);
}

test('login page has no detectable a11y violations', async ({ page }) => {
  await mockAuthSession(page, 'guest');
  await mockSetupAvailable(page, false);
  await page.goto('/login');
  await scan(page);
});

test('setup page has no detectable a11y violations', async ({ page }) => {
  await mockAuthSession(page, 'guest');
  await page.goto('/setup');
  await scan(page);
});

test('onboarding page has no detectable a11y violations', async ({ page }) => {
  await mockAuthSession(page, 'authenticated');
  await mockOnboarding(page, false);
  await mockLive(page);
  await page.goto('/onboarding');
  await expect(
    page.getByRole('navigation', { name: 'Primary navigation' }),
  ).toHaveCount(0);
  await scan(page);
});

test('watch page has no detectable a11y violations', async ({ page }) => {
  await mockAuthSession(page, 'authenticated');
  await mockOnboarding(page, true);
  await mockLive(page);
  await page.goto('/watch');
  await scan(page);
});

test('server page has no detectable a11y violations', async ({ page }) => {
  await mockAuthSession(page, 'authenticated');
  await mockOnboarding(page, true);
  await mockLive(page);
  await mockHistoryApis(page);
  await page.goto('/server');
  await scan(page);
});

test('resources page has no detectable a11y violations', async ({ page }) => {
  await mockAuthSession(page, 'authenticated');
  await mockOnboarding(page, true);
  await mockLive(page);
  await page.goto('/resources');
  await scan(page);
});

test('resource detail page has no detectable a11y violations', async ({
  page,
}) => {
  await mockAuthSession(page, 'authenticated');
  await mockOnboarding(page, true);
  await mockLive(page);
  await mockHistoryApis(page);
  await page.goto('/resources/res1');
  await scan(page);
});

test('events page has no detectable a11y violations', async ({ page }) => {
  await mockAuthSession(page, 'authenticated');
  await mockOnboarding(page, true);
  await mockLive(page);
  await page.goto('/events');
  await scan(page);
});

test('settings page has no detectable a11y violations', async ({ page }) => {
  await mockAuthSession(page, 'authenticated');
  await mockOnboarding(page, true);
  await mockLive(page);
  await mockSettings(page);
  await page.goto('/settings');
  await scan(page);
});

test('monitor health page has no detectable a11y violations', async ({
  page,
}) => {
  await mockAuthSession(page, 'authenticated');
  await mockOnboarding(page, true);
  await mockLive(page);
  await mockMonitorHealth(page);
  await page.goto('/settings/monitor-health');
  await scan(page);
});

test('diagnostics page has no detectable a11y violations', async ({ page }) => {
  await mockAuthSession(page, 'authenticated');
  await mockOnboarding(page, true);
  await mockLive(page);
  await page.goto('/settings/diagnostics');
  await scan(page);
});
