const AIObservations = {
  currentEvidenceId: null,

  open(evidenceId) {
    this.currentEvidenceId = evidenceId;
    const modal = document.getElementById('ai-observations-modal');
    if (!modal) return;
    modal.classList.remove('hidden');
    this.render();
  },

  close() {
    const modal = document.getElementById('ai-observations-modal');
    if (modal) modal.classList.add('hidden');
    this.currentEvidenceId = null;
  },

  async render() {
    const container = document.getElementById('ai-observations-body');
    if (!container) return;

    const evidenceId = this.currentEvidenceId;
    if (!evidenceId) return;

    container.innerHTML = '<div class="loading" style="padding:40px;text-align:center"><div class="loading-spinner" style="margin:0 auto 12px" aria-hidden="true"></div> Loading AI observations…</div>';

    try {
      const res = await listAIObservations(evidenceId, { perPage: 50 });
      const observations = (res.data || []).filter(Boolean);
      const total = res.total || observations.length;

      if (!observations.length) {
        container.innerHTML = '<p class="empty" style="padding:20px;text-align:center">No AI observations generated yet. Click "Generate AI Observations" to analyze this evidence.</p>';
        return;
      }

      container.innerHTML = this.renderObservationsList(evidenceId, observations);
    } catch (err) {
      if (err.status === 503) {
        container.innerHTML = '<p class="empty" style="padding:20px;text-align:center;color:var(--danger)">AI provider is not configured. Contact your administrator to enable AI features.</p>';
      } else {
        container.innerHTML = `<p class="empty" style="padding:20px;text-align:center;color:var(--danger)">Failed to load observations: ${err.message}</p>`;
      }
    }
  },

  renderObservationsList(evidenceId, observations) {
    return `
      <div class="ai-observations-list">
        ${observations.map(obs => this.renderObservationCard(evidenceId, obs)).join('')}
      </div>
    `;
  },

  renderObservationCard(evidenceId, obs) {
    const type = obs.type || 'UNKNOWN';
    const status = (obs.status || 'PENDING_REVIEW').toLowerCase();
    const confidence = obs.confidence ? Math.round(obs.confidence * 100) + '%' : null;
    const model = obs.model ? `${obs.model.name} (${obs.model.provider})` : null;
    const typeLabel = type.replace(/_/g, ' ');

    let statusBadge = '';
    if (status === 'accepted') {
      statusBadge = `<span class="badge completed">${icon('check')} Accepted</span>`;
    } else if (status === 'rejected') {
      statusBadge = `<span class="badge rejected">${icon('x')} Rejected</span>`;
    } else {
      statusBadge = `<span class="badge open">${icon('eye')} Pending Review</span>`;
    }

    const typeBadge = `<span class="badge secondary ai-type-badge">${escapeHTML(typeLabel)}</span>`;

    let actionsHtml = '';
    if (status === 'pending_review' || status === 'pending' || !obs.status) {
      actionsHtml = `
        <div class="ai-observation-actions">
          <button class="btn tiny btn-success" onclick="AIObservations.reviewObservation('${evidenceId}', '${obs.id}', 'accept', event)">${icon('check')} Accept</button>
          <button class="btn tiny btn-danger" onclick="AIObservations.reviewObservation('${evidenceId}', '${obs.id}', 'reject', event)">${icon('x')} Reject</button>
        </div>
      `;
    }

    const contentHtml = this.renderObservationContent(obs.content, type);
    const confidenceHtml = confidence ? `<span class="ai-meta-item"><strong>Confidence:</strong> ${confidence}</span>` : '';
    const modelHtml = model ? `<span class="ai-meta-item"><strong>Model:</strong> ${escapeHTML(model)}</span>` : '';
    const notesHtml = obs.review_notes ? `<div class="ai-review-notes"><strong>Review Notes:</strong> ${escapeHTML(obs.review_notes)}</div>` : '';

    return `
      <div class="ai-observation-card" data-observation-id="${obs.id}">
        <div class="ai-observation-header">
          <div class="ai-observation-title">
            ${typeBadge}
            ${statusBadge}
          </div>
          ${actionsHtml}
        </div>
        <div class="ai-observation-body">
          ${contentHtml}
        </div>
        <div class="ai-observation-meta">
          ${confidenceHtml}
          ${modelHtml}
          <span class="ai-meta-item"><strong>Created:</strong> ${formatDate(obs.created_at)}</span>
          ${notesHtml}
        </div>
      </div>
    `;
  },

  renderObservationContent(content, type) {
    if (!content || typeof content !== 'object') {
      return '<span class="empty">No content</span>';
    }

    const keys = Object.keys(content);
    if (!keys.length) {
      return '<span class="empty">No content</span>';
    }

    switch (type) {
      case 'SUMMARY':
      case 'Classification':
        return `
          <div class="ai-content">
            ${content.text ? `<p class="ai-content-text">${escapeHTML(content.text)}</p>` : ''}
            ${content.summary ? `<p class="ai-content-text">${escapeHTML(content.summary)}</p>` : ''}
            ${content.classification ? `<p>${escapeHTML(content.classification)}</p>` : ''}
            ${content.label ? `<p><strong>Label:</strong> ${escapeHTML(content.label)}</p>` : ''}
          </div>
        `;
      case 'ENTITY_EXTRACTION':
        if (content.entities && Array.isArray(content.entities)) {
          return `
            <div class="ai-content">
              <ul class="ai-entity-list">
                ${content.entities.map(e => `<li><span class="ai-entity-type">${escapeHTML(e.type || e.label || 'Entity')}</span>: ${escapeHTML(e.value || e.text || e)}</span></li>`).join('')}
              </ul>
            </div>
          `;
        }
        return `<pre class="ai-content-pre">${escapeHTML(JSON.stringify(content, null, 2))}</pre>`;
      case 'INCONSISTENCY':
        return `
          <div class="ai-content">
            ${content.description ? `<p class="ai-content-text">${escapeHTML(content.description)}</p>` : ''}
            ${content.field ? `<p><strong>Field:</strong> ${escapeHTML(content.field)}</p>` : ''}
            ${content.expected ? `<p><strong>Expected:</strong> ${escapeHTML(content.expected)}</p>` : ''}
            ${content.actual ? `<p><strong>Actual:</strong> ${escapeHTML(content.actual)}</p>` : ''}
            ${content.details ? `<p><strong>Details:</strong> ${escapeHTML(content.details)}</p>` : ''}
          </div>
        `;
      default:
        return `<pre class="ai-content-pre">${escapeHTML(JSON.stringify(content, null, 2))}</pre>`;
    }
  },

  async generateObservations() {
    const evidenceId = this.currentEvidenceId;
    if (!evidenceId) return;

    const btn = document.getElementById('ai-generate-btn');
    if (btn) {
      btn.disabled = true;
      btn.innerHTML = `${icon('loading')} Generating…`;
    }

    const container = document.getElementById('ai-observations-body');
    if (container) {
      container.innerHTML = '<div class="loading" style="padding:40px;text-align:center"><div class="loading-spinner" style="margin:0 auto 12px" aria-hidden="true"></div> Generating AI observations…</div>';
    }

    try {
      const result = await generateAIObservations(evidenceId, null, 4096);
      const newObs = result && result.observations ? result.observations : [];

      if (!newObs.length) {
        container.innerHTML = '<p class="empty" style="padding:20px;text-align:center">AI returned no observations for this evidence.</p>';
      } else {
        const obsArray = newObs.map(o => this.resultToObservation(o));
        container.innerHTML = this.renderObservationsList(evidenceId, obsArray);
        showToast(`Generated ${newObs.length} AI observation(s)`, 'success');
      }
    } catch (err) {
      if (err.status === 503) {
        container.innerHTML = '<p class="empty" style="padding:20px;text-align:center;color:var(--danger)">AI provider is not configured. Contact your administrator to enable AI features.</p>';
      } else {
        container.innerHTML = `<p class="empty" style="padding:20px;text-align:center;color:var(--danger)">Failed to generate observations: ${err.message}</p>`;
      }
      showToast(`Failed to generate observations: ${err.message}`, 'error');
    } finally {
      if (btn) {
        btn.disabled = false;
        btn.innerHTML = `${icon('ai')} Generate AI Observations`;
      }
    }
  },

  resultToObservation(result) {
    return {
      id: result.id || '',
      type: result.type || 'SUMMARY',
      source: result.source || 'AI_MODEL',
      status: result.status || 'PENDING_REVIEW',
      content: result.content || {},
      confidence: result.confidence || null,
      model: result.model || null,
      input_hash: result.input_hash || '',
      output_hash: result.output_hash || '',
      created_at: result.created_at || new Date().toISOString(),
      reviewed_at: result.reviewed_at || null,
      reviewed_by: result.reviewed_by || null,
      review_notes: result.review_notes || '',
    };
  },

  async reviewObservation(evidenceId, observationId, action, ev) {
    ev.preventDefault();

    const notes = prompt(action === 'accept' ? 'Enter acceptance notes (optional):' : 'Enter rejection reason (optional):', '');
    if (notes === null) return;

    const button = ev.currentTarget;
    button.disabled = true;
    button.innerHTML = `${icon('loading')} Processing…`;

    try {
      if (action === 'accept') {
        await acceptAIObservation(evidenceId, observationId, notes.trim());
      } else {
        await rejectAIObservation(evidenceId, observationId, notes.trim());
      }
      showToast(`Observation ${action}ed`, 'success');
      await this.render();
    } catch (err) {
      showToast(`Failed to ${action} observation: ${err.message}`, 'error');
      button.disabled = false;
      button.innerHTML = action === 'accept'
        ? `${icon('check')} Accept`
        : `${icon('x')} Reject`;
    }
  },
};
