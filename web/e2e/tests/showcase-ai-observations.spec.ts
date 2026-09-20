// Showcase for ai-observations.spec.ts — AI observations on verified evidence
import { test, expect, type Page } from '@playwright/test';
import { openDashboard, setupCaseWorkspaceMocks, regex, ORG_ID, ADMIN_TOKEN } from './helpers';

const CASE_ID = 'case-ai-showcase';
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

test.describe('Showcase: AI Observations', () => {
  test('renders verified evidence and AI observation button', async ({ page }) => {
    await page.setViewportSize({ width: 1400, height: 900 });

    await page.addInitScript(() => {
      localStorage.setItem('civora_token', ADMIN_TOKEN);
      localStorage.setItem('civora_org_id', ORG_ID);
    });

    await openDashboard(page);
    await setupCaseWorkspaceMocks(page, CASE_ID, {
      workflowState: 'IN_PROGRESS',
    });

    page.route(regex(`organizations/${ORG_ID}/evidence/by-service-request/${CASE_ID}`), async (route) => {
      await route.fulfill({
        json: { success: true, data: VERIFIED_EVIDENCE, meta: { page: 1, per_page: 20, total: 1, total_pages: 1 } },
      });
    });

    page.route(regex(`organizations/${ORG_ID}/evidence/${EVIDENCE_ID}/documents`), async (route, request) => {
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
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(800);
    await page.screenshot({ path: 'test-results/showcase-ai-observations/01-verified-evidence.png', fullPage: true });

    await expect(page.locator('#view-case')).toBeVisible();
    await expect(page.locator('#sec-evidence')).toContainText('VERIFIED');
    await expect(page.locator('#sec-evidence button:has-text("AI")')).toBeVisible();
  });
});
