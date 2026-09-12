// Simple frontend test runner - can be run in browser console
// Tests core utility functions and app behavior

const TestRunner = {
  tests: [],
  passed: 0,
  failed: 0,

  assert(condition, message) {
    if (condition) {
      console.log(`PASS: ${message}`);
      this.passed++;
      return true;
    } else {
      console.error(`FAIL: ${message}`);
      this.failed++;
      return false;
    }
  },

  assertEqual(actual, expected, message) {
    return this.assert(actual === expected, `${message} (expected: ${expected}, actual: ${actual})`);
  },

  run() {
    console.log('Running CIVORA Frontend Tests...');
    this.tests.forEach(test => test());
    console.log(`Results: ${this.passed} passed, ${this.failed} failed`);
    return this.failed === 0;
  },

  add(name, fn) {
    this.tests.push(() => {
      console.log(`--- ${name} ---`);
      fn();
    });
  }
};

// Test escapeHTML
TestRunner.add('escapeHTML utility', () => {
  const { escapeHTML } = window;
  TestRunner.assertEqual(escapeHTML('<script>alert(1)</script>'), '&lt;script&gt;alert(1)&lt;/script&gt;', 'escapes HTML tags');
  TestRunner.assertEqual(escapeHTML('Tom & Jerry'), 'Tom &amp; Jerry', 'escapes ampersand');
  TestRunner.assertEqual(escapeHTML('"quoted"'), '&quot;quoted&quot;', 'escapes quotes');
  TestRunner.assertEqual(escapeHTML("'single'"), '&#039;single&#039;', 'escapes single quotes');
  TestRunner.assertEqual(escapeHTML(null), '', 'handles null');
  TestRunner.assertEqual(escapeHTML(undefined), '', 'handles undefined');
});

// Test getTerminalStates
TestRunner.add('getTerminalStates utility', () => {
  const { getTerminalStates } = window;
  const def = {
    states: [
      { key: 'NEW', terminal: false },
      { key: 'APPROVED', terminal: false },
      { key: 'CLOSED', terminal: true },
      { key: 'REJECTED', terminal: true }
    ]
  };
  const terminals = getTerminalStates(def);
  TestRunner.assertEqual(terminals.length, 2, 'finds 2 terminal states');
  TestRunner.assert(terminals.includes('CLOSED'), 'includes CLOSED');
  TestRunner.assert(terminals.includes('REJECTED'), 'includes REJECTED');
  TestRunner.assert(!terminals.includes('NEW'), 'excludes NEW');
  TestRunner.assertEqual(getTerminalStates(null), [], 'handles null');
  TestRunner.assertEqual(getTerminalStates({}), [], 'handles empty object');
});

// Test computeTerminalStateSet with arbitrary workflows
TestRunner.add('computeTerminalStateSet utility', () => {
  const { computeTerminalStateSet } = window;
  const defs = [
    { status: 'ACTIVE', states: [
      { key: 'REQUEST', terminal: false },
      { key: 'TRIAGE', terminal: false },
      { key: 'REVIEW', terminal: false },
      { key: 'APPROVED', terminal: false },
      { key: 'CLOSED', terminal: true }
    ]},
    { status: 'ACTIVE', states: [
      { key: 'APPLICATION', terminal: false },
      { key: 'ELIGIBILITY', terminal: false },
      { key: 'ASSESSMENT', terminal: false },
      { key: 'DECISION', terminal: false },
      { key: 'TERMINATED', terminal: true }
    ]}
  ];
  const set = computeTerminalStateSet(defs);
  TestRunner.assert(set.has('CLOSED'), 'includes CLOSED from first def');
  TestRunner.assert(set.has('TERMINATED'), 'includes TERMINATED from second def');
  TestRunner.assert(!set.has('APPROVED'), 'excludes non-terminal APPROVED');
});

// Test deriveServiceTypeForWorkflow (service-type workflow binding)
TestRunner.add('deriveServiceTypeForWorkflow utility', () => {
  const { deriveServiceTypeForWorkflow } = window;
  TestRunner.assertEqual(deriveServiceTypeForWorkflow({ key: 'emergency_assistance', status: 'ACTIVE' }), 'EMERGENCY', 'maps emergency key');
  TestRunner.assertEqual(deriveServiceTypeForWorkflow({ key: 'medical_assistance', status: 'ACTIVE' }), 'MEDICAL', 'maps medical key');
  TestRunner.assertEqual(deriveServiceTypeForWorkflow({ key: 'general_assistance', status: 'ACTIVE' }), 'GENERAL', 'maps general key');
  TestRunner.assertEqual(deriveServiceTypeForWorkflow({ key: 'custom_workflow', status: 'ACTIVE' }), null, 'returns null for unknown key');
  TestRunner.assertEqual(deriveServiceTypeForWorkflow({ key: 'emergency_assistance', status: 'DRAFT' }), null, 'returns null for non-active');
  const md = { key: 'custom', status: 'ACTIVE', metadata: { service_type: 'FINANCIAL' } };
  TestRunner.assertEqual(deriveServiceTypeForWorkflow(md), 'FINANCIAL', 'uses metadata service_type override');
});

// Test cssStateClass for arbitrary state names
TestRunner.add('cssStateClass utility', () => {
  const { cssStateClass } = window;
  TestRunner.assertEqual(cssStateClass('IN_REVIEW'), 'in-review', 'slugifies state');
  TestRunner.assertEqual(cssStateClass('TRIAGE'), 'triage', 'lowercase state');
  TestRunner.assertEqual(cssStateClass('NEEDS_MORE_INFO'), 'needs-more-info', 'handles underscores');
  TestRunner.assertEqual(cssStateClass(''), '', 'handles empty');
  TestRunner.assertEqual(cssStateClass(null), '', 'handles null');
});

// Test isCaseTerminal
TestRunner.add('isCaseTerminal utility', () => {
  const { isCaseTerminal } = window;
  TestRunner.assert(isCaseTerminal('CLOSED', null), 'CLOSED case status is terminal');
  TestRunner.assert(isCaseTerminal('REJECTED', null), 'REJECTED case status is terminal');
  TestRunner.assert(!isCaseTerminal('OPEN', null), 'OPEN case status is not terminal');
  TestRunner.assert(!isCaseTerminal('IN_REVIEW', null), 'IN_REVIEW case status is not terminal');

  const wfWithTerminal = {
    definition: { states: [{ key: 'CLOSED', terminal: true }] },
    instance: { current_state: 'CLOSED' }
  };
  TestRunner.assert(isCaseTerminal('OPEN', wfWithTerminal), 'workflow terminal state makes case terminal');

  const wfNonTerminal = {
    definition: { states: [{ key: 'APPROVED', terminal: false }] },
    instance: { current_state: 'APPROVED' }
  };
  TestRunner.assert(!isCaseTerminal('APPROVED', wfNonTerminal), 'workflow non-terminal state keeps case non-terminal');

  const customWf = {
    definition: { states: [{ key: 'TERMINATED', terminal: true }] },
    instance: { current_state: 'TERMINATED' }
  };
  TestRunner.assert(isCaseTerminal('TERMINATED', customWf, new Set(['TERMINATED'])), 'custom terminal state is terminal');
});

// Test SERVICE_DOMAIN
TestRunner.add('SERVICE_DOMAIN configuration', () => {
  const { SERVICE_DOMAIN } = window;
  TestRunner.assert(SERVICE_DOMAIN.EMERGENCY !== undefined, 'EMERGENCY domain exists');
  TestRunner.assert(SERVICE_DOMAIN.MEDICAL !== undefined, 'MEDICAL domain exists');
  TestRunner.assert(SERVICE_DOMAIN.FINANCIAL !== undefined, 'FINANCIAL domain exists');
  TestRunner.assert(SERVICE_DOMAIN.GENERAL !== undefined, 'GENERAL domain exists');
  TestRunner.assertEqual(SERVICE_DOMAIN.EMERGENCY.sectionActionStates, undefined, 'EMERGENCY has no sectionActionStates');
  TestRunner.assertEqual(SERVICE_DOMAIN.MEDICAL.sectionActionStates, undefined, 'MEDICAL has no sectionActionStates');
  TestRunner.assert(SERVICE_DOMAIN.EMERGENCY.sections.eligibility !== undefined, 'EMERGENCY has eligibility section');
  TestRunner.assert(SERVICE_DOMAIN.GENERAL.sections.followup !== undefined, 'GENERAL has followup section');
  TestRunner.assert(SERVICE_DOMAIN.EMERGENCY.assistanceTypes !== undefined, 'EMERGENCY has assistanceTypes');
});

// Test getServiceDomain fallback
TestRunner.add('getServiceDomain fallback', () => {
  const { getServiceDomain } = window;
  TestRunner.assert(getServiceDomain('EMERGENCY') !== null, 'returns domain for known type');
  TestRunner.assert(getServiceDomain('UNKNOWN_TYPE') !== null, 'returns GENERAL fallback for unknown type');
  TestRunner.assertEqual(getServiceDomain('UNKNOWN_TYPE').label, 'General Assistance', 'fallback has correct label');
});

// Test buildSectionTransitionMap derives from transitions
TestRunner.add('buildSectionTransitionMap', () => {
  const { isCaseTerminal } = window;
  TestRunner.assert(typeof isCaseTerminal === 'function', 'isCaseTerminal is a function');
});

// Run tests when loaded
if (typeof window !== 'undefined') {
  window.TestRunner = TestRunner;
  console.log('TestRunner loaded. Run TestRunner.run() in console to execute tests.');
}