// Export E2E Tests

import { test, expect, type Page } from '@playwright/test';
import {
  openOperationsDashboard,
  openWorkflowAnalysis,
  openImpactIntelligence,
  routes,
  ORG_ID,
} from './helpers';

test.describe('Export', () => {
  test('operations dashboard export buttons are present', async ({ page }) => {
    await openOperationsDashboard(page);
    await expect(page.locator('#ops-export-json')).toBeVisible();
    await expect(page.locator('#ops-export-csv')).toBeVisible();
    await expect(page.locator('#ops-export-report')).toBeVisible();
  });

  test('workflow analysis export buttons are present', async ({ page }) => {
    await openWorkflowAnalysis(page);
    await expect(page.locator('#wa-export-json')).toBeVisible();
    await expect(page.locator('#wa-export-csv')).toBeVisible();
    await expect(page.locator('#wa-export-report')).toBeVisible();
  });

  test('impact intelligence export buttons are present', async ({ page }) => {
    await openImpactIntelligence(page);
    await expect(page.locator('#impact-export-json')).toBeVisible();
    await expect(page.locator('#impact-export-csv')).toBeVisible();
    await expect(page.locator('#impact-export-report')).toBeVisible();
  });

  test('export triggers download with correct filename', async ({ page }) => {
    await openOperationsDashboard(page);

    page.route(`/api/v1/organizations/${ORG_ID}/operations/export*`, async (route) => {
      const url = new URL(route.request().url());
      const format = url.searchParams.get('format') || 'json';
      const filename = `civora-operations-daily-${new Date().toISOString().split('T')[0]}.${format}`;
      const body = format === 'json' ? '{"metadata":{},"data":{}}' : 'section,metric,value\n';
      await route.fulfill({
        status: 200,
        contentType: format === 'json' ? 'application/json' : 'text/plain',
        headers: {
          'Content-Disposition': `attachment; filename="${filename}"`,
        },
        body,
      });
    });

    const [download] = await Promise.all([
      page.waitForEvent('download', { timeout: 10000 }),
      page.click('#ops-export-json'),
    ]);

    expect(download.suggestedFilename()).toMatch(/^civora-operations-.*\.json$/);
  });
});
