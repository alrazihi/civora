// Showcase for form-management.spec.ts — Form management CRUD
import { test, expect, type Page } from '@playwright/test';
import { openDashboard, setupFormMockRoutes, SAMPLE_FORM_TEMPLATE, routes, regex, ORG_ID } from './helpers';

test.describe('Showcase: Form Management', () => {
  test('renders forms list and create form UI', async ({ page }) => {
    await page.setViewportSize({ width: 1400, height: 900 });

    await page.addInitScript(() => {
      localStorage.setItem('civora_token', 'dev-token');
      localStorage.setItem('civora_org_id', 'org-test');
      localStorage.setItem('civora_role', 'admin');
    });

    page.route(routes.workflows, async (route) => {
      await route.fulfill({ json: { success: true, data: [] } });
    });

    setupFormMockRoutes(page, { forms: { list: [SAMPLE_FORM_TEMPLATE] } });

    await page.goto('/');
    await page.waitForLoadState('networkidle');
    await page.evaluate(() => router.navigate('forms'));
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(500);
    await page.screenshot({ path: 'test-results/showcase-form-management/01-forms-list.png', fullPage: true });

    await expect(page.locator('#view-forms')).toBeVisible();
    await expect(page.locator('#form-table-body tr').first()).toBeVisible();
    await expect(page.locator('#btn-create-form')).toBeVisible();
  });
});
