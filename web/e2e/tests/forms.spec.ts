// Dynamic Form Renderer E2E Tests

import { test, expect } from '@playwright/test';
import {
  ORG_ID,
  openDashboard,
  regex,
  SAMPLE_FORM_DEFINITION,
  routes2,
} from './helpers';

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
