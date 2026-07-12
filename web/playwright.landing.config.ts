import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './tests/landing',
  fullyParallel: true,
  reporter: process.env.CI ? 'github' : 'list',
  use: { baseURL: 'http://127.0.0.1:4174', reducedMotion: 'reduce' },
  webServer: {
    command:
      'python3 -m http.server 4174 --bind 127.0.0.1 --directory ../landing',
    url: 'http://127.0.0.1:4174',
    reuseExistingServer: !process.env.CI,
  },
  projects: [
    { name: 'desktop', use: { ...devices['Desktop Chrome'] } },
    { name: 'mobile', use: { ...devices['Pixel 7'] } },
  ],
});
