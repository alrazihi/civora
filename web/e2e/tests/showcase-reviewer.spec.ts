// Showcase for reviewer.spec.ts — Reviewer workspace and queue
import { test, expect, type Page } from '@playwright/test';
import { openDashboard, routes, regex, ORG_ID } from './helpers';

test.describe('Showcase: Reviewer Workspace', () => {
  test('renders reviewer queue and review controls', async ({ page }) => {
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

    page.route(regex(`organizations/${ORG_ID}/reviews$`), async (route) => {
      await route.fulfill({
        json: {
          success: true,
          data: [
            {
              id: 'review-1',
              case_id: 'case-2',
              case_number: 'CR-2024-002',
              case_title: 'Food assistance request',
              status: 'PENDING',
              assigned_to: 'user-1',
              workflow_state: 'REVIEW',
              priority: 'NORMAL',
              created_at: '2026-09-12T02:00:00Z',
            },
            {
              id: 'review-2',
              case_id: 'case-3',
              case_number: 'CR-2024-003',
              case_title: 'Housing benefit application',
              status: 'ASSIGNED',
              assigned_to: 'user-1',
              workflow_state: 'UNDER_REVIEW',
              priority: 'HIGH',
              created_at: '2026-09-12T03:00:00Z',
            },
          ],
        },
      });
    });

    await page.goto('/');
    await page.waitForLoadState('networkidle');
    await page.evaluate(() => router.navigate('reviewer-queue'));
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(500);
    await page.screenshot({ path: 'test-results/showcase-reviewer/01-reviewer.png', fullPage: true });

    await expect(page.locator('#view-reviewer')).toBeVisible();
    await expect(page.locator('#reviewer-subtitle')).toBeVisible();
  });
});
