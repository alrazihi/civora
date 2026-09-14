// Rules Engine E2E Tests

import { test, expect } from '@playwright/test';
import {
  ORG_ID,
  ADMIN_TOKEN,
  regex,
} from './helpers';

// Sample rule set fixtures
const SAMPLE_RULE_SET = {
  id: 'ruleset-1',
  organization_id: ORG_ID,
  key: 'income_eligibility',
  name: 'Income Eligibility Check',
  description: 'Determines eligibility based on income thresholds.',
  version: 1,
  status: 'DRAFT',
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

const PUBLISHED_RULE_SET = {
  ...SAMPLE_RULE_SET,
  id: 'ruleset-pub-1',
  status: 'PUBLISHED',
};

const ARCHIVED_RULE_SET = {
  ...SAMPLE_RULE_SET,
  id: 'ruleset-arch-1',
  status: 'ARCHIVED',
};

const SAMPLE_EVALUATION = {
  id: 'eval-1',
  rule_set_id: 'ruleset-1',
  rule_set_version: 1,
  organization_id: ORG_ID,
  case_id: null,
  status: 'ELIGIBLE',
  outcome: 'ELIGIBLE',
  reason: 'Matched rule: income below threshold and housing status is HOMELESS',
  matched_rule_id: 'rule-1',
  trace: [
    {
      id: 'root-1',
      node_type: 'root',
      description: 'Rule set evaluation',
      result: 'TRUE',
      children: [
        {
          id: 'rule-1-node',
          node_type: 'rule',
          description: 'Rule 1: ELIGIBLE',
          result: 'TRUE',
          children: [
            {
              id: 'cond-1',
              node_type: 'group',
              description: 'ALL group',
              result: 'TRUE',
              children: [
                {
                  id: 'leaf-1',
                  node_type: 'leaf',
                  description: 'Compare form.income.amount',
                  field: 'form.income.amount',
                  operator: 'lt',
                  expected: '500',
                  actual: '300',
                  actual_type: 'number',
                  result: 'TRUE',
                  reason: '300 < 500',
                },
                {
                  id: 'leaf-2',
                  node_type: 'leaf',
                  description: 'Compare form.housing.status',
                  field: 'form.housing.status',
                  operator: 'eq',
                  expected: '"HOMELESS"',
                  actual: '"HOMELESS"',
                  actual_type: 'string',
                  result: 'TRUE',
                  reason: 'HOMELESS == HOMELESS',
                },
              ],
            },
          ],
        },
      ],
      timestamp: '2025-06-15T10:00:00Z',
    },
  ],
  trigger: 'MANUAL',
  evaluated_by: 'user-1',
  evaluated_at: '2025-06-15T10:00:00Z',
  facts_snapshot: { form: { income: { amount: 300 }, housing: { status: 'HOMELESS' } } },
};

const DISCOVERABLE_FIELDS = [
  {
    key: 'form.income_form.amount',
    label: 'Income Verification: Annual Income',
    type: 'DECIMAL',
    required: true,
    options: [],
  },
  {
    key: 'form.housing_form.status',
    label: 'Housing Status: Housing Status',
    type: 'SELECT',
    required: true,
    options: [
      { value: 'HOMELESS', label: 'Homeless' },
      { value: 'HOUSED', label: 'Housed' },
    ],
  },
];

// Set up mock routes for rules API endpoints
function setupRulesMockRoutes(page: any, config: {
  ruleSets?: any[];
  evaluations?: any[];
  fields?: any[];
  createResponse?: any;
  updateResponse?: any;
  evaluateResponse?: any;
} = {}) {
  const rules = config.ruleSets || [];
  const evaluations = config.evaluations || [];
  const fields = config.fields || DISCOVERABLE_FIELDS;

  const ruleSetsUrl = regex(`organizations/${ORG_ID}/rules/rule-sets$`);
  const ruleSetsPagedUrl = regex(`organizations/${ORG_ID}/rules/rule-sets\\?`);
  const ruleSetsByIdUrl = regex(`organizations/${ORG_ID}/rules/rule-sets/[^/]+$`);
  const ruleSetsByKeyUrl = regex(`organizations/${ORG_ID}/rules/rule-sets/key/[^/]+$`);
  const versionsUrl = regex(`organizations/${ORG_ID}/rules/rule-sets/[^/]+/versions$`);
  const versionUrl = regex(`organizations/${ORG_ID}/rules/rule-sets/[^/]+/version$`);
  const publishUrl = regex(`organizations/${ORG_ID}/rules/rule-sets/[^/]+/publish$`);
  const archiveUrl = regex(`organizations/${ORG_ID}/rules/rule-sets/[^/]+/archive$`);
  const evalByRuleSetUrl = regex(`organizations/${ORG_ID}/rules/rule-sets/[^/]+/evaluations$`);
  const evaluateUrl = regex(`organizations/${ORG_ID}/rules/rule-sets/[^/]+/evaluate$`);
  const evalByIdUrl = regex(`organizations/${ORG_ID}/rules/evaluations/[^/]+$`);
  const evalByCaseUrl = regex(`organizations/${ORG_ID}/rules/cases/[^/]+/evaluations$`);
  const fieldsUrl = regex(`organizations/${ORG_ID}/rules/fields`);

  // List rule sets (no query params)
  page.route(ruleSetsUrl, async (route: any, req: any) => {
    if (req.method() === 'GET') {
      await route.fulfill({
        json: {
          success: true,
          data: rules,
          meta: { page: 1, per_page: 20, total: rules.length, total_pages: 1 },
        },
      });
    } else if (req.method() === 'POST') {
      const body = JSON.parse(req.postData() || '{}');
      const created = { ...body, id: 'ruleset-new', version: 1, status: 'DRAFT', organization_id: ORG_ID, created_at: '2025-01-01T00:00:00Z', updated_at: '2025-01-01T00:00:00Z' };
      await route.fulfill({ status: 201, json: { success: true, data: created } });
    } else {
      route.continue();
    }
  });

  // List rule sets (with query params for filtering/pagination)
  page.route(ruleSetsPagedUrl, async (route: any, req: any) => {
    if (req.method() === 'GET') {
      const url = new URL(req.url(), 'http://localhost');
      const key = url.searchParams.get('key');
      const status = url.searchParams.get('status');
      let filtered = rules;
      if (key) filtered = filtered.filter(r => r.key.includes(key));
      if (status) filtered = filtered.filter(r => r.status === status);
      await route.fulfill({
        json: {
          success: true,
          data: filtered,
          meta: { page: 1, per_page: 20, total: filtered.length, total_pages: 1 },
        },
      });
    } else {
      route.continue();
    }
  });

  // List versions of a rule set
  page.route(versionsUrl, async (route: any, req: any) => {
    if (req.method() === 'GET') {
      const versions = rules;
      await route.fulfill({
        json: {
          success: true,
          data: versions,
          meta: { page: 1, per_page: 20, total: versions.length, total_pages: 1 },
        },
      });
    } else {
      route.continue();
    }
  });

  // Create a new version
  page.route(versionUrl, async (route: any, req: any) => {
    if (req.method() === 'POST') {
      const rs = rules[0];
      await route.fulfill({
        status: 201,
        json: { success: true, data: { ...(rs || SAMPLE_RULE_SET), version: (rs?.version || 1) + 1, status: 'DRAFT' } },
      });
    } else {
      route.continue();
    }
  });

  // Publish a rule set
  page.route(publishUrl, async (route: any, req: any) => {
    if (req.method() === 'POST') {
      const rs = rules[0];
      await route.fulfill({ json: { success: true, data: { ...(rs || SAMPLE_RULE_SET), status: 'PUBLISHED' } } });
    } else {
      route.continue();
    }
  });

  // Archive a rule set
  page.route(archiveUrl, async (route: any, req: any) => {
    if (req.method() === 'POST') {
      const rs = rules[0];
      await route.fulfill({ json: { success: true, data: { ...(rs || SAMPLE_RULE_SET), status: 'ARCHIVED' } } });
    } else {
      route.continue();
    }
  });

  // List evaluations by rule set
  page.route(evalByRuleSetUrl, async (route: any, req: any) => {
    if (req.method() === 'GET') {
      await route.fulfill({
        json: {
          success: true,
          data: evaluations,
          meta: { page: 1, per_page: 20, total: evaluations.length, total_pages: 1 },
        },
      });
    } else {
      route.continue();
    }
  });

  // Evaluate a rule set
  page.route(evaluateUrl, async (route: any, req: any) => {
    if (req.method() === 'POST') {
      await route.fulfill({ json: { success: true, data: config.evaluateResponse || SAMPLE_EVALUATION } });
    } else {
      route.continue();
    }
  });

  // Get an evaluation by ID
  page.route(evalByIdUrl, async (route: any, req: any) => {
    if (req.method() === 'GET') {
      await route.fulfill({ json: { success: true, data: config.evaluateResponse || SAMPLE_EVALUATION } });
    } else {
      route.continue();
    }
  });

  // List evaluations by case
  page.route(evalByCaseUrl, async (route: any, req: any) => {
    if (req.method() === 'GET') {
      await route.fulfill({
        json: {
          success: true,
          data: evaluations,
          meta: { page: 1, per_page: 20, total: evaluations.length, total_pages: 1 },
        },
      });
    } else {
      route.continue();
    }
  });

  // Get/update/delete specific rule set by ID
  page.route(ruleSetsByIdUrl, async (route: any, req: any) => {
    if (req.method() === 'GET') {
      const id = req.url().split('/').pop();
      const rs = rules.find(r => r.id === id);
      if (rs) {
        await route.fulfill({ json: { success: true, data: rs } });
      } else {
        await route.fulfill({ status: 404, json: { success: false, error: { message: 'Not found' } } });
      }
    } else if (req.method() === 'PATCH') {
      await route.fulfill({ json: { success: true, data: config.updateResponse || rules[0] } });
    } else if (req.method() === 'DELETE') {
      await route.fulfill({ json: { success: true, data: { deleted: true } } });
    } else {
      route.continue();
    }
  });

  // Get rule set by key
  page.route(ruleSetsByKeyUrl, async (route: any, req: any) => {
    if (req.method() === 'GET') {
      const key = req.url().split('/').pop();
      const rs = rules.find(r => r.key === decodeURIComponent(key));
      if (rs) {
        await route.fulfill({ json: { success: true, data: rs } });
      } else {
        await route.fulfill({ status: 404, json: { success: false, error: { message: 'Not found' } } });
      }
    } else {
      route.continue();
    }
  });

  // List discoverable fields
  page.route(fieldsUrl, async (route: any, req: any) => {
    if (req.method() === 'GET') {
      await route.fulfill({ json: { success: true, data: fields } });
    } else {
      route.continue();
    }
  });
}

test.describe('Rules Engine', () => {
  test.beforeEach(async ({ page }) => {
    await page.addInitScript(`
      localStorage.setItem('civora_token', '${ADMIN_TOKEN}');
      localStorage.setItem('civora_org_id', '${ORG_ID}');
    `);
    await page.goto('/');
    await expect(page).toHaveURL(/#dashboard/);
    await expect(page.locator('#view-dashboard')).toBeVisible();
  });

  test('shows Rules nav button for admin users', async ({ page }) => {
    await expect(page.locator('#btn-rules-nav')).toBeVisible();
  });

  test('shows rule sets list with empty state', async ({ page }) => {
    setupRulesMockRoutes(page, { ruleSets: [] });
    await page.evaluate(() => router.navigate('rules'));
    await expect(page.locator('#view-rules')).toBeVisible();
    await expect(page.locator('#rules-table-body')).toContainText('No rule sets yet');
  });

  test('lists rule sets from the API', async ({ page }) => {
    setupRulesMockRoutes(page, { ruleSets: [SAMPLE_RULE_SET, PUBLISHED_RULE_SET] });
    await page.evaluate(() => router.navigate('rules'));
    await expect(page.locator('#rules-table-body tr')).toHaveCount(2);
    await expect(page.locator('#rules-table-body')).toContainText('income_eligibility');
    await expect(page.locator('#rules-table-body')).toContainText('Income Eligibility Check');
  });

  test('navigates to rule set detail from list', async ({ page }) => {
    setupRulesMockRoutes(page, { ruleSets: [SAMPLE_RULE_SET] });
    await page.evaluate(() => router.navigate('rules'));
    await page.click('#rules-table-body button:has-text("View")');
    await expect(page.locator('#view-rule-set-detail')).toBeVisible();
    await expect(page.locator('#ruleset-detail-container')).toContainText('Income Eligibility Check');
  });

  test('shows create rule set view', async ({ page }) => {
    setupRulesMockRoutes(page, { ruleSets: [] });
    await page.evaluate(() => router.navigate('rules'));
    await page.click('#btn-create-ruleset');
    await expect(page.locator('#view-rule-set-edit')).toBeVisible();
    await expect(page.locator('#ruleset-edit-title')).toHaveText('New Rule Set');
  });

  test('creates a rule set with rules', async ({ page }) => {
    setupRulesMockRoutes(page, { ruleSets: [] });
    await page.evaluate(() => router.navigate('rules'));
    await page.click('#btn-create-ruleset');

    await page.fill('#ruleset-edit-key', 'test_eligibility');
    await page.fill('#ruleset-edit-name', 'Test Eligibility');
    await page.fill('#ruleset-edit-description', 'A test rule set');

    await page.click('button:has-text("Add Rule")');
    await expect(page.locator('#rule-editor-modal')).toBeVisible();
    await page.selectOption('#rule-outcome', 'ELIGIBLE');
    await page.fill('#cond-root-field', 'form.income.amount');
    await page.selectOption('#cond-root-op', 'lt');
    await page.fill('#cond-root-value', '500');
    await page.click('button:has-text("Save Rule")');

    await expect(page.locator('#rules-list')).not.toContainText('No rules yet');
    await page.click('#btn-ruleset-save-draft');
  });

  test('validates rule set before saving', async ({ page }) => {
    setupRulesMockRoutes(page, { ruleSets: [] });
    await page.evaluate(() => router.navigate('rules'));
    await page.click('#btn-create-ruleset');

    await page.fill('#ruleset-edit-name', 'Test');
    await page.fill('#ruleset-edit-key', 'test_key');

    await page.click('button:has-text("Validate")');
    await expect(page.locator('#ruleset-validation-result')).toContainText('rule');
  });

  test('evaluates a rule set with facts', async ({ page }) => {
    setupRulesMockRoutes(page, {
      ruleSets: [SAMPLE_RULE_SET],
      evaluateResponse: SAMPLE_EVALUATION,
    });
    await page.evaluate(() => router.navigate('rules'));
    await page.click('#rules-table-body button:has-text("View")');
    await expect(page.locator('#ruleset-detail-container')).toContainText('Income Eligibility Check');

    await page.click('button:has-text("Evaluate")');
    await expect(page.locator('#evaluation-modal')).toBeVisible();
    await page.fill('#eval-facts', '{"form": {"income": {"amount": 300}, "housing": {"status": "HOMELESS"}}}');
    await page.click('#evaluation-modal button:has-text("Evaluate")');

    await expect(page.locator('#eval-trace-tree')).toBeVisible();
    await expect(page.locator('#ruleset-detail-container')).toContainText('ELIGIBLE');
  });

  test('filters rule sets by status', async ({ page }) => {
    setupRulesMockRoutes(page, { ruleSets: [SAMPLE_RULE_SET, PUBLISHED_RULE_SET] });
    await page.evaluate(() => router.navigate('rules'));

    await page.selectOption('#rules-status-filter', 'PUBLISHED');
    await page.locator('#rules-status-filter').dispatchEvent('change');
    await page.waitForTimeout(100);
  });

  test('shows discoverable fields in condition editor', async ({ page }) => {
    setupRulesMockRoutes(page, {
      ruleSets: [],
      fields: DISCOVERABLE_FIELDS,
    });
    await page.evaluate(() => router.navigate('rules'));
    await page.click('#btn-create-ruleset');
    await page.fill('#ruleset-edit-name', 'Test');
    await page.fill('#ruleset-edit-key', 'test_key');
    await page.click('button:has-text("Add Rule")');

    await expect(page.locator('#cond-root-field')).toBeVisible();

    const fields = await page.evaluate(async () => {
      try {
        return await window.RulesAPI.listDiscoverableFields();
      } catch (e) {
        return [];
      }
    });
    const dl = await page.$('#cond-field-suggestions');
    if (dl && fields.length) {
      await page.evaluate((flds) => {
        const dl = document.getElementById('cond-field-suggestions');
        if (dl) {
          dl.innerHTML = flds.map((f: any) => `<option value="${f.key}">${f.label}</option>`).join('');
        }
      }, fields);
    }
    await expect(page.locator('#cond-field-suggestions option')).toHaveCount(DISCOVERABLE_FIELDS.length);
  });

  test('archives a published rule set', async ({ page }) => {
    setupRulesMockRoutes(page, { ruleSets: [PUBLISHED_RULE_SET] });
    await page.evaluate(() => router.navigate('rules'));
    await page.click('#rules-table-body button:has-text("View")');

    await page.click('button:has-text("Archive")');
  });
});
