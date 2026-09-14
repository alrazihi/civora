// Case Rule Integration E2E Tests for Education Grant and Healthcare Support processes
//
// Tests workflow selection with rule-assigned processes via mocked API.
// Note: Case-level rule integration UI (evaluation badges, facts display)
// requires frontend implementation in app.js. These tests cover the
// workflow selection and case creation aspects that exist.

import { test, expect } from '@playwright/test';
import {
  ORG_ID,
  ADMIN_TOKEN,
  regex,
  openDashboard,
  attachCaseMocks,
  newCaseSubmit,
  routes,
} from './helpers';

const EDUCATION_GRANT_WORKFLOW = {
  id: 'wf-education-1',
  key: 'education_grant',
  name: 'Education Grant',
  version: 1,
  status: 'ACTIVE',
  initial_state: 'NEW',
  states: [
    { key: 'NEW', name: 'New', terminal: false },
    { key: 'REVIEWING', name: 'Reviewing', terminal: false },
    { key: 'APPROVED', name: 'Approved', terminal: true },
    { key: 'REJECTED', name: 'Rejected', terminal: true },
  ],
  transitions: [{ key: 'start', from_state: 'NEW', to_state: 'IN_PROGRESS' }],
  description: 'Education Grant application workflow.',
};

const HEALTHCARE_SUPPORT_WORKFLOW = {
  id: 'wf-healthcare-1',
  key: 'healthcare_support',
  name: 'Healthcare Support',
  version: 1,
  status: 'ACTIVE',
  initial_state: 'NEW',
  states: [
    { key: 'NEW', name: 'New', terminal: false },
    { key: 'IN_REVIEW', name: 'In Review', terminal: false },
    { key: 'ENROLLED', name: 'Enrolled', terminal: true },
    { key: 'DECLINED', name: 'Declined', terminal: true },
  ],
  transitions: [{ key: 'start', from_state: 'NEW', to_state: 'IN_PROGRESS' }],
  description: 'Healthcare Support application workflow.',
};

test.describe('Education Grant Process', () => {
  test.beforeEach(async ({ page }) => {
    await page.addInitScript(() => {
      localStorage.setItem('civora_token', 'dev-token');
      localStorage.setItem('civora_org_id', ORG_ID);
    });
  });

  test('shows Education Grant workflow in selector', async ({ page }) => {
    await openDashboard(page);
    page.route(routes.workflows, async (route) => {
      await route.fulfill({
        json: { success: true, data: [EDUCATION_GRANT_WORKFLOW] },
      });
    });
    await page.evaluate(() => router.navigate('new-case'));

    const selector = page.locator('#workflow-selector');
    await expect(selector).toBeVisible();
    await expect(selector.locator('.wf-option-card')).toHaveCount(1);
    await expect(selector.locator('.wf-option-card')).toHaveText(/Education Grant/);
    await expect(page.locator('#selected-workflow-id')).toHaveValue('wf-education-1');
  });

  test('creates Education Grant case and navigates to case view', async ({ page }) => {
    await openDashboard(page);
    page.route(routes.workflows, async (route) => {
      await route.fulfill({
        json: { success: true, data: [EDUCATION_GRANT_WORKFLOW] },
      });
    });
    page.route(routes.casesCreate, async (route, request) => {
      if (request.method() !== 'POST') { await route.continue(); return; }
      const body = JSON.parse(request.postData() || '{}');
      await route.fulfill({
        json: { success: true, data: { id: 'case-edu', title: body.title, status: 'NEW', service_type: body.service_type, priority: body.priority, workflow_id: body.workflow_id } },
      });
    });
    page.route(routes.caseWorkflow('case-edu'), async (route) => {
      await route.fulfill({
        json: {
          success: true,
          data: {
            instance: {
              id: 'inst-edu',
              workflow_definition_id: EDUCATION_GRANT_WORKFLOW.id,
              current_state: 'NEW',
              started_at: '2026-09-14T01:00:00Z',
            },
            definition: EDUCATION_GRANT_WORKFLOW,
          },
        },
      });
    });
    page.route(regex(`organizations/${ORG_ID}/cases/case-edu$`), async (route) => {
      await route.fulfill({ json: { success: true, data: { id: 'case-edu', case_number: 'CAS-201', title: 'Education Grant Application', status: 'NEW', service_type: 'EDUCATION', priority: 'NORMAL', description: 'Test.' } } });
    });

    await page.evaluate(() => router.navigate('new-case'));
    await page.fill('#c-title', 'Education Grant Application');
    await page.evaluate((v) => {
      (document.getElementById('new-case-person-id') as HTMLInputElement).value = v;
    }, 'person-1');
    await newCaseSubmit(page).click();

    await expect(page).toHaveURL(/#case\/case-edu/);
    await expect(page.locator('#view-case')).toBeVisible();
    await expect(page.locator('#workflow-instance-state')).toContainText('NEW');
  });
});

test.describe('Healthcare Support Process', () => {
  test.beforeEach(async ({ page }) => {
    await page.addInitScript(() => {
      localStorage.setItem('civora_token', 'dev-token');
      localStorage.setItem('civora_org_id', ORG_ID);
    });
  });

  test('shows Healthcare Support workflow in selector', async ({ page }) => {
    await openDashboard(page);
    page.route(routes.workflows, async (route) => {
      await route.fulfill({
        json: { success: true, data: [HEALTHCARE_SUPPORT_WORKFLOW] },
      });
    });
    await page.evaluate(() => router.navigate('new-case'));

    const selector = page.locator('#workflow-selector');
    await expect(selector).toBeVisible();
    await expect(selector.locator('.wf-option-card')).toHaveCount(1);
    await expect(selector.locator('.wf-option-card')).toHaveText(/Healthcare Support/);
    await expect(page.locator('#selected-workflow-id')).toHaveValue('wf-healthcare-1');
  });

  test('creates Healthcare Support case and navigates to case view', async ({ page }) => {
    await openDashboard(page);
    page.route(routes.workflows, async (route) => {
      await route.fulfill({
        json: { success: true, data: [HEALTHCARE_SUPPORT_WORKFLOW] },
      });
    });
    page.route(routes.casesCreate, async (route, request) => {
      if (request.method() !== 'POST') { await route.continue(); return; }
      const body = JSON.parse(request.postData() || '{}');
      await route.fulfill({
        json: { success: true, data: { id: 'case-hc', title: body.title, status: 'NEW', service_type: body.service_type, priority: body.priority, workflow_id: body.workflow_id } },
      });
    });
    page.route(routes.caseWorkflow('case-hc'), async (route) => {
      await route.fulfill({
        json: {
          success: true,
          data: {
            instance: {
              id: 'inst-hc',
              workflow_definition_id: HEALTHCARE_SUPPORT_WORKFLOW.id,
              current_state: 'NEW',
              started_at: '2026-09-14T01:00:00Z',
            },
            definition: HEALTHCARE_SUPPORT_WORKFLOW,
          },
        },
      });
    });
    page.route(regex(`organizations/${ORG_ID}/cases/case-hc$`), async (route) => {
      await route.fulfill({ json: { success: true, data: { id: 'case-hc', case_number: 'CAS-202', title: 'Healthcare Support Application', status: 'NEW', service_type: 'HEALTHCARE', priority: 'NORMAL', description: 'Test.' } } });
    });

    await page.evaluate(() => router.navigate('new-case'));
    await page.fill('#c-title', 'Healthcare Support Application');
    await page.evaluate((v) => {
      (document.getElementById('new-case-person-id') as HTMLInputElement).value = v;
    }, 'person-1');
    await newCaseSubmit(page).click();

    await expect(page).toHaveURL(/#case\/case-hc/);
    await expect(page.locator('#view-case')).toBeVisible();
    await expect(page.locator('#workflow-instance-state')).toContainText('NEW');
  });
});
