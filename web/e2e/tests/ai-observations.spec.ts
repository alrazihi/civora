// AI Observations E2E Tests

import { test, expect } from '@playwright/test';
import { ORG_ID, openDashboard, setupCaseWorkspaceMocks, ADMIN_TOKEN } from './helpers';

const CASE_ID = 'case-ai-test-1';
const EVIDENCE_ID = 'ev-ai-1';

const VERIFIED_EVIDENCE = [
  {
    id: EVIDENCE_ID,
    case_id: CASE_ID,
    service_request_id: CASE_ID,
    type: 'IDENTITY_DOCUMENT',
    description: 'Passport',
    verification_status: 'VERIFIED',
  },
];

const OBSERVATIONS_DATA = [
  {
    id: 'obs-1',
    organization_id: ORG_ID,
    evidence_id: EVIDENCE_ID,
    type: 'SUMMARY',
    source: 'AI_MODEL',
    status: 'PENDING_REVIEW',
    model: { name: 'llama3', version: '1.0', provider: 'local' },
    content: { text: 'This document appears to be a passport with name Jane Doe.' },
    confidence: 0.92,
    input_hash: 'abc123',
    output_hash: 'def456',
    created_at: '2026-09-16T10:00:00Z',
    reviewed_at: null,
    reviewed_by: null,
    review_notes: '',
  },
  {
    id: 'obs-2',
    organization_id: ORG_ID,
    evidence_id: EVIDENCE_ID,
    type: 'ENTITY_EXTRACTION',
    source: 'AI_MODEL',
    status: 'ACCEPTED',
    model: { name: 'llama3', version: '1.0', provider: 'local' },
    content: { entities: [{ type: 'PERSON', value: 'Jane Doe' }] },
    confidence: 0.88,
    input_hash: 'abc124',
    output_hash: 'def457',
    created_at: '2026-09-16T09:00:00Z',
    reviewed_at: '2026-09-16T09:05:00Z',
    reviewed_by: 'user-1',
    review_notes: 'Verified entity extraction',
  },
];

test.describe('AI Observations', () => {
  test.beforeEach(async ({ page }) => {
    await page.addInitScript(() => {
      localStorage.setItem('civora_token', ADMIN_TOKEN);
      localStorage.setItem('civora_org_id', ORG_ID);
    });
  });

  test('shows AI button for verified evidence', async ({ page }) => {
    await openDashboard(page);
    await setupCaseWorkspaceMocks(page, CASE_ID, {
      workflowState: 'IN_PROGRESS',
    });

    page.route(/.*\/organizations\/.*\/evidence\/by-service-request\/.*/, async (route) => {
      await route.fulfill({
        json: { success: true, data: VERIFIED_EVIDENCE, meta: { page: 1, per_page: 20, total: 1, total_pages: 1 } },
      });
    });
    page.route(/.*\/organizations\/.*\/evidence\/.*\/documents/, async (route, request) => {
      if (request.method() === 'GET') {
        await route.fulfill({
          json: { success: true, data: [], meta: { page: 1, per_page: 50, total: 0, total_pages: 0 } },
        });
      } else {
        await route.continue();
      }
    });

    await page.goto('/');
    await page.evaluate((cid) => router.navigate('case', cid), CASE_ID);

    await expect(page.locator('#sec-evidence')).toContainText('VERIFIED');
    await expect(page.locator('#sec-evidence button:has-text("AI")')).toBeVisible();
  });

  test('opens AI observations modal and displays observations', async ({ page }) => {
    await openDashboard(page);
    await setupCaseWorkspaceMocks(page, CASE_ID, {
      workflowState: 'IN_PROGRESS',
    });

    page.route(/.*\/organizations\/.*\/evidence\/by-service-request\/.*/, async (route) => {
      await route.fulfill({
        json: { success: true, data: VERIFIED_EVIDENCE, meta: { page: 1, per_page: 20, total: 1, total_pages: 1 } },
      });
    });
    page.route(/.*\/organizations\/.*\/evidence\/.*\/documents/, async (route, request) => {
      if (request.method() === 'GET') {
        await route.fulfill({
          json: { success: true, data: [], meta: { page: 1, per_page: 50, total: 0, total_pages: 0 } },
        });
      } else {
        await route.continue();
      }
    });
    page.route(/.*\/evidence\/.*\/ai\/observations/, async (route, request) => {
      if (request.method() === 'GET') {
        await route.fulfill({
          json: { success: true, data: OBSERVATIONS_DATA, meta: { page: 1, per_page: 50, total: 2, total_pages: 1 } },
        });
      } else if (request.method() === 'POST') {
        await route.fulfill({
          json: {
            success: true,
            data: {
              observations: [
                {
                  id: 'obs-3',
                  type: 'SUMMARY',
                  content: { text: 'Generated summary.' },
                  confidence: 0.85,
                  model: { name: 'llama3', version: '1.0', provider: 'local' },
                  created_at: '2026-09-16T12:00:00Z',
                },
              ],
            },
          },
        });
      } else {
        await route.continue();
      }
    });

    await page.goto('/');
    await page.evaluate((cid) => router.navigate('case', cid), CASE_ID);

    await expect(page.locator('#sec-evidence button:has-text("AI")')).toBeVisible();
    await page.click('#sec-evidence button:has-text("AI")');

    await expect(page.locator('#ai-observations-modal')).toBeVisible();
    await expect(page.locator('#ai-observations-body')).toContainText('SUMMARY');
    await expect(page.locator('#ai-observations-body')).toContainText('ENTITY EXTRACTION');
    await expect(page.locator('#ai-observations-body')).toContainText('Jane Doe');
  });

  test('generate observations button creates new observations', async ({ page }) => {
    await openDashboard(page);
    await setupCaseWorkspaceMocks(page, CASE_ID, {
      workflowState: 'IN_PROGRESS',
    });

    let generateCount = 0;

    page.route(/.*\/organizations\/.*\/evidence\/by-service-request\/.*/, async (route) => {
      await route.fulfill({
        json: { success: true, data: VERIFIED_EVIDENCE, meta: { page: 1, per_page: 20, total: 1, total_pages: 1 } },
      });
    });
    page.route(/.*\/organizations\/.*\/evidence\/.*\/documents/, async (route, request) => {
      if (request.method() === 'GET') {
        await route.fulfill({
          json: { success: true, data: [], meta: { page: 1, per_page: 50, total: 0, total_pages: 0 } },
        });
      } else {
        await route.continue();
      }
    });
    page.route(/.*\/evidence\/.*\/ai\/observations($|\?)/, async (route, request) => {
      if (request.method() === 'GET') {
        await route.fulfill({
          json: { success: true, data: [], meta: { page: 1, per_page: 50, total: 0, total_pages: 0 } },
        });
      } else {
        await route.continue();
      }
    });
    page.route(/.*\/evidence\/.*\/ai\/observations\/generate/, async (route, request) => {
      generateCount++;
      await route.fulfill({
        json: {
          success: true,
          data: {
            observations: [
              {
                id: 'obs-new',
                type: 'SUMMARY',
                content: { text: 'Generated summary of the evidence document.' },
                confidence: 0.9,
                model: { name: 'llama3', version: '1.0', provider: 'local' },
                created_at: '2026-09-16T12:00:00Z',
              },
            ],
          },
        },
      });
    });

    await page.goto('/');
    await page.evaluate((cid) => router.navigate('case', cid), CASE_ID);

    await page.click('#sec-evidence button:has-text("AI")');
    await expect(page.locator('#ai-observations-modal')).toBeVisible();

    await page.click('#ai-generate-btn');

    expect(generateCount).toBeGreaterThanOrEqual(1);
    await expect(page.locator('#ai-observations-body')).toContainText('Generated summary');
  });

  test('accept observation triggers accept API and refreshes list', async ({ page }) => {
    await openDashboard(page);
    await setupCaseWorkspaceMocks(page, CASE_ID, {
      workflowState: 'IN_PROGRESS',
    });

    let acceptCount = 0;

    const pendingObs = [{
      id: 'obs-pending',
      organization_id: ORG_ID,
      evidence_id: EVIDENCE_ID,
      type: 'SUMMARY',
      source: 'AI_MODEL',
      status: 'PENDING_REVIEW',
      model: { name: 'llama3', version: '1.0', provider: 'local' },
      content: { text: 'Summary observation.' },
      confidence: 0.8,
      input_hash: 'abc',
      output_hash: 'def',
      created_at: '2026-09-16T10:00:00Z',
      reviewed_at: null,
      reviewed_by: null,
      review_notes: '',
    }];

    page.route(/.*\/organizations\/.*\/evidence\/by-service-request\/.*/, async (route) => {
      await route.fulfill({
        json: { success: true, data: VERIFIED_EVIDENCE, meta: { page: 1, per_page: 20, total: 1, total_pages: 1 } },
      });
    });
    page.route(/.*\/organizations\/.*\/evidence\/.*\/documents/, async (route, request) => {
      if (request.method() === 'GET') {
        await route.fulfill({
          json: { success: true, data: [], meta: { page: 1, per_page: 50, total: 0, total_pages: 0 } },
        });
      } else {
        await route.continue();
      }
    });
    page.route(/.*\/evidence\/.*\/ai\/observations($|\?)/, async (route, request) => {
      if (request.method() === 'GET') {
        await route.fulfill({
          json: { success: true, data: pendingObs, meta: { page: 1, per_page: 50, total: 1, total_pages: 1 } },
        });
      } else {
        await route.continue();
      }
    });
    page.route(/.*\/evidence\/.*\/ai\/observations\/.*\/accept/, async (route) => {
      acceptCount++;
      const accepted = { ...pendingObs[0], status: 'ACCEPTED', reviewed_at: '2026-09-16T10:05:00Z', reviewed_by: 'user-1', review_notes: 'looks good' };
      await route.fulfill({ json: { success: true, data: accepted } });
    });

    await page.goto('/');
    await page.evaluate((cid) => router.navigate('case', cid), CASE_ID);

    await page.click('#sec-evidence button:has-text("AI")');
    await expect(page.locator('#ai-observations-modal')).toBeVisible();

    await expect(page.locator('#ai-observations-body button:has-text("Accept")')).toBeVisible();
    page.on('dialog', async (dialog) => {
      await dialog.accept('looks good');
    });
    await page.click('#ai-observations-body button:has-text("Accept")');

    expect(acceptCount).toBe(1);
  });

  test('shows error message when AI provider is not configured', async ({ page }) => {
    await openDashboard(page);
    await setupCaseWorkspaceMocks(page, CASE_ID, {
      workflowState: 'IN_PROGRESS',
    });

    page.route(/.*\/organizations\/.*\/evidence\/by-service-request\/.*/, async (route) => {
      await route.fulfill({
        json: { success: true, data: VERIFIED_EVIDENCE, meta: { page: 1, per_page: 20, total: 1, total_pages: 1 } },
      });
    });
    page.route(/.*\/organizations\/.*\/evidence\/.*\/documents/, async (route, request) => {
      if (request.method() === 'GET') {
        await route.fulfill({
          json: { success: true, data: [], meta: { page: 1, per_page: 50, total: 0, total_pages: 0 } },
        });
      } else {
        await route.continue();
      }
    });
    page.route(/.*\/evidence\/.*\/ai\/observations($|\?)/, async (route) => {
      await route.fulfill({ status: 503, json: { success: false, error: { message: 'AI provider unavailable' } } });
    });

    await page.goto('/');
    await page.evaluate((cid) => router.navigate('case', cid), CASE_ID);

    await page.click('#sec-evidence button:has-text("AI")');
    await expect(page.locator('#ai-observations-modal')).toBeVisible();

    await expect(page.locator('#ai-observations-body')).toContainText('not configured');
  });
});
