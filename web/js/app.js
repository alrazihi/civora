const app = {
  currentCase: null,

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
      setAuth(res.data.token, org);
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
      const res = await api('POST', '/people', data);
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
      const res = await api('POST', '/cases', data);
      router.navigate('case', res.data.id);
    } catch (err) {
      alert(err.message);
    }
  },

  async loadDashboard() {
    try {
      const res = await api('GET', '/cases?per_page=50');
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
      const res = await api('GET', `/cases/${id}`);
      this.currentCase = res.data;
      document.getElementById('case-title').textContent = res.data.title;
      document.getElementById('case-number').textContent = res.data.case_number;
      document.getElementById('case-status').textContent = res.data.status;
      document.getElementById('case-status').className = `badge ${res.data.status.toLowerCase().replace('_','-')}`;
      document.getElementById('case-service').textContent = res.data.service_type;
      document.getElementById('case-priority').textContent = res.data.priority;
      document.getElementById('case-desc').textContent = res.data.description || 'No description';

      await this.loadCaseSections(id);
      this.renderTimeline(res.data.status);
      this.renderActions(res.data.status);
    } catch (err) {
      alert(err.message);
    }
  },

  async loadCaseSections(id) {
    await this.loadSection('eligibility', `/eligibilities/by-service-request/${id}`);
    await this.loadSection('evidence', `/evidence/by-service-request/${id}`);
    await this.loadSection('assessment', `/assessments/by-service-request/${id}`);
    await this.loadSection('decision', `/decisions/by-service-request/${id}`);
    await this.loadSection('assistance', `/assistance/by-service-request/${id}`);
    await this.loadSection('followup', `/follow-ups/by-service-request/${id}`);
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
          el.innerHTML = '<table><thead><tr><th>Type</th><th>Description</th><th>Storage</th></tr></thead><tbody>' +
            data.map(e => `<tr><td>${e.type}</td><td>${e.description}</td><td>${e.storage_reference}</td></tr>`).join('') +
            '</tbody></table>';
        } else {
          el.innerHTML = `<strong>Type:</strong> ${data.type}<br><strong>Description:</strong> ${data.description}<br><strong>Storage:</strong> ${data.storage_reference}`;
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
      if (el) el.innerHTML = `<p style="color:var(--danger)">Error loading: ${err.message}</p>`;
    }
  },

  renderTimeline(status) {
    const steps = [
      { key: 'NEW', label: 'Request Created' },
      { key: 'OPEN', label: 'Opened' },
      { key: 'IN_REVIEW', label: 'In Review' },
      { key: 'ASSESSMENT', label: 'Assessment' },
      { key: 'DECISION_PENDING', label: 'Decision Pending' },
      { key: 'APPROVED', label: 'Approved' },
      { key: 'REJECTED', label: 'Rejected' },
      { key: 'IN_PROGRESS', label: 'In Progress' },
      { key: 'FOLLOW_UP', label: 'Follow-up' },
      { key: 'CLOSED', label: 'Closed' },
    ];
    const order = ['NEW','OPEN','IN_REVIEW','ASSESSMENT','DECISION_PENDING','APPROVED','REJECTED','IN_PROGRESS','FOLLOW_UP','CLOSED'];
    const currentIdx = order.indexOf(status);
    const container = document.getElementById('timeline');
    if (!container) return;
    container.innerHTML = steps.map((s, i) => {
      let cls = '';
      if (i < currentIdx) cls = 'completed';
      else if (i === currentIdx) cls = '';
      else cls = 'pending';
      return `<div class="timeline-item ${cls}"><div class="timeline-title">${s.label}</div><div class="timeline-meta">${s.key}</div></div>`;
    }).join('');
  },

  renderActions(status) {
    const container = document.getElementById('case-actions');
    if (!container) return;
    let html = '';
    if (status === 'NEW') html += this.btn('Open Case', `transition('OPEN')`);
    if (status === 'OPEN') html += this.btn('Start Review', `transition('IN_REVIEW')`);
    if (status === 'IN_REVIEW') {
      html += this.btn('Move to Assessment', `transition('ASSESSMENT')`);
      html += `<button class="btn secondary" onclick="app.showEligibilityForm()">Add Eligibility</button>`;
      html += `<button class="btn secondary" onclick="app.showEvidenceForm()">Add Evidence</button>`;
    }
    if (status === 'ASSESSMENT') {
      html += this.btn('Request Decision', `transition('DECISION_PENDING')`);
      html += `<button class="btn secondary" onclick="app.showAssessmentForm()">Add Assessment</button>`;
    }
    if (status === 'DECISION_PENDING') {
      html += `<button class="btn secondary" onclick="app.showDecisionForm()">Record Decision</button>`;
    }
    if (status === 'APPROVED') {
      html += this.btn('Start Assistance', `transition('IN_PROGRESS')`);
    }
    if (status === 'IN_PROGRESS') {
      html += this.btn('Schedule Follow-up', `transition('FOLLOW_UP')`);
      html += `<button class="btn secondary" onclick="app.showAssistanceForm()">Add Assistance</button>`;
    }
    if (status === 'FOLLOW_UP') {
      html += this.btn('Close Case', `transition('CLOSED')`);
      html += `<button class="btn secondary" onclick="app.showFollowUpForm()">Add Follow-up</button>`;
    }
    if (!html) html = '<p class="empty">No actions available</p>';
    container.innerHTML = html;
  },

  btn(label, onclick) {
    return `<button class="btn" onclick="${onclick}">${label}</button>`;
  },

  async transition(status) {
    if (!this.currentCase) return;
    try {
      const res = await api('POST', `/cases/${this.currentCase.id}/transitions`, { status });
      this.currentCase = res.data;
      document.getElementById('case-status').textContent = res.data.status;
      document.getElementById('case-status').className = `badge ${res.data.status.toLowerCase().replace('_','-')}`;
      this.renderTimeline(res.data.status);
      this.renderActions(res.data.status);
      await this.loadCaseSections(res.data.id);
    } catch (err) {
      alert(err.message);
    }
  },

  showEligibilityForm() { this.showModal('eligibility-modal'); },
  showEvidenceForm() { this.showModal('evidence-modal'); },
  showAssessmentForm() { this.showModal('assessment-modal'); },
  showDecisionForm() { this.showModal('decision-modal'); },
  showAssistanceForm() { this.showModal('assistance-modal'); },
  showFollowUpForm() { this.showModal('followup-modal'); },

  showModal(id) { document.getElementById(id)?.classList.remove('hidden'); },
  hideModal(id) { document.getElementById(id)?.classList.add('hidden'); },

  async submitEligibility(e) {
    e.preventDefault();
    try {
      await api('POST', '/eligibilities', {
        service_request_id: this.currentCase.id,
        criteria: { manual: true },
        explanation: document.getElementById('elig-explanation').value,
      });
      this.hideModal('eligibility-modal');
      await this.loadCaseSections(this.currentCase.id);
    } catch (err) { alert(err.message); }
  },

  async submitEvidence(e) {
    e.preventDefault();
    try {
      await api('POST', '/evidence', {
        service_request_id: this.currentCase.id,
        type: document.getElementById('ev-type').value,
        description: document.getElementById('ev-desc').value,
        storage_reference: document.getElementById('ev-ref').value,
      });
      this.hideModal('evidence-modal');
      await this.loadCaseSections(this.currentCase.id);
    } catch (err) { alert(err.message); }
  },

  async submitAssessment(e) {
    e.preventDefault();
    try {
      await api('POST', '/assessments', {
        service_request_id: this.currentCase.id,
        findings: document.getElementById('as-findings').value,
        needs_identified: document.getElementById('as-needs').value,
        recommendation: document.getElementById('as-rec').value,
      });
      this.hideModal('assessment-modal');
      await this.loadCaseSections(this.currentCase.id);
    } catch (err) { alert(err.message); }
  },

  async submitDecision(e) {
    e.preventDefault();
    try {
      await api('POST', '/decisions', {
        service_request_id: this.currentCase.id,
        decision: document.getElementById('dec-decision').value,
        reason: document.getElementById('dec-reason').value,
      });
      this.hideModal('decision-modal');
      await this.loadCaseSections(this.currentCase.id);
    } catch (err) { alert(err.message); }
  },

  async submitAssistance(e) {
    e.preventDefault();
    try {
      await api('POST', '/assistance', {
        service_request_id: this.currentCase.id,
        type: document.getElementById('asst-type').value,
        description: document.getElementById('asst-desc').value,
        responsible_staff: document.getElementById('asst-staff').value,
      });
      this.hideModal('assistance-modal');
      await this.loadCaseSections(this.currentCase.id);
    } catch (err) { alert(err.message); }
  },

  async submitFollowUp(e) {
    e.preventDefault();
    try {
      await api('POST', '/follow-ups', {
        service_request_id: this.currentCase.id,
        scheduled_date: document.getElementById('fu-date').value,
        outcome: document.getElementById('fu-outcome').value,
        notes: document.getElementById('fu-notes').value,
      });
      this.hideModal('followup-modal');
      await this.loadCaseSections(this.currentCase.id);
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
