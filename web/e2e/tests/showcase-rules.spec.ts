// Showcase for rules.spec.ts — Rules engine admin UI
import { test, expect, type Page } from '@playwright/test';
import { openDashboard, routes, regex, ORG_ID, ADMIN_TOKEN } from './helpers';

const SAMPLE_RULE_SET = {
  id: 'ruleset-1',
  organization_id: ORG_ID,
  key: 'income_eligibility',
  name: 'Income Eligibility Check',
  description: 'Determines eligibility based on income thresholds.',
  version: 1,
  status: 'PUBLISHED',
  default_outcome: 'INELIGIBLE',
  rules: [
    {
      id: 'rule-1',
      priority: 0,
      outcome: 'ELIGIBLE',
      active: true,
      conditions: {
        all: [
          { field: 'form.income.amount', operator: 'lt', value: 500 },
          { field: 'form.housing.status', operator: 'eq', value: 'HOMELESS' },
        ],
      },
      created_at: '2025-01-01T00:00:00Z',
    },
  ],
  triggers: ['case.created'],
  created_by: 'user-1',
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
};

function setupRulesMockRoutes(page: Page) {
  const ruleSetsUrl = regex(`organizations/${ORG_ID}/rules/rule-sets$`);
  const ruleSetsPagedUrl = regex(`organizations/${ORG_ID}/rules/rule-sets\\?`);
  const ruleSetsByIdUrl = regex(`organizations/${ORG_ID}/rules/rule-sets/[^/]+$`);

  page.route(ruleSetsUrl, async (route, req) => {
    if (req.method() === 'GET') {
      await route.fulfill({
        json: {
          success: true,
          data: [SAMPLE_RULE_SET],
          meta: { page: 1, per_page: 20, total: 1, total_pages: 1 },
        },
      });
    } else {
      await route.continue();
    }
  });

  page.route(ruleSetsPagedUrl, async (route, req) => {
    if (req.method() === 'GET') {
      await route.fulfill({
        json: {
          success: true,
          data: [SAMPLE_RULE_SET],
          meta: { page: 1, per_page: 20, total: 1, total_pages: 1 },
        },
      });
    } else {
      await route.continue();
    }
  });

  page.route(ruleSetsByIdUrl, async (route, req) => {
    if (req.method() === 'GET') {
      await route.fulfill({
        json: {
          success: true,
          data: { ...SAMPLE_RULE_SET, trace: {
            evaluation_id: 'eval-1',
            result: 'PASS',
            facts: { income: 300, housing_status: 'HOMELESS' },
            rule_results: [
              { rule_id: 'rule-1', description: 'Income below threshold', passed: true },
            ],
          } },
        },
      });
    } else {
      await route.continue();
    }
  });
}

test.describe('Showcase: Rules Engine', () => {
  test('renders rules list and rule set detail', async ({ page }) => {
    await page.setViewportSize({ width: 1400, height: 900 });

    await page.addInitScript(() => {
      localStorage.setItem('civora_token', ADMIN_TOKEN);
      localStorage.setItem('civora_org_id', ORG_ID);
    });

    page.route(routes.workflows, async (route) => {
      await route.fulfill({ json: { success: true, data: [] } });
    });

    page.route(routes.dashboardStats, async (route) => {
      await route.fulfill({ json: { success: true, data: { cases: [], by_status: {}, by_service_type: {} } } });
    });

    page.route(routes.casesSearch, async (route) => {
      await route.fulfill({ json: { success: true, data: [] } });
    });

    setupRulesMockRoutes(page);

    await page.goto('/');
    await page.waitForLoadState('networkidle');
    await page.evaluate(() => router.navigate('rules'));
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(500);
    await page.screenshot({ path: 'test-results/showcase-rules/01-rules-list.png', fullPage: true });

    await expect(page.locator('#view-rules')).toBeVisible();
    await expect(page.locator('#rules-table-body')).toBeVisible();
  });
});
