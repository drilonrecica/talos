import { createHash } from 'node:crypto';
import { readFile } from 'node:fs/promises';
import { expect, test } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';

test('field manual assets, links, and accessibility are valid', async ({
  page,
}) => {
  const responses: number[] = [];
  page.on('response', (response) => {
    if (response.url().startsWith('http://127.0.0.1:4174'))
      responses.push(response.status());
  });
  await page.goto('/');
  await expect(page.getByRole('heading', { level: 1 })).toContainText(
    'Know what your server is doing',
  );
  await expect(
    page.locator('img[src="assets/watch-console.png"]'),
  ).toBeVisible();
  await expect(page.locator('html')).toHaveCSS('color-scheme', 'dark');
  expect(responses.every((status) => status < 400)).toBe(true);
  const results = await new AxeBuilder({ page })
    .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
    .analyze();
  expect(results.violations).toEqual([]);
});

test('landing Watch screenshot matches the tested app baseline', async () => {
  const digest = async (path: string) =>
    createHash('sha256')
      .update(await readFile(path))
      .digest('hex');
  expect(await digest('../landing/assets/watch-console.png')).toBe(
    await digest(
      'tests/e2e/watch-visual.spec.ts-snapshots/watch-dark-chromium-linux.png',
    ),
  );
});

test('field manual visual baseline', async ({ page }, testInfo) => {
  await page.goto('/');
  await expect(page).toHaveScreenshot(`landing-${testInfo.project.name}.png`, {
    fullPage: true,
    animations: 'disabled',
  });
});
