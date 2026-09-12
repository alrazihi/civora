import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 1,
  // Cap parallel workers: launching many Chromium instances on a developer
  // machine (especially Windows) is unstable. CI overrides via CI env.
  workers: process.env.CI ? 1 : 2,
  reporter: process.env.CI ? 'github' : 'list',
  timeout: 30000,
  globalSetup: './global-setup',
  globalTeardown: './global-teardown',
  use: {
    baseURL: 'http://localhost:6181',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  // The static server is started externally (see web/e2e/server.js) because
  // Playwright's webServer launches the command through cmd.exe, which is
  // unreliable in this shell host. Run `node web/e2e/server.js` first, or
  // `npm run test` which starts it automatically on non-CI environments.
  // baseURL points at that server.
});
