// Showcase for workspace.spec.ts — Case workspace integration
import { test, expect, type Page } from '@playwright/test';
import { openDashboard, setupCaseWorkspaceMocks, routes3, regex, ORG_ID } from './helpers';

const CASE_ID = 'case-workspace-showcase';

test.describe('Showcase: Case Workspace Integration', () => {
  test('shows forms, workflow state, actions, and terminal notice', async ({ page }) => {
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
          required: true,
          fields: [
            { key: 'full_name', label: 'Full Name', type: 'text', required: true },
            { key: 'email', label: 'Email Address', type: 'email', required: true },
          ],
        },
        {
          id: 'form-sample-2',
          key: 'evidence_form',
          name: 'Evidence Form',
          description: 'Supporting documentation required.',
          version: 1,
          status: 'ACTIVE',
          required: false,
          fields: [
            { key: 'evidence_type', label: 'Evidence Type', type: 'select', required: true, options: [
              { value: 'id', label: 'ID Document' }, { value: 'paycheck', label: 'Pay Stub' },
            ]},
          ],
        },
      ],
      submissions: {
        intake_form: { id: 'sub-1', status: 'SUBMITTED', submitted_at: '2026-09-12T01:00:00Z' },
      },
      transitions: [
        { key: 'start', from_state: 'NEW', to_state: 'IN_PROGRESS', label: 'Start' },
      ],
    });

    page.route(regex(`organizations/${ORG_ID}/cases/${CASE_ID}$`), async (route) => {
      await route.fulfill({ json: { success: true, data: {
        id: CASE_ID, case_number: 'CAS-200', title: 'Workspace Integration Test',
        status: 'IN_PROGRESS', service_type: 'EMERGENCY', priority: 'HIGH', description: 'Test case.',
      } } });
    });

    await page.goto('/');
    await page.waitForLoadState('networkidle');
    await page.evaluate((id) => router.navigate('case', id), CASE_ID);
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(800);
    await page.screenshot({ path: 'test-results/showcase-workspace/01-workspace.png', fullPage: true });

    await expect(page.locator('#workflow-instance-state')).toBeVisible();
    await expect(page.locator('#card-dynamic-form')).toBeVisible();
    await expect(page.locator('#case-actions button').first()).toBeVisible();
    await expect(page.locator('#form-submission-status')).toHaveText(/Status: Submitted/);
  });
});
