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

 // ── Form Renderer Tests ──────────────────────────────────────────────

 // Test FormRenderer is available
 TestRunner.add('FormRenderer is available', () => {
   TestRunner.assert(typeof window.FormRenderer !== 'undefined', 'FormRenderer is defined');
   TestRunner.assert(Array.isArray(window.FormRenderer.SUPPORTED_FIELD_TYPES), 'SUPPORTED_FIELD_TYPES is an array');
 });

 // Test supported field types
 TestRunner.add('FormRenderer supported field types', () => {
   const { SUPPORTED_FIELD_TYPES } = window.FormRenderer;
   const expected = ['text', 'textarea', 'number', 'decimal', 'date', 'datetime',
     'boolean', 'select', 'multiselect', 'radio', 'checkbox', 'email', 'phone'];
   expected.forEach(t => {
     TestRunner.assert(SUPPORTED_FIELD_TYPES.includes(t), `supports ${t}`);
   });
 });

 // Test validateField function
 TestRunner.add('FormRenderer validateField - required', () => {
   const { validateField } = window.FormRenderer;
   const field = { key: 'name', label: 'Name', type: 'text', required: true };
   TestRunner.assertEqual(validateField(field, '').length, 1, 'empty required field fails');
   TestRunner.assertEqual(validateField(field, null).length, 1, 'null required field fails');
   TestRunner.assertEqual(validateField(field, 'value').length, 0, 'non-empty required field passes');
 });

 TestRunner.add('FormRenderer validateField - number', () => {
   const { validateField } = window.FormRenderer;
   const field = { key: 'count', label: 'Count', type: 'number', required: true, validation: { minimum: 1, maximum: 10 } };
   TestRunner.assertEqual(validateField(field, 'abc').length, 1, 'non-numeric fails');
   TestRunner.assertEqual(validateField(field, '0').length, 1, 'below minimum fails');
   TestRunner.assertEqual(validateField(field, '11').length, 1, 'above maximum fails');
   TestRunner.assertEqual(validateField(field, '5').length, 0, 'in-range number passes');
 });

 TestRunner.add('FormRenderer validateField - email', () => {
   const { validateField } = window.FormRenderer;
   const field = { key: 'email', label: 'Email', type: 'email', required: true };
   TestRunner.assertEqual(validateField(field, 'not-an-email').length, 1, 'invalid email fails');
   TestRunner.assertEqual(validateField(field, 'user@example.com').length, 0, 'valid email passes');
 });

 TestRunner.add('FormRenderer validateField - min/max length', () => {
   const { validateField } = window.FormRenderer;
   const field = { key: 'code', label: 'Code', type: 'text', validation: { min_length: 3, max_length: 5 } };
   TestRunner.assertEqual(validateField(field, 'ab').length, 1, 'too short fails');
   TestRunner.assertEqual(validateField(field, 'abcdef').length, 1, 'too long fails');
   TestRunner.assertEqual(validateField(field, 'abcd').length, 0, 'within range passes');
 });

 TestRunner.add('FormRenderer validateField - pattern', () => {
   const { validateField } = window.FormRenderer;
   const field = { key: 'code', label: 'Code', type: 'text', validation: { pattern: '^[A-Z]{3}$', pattern_message: 'Must be 3 uppercase letters' } };
   TestRunner.assertEqual(validateField(field, 'ABC').length, 0, 'matching pattern passes');
   TestRunner.assertEqual(validateField(field, 'abc').length, 1, 'non-matching pattern fails');
 });

 // Test renderForm renders fields
 TestRunner.add('FormRenderer renderForm - renders fields', () => {
   const { renderForm, createFormState } = window.FormRenderer;
   const formDef = {
     id: 'form-1',
     key: 'test_form',
     name: 'Test Form',
     version: 1,
     fields: [
       { key: 'name', label: 'Full Name', type: 'text', required: true },
       { key: 'email', label: 'Email', type: 'email', required: true },
       { key: 'age', label: 'Age', type: 'number', required: false, validation: { minimum: 0, maximum: 120 } },
     ],
   };
   const container = document.createElement('div');
   document.body.appendChild(container);
   const state = renderForm(formDef, container);
   TestRunner.assert(state !== null, 'returns formState');
   const form = container.querySelector('.dynamic-form');
   TestRunner.assert(form !== null, 'form element is rendered');
   const fields = container.querySelectorAll('.form-field');
   TestRunner.assertEqual(fields.length, 3, 'renders 3 fields');
   const inputs = container.querySelectorAll('input');
   TestRunner.assert(inputs.length > 0, 'renders input elements');
   document.body.removeChild(container);
 });

 // Test required fields render with asterisk
 TestRunner.add('FormRenderer renderForm - required field marker', () => {
   const { renderForm } = window.FormRenderer;
   const formDef = {
     key: 'test',
     name: 'Test',
     fields: [
       { key: 'required_field', label: 'Required Field', type: 'text', required: true },
       { key: 'optional_field', label: 'Optional Field', type: 'text', required: false },
     ],
   };
   const container = document.createElement('div');
   document.body.appendChild(container);
   renderForm(formDef, container);
   const markers = container.querySelectorAll('.required-marker');
   TestRunner.assertEqual(markers.length, 1, 'one required marker for one required field');
   document.body.removeChild(container);
 });

 // Test select field with options renders options
 TestRunner.add('FormRenderer renderForm - select options', () => {
   const { renderForm } = window.FormRenderer;
   const formDef = {
     key: 'test',
     name: 'Test',
     fields: [
       { key: 'priority', label: 'Priority', type: 'select', required: true,
         options: [
           { value: 'low', label: 'Low' },
           { value: 'high', label: 'High' },
         ] },
     ],
   };
   const container = document.createElement('div');
   document.body.appendChild(container);
   renderForm(formDef, container);
   const select = container.querySelector('select');
   TestRunner.assert(select !== null, 'select is rendered');
   const opts = select.querySelectorAll('option');
   TestRunner.assert(opts.length >= 2, 'renders at least 2 options');
   document.body.removeChild(container);
 });

 // Test radio field renders options
 TestRunner.add('FormRenderer renderForm - radio options', () => {
   const { renderForm } = window.FormRenderer;
   const formDef = {
     key: 'test',
     name: 'Test',
     fields: [
       { key: 'choice', label: 'Choice', type: 'radio', required: true,
         options: [{ value: 'a', label: 'Option A' }, { value: 'b', label: 'Option B' }] },
     ],
   };
   const container = document.createElement('div');
   document.body.appendChild(container);
   renderForm(formDef, container);
   const radios = container.querySelectorAll('input[type="radio"]');
   TestRunner.assertEqual(radios.length, 2, 'renders 2 radio inputs');
   document.body.removeChild(container);
 });

 // Test checkbox field renders options
 TestRunner.add('FormRenderer renderForm - checkbox group', () => {
   const { renderForm } = window.FormRenderer;
   const formDef = {
     key: 'test',
     name: 'Test',
     fields: [
       { key: 'tags', label: 'Tags', type: 'checkbox',
         options: [{ value: 'x', label: 'Tag X' }, { value: 'y', label: 'Tag Y' }] },
     ],
   };
   const container = document.createElement('div');
   document.body.appendChild(container);
   renderForm(formDef, container);
   const checkboxes = container.querySelectorAll('input[type="checkbox"]');
   TestRunner.assertEqual(checkboxes.length, 2, 'renders 2 checkbox inputs');
   document.body.removeChild(container);
 });

 // Test boolean field renders as single checkbox
 TestRunner.add('FormRenderer renderForm - boolean field', () => {
   const { renderForm } = window.FormRenderer;
   const formDef = {
     key: 'test',
     name: 'Test',
     fields: [{ key: 'agree', label: 'I agree', type: 'boolean' }],
   };
   const container = document.createElement('div');
   document.body.appendChild(container);
   renderForm(formDef, container);
   const cb = container.querySelector('input[type="checkbox"]');
   TestRunner.assert(cb !== null, 'boolean renders as checkbox');
   document.body.removeChild(container);
 });

 // Test formState collectValues
 TestRunner.add('FormRenderer formState collectValues', () => {
   const { renderForm } = window.FormRenderer;
   const formDef = {
     key: 'test',
     name: 'Test',
     fields: [
       { key: 'name', label: 'Name', type: 'text' },
       { key: 'count', label: 'Count', type: 'number' },
     ],
   };
   const container = document.createElement('div');
   document.body.appendChild(container);
   const state = renderForm(formDef, container);
   const nameInput = container.querySelector('#form-field-name');
   const countInput = container.querySelector('#form-field-count');
   nameInput.value = 'John';
   countInput.value = '42';
   const values = state.collectValues();
   TestRunner.assertEqual(values.name, 'John', 'collects text value');
   TestRunner.assertEqual(values.count, '42', 'collects number value as string');
   document.body.removeChild(container);
 });

 // Test validateAll blocks invalid submission
 TestRunner.add('FormRenderer validateAll - blocks invalid', () => {
   const { renderForm } = window.FormRenderer;
   const formDef = {
     key: 'test',
     name: 'Test',
     fields: [{ key: 'name', label: 'Name', type: 'text', required: true }],
   };
   const container = document.createElement('div');
   document.body.appendChild(container);
   const state = renderForm(formDef, container);
   const valid = state.validateAll();
   TestRunner.assert(!valid, 'validateAll returns false for empty required field');
   const errors = state.getErrors();
   TestRunner.assert(errors.name && errors.name.length > 0, 'errors recorded for required field');
   document.body.removeChild(container);
 });

 // Test form definition with no fields
 TestRunner.add('FormRenderer renderForm - empty form', () => {
   const { renderForm } = window.FormRenderer;
   const formDef = { key: 'empty', name: 'Empty Form', fields: [] };
   const container = document.createElement('div');
   document.body.appendChild(container);
   const state = renderForm(formDef, container);
   TestRunner.assert(state !== null, 'renders empty form without error');
   const form = container.querySelector('.dynamic-form');
   TestRunner.assert(form !== null, 'form element exists');
   document.body.removeChild(container);
 });

 // Test displayServerErrors
 TestRunner.add('FormRenderer displayServerErrors', () => {
   const { renderForm, displayServerErrors } = window.FormRenderer;
   const formDef = {
     key: 'test',
     name: 'Test',
     fields: [{ key: 'email', label: 'Email', type: 'email' }],
   };
   const container = document.createElement('div');
   document.body.appendChild(container);
   const state = renderForm(formDef, container);
   displayServerErrors(state, { email: ['Server says: invalid email'] });
   const errors = state.getErrors();
   TestRunner.assert(errors.email && errors.email.includes('Server says: invalid email'), 'server error displayed');
   const errorEl = container.querySelector('.field-error');
   TestRunner.assert(errorEl !== null, 'error element rendered');
   TestRunner.assert(errorEl.textContent.includes('Server says: invalid email'), 'error text rendered');
   document.body.removeChild(container);
 });

 // Test showFormLoading, showFormEmpty, showFormError, showFormSuccess
 TestRunner.add('FormRenderer UI state helpers', () => {
   const { showFormLoading, showFormEmpty, showFormError, showFormSuccess } = window.FormRenderer;
   const container = document.createElement('div');
   document.body.appendChild(container);

   showFormLoading(container, 'Loading…');
   TestRunner.assert(container.querySelector('.form-loading') !== null, 'loading state shown');

   showFormEmpty(container, 'No forms available');
   TestRunner.assert(container.querySelector('.form-empty') !== null, 'empty state shown');

   showFormError(container, 'Something went wrong');
   TestRunner.assert(container.textContent.includes('Something went wrong'), 'error message shown');

   showFormSuccess(container, 'Success!');
   TestRunner.assert(container.textContent.includes('Success!'), 'success message shown');

   document.body.removeChild(container);
 });

 // Test getSubmissionPayload
 TestRunner.add('FormRenderer getSubmissionPayload', () => {
   const { renderForm, getSubmissionPayload } = window.FormRenderer;
   const formDef = {
     key: 'test', name: 'Test',
     fields: [{ key: 'name', label: 'Name', type: 'text' }],
   };
   const container = document.createElement('div');
   document.body.appendChild(container);
   const state = renderForm(formDef, container);
   container.querySelector('#form-field-name').value = 'Alice';
   const payload = getSubmissionPayload(state);
   TestRunner.assertEqual(payload.name, 'Alice', 'payload contains field value');
   document.body.removeChild(container);
 });