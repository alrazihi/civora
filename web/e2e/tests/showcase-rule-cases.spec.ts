// Showcase for rule-cases.spec.ts — Rule-assigned workflow selection
import { test, expect, type Page } from '@playwright/test';
import { openDashboard, routes, regex, ORG_ID } from './helpers';

test.describe('Showcase: Rule-Assigned Workflows', () => {
  test('renders Education Grant workflow selector', async ({ page }) => {
    await page.setViewportSize({ width: 1400, height: 900 });

    await page.addInitScript(() => {
      localStorage.setItem('civora_token', 'dev-token');
      localStorage.setItem('civora_org_id', 'org-test');
    });

    page.route(routes.workflows, async (route) => {
      await route.fulfill({
        json: {
          success: true,
          data: [
            {
              id: 'wf-edu-1',
              key: 'education_grant',
              name: 'Education Grant',
              version: 1,
              status: 'ACTIVE',
              initial_state: 'SUBMITTED',
              states: [
                { key: 'SUBMITTED', name: 'Submitted', terminal: false },
                { key: 'UNDER_REVIEW', name: 'Under Review', terminal: false },
                { key: 'APPROVED', name: 'Approved', terminal: false },
                { key: 'REJECTED', name: 'Rejected', terminal: true },
              ],
              transitions: [
                { key: 'review', from_state: 'SUBMITTED', to_state: 'UNDER_REVIEW' },
                { key: 'approve', from_state: 'UNDER_REVIEW', to_state: 'APPROVED' },
                { key: 'reject', from_state: 'UNDER_REVIEW', to_state: 'REJECTED' },
              ],
              description: 'Education grant application processing.',
            },
          ],
        },
      });
    });

    await openDashboard(page);
    await page.evaluate(() => router.navigate('new-case'));
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(500);
    await page.screenshot({ path: 'test-results/showcase-rule-cases/01-workflow-selector.png', fullPage: true });

    await expect(page.locator('#workflow-selector .wf-option-card')).toHaveCount(1);
    await expect(page.locator('#workflow-selector .wf-option-card')).toContainText(/Education Grant/);
    await expect(page.locator('#selected-workflow-id')).toHaveValue('wf-edu-1');
  });
});
