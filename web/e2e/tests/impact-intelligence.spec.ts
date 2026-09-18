// Impact Intelligence E2E Tests

import { test, expect, type Page } from '@playwright/test';
import {
  openImpactIntelligence,
  attachImpactMocks,
  IMPACT_REPORT,
  routes,
} from './helpers';

test.describe('Impact Intelligence', () => {
  test('loads impact report and renders activity, outcome, and impact sections', async ({ page }) => {
    attachImpactMocks(page);
    await openImpactIntelligence(page);

    await expect(page.locator('#impact-activity-metrics')).toBeVisible();
    await expect(page.locator('#impact-outcome-metrics')).toBeVisible();
    await expect(page.locator('#impact-impact-metrics')).toBeVisible();
  });

  test('renders activity metrics with counts', async ({ page }) => {
    attachImpactMocks(page);
    await openImpactIntelligence(page);

    await expect(page.locator('#impact-activity-metrics')).toContainText('Cases Created');
    await expect(page.locator('#impact-activity-metrics')).toContainText('10');
    await expect(page.locator('#impact-activity-metrics')).toContainText('Assistance Created');
    await expect(page.locator('#impact-activity-metrics')).toContainText('8');
  });

  test('renders outcome metrics with counts', async ({ page }) => {
    attachImpactMocks(page);
    await openImpactIntelligence(page);

    await expect(page.locator('#impact-outcome-metrics')).toContainText('Cases Completed');
    await expect(page.locator('#impact-outcome-metrics')).toContainText('6');
    await expect(page.locator('#impact-outcome-metrics')).toContainText('Cases Rejected');
    await expect(page.locator('#impact-outcome-metrics')).toContainText('1');
    await expect(page.locator('#impact-outcome-metrics')).toContainText('Assistance Completed');
    await expect(page.locator('#impact-outcome-metrics')).toContainText('5');
  });

  test('renders impact metrics with percentages', async ({ page }) => {
    attachImpactMocks(page);
    await openImpactIntelligence(page);

    await expect(page.locator('#impact-impact-metrics')).toContainText('Case Completion Rate');
    await expect(page.locator('#impact-impact-metrics')).toContainText('60.0%');
    await expect(page.locator('#impact-impact-metrics')).toContainText('Assistance Completion Rate');
    await expect(page.locator('#impact-impact-metrics')).toContainText('62.5%');
  });

  test('shows calculated-at timestamp', async ({ page }) => {
    attachImpactMocks(page);
    await openImpactIntelligence(page);

    const timestamp = page.locator('#impact-calculated-at');
    await expect(timestamp).toBeVisible();
    await expect(timestamp).toContainText('Calculated:');
  });

  test('refreshes report when refresh is clicked', async ({ page }) => {
    attachImpactMocks(page);
    await openImpactIntelligence(page);

    await expect(page.locator('#impact-activity-metrics')).toContainText('10');

    page.route(routes.impactReport, async (route) => {
      await route.fulfill({
        json: {
          success: true,
          data: {
            ...IMPACT_REPORT,
            metrics: IMPACT_REPORT.metrics.map(m =>
              m.name === 'cases_created' ? { ...m, activity_count: 99 } : m
            ),
          },
        },
      });
    });

    await page.click('#impact-refresh');
    await expect(page.locator('#impact-activity-metrics')).toContainText('99');
  });
});
