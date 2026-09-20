// Showcase for forms.spec.ts — Dynamic form renderer
import { test, expect, type Page } from '@playwright/test';
import { openDashboard, setupCaseWorkspaceMocks, routes3, regex, ORG_ID } from './helpers';

const CASE_ID = 'case-forms-showcase';

test.describe('Showcase: Dynamic Form Renderer', () => {
  test('renders dynamic form fields and validation states', async ({ page }) => {
    await page.setViewportSize({ width: 1400, height: 900 });

    await page.addInitScript(() => {
      localStorage.setItem('civora_token', 'dev-token');
      localStorage.setItem('civora_org_id', 'org-test');
    });

    setupCaseWorkspaceMocks(page, CASE_ID, {
      workflowState: 'IN_PROGRESS',
      forms: [
        {
          id: 'form-sample-1',
          key: 'intake_form',
          name: 'Intake Questionnaire',
          description: 'Please provide the following information to process your request.',
          version: 1,
          status: 'ACTIVE',
          fields: [
            { key: 'full_name', label: 'Full Name', type: 'text', required: true, placeholder: 'Jane Doe', validation: { minLength: 2, maxLength: 100 } },
            { key: 'email', label: 'Email Address', type: 'email', required: true },
            { key: 'household_size', label: 'Household Size', type: 'number', required: true, validation: { minValue: 1, maxValue: 50 } },
            { key: 'income', label: 'Annual Income', type: 'decimal', required: false, validation: { minValue: 0 } },
            { key: 'preferred_contact', label: 'Preferred Contact Method', type: 'select', required: true, options: [
              { value: 'email', label: 'Email' }, { value: 'phone', label: 'Phone' }, { value: 'text', label: 'Text Message' },
            ]},
            { key: 'consent', label: 'I agree to the terms', type: 'boolean', required: true },
            { key: 'notes', label: 'Additional Notes', type: 'textarea', required: false },
          ],
        },
      ],
    });

    page.route(regex(`organizations/${ORG_ID}/cases/${CASE_ID}$`), async (route) => {
      await route.fulfill({ json: { success: true, data: {
        id: CASE_ID, case_number: 'CAS-100', title: 'Workspace Test',
        status: 'IN_PROGRESS', service_type: 'EMERGENCY', priority: 'HIGH', description: 'Test case.',
      } } });
    });

    await page.goto('/');
    await page.waitForLoadState('networkidle');
    await page.evaluate((id) => router.navigate('case', id), CASE_ID);
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(800);
    await page.screenshot({ path: 'test-results/showcase-forms/01-dynamic-form.png', fullPage: true });

    await expect(page.locator('#card-dynamic-form')).toBeVisible();
    await expect(page.locator('#dynamic-form-title')).toHaveText(/Intake Questionnaire/);
    await expect(page.locator('#form-container [data-field-key="full_name"]')).toBeVisible();
    await expect(page.locator('#form-container [data-field-key="preferred_contact"]')).toBeVisible();
    await expect(page.locator('#form-container [data-field-key="consent"]')).toBeVisible();
  });
});
