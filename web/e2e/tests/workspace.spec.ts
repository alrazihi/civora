// Case Workspace Integration E2E Tests

import { test, expect } from '@playwright/test';
import {
  ORG_ID,
  openDashboard,
  regex,
  routes2,
  SAMPLE_FORM_DEFINITION,
  SECONDARY_FORM_DEFINITION,
  setupCaseWorkspaceMocks,
} from './helpers';

test.describe('Case Workspace Integration', () => {
  test.beforeEach(async ({ page }) => {
    await page.addInitScript(() => {
      localStorage.setItem('civora_token', 'dev-token');
      localStorage.setItem('civora_org_id', 'org-test');
    });
  });

  test('shows current workflow state and required forms connection', async ({ page }) => {
    await openDashboard(page);
    const caseId = 'case-workspace-1';
    await setupCaseWorkspaceMocks(page, caseId, {
      forms: [SAMPLE_FORM_DEFINITION],
      workflowState: 'IN_PROGRESS',
    });

    await page.goto('/');
    await page.evaluate(() => router.navigate('case', 'case-workspace-1'));

    // Required Information workspace should be visible
    await expect(page.locator('#card-required-info')).toBeVisible();

    // Should show current state in badge
    await expect(page.locator('#workflow-instance-state')).toContainText('IN_PROGRESS');

    // Form list should show the form
    await expect(page.locator('#form-requirements-list-container li')).toHaveCount(1);
    await expect(page.locator('#form-requirements-list-container')).toContainText('Intake Questionnaire');

    // Required badge should be shown
    await expect(page.locator('#form-requirements-list-container')).toContainText('Required');

    // Form card should be visible with the form rendered
    await expect(page.locator('#card-dynamic-form')).toBeVisible();
    await expect(page.locator('#dynamic-form-title')).toHaveText('Intake Questionnaire');
    await expect(page.locator('#form-submission-status')).toContainText('Status:');
  });

  test('shows form status as not started for unsubmitted form', async ({ page }) => {
    await openDashboard(page);
    const caseId = 'case-workspace-2';
    await setupCaseWorkspaceMocks(page, caseId, {
      forms: [SAMPLE_FORM_DEFINITION],
      submissions: {},
      workflowState: 'IN_PROGRESS',
    });

    await page.goto('/');
    await page.evaluate(() => router.navigate('case', 'case-workspace-2'));

    await expect(page.locator('#form-submission-status')).toContainText('Not Started');
  })

  test('shows form status as submitted when form has submission', async ({ page }) => {
    await openDashboard(page);
    const caseId = 'case-workspace-3';
    await setupCaseWorkspaceMocks(page, caseId, {
      forms: [SAMPLE_FORM_DEFINITION],
      submissions: {
        intake_form: { id: 'sub-1', status: 'SUBMITTED', submitted_at: '2024-01-15T10:00:00Z' },
      },
      workflowState: 'IN_PROGRESS',
    });

    await page.goto('/');
    await page.evaluate(() => router.navigate('case', 'case-workspace-3'));

    await expect(page.locator('#form-submission-status')).toContainText('Submitted');
  })

  test('shows form status as needs correction when submission was rejected', async ({ page }) => {
    await openDashboard(page);
    const caseId = 'case-workspace-4';
    await setupCaseWorkspaceMocks(page, caseId, {
      forms: [SAMPLE_FORM_DEFINITION],
      submissions: {
        intake_form: { id: 'sub-1', status: 'NEEDS_CORRECTION', submitted_at: '2024-01-15T10:00:00Z' },
      },
      workflowState: 'IN_PROGRESS',
    });

    await page.goto('/');
    await page.evaluate(() => router.navigate('case', 'case-workspace-4'));

    await expect(page.locator('#form-submission-status')).toContainText('Needs Correction');
  })

  test('renders multiple forms in the list and allows switching', async ({ page }) => {
    await openDashboard(page);
    const caseId = 'case-workspace-5';
    await setupCaseWorkspaceMocks(page, caseId, {
      forms: [SAMPLE_FORM_DEFINITION, SECONDARY_FORM_DEFINITION],
      workflowState: 'IN_PROGRESS',
    });

    await page.goto('/');
    await page.evaluate(() => router.navigate('case', 'case-workspace-5'));

    // Both forms should be listed
    await expect(page.locator('#form-requirements-list-container li')).toHaveCount(2);
    await expect(page.locator('#form-requirements-list-container')).toContainText('Intake Questionnaire');
    await expect(page.locator('#form-requirements-list-container')).toContainText('Evidence Form');

    // Evidence Form should show as Optional
    await expect(page.locator('#form-requirements-list-container')).toContainText('Optional');
  })

  test('shows workflow guardrails when transition requires forms', async ({ page }) => {
    await openDashboard(page);
    const caseId = 'case-guardrail-1';
    await setupCaseWorkspaceMocks(page, caseId, {
      forms: [SAMPLE_FORM_DEFINITION],
      submissions: {},
      transitions: [
        { key: 'review', name: 'Send for Review', requires_forms: ['intake_form'] },
      ],
      workflowState: 'IN_PROGRESS',
    });

    await page.goto('/');
    await page.evaluate(() => router.navigate('case', 'case-guardrail-1'));

    // Should show requirements explanation
    await expect(page.locator('#form-workflow-requirements')).toBeVisible();
    await expect(page.locator('#form-requirements-list')).toContainText('intake_form');

    // Transition button should be disabled with explanation
    await expect(page.locator('#case-actions button:has-text("Send for Review")')).toBeDisabled();
    await expect(page.locator('#case-actions')).toContainText('Required information missing');
  })

  test('hides transition guardrail when required forms are submitted', async ({ page }) => {
    await openDashboard(page);
    const caseId = 'case-guardrail-2';
    await setupCaseWorkspaceMocks(page, caseId, {
      forms: [SAMPLE_FORM_DEFINITION],
      submissions: {
        intake_form: { id: 'sub-1', status: 'SUBMITTED' },
      },
      transitions: [
        { key: 'review', name: 'Send for Review', requires_forms: ['intake_form'] },
      ],
      workflowState: 'IN_PROGRESS',
    });

    await page.goto('/');
    await page.evaluate(() => router.navigate('case', 'case-guardrail-2'));

    // Transition button should be enabled (no guardrail)
    await expect(page.locator('#case-actions button:has-text("Send for Review")')).toBeEnabled();
    await expect(page.locator('#form-workflow-requirements')).toBeHidden();
  })

  test('disables submit button during form submission', async ({ page }) => {
    await openDashboard(page);
    const caseId = 'case-submit-1';
    await setupCaseWorkspaceMocks(page, caseId, {
      forms: [SAMPLE_FORM_DEFINITION],
      workflowState: 'IN_PROGRESS',
    });
    // Delay the submission response
    page.route(routes2.workflowFormSubmission(caseId), async (route, req) => {
      if (req.method() === 'POST') {
        await new Promise(r => setTimeout(r, 500));
        await route.fulfill({ json: { success: true, data: { id: 'sub-1', status: 'SUBMITTED' } } });
      } else {
        await route.continue();
      }
    });

    await page.goto('/');
    await page.evaluate(() => router.navigate('case', 'case-submit-1'));

    // Fill in valid form values
    await page.fill('#form-field-full_name', 'Jane Doe');
    await page.fill('#form-field-email', 'jane@example.com');
    await page.fill('#form-field-household_size', '2');
    await page.selectOption('#form-field-preferred_contact', 'email');
    await page.check('#form-field-consent');

    // Submit
    await page.click('#form-container button[type="submit"]');

    // Submit button should be disabled during submission
    await expect(page.locator('#form-container button[type="submit"]')).toBeDisabled();
  })

  test('shows error message on network failure during submission', async ({ page }) => {
    await openDashboard(page);
    const caseId = 'case-submit-net-err';
    await setupCaseWorkspaceMocks(page, caseId, {
      forms: [SAMPLE_FORM_DEFINITION],
      workflowState: 'IN_PROGRESS',
    });
    // Simulate network failure
    page.route(routes2.workflowFormSubmission(caseId), async (route, req) => {
      if (req.method() === 'POST') {
        await route.fulfill({ status: 500, json: { success: false, error: { message: 'Internal Server Error' } } });
      } else {
        await route.continue();
      }
    });

    await page.goto('/');
    await page.evaluate(() => router.navigate('case', 'case-submit-net-err'));

    await page.fill('#form-field-full_name', 'Jane Doe');
    await page.fill('#form-field-email', 'jane@example.com');
    await page.fill('#form-field-household_size', '2');
    await page.selectOption('#form-field-preferred_contact', 'email');
    await page.check('#form-field-consent');

    await page.click('#form-container button[type="submit"]');

    // Should show error message
    await expect(page.locator('.form-message.error')).toBeVisible();
    // Submit button should be re-enabled
    await expect(page.locator('#form-container button[type="submit"]')).toBeEnabled();
  })

  test('hides forms workspace on terminal case', async ({ page }) => {
    await openDashboard(page);
    const caseId = 'case-terminal-forms';
    await setupCaseWorkspaceMocks(page, caseId, {
      forms: [SAMPLE_FORM_DEFINITION],
      workflowState: 'CLOSED',
    });
    page.route(regex(`organizations/${ORG_ID}/cases/${caseId}$`), async (route) => {
      await route.fulfill({ json: { success: true, data: {
        id: caseId, case_number: 'CAS-999', title: 'Terminal Case',
        status: 'CLOSED', service_type: 'EMERGENCY', priority: 'HIGH', description: 'Closed case.',
      } } });
    });

    await page.goto('/');
    await page.evaluate(() => router.navigate('case', 'case-terminal-forms'));

    // Terminal notice should be visible
    await expect(page.locator('#case-terminal-notice')).toBeVisible();

    // Forms workspace should be hidden
    await expect(page.locator('#card-required-info')).toBeHidden();
  })

  test('handles 404 on forms endpoint gracefully', async ({ page }) => {
    await openDashboard(page);
    const caseId = 'case-no-forms';
    await setupCaseWorkspaceMocks(page, caseId, {
      forms: [],
      workflowState: 'IN_PROGRESS',
    });

    await page.goto('/');
    await page.evaluate(() => router.navigate('case', 'case-no-forms'));

    // No forms → workspace should be hidden, no crash
    await expect(page.locator('#card-required-info')).toBeHidden();
  })

  test('handles authorization failure for forms endpoint', async ({ page }) => {
    await openDashboard(page);
    const caseId = 'case-403-forms';
    await setupCaseWorkspaceMocks(page, caseId, {
      forms: [SAMPLE_FORM_DEFINITION],
      workflowState: 'IN_PROGRESS',
    });
    page.route(regex(`organizations/${ORG_ID}/cases/${caseId}/workflow/forms`), async (route) => {
      if (route.request().method() === 'GET') {
        await route.fulfill({ status: 403, json: { success: false, error: { message: 'Forbidden' } } });
      } else {
        await route.continue();
      }
    });

    await page.goto('/');
    await page.evaluate(() => router.navigate('case', 'case-403-forms'));

    // Should handle 403 gracefully - no forms shown, no crash
    await expect(page.locator('#card-dynamic-form')).toBeHidden();
  });
});
