// Reviewer Workspace E2E Tests

import { test, expect, type Page } from '@playwright/test';
import {
  openDashboard,
  orgPath,
  regex,
  ADMIN_TOKEN,
} from './helpers';

const REVIEW_ID = 'review-1';
const CASE_ID = 'case-review-1';

const REVIEW_QUEUE_ENTRY = {
  id: REVIEW_ID,
  organization_id: 'org-test',
  case_id: CASE_ID,
  workflow_instance_id: 'inst-review-1',
  status: 'PENDING',
  priority: 'HIGH',
  workflow_state: 'DECISION_PENDING',
  rule_evaluation_ids: ['eval-1'],
  missing_information: [],
  assigned_to: null,
  created_at: '2026-09-15T10:00:00Z',
  updated_at: '2026-09-15T10:00:00Z',
  completed_at: null,
  metadata: {},
};

const CASE_DATA = {
  id: CASE_ID,
  organization_id: 'org-test',
  case_number: 'CASE-REV-001',
  title: 'Emergency Assistance Request',
  description: 'Needs review for eligibility',
  status: 'OPEN',
  service_type: 'EMERGENCY_ASSISTANCE',
  priority: 'HIGH',
  person_id: 'person-1',
  created_by: 'user-1',
  assigned_to: null,
  created_at: '2026-09-15T09:00:00Z',
  updated_at: '2026-09-15T09:00:00Z',
  closed_at: null,
  version: 1,
  workflow_instance_id: 'inst-review-1',
  workflow_state: 'DECISION_PENDING',
  workflow_key: 'emergency_assistance',
  workflow_id: 'wf-emergency-1',
};

const PERSON_DATA = {
  id: 'person-1',
  organization_id: 'org-test',
  case_id: CASE_ID,
  first_name: 'Jane',
  last_name: 'Doe',
  preferred_language: 'en',
  status: 'ACTIVE',
  date_of_birth: '1985-04-12',
  email: 'jane@example.com',
  phone: '+1234567890',
  address: '123 Main St',
  city: 'Springfield',
  created_at: '2026-09-15T08:00:00Z',
  updated_at: '2026-09-15T08:00:00Z',
};

const WORKFLOW_DATA = {
  instance: {
    id: 'inst-review-1',
    organization_id: 'org-test',
    workflow_definition_id: 'wf-emergency-1',
    workflow_definition_version: 1,
    case_id: CASE_ID,
    current_state: 'DECISION_PENDING',
    started_at: '2026-09-15T09:00:00Z',
    completed_at: null,
    metadata: {},
    version: 1,
  },
  definition: {
    id: 'wf-emergency-1',
    organization_id: 'org-test',
    key: 'emergency_assistance',
    name: 'Emergency Assistance',
    description: 'Emergency assistance workflow',
    version: 1,
    status: 'ACTIVE',
    initial_state: 'NEW',
    states: [
      { key: 'NEW', name: 'New', terminal: false, display_order: 0 },
      { key: 'DECISION_PENDING', name: 'Decision Pending', terminal: false, display_order: 4 },
      { key: 'APPROVED', name: 'Approved', terminal: false, display_order: 5 },
      { key: 'REJECTED', name: 'Rejected', terminal: true, display_order: 6 },
    ],
    transitions: [
      { key: 'submit', from_state: 'NEW', to_state: 'DECISION_PENDING' },
      { key: 'approve', from_state: 'DECISION_PENDING', to_state: 'APPROVED' },
      { key: 'reject', from_state: 'DECISION_PENDING', to_state: 'REJECTED' },
    ],
    created_at: '2026-09-15T08:00:00Z',
    updated_at: '2026-09-15T08:00:00Z',
  },
};

const FORM_SUBMISSIONS = {
  income_form: {
    id: 'sub-1',
    case_id: CASE_ID,
    form_id: 'form-income',
    form_version_id: 'form-income-v1',
    submitted_by: 'user-1',
    status: 'submitted',
    data: { amount: 50000 },
    submitted_at: '2026-09-15T09:30:00Z',
    updated_at: '2026-09-15T09:30:00Z',
  },
};

const EVIDENCE_DATA = [
  {
    id: 'ev-1',
    organization_id: 'org-test',
    case_id: CASE_ID,
    type: 'IDENTITY_DOCUMENT',
    description: 'Government-issued ID',
    storage_reference: 's3://bucket/ev-1',
    created_by: 'user-1',
    created_at: '2026-09-15T09:15:00Z',
    updated_at: '2026-09-15T09:15:00Z',
  },
];

const EVALUATIONS_DATA = [
  {
    id: 'eval-1',
    organization_id: 'org-test',
    rule_set_id: 'rs-eligibility',
    case_id: CASE_ID,
    outcome: 'ELIGIBLE',
    explanation: 'Income and household criteria satisfied. The applicant meets all eligibility requirements for emergency assistance.',
    facts: {},
    trace: [],
    created_at: '2026-09-15T09:45:00Z',
    updated_at: '2026-09-15T09:45:00Z',
  },
];

const DECISIONS_DATA = [
  {
    id: 'dec-1',
    organization_id: 'org-test',
    case_id: CASE_ID,
    decision: 'NEEDS_MORE_INFORMATION',
    reason: 'Please provide proof of income.',
    decided_by: 'reviewer-1',
    decided_at: '2026-09-15T10:30:00Z',
    created_at: '2026-09-15T10:30:00Z',
    updated_at: '2026-09-15T10:30:00Z',
  },
];

function attachReviewerMocks(page: Page) {
  page.route(orgPath('/review-queue'), async (route) => {
    await route.fulfill({ json: { success: true, data: [REVIEW_QUEUE_ENTRY], meta: { page: 1, per_page: 50, total: 1, total_pages: 1 } } });
  });

  page.route(orgPath(`/review-queue/${REVIEW_ID}`), async (route) => {
    await route.fulfill({ json: { success: true, data: REVIEW_QUEUE_ENTRY } });
  });

  page.route(orgPath(`/review-queue/${REVIEW_ID}/claim`), async (route) => {
    if (route.request().method() === 'POST') {
      await route.fulfill({ json: { success: true, data: { ...REVIEW_QUEUE_ENTRY, status: 'ASSIGNED', assigned_to: 'user-1' } } });
    } else {
      await route.continue();
    }
  });

  page.route(orgPath(`/review-queue/${REVIEW_ID}/start`), async (route) => {
    if (route.request().method() === 'POST') {
      await route.fulfill({ json: { success: true, data: { ...REVIEW_QUEUE_ENTRY, status: 'IN_REVIEW', assigned_to: 'user-1' } } });
    } else {
      await route.continue();
    }
  });

  page.route(orgPath(`/review-queue/${REVIEW_ID}/complete`), async (route) => {
    if (route.request().method() === 'POST') {
      await route.fulfill({ json: { success: true, data: { ...REVIEW_QUEUE_ENTRY, status: 'COMPLETED', assigned_to: 'user-1', completed_at: '2026-09-15T11:00:00Z' } } });
    } else {
      await route.continue();
    }
  });

  page.route(orgPath(`/review-queue/${REVIEW_ID}/escalate`), async (route) => {
    if (route.request().method() === 'POST') {
      await route.fulfill({ json: { success: true, data: { ...REVIEW_QUEUE_ENTRY, status: 'ESCALATED', assigned_to: 'user-1', completed_at: '2026-09-15T11:00:00Z' } } });
    } else {
      await route.continue();
    }
  });

  page.route(orgPath(`/review-queue/${REVIEW_ID}/request-information`), async (route) => {
    if (route.request().method() === 'POST') {
      await route.fulfill({ json: { success: true, data: { ...REVIEW_QUEUE_ENTRY, status: 'WAITING_INFORMATION', missing_information: ['income_proof'] } } });
    } else {
      await route.continue();
    }
  });

  page.route(orgPath(`/cases/${CASE_ID}`), async (route) => {
    await route.fulfill({ json: { success: true, data: CASE_DATA } });
  });

  page.route(orgPath(`/cases/${CASE_ID}/workflow`), async (route) => {
    await route.fulfill({ json: { success: true, data: WORKFLOW_DATA } });
  });

  page.route(orgPath(`/people?case_id=${CASE_ID}`), async (route) => {
    await route.fulfill({ json: { success: true, data: [PERSON_DATA] } });
  });

  page.route(orgPath(`/cases/${CASE_ID}/workflow/form-submissions`), async (route) => {
    await route.fulfill({ json: { success: true, data: FORM_SUBMISSIONS } });
  });

  page.route(orgPath(`/evidence/by-service-request/${CASE_ID}`), async (route) => {
    await route.fulfill({ json: { success: true, data: EVIDENCE_DATA } });
  });

  page.route(orgPath(`/decisions/by-service-request/${CASE_ID}`), async (route) => {
    await route.fulfill({ json: { success: true, data: DECISIONS_DATA } });
  });

  page.route(orgPath(`/rules/cases/${CASE_ID}/evaluations`), async (route) => {
    await route.fulfill({ json: { success: true, data: EVALUATIONS_DATA } });
  });
}

test.describe('Reviewer Workspace', () => {
  test.beforeEach(async ({ page }) => {
    await page.addInitScript(() => {
      localStorage.setItem('civora_token', ADMIN_TOKEN);
      localStorage.setItem('civora_org_id', ORG_ID);
    });
  });

  test('shows reviewer workspace with complete decision context', async ({ page }) => {
    await openDashboard(page);
    attachReviewerMocks(page);
    await page.evaluate((reviewId) => router.navigate('reviewer', reviewId), REVIEW_ID);

    await expect(page.locator('#view-reviewer')).toBeVisible();
    await expect(page.locator('#reviewer-subtitle')).toContainText(`Review ${REVIEW_ID}`);

    await expect(page.locator('#review-status-area')).toContainText('PENDING');
    await expect(page.locator('#review-case-area')).toContainText('CASE-REV-001');
    await expect(page.locator('#review-person-area')).toContainText('Jane Doe');
    await expect(page.locator('#review-workflow-area')).toContainText('DECISION_PENDING');
    await expect(page.locator('#review-forms-area')).toContainText('income_form');
    await expect(page.locator('#review-evidence-area')).toContainText('IDENTITY_DOCUMENT');
    await expect(page.locator('#review-rules-area')).toContainText('ELIGIBLE');
    await expect(page.locator('#review-rules-area')).toContainText('Income and household criteria satisfied');
    await expect(page.locator('#review-decisions-area')).toContainText('NEEDS_MORE_INFORMATION');
  });

  test('shows decision controls for pending review', async ({ page }) => {
    await openDashboard(page);
    attachReviewerMocks(page);
    await page.evaluate((reviewId) => router.navigate('reviewer', reviewId), REVIEW_ID);

    await expect(page.locator('#reviewer-card-controls')).toBeVisible();
    await expect(page.locator('#review-controls-area button:has-text("Claim Review")')).toBeVisible();
  });

  test('shows start review control for assigned review', async ({ page }) => {
    const assignedReview = { ...REVIEW_QUEUE_ENTRY, status: 'ASSIGNED', assigned_to: 'user-1' };
    await openDashboard(page);
    attachReviewerMocks(page);
    page.route(orgPath(`/review-queue/${REVIEW_ID}`), async (route) => {
      await route.fulfill({ json: { success: true, data: assignedReview } });
    });
    await page.evaluate((reviewId) => router.navigate('reviewer', reviewId), REVIEW_ID);

    await expect(page.locator('#review-controls-area button:has-text("Start Review")')).toBeVisible();
    await expect(page.locator('#review-controls-area button:has-text("Approve")')).toBeVisible();
  });

  test('shows empty state when no reviews assigned', async ({ page }) => {
    await openDashboard(page);
    attachReviewerMocks(page);
    page.route(orgPath('/review-queue'), async (route) => {
      await route.fulfill({ json: { success: true, data: [], meta: { page: 1, per_page: 50, total: 0, total_pages: 0 } } });
    });
    await page.evaluate(() => router.navigate('reviewer-queue'));

    await expect(page.locator('#review-status-area')).toContainText('No reviews assigned to you');
  });

  test('handles missing case details gracefully', async ({ page }) => {
    await openDashboard(page);
    attachReviewerMocks(page);
    page.route(orgPath(`/cases/${CASE_ID}`), async (route) => {
      await route.fulfill({ status: 404, json: { success: false, error: { code: 'NOT_FOUND', message: 'Case not found' } } });
    });
    await page.evaluate((reviewId) => router.navigate('reviewer', reviewId), REVIEW_ID);

    await expect(page.locator('#review-case-area')).toContainText('Case details unavailable');
  });
});
