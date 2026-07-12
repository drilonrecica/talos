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

test('overview renders health summary and navigation', async ({ page }) => {
  await mockBrowserSession(page);
  await page.goto('/overview');
  await expect(page.getByRole('heading', { name: 'Overview' })).toBeVisible();
  await expect(page.getByRole('navigation')).toBeVisible();
  await expect(page.getByText('Server', { exact: true })).toBeVisible();
  const box = await page.locator('.health-strip').boundingBox();
  expect(box?.width).toBeLessThanOrEqual(await viewportWidth(page));
});

test('server renders telemetry and historical charts', async ({ page }) => {
  await mockBrowserSession(page);
  await page.goto('/server');
  await expect(page.getByRole('heading', { name: 'Server' })).toBeVisible();
  await expect(page.getByText('CPU', { exact: true })).toBeVisible();
  await expect(
    page.getByRole('heading', { name: 'Historical telemetry' }),
  ).toBeVisible();
});

test('resource detail opens from overview', async ({ page }) => {
  await mockBrowserSession(page);
  await page.goto('/overview');
  const link = page.locator('.resources-card a').first();
  await expect(link).toBeVisible();
  const name = (await link.textContent()) ?? 'Resource';
  await link.click();
  await expect(page.getByRole('heading', { name, exact: true })).toBeVisible();
  await expect(page.locator('section.card')).toBeVisible();
});

test('events page renders', async ({ page }) => {
  await mockBrowserSession(page);
  await page.goto('/events');
  await expect(page.locator('h2', { hasText: 'Events' })).toBeVisible();
});

test('settings page renders all sections', async ({ page }) => {
  await mockBrowserSession(page);
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

test('theme matches the configured color scheme', async ({
  page,
}, testInfo) => {
  await mockBrowserSession(page);
  await page.goto('/overview');
  const theme = await expectedTheme(page);
  if (testInfo.project.name.includes('dark')) {
    expect(theme).toBe('dark');
  } else {
    expect(theme).toBe('light');
  }
});

test('mobile layout keeps content inside the viewport', async ({ page }) => {
  await mockBrowserSession(page);
  await page.goto('/overview');
  const heading = page.getByRole('heading', { name: 'Overview' });
  const box = await heading.boundingBox();
  expect(box?.width).toBeLessThanOrEqual(await viewportWidth(page));
});
