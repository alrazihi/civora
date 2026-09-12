// Dashboard E2E Tests

import { test, expect, type Page } from '@playwright/test';
import {
  openDashboard,
  routes,
} from './helpers';

test.describe('Dashboard', () => {
  async function openDashboardWithStats(page: Page) {
    const cases = [
      { id: 'case-1', case_number: 'CR-1', status: 'IN_PROGRESS', service_type: 'EMERGENCY', priority: 'HIGH', title: 'Open', updated_at: '2026-09-12T01:00:00Z' },
      { id: 'case-2', case_number: 'CR-2', status: 'CLOSED', service_type: 'EMERGENCY', priority: 'NORMAL', title: 'Closed', updated_at: '2026-09-12T02:00:00Z' },
      { id: 'case-3', case_number: 'CR-3', status: 'NEW', service_type: 'EMERGENCY', priority: 'NORMAL', title: 'New', updated_at: '2026-09-12T03:00:00Z' },
    ];
    // Case list (rendered as the recent cases table).
    page.route(routes.casesSearch, async (route, request) => {
      if (request.method() === 'GET') {
        await route.fulfill({ json: { success: true, data: cases } });
      } else {
        await route.continue();
      }
    });
    // Dashboard statistics including the by_status breakdown.
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
  }

  test('shows the terminal-state based status breakdown', async ({ page }) => {
    await openDashboardWithStats(page);
    await expect(page.locator('#dashboard-stats')).toBeVisible();
    // Stat cards report open vs closed case counts.
    await expect(page.locator('#dashboard-stats')).toContainText(/Open Cases/);
    await expect(page.locator('#dashboard-stats')).toContainText(/Closed Cases/);
    // Breakdown renders the by_status buckets.
    await expect(page.locator('#dashboard-breakdown')).toContainText(/IN_PROGRESS/);
    await expect(page.locator('#dashboard-breakdown')).toContainText(/CLOSED/);
  });

  test('lists recent cases', async ({ page }) => {
    await openDashboardWithStats(page);
    const rows = page.locator('#dashboard-table tbody tr');
    // One row per case (3).
    await expect(rows).toHaveCount(3);
    await expect(page.locator('#dashboard-table')).toContainText(/Closed/);
  });
});
