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

async function apiRaw(method, path, body) {
  const headers = {};
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
  return res;
}

function downloadBlob(blob, filename) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

async function triggerExport(opts = {}) {
  const res = await exportData(opts);
  const blob = await res.blob();
  const filename = res.headers.get('Content-Disposition')?.split('filename=')[1]?.replace(/"/g, '') || `civora-export-${Date.now()}`;
  downloadBlob(blob, filename);
}

async function apiJSON(method, path, body) {
  return api(method, path, body);
}

async function apiUpload(path, file, extraFields, responseType = 'json') {
  const formData = new FormData();
  formData.append('file', file);
  if (extraFields) {
    for (const [k, v] of Object.entries(extraFields)) {
      formData.append(k, v);
    }
  }
  const headers = {};
  if (token) headers['Authorization'] = `Bearer ${token}`;
  const res = await fetch(`${API_BASE}${path}`, {
    method: 'POST',
    headers: headers,
    body: formData,
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
  if (responseType === 'blob') return res.blob();
  return res.json().catch(() => ({}));
}

function apiUploadWithProgress(path, file, extraFields, onProgress) {
  return new Promise((resolve, reject) => {
    const formData = new FormData();
    formData.append('file', file);
    if (extraFields) {
      for (const [k, v] of Object.entries(extraFields)) {
        formData.append(k, v);
      }
    }
    const headers = {};
    if (token) headers['Authorization'] = `Bearer ${token}`;
    const xhr = new XMLHttpRequest();
    xhr.open('POST', `${API_BASE}${path}`);
    for (const [k, v] of Object.entries(headers)) {
      xhr.setRequestHeader(k, v);
    }
    xhr.upload.addEventListener('progress', (event) => {
      if (event.lengthComputable && onProgress) {
        onProgress((event.loaded / event.total) * 100);
      }
    });
    xhr.addEventListener('load', () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        const json = JSON.parse(xhr.responseText || '{}');
        resolve(json);
      } else if (xhr.status === 401) {
        clearAuth();
        router.navigate('login');
      } else {
        let errData = {};
        try { errData = JSON.parse(xhr.responseText); } catch (e) {}
        const msg = errData.error?.message || `HTTP ${xhr.status}`;
        const err = new Error(`${xhr.status} ${msg}`);
        err.status = xhr.status;
        err.data = errData;
        reject(err);
      }
    });
    xhr.addEventListener('error', () => reject(new Error('Upload failed')));
    xhr.addEventListener('abort', () => reject(new Error('Upload aborted')));
    xhr.send(formData);
  });
}

async function apiDownload(path) {
  const headers = {};
  if (token) headers['Authorization'] = `Bearer ${token}`;
  const res = await fetch(`${API_BASE}${path}`, {
    method: 'GET',
    headers: headers,
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
  return res;
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
  const res = await api('GET', `/organizations/${orgId}/cases/${caseId}/workflow/forms`);
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
 * Get the active (published) version of a form.
 * Falls back to the latest version if no published version exists.
 * @param {string} formId
 * @returns {Promise<object>}
 */
async function getActiveFormVersion(formId) {
  const res = await api('GET', `/organizations/${orgId}/forms/${formId}/active-version`);
  return res.data || null;
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
 * @param {string} formId - The form ID (sent in body for backend validation).
 * @param {string} formVersionId - The form version ID to assign.
 * @param {string} workflowId - The workflow definition ID.
 * @param {object} assignment - { state_key, required, display_order, active }
 * @returns {Promise<object>}
 */
async function assignFormToWorkflowState(formId, formVersionId, workflowId, assignment) {
  const body = {
    form_id: formId,
    form_version_id: formVersionId,
    workflow_state_key: assignment.state_key,
    required: assignment.required,
    display_order: assignment.display_order || 0,
    active: assignment.active !== undefined ? assignment.active : true,
  };
  const res = await api('POST', `/organizations/${orgId}/workflows/${workflowId}/form-assignments`, body);
  return res.data || null;
}

/**
 * List form assignments for a workflow.
 * @param {string} workflowId
 * @returns {Promise<object[]>}
 */
async function getFormAssignments(workflowId) {
  const res = await api('GET', `/organizations/${orgId}/workflows/${workflowId}/form-assignments`);
  return (res.data || []).filter(Boolean);
}

/**
 * Fetch all assignments across all workflows for a specific form.
 * The backend has no single endpoint for this, so we load all workflows
 * and aggregate their assignments, filtering for the target form.
 * @param {string} formId
 * @returns {Promise<object[]>}
 */
async function getFormAssignmentsByForm(formId) {
  try {
    const wfRes = await api('GET', `/organizations/${orgId}/workflows`);
    const workflows = wfRes.data || [];
    const allAssignments = [];
    for (const wf of workflows) {
      try {
        const assignments = await getFormAssignments(wf.id);
        for (const a of assignments) {
          if (a.form_id === formId) {
            allAssignments.push({ ...a, workflow_id: wf.id, workflow_name: wf.name, workflow_key: wf.key });
          }
        }
      } catch (e) {
        // Skip workflows we can't read assignments for
      }
    }
    return allAssignments;
  } catch (e) {
    return [];
  }
}

/**
 * Remove a form assignment from a workflow state.
 * @param {string} workflowId
 * @param {string} assignmentId
 */
async function removeFormAssignment(workflowId, assignmentId) {
  await api('DELETE', `/organizations/${orgId}/workflows/${workflowId}/form-assignments/${assignmentId}`);
}

// ── Rules Engine API ──────────────────────────────────────────────────

/**
 * List rule sets for the organization.
 * @param {object} [opts] - Pagination and filter options.
 * @param {number} [opts.page] - Page number (1-based).
 * @param {number} [opts.per_page] - Results per page.
 * @param {string} [opts.key] - Filter by key.
 * @param {string} [opts.status] - Filter by status (DRAFT, PUBLISHED, ARCHIVED).
 * @param {string} [opts.case_id] - Filter by case ID.
 * @returns {Promise<{data: object[], meta: object}>}
 */
async function listRuleSets(opts = {}) {
  const params = new URLSearchParams();
  if (opts.page) params.set('page', String(opts.page));
  if (opts.per_page) params.set('per_page', String(opts.per_page));
  if (opts.key) params.set('key', opts.key);
  if (opts.status) params.set('status', opts.status);
  if (opts.case_id) params.set('case_id', opts.case_id);
  const qs = params.toString();
  const path = `/organizations/${orgId}/rules/rule-sets${qs ? `?${qs}` : ''}`;
  return api('GET', path);
}

/**
 * Get a rule set by ID.
 * @param {string} ruleSetId
 * @returns {Promise<object>}
 */
async function getRuleSet(ruleSetId) {
  const res = await api('GET', `/organizations/${orgId}/rules/rule-sets/${ruleSetId}`);
  return res.data || null;
}

/**
 * Get a rule set by key.
 * @param {string} key
 * @returns {Promise<object>}
 */
async function getRuleSetByKey(key) {
  const res = await api('GET', `/organizations/${orgId}/rules/rule-sets/key/${encodeURIComponent(key)}`);
  return res.data || null;
}

/**
 * Create a new rule set (draft).
 * @param {object} ruleSetData
 * @returns {Promise<object>}
 */
async function createRuleSet(ruleSetData) {
  const res = await api('POST', `/organizations/${orgId}/rules/rule-sets`, ruleSetData);
  return res.data || null;
}

/**
 * Update a draft rule set.
 * @param {string} ruleSetId
 * @param {object} ruleSetData
 * @returns {Promise<object>}
 */
async function updateRuleSet(ruleSetId, ruleSetData) {
  const res = await api('PATCH', `/organizations/${orgId}/rules/rule-sets/${ruleSetId}`, ruleSetData);
  return res.data || null;
}

/**
 * Create a new version (clone) of a rule set.
 * @param {string} ruleSetId
 * @returns {Promise<object>}
 */
async function createRuleSetVersion(ruleSetId) {
  const res = await api('POST', `/organizations/${orgId}/rules/rule-sets/${ruleSetId}/version`);
  return res.data || null;
}

/**
 * Publish a draft rule set.
 * @param {string} ruleSetId
 * @returns {Promise<object>}
 */
async function publishRuleSet(ruleSetId) {
  const res = await api('POST', `/organizations/${orgId}/rules/rule-sets/${ruleSetId}/publish`);
  return res.data || null;
}

/**
 * Archive a published rule set.
 * @param {string} ruleSetId
 * @returns {Promise<object>}
 */
async function archiveRuleSet(ruleSetId) {
  const res = await api('POST', `/organizations/${orgId}/rules/rule-sets/${ruleSetId}/archive`);
  return res.data || null;
}

/**
 * Delete a draft rule set.
 * @param {string} ruleSetId
 */
async function deleteRuleSet(ruleSetId) {
  await api('DELETE', `/organizations/${orgId}/rules/rule-sets/${ruleSetId}`);
}

/**
 * List versions of a rule set.
 * @param {string} ruleSetId
 * @param {object} [opts] - Pagination options.
 * @returns {Promise<{data: object[], meta: object}>}
 */
async function listRuleSetVersions(ruleSetId, opts = {}) {
  const params = new URLSearchParams();
  if (opts.page) params.set('page', String(opts.page));
  if (opts.per_page) params.set('per_page', String(opts.per_page));
  const qs = params.toString();
  const path = `/organizations/${orgId}/rules/rule-sets/${ruleSetId}/versions${qs ? `?${qs}` : ''}`;
  return api('GET', path);
}

/**
 * Evaluate a rule set against provided facts.
 * @param {string} ruleSetId
 * @param {object} facts - The fact document.
 * @param {string} [caseId] - Optional case ID to attach the evaluation to.
 * @param {string} [trigger] - Trigger type (MANUAL or AUTOMATIC).
 * @returns {Promise<object>}
 */
async function evaluateRuleSet(ruleSetId, facts, caseId, trigger) {
  const body = { facts: facts || {} };
  if (caseId) body.case_id = caseId;
  if (trigger) body.trigger = trigger;
  const res = await api('POST', `/organizations/${orgId}/rules/rule-sets/${ruleSetId}/evaluate`, body);
  return res.data || null;
}

/**
 * List evaluations for a rule set.
 * @param {string} ruleSetId
 * @param {object} [opts] - Pagination options.
 * @returns {Promise<{data: object[], meta: object}>}
 */
async function listEvaluationsByRuleSet(ruleSetId, opts = {}) {
  const params = new URLSearchParams();
  if (opts.page) params.set('page', String(opts.page));
  if (opts.per_page) params.set('per_page', String(opts.per_page));
  const qs = params.toString();
  const path = `/organizations/${orgId}/rules/rule-sets/${ruleSetId}/evaluations${qs ? `?${qs}` : ''}`;
  return api('GET', path);
}

/**
 * Get a specific evaluation.
 * @param {string} evaluationId
 * @returns {Promise<object>}
 */
async function getEvaluation(evaluationId) {
  const res = await api('GET', `/organizations/${orgId}/rules/evaluations/${evaluationId}`);
  return res.data || null;
}

/**
 * List evaluations for a case.
 * @param {string} caseId
 * @param {object} [opts] - Pagination options.
 * @returns {Promise<{data: object[], meta: object}>}
 */
async function listEvaluationsByCase(caseId, opts = {}) {
  const params = new URLSearchParams();
  if (opts.page) params.set('page', String(opts.page));
  if (opts.per_page) params.set('per_page', String(opts.per_page));
  const qs = params.toString();
  const path = `/organizations/${orgId}/rules/cases/${caseId}/evaluations${qs ? `?${qs}` : ''}`;
  return api('GET', path);
}

/**
 * List discoverable form fields for fact-path suggestions.
 * @returns {Promise<object[]>}
 */
async function listDiscoverableFields() {
  const res = await api('GET', `/organizations/${orgId}/rules/fields`);
  return (res.data || []).filter(Boolean);
}

// ── Review Queue API ───────────────────────────────────────────────────

/**
 * List review queue entries for the organization.
 * @param {object} [opts]
 * @param {string} [opts.status]
 * @param {boolean} [opts.assigned_to_me]
 * @param {number} [opts.page]
 * @param {number} [opts.per_page]
 * @returns {Promise<{data: object[], meta: object}>}
 */
async function listReviewQueue(opts = {}) {
  const params = new URLSearchParams();
  if (opts.status) params.set('status', opts.status);
  if (opts.assigned_to_me) params.set('assigned_to_me', 'true');
  if (opts.page) params.set('page', String(opts.page));
  if (opts.per_page) params.set('per_page', String(opts.per_page));
  const qs = params.toString();
  const path = `/organizations/${orgId}/review-queue${qs ? `?${qs}` : ''}`;
  return api('GET', path);
}

/**
 * Get a single review queue entry.
 * @param {string} reviewId
 * @returns {Promise<object>}
 */
async function getReviewQueueEntry(reviewId) {
  const res = await api('GET', `/organizations/${orgId}/review-queue/${reviewId}`);
  return res.data || null;
}

/**
 * Claim a review queue entry.
 * @param {string} reviewId
 * @returns {Promise<object>}
 */
async function claimReview(reviewId) {
  const res = await api('POST', `/organizations/${orgId}/review-queue/${reviewId}/claim`);
  return res.data || null;
}

/**
 * Start reviewing a claim.
 * @param {string} reviewId
 * @returns {Promise<object>}
 */
async function startReview(reviewId) {
  const res = await api('POST', `/organizations/${orgId}/review-queue/${reviewId}/start`);
  return res.data || null;
}

/**
 * Complete a review with a human decision.
 * @param {string} reviewId
 * @param {string} decision
 * @param {string} [reason]
 * @returns {Promise<object>}
 */
async function completeReview(reviewId, decision, reason) {
  const res = await api('POST', `/organizations/${orgId}/review-queue/${reviewId}/complete`, { decision, reason });
  return res.data || null;
}

/**
 * Escalate a review for senior review.
 * @param {string} reviewId
 * @param {string} reason
 * @returns {Promise<object>}
 */
async function escalateReview(reviewId, reason) {
  const res = await api('POST', `/organizations/${orgId}/review-queue/${reviewId}/escalate`, { reason });
  return res.data || null;
}

/**
 * Request more information for a review.
 * @param {string} reviewId
 * @param {string[]} missingFields
 * @param {string} reason
 * @returns {Promise<object>}
 */
async function requestReviewInformation(reviewId, missingFields, reason) {
  const res = await api('POST', `/organizations/${orgId}/review-queue/${reviewId}/request-information`, { missing_fields: missingFields, reason });
  return res.data || null;
}

// ── AI Observations API ───────────────────────────────────────────────────

/**
 * Generate AI observations for an evidence item.
 * @param {string} evidenceId
 * @param {string[]} [types] - Observation types to request (SUMMARY, ENTITY_EXTRACTION, CLASSIFICATION, INCONSISTENCY)
 * @param {number} [maxTokens] - Max tokens for the response
 * @returns {Promise<object>} Generated observations result
 */
async function generateAIObservations(evidenceId, types, maxTokens) {
  const body = {};
  if (types && types.length) body.types = types;
  if (maxTokens) body.max_tokens = maxTokens;
  const res = await api('POST', `/organizations/${orgId}/evidence/${evidenceId}/ai/observations/generate`, body);
  return res.data || null;
}

/**
 * List AI observations for an evidence item.
 * @param {string} evidenceId
 * @param {object} [opts] - Pagination options.
 * @param {number} [opts.page] - Page number (1-based).
 * @param {number} [opts.per_page] - Results per page.
 * @returns {Promise<{data: object[], meta: object}>}
 */
async function listAIObservations(evidenceId, opts = {}) {
  const params = new URLSearchParams();
  if (opts.page) params.set('page', String(opts.page));
  if (opts.perPage) params.set('per_page', String(opts.perPage));
  const qs = params.toString();
  const path = `/organizations/${orgId}/evidence/${evidenceId}/ai/observations${qs ? `?${qs}` : ''}`;
  return api('GET', path);
}

/**
 * Accept an AI observation (human verification).
 * @param {string} evidenceId
 * @param {string} observationId
 * @param {string} [notes] - Optional reviewer notes
 * @returns {Promise<object>} The updated observation
 */
async function acceptAIObservation(evidenceId, observationId, notes) {
  const res = await api('POST', `/organizations/${orgId}/evidence/${evidenceId}/ai/observations/${observationId}/accept`, { notes });
  return res.data || null;
}

/**
 * Reject an AI observation (human verification).
 * @param {string} evidenceId
 * @param {string} observationId
 * @param {string} [notes] - Optional reviewer notes
 * @returns {Promise<object>} The updated observation
 */
async function rejectAIObservation(evidenceId, observationId, notes) {
  const res = await api('POST', `/organizations/${orgId}/evidence/${evidenceId}/ai/observations/${observationId}/reject`, { notes });
  return res.data || null;
}

// Exported so app.js can call these helpers.
if (typeof module !== 'undefined' && module.exports) {
  module.exports = { getRequiredForm, getFormSubmission, submitForm, listForms, getForm, createForm, updateForm, deleteForm, getActiveFormVersion, getFormSubmissions, getCaseFormSubmissions, getCaseFormSubmission, assignFormToWorkflowState, getFormAssignments, getFormAssignmentsByForm, removeFormAssignment, listRuleSets, getRuleSet, getRuleSetByKey, createRuleSet, updateRuleSet, createRuleSetVersion, publishRuleSet, archiveRuleSet, deleteRuleSet, listRuleSetVersions, evaluateRuleSet, listEvaluationsByRuleSet, getEvaluation, listEvaluationsByCase, listDiscoverableFields, listReviewQueue, getReviewQueueEntry, claimReview, startReview, completeReview, escalateReview, requestReviewInformation, generateAIObservations, listAIObservations, acceptAIObservation, rejectAIObservation };
} else {
  window.FormAPI = { getRequiredForm, getFormSubmission, submitForm, listForms, getForm, createForm, updateForm, deleteForm, getActiveFormVersion, getFormSubmissions, getCaseFormSubmissions, getCaseFormSubmission, assignFormToWorkflowState, getFormAssignments, getFormAssignmentsByForm, removeFormAssignment };
  window.RulesAPI = { listRuleSets, getRuleSet, getRuleSetByKey, createRuleSet, updateRuleSet, createRuleSetVersion, publishRuleSet, archiveRuleSet, deleteRuleSet, listRuleSetVersions, evaluateRuleSet, listEvaluationsByRuleSet, getEvaluation, listEvaluationsByCase, listDiscoverableFields };
  window.ReviewQueueAPI = { listReviewQueue, getReviewQueueEntry, claimReview, startReview, completeReview, escalateReview, requestReviewInformation };
  window.AIObservationsAPI = { generateAIObservations, listAIObservations, acceptAIObservation, rejectAIObservation };
}

async function getOperationsDashboard(opts = {}) {
  const params = new URLSearchParams();
  if (opts.period) params.set('period', opts.period);
  if (opts.bucket) params.set('bucket', opts.bucket);
  if (opts.workflow_key) params.set('workflow_key', opts.workflow_key);
  if (opts.status) params.set('status', opts.status);
  const qs = params.toString();
  const path = `/organizations/${orgId}/operations/metrics/dashboard${qs ? `?${qs}` : ''}`;
  return api('GET', path);
}

async function getAllOperationsMetrics(opts = {}) {
  const params = new URLSearchParams();
  if (opts.period) params.set('period', opts.period);
  if (opts.bucket) params.set('bucket', opts.bucket);
  const qs = params.toString();
  const path = `/organizations/${orgId}/operations/metrics${qs ? `?${qs}` : ''}`;
  return api('GET', path);
}

async function getWorkflowAnalysis(opts = {}) {
  const params = new URLSearchParams();
  if (opts.state_accumulation_threshold) params.set('state_accumulation_threshold', opts.state_accumulation_threshold);
  if (opts.state_duration_threshold_hours) params.set('state_duration_threshold_hours', opts.state_duration_threshold_hours);
  if (opts.aging_threshold_hours) params.set('aging_threshold_hours', opts.aging_threshold_hours);
  if (opts.review_backlog_threshold) params.set('review_backlog_threshold', opts.review_backlog_threshold);
  if (opts.info_request_frequency_per_case) params.set('info_request_frequency_per_case', opts.info_request_frequency_per_case);
  if (opts.cycle_time_threshold_hours) params.set('cycle_time_threshold_hours', opts.cycle_time_threshold_hours);
  const qs = params.toString();
  const path = `/organizations/${orgId}/operations/analysis/workflow${qs ? `?${qs}` : ''}`;
  return api('GET', path);
}

async function getAnalysisThresholds() {
  const path = `/organizations/${orgId}/operations/analysis/thresholds`;
  return api('GET', path);
}

async function getImpactReport(opts = {}) {
  const params = new URLSearchParams();
  if (opts.period) params.set('period', opts.period);
  if (opts.bucket) params.set('bucket', opts.bucket);
  const qs = params.toString();
  const path = `/organizations/${orgId}/operations/impact/report${qs ? `?${qs}` : ''}`;
  return api('GET', path);
}

async function exportData(opts = {}) {
  const params = new URLSearchParams();
  if (opts.format) params.set('format', opts.format);
  if (opts.scope) params.set('scope', opts.scope);
  if (opts.period) params.set('period', opts.period);
  if (opts.bucket) params.set('bucket', opts.bucket);
  if (opts.workflow_key) params.set('workflow_key', opts.workflow_key);
  if (opts.status) params.set('status', opts.status);
  const qs = params.toString();
  const path = `/organizations/${orgId}/operations/export${qs ? `?${qs}` : ''}`;
  return apiRaw('GET', path);
}

window.OperationsAPI = { getOperationsDashboard, getAllOperationsMetrics, getWorkflowAnalysis, getAnalysisThresholds, getImpactReport, exportData };
