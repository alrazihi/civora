// Workflow Analysis E2E Tests

import { test, expect, type Page } from '@playwright/test';
import {
  openWorkflowAnalysis,
  attachWorkflowAnalysisMocks,
  WORKFLOW_ANALYSIS_REPORT,
} from './helpers';

test.describe('Workflow Analysis', () => {
  test('loads workflow analysis report and renders sections', async ({ page }) => {
    attachWorkflowAnalysisMocks(page);
    await openWorkflowAnalysis(page);

    await expect(page.locator('#wa-state-accumulations')).toBeVisible();
    await expect(page.locator('#wa-state-durations')).toBeVisible();
    await expect(page.locator('#wa-closure-patterns')).toBeVisible();
    await expect(page.locator('#wa-info-request-patterns')).toBeVisible();
    await expect(page.locator('#wa-review-backlog')).toBeVisible();
    await expect(page.locator('#wa-threshold-exceedances')).toBeVisible();
  });

  test('renders state accumulations with threshold data', async ({ page }) => {
    attachWorkflowAnalysisMocks(page);
    await openWorkflowAnalysis(page);

    await expect(page.locator('#wa-state-accumulations')).toContainText('emergency_assistance');
    await expect(page.locator('#wa-state-accumulations')).toContainText('IN_PROGRESS');
    await expect(page.locator('#wa-state-accumulations')).toContainText('15');
    await expect(page.locator('#wa-state-accumulations')).toContainText('high_volume');
  });

  test('renders state duration anomalies', async ({ page }) => {
    attachWorkflowAnalysisMocks(page);
    await openWorkflowAnalysis(page);

    await expect(page.locator('#wa-state-durations')).toContainText('REVIEW');
    await expect(page.locator('#wa-state-durations')).toContainText('36');
    await expect(page.locator('#wa-state-durations')).toContainText('longer_duration');
  });

  test('renders workflow closure patterns', async ({ page }) => {
    attachWorkflowAnalysisMocks(page);
    await openWorkflowAnalysis(page);

    await expect(page.locator('#wa-closure-patterns')).toContainText('emergency_assistance');
    await expect(page.locator('#wa-closure-patterns')).toContainText('100');
    await expect(page.locator('#wa-closure-patterns')).toContainText('80.0%');
  });

  test('renders information request patterns', async ({ page }) => {
    attachWorkflowAnalysisMocks(page);
    await openWorkflowAnalysis(page);

    await expect(page.locator('#wa-info-request-patterns')).toContainText('180');
    await expect(page.locator('#wa-info-request-patterns')).toContainText('1.80');
    await expect(page.locator('#wa-info-request-patterns')).toContainText('higher_frequency');
  });

  test('renders review backlog observations', async ({ page }) => {
    attachWorkflowAnalysisMocks(page);
    await openWorkflowAnalysis(page);

    await expect(page.locator('#wa-review-backlog')).toContainText('25');
    await expect(page.locator('#wa-review-backlog')).toContainText('increased_backlog');
  });

  test('renders threshold exceedances', async ({ page }) => {
    attachWorkflowAnalysisMocks(page);
    await openWorkflowAnalysis(page);

    await expect(page.locator('#wa-threshold-exceedances')).toContainText('emergency_assistance');
    await expect(page.locator('#wa-threshold-exceedances')).toContainText('60');
  });

  test('shows calculated-at timestamp', async ({ page }) => {
    attachWorkflowAnalysisMocks(page);
    await openWorkflowAnalysis(page);

    const timestamp = page.locator('#wa-calculated-at');
    await expect(timestamp).toBeVisible();
    await expect(timestamp).toContainText('Calculated:');
  });

  test('refreshes analysis when refresh is clicked', async ({ page }) => {
    attachWorkflowAnalysisMocks(page);
    await openWorkflowAnalysis(page);

    await expect(page.locator('#wa-state-accumulations')).toContainText('15');

    page.route((url) => url.toString().includes('/operations/analysis/workflow'), async (route) => {
      await route.fulfill({
        json: {
          success: true,
          data: {
            ...WORKFLOW_ANALYSIS_REPORT,
            state_accumulations: [
              {
                ...WORKFLOW_ANALYSIS_REPORT.state_accumulations[0],
                case_count: 99,
              },
            ],
          },
        },
      });
    });

    await page.click('#wa-refresh');
    await expect(page.locator('#wa-state-accumulations')).toContainText('99');
  });
});
