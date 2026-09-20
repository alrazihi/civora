// Showcase for workflow-analysis.spec.ts — Workflow analysis report
import { test, expect, type Page } from '@playwright/test';
import { openWorkflowAnalysis, attachWorkflowAnalysisMocks, routes } from './helpers';

test.describe('Showcase: Workflow Analysis', () => {
  test('renders anomaly sections and refresh controls', async ({ page }) => {
    await page.setViewportSize({ width: 1400, height: 900 });
    attachWorkflowAnalysisMocks(page);

    await openWorkflowAnalysis(page);
    await page.waitForTimeout(500);
    await page.screenshot({ path: 'test-results/showcase-workflow-analysis/01-workflow-analysis.png', fullPage: true });

    await expect(page.locator('#wa-state-accumulations')).toBeVisible();
    await expect(page.locator('#wa-state-durations')).toBeVisible();
    await expect(page.locator('#wa-closure-patterns')).toBeVisible();
    await expect(page.locator('#wa-info-request-patterns')).toBeVisible();
    await expect(page.locator('#wa-review-backlog')).toBeVisible();
    await expect(page.locator('#wa-threshold-exceedances')).toBeVisible();
    await expect(page.locator('#wa-refresh')).toBeVisible();
  });
});
