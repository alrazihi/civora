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
