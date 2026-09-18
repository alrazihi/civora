// Playwright End-to-End Tests for the CIVORA web frontend
//
// Shared test fixtures and helpers used by category-specific spec files.
// These tests exercise the generic, workflow-driven case operations living in
// web/js/app.js. They run against a static server (web/ served verbatim) so
// no PostgreSQL instance or Go backend is required; the backend API is mocked
// with Playwright request interception for fast, deterministic, offline tests.
//
// Run from web/e2e: npm test   (the globalSetup starts the static server)

import { test, expect, type Page } from '@playwright/test';

const ORG_ID = 'org-test';

const ADMIN_TOKEN = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ1c2VyLTEiLCJvcmdhbml6YXRpb25faWQiOiJvcmdlLXRlc3QiLCJyb2xlIjoiYWRtaW4ifQ.sig';

// A test workflow used by the case creation mocks. Its shape mirrors a real
// workflow definition returned by the backend.
const EMERGENCY_WORKFLOW = {
  id: 'wf-emergency-1',
  key: 'emergency_assistance',
  name: 'Emergency Assistance',
  version: 1,
  status: 'ACTIVE',
  initial_state: 'NEW',
  states: [
    { key: 'NEW', name: 'New', terminal: false },
    { key: 'IN_PROGRESS', name: 'In Progress', terminal: false },
    { key: 'CLOSED', name: 'Closed', terminal: true },
  ],
  transitions: [{ key: 'start', from_state: 'NEW', to_state: 'IN_PROGRESS' }],
  description: 'Generic emergency workflow for cases.',
};

const PERSON = { id: 'person-1', name: 'Jane Doe' };

const regex = (s: string) => new RegExp(s);

function orgPath(path: string) {
  return `/api/v1/organizations/${ORG_ID}${path}`;
}

const routes = {
  workflows: regex(`organizations/${ORG_ID}/workflows`),
  casesCreate: regex(`organizations/${ORG_ID}/cases$`),
  casesSearch: regex(`organizations/${ORG_ID}/cases\\?`),
  caseWorkflow: (id: string) => regex(`organizations/${ORG_ID}/cases/${id}/workflow`),
  dashboardStats: regex(`organizations/${ORG_ID}/cases/dashboard/statistics`),
  operationsDashboard: regex(`organizations/${ORG_ID}/operations/metrics/dashboard`),
  workflowAnalysis: regex(`organizations/${ORG_ID}/operations/analysis/workflow`),
  analysisThresholds: regex(`organizations/${ORG_ID}/operations/analysis/thresholds`),
  impactReport: regex(`organizations/${ORG_ID}/operations/impact/report`),
};

// Install request handlers mocking the CIVORA API for the case workflows.
function attachCaseMocks(page: Page) {
  page.route(routes.workflows, async (route) => {
    await route.fulfill({ json: { success: true, data: [EMERGENCY_WORKFLOW] } });
  });
  page.route(routes.casesCreate, async (route, request) => {
    if (request.method() !== 'POST') {
      await route.continue();
      return;
    }
    const body = JSON.parse(request.postData() || '{}');
    const created = {
      id: 'case-new',
      title: body.title,
      status: 'NEW',
      service_type: body.service_type,
      priority: body.priority,
      person_id: body.person_id,
    };
    await route.fulfill({ json: { success: true, data: created } });
  });
  // verifyCaseWorkflow GET after creation.
  page.route(routes.caseWorkflow('case-new'), async (route) => {
    await route.fulfill({
      json: {
        success: true,
        data: {
          instance: {
            id: 'inst-1',
            workflow_definition_id: EMERGENCY_WORKFLOW.id,
            definition: EMERGENCY_WORKFLOW,
            instance: { current_state: EMERGENCY_WORKFLOW.initial_state },
          },
        },
      },
    });
  });
}

// Open the dashboard (app.init auto-navigates here when a token is present).
async function openDashboard(page: Page) {
  // NOTE: addInitScript runs in the page context, so string literals only.
  await page.addInitScript(() => {
    localStorage.setItem('civora_token', 'dev-token');
    localStorage.setItem('civora_org_id', 'org-test');
  });
  await page.goto('/');
  await expect(page).toHaveURL(/#dashboard/);
  await expect(page.locator('#view-dashboard')).toBeVisible();
}

async function openOperationsDashboard(page: Page) {
  await page.addInitScript(() => {
    localStorage.setItem('civora_token', 'dev-token');
    localStorage.setItem('civora_org_id', 'org-test');
  });
  await page.goto('/');
  await page.waitForLoadState('networkidle');
  await page.evaluate(() => router.navigate('operations-dashboard'));
  await expect(page).toHaveURL(/#operations-dashboard/);
  await expect(page.locator('#view-operations-dashboard')).toBeVisible();
}

const WORKFLOW_ANALYSIS_REPORT = {
  organization_id: ORG_ID,
  calculated_at: new Date().toISOString(),
  period: 'daily',
  state_accumulations: [
    { workflow_key: 'emergency_assistance', state_key: 'IN_PROGRESS', case_count: 15, threshold: 10, observation: 'high volume in state IN_PROGRESS', language: 'high_volume', calculated_at: new Date().toISOString() },
  ],
  state_duration_anomalies: [
    { workflow_key: 'emergency_assistance', state_key: 'REVIEW', avg_duration_hours: 36, median_duration_hours: 32, max_duration_hours: 60, threshold_hours: 24, observation: 'longer observed duration in state REVIEW', language: 'longer_duration', calculated_at: new Date().toISOString() },
  ],
  workflow_closure_patterns: [
    { workflow_key: 'emergency_assistance', total_cases: 100, closed_cases: 80, rejected_cases: 5, closure_rate: 0.8, rejection_rate: 0.05, observation: 'closure rate within expected range', language: 'high_volume', calculated_at: new Date().toISOString() },
  ],
  information_request_patterns: [
    { workflow_key: 'emergency_assistance', total_cases: 100, info_request_count: 180, frequency_per_case: 1.8, threshold_per_case: 1.5, observation: 'higher frequency of information requests', language: 'higher_frequency', calculated_at: new Date().toISOString() },
  ],
  review_backlog_observations: [
    { pending_reviews: 25, assigned_reviews: 10, in_review_reviews: 15, avg_wait_time_hours: 48, threshold: 20, observation: 'increased backlog of pending reviews', language: 'increased_backlog', calculated_at: new Date().toISOString() },
  ],
  threshold_exceedances: [
    { case_id: 'case-1', case_number: 'CAS-001', workflow_key: 'emergency_assistance', current_state: 'REVIEW', threshold_type: 'state_duration_threshold_hours', threshold_value: 24, actual_value: 60, observation: 'longer observed duration in state REVIEW', language: 'longer_duration', calculated_at: new Date().toISOString() },
  ],
};

async function openWorkflowAnalysis(page: Page) {
  await page.addInitScript(() => {
    localStorage.setItem('civora_token', 'dev-token');
    localStorage.setItem('civora_org_id', 'org-test');
  });
  await page.goto('/');
  await page.waitForLoadState('networkidle');
  await page.evaluate(() => router.navigate('workflow-analysis'));
  await expect(page).toHaveURL(/#workflow-analysis/);
  await expect(page.locator('#view-workflow-analysis')).toBeVisible();
}

function attachWorkflowAnalysisMocks(page: Page) {
  page.route(routes.workflowAnalysis, async (route) => {
    await route.fulfill({ json: { success: true, data: WORKFLOW_ANALYSIS_REPORT } });
  });
  page.route(routes.analysisThresholds, async (route) => {
    await route.fulfill({ json: { success: true, data: { state_accumulation_threshold: 10, state_duration_threshold_hours: 24, aging_threshold_hours: 720, review_backlog_threshold: 20, info_request_frequency_per_case: 1.5, cycle_time_threshold_hours: 72 } } });
  });
}

const IMPACT_REPORT = {
  organization_id: ORG_ID,
  calculated_at: new Date().toISOString(),
  period: 'daily',
  bucket: new Date().toISOString(),
  metrics: [
    { category: 'activity', name: 'cases_created', label: 'Cases Created', description: 'Total cases opened in the period', activity_count: 10, outcome_count: 0, impact_value: 0, unit: 'cases', calculated_at: new Date().toISOString() },
    { category: 'outcome', name: 'cases_completed', label: 'Cases Completed', description: 'Cases moved to a closed terminal state', activity_count: 0, outcome_count: 6, impact_value: 0, unit: 'cases', calculated_at: new Date().toISOString() },
    { category: 'outcome', name: 'cases_rejected', label: 'Cases Rejected', description: 'Cases closed with a rejected status', activity_count: 0, outcome_count: 1, impact_value: 0, unit: 'cases', calculated_at: new Date().toISOString() },
    { category: 'outcome', name: 'cases_needing_information', label: 'Cases Needing Additional Information', description: 'Cases with a decision requesting more information', activity_count: 0, outcome_count: 2, impact_value: 0, unit: 'cases', calculated_at: new Date().toISOString() },
    { category: 'activity', name: 'assistance_created', label: 'Assistance Created', description: 'Assistance records opened', activity_count: 8, outcome_count: 0, impact_value: 0, unit: 'records', calculated_at: new Date().toISOString() },
    { category: 'outcome', name: 'assistance_completed', label: 'Assistance Completed', description: 'Assistance records marked as completed', activity_count: 0, outcome_count: 5, impact_value: 0, unit: 'records', calculated_at: new Date().toISOString() },
    { category: 'activity', name: 'follow_ups_scheduled', label: 'Follow-ups Scheduled', description: 'Follow-up appointments scheduled', activity_count: 4, outcome_count: 0, impact_value: 0, unit: 'records', calculated_at: new Date().toISOString() },
    { category: 'outcome', name: 'follow_ups_completed', label: 'Follow-ups Completed', description: 'Follow-up appointments marked as completed', activity_count: 0, outcome_count: 3, impact_value: 0, unit: 'records', calculated_at: new Date().toISOString() },
    { category: 'activity', name: 'evidence_submitted', label: 'Evidence Submitted', description: 'Evidence items uploaded to cases', activity_count: 12, outcome_count: 0, impact_value: 0, unit: 'items', calculated_at: new Date().toISOString() },
    { category: 'outcome', name: 'evidence_verified', label: 'Evidence Verified', description: 'Evidence items verified by staff', activity_count: 0, outcome_count: 9, impact_value: 0, unit: 'items', calculated_at: new Date().toISOString() },
    { category: 'activity', name: 'decisions_made', label: 'Decisions Made', description: 'Total human decisions recorded', activity_count: 7, outcome_count: 0, impact_value: 0, unit: 'decisions', calculated_at: new Date().toISOString() },
    { category: 'outcome', name: 'decisions_approved', label: 'Decisions Approved', description: 'Decisions with an approved outcome', activity_count: 0, outcome_count: 4, impact_value: 0, unit: 'decisions', calculated_at: new Date().toISOString() },
    { category: 'impact', name: 'case_completion_rate', label: 'Case Completion Rate', description: 'Share of opened cases that were completed in the period', activity_count: 10, outcome_count: 6, impact_value: 0.6, unit: 'ratio', calculated_at: new Date().toISOString() },
    { category: 'impact', name: 'case_rejection_rate', label: 'Case Rejection Rate', description: 'Share of opened cases that were rejected in the period', activity_count: 10, outcome_count: 1, impact_value: 0.1, unit: 'ratio', calculated_at: new Date().toISOString() },
    { category: 'impact', name: 'information_request_rate', label: 'Information Request Rate', description: 'Share of opened cases that needed more information', activity_count: 10, outcome_count: 2, impact_value: 0.2, unit: 'ratio', calculated_at: new Date().toISOString() },
    { category: 'impact', name: 'assistance_completion_rate', label: 'Assistance Completion Rate', description: 'Share of assistance records completed in the period', activity_count: 8, outcome_count: 5, impact_value: 0.625, unit: 'ratio', calculated_at: new Date().toISOString() },
    { category: 'impact', name: 'follow_up_completion_rate', label: 'Follow-up Completion Rate', description: 'Share of scheduled follow-ups completed in the period', activity_count: 4, outcome_count: 3, impact_value: 0.75, unit: 'ratio', calculated_at: new Date().toISOString() },
    { category: 'impact', name: 'evidence_verification_rate', label: 'Evidence Verification Rate', description: 'Share of submitted evidence verified in the period', activity_count: 12, outcome_count: 9, impact_value: 0.75, unit: 'ratio', calculated_at: new Date().toISOString() },
    { category: 'impact', name: 'decision_approval_rate', label: 'Decision Approval Rate', description: 'Share of decisions that were approved', activity_count: 7, outcome_count: 4, impact_value: 0.5714285714285714, unit: 'ratio', calculated_at: new Date().toISOString() },
  ],
};

async function openImpactIntelligence(page: Page) {
  await page.addInitScript(() => {
    localStorage.setItem('civora_token', 'dev-token');
    localStorage.setItem('civora_org_id', 'org-test');
  });
  await page.goto('/');
  await page.waitForLoadState('networkidle');
  await page.evaluate(() => router.navigate('impact-intelligence'));
  await expect(page).toHaveURL(/#impact-intelligence/);
  await expect(page.locator('#view-impact-intelligence')).toBeVisible();
}

function attachImpactMocks(page: Page) {
  page.route(routes.impactReport, async (route) => {
    await route.fulfill({ json: { success: true, data: IMPACT_REPORT } });
  });
}

const newCaseSubmit = (page: Page) =>
  page.locator('#view-new-case form[onsubmit*="createCase"] button[type="submit"]');

// Form definitions shared across Dynamic Form Renderer and Case Workspace specs.
const SAMPLE_FORM_DEFINITION = {
  id: 'form-sample-1',
  key: 'intake_form',
  name: 'Intake Questionnaire',
  description: 'Please provide the following information to process your request.',
  version: 1,
  status: 'ACTIVE',
  fields: [
    {
      key: 'full_name',
      label: 'Full Name',
      type: 'text',
      required: true,
      placeholder: 'Jane Doe',
      validation: { minLength: 2, maxLength: 100 },
    },
    {
      key: 'email',
      label: 'Email Address',
      type: 'email',
      required: true,
    },
    {
      key: 'household_size',
      label: 'Household Size',
      type: 'number',
      required: true,
      validation: { minValue: 1, maxValue: 50, minValue_message: 'Household size must be at least 1.', maxValue_message: 'Household size cannot exceed 50.' },
    },
    {
      key: 'income',
      label: 'Annual Income',
      type: 'decimal',
      required: false,
      validation: { minValue: 0 },
    },
    {
      key: 'preferred_contact',
      label: 'Preferred Contact Method',
      type: 'select',
      required: true,
      options: [
        { value: 'email', label: 'Email' },
        { value: 'phone', label: 'Phone' },
        { value: 'text', label: 'Text Message' },
      ],
    },
    {
      key: 'consent',
      label: 'I agree to the terms',
      type: 'boolean',
      required: true,
    },
    {
      key: 'notes',
      label: 'Additional Notes',
      type: 'textarea',
      required: false,
    },
  ],
};

const routes2 = {
  workflowForms: (caseId: string) => regex(`organizations/${ORG_ID}/cases/${caseId}/workflow/forms`),
  workflowFormSubmission: (caseId: string) => regex(`organizations/${ORG_ID}/cases/${caseId}/form/submission`),
};

const SECONDARY_FORM_DEFINITION = {
  id: 'form-sample-2',
  key: 'evidence_form',
  name: 'Evidence Form',
  description: 'Supporting documentation required.',
  version: 1,
  status: 'ACTIVE',
  required: false,
  fields: [
    {
      key: 'evidence_type',
      label: 'Evidence Type',
      type: 'select',
      required: true,
      options: [
        { value: 'id', label: 'ID Document' },
        { value: 'paycheck', label: 'Pay Stub' },
        { value: 'letter', label: 'Supporting Letter' },
      ],
    },
    {
      key: 'notes',
      label: 'Notes',
      type: 'textarea',
      required: false,
    },
  ],
};

const routes3 = {
  workflowForms: (caseId: string) => regex(`organizations/${ORG_ID}/cases/${caseId}/workflow/forms`),
  workflowFormSubmissions: (caseId: string) => regex(`organizations/${ORG_ID}/cases/${caseId}/workflow/form-submissions`),
  workflowFormSubmission: (caseId: string, formKey: string) => regex(`organizations/${ORG_ID}/cases/${caseId}/form/${formKey}/submission`),
};

async function setupCaseWorkspaceMocks(page: Page, caseId: string, options: {
  forms?: any[];
  submissions?: Record<string, any>;
  transitions?: any[];
  workflowState?: string;
} = {}) {
  const forms = options.forms || [SAMPLE_FORM_DEFINITION];
  const submissions = options.submissions || {};
  const transitions = options.transitions || [];
  const workflowState = options.workflowState || 'IN_PROGRESS';

  page.route(routes3.workflowForms(caseId), async (route) => {
    if (route.request().method() !== 'GET') { await route.continue(); return; }
    await route.fulfill({ json: { success: true, data: forms } });
  });
  page.route(routes3.workflowFormSubmissions(caseId), async (route) => {
    if (route.request().method() !== 'GET') { await route.continue(); return; }
    await route.fulfill({ json: { success: true, data: submissions } });
  });
  page.route(routes2.workflowFormSubmission(caseId), async (route) => {
    if (route.request().method() !== 'POST') { await route.continue(); return; }
    await route.fulfill({ json: { success: true, data: { id: 'submission-1', status: 'SUBMITTED' } } });
  });
  page.route(regex(`organizations/${ORG_ID}/cases/${caseId}$`), async (route) => {
    await route.fulfill({ json: { success: true, data: {
      id: caseId, case_number: 'CAS-100', title: 'Workspace Test',
      status: 'IN_PROGRESS', service_type: 'EMERGENCY', priority: 'HIGH', description: 'Test case.',
    } } });
  });
  page.route(regex(`organizations/${ORG_ID}/cases/${caseId}/workflow$`), async (route) => {
    await route.fulfill({ json: { success: true, data: {
      instance: { id: 'inst-1', current_state: workflowState },
      definition: { id: 'wf-1', states: [{ key: 'NEW' }, { key: workflowState }], transitions: [] },
    } } });
  });
  ['eligibilities', 'evidence', 'assessments', 'decisions', 'assistance', 'follow-ups'].forEach(section => {
    page.route(regex(`organizations/${ORG_ID}/${section}/by-service-request/${caseId}`), async (route, req) => {
      if (req.method() === 'GET') await route.fulfill({ json: { success: true, data: null } });
      else await route.continue();
    });
  });
  page.route(regex(`organizations/${ORG_ID}/cases/${caseId}/workflow/transitions$`), async (route) => {
    await route.fulfill({ json: { success: true, data: transitions } });
  });
  page.route(regex(`organizations/${ORG_ID}/cases/${caseId}/workflow/history`), async (route) => {
    await route.fulfill({ json: { success: true, data: [] } });
  });
}

// ── Form Management shared fixtures ──

const SAMPLE_FORM_TEMPLATE = {
  id: 'form-mgmt-1',
  key: 'intake_form',
  name: 'Intake Questionnaire',
  description: 'Please provide the following information to process your request.',
  version: 1,
  status: 'DRAFT',
  fields: [
    {
      key: 'full_name',
      label: 'Full Name',
      type: 'text',
      required: true,
      description: 'Applicant full name',
      placeholder: 'Jane Doe',
      validation: { minLength: 2, maxLength: 100 },
    },
    {
      key: 'email',
      label: 'Email Address',
      type: 'email',
      required: true,
    },
    {
      key: 'notes',
      label: 'Additional Notes',
      type: 'textarea',
      required: false,
    },
  ],
};

const SAMPLE_WORKFLOWS = [
  {
    id: 'wf-1',
    key: 'emergency_response',
    name: 'Emergency Response',
    description: 'Emergency assistance workflow',
    version: 1,
    status: 'ACTIVE',
    initial_state: 'NEW',
    states: [
      { key: 'NEW', name: 'New', terminal: false },
      { key: 'IN_REVIEW', name: 'In Review', terminal: false },
      { key: 'COMPLETED', name: 'Completed', terminal: true },
    ],
    transitions: [],
  },
];

function setupFormMockRoutes(page: Page, routeConfig: {
  forms?: { list?: any[], get?: Record<string, any>, create?: any, update?: any, delete?: any, assignments?: any[] };
  workflows?: any[];
}) {
  const cfg = routeConfig.forms || {};

  // List forms
  page.route(regex(`organizations/${ORG_ID}/forms`), async (route, req) => {
    if (req.method() === 'GET') {
      await route.fulfill({ json: { success: true, data: cfg.list || [], meta: { page: 1, total_pages: 1, total: (cfg.list || []).length, per_page: 20 } } });
    } else if (req.method() === 'POST') {
      await route.fulfill({ json: { success: true, data: cfg.create || SAMPLE_FORM_TEMPLATE } });
    } else {
      route.continue();
    }
  });

  // Get/update/delete specific form
  page.route(regex(`organizations/${ORG_ID}/forms/[^/]+/active-version`), async (route, req) => {
    if (req.method() === 'GET') {
      const formId = req.url().split('/').slice(-2)[0];
      const form = cfg.get?.[formId] || SAMPLE_FORM_TEMPLATE;
      await route.fulfill({ json: { success: true, data: { id: form.version_id || 'ver-1', form_id: form.id, version: form.version || 1, status: 'PUBLISHED' } } });
    } else {
      route.continue();
    }
  });

  // Get/update/delete specific form
  page.route(regex(`organizations/${ORG_ID}/forms/[^/]+$`), async (route, req) => {
    if (req.method() === 'GET') {
      await route.fulfill({ json: { success: true, data: cfg.get?.[req.url().split('/').pop()] || SAMPLE_FORM_TEMPLATE } });
    } else if (req.method() === 'PUT') {
      await route.fulfill({ json: { success: true, data: cfg.update || SAMPLE_FORM_TEMPLATE } });
    } else if (req.method() === 'DELETE') {
      await route.fulfill({ json: { success: true, data: null } });
    } else {
      route.continue();
    }
  });

  // Form assignments (workflow-scoped)
  page.route(regex(`organizations/${ORG_ID}/workflows/[^/]+/form-assignments`), async (route, req) => {
    if (req.method() === 'GET') {
      const assignments = (cfg.assignments || []).map(a => ({
        id: a.id,
        form_id: a.form_id || 'form-mgmt-1',
        workflow_state_key: a.state_key || a.workflow_state_key,
        required: a.required,
        display_order: a.display_order || 0,
        active: a.active !== undefined ? a.active : true,
      }));
      await route.fulfill({ json: { success: true, data: assignments } });
    } else if (req.method() === 'POST') {
      await route.fulfill({ json: { success: true, data: { id: 'assign-1', form_id: 'form-mgmt-1', workflow_state_key: 'IN_REVIEW', required: true } } });
    } else if (req.method() === 'DELETE') {
      await route.fulfill({ json: { success: true, data: null } });
    } else {
      route.continue();
    }
  });

  // Workflows list (for assignment)
  if (routeConfig.workflows) {
    page.route(regex(`organizations/${ORG_ID}/workflows(?:$|\\?)`), async (route, req) => {
      if (req.method() === 'GET') {
        const wfList = routeConfig.workflows || SAMPLE_WORKFLOWS;
        const url = req.url();
        if (url.includes('selectable')) {
          await route.fulfill({ json: { success: true, data: wfList.filter(w => w.status === 'ACTIVE'), meta: { page: 1 } } });
        } else if (url.includes('workflows/')) {
          // Single workflow
          const wf = wfList.find(w => w.id === url.split('/').pop()) || wfList[0];
          await route.fulfill({ json: { success: true, data: wf } });
        } else {
          await route.fulfill({ json: { success: true, data: wfList, meta: { page: 1 } } });
        }
      } else {
        route.continue();
      }
    });
  }
}

// Export everything for use in category-specific spec files
export {
  ORG_ID,
  ADMIN_TOKEN,
  EMERGENCY_WORKFLOW,
  PERSON,
  WORKFLOW_ANALYSIS_REPORT,
  IMPACT_REPORT,
  regex,
  orgPath,
  routes,
  routes2,
  routes3,
  SAMPLE_FORM_DEFINITION,
  SECONDARY_FORM_DEFINITION,
  SAMPLE_FORM_TEMPLATE,
  SAMPLE_WORKFLOWS,
  attachCaseMocks,
  openDashboard,
  openOperationsDashboard,
  openWorkflowAnalysis,
  attachWorkflowAnalysisMocks,
  openImpactIntelligence,
  attachImpactMocks,
  newCaseSubmit,
  setupCaseWorkspaceMocks,
  setupFormMockRoutes,
  test,
  expect,
  type Page,
};
