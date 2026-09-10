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
      router.navigate('dashboard');
    } catch (err) {
      alert(err.message);
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
    } catch (err) {
      alert(err.message);
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
    } catch (err) {
      alert(err.message);
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
      router.navigate('case', res.data.id);
    } catch (err) {
      alert(err.message);
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
          <td>${c.case_number}</td>
          <td>${c.title}</td>
          <td><span class="badge ${c.status.toLowerCase().replace('_','-')}">${c.status}</span></td>
          <td>${c.service_type}</td>
          <td>${c.priority}</td>
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
      document.getElementById('case-status').className = `badge ${res.data.status.toLowerCase().replace('_','-')}`;
      document.getElementById('case-service').textContent = res.data.service_type;
      document.getElementById('case-priority').textContent = res.data.priority;
      document.getElementById('case-desc').textContent = res.data.description || 'No description';

      await this.loadWorkflow(id);
      await this.loadCaseSections(id);
      await this.loadTimeline(id);
      this.renderActions();
    } catch (err) {
      alert(err.message);
    }
  },

  async loadWorkflow(caseId) {
    try {
      const res = await api('GET', this.orgPath(`/cases/${caseId}/workflow`));
      this.currentWorkflow = res.data;
    } catch (err) {
      this.currentWorkflow = null;
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
      if (name === 'eligibility') {
        el.innerHTML = `<strong>Result:</strong> ${data.result}<br><strong>Explanation:</strong> ${data.explanation}`;
      } else if (name === 'evidence') {
        if (Array.isArray(data)) {
          if (!data.length) { el.innerHTML = '<p class="empty">No evidence yet</p>'; return; }
          el.innerHTML = '<table><thead><tr><th>Type</th><th>Description</th></tr></thead><tbody>' +
            data.map(e => `<tr><td>${e.type}</td><td>${e.description}</td></tr>`).join('') +
            '</tbody></table>';
        } else {
          el.innerHTML = `<strong>Type:</strong> ${data.type}<br><strong>Description:</strong> ${data.description}`;
        }
      } else if (name === 'assessment') {
        if (Array.isArray(data)) {
          if (!data.length) { el.innerHTML = '<p class="empty">No assessment yet</p>'; return; }
          const a = data[0];
          el.innerHTML = `<strong>Findings:</strong> ${a.findings}<br><strong>Needs:</strong> ${a.needs_identified || 'N/A'}<br><strong>Recommendation:</strong> ${a.recommendation}`;
        } else {
          el.innerHTML = `<strong>Findings:</strong> ${data.findings}<br><strong>Needs:</strong> ${data.needs_identified || 'N/A'}<br><strong>Recommendation:</strong> ${data.recommendation}`;
        }
      } else if (name === 'decision') {
        if (Array.isArray(data)) {
          if (!data.length) { el.innerHTML = '<p class="empty">No decision yet</p>'; return; }
          const d = data[0];
          el.innerHTML = `<strong>Decision:</strong> <span class="badge ${d.decision.toLowerCase().replace('_','-')}">${d.decision}</span><br><strong>Reason:</strong> ${d.reason}`;
        } else {
          el.innerHTML = `<strong>Decision:</strong> <span class="badge ${data.decision.toLowerCase().replace('_','-')}">${data.decision}</span><br><strong>Reason:</strong> ${data.reason}`;
        }
      } else if (name === 'assistance') {
        if (Array.isArray(data)) {
          if (!data.length) { el.innerHTML = '<p class="empty">No assistance yet</p>'; return; }
          el.innerHTML = data.map(a => `<div><strong>${a.type}</strong> - ${a.status}<br>${a.description}</div>`).join('');
        } else {
          el.innerHTML = `<strong>${data.type}</strong> - ${data.status}<br>${data.description}`;
        }
      } else if (name === 'followup') {
        if (Array.isArray(data)) {
          if (!data.length) { el.innerHTML = '<p class="empty">No follow-up yet</p>'; return; }
          el.innerHTML = data.map(f => `<div><strong>${f.scheduled_date}</strong> - ${f.outcome}</div>`).join('');
        } else {
          el.innerHTML = `<strong>${data.scheduled_date}</strong> - ${data.outcome}`;
        }
      }
    } catch (err) {
      const el = document.getElementById(`sec-${name}`);
      if (el) {
        if (err.message && err.message.includes('404')) {
          el.innerHTML = '<p class="empty">Not yet recorded</p>';
        } else {
          el.innerHTML = `<p style="color:var(--danger)">Error loading: ${err.message}</p>`;
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
      for (const t of transitions) {
        html += this.btn(t.name || t.key, `workflowTransition('${t.key}')`);
      }
      if (!html) html = '<p class="empty">No actions available</p>';
      container.innerHTML = html;
    });
  },

  async loadTimeline(id) {
    const container = document.getElementById('timeline');
    if (!container) return;
    try {
      const [caseRes, workflowRes] = await Promise.all([
        api('GET', this.orgPath(`/cases/${id}/timeline`)),
        api('GET', this.orgPath(`/cases/${id}/workflow/history`)).catch(() => ({ data: [] })),
      ]);
      const caseEvents = caseRes.data || [];
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
      const events = [...caseEvents, ...workflowEvents].sort((a, b) => new Date(a.timestamp) - new Date(b.timestamp));
      if (!events.length) {
        container.innerHTML = '<p class="empty">No timeline events yet</p>';
        return;
      }
      const decisionEvents = events.filter(e => e.action === 'decision.made');
      const approved = decisionEvents.find(e => {
        const d = (e.metadata && e.metadata.decision) || '';
        return d === 'APPROVED';
      });
      const rejected = decisionEvents.find(e => {
        const d = (e.metadata && e.metadata.decision) || '';
        return d === 'REJECTED';
      });
      container.innerHTML = events.map(ev => {
        let label = ev.action;
        if (ev.metadata) {
          if (ev.metadata.decision) label += ` (${ev.metadata.decision})`;
          if (ev.metadata.to) label += ` → ${ev.metadata.to}`;
          if (ev.metadata.from) label += ` from ${ev.metadata.from}`;
          if (ev.metadata.assistance_type) label += ` (${ev.metadata.assistance_type})`;
          if (ev.metadata.action && ev.resource === 'assistance') label += ` (${ev.metadata.action})`;
        }
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
      await this.loadCase(this.currentCase.id);
    } catch (err) {
      alert(err.message);
    }
  },

  showEligibilityForm() { this.showModal('eligibility-modal'); },
  showEvidenceForm() { this.showModal('evidence-modal'); },
  showAssessmentForm() { this.showModal('assessment-modal'); },
  showDecisionForm() { this.showModal('decision-modal'); },
  showAssistanceForm() { this.loadStaffOptions().then(() => this.showModal('assistance-modal')); },
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
        users.map(u => `<option value="${u.id}">${u.name} (${u.email})</option>`).join('');
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
      this.renderActions();
    } catch (err) { alert(err.message); }
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
      this.renderActions();
    } catch (err) { alert(err.message); }
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
      this.renderActions();
    } catch (err) { alert(err.message); }
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
      this.renderActions();
    } catch (err) { alert(err.message); }
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
      this.renderActions();
    } catch (err) { alert(err.message); }
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
      this.renderActions();
    } catch (err) { alert(err.message); }
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
