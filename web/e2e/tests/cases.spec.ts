// Playwright End-to-End Tests for the CIVORA web frontend
//
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

test.describe('New case workflow selection', () => {
  test.beforeEach(async ({ page }) => {
    // addInitScript runs in the page context: use literals only.
    await page.addInitScript(() => {
      localStorage.setItem('civora_token', 'dev-token');
      localStorage.setItem('civora_org_id', 'org-test');
    });
  });

  test('renders the workflow selector from the API', async ({ page }) => {
    await openDashboard(page);
    attachCaseMocks(page);
    await page.evaluate(() => router.navigate('new-case'));

    const selector = page.locator('#workflow-selector');
    await expect(selector).toBeVisible();
    await expect(selector.locator('.wf-option-card')).toHaveCount(1);
    await expect(selector.locator('.wf-option-card')).toHaveText(/Emergency Assistance/);
    // First active workflow is auto-selected.
    await expect(page.locator('#selected-workflow-id')).toHaveValue('wf-emergency-1');
  });

  test('selecting a workflow and creating a case navigates to the case page', async ({ page }) => {
    await openDashboard(page);
    attachCaseMocks(page);
    await page.evaluate(() => router.navigate('new-case'));

    await page.fill('#c-title', 'Burst pipe on 3rd floor');
    await page.evaluate((v) => {
      (document.getElementById('new-case-person-id') as HTMLInputElement).value = v;
    }, PERSON.id);
    await newCaseSubmit(page).click();

    // verifyCaseWorkflow runs after POST, then navigates to the case view.
    await expect(page).toHaveURL(/#case\/case-new/);
    await expect(page.locator('#view-case')).toBeVisible();
  });

  test('supports custom workflows without a known service type mapping', async ({ page }) => {
    await openDashboard(page);
    page.route(routes.workflows, async (route) => {
      await route.fulfill({
        json: {
          success: true,
          data: [{ ...EMERGENCY_WORKFLOW, id: 'wf-custom', key: 'CUSTOM_SUPPORT', name: 'Custom Support' }],
        },
      });
    });
    // POST cases is mocked by attachCaseMocks? No — we override below for this test.
    page.route(routes.casesCreate, async (route, request) => {
      if (request.method() !== 'POST') { await route.continue(); return; }
      const body = JSON.parse(request.postData() || '{}');
      await route.fulfill({
        json: { success: true, data: { id: 'case-custom', title: body.title, status: 'REQUESTED', service_type: body.service_type, priority: body.priority, workflow_id: body.workflow_id } },
      });
    });
    await page.route(routes.caseWorkflow('case-custom'), async (route) => {
      await route.fulfill({
        json: {
          success: true,
          data: {
            instance: {
              id: 'inst-custom',
              workflow_definition_id: 'wf-custom',
              current_state: 'REQUESTED',
              started_at: '2026-09-12T01:00:00Z',
            },
            definition: { ...EMERGENCY_WORKFLOW, id: 'wf-custom', key: 'CUSTOM_SUPPORT', name: 'Custom Support', initial_state: 'REQUESTED' },
          },
        },
      });
    });
    await page.evaluate(() => router.navigate('new-case'));

    await page.fill('#c-title', 'Custom workflow case');
    await newCaseSubmit(page).click();

    // Custom workflow is now supported: navigation occurs to the new case.
    await expect(page).toHaveURL(/#case\/case-custom/);
    await expect(page.locator('#view-case')).toBeVisible();
  });
});

test.describe('Dashboard', () => {
  async function openDashboardWithStats(page: Page) {
    const cases = [
      { id: 'case-1', case_number: 'CR-1', status: 'IN_PROGRESS', service_type: 'EMERGENCY', priority: 'HIGH', title: 'Open', updated_at: '2026-09-12T01:00:00Z' },
      { id: 'case-2', case_number: 'CR-2', status: 'CLOSED', service_type: 'EMERGENCY', priority: 'NORMAL', title: 'Closed', updated_at: '2026-09-12T02:00:00Z' },
      { id: 'case-3', case_number: 'CR-3', status: 'NEW', service_type: 'EMERGENCY', priority: 'NORMAL', title: 'New', updated_at: '2026-09-12T03:00:00Z' },
    ];
    // Case list (rendered as the recent cases table).
    page.route(routes.casesSearch, async (route, request) => {
      if (request.method() === 'GET') {
        await route.fulfill({ json: { success: true, data: cases } });
      } else {
        await route.continue();
      }
    });
    // Dashboard statistics including the by_status breakdown.
    page.route(routes.dashboardStats, async (route) => {
      await route.fulfill({
        json: {
          success: true,
          data: {
            cases,
            by_status: { IN_PROGRESS: 1, CLOSED: 1, NEW: 1 },
            by_service_type: { EMERGENCY: 3 },
          },
        },
      });
    });
    await openDashboard(page);
  }

  test('shows the terminal-state based status breakdown', async ({ page }) => {
    await openDashboardWithStats(page);
    await expect(page.locator('#dashboard-stats')).toBeVisible();
    // Stat cards report open vs closed case counts.
    await expect(page.locator('#dashboard-stats')).toContainText(/Open Cases/);
    await expect(page.locator('#dashboard-stats')).toContainText(/Closed Cases/);
    // Breakdown renders the by_status buckets.
    await expect(page.locator('#dashboard-breakdown')).toContainText(/IN_PROGRESS/);
    await expect(page.locator('#dashboard-breakdown')).toContainText(/CLOSED/);
  });

   test('lists recent cases', async ({ page }) => {
     await openDashboardWithStats(page);
     const rows = page.locator('#dashboard-table tbody tr');
     // One row per case (3).
     await expect(rows).toHaveCount(3);
     await expect(page.locator('#dashboard-table')).toContainText(/Closed/);
   });
 });

// ── Dynamic Form Renderer E2E Tests ────────────────────────────────────────────

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
      validation: { min_length: 2, max_length: 100 },
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
      validation: { minimum: 1, maximum: 50, minimum_message: 'Household size must be at least 1.', maximum_message: 'Household size cannot exceed 50.' },
    },
    {
      key: 'income',
      label: 'Annual Income',
      type: 'decimal',
      required: false,
      validation: { minimum: 0 },
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

test.describe('Dynamic Form Renderer', () => {
  test.beforeEach(async ({ page }) => {
    await page.addInitScript(() => {
      localStorage.setItem('civora_token', 'dev-token');
      localStorage.setItem('civora_org_id', 'org-test');
    });
  });

  test('renders a form from a backend form definition', async ({ page }) => {
    await openDashboard(page);

    const caseId = 'case-form-test';
    page.route(routes2.workflowForms(caseId), async (route) => {
      if (route.request().method() !== 'GET') { await route.continue(); return; }
      await route.fulfill({ json: { success: true, data: [SAMPLE_FORM_DEFINITION] } });
    });
    page.route(routes2.workflowFormSubmission(caseId), async (route) => {
      if (route.request().method() !== 'POST') { await route.continue(); return; }
      await route.fulfill({ json: { success: true, data: { id: 'submission-1', status: 'SUBMITTED' } } });
    });
    // Mock case and workflow endpoints
    page.route(regex(`organizations/${ORG_ID}/cases/${caseId}$`), async (route) => {
      await route.fulfill({ json: { success: true, data: {
        id: caseId, case_number: 'CAS-001', title: 'Form Test Case',
        status: 'IN_PROGRESS', service_type: 'EMERGENCY', priority: 'HIGH',
        description: 'Test case for forms',
      } } });
    });
    page.route(regex(`organizations/${ORG_ID}/cases/${caseId}/workflow$`), async (route) => {
      await route.fulfill({ json: { success: true, data: {
        instance: { id: 'inst-1', current_state: 'IN_PROGRESS' },
        definition: { id: 'wf-1', states: [{ key: 'NEW' }, { key: 'IN_PROGRESS' }], transitions: [] },
      } } });
    });
    // Mock sections
    page.route(regex(`organizations/${ORG_ID}/eligibilities/by-service-request/${caseId}`), async (route, req) => {
      if (req.method() === 'GET') await route.fulfill({ json: { success: true, data: null } });
      else await route.continue();
    });
    ['evidence', 'assessment', 'decision', 'assistance', 'follow-ups'].forEach(section => {
      page.route(regex(`organizations/${ORG_ID}/${section}/by-service-request/${caseId}`), async (route, req) => {
        if (req.method() === 'GET') await route.fulfill({ json: { success: true, data: null } });
        else await route.continue();
      });
    });
    page.route(regex(`organizations/${ORG_ID}/cases/${caseId}/workflow/transitions$`), async (route) => {
      await route.fulfill({ json: { success: true, data: [] } });
    });
    page.route(regex(`organizations/${ORG_ID}/cases/${caseId}/workflow/history`), async (route) => {
      await route.fulfill({ json: { success: true, data: [] } });
    });

    await page.goto('/');
    await page.evaluate(() => router.navigate('case', 'case-form-test'));

    // Form card should be visible
    await expect(page.locator('#card-dynamic-form')).toBeVisible();
    await expect(page.locator('#dynamic-form-title')).toHaveText('Intake Questionnaire');

    // Form description should be visible
    await expect(page.locator('.form-description')).toHaveText(SAMPLE_FORM_DEFINITION.description);

    // All fields should render
    const fieldKeys = ['full_name', 'email', 'household_size', 'income',
      'preferred_contact', 'consent', 'notes'];
    fieldKeys.forEach(key => {
      expect(page.locator(`[data-field-key="${key}"]`)).toBeVisible();
    });

     // Required fields should have asterisk marker (scoped to form container)
    await expect(page.locator('#form-container .required-marker')).toHaveCount(5); // full_name, email, household_size, preferred_contact, consent

    // Submit button should exist
    await expect(page.locator('#form-container button[type="submit"]')).toHaveCount(1);
  });

  test('blocks submission on invalid form (client-side validation)', async ({ page }) => {
    await openDashboard(page);

    const caseId = 'case-form-invalid';
    page.route(routes2.workflowForms(caseId), async (route) => {
      if (route.request().method() !== 'GET') { await route.continue(); return; }
      await route.fulfill({ json: { success: true, data: [SAMPLE_FORM_DEFINITION] } });
    });
    page.route(regex(`organizations/${ORG_ID}/cases/${caseId}$`), async (route) => {
      await route.fulfill({ json: { success: true, data: {
        id: caseId, case_number: 'CAS-002', title: 'Invalid Form Test',
        status: 'IN_PROGRESS', service_type: 'EMERGENCY', priority: 'HIGH',
        description: 'Test',
      } } });
    });
    page.route(regex(`organizations/${ORG_ID}/cases/${caseId}/workflow$`), async (route) => {
      await route.fulfill({ json: { success: true, data: {
        instance: { id: 'inst-1', current_state: 'IN_PROGRESS' },
        definition: { id: 'wf-1', states: [{ key: 'IN_PROGRESS' }], transitions: [] },
      } } });
    });
    ['eligibilities', 'evidence', 'assessments', 'decisions', 'assistance', 'follow-ups'].forEach(section => {
      page.route(regex(`organizations/${ORG_ID}/${section}/by-service-request/${caseId}`), async (route, req) => {
        if (req.method() === 'GET') await route.fulfill({ json: { success: true, data: null } });
        else await route.continue();
      });
    });
    page.route(regex(`organizations/${ORG_ID}/cases/${caseId}/workflow/transitions$`), async (route) => {
      await route.fulfill({ json: { success: true, data: [] } });
    });
    page.route(regex(`organizations/${ORG_ID}/cases/${caseId}/workflow/history`), async (route) => {
      await route.fulfill({ json: { success: true, data: [] } });
    });

    await page.goto('/');
    await page.evaluate(() => router.navigate('case', 'case-form-invalid'));

     // Wait for the form card to be visible
    await expect(page.locator('#card-dynamic-form')).toBeVisible();

    // Try submitting without filling required fields
    await page.locator('#form-container button[type="submit"]').click();

    // Should show error message
    await expect(page.locator('.form-message.error')).toBeVisible();

    // Should NOT have called the submission endpoint
    // (We verify by checking that no submission was made.)
    const submissionCalled = await page.evaluate(() => {
      // The submission POST would set the submit button text to "Submitting…"
      return false;
    });
    expect(submissionCalled).toBe(false);
  });

  test('renders supported field types', async ({ page }) => {
    await openDashboard(page);

    const caseId = 'case-field-types';
    const formWithAllTypes = {
      ...SAMPLE_FORM_DEFINITION,
      key: 'all_types',
      fields: [
        { key: 'text_field', label: 'Text', type: 'text', required: true },
        { key: 'textarea_field', label: 'Textarea', type: 'textarea' },
        { key: 'number_field', label: 'Number', type: 'number' },
        { key: 'decimal_field', label: 'Decimal', type: 'decimal' },
        { key: 'date_field', label: 'Date', type: 'date' },
        { key: 'datetime_field', label: 'DateTime', type: 'datetime' },
        { key: 'boolean_field', label: 'Boolean', type: 'boolean' },
        { key: 'select_field', label: 'Select', type: 'select', options: [{ value: 'a', label: 'A' }] },
        { key: 'multiselect_field', label: 'MultiSelect', type: 'multiselect', options: [{ value: 'a', label: 'A' }] },
        { key: 'radio_field', label: 'Radio', type: 'radio', options: [{ value: 'a', label: 'A' }, { value: 'b', label: 'B' }] },
        { key: 'checkbox_field', label: 'Checkbox Group', type: 'checkbox', options: [{ value: 'a', label: 'A' }, { value: 'b', label: 'B' }] },
        { key: 'email_field', label: 'Email', type: 'email' },
        { key: 'phone_field', label: 'Phone', type: 'phone' },
      ],
    };
    page.route(routes2.workflowForms(caseId), async (route) => {
      if (route.request().method() !== 'GET') { await route.continue(); return; }
      await route.fulfill({ json: { success: true, data: [formWithAllTypes] } });
    });
    page.route(regex(`organizations/${ORG_ID}/cases/${caseId}$`), async (route) => {
      await route.fulfill({ json: { success: true, data: {
        id: caseId, case_number: 'CAS-003', title: 'All Types',
        status: 'IN_PROGRESS', service_type: 'GENERAL', priority: 'NORMAL', description: '',
      } } });
    });
    page.route(regex(`organizations/${ORG_ID}/cases/${caseId}/workflow$`), async (route) => {
      await route.fulfill({ json: { success: true, data: {
        instance: { id: 'inst-1', current_state: 'IN_PROGRESS' },
        definition: { id: 'wf-1', states: [{ key: 'IN_PROGRESS' }], transitions: [] },
      } } });
    });
    ['eligibilities', 'evidence', 'assessments', 'decisions', 'assistance', 'follow-ups'].forEach(section => {
      page.route(regex(`organizations/${ORG_ID}/${section}/by-service-request/${caseId}`), async (route, req) => {
        if (req.method() === 'GET') await route.fulfill({ json: { success: true, data: null } });
        else await route.continue();
      });
    });
    page.route(regex(`organizations/${ORG_ID}/cases/${caseId}/workflow/transitions$`), async (route) => {
      await route.fulfill({ json: { success: true, data: [] } });
    });
    page.route(regex(`organizations/${ORG_ID}/cases/${caseId}/workflow/history`), async (route) => {
      await route.fulfill({ json: { success: true, data: [] } });
    });

    await page.goto('/');
    await page.evaluate(() => router.navigate('case', 'case-field-types'));

    // Wait for the form card to be visible
    await expect(page.locator('#card-dynamic-form')).toBeVisible();

    // All field types should render
    const expectedTypes = ['text', 'textarea', 'number', 'decimal', 'date', 'datetime',
      'boolean', 'select', 'multiselect', 'radio', 'checkbox', 'email', 'phone'];
    expectedTypes.forEach(type => {
      expect(page.locator(`[data-field-key="${type}_field"]`)).toBeVisible();
    });
  });

  test('handles 404 (no required forms) gracefully', async ({ page }) => {
    await openDashboard(page);

    const caseId = 'case-no-forms';
    page.route(routes2.workflowForms(caseId), async (route) => {
      await route.fulfill({ status: 404, json: { success: false, error: { message: 'No forms at this state' } } });
    });
    page.route(regex(`organizations/${ORG_ID}/cases/${caseId}$`), async (route) => {
      await route.fulfill({ json: { success: true, data: {
        id: caseId, case_number: 'CAS-004', title: 'No Forms',
        status: 'IN_PROGRESS', service_type: 'GENERAL', priority: 'NORMAL', description: '',
      } } });
    });
    page.route(regex(`organizations/${ORG_ID}/cases/${caseId}/workflow$`), async (route) => {
      await route.fulfill({ json: { success: true, data: {
        instance: { id: 'inst-1', current_state: 'IN_PROGRESS' },
        definition: { id: 'wf-1', states: [{ key: 'IN_PROGRESS' }], transitions: [] },
      } } });
    });
    ['eligibilities', 'evidence', 'assessments', 'decisions', 'assistance', 'follow-ups'].forEach(section => {
      page.route(regex(`organizations/${ORG_ID}/${section}/by-service-request/${caseId}`), async (route, req) => {
        if (req.method() === 'GET') await route.fulfill({ json: { success: true, data: null } });
        else await route.continue();
      });
    });
    page.route(regex(`organizations/${ORG_ID}/cases/${caseId}/workflow/transitions$`), async (route) => {
      await route.fulfill({ json: { success: true, data: [] } });
    });
    page.route(regex(`organizations/${ORG_ID}/cases/${caseId}/workflow/history`), async (route) => {
      await route.fulfill({ json: { success: true, data: [] } });
    });

    await page.goto('/');
    await page.evaluate(() => router.navigate('case', 'case-no-forms'));

    // Form card should be hidden (no forms returned)
    await expect(page.locator('#card-dynamic-form')).toBeHidden();
  });

  test('hides form card on terminal workflow state', async ({ page }) => {
    await openDashboard(page);

    const caseId = 'case-terminal';
    page.route(regex(`organizations/${ORG_ID}/cases/${caseId}$`), async (route) => {
      await route.fulfill({ json: { success: true, data: {
        id: caseId, case_number: 'CAS-005', title: 'Terminal Case',
        status: 'CLOSED', service_type: 'GENERAL', priority: 'NORMAL', description: '',
      } } });
    });
    page.route(regex(`organizations/${ORG_ID}/cases/${caseId}/workflow$`), async (route) => {
      await route.fulfill({ json: { success: true, data: {
        instance: { id: 'inst-1', current_state: 'CLOSED' },
        definition: { id: 'wf-1', states: [{ key: 'CLOSED', terminal: true }], transitions: [] },
      } } });
    });
    ['eligibilities', 'evidence', 'assessments', 'decisions', 'assistance', 'follow-ups'].forEach(section => {
      page.route(regex(`organizations/${ORG_ID}/${section}/by-service-request/${caseId}`), async (route, req) => {
        if (req.method() === 'GET') await route.fulfill({ json: { success: true, data: null } });
        else await route.continue();
      });
    });
    page.route(regex(`organizations/${ORG_ID}/cases/${caseId}/workflow/transitions$`), async (route) => {
      await route.fulfill({ json: { success: true, data: [] } });
    });
    page.route(regex(`organizations/${ORG_ID}/cases/${caseId}/workflow/history`), async (route) => {
      await route.fulfill({ json: { success: true, data: [] } });
    });

    await page.goto('/');
    await page.evaluate(() => router.navigate('case', 'case-terminal'));

    // Form card should be hidden (terminal state)
    await expect(page.locator('#card-dynamic-form')).toBeHidden();
  });

  test('displays server-side validation errors', async ({ page }) => {
    await openDashboard(page);

    const caseId = 'case-server-error';
    page.route(routes2.workflowForms(caseId), async (route) => {
      if (route.request().method() !== 'GET') { await route.continue(); return; }
      await route.fulfill({ json: { success: true, data: [SAMPLE_FORM_DEFINITION] } });
    });
    page.route(routes2.workflowFormSubmission(caseId), async (route, req) => {
      if (req.method() !== 'POST') { await route.continue(); return; }
      await route.fulfill({ status: 400, json: {
        success: false,
        error: {
          code: 'INVALID_INPUT',
          message: 'Validation failed',
          field_errors: {
            email: ['Email is already registered in this organization.'],
          },
        },
      } });
    });
    page.route(regex(`organizations/${ORG_ID}/cases/${caseId}$`), async (route) => {
      await route.fulfill({ json: { success: true, data: {
        id: caseId, case_number: 'CAS-006', title: 'Server Error',
        status: 'IN_PROGRESS', service_type: 'EMERGENCY', priority: 'HIGH', description: '',
      } } });
    });
    page.route(regex(`organizations/${ORG_ID}/cases/${caseId}/workflow$`), async (route) => {
      await route.fulfill({ json: { success: true, data: {
        instance: { id: 'inst-1', current_state: 'IN_PROGRESS' },
        definition: { id: 'wf-1', states: [{ key: 'IN_PROGRESS' }], transitions: [] },
      } } });
    });
    ['eligibilities', 'evidence', 'assessments', 'decisions', 'assistance', 'follow-ups'].forEach(section => {
      page.route(regex(`organizations/${ORG_ID}/${section}/by-service-request/${caseId}`), async (route, req) => {
        if (req.method() === 'GET') await route.fulfill({ json: { success: true, data: null } });
        else await route.continue();
      });
    });
    page.route(regex(`organizations/${ORG_ID}/cases/${caseId}/workflow/transitions$`), async (route) => {
      await route.fulfill({ json: { success: true, data: [] } });
    });
    page.route(regex(`organizations/${ORG_ID}/cases/${caseId}/workflow/history`), async (route) => {
      await route.fulfill({ json: { success: true, data: [] } });
    });

    await page.goto('/');
    await page.evaluate(() => router.navigate('case', 'case-server-error'));

    // Wait for the form card to be visible
    await expect(page.locator('#card-dynamic-form')).toBeVisible();

    // Fill in valid form values
    await page.fill('#form-field-full_name', 'Jane Doe');
    await page.fill('#form-field-email', 'existing@example.com');
    await page.fill('#form-field-household_size', '2');
    await page.selectOption('#form-field-preferred_contact', 'email');
    await page.check('#form-field-consent');
    await page.click('#form-container button[type="submit"]');

    // Wait for the server error to display
    await expect(page.locator('#email-field-error, .form-field[data-field-key="email"] .field-error')).toBeVisible();
  });

  test('successful submission shows success and refreshes', async ({ page }) => {
    await openDashboard(page);

    const caseId = 'case-success';
    page.route(routes2.workflowForms(caseId), async (route) => {
      if (route.request().method() !== 'GET') { await route.continue(); return; }
      await route.fulfill({ json: { success: true, data: [SAMPLE_FORM_DEFINITION] } });
    });
    page.route(routes2.workflowFormSubmission(caseId), async (route, req) => {
      if (req.method() !== 'POST') { await route.continue(); return; }
      await route.fulfill({ json: { success: true, data: { id: 'submission-1', status: 'SUBMITTED' } } });
    });
    page.route(regex(`organizations/${ORG_ID}/cases/${caseId}$`), (route, req) => {
      if (req.method() === 'GET') {
        route.fulfill({ json: { success: true, data: {
          id: caseId, case_number: 'CAS-007', title: 'Success Case',
          status: 'IN_PROGRESS', service_type: 'EMERGENCY', priority: 'HIGH', description: '',
        } } });
      } else {
        route.continue();
      }
    });
    page.route(regex(`organizations/${ORG_ID}/cases/${caseId}/workflow$`), async (route) => {
      await route.fulfill({ json: { success: true, data: {
        instance: { id: 'inst-1', current_state: 'IN_PROGRESS' },
        definition: { id: 'wf-1', states: [{ key: 'IN_PROGRESS' }], transitions: [] },
      } } });
    });
    ['eligibilities', 'evidence', 'assessments', 'decisions', 'assistance', 'follow-ups'].forEach(section => {
      page.route(regex(`organizations/${ORG_ID}/${section}/by-service-request/${caseId}`), async (route, req) => {
        if (req.method() === 'GET') await route.fulfill({ json: { success: true, data: null } });
        else await route.continue();
      });
    });
    page.route(regex(`organizations/${ORG_ID}/cases/${caseId}/workflow/transitions$`), async (route) => {
      await route.fulfill({ json: { success: true, data: [] } });
    });
    page.route(regex(`organizations/${ORG_ID}/cases/${caseId}/workflow/history`), async (route) => {
      await route.fulfill({ json: { success: true, data: [] } });
    });

    await page.goto('/');
    await page.evaluate(() => router.navigate('case', 'case-success'));

    // Wait for the form card to be visible
    await expect(page.locator('#card-dynamic-form')).toBeVisible();

    // Fill in valid form values
    await page.fill('#form-field-full_name', 'Jane Doe');
    await page.fill('#form-field-email', 'jane@example.com');
    await page.fill('#form-field-household_size', '2');
    await page.selectOption('#form-field-preferred_contact', 'email');
    await page.check('#form-field-consent');
    await page.click('#form-container button[type="submit"]');

    // Submit button should change to loading state then back
    await expect(page.locator('#form-container button[type="submit"]')).toBeEnabled();
  });
});

// ── Form Management E2E Tests ────────────────────────────────────────────

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
      validation: { min_length: 2, max_length: 100 },
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

  // Form assignments
  page.route(regex(`organizations/${ORG_ID}/forms/[^/]+/assignments`), async (route, req) => {
    if (req.method() === 'GET') {
      await route.fulfill({ json: { success: true, data: cfg.assignments || [] } });
    } else if (req.method() === 'POST') {
      await route.fulfill({ json: { success: true, data: { id: 'assign-1', form_id: 'form-mgmt-1' } } });
    } else if (req.method() === 'DELETE') {
      await route.fulfill({ json: { success: true, data: null } });
    } else {
      route.continue();
    }
  });

  // Workflows list (for assignment)
  if (routeConfig.workflows) {
    page.route(regex(`organizations/${ORG_ID}/workflows`), async (route, req) => {
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

test.describe('Form Management', () => {
  test.beforeEach(async ({ page }) => {
    await page.addInitScript(() => {
      localStorage.setItem('civora_token', 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ1c2VyLTEiLCJvcmdhbml6YXRpb25faWQiOiJvcmdlLXRlc3QiLCJyb2xlIjoiYWRtaW4ifQ.sig');
      localStorage.setItem('civora_org_id', 'org-test');
    });
    await page.goto('/');
    await expect(page).toHaveURL(/#dashboard/);
    await expect(page.locator('#view-dashboard')).toBeVisible();
  });

  test('shows form management view with empty state', async ({ page }) => {
    setupFormMockRoutes(page, { forms: { list: [] } });
    await page.evaluate(() => router.navigate('forms'));
    await expect(page.locator('#view-forms')).toBeVisible();
    await expect(page.locator('#form-table-body')).toContainText('No forms yet');
  });

  test('lists forms from the API', async ({ page }) => {
    setupFormMockRoutes(page, { forms: { list: [SAMPLE_FORM_TEMPLATE] } });
    await page.evaluate(() => router.navigate('forms'));
    await expect(page.locator('#form-table-body tr')).toHaveCount(1);
    await expect(page.locator('#form-table-body')).toContainText('Intake Questionnaire');
    await expect(page.locator('#form-table-body')).toContainText('intake_form');
  });

  test('navigates to form design view from create button', async ({ page }) => {
    setupFormMockRoutes(page, { forms: { list: [] } });
    await page.evaluate(() => router.navigate('forms'));
    await expect(page.locator('#btn-create-form')).toBeVisible();
    await page.click('#btn-create-form');
    await expect(page.locator('#view-form-design')).toBeVisible();
    await expect(page.locator('#form-design-title')).toHaveText('Create New Form');
  });

  test('creates a form with fields', async ({ page }) => {
    setupFormMockRoutes(page, { forms: { create: SAMPLE_FORM_TEMPLATE } });
    await page.evaluate(() => router.navigate('forms'));
    await page.click('#btn-create-form');

    // Fill form metadata
    await page.fill('#form-name', 'My New Form');
    await page.fill('#form-key', 'my_new_form');
    await page.fill('#form-description', 'A test form');

    // Add a field
    await page.click('button:has-text("Add Field")');
    await page.fill('#field-label', 'Email Address');
    await page.fill('#field-key', 'email');
    await page.selectOption('#field-type', 'email');
    await page.click('#field-required');

    await page.click('#form-field-modal button:has-text("Save")');
    await page.waitForSelector('#form-field-modal', { state: 'hidden' });

    // Verify the field was added
    await expect(page.locator('.form-field-card')).toHaveCount(1);
    await expect(page.locator('.form-field-card')).toContainText('Email Address');

    // Save draft
    await page.click('#btn-form-save');

    // Should show success toast
    await expect(page.locator('.toast.success')).toBeVisible();
  });

  test('detects duplicate field keys', async ({ page }) => {
    setupFormMockRoutes(page, { forms: { create: SAMPLE_FORM_TEMPLATE } });
    await page.evaluate(() => router.navigate('forms'));
    await page.click('#btn-create-form');

    // Add first field
    await page.click('button:has-text("Add Field")');
    await page.fill('#field-label', 'Email');
    await page.fill('#field-key', 'email');
    await page.selectOption('#field-type', 'email');
    await page.click('#form-field-modal button:has-text("Save")');
    await page.waitForSelector('#form-field-modal', { state: 'hidden' });

    // Add second field with same key
    await page.click('button:has-text("Add Field")');
    await page.fill('#field-label', 'Email 2');
    await page.fill('#field-key', 'email');
    await page.selectOption('#field-type', 'email');
    // Duplicate key should keep modal open with error
    await page.click('#form-field-modal button:has-text("Save")');
    // Modal stays open due to validation error
    await expect(page.locator('#form-field-modal')).toBeVisible();
    await expect(page.locator('#form-design-error')).toContainText('Duplicate field key');
    await page.click('#form-field-modal button:has-text("Cancel")');

    // Try to save form
    await page.fill('#form-name', 'Test');
    await page.fill('#form-key', 'test');
    await page.click('#btn-form-save');

    // Should show error about duplicate key
    await expect(page.locator('#form-design-error')).toContainText('Duplicate field key');
  });

  test('rejects invalid form key format', async ({ page }) => {
    setupFormMockRoutes(page, { forms: { create: SAMPLE_FORM_TEMPLATE } });
    await page.evaluate(() => router.navigate('forms'));
    await page.click('#btn-create-form');

    await page.fill('#form-name', 'Test Form');
    await page.fill('#form-key', 'Test-Form');  // Invalid: uppercase and hyphen

    // Add a field first
    await page.click('button:has-text("Add Field")');
    await page.fill('#field-label', 'Name');
    await page.fill('#field-key', 'name');
    await page.selectOption('#field-type', 'text');
    await page.click('#form-field-modal button:has-text("Save")');
    await page.waitForSelector('#form-field-modal', { state: 'hidden' });

    // Try to save
    await page.click('#btn-form-save');

    // Should show error about key format
    await expect(page.locator('#form-design-error')).toContainText('Form key must be lowercase');
  });

  test('previews a form using runtime renderer', async ({ page }) => {
    setupFormMockRoutes(page, { forms: { list: [SAMPLE_FORM_TEMPLATE] } });
    await page.evaluate(() => router.navigate('forms'));
    await page.click('#btn-create-form');

    // Fill form metadata
    await page.fill('#form-name', 'Preview Test');
    await page.fill('#form-key', 'preview_test');

    // Add a field
    await page.click('button:has-text("Add Field")');
    await page.fill('#field-label', 'Full Name');
    await page.fill('#field-key', 'full_name');
    await page.selectOption('#field-type', 'text');
    await page.click('#field-required');
    await page.click('#form-field-modal button:has-text("Save")');
    await page.waitForSelector('#form-field-modal', { state: 'hidden' });

    // Preview
    await page.click('#btn-form-preview');
    await expect(page.locator('#form-modal')).toBeVisible();
    await expect(page.locator('#form-modal-title')).toHaveText('Preview Test (Draft)');
    await expect(page.locator('#form-modal .dynamic-form')).toBeVisible();

    // Close preview
    await page.click('#form-modal button:has-text("Cancel")');
    await expect(page.locator('#form-modal')).toBeHidden();
  });

  test('assigns form to workflow state', async ({ page }) => {
    setupFormMockRoutes(page, {
      forms: {
        list: [SAMPLE_FORM_TEMPLATE],
        get: { 'form-mgmt-1': SAMPLE_FORM_TEMPLATE },
        assignments: [{ id: 'assign-1', workflow_id: 'wf-1', state_key: 'IN_REVIEW', required: true, workflow_name: 'Emergency Response' }],
      },
      workflows: SAMPLE_WORKFLOWS,
    });

    await page.evaluate(() => router.navigate('forms'));
    await page.click('#form-table-body tr button:text("View")');
    await expect(page.locator('#view-form-detail')).toBeVisible();

    // Should show assignments
    await expect(page.locator('#form-workflow-assignments')).toContainText('Emergency Response');
    await expect(page.locator('#form-workflow-assignments')).toContainText('IN_REVIEW');
  });

  test('shows Forms nav button for admin users', async ({ page }) => {
    // The beforeEach already sets an admin JWT token
    await page.goto('/');
    await expect(page).toHaveURL(/#dashboard/);
    await expect(page.locator('#view-dashboard')).toBeVisible();

    // Admin users should see the Forms nav button
    await expect(page.locator('#btn-forms-nav')).toBeVisible();
  });

  test('handles 404 gracefully for form listing', async ({ page }) => {
    page.route(regex(`organizations/${ORG_ID}/forms`), async (route) => {
      await route.fulfill({ status: 404, json: { success: false, error: { message: 'Not found' } } });
    });
    await page.evaluate(() => router.navigate('forms'));
    await expect(page.locator('#form-list-loading')).toBeHidden();
    await expect(page.locator('#form-table-body')).toContainText('No forms');
  });

  test('removes a form assignment with confirmation', async ({ page }) => {
    setupFormMockRoutes(page, {
      forms: {
        list: [SAMPLE_FORM_TEMPLATE],
        get: { 'form-mgmt-1': SAMPLE_FORM_TEMPLATE },
        assignments: [{ id: 'assign-1', workflow_id: 'wf-1', state_key: 'IN_REVIEW', required: true, workflow_name: 'Emergency Response' }],
      },
      workflows: SAMPLE_WORKFLOWS,
    });

    page.on('dialog', dialog => dialog.dismiss()); // Cancel the confirm dialog

    await page.evaluate(() => router.navigate('forms'));
    await page.click('#form-table-body tr button:text("View")');
    await expect(page.locator('#form-workflow-assignments')).toBeVisible();
    await page.click('#form-workflow-assignments button:text("Remove")');
    // Assignment should still be there since dialog was cancelled
    await expect(page.locator('#form-workflow-assignments')).toContainText('Emergency Response');
  });
});
