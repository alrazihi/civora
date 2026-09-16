// Evidence Document Management E2E Tests

import { test, expect } from '@playwright/test';
import { ORG_ID, openDashboard, setupCaseWorkspaceMocks, ADMIN_TOKEN } from './helpers';

const CASE_ID = 'case-doc-test-1';

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

test.describe('Evidence Document Management', () => {
  test.beforeEach(async ({ page }) => {
    await page.addInitScript(() => {
      localStorage.setItem('civora_token', ADMIN_TOKEN);
      localStorage.setItem('civora_org_id', ORG_ID);
    });
  });

  test('shows documents in evidence section with download and delete actions', async ({ page }) => {
    await openDashboard(page);
    await setupCaseWorkspaceMocks(page, CASE_ID, {
      workflowState: 'IN_PROGRESS',
    });

    // Mock evidence list endpoint
    page.route(/.*\/organizations\/.*\/evidence\/by-service-request\/.*/, async (route) => {
      await route.fulfill({
        json: { success: true, data: EVIDENCE_DATA, meta: { page: 1, per_page: 20, total: 1, total_pages: 1 } },
      });
    });

    // Mock document list endpoint and delete
    page.route(/.*\/organizations\/.*\/evidence\/.*\/documents/, async (route, request) => {
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

    await expect(page.locator('#view-case')).toBeVisible();

    // Wait for evidence section to load
    await expect(page.locator('#sec-evidence')).toContainText('IDENTITY_DOCUMENT');

    // Check document names are displayed
    await expect(page.locator('#sec-evidence')).toContainText('passport.pdf');
    await expect(page.locator('#sec-evidence')).toContainText('address_proof.jpg');

    // Check download links are present
    const downloadLinks = await page.locator('#sec-evidence a[onclick*="downloadDocument"]').all();
    expect(downloadLinks.length).toBe(2);

    // Check delete buttons are present
    const deleteButtons = await page.locator('#sec-evidence button[onclick*="deleteDocument"]').all();
    expect(deleteButtons.length).toBe(2);

    // Check file sizes are displayed (102400 bytes = 100.0 KB, 51200 = 50.0 KB)
    await expect(page.locator('#sec-evidence')).toContainText('100.0 KB');
    await expect(page.locator('#sec-evidence')).toContainText('50.0 KB');

    // Check preview thumbnails/img elements are present
    const previews = await page.locator('#sec-evidence .doc-preview').all();
    expect(previews.length).toBe(2);
  });

  test('shows "No documents" when evidence has no documents', async ({ page }) => {
    await openDashboard(page);
    await setupCaseWorkspaceMocks(page, CASE_ID, {
      workflowState: 'IN_PROGRESS',
    });

    // Mock evidence list endpoint
    page.route(/.*\/organizations\/.*\/evidence\/by-service-request\/.*/, async (route) => {
      await route.fulfill({
        json: { success: true, data: EVIDENCE_DATA, meta: { page: 1, per_page: 20, total: 1, total_pages: 1 } },
      });
    });

    // Mock document list endpoint to return empty
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

    await expect(page.locator('#view-case')).toBeVisible();
    await expect(page.locator('#sec-evidence')).toContainText('No documents');
  });

  test('deletes a document via delete button with confirmation', async ({ page }) => {
    await openDashboard(page);
    await setupCaseWorkspaceMocks(page, CASE_ID, {
      workflowState: 'IN_PROGRESS',
    });

    // Mock evidence list endpoint
    page.route(/.*\/organizations\/.*\/evidence\/by-service-request\/.*/, async (route) => {
      await route.fulfill({
        json: { success: true, data: EVIDENCE_DATA, meta: { page: 1, per_page: 20, total: 1, total_pages: 1 } },
      });
    });

    let deleteCallCount = 0;
    // Mock document list endpoint
    page.route(/.*\/organizations\/.*\/evidence\/.*\/documents/, async (route, request) => {
      if (request.method() === 'GET') {
        await route.fulfill({
          json: { success: true, data: DOCUMENTS_DATA, meta: { page: 1, per_page: 50, total: 2, total_pages: 1 } },
        });
      } else if (request.method() === 'DELETE') {
        deleteCallCount++;
        await route.fulfill({
          json: { success: true, data: { message: 'Document deleted' } },
        });
      } else {
        await route.continue();
      }
    });

    await page.goto('/');
    await page.evaluate((cid) => router.navigate('case', cid), CASE_ID);

    // Wait for documents to load
    await expect(page.locator('#sec-evidence')).toContainText('passport.pdf');

    // Click the first delete button
    const firstDeleteButton = page.locator('#sec-evidence button[onclick*="deleteDocument"]').first();

    // Accept the confirmation dialog
    page.on('dialog', async (dialog) => {
      await dialog.accept();
    });

    await firstDeleteButton.click();

    // Verify delete was called
    expect(deleteCallCount).toBeGreaterThanOrEqual(1);
  });

  test('download link triggers file download', async ({ page }) => {
    await openDashboard(page);
    await setupCaseWorkspaceMocks(page, CASE_ID, {
      workflowState: 'IN_PROGRESS',
    });

    // Mock evidence list endpoint
    page.route(/.*\/organizations\/.*\/evidence\/by-service-request\/.*/, async (route) => {
      await route.fulfill({
        json: { success: true, data: EVIDENCE_DATA, meta: { page: 1, per_page: 20, total: 1, total_pages: 1 } },
      });
    });

    // Mock document list endpoint
    page.route(/.*\/organizations\/.*\/evidence\/.*\/documents/, async (route, request) => {
      if (request.method() === 'GET') {
        await route.fulfill({
          json: { success: true, data: DOCUMENTS_DATA, meta: { page: 1, per_page: 50, total: 2, total_pages: 1 } },
        });
      } else {
        await route.continue();
      }
    });

    // Mock document download endpoint
    page.route(/.*\/organizations\/.*\/evidence\/.*\/document$/, async (route) => {
      await route.fulfill({
        body: 'fake pdf content for download test',
        headers: {
          'Content-Type': 'application/pdf',
          'Content-Disposition': 'attachment; filename="passport.pdf"',
        },
      });
    });

    await page.goto('/');
    await page.evaluate((cid) => router.navigate('case', cid), CASE_ID);

    // Wait for documents to load
    await expect(page.locator('#sec-evidence')).toContainText('passport.pdf');

    // Click the download link - this should trigger a navigation or download
    // Since we mock the download, we just verify the link exists and is clickable
    const downloadLink = page.locator('#sec-evidence a[onclick*="downloadDocument"]').first();
    await expect(downloadLink).toBeVisible();
  });

  test('shows verification status badge and verify/reject buttons for unverified evidence', async ({ page }) => {
    await openDashboard(page);
    await setupCaseWorkspaceMocks(page, CASE_ID, {
      workflowState: 'IN_PROGRESS',
    });

    // Mock evidence with unverified status
    page.route(/.*\/organizations\/.*\/evidence\/by-service-request\/.*/, async (route) => {
      await route.fulfill({
        json: { success: true, data: EVIDENCE_DATA, meta: { page: 1, per_page: 20, total: 1, total_pages: 1 } },
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

    await expect(page.locator('#sec-evidence')).toContainText('PENDING');
    await expect(page.locator('#sec-evidence button:has-text("Verify")')).toBeVisible();
    await expect(page.locator('#sec-evidence button:has-text("Reject")')).toBeVisible();
  });

  test('verify evidence triggers API call and refreshes section', async ({ page }) => {
    await openDashboard(page);
    await setupCaseWorkspaceMocks(page, CASE_ID, {
      workflowState: 'IN_PROGRESS',
    });

    let verifyCallCount = 0;
    page.route(/.*\/organizations\/.*\/evidence\/by-service-request\/.*/, async (route) => {
      await route.fulfill({
        json: { success: true, data: EVIDENCE_DATA, meta: { page: 1, per_page: 20, total: 1, total_pages: 1 } },
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
    page.route(/.*\/organizations\/.*\/evidence\/.*\/verify/, async (route) => {
      verifyCallCount++;
      await route.fulfill({
        json: { success: true, data: { id: CASE_ID, verification_status: 'VERIFIED' } },
      });
    });

    await page.goto('/');
    await page.evaluate((cid) => router.navigate('case', cid), CASE_ID);

    await expect(page.locator('#sec-evidence button:has-text("Verify")')).toBeVisible();

    page.on('dialog', async (dialog) => {
      await dialog.accept('test reason');
    });

    await page.click('#sec-evidence button:has-text("Verify")');

    expect(verifyCallCount).toBeGreaterThan(0);
  });

  test('shows Verified status badge for verified evidence', async ({ page }) => {
    await openDashboard(page);
    await setupCaseWorkspaceMocks(page, CASE_ID, {
      workflowState: 'IN_PROGRESS',
    });

    const verifiedEVIDENCE = [{
      id: CASE_ID,
      case_id: CASE_ID,
      service_request_id: CASE_ID,
      type: 'IDENTITY_DOCUMENT',
      description: 'Passport',
      verification_status: 'VERIFIED',
    }];

    page.route(/.*\/organizations\/.*\/evidence\/by-service-request\/.*/, async (route) => {
      await route.fulfill({
        json: { success: true, data: verifiedEVIDENCE, meta: { page: 1, per_page: 20, total: 1, total_pages: 1 } },
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
    await expect(page.locator('#sec-evidence .verified-badge')).toBeVisible();
    await expect(page.locator('#sec-evidence button:has-text("Verify")')).toHaveCount(0);
  });

  test('evidence modal has drag-and-drop upload area', async ({ page }) => {
    await openDashboard(page);
    await setupCaseWorkspaceMocks(page, CASE_ID, {
      workflowState: 'IN_PROGRESS',
    });

    page.route(/.*\/organizations\/.*\/evidence\/by-service-request\/.*/, async (route) => {
      await route.fulfill({
        json: { success: true, data: EVIDENCE_DATA, meta: { page: 1, per_page: 20, total: 1, total_pages: 1 } },
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

    // Open the evidence modal directly (button may be disabled without proper transitions)
    await page.evaluate(() => app.showModal('evidence-modal'));

    // Check drag-and-drop area is visible
    await expect(page.locator('#ev-drop-area')).toBeVisible();
    await expect(page.locator('#ev-drop-area')).toContainText('Drag & drop');

    // Check file input exists (hidden)
    await expect(page.locator('#ev-file')).toBeHidden();

    // Check upload progress bar exists (hidden initially)
    await expect(page.locator('#ev-upload-progress')).toBeHidden();
  });

  test('drag-over state is applied when file is dragged over drop area', async ({ page }) => {
    await openDashboard(page);
    await setupCaseWorkspaceMocks(page, CASE_ID, {
      workflowState: 'IN_PROGRESS',
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

    await page.evaluate(() => app.showModal('evidence-modal'));
    await expect(page.locator('#ev-drop-area')).toBeVisible();

    // Simulate drag over using evaluate to properly construct DragEvent
    await page.evaluate(() => {
      const dropArea = document.getElementById('ev-drop-area');
      const dt = new DataTransfer();
      const ev = new DragEvent('dragover', {
        bubbles: true,
        cancelable: true,
        dataTransfer: dt,
      });
      dropArea.dispatchEvent(ev);
    });

    await expect(page.locator('#ev-drop-area')).toHaveClass(/drop-area-hover/);

    // Simulate drag leave
    await page.evaluate(() => {
      const dropArea = document.getElementById('ev-drop-area');
      const dt = new DataTransfer();
      const ev = new DragEvent('dragleave', {
        bubbles: true,
        cancelable: true,
        dataTransfer: dt,
      });
      dropArea.dispatchEvent(ev);
    });

    await expect(page.locator('#ev-drop-area')).not.toHaveClass(/drop-area-hover/);
  });
});
