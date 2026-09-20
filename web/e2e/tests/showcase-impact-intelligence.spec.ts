// Showcase for impact-intelligence.spec.ts — Impact intelligence report
import { test, expect, type Page } from '@playwright/test';
import { openImpactIntelligence, attachImpactMocks, routes } from './helpers';

test.describe('Showcase: Impact Intelligence', () => {
  test('renders activity, outcome, and impact metrics', async ({ page }) => {
    await page.setViewportSize({ width: 1400, height: 900 });
    attachImpactMocks(page);

    await openImpactIntelligence(page);
    await page.waitForTimeout(500);
    await page.screenshot({ path: 'test-results/showcase-impact-intelligence/01-impact.png', fullPage: true });

    await expect(page.locator('#impact-activity-metrics')).toBeVisible();
    await expect(page.locator('#impact-outcome-metrics')).toBeVisible();
    await expect(page.locator('#impact-impact-metrics')).toBeVisible();
    await expect(page.locator('#impact-calculated-at')).toBeVisible();
    await expect(page.locator('#impact-refresh')).toBeVisible();
  });
});
