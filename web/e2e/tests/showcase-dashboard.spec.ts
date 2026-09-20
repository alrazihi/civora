// Showcase for dashboard.spec.ts — Dashboard rendering
import { test, expect, type Page } from '@playwright/test';
import { openDashboard, routes } from './helpers';

test.describe('Showcase: Dashboard', () => {
  test('renders stats and recent cases', async ({ page }) => {
    await page.setViewportSize({ width: 1400, height: 900 });

    const cases = [
      { id: 'case-1', case_number: 'CR-1', status: 'IN_PROGRESS', service_type: 'EMERGENCY', priority: 'HIGH', title: 'Open', updated_at: '2026-09-12T01:00:00Z' },
      { id: 'case-2', case_number: 'CR-2', status: 'CLOSED', service_type: 'EMERGENCY', priority: 'NORMAL', title: 'Closed', updated_at: '2026-09-12T02:00:00Z' },
      { id: 'case-3', case_number: 'CR-3', status: 'NEW', service_type: 'EMERGENCY', priority: 'NORMAL', title: 'New', updated_at: '2026-09-12T03:00:00Z' },
    ];

    page.route(routes.casesSearch, async (route, request) => {
      if (request.method() === 'GET') {
        await route.fulfill({ json: { success: true, data: cases } });
      } else {
        await route.continue();
      }
    });

    page.route(routes.dashboardStats, async (route) => {
      await route.fulfill({
        json: {
          success: true,
          data: {
            cases,
            by_status: { IN_PROGRESS: 1, CLOSED: 1, NEW: 1 },
            by_service_type: { EMERGENCY: 3 },
          },
        },
      });
    });

    await openDashboard(page);
    await page.waitForTimeout(500);
    await page.screenshot({ path: 'test-results/showcase-dashboard/01-dashboard.png', fullPage: true });

    await expect(page.locator('#dashboard-stats')).toBeVisible();
    await expect(page.locator('#dashboard-stats')).toContainText(/Open Cases/);
    await expect(page.locator('#dashboard-stats')).toContainText(/Closed Cases/);
    await expect(page.locator('#dashboard-breakdown')).toContainText(/IN_PROGRESS/);
    await expect(page.locator('#dashboard-breakdown')).toContainText(/CLOSED/);

    const rows = page.locator('#dashboard-table tbody tr');
    await expect(rows).toHaveCount(3);
    await expect(page.locator('#dashboard-table')).toContainText(/Closed/);
  });
});
