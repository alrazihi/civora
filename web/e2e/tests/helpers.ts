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

// A fully-defined workflow. Its `key` (EMERGENCY) is mapped by
// deriveServiceTypeForWorkflow to the EMERGENCY service domain.
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
  newCaseSubmit,
  setupCaseWorkspaceMocks,
  setupFormMockRoutes,
  test,
  expect,
  type Page,
};
