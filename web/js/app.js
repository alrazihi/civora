const SERVICE_DOMAIN = {
  EMERGENCY: {
    label: 'Emergency Assistance',
    color: '#dc2626',
    icon: '🚨',
    sections: {
      eligibility: { title: 'Eligibility Check', hint: 'Verify immediate eligibility for emergency response.' },
      evidence: { title: 'Evidence Collection', hint: 'Gather ID, referral, and situational evidence quickly.' },
      assessment: { title: 'Needs Assessment', hint: 'Assess immediate needs (food, shelter, transport, safety).' },
      decision: { title: 'Approval Decision', hint: 'Fast-track decision for emergency response.' },
      assistance: { title: 'Emergency Assistance', hint: 'Deploy immediate assistance (food, shelter, transport, medical).' },
      followup: { title: 'Follow-up', hint: 'Schedule follow-up to verify ongoing safety and needs.' }
    },
    assistanceTypes: ['FOOD', 'SHELTER', 'TRANSPORT', 'MEDICAL', 'FINANCIAL', 'OTHER'],
    sectionActionStates: {
      eligibility: ['NEW', 'OPEN'],
      evidence: ['NEW', 'OPEN'],
      assessment: ['IN_REVIEW', 'ASSESSMENT'],
      decision: ['ASSESSMENT', 'DECISION_PENDING'],
      assistance: ['APPROVED', 'IN_PROGRESS'],
      followup: ['IN_PROGRESS', 'FOLLOW_UP']
    }
  },
  MEDICAL: {
    label: 'Medical Assistance',
    color: '#2563eb',
    icon: '🏥',
    sections: {
      eligibility: { title: 'Medical Eligibility', hint: 'Verify medical eligibility criteria.' },
      evidence: { title: 'Medical Evidence', hint: 'Collect medical records, referrals, and diagnosis documents.' },
      assessment: { title: 'Medical Assessment', hint: 'Assess medical needs, treatment plan, and urgency.' },
      decision: { title: 'Treatment Decision', hint: 'Decision on medical assistance coverage.' },
      assistance: { title: 'Medical Assistance', hint: 'Arrange medication, transport, treatment, and care.' },
      followup: { title: 'Medical Follow-up', hint: 'Monitor treatment progress and recovery.' }
    },
    assistanceTypes: ['MEDICAL', 'TRANSPORT', 'FINANCIAL', 'FOOD', 'SHELTER', 'OTHER'],
    sectionActionStates: {
      eligibility: ['NEW', 'OPEN'],
      evidence: ['NEW', 'OPEN'],
      assessment: ['IN_REVIEW', 'ASSESSMENT'],
      decision: ['ASSESSMENT', 'DECISION_PENDING'],
      assistance: ['APPROVED', 'IN_PROGRESS'],
      followup: ['IN_PROGRESS', 'FOLLOW_UP']
    }
  }
};

const DEFAULT_TERMINAL_STATES = ['REJECTED', 'CLOSED'];

function escapeHTML(str) {
  if (str == null) return '';
  return String(str)
    .replace(/&/g, '&')
    .replace(/</g, '<')
    .replace(/>/g, '>')
    .replace(/"/g, '"')
    .replace(/'/g, '&#039;');
}

function showToast(message, type = 'info') {
  const container = document.getElementById('toast-container') || (() => {
    const c = document.createElement('div');
    c.id = 'toast-container';
    c.style.cssText = 'position:fixed;bottom:24px;right:24px;z-index:1000;display:flex;flex-direction:column;gap:8px;';
    document.body.appendChild(c);
    return c;
  })();
  const toast = document.createElement('div');
  const bg = type === 'error' ? 'var(--danger)' : type === 'success' ? 'var(--success)' : 'var(--primary)';
  toast.style.cssText = `background:${bg};color:#fff;padding:12px 16px;border-radius:var(--radius);box-shadow:var(--shadow);max-width:320px;animation:slideIn 0.3s ease;`;
  toast.textContent = message;
  container.appendChild(toast);
  setTimeout(() => {
    toast.style.animation = 'slideOut 0.3s ease';
    setTimeout(() => toast.remove(), 300);
  }, 4000);
}

const styleEl = document.createElement('style');
styleEl.textContent = `
@keyframes slideIn { from { opacity:0; transform:translateX(100%); } to { opacity:1; transform:translateX(0); } }
@keyframes slideOut { from { opacity:1; transform:translateX(0); } to { opacity:0; transform:translateX(100%); } }
`;
document.head.appendChild(styleEl);

function getServiceDomain(serviceType) {
  return SERVICE_DOMAIN[serviceType] || null;
}

const app = {
  currentCase: null,
  currentWorkflow: null,

  orgPath(path) {
    return `/organizations/${orgId}${path}`;
  },

  init() {
    if (token) router.navigate('dashboard');
    else router.navigate('login');
    router.start();
  },

  async login(e) {
    e.preventDefault();
    const org = document.getElementById('login-org').value.trim();
    const email = document.getElementById('login-email').value.trim();
    const password = document.getElementById('login-password').value;
    try {
      const res = await api('POST', `/organizations/${org}/auth/login`, { email, password });
      setAuth(res.data.token, res.data.user.organization_id);
      showToast('Signed in successfully', 'success');
      router.navigate('dashboard');
    } catch (err) {
      showToast(err.message, 'error');
    }
  },

  async register(e) {
    e.preventDefault();
    const org = document.getElementById('reg-org').value.trim();
    const email = document.getElementById('reg-email').value.trim();
    const name = document.getElementById('reg-name').value.trim();
    const password = document.getElementById('reg-password').value;
    try {
      await api('POST', `/organizations/${org}/auth/register`, { email, name, password });
      document.getElementById('register-form').reset();
      document.getElementById('register-success').classList.remove('hidden');
      showToast('Registered successfully! You can now sign in.', 'success');
    } catch (err) {
      showToast(err.message, 'error');
    }
  },

  async createPerson(e) {
    e.preventDefault();
    const data = {
      first_name: document.getElementById('p-first').value.trim(),
      last_name: document.getElementById('p-last').value.trim(),
      preferred_language: document.getElementById('p-lang').value.trim() || 'en',
      date_of_birth: document.getElementById('p-dob').value || undefined,
      email: document.getElementById('p-email').value.trim() || undefined,
      phone: document.getElementById('p-phone').value.trim() || undefined,
      address: document.getElementById('p-address').value.trim() || undefined,
      city: document.getElementById('p-city').value.trim() || undefined,
    };
    try {
      const res = await api('POST', this.orgPath('/people'), data);
      document.getElementById('new-case-person-id').value = res.data.id;
      document.getElementById('person-result').textContent = `Person created: ${res.data.first_name} ${res.data.last_name}`;
      showToast('Person created successfully', 'success');
    } catch (err) {
      showToast(err.message, 'error');
    }
  },

  async createCase(e) {
    e.preventDefault();
    const data = {
      title: document.getElementById('c-title').value.trim(),
      description: document.getElementById('c-desc').value.trim(),
      service_type: document.getElementById('c-service').value || 'EMERGENCY',
      priority: document.getElementById('c-priority').value || 'HIGH',
      person_id: document.getElementById('new-case-person-id').value || undefined,
    };
    try {
      const res = await api('POST', this.orgPath('/cases'), data);
      showToast('Case created successfully', 'success');
      router.navigate('case', res.data.id);
    } catch (err) {
      showToast(err.message, 'error');
    }
  },

  async loadDashboard() {
    try {
      const res = await api('GET', this.orgPath('/cases?per_page=50'));
      const cases = res.data || [];
      const tbody = document.getElementById('case-table-body');
      if (!cases.length) { tbody.innerHTML = '<tr><td colspan="5" class="empty">No cases yet</td></tr>'; return; }
      tbody.innerHTML = cases.map(c => `
        <tr style="cursor:pointer" onclick="router.navigate('case','${c.id}')">
          <td>${escapeHTML(c.case_number)}</td>
          <td>${escapeHTML(c.title)}</td>
          <td><span class="badge ${(c.status || '').toLowerCase().replace('_','-')}">${escapeHTML(c.status)}</span></td>
          <td>${escapeHTML(c.service_type)}</td>
          <td>${escapeHTML(c.priority)}</td>
        </tr>
      `).join('');
    } catch (err) {
      console.error(err);
    }
  },

  async loadCase(id) {
    try {
      const res = await api('GET', this.orgPath(`/cases/${id}`));
      this.currentCase = res.data;
      document.getElementById('case-title').textContent = res.data.title;
      document.getElementById('case-number').textContent = res.data.case_number;
      document.getElementById('case-status').textContent = res.data.status;
      document.getElementById('case-status').className = `badge ${(res.data.status || '').toLowerCase().replace('_','-')}`;
      document.getElementById('case-service').textContent = res.data.service_type;
      document.getElementById('case-priority').textContent = res.data.priority;
      document.getElementById('case-desc').textContent = res.data.description || 'No description';

      this.renderServiceBanner(res.data.service_type);
      await this.loadWorkflow(id);
      this.renderWorkflowProgress();
      await this.loadCaseSections(id);
      this.renderSectionActions();
      this.renderActions();
      await this.loadTimeline(id);
    } catch (err) {
      showToast(err.message, 'error');
    }
  },

  async loadWorkflow(caseId) {
    try {
      const res = await api('GET', this.orgPath(`/cases/${caseId}/workflow`));
      this.currentWorkflow = res.data;
      if (this.currentWorkflow && this.currentWorkflow.instance) {
        const inst = this.currentWorkflow.instance;
        const def = this.currentWorkflow.definition;

        document.getElementById('workflow-label').classList.remove('hidden');
        document.getElementById('workflow-label').textContent = def ? `Workflow: ${def.name} (v${def.version || 1})` : '';

        const stateBadge = document.getElementById('workflow-state-badge');
        const stateName = document.getElementById('workflow-state-name');
        if (stateBadge && inst.current_state) {
          stateBadge.textContent = inst.current_state;
          stateBadge.className = `badge ${inst.current_state.toLowerCase().replace('_','-')}`;
          stateBadge.style.display = 'inline-block';
        }
        if (stateName && def) {
          const stateDef = def.states ? def.states.find(s => s.key === inst.current_state) : null;
          stateName.textContent = stateDef ? (stateDef.description || stateDef.name) : '';
        }
      } else {
        document.getElementById('workflow-label').classList.add('hidden');
        const stateBadge = document.getElementById('workflow-state-badge');
        if (stateBadge) stateBadge.style.display = 'none';
      }
    } catch (err) {
      this.currentWorkflow = null;
      document.getElementById('workflow-label').classList.add('hidden');
      const stateBadge = document.getElementById('workflow-state-badge');
      if (stateBadge) stateBadge.style.display = 'none';
    }
  },

  renderServiceBanner(serviceType) {
    const domain = getServiceDomain(serviceType);
    const banner = document.getElementById('service-banner');
    if (!banner) return;
    if (!domain) {
      banner.classList.add('hidden');
      return;
    }
    banner.classList.remove('hidden');
    banner.innerHTML = `<span class="service-icon">${domain.icon}</span> <span class="service-label">${escapeHTML(domain.label)}</span>`;
    banner.style.backgroundColor = domain.color + '15';
    banner.style.color = domain.color;
    banner.style.borderColor = domain.color + '40';
  },

  renderWorkflowProgress() {
    const container = document.getElementById('workflow-progress');
    if (!container || !this.currentWorkflow || !this.currentWorkflow.instance) {
      if (container) container.innerHTML = '';
      return;
    }
    const currentState = this.currentWorkflow.instance.current_state;
    const def = this.currentWorkflow.definition;
    if (!def || !def.states || !def.states.length) {
      container.innerHTML = '';
      return;
    }
    // Sort states by display_order from the workflow definition
    const sortedStates = [...def.states].sort((a, b) => (a.display_order || 0) - (b.display_order || 0));
    const stateKeys = sortedStates.map(s => s.key);
    const currentIndex = stateKeys.indexOf(currentState);
    if (currentIndex < 0) {
      container.innerHTML = '';
      return;
    }
    const isTerminal = DEFAULT_TERMINAL_STATES.includes(currentState);
    const steps = stateKeys.map((state, idx) => {
      let cls = 'workflow-step';
      if (idx < currentIndex) cls += ' completed';
      else if (idx === currentIndex) cls += ' active';
      else cls += ' pending';
      if (isTerminal && idx <= currentIndex) cls += ' completed';
      const stateDef = sortedStates[idx];
      const label = stateDef?.name || state;
      return `<div class="${cls}"><span class="step-label">${escapeHTML(label)}</span></div>`;
    });
    const connectors = stateKeys.slice(0, -1).map(() => '<div class="workflow-connector"></div>').join('');
    container.innerHTML = steps.join(connectors ? connectors : '');
  },

  renderSectionActions() {
    if (!this.currentCase) return;
    const state = this.currentWorkflow?.instance?.current_state;
    const domain = getServiceDomain(this.currentCase.service_type);
    const sectionActionStates = domain?.sectionActionStates || {};
    const containers = {
      eligibility: document.getElementById('sec-eligibility'),
      evidence: document.getElementById('sec-evidence'),
      assessment: document.getElementById('sec-assessment'),
      decision: document.getElementById('sec-decision'),
      assistance: document.getElementById('sec-assistance'),
      followup: document.getElementById('sec-followup')
    };
    for (const [name, el] of Object.entries(containers)) {
      if (!el) continue;
      el.querySelectorAll('.section-action-btn').forEach(btn => btn.remove());
      const hasData = el.querySelector('.empty') === null && !el.innerHTML.includes('Not yet recorded') && el.textContent.trim().length > 0;
      if (hasData) continue;
      const actionBtn = document.createElement('button');
      actionBtn.className = 'btn section-action-btn';
      const title = domain?.sections[name]?.title || name.charAt(0).toUpperCase() + name.slice(1);
      actionBtn.textContent = `Add ${title}`;
      actionBtn.style.marginBottom = '8px';
      const allowedStates = sectionActionStates[name] || [];
      if (!state || !allowedStates.includes(state)) {
        actionBtn.disabled = true;
        actionBtn.title = 'Not available in current workflow state';
      } else {
        actionBtn.onclick = () => this.showSectionForm(name);
      }
      el.insertBefore(actionBtn, el.firstChild);
    }
  },

  showSectionForm(name) {
    const map = {
      eligibility: 'eligibility-modal',
      evidence: 'evidence-modal',
      assessment: 'assessment-modal',
      decision: 'decision-modal',
      assistance: 'assistance-modal',
      followup: 'followup-modal'
    };
    const modalId = map[name];
    if (!modalId) return;
    if (name === 'assistance') {
      this.loadStaffOptions().then(() => this.showModal(modalId));
    } else {
      this.showModal(modalId);
    }
  },

  async loadCaseSections(id) {
    await this.loadSection('eligibility', this.orgPath(`/eligibilities/by-service-request/${id}`));
    await this.loadSection('evidence', this.orgPath(`/evidence/by-service-request/${id}`));
    await this.loadSection('assessment', this.orgPath(`/assessments/by-service-request/${id}`));
    await this.loadSection('decision', this.orgPath(`/decisions/by-service-request/${id}`));
    await this.loadSection('assistance', this.orgPath(`/assistance/by-service-request/${id}`));
    await this.loadSection('followup', this.orgPath(`/follow-ups/by-service-request/${id}`));
  },

  async loadSection(name, path) {
    try {
      const res = await api('GET', path);
      const el = document.getElementById(`sec-${name}`);
      if (!el) return;
      const data = res.data;
      if (!data) { el.innerHTML = '<p class="empty">Not yet recorded</p>'; return; }
      const domain = getServiceDomain(this.currentCase?.service_type);
      const hint = domain?.sections[name]?.hint;
      let hintHTML = hint ? `<p class="section-hint">${escapeHTML(hint)}</p>` : '';
      if (name === 'eligibility') {
        el.innerHTML = `${hintHTML}<strong>Result:</strong> ${escapeHTML(data.result)}<br><strong>Explanation:</strong> ${escapeHTML(data.explanation)}`;
      } else if (name === 'evidence') {
        if (Array.isArray(data)) {
          if (!data.length) { el.innerHTML = hintHTML + '<p class="empty">No evidence yet</p>'; return; }
          el.innerHTML = hintHTML + '<table><thead><tr><th>Type</th><th>Description</th></tr></thead><tbody>' +
            data.map(e => `<tr><td>${escapeHTML(e.type)}</td><td>${escapeHTML(e.description)}</td></tr>`).join('') +
            '</tbody></table>';
        } else {
          el.innerHTML = `${hintHTML}<strong>Type:</strong> ${escapeHTML(data.type)}<br><strong>Description:</strong> ${escapeHTML(data.description)}`;
        }
      } else if (name === 'assessment') {
        if (Array.isArray(data)) {
          if (!data.length) { el.innerHTML = hintHTML + '<p class="empty">No assessment yet</p>'; return; }
          const a = data[0];
          el.innerHTML = `${hintHTML}<strong>Findings:</strong> ${escapeHTML(a.findings)}<br><strong>Needs:</strong> ${escapeHTML(a.needs_identified || 'N/A')}<br><strong>Recommendation:</strong> ${escapeHTML(a.recommendation)}`;
        } else {
          el.innerHTML = `${hintHTML}<strong>Findings:</strong> ${escapeHTML(data.findings)}<br><strong>Needs:</strong> ${escapeHTML(data.needs_identified || 'N/A')}<br><strong>Recommendation:</strong> ${escapeHTML(data.recommendation)}`;
        }
      } else if (name === 'decision') {
        if (Array.isArray(data)) {
          if (!data.length) { el.innerHTML = hintHTML + '<p class="empty">No decision yet</p>'; return; }
          const d = data[0];
          el.innerHTML = `${hintHTML}<strong>Decision:</strong> <span class="badge ${d.decision.toLowerCase().replace('_','-')}">${escapeHTML(d.decision)}</span><br><strong>Reason:</strong> ${escapeHTML(d.reason)}`;
        } else {
          el.innerHTML = `${hintHTML}<strong>Decision:</strong> <span class="badge ${data.decision.toLowerCase().replace('_','-')}">${escapeHTML(data.decision)}</span><br><strong>Reason:</strong> ${escapeHTML(data.reason)}`;
        }
      } else if (name === 'assistance') {
        if (Array.isArray(data)) {
          if (!data.length) { el.innerHTML = hintHTML + '<p class="empty">No assistance yet</p>'; return; }
          el.innerHTML = hintHTML + data.map(a => {
            const statusClass = (a.status || '').toLowerCase().replace('_','-');
            return `<div><strong class="badge ${statusClass}">${escapeHTML(a.type)}</strong> - <span class="badge assistance-status ${statusClass}">${escapeHTML(a.status)}</span><br>${escapeHTML(a.description)}</div>`;
          }).join('');
        } else {
          const statusClass = (data.status || '').toLowerCase().replace('_','-');
          el.innerHTML = `${hintHTML}<strong class="badge ${statusClass}">${escapeHTML(data.type)}</strong> - <span class="badge assistance-status ${statusClass}">${escapeHTML(data.status)}</span><br>${escapeHTML(data.description)}`;
        }
      } else if (name === 'followup') {
        if (Array.isArray(data)) {
          if (!data.length) { el.innerHTML = hintHTML + '<p class="empty">No follow-up yet</p>'; return; }
          el.innerHTML = hintHTML + data.map(f => `<div><strong>${escapeHTML(f.scheduled_date)}</strong> - ${escapeHTML(f.outcome)}</div>`).join('');
        } else {
          el.innerHTML = `${hintHTML}<strong>${escapeHTML(data.scheduled_date)}</strong> - ${escapeHTML(data.outcome)}`;
        }
      }
    } catch (err) {
      const el = document.getElementById(`sec-${name}`);
      if (el) {
        if (err.message && err.message.includes('404')) {
          el.innerHTML = '<p class="empty">Not yet recorded</p>';
        } else {
          el.innerHTML = `<p style="color:var(--danger)">Error loading: ${escapeHTML(err.message)}</p>`;
        }
      }
    }
  },

  btn(label, onclick) {
    return `<button class="btn" onclick="app.${onclick}">${label}</button>`;
  },

  async loadWorkflowTransitions(caseId) {
    try {
      const res = await api('GET', this.orgPath(`/cases/${caseId}/workflow/transitions`));
      return res.data || [];
    } catch (err) {
      return [];
    }
  },

  renderActions() {
    const container = document.getElementById('case-actions');
    if (!container || !this.currentCase) return;
    this.loadWorkflowTransitions(this.currentCase.id).then(transitions => {
      let html = '';
      if (transitions.length) {
        html += '<div style="display:flex;gap:8px;flex-wrap:wrap;">';
        for (const t of transitions) {
          html += this.btn(t.name || t.key, `workflowTransition('${t.key}')`);
        }
        html += '</div>';
      }
      const state = this.currentWorkflow?.instance?.current_state;
      if (state) {
        const isTerminal = TERMINAL_STATES.includes(state);
        if (isTerminal) {
          html += `<p class="empty" style="margin-top:8px;">This case is in a terminal state (<strong>${escapeHTML(state)}</strong>) and no further workflow actions are available.</p>`;
        } else if (!transitions.length) {
          html += `<p class="empty" style="margin-top:8px;">No actions available from the current state. The workflow may require conditions to be met.</p>`;
        }
      } else {
        html += '<p class="empty" style="margin-top:8px;">No workflow instance available for this case.</p>';
      }
      container.innerHTML = html;
    });
  },

  async loadTimeline(id) {
    const container = document.getElementById('timeline');
    if (!container) return;
    try {
      const workflowRes = await api('GET', this.orgPath(`/cases/${id}/workflow/history`)).catch(() => ({ data: [] }));
      const workflowEvents = (workflowRes.data || []).map(ev => ({
        ...ev,
        action: ev.transition_key ? `workflow.transition` : ev.action,
        metadata: {
          ...ev.metadata,
          from: ev.from_state,
          to: ev.to_state,
          transition: ev.transition_key,
        },
        timestamp: ev.occurred_at,
      }));
      const events = workflowEvents.sort((a, b) => new Date(a.timestamp) - new Date(b.timestamp));
      if (!events.length) {
        container.innerHTML = '<p class="empty">No workflow history yet</p>';
        return;
      }
      container.innerHTML = events.map(ev => {
        let label = escapeHTML(ev.action);
        if (ev.metadata) {
          if (ev.metadata.to) label += ` → ${escapeHTML(ev.metadata.to)}`;
          if (ev.metadata.from) label += ` from ${escapeHTML(ev.metadata.from)}`;
        }
        if (ev.transition_key) label += ` <span class="timeline-transition">(${escapeHTML(ev.transition_key)})</span>`;
        if (ev.reason) label += `<div class="timeline-reason">${escapeHTML(ev.reason)}</div>`;
        const time = new Date(ev.timestamp).toLocaleString();
        return `<div class="timeline-item"><div class="timeline-title">${label}</div><div class="timeline-meta">${time}</div></div>`;
      }).join('');
    } catch (err) {
      container.innerHTML = '<p class="empty">Unable to load timeline</p>';
    }
  },

  async workflowTransition(transitionKey) {
    if (!this.currentCase) return;
    try {
      await api('POST', this.orgPath(`/cases/${this.currentCase.id}/workflow/transitions/${transitionKey}`), {});
      showToast('Transition executed successfully', 'success');
      await this.loadCase(this.currentCase.id);
    } catch (err) {
      showToast(err.message, 'error');
    }
  },

  showEligibilityForm() { this.showModal('eligibility-modal'); },
  showEvidenceForm() { this.showModal('evidence-modal'); },
  showAssessmentForm() { this.showModal('assessment-modal'); },
  showDecisionForm() { this.showModal('decision-modal'); },
  showAssistanceForm() { this.loadStaffOptions().then(() => { this.filterAssistanceTypes(); this.showModal('assistance-modal'); }); },

  filterAssistanceTypes() {
    const select = document.getElementById('asst-type');
    if (!select || !this.currentCase) return;
    const domain = getServiceDomain(this.currentCase.service_type);
    const preferred = domain ? domain.assistanceTypes : null;
    if (!select._originalOptions) {
      select._originalOptions = Array.from(select.options).map(o => ({ value: o.value, text: o.textContent }));
    }
    const original = select._originalOptions;
    select.innerHTML = '';
    const seen = new Set();
    const sorted = preferred ? [...preferred] : original.map(o => o.value);
    for (const val of sorted) {
      if (!seen.has(val)) {
        seen.add(val);
        const opt = document.createElement('option');
        opt.value = val;
        opt.textContent = val;
        select.appendChild(opt);
      }
    }
    for (const o of original) {
      if (!seen.has(o.value)) {
        seen.add(o.value);
        const opt = document.createElement('option');
        opt.value = o.value;
        opt.textContent = o.text;
        select.appendChild(opt);
      }
    }
  },
  showFollowUpForm() { this.showModal('followup-modal'); },

  showModal(id) { document.getElementById(id)?.classList.remove('hidden'); },
  hideModal(id) { document.getElementById(id)?.classList.add('hidden'); },

  async loadStaffOptions() {
    const select = document.getElementById('asst-staff');
    if (!select) return;
    try {
      const res = await api('GET', this.orgPath('/users?per_page=200'));
      const users = res.data || [];
      select.innerHTML = '<option value="">Select staff...</option>' +
        users.map(u => `<option value="${u.id}">${escapeHTML(u.name)} (${escapeHTML(u.email)})</option>`).join('');
    } catch (err) {
      select.innerHTML = '<option value="">Failed to load users</option>';
    }
  },

  async submitEligibility(e) {
    e.preventDefault();
    try {
      await api('POST', this.orgPath('/eligibilities'), {
        service_request_id: this.currentCase.id,
        criteria: { manual: true },
        explanation: document.getElementById('elig-explanation').value,
      });
      this.hideModal('eligibility-modal');
      await this.loadCaseSections(this.currentCase.id);
      this.renderSectionActions();
      this.renderActions();
    } catch (err) { showToast(err.message, 'error'); }
  },

  async submitEvidence(e) {
    e.preventDefault();
    try {
      await api('POST', this.orgPath('/evidence'), {
        service_request_id: this.currentCase.id,
        type: document.getElementById('ev-type').value,
        description: document.getElementById('ev-desc').value,
        storage_reference: document.getElementById('ev-ref').value,
      });
      this.hideModal('evidence-modal');
      await this.loadCaseSections(this.currentCase.id);
      this.renderSectionActions();
      this.renderActions();
    } catch (err) { showToast(err.message, 'error'); }
  },

  async submitAssessment(e) {
    e.preventDefault();
    try {
      await api('POST', this.orgPath('/assessments'), {
        service_request_id: this.currentCase.id,
        findings: document.getElementById('as-findings').value,
        needs_identified: document.getElementById('as-needs').value,
        recommendation: document.getElementById('as-rec').value,
      });
      this.hideModal('assessment-modal');
      await this.loadCaseSections(this.currentCase.id);
      this.renderSectionActions();
      this.renderActions();
    } catch (err) { showToast(err.message, 'error'); }
  },

  async submitDecision(e) {
    e.preventDefault();
    try {
      await api('POST', this.orgPath('/decisions'), {
        service_request_id: this.currentCase.id,
        decision: document.getElementById('dec-decision').value,
        reason: document.getElementById('dec-reason').value,
      });
      this.hideModal('decision-modal');
      await this.loadCaseSections(this.currentCase.id);
      this.renderSectionActions();
      this.renderActions();
    } catch (err) { showToast(err.message, 'error'); }
  },

  async submitAssistance(e) {
    e.preventDefault();
    try {
      await api('POST', this.orgPath('/assistance'), {
        service_request_id: this.currentCase.id,
        type: document.getElementById('asst-type').value,
        description: document.getElementById('asst-desc').value,
        responsible_staff: document.getElementById('asst-staff').value,
      });
      this.hideModal('assistance-modal');
      await this.loadCaseSections(this.currentCase.id);
      this.renderSectionActions();
      this.renderActions();
    } catch (err) { showToast(err.message, 'error'); }
  },

  async submitFollowUp(e) {
    e.preventDefault();
    try {
      await api('POST', this.orgPath('/follow-ups'), {
        service_request_id: this.currentCase.id,
        scheduled_date: document.getElementById('fu-date').value,
        outcome: document.getElementById('fu-outcome').value,
        notes: document.getElementById('fu-notes').value,
      });
      this.hideModal('followup-modal');
      await this.loadCaseSections(this.currentCase.id);
      this.renderSectionActions();
      this.renderActions();
    } catch (err) { showToast(err.message, 'error'); }
  },

  logout() {
    clearAuth();
    router.navigate('login');
  }
};

const router = {
  routes: {},
  current: null,
  on(path, fn) { this.routes[path] = fn; },
  start() {
    window.addEventListener('hashchange', () => this.resolve());
    this.resolve();
  },
  navigate(path, param) {
    const hash = param ? `${path}/${param}` : path;
    window.location.hash = hash;
  },
  resolve() {
    const hash = window.location.hash.replace('#', '') || 'login';
    const [path, param] = hash.split('/');
    const fn = this.routes[path];
    if (fn) { fn(param); this.current = path; }
    else this.routes['login']();
  }
};

router.on('login', () => {
  document.getElementById('view-login').classList.remove('hidden');
  document.getElementById('view-dashboard').classList.add('hidden');
  document.getElementById('view-case').classList.add('hidden');
  document.getElementById('view-new-case').classList.add('hidden');
});

router.on('dashboard', async () => {
  document.getElementById('view-login').classList.add('hidden');
  document.getElementById('view-dashboard').classList.remove('hidden');
  document.getElementById('view-case').classList.add('hidden');
  document.getElementById('view-new-case').classList.add('hidden');
  await app.loadDashboard();
});

router.on('case', async (id) => {
  if (!id) { router.navigate('dashboard'); return; }
  document.getElementById('view-login').classList.add('hidden');
  document.getElementById('view-dashboard').classList.add('hidden');
  document.getElementById('view-case').classList.remove('hidden');
  document.getElementById('view-new-case').classList.add('hidden');
  await app.loadCase(id);
});

router.on('new-case', () => {
  document.getElementById('view-login').classList.add('hidden');
  document.getElementById('view-dashboard').classList.add('hidden');
  document.getElementById('view-case').classList.add('hidden');
  document.getElementById('view-new-case').classList.remove('hidden');
});
