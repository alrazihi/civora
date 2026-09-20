// Showcase for cases.spec.ts — New case workflow selection
import { test, expect, type Page } from '@playwright/test';
import { EMERGENCY_WORKFLOW, PERSON, openDashboard, attachCaseMocks, routes, regex, ORG_ID } from './helpers';

const CASE_ID = 'case-cases-showcase';

test.describe('Showcase: New Case Workflow Selection', () => {
  test.beforeEach(async ({ page }) => {
    await page.addInitScript(() => {
      localStorage.setItem('civora_token', 'dev-token');
      localStorage.setItem('civora_org_id', 'org-test');
    });
  });

  test('renders workflow selector and case workspace', async ({ page }) => {
    await page.setViewportSize({ width: 1400, height: 900 });

    page.route(routes.workflows, async (route) => {
      await route.fulfill({
        json: {
          success: true,
          data: [
            {
              id: 'wf-emergency-1',
              key: 'emergency_assistance',
              name: 'Emergency Assistance',
              version: 1,
              status: 'ACTIVE',
              initial_state: 'NEW',
              states: [
                { key: 'NEW', name: 'New', terminal: false },
                { key: 'IN_PROGRESS', name: 'In Progress', terminal: false },
                { key: 'REVIEW', name: 'Review', terminal: false },
                { key: 'CLOSED', name: 'Closed', terminal: true },
              ],
              transitions: [
                { key: 'start', from_state: 'NEW', to_state: 'IN_PROGRESS' },
                { key: 'submit_for_review', from_state: 'IN_PROGRESS', to_state: 'REVIEW' },
                { key: 'approve', from_state: 'REVIEW', to_state: 'CLOSED' },
              ],
              description: 'Emergency assistance workflow for crisis response.',
            },
            {
              id: 'wf-benefits-1',
              key: 'benefits_application',
              name: 'Benefits Application',
              version: 1,
              status: 'ACTIVE',
              initial_state: 'SUBMITTED',
              states: [
                { key: 'SUBMITTED', name: 'Submitted', terminal: false },
                { key: 'UNDER_REVIEW', name: 'Under Review', terminal: false },
                { key: 'APPROVED', name: 'Approved', terminal: false },
                { key: 'REJECTED', name: 'Rejected', terminal: true },
              ],
              transitions: [
                { key: 'review', from_state: 'SUBMITTED', to_state: 'UNDER_REVIEW' },
                { key: 'approve', from_state: 'UNDER_REVIEW', to_state: 'APPROVED' },
                { key: 'reject', from_state: 'UNDER_REVIEW', to_state: 'REJECTED' },
              ],
              description: 'Benefits application processing workflow.',
            },
          ],
        },
      });
    });

    page.route(routes.casesCreate, async (route, request) => {
      if (request.method() !== 'POST') {
        await route.continue();
        return;
      }
      const body = JSON.parse(request.postData() || '{}');
      const created = {
        id: CASE_ID,
        title: body.title,
        status: 'NEW',
        service_type: body.service_type,
        priority: body.priority,
        person_id: body.person_id,
      };
      await route.fulfill({ json: { success: true, data: created } });
    });

    page.route(regex(`organizations/${ORG_ID}/cases/${CASE_ID}/workflow$`), async (route) => {
      await route.fulfill({
        json: {
          success: true,
          data: {
            instance: {
              id: 'inst-cases',
              workflow_definition_id: 'wf-emergency-1',
              definition: {
                id: 'wf-emergency-1',
                key: 'emergency_assistance',
                name: 'Emergency Assistance',
                initial_state: 'NEW',
                states: [
                  { key: 'NEW', name: 'New', terminal: false },
                  { key: 'IN_PROGRESS', name: 'In Progress', terminal: false },
                  { key: 'REVIEW', name: 'Review', terminal: false },
                  { key: 'CLOSED', name: 'Closed', terminal: true },
                ],
                transitions: [
                  { key: 'start', from_state: 'NEW', to_state: 'IN_PROGRESS' },
                  { key: 'submit_for_review', from_state: 'IN_PROGRESS', to_state: 'REVIEW' },
                  { key: 'approve', from_state: 'REVIEW', to_state: 'CLOSED' },
                ],
              },
              instance: { current_state: 'NEW' },
            },
          },
        },
      });
    });

    page.route(regex(`organizations/${ORG_ID}/cases/${CASE_ID}$`), async (route) => {
      await route.fulfill({
        json: {
          success: true,
          data: {
            id: CASE_ID,
            case_number: 'CR-2024-005',
            title: 'Emergency Food Assistance Request',
            status: 'NEW',
            service_type: 'EMERGENCY',
            priority: 'HIGH',
            description: 'Client needs emergency food assistance and shelter referral.',
            person_id: 'person-1',
            created_at: '2026-09-12T01:00:00Z',
            updated_at: '2026-09-12T02:00:00Z',
          },
        },
      });
    });

    await openDashboard(page);

    await page.evaluate(() => router.navigate('new-case'));
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(500);
    await page.screenshot({ path: 'test-results/showcase-cases/01-workflow-selector.png', fullPage: true });

    const selector = page.locator('#workflow-selector');
    await expect(selector).toBeVisible();
    await expect(selector.locator('.wf-option-card')).toHaveCount(2);
    await expect(selector.locator('.wf-option-card').first()).toContainText(/Emergency Assistance/);
    await expect(selector.locator('.wf-option-card').last()).toContainText(/Benefits Application/);
    await expect(page.locator('#selected-workflow-id')).toHaveValue('wf-emergency-1');

    await page.fill('#c-title', 'Burst pipe on 3rd floor');
    await page.evaluate((v) => {
      (document.getElementById('new-case-person-id') as HTMLInputElement).value = v;
    }, PERSON.id);
    await page.click('#view-new-case form button[type="submit"]', { noWaitAfter: true });
    await page.waitForTimeout(1500);
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(800);
    await page.screenshot({ path: 'test-results/showcase-cases/02-case-workspace.png', fullPage: true });
  });
});
