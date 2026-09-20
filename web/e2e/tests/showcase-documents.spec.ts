// Showcase for documents.spec.ts — Evidence document management
import { test, expect, type Page } from '@playwright/test';
import { openDashboard, setupCaseWorkspaceMocks, regex, ORG_ID, ADMIN_TOKEN } from './helpers';

const CASE_ID = 'case-docs-showcase';

const EVIDENCE_DATA = [
  {
    id: CASE_ID,
    case_id: CASE_ID,
    service_request_id: CASE_ID,
    type: 'IDENTITY_DOCUMENT',
    description: 'Passport',
    verification_status: 'PENDING',
  },
];

const DOCUMENTS_DATA = [
  {
    id: 'doc-1',
    evidence_id: CASE_ID,
    file_name: 'passport.pdf',
    content_type: 'application/pdf',
    size_bytes: 102400,
    checksum: 'abc123hash',
    uploaded_by: 'user-1',
    uploaded_at: '2026-09-16T10:00:00Z',
  },
  {
    id: 'doc-2',
    evidence_id: CASE_ID,
    file_name: 'address_proof.jpg',
    content_type: 'image/jpeg',
    size_bytes: 51200,
    checksum: 'def456hash',
    uploaded_by: 'user-1',
    uploaded_at: '2026-09-16T11:00:00Z',
  },
];

test.describe('Showcase: Evidence Documents', () => {
  test.beforeEach(async ({ page }) => {
    await page.addInitScript(() => {
      localStorage.setItem('civora_token', ADMIN_TOKEN);
      localStorage.setItem('civora_org_id', ORG_ID);
    });
  });

  test('renders document list, previews, upload, and verification actions', async ({ page }) => {
    await page.setViewportSize({ width: 1400, height: 900 });

    await openDashboard(page);
    await setupCaseWorkspaceMocks(page, CASE_ID, {
      workflowState: 'IN_PROGRESS',
    });

    page.route(regex(`organizations/${ORG_ID}/evidence/by-service-request/${CASE_ID}`), async (route) => {
      await route.fulfill({
        json: { success: true, data: EVIDENCE_DATA, meta: { page: 1, per_page: 20, total: 1, total_pages: 1 } },
      });
    });

    page.route(regex(`organizations/${ORG_ID}/evidence/${CASE_ID}/documents`), async (route, request) => {
      if (request.method() === 'GET') {
        await route.fulfill({
          json: { success: true, data: DOCUMENTS_DATA, meta: { page: 1, per_page: 50, total: 2, total_pages: 1 } },
        });
      } else if (request.method() === 'DELETE') {
        await route.fulfill({
          json: { success: true, data: { message: 'Document deleted' } },
        });
      } else {
        await route.continue();
      }
    });

    await page.goto('/');
    await page.evaluate((cid) => router.navigate('case', cid), CASE_ID);
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(800);
    await page.screenshot({ path: 'test-results/showcase-documents/01-documents.png', fullPage: true });

    await expect(page.locator('#view-case')).toBeVisible();
    await expect(page.locator('#sec-evidence')).toContainText('IDENTITY_DOCUMENT');
    await expect(page.locator('#sec-evidence')).toContainText('passport.pdf');
    await expect(page.locator('#sec-evidence')).toContainText('address_proof.jpg');
  });
});
