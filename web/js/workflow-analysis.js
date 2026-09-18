const WorkflowAnalysis = {
  report: null,
  loading: false,
  thresholds: {
    state_accumulation_threshold: 10,
    state_duration_threshold_hours: 24,
    aging_threshold_hours: 720,
    review_backlog_threshold: 20,
    info_request_frequency_per_case: 1.5,
    cycle_time_threshold_hours: 72,
  },

  init() {
    this.cacheElements();
    this.bindControls();
    this.loadAnalysis();
  },

  cacheElements() {
    this.el = {
      loading: document.getElementById('wa-loading'),
      error: document.getElementById('wa-error'),
      errorMsg: document.getElementById('wa-error-msg'),
      retry: document.getElementById('wa-retry'),
      content: document.getElementById('wa-content'),
      calculatedAt: document.getElementById('wa-calculated-at'),
      stateAccum: document.getElementById('wa-state-accumulations'),
      stateDurations: document.getElementById('wa-state-durations'),
      closurePatterns: document.getElementById('wa-closure-patterns'),
      infoRequestPatterns: document.getElementById('wa-info-request-patterns'),
      reviewBacklog: document.getElementById('wa-review-backlog'),
      thresholdExceedances: document.getElementById('wa-threshold-exceedances'),
      apply: document.getElementById('wa-apply'),
      refresh: document.getElementById('wa-refresh'),
      stateAccumulation: document.getElementById('wa-state-accumulation'),
      stateDuration: document.getElementById('wa-state-duration'),
      aging: document.getElementById('wa-aging'),
      reviewBacklogThreshold: document.getElementById('wa-review-backlog-input'),
      infoFrequency: document.getElementById('wa-info-frequency'),
      cycleTime: document.getElementById('wa-cycle-time'),
    };
  },

  bindControls() {
    if (this.el.apply) {
      this.el.apply.addEventListener('click', () => {
        this.thresholds.state_accumulation_threshold = Number(this.el.stateAccumulation?.value) || 10;
        this.thresholds.state_duration_threshold_hours = Number(this.el.stateDuration?.value) || 24;
        this.thresholds.aging_threshold_hours = Number(this.el.aging?.value) || 720;
        this.thresholds.review_backlog_threshold = Number(this.el.reviewBacklogThreshold?.value) || 20;
        this.thresholds.info_request_frequency_per_case = Number(this.el.infoFrequency?.value) || 1.5;
        this.thresholds.cycle_time_threshold_hours = Number(this.el.cycleTime?.value) || 72;
        this.loadAnalysis();
      });
    }
    if (this.el.refresh) {
      this.el.refresh.addEventListener('click', () => this.loadAnalysis());
    }
    if (this.el.retry) {
      this.el.retry.addEventListener('click', () => this.loadAnalysis());
    }
    this.bindExport('wa-export-json', 'json');
    this.bindExport('wa-export-csv', 'csv');
    this.bindExport('wa-export-report', 'report');
  },

  bindExport(id, format) {
    const btn = document.getElementById(id);
    if (!btn) return;
    btn.addEventListener('click', () => {
      triggerExport({ format, scope: 'analysis' }).catch(() => {});
    });
  },

  async loadAnalysis() {
    if (this.loading) return;
    this.loading = true;
    this.showLoading();
    this.hideError();
    this.hideContent();

    try {
      const res = await getWorkflowAnalysis(this.thresholds);
      this.report = res.data || null;
      this.render();
    } catch (err) {
      this.showError(err.message || 'Failed to load workflow analysis');
    } finally {
      this.loading = false;
    }
  },

  showLoading() {
    if (this.el.loading) this.el.loading.classList.remove('hidden');
  },

  hideLoading() {
    if (this.el.loading) this.el.loading.classList.add('hidden');
  },

  showError(msg) {
    this.hideLoading();
    this.hideContent();
    if (this.el.error) {
      this.el.error.classList.remove('hidden');
      if (this.el.errorMsg) this.el.errorMsg.textContent = msg;
    }
  },

  hideError() {
    if (this.el.error) this.el.error.classList.add('hidden');
  },

  hideContent() {
    if (this.el.content) this.el.content.classList.add('hidden');
  },

  showContent() {
    if (this.el.content) this.el.content.classList.remove('hidden');
  },

  render() {
    this.hideLoading();
    if (!this.report) {
      this.showError('No analysis data available');
      return;
    }
    if (this.el.calculatedAt) {
      this.el.calculatedAt.textContent = 'Calculated: ' + new Date(this.report.calculated_at).toLocaleString();
    }
    this.renderStateAccumulations();
    this.renderStateDurations();
    this.renderClosurePatterns();
    this.renderInfoRequestPatterns();
    this.renderReviewBacklog();
    this.renderThresholdExceedances();
    this.showContent();
  },

  renderStateAccumulations() {
    if (!this.el.stateAccum) return;
    const items = this.report.state_accumulations || [];
    if (!items.length) {
      this.el.stateAccum.innerHTML = '<p class="empty">No state accumulations observed.</p>';
      return;
    }
    this.el.stateAccum.innerHTML = `
      <table class="data-table" aria-label="State accumulations">
        <thead><tr><th>Workflow</th><th>State</th><th>Cases</th><th>Threshold</th><th>Language</th></tr></thead>
        <tbody>${items.map(i => `<tr><td>${escapeHTML(i.workflow_key)}</td><td>${escapeHTML(i.state_key)}</td><td>${i.case_count}</td><td>${i.threshold}</td><td>${escapeHTML(i.language)}</td></tr>`).join('')}</tbody>
      </table>
    `;
  },

  renderStateDurations() {
    if (!this.el.stateDurations) return;
    const items = this.report.state_duration_anomalies || [];
    if (!items.length) {
      this.el.stateDurations.innerHTML = '<p class="empty">No state duration anomalies observed.</p>';
      return;
    }
    this.el.stateDurations.innerHTML = `
      <table class="data-table" aria-label="State duration anomalies">
        <thead><tr><th>Workflow</th><th>State</th><th>Avg (hrs)</th><th>Median (hrs)</th><th>Max (hrs)</th><th>Language</th></tr></thead>
        <tbody>${items.map(i => `<tr><td>${escapeHTML(i.workflow_key)}</td><td>${escapeHTML(i.state_key)}</td><td>${i.avg_duration_hours != null ? i.avg_duration_hours.toFixed(1) : '—'}</td><td>${i.median_duration_hours != null ? i.median_duration_hours.toFixed(1) : '—'}</td><td>${i.max_duration_hours != null ? i.max_duration_hours.toFixed(1) : '—'}</td><td>${escapeHTML(i.language)}</td></tr>`).join('')}</tbody>
      </table>
    `;
  },

  renderClosurePatterns() {
    if (!this.el.closurePatterns) return;
    const items = this.report.workflow_closure_patterns || [];
    if (!items.length) {
      this.el.closurePatterns.innerHTML = '<p class="empty">No workflow closure patterns observed.</p>';
      return;
    }
    this.el.closurePatterns.innerHTML = `
      <table class="data-table" aria-label="Workflow closure patterns">
        <thead><tr><th>Workflow</th><th>Total</th><th>Closed</th><th>Rejected</th><th>Closure Rate</th><th>Rejection Rate</th><th>Language</th></tr></thead>
        <tbody>${items.map(i => `<tr><td>${escapeHTML(i.workflow_key)}</td><td>${i.total_cases}</td><td>${i.closed_cases}</td><td>${i.rejected_cases}</td><td>${(i.closure_rate * 100).toFixed(1)}%</td><td>${(i.rejection_rate * 100).toFixed(1)}%</td><td>${escapeHTML(i.language)}</td></tr>`).join('')}</tbody>
      </table>
    `;
  },

  renderInfoRequestPatterns() {
    if (!this.el.infoRequestPatterns) return;
    const items = this.report.information_request_patterns || [];
    if (!items.length) {
      this.el.infoRequestPatterns.innerHTML = '<p class="empty">No information request patterns observed.</p>';
      return;
    }
    this.el.infoRequestPatterns.innerHTML = `
      <table class="data-table" aria-label="Information request patterns">
        <thead><tr><th>Workflow</th><th>Cases</th><th>Info Requests</th><th>Frequency/Case</th><th>Threshold</th><th>Language</th></tr></thead>
        <tbody>${items.map(i => `<tr><td>${escapeHTML(i.workflow_key)}</td><td>${i.total_cases}</td><td>${i.info_request_count}</td><td>${i.frequency_per_case.toFixed(2)}</td><td>${i.threshold_per_case.toFixed(1)}</td><td>${escapeHTML(i.language)}</td></tr>`).join('')}</tbody>
      </table>
    `;
  },

  renderReviewBacklog() {
    if (!this.el.reviewBacklog) return;
    const items = this.report.review_backlog_observations || [];
    if (!items.length) {
      this.el.reviewBacklog.innerHTML = '<p class="empty">No review backlog observations.</p>';
      return;
    }
    this.el.reviewBacklog.innerHTML = `
      <table class="data-table" aria-label="Review backlog observations">
        <thead><tr><th>Pending</th><th>Assigned</th><th>In Review</th><th>Avg Wait (hrs)</th><th>Threshold</th><th>Language</th></tr></thead>
        <tbody>${items.map(i => `<tr><td>${i.pending_reviews}</td><td>${i.assigned_reviews}</td><td>${i.in_review_reviews}</td><td>${i.avg_wait_time_hours != null ? i.avg_wait_time_hours.toFixed(1) : '—'}</td><td>${i.threshold}</td><td>${escapeHTML(i.language)}</td></tr>`).join('')}</tbody>
      </table>
    `;
  },

  renderThresholdExceedances() {
    if (!this.el.thresholdExceedances) return;
    const items = this.report.threshold_exceedances || [];
    if (!items.length) {
      this.el.thresholdExceedances.innerHTML = '<p class="empty">No threshold exceedances observed.</p>';
      return;
    }
    this.el.thresholdExceedances.innerHTML = `
      <table class="data-table" aria-label="Threshold exceedances">
        <thead><tr><th>Case</th><th>Workflow</th><th>State</th><th>Threshold</th><th>Actual</th><th>Language</th></tr></thead>
        <tbody>${items.map(i => `<tr><td>${escapeHTML(i.case_number)}</td><td>${escapeHTML(i.workflow_key)}</td><td>${escapeHTML(i.current_state)}</td><td>${escapeHTML(i.threshold_type)}</td><td>${i.actual_value != null ? i.actual_value.toFixed(1) : '—'}</td><td>${escapeHTML(i.language)}</td></tr>`).join('')}</tbody>
      </table>
    `;
  },
};
