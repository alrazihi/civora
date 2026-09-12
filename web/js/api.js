const API_BASE = '/api/v1';
let token = localStorage.getItem('civora_token') || '';
let orgId = localStorage.getItem('civora_org_id') || '';

function setAuth(t, o) {
  token = t;
  orgId = o;
  localStorage.setItem('civora_token', t);
  localStorage.setItem('civora_org_id', o);
}

function clearAuth() {
  token = '';
  orgId = '';
  localStorage.removeItem('civora_token');
  localStorage.removeItem('civora_org_id');
}

async function api(method, path, body) {
  const headers = { 'Content-Type': 'application/json' };
  if (token) headers['Authorization'] = `Bearer ${token}`;
  if ((method === 'POST' || method === 'PUT') && !headers['Idempotency-Key']) {
    headers['Idempotency-Key'] = crypto.randomUUID();
  }
  const res = await fetch(`${API_BASE}${path}`, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  });
  if (res.status === 401) { clearAuth(); router.navigate('login'); }
   if (!res.ok) {
    const errData = await res.json().catch(() => ({}));
    const msg = errData.error?.message || `HTTP ${res.status}`;
    const err = new Error(`${res.status} ${msg}`);
    err.status = res.status;
    err.data = errData;
    throw err;
  }
  return res.json().catch(() => ({}));
}

async function apiJSON(method, path, body) {
  return api(method, path, body);
}

function decodeJWT(t) {
  try {
    const parts = t.split('.');
    if (parts.length < 2) return null;
    const b64 = parts[1].replace(/-/g, '+').replace(/_/g, '/');
    return JSON.parse(atob(b64));
  } catch (e) {
    return null;
  }
}

function getCurrentUser() {
  if (!token) return null;
  const claims = decodeJWT(token);
  if (!claims) return null;
  return { id: claims.sub, organization_id: claims.organization_id, role: claims.role };
}

function currentUserIsAdmin() {
  const u = getCurrentUser();
  return !!(u && u.role === 'admin');
}

// ── Form API abstraction ──────────────────────────────────────────────
// These helpers provide a clean integration point for dynamic forms.
// The backend may eventually expose form definition and submission
// endpoints. Until then the API contract is isolated here so that
// switching to real endpoints requires changing only this module.

/**
 * Fetch the form definition required for the current workflow state
 * of a case.
 *
 * @param {string} caseId - The case/service-request ID.
 * @returns {Promise<object>} Form definition or null if no form is required.
 *   HTTP 404 → no form required at this state.
 *   HTTP 403 → user is not authorised to view the form.
 */
async function getRequiredForm(caseId) {
  const res = await api('GET', `/organizations/${orgId}/cases/${caseId}/workflow/form`);
  return res.data || null;
}

/**
 * Fetch an existing form submission (for editing).
 * Returns null on 404 (no submission yet).
 *
 * @param {string} caseId - The case/service-request ID.
 * @param {string} formKey - The form key/slug.
 * @returns {Promise<object|null>}
 */
async function getFormSubmission(caseId, formKey) {
  const res = await api('GET', `/organizations/${orgId}/cases/${caseId}/form/${formKey}/submission`);
  return res.data || null;
}

/**
 * Submit form values for a case at the current workflow state.
 *
 * @param {string} caseId - The case/service-request ID.
 * @param {object} values - Field values collected from the form.
 * @param {object} [opts] - Optional metadata.
 * @param {string} [opts.formKey] - Form key, if known.
 * @param {string} [opts.submissionId] - Existing submission ID for edits.
 * @returns {Promise<object>}
 */
async function submitForm(caseId, values, opts) {
  const body = { values: values || {} };
  if (opts && opts.formKey) body.form_key = opts.formKey;
  if (opts && opts.submissionId) body.submission_id = opts.submissionId;
  const path = opts && opts.submissionId
    ? `/organizations/${orgId}/cases/${caseId}/form/${opts.submissionId}/submission`
    : `/organizations/${orgId}/cases/${caseId}/form/submission`;
  const res = await api('POST', path, body);
  return res.data || null;
}

// ── Form Management API ───────────────────────────────────────────────
// These endpoints are the admin-facing contract for managing form
// definitions. They mirror the workflow-definition CRUD pattern.
// Until the backend implements these, 404 is expected for all calls.

/**
 * List form definitions for the organization.
 * @param {object} [opts] - Pagination and filter options.
 * @param {number} [opts.page] - Page number (1-based).
 * @param {number} [opts.per_page] - Results per page.
 * @param {string} [opts.search] - Search by name or key.
 * @param {string} [opts.status] - Filter by status (DRAFT, ACTIVE, ARCHIVED).
 * @returns {Promise<{data: object[], meta: object}>}
 */
async function listForms(opts = {}) {
  const params = new URLSearchParams();
  if (opts.page) params.set('page', String(opts.page));
  if (opts.per_page) params.set('per_page', String(opts.per_page));
  if (opts.search) params.set('search', opts.search);
  if (opts.status) params.set('status', opts.status);
  const qs = params.toString();
  const path = `/organizations/${orgId}/forms${qs ? `?${qs}` : ''}`;
  return api('GET', path);
}

/**
 * Get a single form definition by ID.
 * @param {string} formId
 * @returns {Promise<object>}
 */
async function getForm(formId) {
  const res = await api('GET', `/organizations/${orgId}/forms/${formId}`);
  return res.data || null;
}

/**
 * Create a new form definition (draft status).
 * @param {object} formData - The form definition.
 * @returns {Promise<object>}
 */
async function createForm(formData) {
  const res = await api('POST', `/organizations/${orgId}/forms`, formData);
  return res.data || null;
}

/**
 * Update a form definition.
 * Only drafts can be modified. Active/published forms return 409.
 * @param {string} formId
 * @param {object} formData - Updated form definition.
 * @returns {Promise<object>}
 */
async function updateForm(formId, formData) {
  const res = await api('PUT', `/organizations/${orgId}/forms/${formId}`, formData);
  return res.data || null;
}

/**
 * Delete a form definition. Only drafts can be deleted.
 * @param {string} formId
 */
async function deleteForm(formId) {
  await api('DELETE', `/organizations/${orgId}/forms/${formId}`);
}

/**
 * List form submissions for a case and form key.
 * @param {string} caseId
 * @param {string} formKey
 * @returns {Promise<object[]>}
 */
async function getFormSubmissions(caseId, formKey) {
  const encodedKey = encodeURIComponent(formKey);
  const res = await api('GET', `/organizations/${orgId}/cases/${caseId}/form/${encodedKey}/submissions`);
  return (res.data || []).filter(Boolean);
}

/**
 * Fetch submission status for all forms required at the current workflow state.
 * Returns a map of formKey -> submission object (or null if no submission).
 * Handles 404 gracefully (no submissions yet).
 * @param {string} caseId
 * @param {string} workflowState
 * @returns {Promise<object>}
 */
async function getCaseFormSubmissions(caseId, workflowState) {
  try {
    const res = await api('GET',
      `/organizations/${orgId}/cases/${caseId}/workflow/form-submissions` +
      (workflowState ? `?state=${encodeURIComponent(workflowState)}` : ''));
    return res.data || {};
  } catch (err) {
    if (err.message && err.message.includes('404')) {
      return {};
    }
    throw err;
  }
}

/**
 * Fetch details of a specific form submission.
 * @param {string} caseId
 * @param {string} formKey
 * @returns {Promise<object|null>}
 */
async function getCaseFormSubmission(caseId, formKey) {
  try {
    const encodedKey = encodeURIComponent(formKey);
    const res = await api('GET', `/organizations/${orgId}/cases/${caseId}/form/${encodedKey}/submission`);
    return res.data || null;
  } catch (err) {
    if (err.message && err.message.includes('404')) {
      return null;
    }
    throw err;
  }
}

/**
 * Assign a form to a workflow state.
 * @param {string} formId
 * @param {object} assignment - { workflow_id, state_key, required, display_order }
 * @returns {Promise<object>}
 */
async function assignFormToWorkflowState(formId, assignment) {
  const res = await api('POST', `/organizations/${orgId}/forms/${formId}/assignments`, assignment);
  return res.data || null;
}

async function getFormAssignments(formId) {
  const res = await api('GET', `/organizations/${orgId}/forms/${formId}/assignments`);
  return (res.data || []).filter(Boolean);
}

async function removeFormAssignment(formId, assignmentId) {
  await api('DELETE', `/organizations/${orgId}/forms/${formId}/assignments/${assignmentId}`);
}

// Exported so app.js can call these helpers.
if (typeof module !== 'undefined' && module.exports) {
  module.exports = { getRequiredForm, getFormSubmission, submitForm, listForms, getForm, createForm, updateForm, deleteForm, getFormSubmissions, getCaseFormSubmissions, getCaseFormSubmission, assignFormToWorkflowState, getFormAssignments, removeFormAssignment };
} else {
  // Browser global
  window.FormAPI = { getRequiredForm, getFormSubmission, submitForm, listForms, getForm, createForm, updateForm, deleteForm, getFormSubmissions, getCaseFormSubmissions, getCaseFormSubmission, assignFormToWorkflowState, getFormAssignments, removeFormAssignment };
}
