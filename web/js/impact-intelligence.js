const ImpactIntelligence = {
  report: null,
  loading: false,
  filters: {
    period: 'daily',
  },

  init() {
    this.cacheElements();
    this.bindControls();
    this.loadReport();
  },

  cacheElements() {
    this.el = {
      loading: document.getElementById('impact-loading'),
      error: document.getElementById('impact-error'),
      errorMsg: document.getElementById('impact-error-msg'),
      retry: document.getElementById('impact-retry'),
      content: document.getElementById('impact-content'),
      calculatedAt: document.getElementById('impact-calculated-at'),
      activity: document.getElementById('impact-activity-metrics'),
      outcome: document.getElementById('impact-outcome-metrics'),
      impact: document.getElementById('impact-impact-metrics'),
      period: document.getElementById('impact-period'),
      apply: document.getElementById('impact-apply'),
      refresh: document.getElementById('impact-refresh'),
    };
  },

  bindControls() {
    if (this.el.apply) {
      this.el.apply.addEventListener('click', () => {
        this.filters.period = this.el.period?.value || 'daily';
        this.loadReport();
      });
    }
    if (this.el.refresh) {
      this.el.refresh.addEventListener('click', () => this.loadReport());
    }
    if (this.el.retry) {
      this.el.retry.addEventListener('click', () => this.loadReport());
    }
    this.bindExport('impact-export-json', 'json');
    this.bindExport('impact-export-csv', 'csv');
    this.bindExport('impact-export-report', 'report');
  },

  bindExport(id, format) {
    const btn = document.getElementById(id);
    if (!btn) return;
    btn.addEventListener('click', () => {
      triggerExport({ format, scope: 'impact', period: this.filters.period }).catch(() => {});
    });
  },

  async loadReport() {
    if (this.loading) return;
    this.loading = true;
    this.showLoading();
    this.hideError();
    this.hideContent();

    try {
      const res = await getImpactReport(this.filters);
      this.report = res.data || null;
      this.render();
    } catch (err) {
      this.showError(err.message || 'Failed to load impact intelligence');
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

  renderMetricCard(metric) {
    const categoryLabel = metric.category.charAt(0).toUpperCase() + metric.category.slice(1);
    const count = metric.category === 'impact' ? (metric.impact_value * 100).toFixed(1) + '%' : (metric.outcome_count || metric.activity_count || 0);
    const sub = metric.category === 'impact' ? `${metric.outcome_count} / ${metric.activity_count}` : metric.description;
    return `<div class="stat-card" role="status" aria-label="${escapeHTML(metric.label)}: ${count}">
      <div class="stat-value">${count}</div>
      <div class="stat-label">${escapeHTML(metric.label)}</div>
      <small class="text-muted">${escapeHTML(sub)}</small>
    </div>`;
  },

  render() {
    this.hideLoading();
    if (!this.report) {
      this.showError('No impact data available');
      return;
    }
    if (this.el.calculatedAt) {
      this.el.calculatedAt.textContent = 'Calculated: ' + new Date(this.report.calculated_at).toLocaleString();
    }

    const metrics = this.report.metrics || [];
    const activity = metrics.filter(m => m.category === 'activity');
    const outcome = metrics.filter(m => m.category === 'outcome');
    const impact = metrics.filter(m => m.category === 'impact');

    if (this.el.activity) {
      this.el.activity.innerHTML = activity.length
        ? '<div class="stats-row">' + activity.map(m => this.renderMetricCard(m)).join('') + '</div>'
        : '<p class="empty">No activity metrics available.</p>';
    }
    if (this.el.outcome) {
      this.el.outcome.innerHTML = outcome.length
        ? '<div class="stats-row">' + outcome.map(m => this.renderMetricCard(m)).join('') + '</div>'
        : '<p class="empty">No outcome metrics available.</p>';
    }
    if (this.el.impact) {
      this.el.impact.innerHTML = impact.length
        ? '<div class="stats-row">' + impact.map(m => this.renderMetricCard(m)).join('') + '</div>'
        : '<p class="empty">No impact metrics available.</p>';
    }

    this.showContent();
  },
};
