// Showcase for operations-dashboard.spec.ts — Operations dashboard metrics
import { test, expect, type Page } from '@playwright/test';
import { openOperationsDashboard, routes, regex, ORG_ID } from './helpers';

const OPERATIONS_METRICS = {
  case_volume: {
    total_cases: 12,
    open_cases: 7,
    closed_cases: 4,
    rejected_cases: 1,
    by_status: { NEW: 2, IN_PROGRESS: 5, CLOSED: 4, REJECTED: 1 },
    by_service_type: { EMERGENCY: 8, GENERAL: 4 },
    by_workflow_key: { emergency_assistance: 9, general_intake: 3 },
    by_state: { NEW: 2, IN_PROGRESS: 5, CLOSED: 4, REJECTED: 1 },
    calculated_at: new Date().toISOString(),
  },
  workflow_throughput: {
    workflow_key: 'emergency_assistance',
    total_transitions: 18,
    unique_cases: 9,
    avg_transitions_per_case: 2,
    calculated_at: new Date().toISOString(),
  },
  state_duration: {
    workflow_key: 'emergency_assistance',
    state_key: 'IN_PROGRESS',
    avg_duration_hours: 4.2,
    median_duration_hours: 3.5,
    max_duration_hours: 12,
    calculated_at: new Date().toISOString(),
  },
  case_cycle_time: {
    workflow_key: 'emergency_assistance',
    completed_cases: 4,
    avg_cycle_time_hours: 18.5,
    median_cycle_time_hours: 16.0,
    calculated_at: new Date().toISOString(),
  },
  aging_cases: [
    {
      case_id: 'case-aging-1',
      workflow_key: 'emergency_assistance',
      current_state: 'IN_PROGRESS',
      service_type: 'EMERGENCY',
      age_hours: 48,
      in_current_state_hours: 2,
      calculated_at: new Date().toISOString(),
    },
  ],
  pending_reviews: {
    pending_reviews: 3,
    assigned_reviews: 1,
    in_review_reviews: 1,
    waiting_info_reviews: 1,
    avg_wait_time_hours: 6.5,
    calculated_at: new Date().toISOString(),
  },
  decisions: {
    total_decisions: 10,
    approved: 7,
    rejected: 2,
    needs_more_info: 1,
    escalated: 0,
    approval_rate: 0.7,
    avg_decisions_per_reviewer: 3.3,
    calculated_at: new Date().toISOString(),
  },
  assistance_outcomes: {
    total_assistance: 6,
    planned: 1,
    in_progress: 2,
    completed: 2,
    cancelled: 1,
    completion_rate: 0.66,
    calculated_at: new Date().toISOString(),
  },
  evidence_verification: {
    total_evidence: 15,
    verified: 9,
    rejected: 2,
    needs_review: 2,
    unverified: 2,
    verification_rate: 0.6,
    calculated_at: new Date().toISOString(),
  },
  information_required: {
    total_cases: 4,
    information_required: 2,
    escalated: 1,
    awaiting_info_reviews: 1,
    calculated_at: new Date().toISOString(),
  },
};

test.describe('Showcase: Operations Dashboard', () => {
  test('renders summary, breakdowns, and data tables', async ({ page }) => {
    await page.setViewportSize({ width: 1400, height: 900 });

    await page.addInitScript(() => {
      localStorage.setItem('civora_token', 'dev-token');
      localStorage.setItem('civora_org_id', 'org-test');
    });

    page.route(routes.workflows, async (route) => {
      await route.fulfill({ json: { success: true, data: [] } });
    });

    page.route(routes.dashboardStats, async (route) => {
      await route.fulfill({
        json: {
          success: true,
          data: {
            cases: [],
            by_status: {},
            by_service_type: {},
          },
        },
      });
    });

    page.route(routes.operationsDashboard, async (route) => {
      await route.fulfill({
        json: {
          success: true,
          data: OPERATIONS_METRICS,
        },
      });
    });

    page.route(routes.workflowAnalysis, async (route) => {
      await route.fulfill({ json: { success: true, data: {
        organization_id: ORG_ID,
        calculated_at: new Date().toISOString(),
        period: 'daily',
        state_accumulations: [],
        state_duration_anomalies: [],
        workflow_closure_patterns: [],
        information_request_patterns: [],
        review_backlog_observations: [],
        threshold_exceedances: [],
      } } });
    });

    page.route(routes.analysisThresholds, async (route) => {
      await route.fulfill({ json: { success: true, data: {
        state_accumulation_threshold: 10,
        state_duration_threshold_hours: 24,
        aging_threshold_hours: 720,
        review_backlog_threshold: 20,
        info_request_frequency_per_case: 1.5,
        cycle_time_threshold_hours: 72,
      } } });
    });

    await page.goto('/');
    await page.waitForLoadState('networkidle');
    await page.evaluate(() => router.navigate('operations-dashboard'));
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(500);
    await page.screenshot({ path: 'test-results/showcase-operations-dashboard/01-operations.png', fullPage: true });

    await expect(page.locator('#view-operations-dashboard')).toBeVisible();
    await expect(page.locator('#ops-summary')).toContainText('Total Cases');
    await expect(page.locator('#ops-workflow-dist')).toBeVisible();
    await expect(page.locator('#ops-state-duration')).toBeVisible();
    await expect(page.locator('#ops-cycle-time')).toBeVisible();
  });
});
