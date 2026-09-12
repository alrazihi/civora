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

// Exported so app.js can call these helpers.
if (typeof module !== 'undefined' && module.exports) {
  module.exports = { getRequiredForm, getFormSubmission, submitForm };
} else {
  // Browser global
  window.FormAPI = { getRequiredForm, getFormSubmission, submitForm };
}
