// Form Management E2E Tests

import { test, expect } from '@playwright/test';
import {
  ORG_ID,
  regex,
  SAMPLE_FORM_TEMPLATE,
  SAMPLE_WORKFLOWS,
  setupFormMockRoutes,
} from './helpers';

test.describe('Form Management', () => {
  test.beforeEach(async ({ page }) => {
    await page.addInitScript(() => {
      localStorage.setItem('civora_token', 'dev-token');
      localStorage.setItem('civora_org_id', 'org-test');
    });
    await page.goto('/');
    await expect(page).toHaveURL(/#dashboard/);
    await expect(page.locator('#view-dashboard')).toBeVisible();
  });

  test('shows form management view with empty state', async ({ page }) => {
    setupFormMockRoutes(page, { forms: { list: [] } });
    await page.evaluate(() => router.navigate('forms'));
    await expect(page.locator('#view-forms')).toBeVisible();
    await expect(page.locator('#form-table-body')).toContainText('No forms yet');
  });

  test('lists forms from the API', async ({ page }) => {
    setupFormMockRoutes(page, { forms: { list: [SAMPLE_FORM_TEMPLATE] } });
    await page.evaluate(() => router.navigate('forms'));
    await expect(page.locator('#form-table-body tr')).toHaveCount(1);
    await expect(page.locator('#form-table-body')).toContainText('Intake Questionnaire');
    await expect(page.locator('#form-table-body')).toContainText('intake_form');
  });

  test('navigates to form design view from create button', async ({ page }) => {
    setupFormMockRoutes(page, { forms: { list: [] } });
    await page.evaluate(() => router.navigate('forms'));
    await expect(page.locator('#btn-create-form')).toBeVisible();
    await page.click('#btn-create-form');
    await expect(page.locator('#view-form-design')).toBeVisible();
    await expect(page.locator('#form-design-title')).toHaveText('Create New Form');
  });

  test('creates a form with fields', async ({ page }) => {
    setupFormMockRoutes(page, { forms: { create: SAMPLE_FORM_TEMPLATE } });
    await page.evaluate(() => router.navigate('forms'));
    await page.click('#btn-create-form');

    // Fill form metadata
    await page.fill('#form-name', 'My New Form');
    await page.fill('#form-key', 'my_new_form');
    await page.fill('#form-description', 'A test form');

    // Add a field
    await page.click('button:has-text("Add Field")');
    await page.fill('#field-label', 'Email Address');
    await page.fill('#field-key', 'email');
    await page.selectOption('#field-type', 'email');
    await page.click('#field-required');

    await page.click('#form-field-modal button:has-text("Save")');
    await page.waitForSelector('#form-field-modal', { state: 'hidden' });

    // Verify the field was added
    await expect(page.locator('.form-field-card')).toHaveCount(1);
    await expect(page.locator('.form-field-card')).toContainText('Email Address');

    // Save draft
    await page.click('#btn-form-save');

    // Should show success toast
    await expect(page.locator('.toast.success')).toBeVisible();
  });

  test('detects duplicate field keys', async ({ page }) => {
    setupFormMockRoutes(page, { forms: { create: SAMPLE_FORM_TEMPLATE } });
    await page.evaluate(() => router.navigate('forms'));
    await page.click('#btn-create-form');

    // Add first field
    await page.click('button:has-text("Add Field")');
    await page.fill('#field-label', 'Email');
    await page.fill('#field-key', 'email');
    await page.selectOption('#field-type', 'email');
    await page.click('#form-field-modal button:has-text("Save")');
    await page.waitForSelector('#form-field-modal', { state: 'hidden' });

    // Add second field with same key
    await page.click('button:has-text("Add Field")');
    await page.fill('#field-label', 'Email 2');
    await page.fill('#field-key', 'email');
    await page.selectOption('#field-type', 'email');
    // Duplicate key should keep modal open with error
    await page.click('#form-field-modal button:has-text("Save")');
    // Modal stays open due to validation error
    await expect(page.locator('#form-field-modal')).toBeVisible();
    await expect(page.locator('#form-design-error')).toContainText('Duplicate field key');
    await page.click('#form-field-modal button:has-text("Cancel")');

    // Try to save form
    await page.fill('#form-name', 'Test');
    await page.fill('#form-key', 'test');
    await page.click('#btn-form-save');

    // Should show error about duplicate key
    await expect(page.locator('#form-design-error')).toContainText('Duplicate field key');
  });

  test('rejects invalid form key format', async ({ page }) => {
    setupFormMockRoutes(page, { forms: { create: SAMPLE_FORM_TEMPLATE } });
    await page.evaluate(() => router.navigate('forms'));
    await page.click('#btn-create-form');

    await page.fill('#form-name', 'Test Form');
    await page.fill('#form-key', 'Test-Form');  // Invalid: uppercase and hyphen

    // Add a field first
    await page.click('button:has-text("Add Field")');
    await page.fill('#field-label', 'Name');
    await page.fill('#field-key', 'name');
    await page.selectOption('#field-type', 'text');
    await page.click('#form-field-modal button:has-text("Save")');
    await page.waitForSelector('#form-field-modal', { state: 'hidden' });

    // Try to save
    await page.click('#btn-form-save');

    // Should show error about key format
    await expect(page.locator('#form-design-error')).toContainText('Form key must be lowercase');
  });

  test('previews a form using runtime renderer', async ({ page }) => {
    setupFormMockRoutes(page, { forms: { list: [SAMPLE_FORM_TEMPLATE] } });
    await page.evaluate(() => router.navigate('forms'));
    await page.click('#btn-create-form');

    // Fill form metadata
    await page.fill('#form-name', 'Preview Test');
    await page.fill('#form-key', 'preview_test');

    // Add a field
    await page.click('button:has-text("Add Field")');
    await page.fill('#field-label', 'Full Name');
    await page.fill('#field-key', 'full_name');
    await page.selectOption('#field-type', 'text');
    await page.click('#field-required');
    await page.click('#form-field-modal button:has-text("Save")');
    await page.waitForSelector('#form-field-modal', { state: 'hidden' });

    // Preview
    await page.click('#btn-form-preview');
    await expect(page.locator('#form-modal')).toBeVisible();
    await expect(page.locator('#form-modal-title')).toHaveText('Preview Test (Draft)');
    await expect(page.locator('#form-modal .dynamic-form')).toBeVisible();

    // Close preview
    await page.click('#form-modal button:has-text("Cancel")');
    await expect(page.locator('#form-modal')).toBeHidden();
  });

  test('assigns form to workflow state', async ({ page }) => {
    setupFormMockRoutes(page, {
      forms: {
        list: [SAMPLE_FORM_TEMPLATE],
        get: { 'form-mgmt-1': SAMPLE_FORM_TEMPLATE },
        assignments: [{ id: 'assign-1', workflow_id: 'wf-1', state_key: 'IN_REVIEW', required: true, workflow_name: 'Emergency Response' }],
      },
      workflows: SAMPLE_WORKFLOWS,
    });

    await page.evaluate(() => router.navigate('forms'));
    await page.click('#form-table-body tr button:text("View")');
    await expect(page.locator('#view-form-detail')).toBeVisible();

    // Should show assignments
    await expect(page.locator('#form-workflow-assignments')).toContainText('Emergency Response');
    await expect(page.locator('#form-workflow-assignments')).toContainText('IN_REVIEW');
  });

  test('shows Forms nav button for admin users', async ({ page }) => {
    // The beforeEach already sets an admin JWT token
    await page.goto('/');
    await expect(page).toHaveURL(/#dashboard/);
    await expect(page.locator('#view-dashboard')).toBeVisible();

    // Admin users should see the Forms nav button
    await expect(page.locator('#btn-forms-nav')).toBeVisible();
  });

  test('handles 404 gracefully for form listing', async ({ page }) => {
    page.route(regex(`organizations/${ORG_ID}/forms`), async (route) => {
      await route.fulfill({ status: 404, json: { success: false, error: { message: 'Not found' } } });
    });
    await page.evaluate(() => router.navigate('forms'));
    await expect(page.locator('#form-list-loading')).toBeHidden();
    await expect(page.locator('#form-table-body')).toContainText('No forms');
  });

  test('removes a form assignment with confirmation', async ({ page }) => {
    setupFormMockRoutes(page, {
      forms: {
        list: [SAMPLE_FORM_TEMPLATE],
        get: { 'form-mgmt-1': SAMPLE_FORM_TEMPLATE },
        assignments: [{ id: 'assign-1', workflow_id: 'wf-1', state_key: 'IN_REVIEW', required: true, workflow_name: 'Emergency Response' }],
      },
      workflows: SAMPLE_WORKFLOWS,
    });

    page.on('dialog', dialog => dialog.dismiss()); // Cancel the confirm dialog

    await page.evaluate(() => router.navigate('forms'));
    await page.click('#form-table-body tr button:text("View")');
    await expect(page.locator('#form-workflow-assignments')).toBeVisible();
    await page.click('#form-workflow-assignments button:text("Remove")');
    // Assignment should still be there since dialog was cancelled
    await expect(page.locator('#form-workflow-assignments')).toContainText('Emergency Response');
  });
});
