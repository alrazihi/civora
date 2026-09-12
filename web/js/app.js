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
    assistanceTypes: ['FOOD', 'SHELTER', 'TRANSPORT', 'MEDICAL', 'FINANCIAL', 'OTHER']
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
    assistanceTypes: ['MEDICAL', 'TRANSPORT', 'FINANCIAL', 'FOOD', 'SHELTER', 'OTHER']
  },
  FINANCIAL: {
    label: 'Financial Assistance',
    color: '#059669',
    icon: '💰',
    sections: {
      eligibility: { title: 'Financial Eligibility', hint: 'Verify financial eligibility and means testing.' },
      evidence: { title: 'Financial Evidence', hint: 'Collect income statements, bank records, and expense documentation.' },
      assessment: { title: 'Financial Assessment', hint: 'Assess financial need and recommended support level.' },
      decision: { title: 'Financial Decision', hint: 'Decision on financial assistance approval.' },
      assistance: { title: 'Financial Assistance', hint: 'Provide grants, vouchers, or direct payments.' },
      followup: { title: 'Financial Follow-up', hint: 'Review ongoing financial circumstances.' }
    },
    assistanceTypes: ['FINANCIAL', 'FOOD', 'SHELTER', 'TRANSPORT', 'OTHER']
  },
  GENERAL: {
    label: 'General Assistance',
    color: '#6366f1',
    icon: '📋',
    sections: {
      eligibility: { title: 'Eligibility Check', hint: 'Verify eligibility for general assistance.' },
      evidence: { title: 'Evidence Collection', hint: 'Gather supporting documentation.' },
      assessment: { title: 'Assessment', hint: 'Assess needs and circumstances.' },
      decision: { title: 'Decision', hint: 'Record approval or rejection decision.' },
      assistance: { title: 'Assistance', hint: 'Provide approved assistance.' },
      followup: { title: 'Follow-up', hint: 'Schedule follow-up review.' }
    },
    assistanceTypes: ['FINANCIAL', 'FOOD', 'SHELTER', 'MEDICAL', 'TRANSPORT', 'EDUCATION', 'OTHER']
  }
};

function getTerminalStates(def) {
  if (!def || !Array.isArray(def.states)) return [];
  return def.states.filter(s => s.terminal).map(s => s.key);
}

function cssStateClass(state) {
  if (!state) return '';
  const base = state.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '');
  return base;
}

function isCaseTerminal(caseStatus, workflowInstance, terminalStateSet) {
  if (!caseStatus) return false;
  if (FALLBACK_TERMINAL_STATES.includes(caseStatus)) return true;
  if (terminalStateSet && terminalStateSet.has(caseStatus)) return true;
  if (workflowInstance && workflowInstance.definition) {
    const terminalStates = getTerminalStates(workflowInstance.definition);
    if (terminalStates.includes(caseStatus)) return true;
    if (terminalStates.includes(workflowInstance.instance.current_state)) return true;
  }
  return false;
}

function escapeHTML(str) {
  if (str == null) return '';
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
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
  toast.className = `toast ${type}`;
  toast.textContent = message;
  container.appendChild(toast);
  setTimeout(() => {
    toast.style.animation = 'slideOut 0.3s ease';
    setTimeout(() => toast.remove(), 300);
  }, 4000);
}

const styleEl = document.createElement('style');
styleEl.textContent = '';
document.head.appendChild(styleEl);

const SERVICE_TYPE_WORKFLOW_KEYS = {
  EMERGENCY: 'emergency_assistance',
  MEDICAL: 'medical_assistance',
  FINANCIAL: 'financial_assistance',
  FOOD: 'food_assistance',
  SHELTER: 'shelter_assistance',
  EDUCATION: 'education_assistance',
  TRANSPORT: 'transport_assistance',
  GENERAL: 'general_assistance'
};

const FALLBACK_TERMINAL_STATES = ['CLOSED', 'REJECTED'];

function deriveServiceTypeForWorkflow(def) {
  if (!def || def.status !== 'ACTIVE') return null;
  const md = def.metadata || {};
  if (typeof md.service_type === 'string' && SERVICE_DOMAIN[md.service_type]) {
    return md.service_type;
  }
  for (const [serviceType, key] of Object.entries(SERVICE_TYPE_WORKFLOW_KEYS)) {
    if (key === def.key) return serviceType;
  }
  return null;
}

function computeTerminalStateSet(defs) {
  const set = new Set(FALLBACK_TERMINAL_STATES);
  (Array.isArray(defs) ? defs : [])
    .filter(d => Array.isArray(d.states))
    .forEach(d => d.states.forEach(s => { if (s.terminal) set.add(s.key); }));
  return set;
}

function getServiceDomain(serviceType) {
  if (SERVICE_DOMAIN[serviceType]) return SERVICE_DOMAIN[serviceType];
  if (SERVICE_DOMAIN.GENERAL) return SERVICE_DOMAIN.GENERAL;
  return null;
}

const app = {
  currentCase: null,
  currentWorkflow: null,
  workflowDraft: null,
  activeWorkflows: [],
  selectedWorkflow: null,
  _terminalStates: new Set(FALLBACK_TERMINAL_STATES),
  _cachedTransitions: [],

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
    const workflow = this.selectedWorkflow;
    if (!workflow) {
      showToast('Select a workflow definition before creating a case', 'error');
      return;
    }
    const serviceType = deriveServiceTypeForWorkflow(workflow) || 'GENERAL';
    const data = {
      title: document.getElementById('c-title').value.trim(),
      description: document.getElementById('c-desc').value.trim(),
      service_type: serviceType,
      priority: document.getElementById('c-priority').value || 'NORMAL',
      person_id: document.getElementById('new-case-person-id').value || undefined,
      workflow_id: workflow.id,
    };
    if (!data.title) {
      showToast('Title is required', 'error');
      return;
    }
    try {
      const res = await api('POST', this.orgPath('/cases'), data);
      const created = res.data;
      if (!created || !created.id) {
        throw new Error('Case creation response did not include an id');
      }
      await this.verifyCaseWorkflow(created.id, workflow.id);
      showToast('Case created successfully', 'success');
      router.navigate('case', created.id);
    } catch (err) {
      showToast(err.message, 'error');
    }
  },

  async verifyCaseWorkflow(caseId, expectedDefId) {
    const res = await api('GET', this.orgPath(`/cases/${caseId}/workflow`));
    const instance = res.data && res.data.instance;
    if (!instance || !instance.workflow_definition_id) {
      throw new Error('No workflow instance was attached to the created case');
    }
    if (String(instance.workflow_definition_id) !== String(expectedDefId)) {
      throw new Error(`Selected workflow was not attached to the case (expected definition ${expectedDefId}, got ${instance.workflow_definition_id}).`);
    }
  },

  async loadNewCaseWorkflows(preselectId) {
    const container = document.getElementById('workflow-selector');
    if (container) container.innerHTML = '<p class="empty">Loading workflows…</p>';
    try {
      const defs = await this.loadAllWorkflows();
      this.activeWorkflows = defs;
      this._terminalStates = computeTerminalStateSet(defs);
      this.renderWorkflowSelector(defs, preselectId);
    } catch (err) {
      if (container) container.innerHTML = `<p class="empty" style="color:var(--danger)">Failed to load workflows: ${escapeHTML(err.message)}</p>`;
    }
  },

  async loadAllWorkflows() {
    let defs = [];
    let page = 1;
    for (;;) {
      const res = await api('GET', this.orgPath(`/workflows?page=${page}&per_page=100`));
      const pageDefs = res.data || [];
      defs = defs.concat(pageDefs);
      const totalPages = (res.meta && res.meta.total_pages) || 1;
      if (page >= totalPages || pageDefs.length < 100) break;
      page++;
    }
    return defs;
  },

  renderWorkflowSelector(defs, preselectId) {
    const container = document.getElementById('workflow-selector');
    if (!container) return;
    const hidden = document.getElementById('selected-workflow-id');
    if (hidden) hidden.value = '';
    const active = defs.filter(d => d.status === 'ACTIVE');
    if (!defs.length) {
      container.innerHTML = '<p class="empty">No workflow definitions are available for your organization. Create one in the Workflows admin area first.</p>';
      return;
    }
    if (!active.length) {
      container.innerHTML = '<p class="empty">No active workflow definitions are available. Contact an administrator to activate a workflow before creating a case.</p>';
      return;
    }
    const preselected = preselectId ? active.find(d => d.id === preselectId) : null;
    const firstActive = preselected || active[0];
    container.innerHTML = active.map(d => this.renderWorkflowOptionCard(d, d.id === firstActive.id)).join('');
    this.selectWorkflow(firstActive.id);
  },

  renderWorkflowOptionCard(d, selected) {
    const usable = d.status === 'ACTIVE';
    const states = (d.states || []).length;
    const transitions = (d.transitions || []).length;
    const selectedMark = selected ? ' selected' : '';
    const disabledAttr = usable ? '' : 'disabled';
    return `<div class="wf-option-card${selectedMark}${usable ? '' : ' wf-unusable'}" data-wf-id="${escapeHTML(d.id)}" onclick="${usable ? `app.selectWorkflow('${d.id}')` : ''}">
      <div class="wf-option-header">
        <span class="wf-option-name">${escapeHTML(d.name)} <span class="text-muted" style="font-weight:400">(${escapeHTML(d.key)})</span></span>
        <span class="badge wf-status-${(d.status || 'DRAFT').toLowerCase()}">${escapeHTML(d.status || 'DRAFT')}</span>
      </div>
      <div class="wf-option-body">
        <div class="wf-option-meta"><span class="text-muted">Version</span> <strong>${d.version || 1}</strong></div>
        <div class="wf-option-meta"><span class="text-muted">States</span> <strong>${states}</strong></div>
        <div class="wf-option-meta"><span class="text-muted">Transitions</span> <strong>${transitions}</strong></div>
        <div class="wf-option-meta"><span class="text-muted">Initial State</span> <strong>${escapeHTML(d.initial_state || '—')}</strong></div>
        <p class="wf-option-desc">${escapeHTML(d.description || 'No description provided.')}</p>
        ${!usable ? '<p class="wf-option-note">Not usable: only active workflows can be selected.</p>' : ''}
      </div>
    </div>`;
  },

  selectWorkflow(id) {
    const defs = this.activeWorkflows;
    const wf = defs.find(d => d.id === id);
    if (!wf) {
      showToast('Selected workflow not found', 'error');
      return;
    }
    this.selectedWorkflow = wf;
    const hidden = document.getElementById('selected-workflow-id');
    if (hidden) hidden.value = wf.id;
    const cards = document.querySelectorAll('.wf-option-card');
    cards.forEach(c => {
      const isSelected = c.getAttribute('data-wf-id') === id;
      c.classList.toggle('selected', isSelected);
    });
    const preview = document.getElementById('workflow-preview');
    if (preview) {
      preview.innerHTML = `
        <h3 style="margin-top:0">Selected Workflow</h3>
        <p style="margin:4px 0"><strong>${escapeHTML(wf.name)}</strong> <span class="badge wf-status-${(wf.status || 'DRAFT').toLowerCase()}">${escapeHTML(wf.status || 'DRAFT')}</span></p>
        <p class="text-muted" style="margin:4px 0">Key: ${escapeHTML(wf.key)} · Version ${wf.version || 1} · Initial state: ${escapeHTML(wf.initial_state || '—')}</p>
        <p class="text-muted" style="margin:4px 0">States: ${(wf.states || []).length} · Transitions: ${(wf.transitions || []).length}</p>
      `;
    }
  },

  async loadDashboard() {
    const tbody = document.getElementById('case-table-body');
    const statsEl = document.getElementById('dashboard-stats');
    const loadingEl = document.getElementById('dashboard-stats-loading');
    const errorEl = document.getElementById('dashboard-stats-error');
    const emptyEl = document.getElementById('dashboard-empty');

    // Show loading state
    if (loadingEl) loadingEl.classList.remove('hidden');
    if (errorEl) errorEl.classList.add('hidden');
    if (emptyEl) emptyEl.classList.add('hidden');
    if (tbody) tbody.innerHTML = '';
    if (statsEl) statsEl.innerHTML = '';

    try {
      const casesRes = await api('GET', this.orgPath('/cases?per_page=200'));
      const cases = casesRes.data || [];

      let stats = null;
      try {
        const statsRes = await api('GET', this.orgPath('/cases/dashboard/statistics'));
        stats = statsRes.data || {};
      } catch (e) {
        try { this._terminalStates = computeTerminalStateSet(await this.loadAllWorkflows()); } catch (_) {}
        stats = this.computeDashboardStats(cases);
      }

      this.renderDashboardStats(stats, cases);

      // Render recent cases table (limit to 20 most recent)
      const recentCases = cases
        .sort((a, b) => new Date(b.updated_at) - new Date(a.updated_at))
        .slice(0, 20);

      if (!recentCases.length) {
        if (emptyEl) emptyEl.classList.remove('hidden');
        if (tbody) tbody.innerHTML = '';
      } else {
        if (tbody) {
          tbody.innerHTML = recentCases.map(c => `
            <tr style="cursor:pointer" onclick="router.navigate('case','${c.id}')">
              <td>${escapeHTML(c.case_number)}</td>
              <td>${escapeHTML(c.title)}</td>
              <td><span class="badge case-status ${cssStateClass(c.status)}">${escapeHTML(c.status)}</span></td>
              <td>${escapeHTML(c.service_type)}</td>
              <td>${escapeHTML(c.priority)}</td>
              <td>${c.updated_at ? new Date(c.updated_at).toLocaleString() : '—'}</td>
            </tr>
          `).join('');
        }
      }
    } catch (err) {
      console.error(err);
      if (errorEl) {
        errorEl.textContent = `Failed to load dashboard: ${escapeHTML(err.message)}`;
        errorEl.classList.remove('hidden');
      }
      if (tbody) tbody.innerHTML = '<tr><td colspan="6" class="empty" style="color:var(--danger)">Error loading cases</td></tr>';
    if (statsEl) statsEl.innerHTML = '';
    const breakdownEl = document.getElementById('dashboard-breakdown');
    if (breakdownEl) { breakdownEl.classList.add('hidden'); breakdownEl.innerHTML = ''; }
    } finally {
      if (loadingEl) loadingEl.classList.add('hidden');
    }
  },

  computeDashboardStats(cases) {
    const statusCounts = {};
    const serviceTypeCounts = {};
    const priorityCounts = {};

    for (const c of cases) {
      statusCounts[c.status] = (statusCounts[c.status] || 0) + 1;
      serviceTypeCounts[c.service_type] = (serviceTypeCounts[c.service_type] || 0) + 1;
      priorityCounts[c.priority] = (priorityCounts[c.priority] || 0) + 1;
    }

    const terminalStates = this._terminalStates || new Set(FALLBACK_TERMINAL_STATES);
    const isClosed = s => !s || terminalStates.has(s);
    const closedCases = cases.filter(c => isClosed(c.status)).length;
    const openCases = cases.length - closedCases;
    const urgentCases = cases.filter(c => c.priority === 'URGENT').length;

    return {
      total: cases.length,
      open: openCases,
      closed: closedCases,
      urgent: urgentCases,
      byStatus: statusCounts,
      byServiceType: serviceTypeCounts,
      byPriority: priorityCounts,
    };
  },

  renderDashboardStats(stats, cases) {
    const el = document.getElementById('dashboard-stats');
    if (!el) return;
    const statCards = [
      { label: 'Total Cases', value: (stats && stats.total != null) ? stats.total : (cases || []).length, key: 'total' },
      { label: 'Open Cases', value: (stats && stats.open != null) ? stats.open : 0, key: 'open' },
      { label: 'Closed Cases', value: (stats && stats.closed != null) ? stats.closed : 0, key: 'closed' },
      { label: 'Rejected Cases', value: (stats && stats.rejected != null) ? stats.rejected : 0, key: 'rejected' },
      { label: 'Urgent Cases', value: (stats && stats.urgent != null) ? stats.urgent : 0, key: 'urgent' },
      { label: 'Service Types', value: (stats && stats.by_service_type) ? Object.keys(stats.by_service_type).length : (stats && stats.byServiceType ? Object.keys(stats.byServiceType).length : 0), key: 'serviceTypes' },
    ];

    let html = statCards.map(s => `
      <div class="stat-card" role="status" aria-label="${s.label}: ${s.value}">
        <div class="stat-value">${s.value}</div>
        <div class="stat-label">${escapeHTML(s.label)}</div>
      </div>
    `).join('');

    el.innerHTML = html;

    const breakdownEl = document.getElementById('dashboard-breakdown');
    if (breakdownEl) {
      const byStatus = (stats && (stats.by_status || stats.byStatus)) || {};
      const statusKeys = Object.keys(byStatus);
      if (statusKeys.length) {
        let bhtml = '<div class="stat-breakdown"><h3>Cases by Status</h3></div>';
        statusKeys.forEach(k => {
          bhtml += `<div class="stat-breakdown-grid"><span class="badge case-status ${cssStateClass(k)}">${escapeHTML(k)}</span><span class="stat-breakdown-count">${byStatus[k]}</span></div>`;
        });
        breakdownEl.innerHTML = bhtml;
        breakdownEl.classList.remove('hidden');
      } else {
        breakdownEl.classList.add('hidden');
        breakdownEl.innerHTML = '';
      }
    }
  },

  async loadCase(id) {
    const loadingEl = document.getElementById('case-loading');
    const errorEl = document.getElementById('case-error');
    const contentEl = document.getElementById('case-content');

    // Show loading, hide error and content
    if (loadingEl) loadingEl.classList.remove('hidden');
    if (errorEl) errorEl.classList.add('hidden');
    if (contentEl) contentEl.style.display = 'none';

    try {
      const res = await api('GET', this.orgPath(`/cases/${id}`));
      this.currentCase = res.data;
      document.getElementById('case-title').textContent = res.data.title;
      document.getElementById('case-number').textContent = `Case #${escapeHTML(res.data.case_number)}`;
      document.getElementById('case-status').textContent = res.data.status;
      document.getElementById('case-status').className = `badge case-status ${cssStateClass(res.data.status)}`;
      document.getElementById('case-service').textContent = `Service: ${escapeHTML(res.data.service_type)}`;
      document.getElementById('case-priority-badge').textContent = `Priority: ${escapeHTML(res.data.priority)}`;
      document.getElementById('case-desc').textContent = res.data.description || 'No description provided.';

      this.renderServiceBanner(res.data.service_type);
      this.currentCase = res.data;
      await this.loadWorkflow(id);
      this._terminalStates = computeTerminalStateSet(this.currentWorkflow?.definition ? [this.currentWorkflow.definition] : []);

      const isTerminal = isCaseTerminal(res.data.status, this.currentWorkflow, this._terminalStates);
      const terminalNotice = document.getElementById('case-terminal-notice');
      if (terminalNotice) { terminalNotice.classList.toggle('hidden', !isTerminal); }
      this.renderWorkflowProgress();
      this._cachedTransitions = this.currentWorkflow?.instance?.current_state
        ? await this.loadWorkflowTransitions(id)
        : [];
      await this.loadCaseSections(id, this._cachedTransitions);
      this.renderSectionActions();
      this.renderActions();
      await this.loadTimeline(id);

      // Show content, hide loading
      if (loadingEl) loadingEl.classList.add('hidden');
      if (contentEl) contentEl.style.display = 'block';
    } catch (err) {
      if (loadingEl) loadingEl.classList.add('hidden');
      if (errorEl) {
        errorEl.textContent = `Failed to load case: ${escapeHTML(err.message)}`;
        errorEl.classList.remove('hidden');
      }
      if (contentEl) contentEl.style.display = 'none';
    }
  },

  async loadWorkflow(caseId) {
    try {
      const res = await api('GET', this.orgPath(`/cases/${caseId}/workflow`));
      this.currentWorkflow = res.data;
      if (this.currentWorkflow && this.currentWorkflow.instance) {
        const inst = this.currentWorkflow.instance;
        const def = this.currentWorkflow.definition;

        const workflowMeta = document.getElementById('workflow-meta');
        if (workflowMeta) workflowMeta.classList.remove('hidden');

        document.getElementById('workflow-name').textContent = def ? `${escapeHTML(def.name)} (${def.key}, v${def.version || 1})` : 'Unknown';

        const defStatus = document.getElementById('workflow-def-status');
        if (defStatus && def) {
          defStatus.textContent = def.status || 'DRAFT';
          defStatus.className = `badge wf-status-${(def.status || 'DRAFT').toLowerCase()}`;
        }

        const stateBadge = document.getElementById('workflow-instance-state');
        const stateDesc = document.getElementById('workflow-state-desc');
        if (stateBadge && inst.current_state) {
          stateBadge.textContent = `State: ${escapeHTML(inst.current_state)}`;
          stateBadge.className = `badge ${cssStateClass(inst.current_state)}`;
          stateBadge.style.display = 'inline-block';
        }
        if (stateDesc && def) {
          const stateDef = def.states ? def.states.find(s => s.key === inst.current_state) : null;
          stateDesc.textContent = stateDef ? (stateDef.description || stateDef.name) : '';
        }
      } else {
        document.getElementById('workflow-meta')?.classList.add('hidden');
        const stateBadge = document.getElementById('workflow-instance-state');
        if (stateBadge) stateBadge.style.display = 'none';
      }
    } catch (err) {
      this.currentWorkflow = null;
      document.getElementById('workflow-meta')?.classList.add('hidden');
      const stateBadge = document.getElementById('workflow-instance-state');
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

    let cleanHtml = '<div class="workflow-steps-container" role="list" aria-label="Workflow progress">';
    stateKeys.forEach((state, idx) => {
      let cls = 'workflow-step';
      if (idx < currentIndex) cls += ' completed';
      else if (idx === currentIndex) cls += ' active';
      else cls += ' pending';
      if (isTerminal && idx <= currentIndex) cls += ' completed';
      const stateDef = sortedStates[idx];
      const label = stateDef?.name || state;
      const description = stateDef?.description || '';
      cleanHtml += `
        <div class="${cls}" data-state="${escapeHTML(state)}" role="listitem" aria-current="${idx === currentIndex ? 'step' : 'false'}">
          <div class="step-marker" aria-hidden="true">
            ${idx < currentIndex ? '✓' : (idx === currentIndex ? '' : (idx + 1))}
          </div>
          <div class="step-content">
            <span class="step-label">${escapeHTML(label)}</span>
            ${description ? `<span class="step-description">${escapeHTML(description)}</span>` : ''}
          </div>
        </div>
      `;
      if (idx < stateKeys.length - 1) {
        const connectorCls = (idx < currentIndex || (isTerminal && idx <= currentIndex)) ? 'workflow-connector completed' : 'workflow-connector';
        cleanHtml += `<div class="${connectorCls}" aria-hidden="true"></div>`;
      }
    });
    cleanHtml += '</div>';

    cleanHtml += `
      <div class="workflow-legend" aria-hidden="true">
        <span class="legend-item"><span class="legend-dot completed"></span> Completed</span>
        <span class="legend-item"><span class="legend-dot active"></span> Current</span>
        <span class="legend-item"><span class="legend-dot pending"></span> Pending</span>
      </div>
    `;

    container.innerHTML = cleanHtml;
  },

  async renderSectionActions() {
    if (!this.currentCase) return;

    const isTerminal = isCaseTerminal(this.currentCase.status, this.currentWorkflow, this._terminalStates);

    const containers = {
      eligibility: document.getElementById('sec-eligibility'),
      evidence: document.getElementById('sec-evidence'),
      assessment: document.getElementById('sec-assessment'),
      decision: document.getElementById('sec-decision'),
      assistance: document.getElementById('sec-assistance'),
      followup: document.getElementById('sec-followup')
    };

    let availableTransitions = [];
    if (!isTerminal && this.currentWorkflow?.instance?.current_state) {
      availableTransitions = this._cachedTransitions || [];
      if (!availableTransitions.length) {
        try {
          const res = await api('GET', this.orgPath(`/cases/${this.currentCase.id}/workflow/transitions`));
          availableTransitions = res.data || [];
          this._cachedTransitions = availableTransitions;
        } catch (err) {
          console.warn('Could not fetch workflow transitions:', err);
        }
      }
    }

    const sectionTransitionMap = this.buildSectionTransitionMap(availableTransitions);

    for (const [name, el] of Object.entries(containers)) {
      if (!el) continue;
      el.querySelectorAll('.section-action-btn').forEach(btn => btn.remove());

      const hasData = el.querySelector('.empty') === null && el.textContent.trim().length > 0;
      if (hasData) continue;

      if (isTerminal) continue;

      const relevantTransitions = sectionTransitionMap[name] || [];
      const hasAvailableTransition = availableTransitions.some(t => relevantTransitions.includes(t.key));

      const actionBtn = document.createElement('button');
      actionBtn.className = 'btn section-action-btn';
      actionBtn.type = 'button';
      const sectionDomain = getServiceDomain(this.currentCase.service_type);
      const title = sectionDomain?.sections?.[name]?.title || name.charAt(0).toUpperCase() + name.slice(1);
      actionBtn.innerHTML = `<span class="icon" aria-hidden="true">+</span> Add ${title}`;
      actionBtn.style.marginBottom = '8px';

      if (!hasAvailableTransition) {
        actionBtn.disabled = true;
        actionBtn.title = 'Not available in current workflow state';
        actionBtn.setAttribute('aria-disabled', 'true');
      } else {
        actionBtn.onclick = () => this.showSectionForm(name);
      }
      el.insertBefore(actionBtn, el.firstChild);
    }
  },

  buildSectionTransitionMap(availableTransitions) {
    const map = {
      eligibility: [], evidence: [], assessment: [],
      decision: [], assistance: [], followup: []
    };

    const matchedKeys = new Set();

    for (const t of availableTransitions) {
      const key = (t.key || '').toLowerCase();
      const name = (t.name || '').toLowerCase();

      let matched = false;
      if (key.includes('elig') || key === 'open' || key === 'review' || key === 'reopen') { map.eligibility.push(t.key); matched = true; }
      if (key.includes('evidence') || key.includes('doc') || key.includes('upload') || key === 'open' || key === 'review') { map.evidence.push(t.key); matched = true; }
      if (key.includes('assess')) { map.assessment.push(t.key); matched = true; }
      if (key.includes('decide') || key.includes('approve') || key.includes('reject')) { map.decision.push(t.key); matched = true; }
      if (key.includes('assist') || key.includes('start') || key.includes('progress') || key.includes('follow') || key === 'complete') { map.assistance.push(t.key); matched = true; }
      if (key.includes('follow') || key === 'complete') { map.followup.push(t.key); matched = true; }

      if (!map.eligibility.includes(t.key) && (name.includes('elig') || name.includes('open') || name.includes('review'))) { map.eligibility.push(t.key); matched = true; }
      if (!map.evidence.includes(t.key) && (name.includes('evidence') || name.includes('document') || name.includes('open') || name.includes('review'))) { map.evidence.push(t.key); matched = true; }
      if (!map.assessment.includes(t.key) && name.includes('assess')) { map.assessment.push(t.key); matched = true; }
      if (!map.decision.includes(t.key) && (name.includes('decide') || name.includes('approve') || name.includes('reject'))) { map.decision.push(t.key); matched = true; }
      if (!map.assistance.includes(t.key) && (name.includes('assist') || name.includes('start') || name.includes('progress') || name.includes('follow'))) { map.assistance.push(t.key); matched = true; }
      if (!map.followup.includes(t.key) && name.includes('follow')) { map.followup.push(t.key); matched = true; }

      if (matched) matchedKeys.add(t.key);
    }

    for (const k of Object.keys(map)) {
      map[k] = [...new Set(map[k])];
    }

    // For generic workflows where no transition keywords match section names,
    // enable all sections since sections are independent data collections.
    const anyUnmatched = availableTransitions.some(t => !matchedKeys.has(t.key));
    if (anyUnmatched && availableTransitions.length > 0) {
      for (const k of Object.keys(map)) {
        if (map[k].length === 0) {
          map[k] = availableTransitions.map(t => t.key);
        }
      }
    }

    return map;
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

  async loadCaseSections(id, cachedTransitions) {
    if (cachedTransitions) {
      this._cachedTransitions = cachedTransitions;
    }
    await this._loadCaseSectionsWithTransitions(id);
  },

  async _loadCaseSectionsWithTransitions(id) {
    await this.loadSection('eligibility', this.orgPath(`/eligibilities/by-service-request/${id}`));
    await this.loadSection('evidence', this.orgPath(`/evidence/by-service-request/${id}`));
    await this.loadSection('assessment', this.orgPath(`/assessments/by-service-request/${id}`));
    await this.loadSection('decision', this.orgPath(`/decisions/by-service-request/${id}`));
    await this.loadSection('assistance', this.orgPath(`/assistance/by-service-request/${id}`));
    await this.loadSection('followup', this.orgPath(`/follow-ups/by-service-request/${id}`));
  },

async loadSection(name, path) {
    const el = document.getElementById(`sec-${name}`);
    if (!el) return;

    // Show loading state
    el.innerHTML = '<div class="loading" style="padding:16px;text-align:center"><div class="loading-spinner"></div> Loading…</div>';

    try {
      const res = await api('GET', path);
      const data = res.data;
      const domain = getServiceDomain(this.currentCase?.service_type);
      const hint = domain?.sections[name]?.hint;
      const isTerminal = isCaseTerminal(this.currentCase?.status, this.currentWorkflow, this._terminalStates);
      const ts = this.buildSectionTransitionMap(this._cachedTransitions || []);
      const rel = ts[name] || [];
      const hasTransition = (this._cachedTransitions || []).some(t => rel.includes(t.key));

      let statusLabel = '';
      if (data) {
        statusLabel = '<span class="badge completed" style="margin-bottom:6px">Completed</span> ';
      } else if (!isTerminal && hasTransition) {
        statusLabel = '<span class="badge" style="margin-bottom:6px">Available</span> ';
      } else if (!isTerminal) {
        statusLabel = '<span class="badge" style="background:#fef3c7;color:#92400e;margin-bottom:6px">Not applicable</span> ';
      }
      let hintHTML = hint ? `<p class="section-hint">${escapeHTML(hint)}</p>` : '';
      hintHTML = statusLabel + hintHTML;

      if (!data) {
        // Determine appropriate empty state message
        let emptyMessage = '';
        let emptyClass = 'empty';
        if (isTerminal) {
          emptyMessage = 'Not recorded (case is closed)';
          emptyClass += ' terminal-empty';
        } else {
          // Check if this section is relevant for current workflow state using cached transitions
          const state = this.currentWorkflow?.instance?.current_state;
          const sectionTransitionMap = this.buildSectionTransitionMap(this._cachedTransitions || []);
          const relevantTransitions = sectionTransitionMap[name] || [];
          const availableTransitions = this._cachedTransitions || [];
          const hasAvailableTransition = availableTransitions.some(t => relevantTransitions.includes(t.key));

          if (!hasAvailableTransition && state) {
            emptyMessage = 'Not applicable in current workflow state';
            emptyClass += ' not-applicable';
          } else {
            emptyMessage = 'Not yet recorded';
            emptyClass += ' not-recorded';
          }
        }
        el.innerHTML = `${hintHTML}<p class="${emptyClass}">${escapeHTML(emptyMessage)}</p>`;
        return;
      }

      // Render data based on section type
      if (name === 'eligibility') {
        const resultClass = `badge ${(data.result || '').toLowerCase().replace('_','-')}`;
        el.innerHTML = `${hintHTML}<strong>Result:</strong> <span class="${resultClass}">${escapeHTML(data.result)}</span><br><strong>Explanation:</strong> ${escapeHTML(data.explanation)}`;
      } else if (name === 'evidence') {
        if (Array.isArray(data)) {
          if (!data.length) { el.innerHTML = hintHTML + '<p class="empty not-recorded">No evidence recorded yet</p>'; return; }
          el.innerHTML = hintHTML + '<table><thead><tr><th>Type</th><th>Description</th></tr></thead><tbody>' +
            data.map(e => `<tr><td>${escapeHTML(e.type)}</td><td>${escapeHTML(e.description)}</td></tr>`).join('') +
            '</tbody></table>';
        } else {
          el.innerHTML = `${hintHTML}<strong>Type:</strong> ${escapeHTML(data.type)}<br><strong>Description:</strong> ${escapeHTML(data.description)}`;
        }
      } else if (name === 'assessment') {
        if (Array.isArray(data)) {
          if (!data.length) { el.innerHTML = hintHTML + '<p class="empty not-recorded">No assessment recorded yet</p>'; return; }
          const a = data[0];
          el.innerHTML = `${hintHTML}<strong>Findings:</strong> ${escapeHTML(a.findings)}<br><strong>Needs:</strong> ${escapeHTML(a.needs_identified || 'N/A')}<br><strong>Recommendation:</strong> ${escapeHTML(a.recommendation)}`;
        } else {
          el.innerHTML = `${hintHTML}<strong>Findings:</strong> ${escapeHTML(data.findings)}<br><strong>Needs:</strong> ${escapeHTML(data.needs_identified || 'N/A')}<br><strong>Recommendation:</strong> ${escapeHTML(data.recommendation)}`;
        }
        } else if (name === 'decision') {
          if (Array.isArray(data)) {
            if (!data.length) { el.innerHTML = hintHTML + '<p class="empty not-recorded">No decision recorded yet</p>'; return; }
            const d = data[0];
            el.innerHTML = `${hintHTML}<strong>Decision:</strong> <span class="badge ${cssStateClass(d.decision)}">${escapeHTML(d.decision)}</span><br><strong>Reason:</strong> ${escapeHTML(d.reason)}`;
          } else {
            el.innerHTML = `${hintHTML}<strong>Decision:</strong> <span class="badge ${cssStateClass(data.decision)}">${escapeHTML(data.decision)}</span><br><strong>Reason:</strong> ${escapeHTML(data.reason)}`;
          }
        } else if (name === 'assistance') {
          if (Array.isArray(data)) {
            if (!data.length) { el.innerHTML = hintHTML + '<p class="empty not-recorded">No assistance recorded yet</p>'; return; }
            el.innerHTML = hintHTML + data.map(a => {
              const statusClass = cssStateClass(a.status);
              return `<div><strong class="badge ${statusClass}">${escapeHTML(a.type)}</strong> - <span class="badge assistance-status ${statusClass}">${escapeHTML(a.status)}</span><br>${escapeHTML(a.description)}</div>`;
            }).join('');
          } else {
            const statusClass = cssStateClass(data.status);
            el.innerHTML = `${hintHTML}<strong class="badge ${statusClass}">${escapeHTML(data.type)}</strong> - <span class="badge assistance-status ${statusClass}">${escapeHTML(data.status)}</span><br>${escapeHTML(data.description)}`;
          }
        } else if (name === 'followup') {
        if (Array.isArray(data)) {
          if (!data.length) { el.innerHTML = hintHTML + '<p class="empty not-recorded">No follow-up scheduled yet</p>'; return; }
          el.innerHTML = hintHTML + data.map(f => `<div><strong>${escapeHTML(f.scheduled_date)}</strong> - ${escapeHTML(f.outcome)}</div>`).join('');
        } else {
          el.innerHTML = `${hintHTML}<strong>${escapeHTML(data.scheduled_date)}</strong> - ${escapeHTML(data.outcome)}`;
        }
      }
    } catch (err) {
      if (err.message && err.message.includes('404')) {
        const isTerminal = isCaseTerminal(this.currentCase?.status, this.currentWorkflow, this._terminalStates);
        let emptyMessage = isTerminal ? 'Not recorded (case is closed)' : 'Not yet recorded';
        let emptyClass = isTerminal ? 'empty terminal-empty' : 'empty not-recorded';
        el.innerHTML = `<p class="${emptyClass}">${escapeHTML(emptyMessage)}</p>`;
      } else if (err.message && err.message.includes('403')) {
        el.innerHTML = `<p class="empty not-applicable">Not applicable for your role</p>`;
      } else {
        el.innerHTML = `<p style="color:var(--danger)">Error loading: ${escapeHTML(err.message)}</p>`;
      }
    }
  },

  btn(label, onclick) {
    return `<button class="btn" onclick="app.${onclick}"><span class="icon">⚡</span> ${label}</button>`;
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

    const isTerminal = isCaseTerminal(this.currentCase.status, this.currentWorkflow, this._terminalStates);

    const renderFrom = async (transitions) => {
      this._cachedTransitions = transitions;
      const tan = document.getElementById('terminal-action-notice');
      if (tan) tan.classList.add('hidden');
      let html = '<div style="display:flex;gap:8px;flex-wrap:wrap">';

      if (isTerminal) {
        const terminalState = this.currentWorkflow?.instance?.current_state || this.currentCase.status;
        container.innerHTML = '';
        const tsn = document.getElementById('terminal-state-name');
        if (tsn) tsn.textContent = escapeHTML(terminalState);
        const tan = document.getElementById('terminal-action-notice');
        if (tan) tan.classList.remove('hidden');
        return;
      }

      for (const t of transitions) {
        html += this.btn(t.name || t.key, `workflowTransition('${t.key}')`);
      }
      html += '</div>';

      const state = this.currentWorkflow?.instance?.current_state;
      if (state) {
        if (!transitions.length) {
          html += `<p class="empty" style="margin-top:8px;">No actions available from the current state (<strong>${escapeHTML(state)}</strong>). The workflow may require conditions to be met.</p>`;
        }
      } else {
        html += '<p class="empty" style="margin-top:8px;">No workflow instance available for this case.</p>';
      }
      container.innerHTML = html;
    };

    if (this._cachedTransitions && this._cachedTransitions.length) {
      renderFrom(this._cachedTransitions);
    } else {
      this.loadWorkflowTransitions(this.currentCase.id).then(renderFrom);
    }
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

  showModal(id) {
    const el = document.getElementById(id);
    if (!el) return;
    el.classList.remove('hidden');
    if (!el.style.position) {
      el.classList.add('modal-overlay');
    }
  },
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
  wfUpdate(id, body) { return api('PUT', this.orgPath(`/workflows/${id}`), body); },
  wfDelete(id) { return api('DELETE', this.orgPath(`/workflows/${id}`)); },
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
    actions.innerHTML = `<button class="btn secondary" onclick="router.navigate('dashboard')"><span class="icon">🏠</span> Dashboard</button>` +
      (currentUserIsAdmin()
        ? ` <button class="btn" onclick="app.showWorkflowCreateView()"><span class="icon">⚙️</span> Create Workflow</button>`
        : '');
  },

  renderWorkflowRow(d) {
    const states = d.states || [];
    const transitions = d.transitions || [];
    const terminal = getTerminalStates(d);
    const statusClass = (d.status || 'DRAFT').toLowerCase();
    const badge = `<span class="badge wf-status-${statusClass}" style="text-transform:none">${escapeHTML(d.status)}</span>`;
    const canActivate = d.status === 'DRAFT';
    const canArchive = d.status === 'ACTIVE';
    const canEdit = d.status === 'DRAFT';
    const canDelete = d.status === 'DRAFT';
    let actionBtns = `<button class="btn secondary sm" style="font-size:0.8rem" onclick="app.showWorkflowDetail('${d.id}')"><span class="icon">👁️</span> View</button>`;
    if (canEdit) {
      actionBtns += ` <button class="btn secondary sm" style="font-size:0.8rem;margin-left:4px" onclick="app.showWorkflowEditView('${d.id}')"><span class="icon">✏️</span> Edit</button>`;
    }
    if (canActivate) {
      actionBtns += ` <button class="btn sm" style="font-size:0.8rem;margin-left:4px" onclick="app.activateWorkflow('${d.id}')"><span class="icon">▶️</span> Activate</button>`;
    }
    if (canArchive) {
      actionBtns += ` <button class="btn warning sm" style="font-size:0.8rem;margin-left:4px" onclick="app.archiveWorkflow('${d.id}')"><span class="icon">📦</span> Archive</button>`;
    }
    if (canDelete) {
      actionBtns += ` <button class="btn danger sm" style="font-size:0.8rem;margin-left:4px" onclick="app.deleteWorkflow('${d.id}')"><span class="icon">🗑️</span> Delete</button>`;
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
        else html += `<button class="btn secondary sm" style="margin-right:4px" onclick="app.loadWorkflowList(${i})">${i}</button>`;
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

  async deleteWorkflow(id) {
    if (!confirm('Delete this workflow definition? This action is irreversible and will remove all associated states and transitions.')) return;
    try {
      await this.wfDelete(id);
      showToast('Workflow definition deleted', 'success');
      if (this.currentWorkflowDef && this.currentWorkflowDef.id === id) {
        this.showWorkflowsView();
      }
      this.loadWorkflowList(this.workflowListPage || 1);
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
    let actions = `<button class="btn secondary sm" onclick="app.showWorkflowsView()"><span class="icon">←</span> Back to Workflows</button>`;
    if (isDraft) {
      actions += ` <button class="btn sm" onclick="app.activateWorkflow('${def.id}')"><span class="icon">▶️</span> Activate</button>`;
    } else if (isActive) {
      actions += ` <button class="btn sm" onclick="router.navigate('new-case', '${def.id}')"><span class="icon">➕</span> Use for New Case</button>`;
      actions += ` <button class="btn warning sm" onclick="app.archiveWorkflow('${def.id}')"><span class="icon">📦</span> Archive</button>`;
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

  async showWorkflowEditView(id) {
    if (!currentUserIsAdmin()) {
      showToast('Only administrators can edit workflow definitions', 'error');
      return;
    }
    this.switchView('view-new-workflow');
    this.workflowEditId = id;
    this.workflowCreateErrors = [];
    const container = document.getElementById('wf-create-container');
    if (!container) return;
    container.innerHTML = '<p class="empty">Loading workflow definition…</p>';
    try {
      const res = await this.wfGet(id);
      this.workflowDraft = {
        key: res.data.key,
        name: res.data.name,
        description: res.data.description || '',
        version: res.data.version || 1,
        initial_state: res.data.initial_state || '',
        states: (res.data.states || []).map(s => ({
          key: s.key,
          name: s.name || '',
          description: s.description || '',
          category: s.category || '',
          terminal: !!s.terminal,
          display_order: s.display_order || 0,
          responsible_role: s.responsible_role || '',
        })),
        transitions: (res.data.transitions || []).map(t => ({
          key: t.key || '',
          name: t.name || '',
          from_state: t.from_state || '',
          to_state: t.to_state || '',
          description: t.description || '',
          conditions: t.conditions || [],
          allowed_roles: t.allowed_roles || [],
          active: t.active !== false,
        })),
      };
      container.innerHTML = '';
      this.renderWorkflowEditForm();
    } catch (err) {
      container.innerHTML = `<p class="empty" style="color:var(--danger)">Error loading workflow: ${escapeHTML(err.message)}</p>`;
    }
  },

  renderWorkflowEditForm() {
    this.renderWorkflowCreateForm(true);
  },

  renderWorkflowCreateForm(isEdit) {
    const c = this.workflowDraft;
    const stateKeys = c.states.map(s => s.key).filter(k => k);
    const container = document.getElementById('wf-create-container');
    if (!container) return;
    const title = isEdit ? 'Edit Workflow Definition' : 'Create Workflow Definition';
    const submitLabel = isEdit ? 'Update Definition' : 'Create Definition';
    container.innerHTML = `
      <div class="header" style="margin-bottom:16px">
        <h1>${title}</h1>
        <nav><button class="btn secondary sm" onclick="app.cancelWorkflowCreate()"><span class="icon">✖</span> Cancel</button></nav>
      </div>
      <div class="card">
        <h2>Workflow</h2>
        <p class="section-hint">${isEdit ? 'Editing a draft definition. Only draft definitions can be modified.' : 'A new definition is created in <strong>DRAFT</strong> status. Activate it to start using it for new cases.'}</p>
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
          <button class="btn success" onclick="app.addWorkflowState()"><span class="icon">➕</span> Add State</button>
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
          <button class="btn success" onclick="app.addWorkflowTransition()"><span class="icon">➕</span> Add Transition</button>
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
          <button class="btn secondary" onclick="app.cancelWorkflowCreate()"><span class="icon">←</span> Back</button>
          <button class="btn" onclick="app.validateAndSubmitWorkflow()">${submitLabel}</button>
        </div>
      </div>
      `;
    this.renderWorkflowStatesEditor();
    this.renderWorkflowTransitionsEditor();
    this.refreshInitialSelect();
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
          <button class="btn secondary sm" onclick="app.moveStateUp(${i})">↑</button>
          <button class="btn secondary sm" onclick="app.moveStateDown(${i})">↓</button>
          <button class="btn danger sm" onclick="app.removeWorkflowState(${i})">✕</button>
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
    const option = (val, label, selected) => `<option value="${escapeHTML(val)}"${selected ? ' selected' : ''}>${escapeHTML(label)}</option>`;
    const selectOptions = (selected) => [option('', '— select —', !selected), ...stateKeys.map(k => option(k, k, k === selected))].join('');
    tbody.innerHTML = c.transitions.map((t, i) => {
      return `<tr>
        <td><input style="width:110px" value="${escapeHTML(t.key)}" oninput="app.onTransitionFieldChange(${i}, 'key', this.value)"></td>
        <td><input value="${escapeHTML(t.name)}" oninput="app.onTransitionFieldChange(${i}, 'name', this.value)"></td>
        <td><select onchange="app.onTransitionFieldChange(${i}, 'from_state', this.value)">${selectOptions(t.from_state)}</select></td>
        <td><select onchange="app.onTransitionFieldChange(${i}, 'to_state', this.value)">${selectOptions(t.to_state)}</select></td>
        <td style="text-align:center"><input type="checkbox" ${t.active ? 'checked' : ''} onchange="app.onTransitionFieldChange(${i}, 'active', this.checked)"></td>
        <td><input value="${Array.isArray(t.allowed_roles) ? t.allowed_roles.join(', ') : ''}" oninput="app.onTransitionFieldChange(${i}, 'allowed_roles_raw', this.value)" placeholder="admin, staff"></td>
        <td><input style="width:140px" value="${Array.isArray(t.conditions) && t.conditions.length ? JSON.stringify(t.conditions) : ''}" oninput="app.onTransitionFieldChange(${i}, 'conditions_raw', this.value)" placeholder="[]"></td>
        <td><input value="${escapeHTML(t.description)}" oninput="app.onTransitionFieldChange(${i}, 'description', this.value)"></td>
        <td style="white-space:nowrap"><button class="btn danger sm" onclick="app.removeWorkflowTransition(${i})">✕</button></td>
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
    if (this.workflowEditId) {
      this.submitWorkflowUpdate();
    } else {
      this.submitWorkflowCreate();
    }
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

  async submitWorkflowUpdate() {
    const payload = this.buildWorkflowPayload();
    try {
      const res = await this.wfUpdate(this.workflowEditId, payload);
      const def = res.data || payload;
      showToast('Workflow definition updated', 'success');
      this.workflowDraft = null;
      this.workflowEditId = null;
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
    this.workflowEditId = null;
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

router.on('new-case', (preselectId) => {
  document.getElementById('view-login').classList.add('hidden');
  document.getElementById('view-dashboard').classList.add('hidden');
  document.getElementById('view-case').classList.add('hidden');
  document.getElementById('view-new-case').classList.remove('hidden');
  document.getElementById('view-workflows').classList.add('hidden');
  document.getElementById('view-workflow-detail').classList.add('hidden');
  document.getElementById('view-new-workflow').classList.add('hidden');
  app.selectedWorkflow = null;
  app.loadNewCaseWorkflows(preselectId);
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
