// Operations Dashboard E2E Tests

import { test, expect, type Page } from '@playwright/test';
import {
  openOperationsDashboard,
  routes,
} from './helpers';

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
    {
      case_id: 'case-aging-2',
      workflow_key: 'general_intake',
      current_state: 'NEW',
      service_type: 'GENERAL',
      age_hours: 72,
      in_current_state_hours: 5,
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

function attachOperationsMocks(page: Page) {
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
}

test.describe('Operations Dashboard', () => {
  test('loads dashboard metrics and renders summary cards', async ({ page }) => {
    attachOperationsMocks(page);
    await openOperationsDashboard(page);

    await expect(page.locator('#ops-summary')).toBeVisible();
    await expect(page.locator('#ops-summary')).toContainText('Total Cases');
    await expect(page.locator('#ops-summary')).toContainText('Active Cases');
    await expect(page.locator('#ops-summary')).toContainText('Completed Cases');
    await expect(page.locator('#ops-summary')).toContainText('Cases Requiring Review');
    await expect(page.locator('#ops-summary')).toContainText('Aging Cases');

    await expect(page.locator('#ops-summary')).toContainText('12');
    await expect(page.locator('#ops-summary')).toContainText('7');
    await expect(page.locator('#ops-summary')).toContainText('4');
  });

  test('renders workflow distribution', async ({ page }) => {
    attachOperationsMocks(page);
    await openOperationsDashboard(page);

    await expect(page.locator('#ops-workflow-dist')).toBeVisible();
    await expect(page.locator('#ops-workflow-dist')).toContainText('emergency_assistance');
    await expect(page.locator('#ops-workflow-dist')).toContainText('general_intake');
  });

  test('renders state duration with average, median, and max', async ({ page }) => {
    attachOperationsMocks(page);
    await openOperationsDashboard(page);

    await expect(page.locator('#ops-state-duration')).toBeVisible();
    await expect(page.locator('#ops-state-duration')).toContainText('4.2');
    await expect(page.locator('#ops-state-duration')).toContainText('3.5');
    await expect(page.locator('#ops-state-duration')).toContainText('12');
  });

  test('renders case cycle time when completed cases exist', async ({ page }) => {
    attachOperationsMocks(page);
    await openOperationsDashboard(page);

    await expect(page.locator('#ops-cycle-time')).toBeVisible();
    await expect(page.locator('#ops-cycle-time')).toContainText('18.5');
    await expect(page.locator('#ops-cycle-time')).toContainText('16.0');
    await expect(page.locator('#ops-cycle-time')).toContainText('Completed Cases: 4');
  });

  test('renders aging cases table', async ({ page }) => {
    attachOperationsMocks(page);
    await openOperationsDashboard(page);

    const rows = page.locator('#ops-aging table.data-table tbody tr');
    await expect(rows).toHaveCount(2);
    await expect(page.locator('#ops-aging')).toContainText('emergency_assistance');
    await expect(page.locator('#ops-aging')).toContainText('48');
  });

  test('renders pending review metrics', async ({ page }) => {
    attachOperationsMocks(page);
    await openOperationsDashboard(page);

    await expect(page.locator('#ops-pending')).toBeVisible();
    await expect(page.locator('#ops-pending')).toContainText('3');
    await expect(page.locator('#ops-pending')).toContainText('Assigned: 1');
    await expect(page.locator('#ops-pending')).toContainText('In Review: 1');
    await expect(page.locator('#ops-pending')).toContainText('6.5');
  });

  test('renders decision metrics with approval rate', async ({ page }) => {
    attachOperationsMocks(page);
    await openOperationsDashboard(page);

    await expect(page.locator('#ops-decisions')).toBeVisible();
    await expect(page.locator('#ops-decisions')).toContainText('Total: 10');
    await expect(page.locator('#ops-decisions')).toContainText('Approved: 7');
    await expect(page.locator('#ops-decisions')).toContainText('Rejected: 2');
    await expect(page.locator('#ops-decisions')).toContainText('70.0%');
  });

  test('renders assistance outcomes with completion rate', async ({ page }) => {
    attachOperationsMocks(page);
    await openOperationsDashboard(page);

    await expect(page.locator('#ops-assistance')).toBeVisible();
    await expect(page.locator('#ops-assistance')).toContainText('Total: 6');
    await expect(page.locator('#ops-assistance')).toContainText('In Progress: 2');
    await expect(page.locator('#ops-assistance')).toContainText('Completed: 2');
    await expect(page.locator('#ops-assistance')).toContainText('66.0%');
  });

  test('renders evidence verification metrics', async ({ page }) => {
    attachOperationsMocks(page);
    await openOperationsDashboard(page);

    await expect(page.locator('#ops-evidence')).toBeVisible();
    await expect(page.locator('#ops-evidence')).toContainText('Total: 15');
    await expect(page.locator('#ops-evidence')).toContainText('Verified: 9');
    await expect(page.locator('#ops-evidence')).toContainText('Needs Review: 2');
    await expect(page.locator('#ops-evidence')).toContainText('60.0%');
  });

  test('renders information required metrics', async ({ page }) => {
    attachOperationsMocks(page);
    await openOperationsDashboard(page);

    await expect(page.locator('#ops-info-required')).toBeVisible();
    await expect(page.locator('#ops-info-required')).toContainText('Total Requests: 4');
    await expect(page.locator('#ops-info-required')).toContainText('Information Required: 2');
    await expect(page.locator('#ops-info-required')).toContainText('Escalated: 1');
  });

  test('shows calculated-at timestamp', async ({ page }) => {
    attachOperationsMocks(page);
    await openOperationsDashboard(page);

    const timestamp = page.locator('#ops-calculated-at');
    await expect(timestamp).toBeVisible();
    await expect(timestamp).toContainText('Updated:');
  });

  test('refreshes metrics when refresh is clicked', async ({ page }) => {
    attachOperationsMocks(page);
    await openOperationsDashboard(page);

    await expect(page.locator('#ops-summary')).toContainText('12');

    page.route(routes.operationsDashboard, async (route) => {
      await route.fulfill({
        json: {
          success: true,
          data: {
            ...OPERATIONS_METRICS,
            case_volume: {
              ...OPERATIONS_METRICS.case_volume,
              total_cases: 99,
              open_cases: 55,
              closed_cases: 33,
              rejected_cases: 11,
            },
          },
        },
      });
    });

    await page.click('#ops-refresh');
    await expect(page.locator('#ops-summary')).toContainText('99');
    await expect(page.locator('#ops-summary')).toContainText('55');
  });
});
