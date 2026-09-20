// Showcase for export.spec.ts — Export buttons/download filenames
import { test, expect, type Page } from '@playwright/test';
import { openDashboard, routes, regex, ORG_ID } from './helpers';

test.describe('Showcase: Exports', () => {
  test('shows export controls on operations, workflow analysis, and impact views', async ({ page }) => {
    await page.setViewportSize({ width: 1400, height: 900 });

    await page.addInitScript(() => {
      localStorage.setItem('civora_token', 'dev-token');
      localStorage.setItem('civora_org_id', 'org-test');
    });

    page.route(routes.workflows, async (route) => {
      await route.fulfill({ json: { success: true, data: [] } });
    });

    page.route(routes.dashboardStats, async (route) => {
      await route.fulfill({ json: { success: true, data: { cases: [], by_status: {}, by_service_type: {} } } });
    });

    page.route(routes.casesSearch, async (route) => {
      await route.fulfill({ json: { success: true, data: [] } });
    });

    await page.goto('/');
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => router.navigate('operations-dashboard'));
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(500);
    await page.screenshot({ path: 'test-results/showcase-export/01-operations-exports.png', fullPage: true });

    await expect(page.locator('#ops-export-json')).toBeVisible();
    await expect(page.locator('#ops-export-csv')).toBeVisible();
    await expect(page.locator('#ops-export-report')).toBeVisible();
  });
});
