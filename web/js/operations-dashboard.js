const OperationsDashboard = {
  metrics: null,
  loading: false,
  filters: {
    period: 'daily',
    workflow_key: '',
    status: '',
    start_date: '',
    end_date: '',
  },

  init() {
    this.cacheElements();
    this.bindFilters();
    this.loadMetrics();
  },

  cacheElements() {
    this.el = {
      loading: document.getElementById('ops-loading'),
      error: document.getElementById('ops-error'),
      errorMsg: document.getElementById('ops-error-msg'),
      retry: document.getElementById('ops-retry'),
      summary: document.getElementById('ops-summary'),
      breakdown: document.getElementById('ops-breakdown'),
      workflowDist: document.getElementById('ops-workflow-dist'),
      stateDuration: document.getElementById('ops-state-duration'),
      cycleTime: document.getElementById('ops-cycle-time'),
      aging: document.getElementById('ops-aging'),
      pending: document.getElementById('ops-pending'),
      decisions: document.getElementById('ops-decisions'),
      assistance: document.getElementById('ops-assistance'),
      evidence: document.getElementById('ops-evidence'),
      infoRequired: document.getElementById('ops-info-required'),
      period: document.getElementById('ops-period'),
      workflowKey: document.getElementById('ops-workflow-key'),
      status: document.getElementById('ops-status'),
      startDate: document.getElementById('ops-start-date'),
      endDate: document.getElementById('ops-end-date'),
      apply: document.getElementById('ops-apply'),
      refresh: document.getElementById('ops-refresh'),
      calculatedAt: document.getElementById('ops-calculated-at'),
      unavailable: document.getElementById('ops-unavailable'),
    };
  },

  bindFilters() {
    if (this.el.apply) {
      this.el.apply.addEventListener('click', () => {
        this.filters.period = this.el.period?.value || 'daily';
        this.filters.workflow_key = this.el.workflowKey?.value || '';
        this.filters.status = this.el.status?.value || '';
        this.loadMetrics();
      });
    }
    if (this.el.refresh) {
      this.el.refresh.addEventListener('click', () => this.loadMetrics());
    }
    this.bindExport('ops-export-json', 'json');
    this.bindExport('ops-export-csv', 'csv');
    this.bindExport('ops-export-report', 'report');
  },

  bindExport(id, format) {
    const btn = document.getElementById(id);
    if (!btn) return;
    btn.addEventListener('click', () => {
      triggerExport({
        format,
        scope: 'operations',
        period: this.filters.period,
        workflow_key: this.filters.workflow_key,
        status: this.filters.status,
      }).catch(() => {});
    });
  },

  async loadMetrics() {
    if (this.loading) return;
    this.loading = true;
    this.showLoading();
    this.hideError();

    try {
      const res = await getOperationsDashboard({
        period: this.filters.period,
        workflow_key: this.filters.workflow_key,
        status: this.filters.status,
      });
      this.metrics = res.data || null;
      this.render();
    } catch (err) {
      this.showError(err.message || 'Failed to load operations metrics');
    } finally {
      this.loading = false;
    }
  },

  showLoading() {
    Object.values(this.el).forEach(el => {
      if (!el) return;
      if (el.id && (el.id.startsWith('ops-') || el.id === 'ops-loading')) {
        if (el.id === 'ops-loading') el.classList.remove('hidden');
        else if (el.tagName === 'DIV' || el.tagName === 'SECTION') el.style.visibility = 'hidden';
      }
    });
  },

  hideLoading() {
    if (this.el.loading) this.el.loading.classList.add('hidden');
    Object.values(this.el).forEach(el => {
      if (!el) return;
      if (el.id && el.id.startsWith('ops-') && el.id !== 'ops-loading' && el.id !== 'ops-error') {
        if (el.tagName === 'DIV' || el.tagName === 'SECTION') el.style.visibility = 'visible';
      }
    });
  },

  showError(msg) {
    if (this.el.error) {
      this.el.error.classList.remove('hidden');
      if (this.el.errorMsg) this.el.errorMsg.textContent = msg;
    }
  },

  hideError() {
    if (this.el.error) this.el.error.classList.add('hidden');
  },

  render() {
    this.hideLoading();
    if (!this.metrics) {
      this.showError('No metrics available');
      return;
    }
    if (this.el.calculatedAt) {
      this.el.calculatedAt.textContent = 'Updated: ' + new Date(this.metrics.calculated_at).toLocaleString();
    }
    this.renderSummary();
    this.renderBreakdown();
    this.renderWorkflowDistribution();
    this.renderStateDuration();
    this.renderCycleTime();
    this.renderAgingCases();
    this.renderPendingReviews();
    this.renderDecisions();
    this.renderAssistance();
    this.renderEvidence();
    this.renderInfoRequired();
  },

  renderSummary() {
    if (!this.el.summary || !this.metrics.case_volume) return;
    const cv = this.metrics.case_volume;
    const total = cv.total_cases || 0;
    const open = cv.open_cases || 0;
    const closed = cv.closed_cases || 0;
    const rejected = cv.rejected_cases || 0;
    const aging = (this.metrics.aging_cases || []).length;
    const pending = this.metrics.pending_reviews?.pending_reviews || 0;
    const cards = [
      { label: 'Total Cases', value: total, type: 'fact' },
      { label: 'Active Cases', value: open, type: 'fact' },
      { label: 'Completed Cases', value: closed, type: 'fact' },
      { label: 'Rejected Cases', value: rejected, type: 'fact' },
      { label: 'Cases Requiring Review', value: pending, type: 'metric' },
      { label: 'Aging Cases', value: aging, type: 'metric' },
    ];
    this.el.summary.innerHTML = cards.map(c => `
      <div class="stat-card" role="status" aria-label="${c.label}: ${c.value}">
        <div class="stat-value">${c.value}</div>
        <div class="stat-label">${escapeHTML(c.label)}</div>
        ${c.type === 'metric' ? '<small class="text-muted">Calculated</small>' : ''}
      </div>
    `).join('');
  },

  renderBreakdown() {
    if (!this.el.breakdown || !this.metrics.case_volume) return;
    const cv = this.metrics.case_volume;
    const byStatus = cv.by_status || {};
    const keys = Object.keys(byStatus);
    if (!keys.length) {
      this.el.breakdown.innerHTML = '';
      return;
    }
    let html = '<div class="stat-breakdown"><h3>Cases by Status</h3></div>';
    keys.forEach(k => {
      html += `<div class="stat-breakdown-grid"><span class="badge case-status ${cssStateClass(k)}">${escapeHTML(k)}</span><span class="stat-breakdown-count">${byStatus[k]}</span></div>`;
    });
    this.el.breakdown.innerHTML = html;
  },

  renderWorkflowDistribution() {
    if (!this.el.workflowDist || !this.metrics.case_volume) return;
    const cv = this.metrics.case_volume;
    const byWorkflow = cv.by_workflow_key || {};
    const keys = Object.keys(byWorkflow);
    if (!keys.length) {
      this.el.workflowDist.innerHTML = '<p class="empty">No workflow data available.</p>';
      return;
    }
    const total = Object.values(byWorkflow).reduce((a, b) => a + b, 0);
    this.el.workflowDist.innerHTML = keys.map(k => {
      const count = byWorkflow[k];
      const pct = total > 0 ? Math.round((count / total) * 100) : 0;
      return `<div class="bar-row"><span>${escapeHTML(k)}</span><div class="bar-track"><div class="bar-fill" style="width:${pct}%"></div></div><span>${count} (${pct}%)</span></div>`;
    }).join('');
  },

  renderStateDuration() {
    if (!this.el.stateDuration) return;
    const sd = this.metrics.state_duration;
    if (!sd) {
      this.el.stateDuration.innerHTML = '<p class="empty">Insufficient data for state duration.</p>';
      return;
    }
    const avg = sd.avg_duration_hours != null ? sd.avg_duration_hours.toFixed(1) : '—';
    const median = sd.median_duration_hours != null ? sd.median_duration_hours.toFixed(1) : '—';
    const max = sd.max_duration_hours != null ? sd.max_duration_hours.toFixed(1) : '—';
    this.el.stateDuration.innerHTML = `
      <div class="metric-grid">
        <div><strong>State:</strong> ${escapeHTML(sd.workflow_key || '—')}</div>
        <div><strong>Average:</strong> ${avg} hrs</div>
        <div><strong>Median:</strong> ${median} hrs</div>
        <div><strong>Max:</strong> ${max} hrs</div>
      </div>
    `;
  },

  renderCycleTime() {
    if (!this.el.cycleTime) return;
    const ct = this.metrics.case_cycle_time;
    if (!ct) {
      this.el.cycleTime.innerHTML = '<p class="empty">Insufficient data for cycle time.</p>';
      return;
    }
    const avg = ct.avg_cycle_time_hours != null ? ct.avg_cycle_time_hours.toFixed(1) : '—';
    const median = ct.median_cycle_time_hours != null ? ct.median_cycle_time_hours.toFixed(1) : '—';
    const completed = ct.completed_cases || 0;
    this.el.cycleTime.innerHTML = `
      <div class="metric-grid">
        <div><strong>Completed Cases:</strong> ${completed}</div>
        <div><strong>Average Cycle Time:</strong> ${avg} hrs</div>
        <div><strong>Median Cycle Time:</strong> ${median} hrs</div>
      </div>
    `;
  },

  renderAgingCases() {
    if (!this.el.aging) return;
    const cases = this.metrics.aging_cases || [];
    if (!cases.length) {
      this.el.aging.innerHTML = '<p class="empty">No aging cases.</p>';
      return;
    }
    this.el.aging.innerHTML = `
      <table class="data-table" aria-label="Aging cases">
        <thead><tr><th>Workflow</th><th>Status</th><th>Age (hrs)</th><th>Service</th></tr></thead>
        <tbody>${cases.map(c => `<tr><td>${escapeHTML(c.workflow_key)}</td><td><span class="badge ${cssStateClass(c.current_state)}">${escapeHTML(c.current_state)}</span></td><td>${c.age_hours != null ? c.age_hours.toFixed(1) : '—'}</td><td>${escapeHTML(c.service_type)}</td></tr>`).join('')}</tbody>
      </table>
    `;
  },

  renderPendingReviews() {
    if (!this.el.pending) return;
    const pr = this.metrics.pending_reviews;
    if (!pr) {
      this.el.pending.innerHTML = '<p class="empty">No pending review data.</p>';
      return;
    }
    const avgWait = pr.avg_wait_time_hours != null ? pr.avg_wait_time_hours.toFixed(1) : '—';
    this.el.pending.innerHTML = `
      <div class="metric-grid">
        <div><strong>Pending:</strong> ${pr.pending_reviews || 0}</div>
        <div><strong>Assigned:</strong> ${pr.assigned_reviews || 0}</div>
        <div><strong>In Review:</strong> ${pr.in_review_reviews || 0}</div>
        <div><strong>Avg Wait:</strong> ${avgWait} hrs</div>
      </div>
    `;
  },

  renderDecisions() {
    if (!this.el.decisions) return;
    const d = this.metrics.decisions;
    if (!d) {
      this.el.decisions.innerHTML = '<p class="empty">No decision data.</p>';
      return;
    }
    const rate = d.approval_rate != null ? (d.approval_rate * 100).toFixed(1) + '%' : '—';
    this.el.decisions.innerHTML = `
      <div class="metric-grid">
        <div><strong>Total:</strong> ${d.total_decisions || 0}</div>
        <div><strong>Approved:</strong> ${d.approved || 0}</div>
        <div><strong>Rejected:</strong> ${d.rejected || 0}</div>
        <div><strong>Approval Rate:</strong> ${rate}</div>
      </div>
    `;
  },

  renderAssistance() {
    if (!this.el.assistance) return;
    const a = this.metrics.assistance_outcomes;
    if (!a) {
      this.el.assistance.innerHTML = '<p class="empty">No assistance data.</p>';
      return;
    }
    const rate = a.completion_rate != null ? (a.completion_rate * 100).toFixed(1) + '%' : '—';
    this.el.assistance.innerHTML = `
      <div class="metric-grid">
        <div><strong>Total:</strong> ${a.total_assistance || 0}</div>
        <div><strong>In Progress:</strong> ${a.in_progress || 0}</div>
        <div><strong>Completed:</strong> ${a.completed || 0}</div>
        <div><strong>Completion Rate:</strong> ${rate}</div>
      </div>
    `;
  },

  renderEvidence() {
    if (!this.el.evidence) return;
    const e = this.metrics.evidence_verification;
    if (!e) {
      this.el.evidence.innerHTML = '<p class="empty">No evidence data.</p>';
      return;
    }
    const rate = e.verification_rate != null ? (e.verification_rate * 100).toFixed(1) + '%' : '—';
    const needsReview = e.needs_review || 0;
    this.el.evidence.innerHTML = `
      <div class="metric-grid">
        <div><strong>Total:</strong> ${e.total_evidence || 0}</div>
        <div><strong>Verified:</strong> ${e.verified || 0}</div>
        <div><strong>Rejected:</strong> ${e.rejected || 0}</div>
        <div><strong>Needs Review:</strong> ${needsReview}</div>
        <div><strong>Verification Rate:</strong> ${rate}</div>
      </div>
    `;
  },

  renderInfoRequired() {
    if (!this.el.infoRequired) return;
    const ir = this.metrics.information_required;
    if (!ir) {
      this.el.infoRequired.innerHTML = '<p class="empty">No information-required data.</p>';
      return;
    }
    this.el.infoRequired.innerHTML = `
      <div class="metric-grid">
        <div><strong>Total Requests:</strong> ${ir.total_cases || 0}</div>
        <div><strong>Information Required:</strong> ${ir.information_required || 0}</div>
        <div><strong>Escalated:</strong> ${ir.escalated || 0}</div>
        <div><strong>Awaiting Info Reviews:</strong> ${ir.awaiting_info_reviews || 0}</div>
      </div>
    `;
  },
};
