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

function getTerminalStates(def) {
  if (!def || !Array.isArray(def.states)) return [];
  return def.states.filter(s => s.terminal).map(s => s.key);
}

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
  workflowDraft: null,

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
         document.getElementById('workflow-label').textContent = def ? `Workflow: ${def.name} (${def.key}, v${def.version || 1}) — ${def.status || 'DRAFT'}` : '';

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
    const terminalStates = getTerminalStates(def);
    const isTerminal = terminalStates.includes(currentState);
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
        const terminalStates = getTerminalStates(this.currentWorkflow?.definition);
        const isTerminal = terminalStates.includes(state);
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

  // ---- Workflow administration API helpers ----
  wfList(page) { return api('GET', this.orgPath(`/workflows?page=${page || 1}&per_page=50`)); },
  wfGet(id) { return api('GET', this.orgPath(`/workflows/${id}`)); },
  wfCreate(body) { return api('POST', this.orgPath('/workflows'), body); },
  wfActivate(id) { return api('POST', this.orgPath(`/workflows/${id}/activate`), {}); },
  wfArchive(id) { return api('POST', this.orgPath(`/workflows/${id}/archive`), {}); },

  // ---- Workflow administration views ----
  resetWorkflowDraft() {
    this.workflowDraft = {
      key: '',
      name: '',
      description: '',
      version: 1,
      initial_state: '',
      states: [],
      transitions: [],
    };
  },

  switchView(viewId) {
    const views = ['view-login', 'view-dashboard', 'view-case', 'view-new-case', 'view-workflows', 'view-workflow-detail', 'view-new-workflow'];
    views.forEach(id => {
      const el = document.getElementById(id);
      if (el) el.classList.toggle('hidden', id !== viewId);
    });
  },

  showWorkflowsView() {
    this.switchView('view-workflows');
    this.loadWorkflowList(1);
  },

  async loadWorkflowList(page) {
    page = page || 1;
    this.workflowListPage = page;
    const tbody = document.getElementById('wf-table-body');
    const metaEl = document.getElementById('wf-page-meta');
    if (tbody) tbody.innerHTML = '<tr><td colspan="7" class="empty">Loading workflows…</td></tr>';
    this.renderWorkflowListActions();
    try {
      const res = await this.wfList(page);
      const defs = res.data || [];
      const totalPages = (res.meta && res.meta.total_pages) || 1;
      const total = (res.meta && res.meta.total) || defs.length;
      if (!defs.length) {
        tbody.innerHTML = '<tr><td colspan="7" class="empty">No workflow definitions yet. Create one to get started.</td></tr>';
      } else {
        tbody.innerHTML = defs.map(d => this.renderWorkflowRow(d)).join('');
      }
      if (metaEl) metaEl.textContent = `Page ${page} of ${totalPages} (${total} definition${total === 1 ? '' : 's'})`;
      this.renderWorkflowPagination(page, totalPages);
    } catch (err) {
      if (tbody) tbody.innerHTML = `<tr><td colspan="7" class="empty">Error loading workflows: ${escapeHTML(err.message)}</td></tr>`;
      if (metaEl) metaEl.textContent = '';
      const pag = document.getElementById('wf-pagination');
      if (pag) pag.innerHTML = '';
    }
  },

  renderWorkflowListActions() {
    const actions = document.getElementById('wf-list-actions');
    if (!actions) return;
    actions.innerHTML = currentUserIsAdmin()
      ? `<button class="btn" onclick="app.showWorkflowCreateView()">Create Workflow</button>`
      : '';
  },

  renderWorkflowRow(d) {
    const states = d.states || [];
    const transitions = d.transitions || [];
    const terminal = getTerminalStates(d);
    const statusClass = (d.status || 'DRAFT').toLowerCase();
    const badge = `<span class="badge wf-status-${statusClass}" style="text-transform:none">${escapeHTML(d.status)}</span>`;
    const canActivate = d.status === 'DRAFT';
    const canArchive = d.status === 'ACTIVE';
    let actionBtns = `<button class="btn secondary" style="font-size:0.8rem;padding:4px 8px" onclick="app.showWorkflowDetail('${d.id}')">View</button>`;
    if (canActivate) {
      actionBtns += `<button class="btn" style="font-size:0.8rem;padding:4px 8px;margin-left:4px" onclick="app.activateWorkflow('${d.id}')">Activate</button>`;
    }
    if (canArchive) {
      actionBtns += `<button class="btn warning" style="font-size:0.8rem;padding:4px 8px;margin-left:4px" onclick="app.archiveWorkflow('${d.id}')">Archive</button>`;
    }
    return `<tr>
      <td>${escapeHTML(d.name)} <span class="text-muted" style="font-size:0.8rem">(${escapeHTML(d.key)})</span></td>
      <td>${escapeHTML(d.description || '')}</td>
      <td>${d.version || 1}</td>
      <td>${badge}</td>
      <td>${states.length}</td>
      <td>${transitions.length}</td>
      <td>${d.initial_state ? escapeHTML(d.initial_state) : '<span class="text-muted">—</span>'} ${terminal.length ? '(terminal: ' + escapeHTML(terminal.join(', ')) + ')' : ''}</td>
      <td style="white-space:nowrap;font-size:0.8rem;color:var(--text-muted)">${d.created_at ? new Date(d.created_at).toLocaleDateString() : '—'}<br>${d.updated_at ? new Date(d.updated_at).toLocaleDateString() : '—'}</td>
      <td>${actionBtns}</td>
    </tr>`;
  },

  renderWorkflowPagination(page, totalPages) {
    const el = document.getElementById('wf-pagination');
    if (!el) return;
    if (totalPages <= 1) { el.innerHTML = ''; return; }
    const max = 5;
    let p = Math.max(1, page - Math.floor(max / 2));
    const end = Math.min(totalPages, p + max - 1);
    p = Math.max(1, end - max + 1);
    let html = '';
    for (let i = p; i <= end; i++) {
      if (i === page) html += `<span class="badge" style="margin-right:4px">${i}</span>`;
      else html += `<button class="btn secondary" style="font-size:0.8rem;padding:4px 8px;margin-right:4px" onclick="app.loadWorkflowList(${i})">${i}</button>`;
    }
    el.innerHTML = html;
  },

  async activateWorkflow(id) {
    if (!confirm('Activate this workflow definition? Only draft definitions can be activated, and there can only be one active version per key.')) return;
    try {
      await this.wfActivate(id);
      showToast('Workflow definition activated', 'success');
      this.refreshWorkflowLocation(id);
    } catch (err) {
      showToast(err.message, 'error');
    }
  },

  async archiveWorkflow(id) {
    if (!confirm('Archive this workflow definition? It cannot be archived while active instances are running, and it cannot be un-archived.')) return;
    try {
      await this.wfArchive(id);
      showToast('Workflow definition archived', 'success');
      this.refreshWorkflowLocation(id);
    } catch (err) {
      showToast(err.message, 'error');
    }
  },

  refreshWorkflowLocation(id) {
    if (this.currentWorkflowDef && this.currentWorkflowDef.id === id) {
      this.showWorkflowDetail(id);
    }
    this.loadWorkflowList(this.workflowListPage || 1);
  },

  async showWorkflowDetail(id) {
    this.switchView('view-workflow-detail');
    const container = document.getElementById('wf-detail-container');
    if (!container) return;
    container.innerHTML = '<p class="empty">Loading workflow definition…</p>';
    this.currentWorkflowDef = null;
    try {
      const res = await this.wfGet(id);
      this.currentWorkflowDef = res.data;
      container.innerHTML = '';
      this.renderWorkflowDetail(this.currentWorkflowDef);
    } catch (err) {
      container.innerHTML = `<p class="empty" style="color:var(--danger)">Error loading workflow: ${escapeHTML(err.message)}</p>`;
    }
  },

  renderWorkflowDetail(def) {
    const container = document.getElementById('wf-detail-container');
    if (!container) return;
    const terminal = getTerminalStates(def);
    const states = def.states || [];
    const transitions = def.transitions || [];
    const isDraft = def.status === 'DRAFT';
    const isActive = def.status === 'ACTIVE';
    const isArchived = def.status === 'ARCHIVED';

    const statusBadge = (status, text) => `<span class="badge wf-status-${status.toLowerCase()}" style="text-transform:none">${escapeHTML(text)}</span>`;
    let actions = `<button class="btn secondary" style="font-size:0.8rem" onclick="app.showWorkflowsView()">Back to Workflows</button>`;
    if (isDraft) {
      actions += ` <button class="btn" style="font-size:0.8rem" onclick="app.activateWorkflow('${def.id}')">Activate</button>`;
    } else if (isActive) {
      actions += ` <button class="btn warning" style="font-size:0.8rem" onclick="app.archiveWorkflow('${def.id}')">Archive</button>`;
    }
    const badge = statusBadge(def.status, def.status);

    container.innerHTML = `
      <div class="header" style="margin-bottom:8px">
        <div><h1>${escapeHTML(def.name)}</h1>
        <p style="color:var(--text-muted);margin-top:4px">${badge} v${def.version || 1} · key: ${escapeHTML(def.key)}</p></div>
        <nav>${actions}</nav>
      </div>
      <div class="card">
        <h2>Workflow Metadata</h2>
        <table>
          <tr><th>Key</th><td>${escapeHTML(def.key)}</td></tr>
          <tr><th>Name</th><td>${escapeHTML(def.name)}</td></tr>
          <tr><th>Description</th><td>${escapeHTML(def.description || '')}</td></tr>
          <tr><th>Version</th><td>${def.version || 1}</td></tr>
          <tr><th>Status</th><td>${badge}</td></tr>
          <tr><th>Initial State</th><td>${escapeHTML(def.initial_state || '')}</td></tr>
          <tr><th>Terminal States</th><td>${terminal.length ? escapeHTML(terminal.join(', ')) : '<span class="text-muted">none</span>'}</td></tr>
          <tr><th>States</th><td>${states.length}</td></tr>
          <tr><th>Transitions</th><td>${transitions.length}</td></tr>
          <tr><th>Created</th><td>${def.created_at ? new Date(def.created_at).toLocaleString() : '—'}</td></tr>
          <tr><th>Updated</th><td>${def.updated_at ? new Date(def.updated_at).toLocaleString() : '—'}</td></tr>
          <tr><th>Lifecycle</th><td>
            ${isDraft ? '<span class="badge wf-status-draft">Draft — not yet in use</span>' : ''}
            ${isActive ? '<span class="badge wf-status-active" style="background:#dcfce7;color:#15803d">Active — governs new cases</span>' : ''}
            ${isArchived ? '<span class="badge" style="background:#f1f5f9;color:#475569">Archived — read-only</span>' : ''}
          </td></tr>
        </table>
        ${!currentUserIsAdmin() && isDraft ? '<p class="text-muted" style="margin-top:8px">Only administrators can activate draft workflows.</p>' : ''}
      </div>
      <div class="card">
        <h2>States (${states.length})</h2>
        ${states.length ? '<div class="table-wrap">' + this.renderWorkflowStateTable(states, terminal, def.initial_state) + '</div>' : '<p class="empty">No states defined.</p>'}
      </div>
      <div class="card">
        <h2>Transitions (${transitions.length})</h2>
        ${transitions.length ? '<div class="table-wrap">' + this.renderWorkflowTransitionTable(transitions) + '</div>' : '<p class="empty">No transitions defined.</p>'}
      </div>
    `;
    if (isArchived) {
      const draftBanner = document.createElement('div');
      draftBanner.className = 'card';
      draftBanner.style.background = 'var(--bg)';
      draftBanner.innerHTML = '<p style="color:var(--text-muted)">This workflow definition is archived and cannot be modified or re-activated from the UI. To change it, create a new version (same key) and activate it.</p>';
      container.appendChild(draftBanner);
    }
  },

  renderWorkflowStateTable(states, terminal, initial) {
    return `<table><thead><tr><th>Key</th><th>Name</th><th>Description</th><th>Category</th><th>Initial</th><th>Terminal</th><th>Display Order</th><th>Responsible Role</th></tr></thead><tbody>` +
      states.map(s => `
        <tr>
          <td><code>${escapeHTML(s.key)}</code></td>
          <td>${escapeHTML(s.name || '')}</td>
          <td>${escapeHTML(s.description || '')}</td>
          <td>${escapeHTML(s.category || '')}</td>
          <td>${s.key === initial ? '<span class="badge" style="background:#dbeafe;color:#1e40af">initial</span>' : ''}</td>
          <td>${s.terminal ? '<span class="badge" style="background:#fee2e2;color:#991b1b">terminal</span>' : ''}</td>
          <td>${s.display_order ?? ''}</td>
          <td>${escapeHTML(s.responsible_role || '')}</td>
        </tr>`).join('') +
      '</tbody></table>';
  },

  renderWorkflowTransitionTable(transitions) {
    return `<table><thead><tr><th>Key</th><th>Name</th><th>Source</th><th>Destination</th><th>Active</th><th>Allowed Roles</th><th>Conditions</th><th>Description</th></tr></thead><tbody>` +
      transitions.map(t => `
        <tr>
          <td><code>${escapeHTML(t.key)}</code></td>
          <td>${escapeHTML(t.name || '')}</td>
          <td><code>${escapeHTML(t.from_state)}</code></td>
          <td><code>${escapeHTML(t.to_state)}</code></td>
          <td>${t.active ? '<span class="badge" style="background:#d1fae5;color:#065f46">yes</span>' : '<span class="badge" style="background:#fee2e2;color:#991b1b">no</span>'}</td>
          <td>${Array.isArray(t.allowed_roles) && t.allowed_roles.length ? escapeHTML(t.allowed_roles.join(', ')) : '<span class="text-muted">—</span>'}</td>
          <td>${Array.isArray(t.conditions) && t.conditions.length ? JSON.stringify(t.conditions) : '<span class="text-muted">none</span>'}</td>
          <td>${escapeHTML(t.description || '')}</td>
        </tr>`).join('') +
      '</tbody></table>';
  },

  showWorkflowCreateView() {
    if (!currentUserIsAdmin()) {
      showToast('Only administrators can create workflow definitions', 'error');
      return;
    }
    this.switchView('view-new-workflow');
    this.resetWorkflowDraft();
    this.workflowCreateErrors = [];
    const container = document.getElementById('wf-create-container');
    if (!container) return;
    container.innerHTML = '';
    this.renderWorkflowCreateForm();
  },

  renderWorkflowCreateForm() {
    const c = this.workflowDraft;
    const stateKeys = c.states.map(s => s.key).filter(k => k);
    const container = document.getElementById('wf-create-container');
    if (!container) return;
    container.innerHTML = `
      <div class="header" style="margin-bottom:16px">
        <h1>Create Workflow Definition</h1>
        <nav><button class="btn secondary" onclick="app.cancelWorkflowCreate()">Cancel</button></nav>
      </div>
      <div class="card">
        <h2>Workflow</h2>
        <p class="section-hint">A new definition is created in <strong>DRAFT</strong> status. Activate it to start using it for new cases.</p>
        <label>Key</label><input id="wf-key" value="${escapeHTML(c.key)}" oninput="app.onWorkflowMetaChange('key', this.value)">
        <label>Name</label><input id="wf-name" value="${escapeHTML(c.name)}" oninput="app.onWorkflowMetaChange('name', this.value)">
        <label>Description</label><textarea id="wf-desc" rows="2" oninput="app.onWorkflowMetaChange('description', this.value)">${escapeHTML(c.description)}</textarea>
        <label>Version</label><input id="wf-version" type="number" min="1" value="${c.version}" oninput="app.onWorkflowMetaChange('version', this.value)">
        <label>Initial State</label>
        <select id="wf-initial" onchange="app.onWorkflowMetaChange('initial_state', this.value)">
          <option value="">— select initial state —</option>
          ${stateKeys.map(k => `<option value="${escapeHTML(k)}" ${k === c.initial_state ? 'selected' : ''}>${escapeHTML(k)}</option>`).join('')}
        </select>
      </div>
      <div class="card">
        <div style="display:flex;justify-content:space-between;align-items:center">
          <h2>States (${c.states.length})</h2>
          <button class="btn success" onclick="app.addWorkflowState()">Add State</button>
        </div>
        <p class="section-hint">Define the lifecycle states. Mark one as the initial state and any number as terminal.</p>
        <div class="table-wrap">
        <table id="wf-states-table">
          <thead><tr><th>Key</th><th>Name</th><th>Description</th><th>Category</th><th>Display Order</th><th>Terminal</th><th>Responsible Role</th><th>Actions</th></tr></thead>
          <tbody id="wf-states-tbody"></tbody>
        </table>
        </div>
      </div>
      <div class="card">
        <div style="display:flex;justify-content:space-between;align-items:center">
          <h2>Transitions (${c.transitions.length})</h2>
          <button class="btn success" onclick="app.addWorkflowTransition()">Add Transition</button>
        </div>
        <p class="section-hint">Define transitions between states. A transition's source and target must reference existing state keys.</p>
        <div class="table-wrap">
        <table id="wf-transitions-table">
          <thead><tr><th>Key</th><th>Name</th><th>From State</th><th>To State</th><th>Active</th><th>Allowed Roles</th><th>Conditions (JSON)</th><th>Description</th><th>Actions</th></tr></thead>
          <tbody id="wf-transitions-tbody"></tbody>
        </table>
        </div>
      </div>
      <div class="card">
        <div id="wf-create-errors" style="color:var(--danger);margin-bottom:12px;"></div>
        <div style="display:flex;gap:8px;justify-content:flex-end">
          <button class="btn secondary" onclick="app.cancelWorkflowCreate()">Cancel</button>
          <button class="btn" onclick="app.validateAndSubmitWorkflow()">Create Definition</button>
        </div>
      </div>
    `;
    this.renderWorkflowStatesEditor();
    this.renderWorkflowTransitionsEditor();
  },

  addWorkflowState() {
    this.workflowDraft.states.push({
      key: '', name: '', description: '', category: '',
      terminal: false, display_order: this.workflowDraft.states.length,
      responsible_role: '',
    });
    this.renderWorkflowStatesEditor();
    this.renderWorkflowTransitionsEditor();
  },

  addWorkflowTransition() {
    this.workflowDraft.transitions.push({
      key: '', name: '', from_state: '', to_state: '',
      description: '', conditions: [], allowed_roles: [], active: true,
    });
    this.renderWorkflowTransitionsEditor();
  },

  removeWorkflowState(idx) {
    const removed = this.workflowDraft.states.splice(idx, 1)[0];
    if (this.workflowDraft.initial_state === removed.key) {
      this.workflowDraft.initial_state = '';
    }
    this.workflowDraft.states.forEach((s, i) => { s.display_order = i; });
    this.renderWorkflowStatesEditor();
    this.renderWorkflowTransitionsEditor();
  },

  removeWorkflowTransition(idx) {
    this.workflowDraft.transitions.splice(idx, 1);
    this.renderWorkflowTransitionsEditor();
  },

  moveStateUp(idx) {
    if (idx <= 0) return;
    const s = this.workflowDraft.states.splice(idx, 1)[0];
    this.workflowDraft.states.splice(idx - 1, 0, s);
    this.workflowDraft.states.forEach((st, i) => { st.display_order = i; });
    this.renderWorkflowStatesEditor();
    this.renderWorkflowTransitionsEditor();
  },

  moveStateDown(idx) {
    if (idx >= this.workflowDraft.states.length - 1) return;
    const s = this.workflowDraft.states.splice(idx, 1)[0];
    this.workflowDraft.states.splice(idx + 1, 0, s);
    this.workflowDraft.states.forEach((st, i) => { st.display_order = i; });
    this.renderWorkflowStatesEditor();
    this.renderWorkflowTransitionsEditor();
  },

  onWorkflowMetaChange(field, value) {
    const v = value === '' ? '' : value;
    if (field === 'version') {
      const n = parseInt(value, 10);
      this.workflowDraft.version = isNaN(n) || n < 1 ? 1 : n;
    } else {
      this.workflowDraft[field] = v;
    }
    this.clearWorkflowErrors();
  },

  onStateFieldChange(idx, field, value, isCheckbox) {
    const s = this.workflowDraft.states[idx];
    if (!s) return;
    if (field === 'terminal') {
      s.terminal = !!value;
      return;
    }
    s[field] = isCheckbox ? !!value : value;
    if (field === 'key') this.refreshInitialSelect();
  },

  refreshInitialSelect() {
    const sel = document.getElementById('wf-initial');
    if (!sel) return;
    const keys = this.workflowDraft.states.map(s => s.key).filter(k => k);
    const current = this.workflowDraft.initial_state;
    sel.innerHTML = '<option value="">— select initial state —</option>' +
      keys.map(k => `<option value="${escapeHTML(k)}" ${k === current ? 'selected' : ''}>${escapeHTML(k)}</option>`).join('');
  },

  onTransitionFieldChange(idx, field, value) {
    const t = this.workflowDraft.transitions[idx];
    if (!t) return;
    if (field === 'active') { t.active = !!value; }
    else { t[field] = value; }
  },

  renderWorkflowStatesEditor() {
    const c = this.workflowDraft;
    const tbody = document.getElementById('wf-states-tbody');
    if (!tbody) return;
     tbody.innerHTML = c.states.map((s, i) => {
       return `<tr>
        <td><input style="width:110px" value="${escapeHTML(s.key)}" oninput="app.onStateFieldChange(${i}, 'key', this.value)"></td>
        <td><input value="${escapeHTML(s.name)}" oninput="app.onStateFieldChange(${i}, 'name', this.value)"></td>
        <td><input value="${escapeHTML(s.description)}" oninput="app.onStateFieldChange(${i}, 'description', this.value)"></td>
        <td><input value="${escapeHTML(s.category)}" oninput="app.onStateFieldChange(${i}, 'category', this.value)"></td>
         <td><input type="number" min="0" style="width:80px" value="${s.display_order}" onchange="app.onStateFieldChange(${i}, 'display_order', parseInt(this.value,10)||0)"></td>
         <td style="text-align:center"><input type="checkbox" ${s.terminal ? 'checked' : ''} onchange="app.onStateFieldChange(${i}, 'terminal', this.checked)"></td>
        <td><input value="${escapeHTML(s.responsible_role)}" oninput="app.onStateFieldChange(${i}, 'responsible_role', this.value)"></td>
        <td style="white-space:nowrap">
          <button class="btn secondary" style="font-size:0.75rem;padding:2px 6px" onclick="app.moveStateUp(${i})">↑</button>
          <button class="btn secondary" style="font-size:0.75rem;padding:2px 6px" onclick="app.moveStateDown(${i})">↓</button>
          <button class="btn danger" style="font-size:0.75rem;padding:2px 6px" onclick="app.removeWorkflowState(${i})">✕</button>
        </td>
      </tr>`;
      }).join('');
    this.refreshInitialSelect();
  },

  renderWorkflowTransitionsEditor() {
    const c = this.workflowDraft;
    const stateKeys = c.states.map(s => s.key).filter(k => k);
    const tbody = document.getElementById('wf-transitions-tbody');
    if (!tbody) return;
    tbody.innerHTML = c.transitions.map((t, i) => {
      const fromOptions = [{ v: '', l: '— select —' }, ...stateKeys.map(k => k)].map(k => {
        const val = typeof k === 'string' ? k : k.v;
        const label = typeof k === 'string' ? k : k.l;
        return `<option value="${escapeHTML(val)}" ${val === t.from_state ? 'selected' : ''}>${escapeHTML(label)}</option>`;
      }).join('');
      const toOptions = fromOptions;
      return `<tr>
        <td><input style="width:110px" value="${escapeHTML(t.key)}" oninput="app.onTransitionFieldChange(${i}, 'key', this.value)"></td>
        <td><input value="${escapeHTML(t.name)}" oninput="app.onTransitionFieldChange(${i}, 'name', this.value)"></td>
        <td><select onchange="app.onTransitionFieldChange(${i}, 'from_state', this.value)">${fromOptions}</select></td>
        <td><select onchange="app.onTransitionFieldChange(${i}, 'to_state', this.value)">${toOptions}</select></td>
        <td style="text-align:center"><input type="checkbox" ${t.active ? 'checked' : ''} onchange="app.onTransitionFieldChange(${i}, 'active', this.checked)"></td>
        <td><input value="${Array.isArray(t.allowed_roles) ? t.allowed_roles.join(', ') : ''}" oninput="app.onTransitionFieldChange(${i}, 'allowed_roles_raw', this.value)" placeholder="admin, staff"></td>
        <td><input style="width:140px" value="${Array.isArray(t.conditions) && t.conditions.length ? JSON.stringify(t.conditions) : ''}" oninput="app.onTransitionFieldChange(${i}, 'conditions_raw', this.value)" placeholder="[]"></td>
        <td><input value="${escapeHTML(t.description)}" oninput="app.onTransitionFieldChange(${i}, 'description', this.value)"></td>
        <td style="white-space:nowrap"><button class="btn danger" style="font-size:0.75rem;padding:2px 6px" onclick="app.removeWorkflowTransition(${i})">✕</button></td>
      </tr>`;
    }).join('');
  },

  clearWorkflowErrors() {
    const el = document.getElementById('wf-create-errors');
    if (el) el.innerHTML = '';
    this.workflowCreateErrors = [];
  },

  validateWorkflowDraft() {
    const errors = [];
    const d = this.workflowDraft;
    if (!d.key.trim()) errors.push('Workflow key is required');
    if (!d.name.trim()) errors.push('Workflow name is required');
    if (!d.version || d.version < 1) errors.push('Version must be at least 1');
    if (!d.initial_state) errors.push('An initial state is required');
    if (!d.states.length) errors.push('At least one state is required');

    const stateKeys = new Set();
    d.states.forEach((s, i) => {
      if (!s.key.trim()) errors.push(`State #${i + 1}: key is required`);
      else if (stateKeys.has(s.key)) errors.push(`Duplicate state key: "${s.key}"`);
      else stateKeys.add(s.key);
      if (!s.name.trim()) errors.push(`State #${i + 1}: name is required`);
    });
    if (d.initial_state) {
      if (!stateKeys.has(d.initial_state)) {
        errors.push(`Initial state "${d.initial_state}" does not match any defined state key`);
      }
    } else if (stateKeys.size > 0) {
      errors.push('An initial state must be selected');
    }

    const seenTransitions = new Set();
    d.transitions.forEach((t, i) => {
      if (!t.key.trim()) errors.push(`Transition #${i + 1}: key is required`);
      else if (seenTransitions.has(t.key)) errors.push(`Duplicate transition key: "${t.key}"`);
      else seenTransitions.add(t.key);
      if (!t.name.trim()) errors.push(`Transition #${i + 1}: name is required`);
      if (!stateKeys.has(t.from_state)) errors.push(`Transition #${i + 1}: source state "${t.from_state}" is unknown`);
      if (!stateKeys.has(t.to_state)) errors.push(`Transition #${i + 1}: target state "${t.to_state}" is unknown`);
      if (t.from_state && t.from_state === t.to_state) errors.push(`Transition #${i + 1}: source and target must differ`);
      if (t.conditions_raw !== undefined && String(t.conditions_raw).trim() !== '') {
        try {
          const parsed = JSON.parse(t.conditions_raw);
          if (!Array.isArray(parsed)) errors.push(`Transition #${i + 1}: conditions must be a JSON array`);
        } catch (e) { errors.push(`Transition #${i + 1}: conditions must be valid JSON`); }
      }
    });

    d.states.forEach(s => {
      if (s.terminal && stateKeys.has(s.key)) {
        const hasOutgoing = d.transitions.some(t => t.from_state === s.key && t.active !== false);
        if (hasOutgoing) errors.push(`Terminal state "${s.key}" cannot have outgoing transitions`);
      }
    });

    return errors;
  },

  validateAndSubmitWorkflow() {
    this.clearWorkflowErrors();
    const errors = this.validateWorkflowDraft();
    if (errors.length) {
      this.workflowCreateErrors = errors;
      this.renderWorkflowErrors();
      return;
    }
    this.submitWorkflowCreate();
  },

  renderWorkflowErrors() {
    const el = document.getElementById('wf-create-errors');
    if (!el) return;
    el.innerHTML = this.workflowCreateErrors.map(e => `<div>• ${escapeHTML(e)}</div>`).join('');
  },

  buildWorkflowPayload() {
    const d = this.workflowDraft;
    const transitions = d.transitions.map(t => {
      let conditions = [];
      if (typeof t.conditions_raw === 'string' && t.conditions_raw.trim()) {
        try { conditions = JSON.parse(t.conditions_raw); } catch (e) { conditions = []; }
      } else if (Array.isArray(t.conditions)) {
        conditions = t.conditions;
      }
      let allowed_roles = [];
      const raw = typeof t.allowed_roles_raw === 'string' ? t.allowed_roles_raw.trim() : '';
      if (raw) allowed_roles = raw.split(',').map(r => r.trim()).filter(Boolean);
      return {
        key: t.key, name: t.name, from_state: t.from_state, to_state: t.to_state,
        description: t.description, conditions, allowed_roles, active: t.active !== false,
      };
    });
    const states = d.states.map(s => ({
      key: s.key, name: s.name, description: s.description, category: s.category,
      terminal: !!s.terminal, display_order: s.display_order, responsible_role: s.responsible_role,
    }));
    return {
      key: d.key, name: d.name, description: d.description, version: d.version,
      initial_state: d.initial_state, states, transitions, metadata: {},
    };
  },

  async submitWorkflowCreate() {
    const payload = this.buildWorkflowPayload();
    try {
      const res = await this.wfCreate(payload);
      const def = res.data || payload;
      showToast('Workflow definition created', 'success');
      this.workflowDraft = null;
      this.switchView('view-workflow-detail');
      const container = document.getElementById('wf-detail-container');
      if (container) {
        container.innerHTML = '';
        this.renderWorkflowDetail(def);
      }
    } catch (err) {
      showToast(err.message, 'error');
    }
  },

  cancelWorkflowCreate() {
    this.workflowDraft = null;
    this.switchView('view-workflows');
    this.loadWorkflowList(this.workflowListPage || 1);
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
  document.getElementById('view-workflows').classList.add('hidden');
  document.getElementById('view-workflow-detail').classList.add('hidden');
  document.getElementById('view-new-workflow').classList.add('hidden');
});

router.on('dashboard', async () => {
  document.getElementById('view-login').classList.add('hidden');
  document.getElementById('view-dashboard').classList.remove('hidden');
  document.getElementById('view-case').classList.add('hidden');
  document.getElementById('view-new-case').classList.add('hidden');
  document.getElementById('view-workflows').classList.add('hidden');
  document.getElementById('view-workflow-detail').classList.add('hidden');
  document.getElementById('view-new-workflow').classList.add('hidden');
  await app.loadDashboard();
});

router.on('case', async (id) => {
  if (!id) { router.navigate('dashboard'); return; }
  document.getElementById('view-login').classList.add('hidden');
  document.getElementById('view-dashboard').classList.add('hidden');
  document.getElementById('view-case').classList.remove('hidden');
  document.getElementById('view-new-case').classList.add('hidden');
  document.getElementById('view-workflows').classList.add('hidden');
  document.getElementById('view-workflow-detail').classList.add('hidden');
  document.getElementById('view-new-workflow').classList.add('hidden');
  await app.loadCase(id);
});

router.on('new-case', () => {
  document.getElementById('view-login').classList.add('hidden');
  document.getElementById('view-dashboard').classList.add('hidden');
  document.getElementById('view-case').classList.add('hidden');
  document.getElementById('view-new-case').classList.remove('hidden');
  document.getElementById('view-workflows').classList.add('hidden');
  document.getElementById('view-workflow-detail').classList.add('hidden');
  document.getElementById('view-new-workflow').classList.add('hidden');
});

router.on('workflows', () => {
  app.switchView('view-workflows');
  app.loadWorkflowList(1);
});

router.on('workflow', (id) => {
  if (!id) { router.navigate('workflows'); return; }
  app.showWorkflowDetail(id);
});

router.on('new-workflow', () => {
  app.showWorkflowCreateView();
});
