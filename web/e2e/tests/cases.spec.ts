// Case Workflow Selection E2E Tests

import { test, expect } from '@playwright/test';
import {
  EMERGENCY_WORKFLOW,
  PERSON,
  openDashboard,
  attachCaseMocks,
  newCaseSubmit,
  routes,
} from './helpers';

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
