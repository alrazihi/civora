// CIVORA App Capability Showcase
// Navigates through all major views and captures screenshots.

import { test, expect, type Page } from '@playwright/test';
import {
  openDashboard,
  openOperationsDashboard,
  openWorkflowAnalysis,
  openImpactIntelligence,
  attachCaseMocks,
  attachWorkflowAnalysisMocks,
  attachImpactMocks,
  setupCaseWorkspaceMocks,
  routes,
  regex,
  ORG_ID,
} from './helpers';

const CASE_ID = 'case-showcase-1';

async function mockAll(page: Page) {
  page.route(routes.workflows, async (route) => {
    await route.fulfill({
      json: {
        success: true,
        data: [
          {
            id: 'wf-emergency-1',
            key: 'emergency_assistance',
            name: 'Emergency Assistance',
            version: 1,
            status: 'ACTIVE',
            initial_state: 'NEW',
            states: [
              { key: 'NEW', name: 'New', terminal: false },
              { key: 'IN_PROGRESS', name: 'In Progress', terminal: false },
              { key: 'REVIEW', name: 'Review', terminal: false },
              { key: 'CLOSED', name: 'Closed', terminal: true },
            ],
            transitions: [
              { key: 'start', from_state: 'NEW', to_state: 'IN_PROGRESS' },
              { key: 'submit_for_review', from_state: 'IN_PROGRESS', to_state: 'REVIEW' },
              { key: 'approve', from_state: 'REVIEW', to_state: 'CLOSED' },
            ],
            description: 'Emergency assistance workflow for crisis response.',
          },
          {
            id: 'wf-benefits-1',
            key: 'benefits_application',
            name: 'Benefits Application',
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
            description: 'Benefits application processing workflow.',
          },
        ],
      },
    });
  });

  const cases = [
    { id: 'case-1', case_number: 'CR-2024-001', status: 'IN_PROGRESS', service_type: 'EMERGENCY', priority: 'HIGH', title: 'Burst pipe on 3rd floor', updated_at: '2026-09-12T01:00:00Z' },
    { id: 'case-2', case_number: 'CR-2024-002', status: 'REVIEW', service_type: 'EMERGENCY', priority: 'NORMAL', title: 'Food assistance request', updated_at: '2026-09-12T02:00:00Z' },
    { id: 'case-3', case_number: 'CR-2024-003', status: 'NEW', service_type: 'BENEFITS', priority: 'HIGH', title: 'Housing benefit application', updated_at: '2026-09-12T03:00:00Z' },
    { id: CASE_ID, case_number: 'CR-2024-005', status: 'IN_PROGRESS', service_type: 'EMERGENCY', priority: 'HIGH', title: 'Emergency Food Assistance Request', updated_at: '2026-09-12T04:00:00Z' },
  ];

  page.route(routes.casesSearch, async (route, request) => {
    if (request.method() === 'GET') {
      await route.fulfill({ json: { success: true, data: cases } });
    } else {
      await route.continue();
    }
  });

  page.route(routes.dashboardStats, async (route) => {
    await route.fulfill({
      json: {
        success: true,
        data: {
          cases,
          by_status: { NEW: 1, IN_PROGRESS: 1, REVIEW: 1, CLOSED: 0 },
          by_service_type: { EMERGENCY: 3, BENEFITS: 1 },
          by_priority: { HIGH: 2, NORMAL: 2 },
        },
      },
    });
  });

  page.route(routes.casesCreate, async (route, request) => {
    if (request.method() !== 'POST') {
      await route.continue();
      return;
    }
    const body = JSON.parse(request.postData() || '{}');
    const created = {
      id: CASE_ID,
      title: body.title,
      status: 'NEW',
      service_type: body.service_type,
      priority: body.priority,
      person_id: body.person_id,
    };
    await route.fulfill({ json: { success: true, data: created } });
  });

  page.route(regex(`organizations/${ORG_ID}/cases/${CASE_ID}$`), async (route) => {
    await route.fulfill({
      json: {
        success: true,
        data: {
          id: CASE_ID,
          case_number: 'CR-2024-005',
          title: 'Emergency Food Assistance Request',
          status: 'IN_PROGRESS',
          service_type: 'EMERGENCY',
          priority: 'HIGH',
          description: 'Client needs emergency food assistance and shelter referral.',
          person_id: 'person-1',
          created_at: '2026-09-12T01:00:00Z',
          updated_at: '2026-09-12T02:00:00Z',
        },
      },
    });
  });

  page.route(regex(`organizations/${ORG_ID}/cases/${CASE_ID}/workflow$`), async (route) => {
    await route.fulfill({
      json: {
        success: true,
        data: {
          instance: {
            id: 'inst-showcase',
            workflow_definition_id: 'wf-emergency-1',
            definition: {
              id: 'wf-emergency-1',
              key: 'emergency_assistance',
              name: 'Emergency Assistance',
              initial_state: 'NEW',
              states: [
                { key: 'NEW', name: 'New', terminal: false },
                { key: 'IN_PROGRESS', name: 'In Progress', terminal: false },
                { key: 'REVIEW', name: 'Review', terminal: false },
                { key: 'CLOSED', name: 'Closed', terminal: true },
              ],
              transitions: [
                { key: 'start', from_state: 'NEW', to_state: 'IN_PROGRESS' },
                { key: 'submit_for_review', from_state: 'IN_PROGRESS', to_state: 'REVIEW' },
                { key: 'approve', from_state: 'REVIEW', to_state: 'CLOSED' },
              ],
            },
            instance: { current_state: 'IN_PROGRESS' },
          },
        },
      },
    });
  });

  // Forms
  page.route(regex(`organizations/${ORG_ID}/forms$`), async (route) => {
    await route.fulfill({
      json: {
        success: true,
        data: [
          {
            id: 'form-1',
            key: 'intake_form',
            name: 'Intake Questionnaire',
            description: 'Initial client intake form',
            version: 2,
            status: 'ACTIVE',
            fields: [
              { key: 'full_name', label: 'Full Name', type: 'text', required: true },
              { key: 'email', label: 'Email', type: 'email', required: true },
              { key: 'household_size', label: 'Household Size', type: 'number', required: true },
            ],
          },
          {
            id: 'form-2',
            key: 'evidence_form',
            name: 'Evidence Form',
            description: 'Supporting documentation',
            version: 1,
            status: 'ACTIVE',
            fields: [
              { key: 'evidence_type', label: 'Evidence Type', type: 'select', required: true, options: [{ value: 'id', label: 'ID' }, { value: 'paystub', label: 'Pay Stub' }] },
            ],
          },
          {
            id: 'form-3',
            key: 'consent_form',
            name: 'Consent Form',
            description: 'Client consent and data sharing agreement',
            version: 1,
            status: 'ACTIVE',
            fields: [
              { key: 'consent_given', label: 'Consent Given', type: 'boolean', required: true },
            ],
          },
        ],
      },
    });
  });

  // Rules
  page.route(regex(`organizations/${ORG_ID}/rule-sets$`), async (route) => {
    await route.fulfill({
      json: {
        success: true,
        data: [
          {
            id: 'ruleset-1',
            key: 'eligibility_check',
            name: 'Eligibility Check',
            description: 'Determines client eligibility for emergency assistance',
            version: 1,
            status: 'ACTIVE',
            rules: [
              { id: 'r1', description: 'Income below threshold', fact: 'income', operator: 'less_than', value: 30000 },
              { id: 'r2', description: 'Household size greater than 1', fact: 'household_size', operator: 'greater_than', value: 0 },
            ],
          },
          {
            id: 'ruleset-2',
            key: 'priority_routing',
            name: 'Priority Routing',
            description: 'Routes cases based on priority level',
            version: 1,
            status: 'ACTIVE',
            rules: [
              { id: 'r1', description: 'High priority cases', fact: 'priority', operator: 'equals', value: 'HIGH' },
            ],
          },
        ],
      },
    });
  });

  page.route(regex(`organizations/${ORG_ID}/rule-sets/ruleset-1$`), async (route) => {
    await route.fulfill({
      json: {
        success: true,
        data: {
          id: 'ruleset-1',
          key: 'eligibility_check',
          name: 'Eligibility Check',
          description: 'Determines client eligibility for emergency assistance',
          version: 1,
          status: 'ACTIVE',
          rules: [
            { id: 'r1', description: 'Income below threshold', fact: 'income', operator: 'less_than', value: 30000 },
            { id: 'r2', description: 'Household size greater than 1', fact: 'household_size', operator: 'greater_than', value: 0 },
          ],
          trace: {
            evaluation_id: 'eval-1',
            result: 'PASS',
            facts: { income: 25000, household_size: 3 },
            rule_results: [
              { rule_id: 'r1', description: 'Income below threshold', passed: true, actual: 25000, expected: 'less_than 30000' },
              { rule_id: 'r2', description: 'Household size greater than 1', passed: true, actual: 3, expected: 'greater_than 0' },
            ],
          },
        },
      },
    });
  });

  // Operations
  page.route(routes.operationsDashboard, async (route) => {
    await route.fulfill({
      json: {
        success: true,
        data: {
          total_cases: 1247,
          open_cases: 843,
          closed_cases: 404,
          avg_cycle_time_hours: 72.5,
          period: 'daily',
          calculated_at: new Date().toISOString(),
        },
      },
    });
  });

  page.route(routes.workflowAnalysis, async (route) => {
    await route.fulfill({ json: { success: true, data: {
      organization_id: ORG_ID,
      calculated_at: new Date().toISOString(),
      period: 'daily',
      state_accumulations: [
        { workflow_key: 'emergency_assistance', state_key: 'IN_PROGRESS', case_count: 15, threshold: 10, observation: 'high volume in state IN_PROGRESS', language: 'high_volume' },
      ],
      state_duration_anomalies: [
        { workflow_key: 'emergency_assistance', state_key: 'REVIEW', avg_duration_hours: 36, threshold_hours: 24, observation: 'longer observed duration in state REVIEW', language: 'longer_duration' },
      ],
      workflow_closure_patterns: [
        { workflow_key: 'emergency_assistance', total_cases: 100, closed_cases: 80, closure_rate: 0.8, observation: 'closure rate within expected range' },
      ],
      information_request_patterns: [
        { workflow_key: 'emergency_assistance', total_cases: 100, info_request_count: 180, frequency_per_case: 1.8, threshold_per_case: 1.5, observation: 'higher frequency of information requests', language: 'higher_frequency' },
      ],
      review_backlog_observations: [
        { pending_reviews: 25, threshold: 20, observation: 'increased backlog of pending reviews', language: 'increased_backlog' },
      ],
      threshold_exceedances: [
        { workflow_key: 'emergency_assistance', current_state: 'REVIEW', threshold_type: 'state_duration_threshold_hours', threshold_value: 24, actual_value: 60, observation: 'longer observed duration in state REVIEW' },
      ],
    } } });
  });

  page.route(routes.analysisThresholds, async (route) => {
    await route.fulfill({ json: { success: true, data: { state_accumulation_threshold: 10, state_duration_threshold_hours: 24, aging_threshold_hours: 720, review_backlog_threshold: 20, info_request_frequency_per_case: 1.5, cycle_time_threshold_hours: 72 } } });
  });

  page.route(routes.impactReport, async (route) => {
    await route.fulfill({ json: { success: true, data: {
      organization_id: ORG_ID,
      calculated_at: new Date().toISOString(),
      period: 'daily',
      metrics: [
        { category: 'activity', name: 'cases_created', label: 'Cases Created', activity_count: 10 },
        { category: 'outcome', name: 'cases_completed', label: 'Cases Completed', outcome_count: 6 },
        { category: 'outcome', name: 'cases_rejected', label: 'Cases Rejected', outcome_count: 1 },
        { category: 'activity', name: 'assistance_created', label: 'Assistance Created', activity_count: 8 },
        { category: 'outcome', name: 'assistance_completed', label: 'Assistance Completed', outcome_count: 5 },
        { category: 'activity', name: 'evidence_submitted', label: 'Evidence Submitted', activity_count: 12 },
        { category: 'outcome', name: 'evidence_verified', label: 'Evidence Verified', outcome_count: 9 },
        { category: 'activity', name: 'decisions_made', label: 'Decisions Made', activity_count: 7 },
        { category: 'outcome', name: 'decisions_approved', label: 'Decisions Approved', outcome_count: 4 },
        { category: 'impact', name: 'case_completion_rate', label: 'Case Completion Rate', impact_value: 0.6 },
      ],
    } } });
  });

  // Reviewer
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

  // Case workspace forms
  page.route(regex(`organizations/${ORG_ID}/cases/${CASE_ID}/workflow/forms$`), async (route) => {
    if (route.request().method() !== 'GET') { await route.continue(); return; }
    await route.fulfill({ json: { success: true, data: [
      {
        id: 'form-sample-1',
        key: 'intake_form',
        name: 'Intake Questionnaire',
        description: 'Please provide the following information to process your request.',
        version: 1,
        status: 'ACTIVE',
        fields: [
          { key: 'full_name', label: 'Full Name', type: 'text', required: true, placeholder: 'Jane Doe' },
          { key: 'email', label: 'Email Address', type: 'email', required: true },
          { key: 'household_size', label: 'Household Size', type: 'number', required: true },
          { key: 'income', label: 'Annual Income', type: 'decimal', required: false },
          { key: 'preferred_contact', label: 'Preferred Contact Method', type: 'select', required: true, options: [
            { value: 'email', label: 'Email' }, { value: 'phone', label: 'Phone' }, { value: 'text', label: 'Text Message' },
          ]},
          { key: 'consent', label: 'I agree to the terms', type: 'boolean', required: true },
          { key: 'notes', label: 'Additional Notes', type: 'textarea', required: false },
        ],
      },
    ] } });
  });

  page.route(regex(`organizations/${ORG_ID}/cases/${CASE_ID}/workflow/form-submissions$`), async (route) => {
    if (route.request().method() !== 'GET') { await route.continue(); return; }
    await route.fulfill({ json: { success: true, data: {} } });
  });

  page.route(regex(`organizations/${ORG_ID}/cases/${CASE_ID}/form/[^/]+/submission$`), async (route) => {
    if (route.request().method() !== 'POST') { await route.continue(); return; }
    await route.fulfill({ json: { success: true, data: { id: 'submission-1', status: 'SUBMITTED' } } });
  });
}

test.describe('CIVORA App Capability Showcase', () => {
  test('showcase all app capabilities', async ({ page }) => {
    await page.setViewportSize({ width: 1400, height: 900 });

    // Setup all mocks before navigation
    await mockAll(page);
    attachCaseMocks(page);
    attachWorkflowAnalysisMocks(page);
    attachImpactMocks(page);

    // 1. Login Page
    await page.addInitScript(() => {
      localStorage.setItem('civora_token', 'dev-token');
      localStorage.setItem('civora_org_id', 'org-test');
    });
    await page.goto('/');
    await page.waitForLoadState('networkidle');
    await page.screenshot({ path: 'test-results/showcase/01-login.png', fullPage: true });

    // 2. Dashboard
    await page.evaluate(() => router.navigate('dashboard'));
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(500);
    await page.screenshot({ path: 'test-results/showcase/02-dashboard.png', fullPage: true });

    // 3. New Case / Workflow Selection
    await page.evaluate(() => router.navigate('new-case'));
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(500);
    await page.screenshot({ path: 'test-results/showcase/03-new-case.png', fullPage: true });

    // 4. Case Workspace (navigate directly)
    await page.evaluate(() => router.navigate('case', 'case-showcase-1'));
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(800);
    await page.screenshot({ path: 'test-results/showcase/04-case-workspace.png', fullPage: true });

    // 5. Workflows
    await page.evaluate(() => router.navigate('workflows'));
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(500);
    await page.screenshot({ path: 'test-results/showcase/06-workflows.png', fullPage: true });

    // 6. Forms Management
    await page.evaluate(() => router.navigate('forms'));
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(500);
    await page.screenshot({ path: 'test-results/showcase/07-forms.png', fullPage: true });

    // 7. Rules Management
    await page.evaluate(() => router.navigate('rules'));
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(500);
    await page.screenshot({ path: 'test-results/showcase/08-rules.png', fullPage: true });

    // 8. Operations Dashboard
    await page.evaluate(() => router.navigate('operations-dashboard'));
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(500);
    await page.screenshot({ path: 'test-results/showcase/09-operations.png', fullPage: true });

    // 9. Workflow Analysis
    await page.evaluate(() => router.navigate('workflow-analysis'));
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(500);
    await page.screenshot({ path: 'test-results/showcase/10-workflow-analysis.png', fullPage: true });

    // 10. Impact Intelligence
    await page.evaluate(() => router.navigate('impact-intelligence'));
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(500);
    await page.screenshot({ path: 'test-results/showcase/11-impact.png', fullPage: true });

    // 11. Reviewer Workspace
    await page.evaluate(() => router.navigate('reviewer-queue'));
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(500);
    await page.screenshot({ path: 'test-results/showcase/12-reviewer.png', fullPage: true });
  });
});
