/**
 * CIVORA Dynamic Form Renderer
 *
 * A generic, domain-agnostic runtime form renderer.
 * It consumes a Form Definition object from the API and renders
 * a fully interactive form with data-driven client-side validation.
 *
 * Form Definition shape (consumed, not assumed):
 * {
 *   id, key, name, description, version, status,
 *   fields: [
 *     {
 *       key, label, type, required, description,
 *       placeholder, default_value, options,
 *       validation: { min, max, min_length, max_length, pattern, pattern_message, custom },
 *       display_order, hidden
 *     }
 *   ]
 * }
 *
 * Supported field types:
 *   text, textarea, number, decimal, date, datetime, boolean,
 *   select, multiselect, radio, checkbox, email, phone
 */
(function (global) {
  'use strict';

  const SUPPORTED_FIELD_TYPES = [
    'text', 'textarea', 'number', 'decimal', 'date', 'datetime',
    'boolean', 'select', 'multiselect', 'radio', 'checkbox',
    'email', 'phone'
  ];

  /**
   * Escape user-supplied string content for safe HTML interpolation.
   * Reuses the global escapeHTML if available.
   */
  function esc(str) {
    if (typeof window.escapeHTML === 'function') {
      return window.escapeHTML(str);
    }
    if (str == null) return '';
    return String(str)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#039;');
  }

  /**
   * Validation engine. Reads validation metadata from the field definition
   * and produces a list of error messages.
   * Returns [] when the value is valid.
   */
  function validateFieldValue(field, value) {
    const errors = [];
    const v = field.validation || {};
    const isEmpty = value === null || value === undefined || value === '';

    if ((field.required || v.required) && isEmpty) {
      errors.push(v.required_message || `${field.label || field.key} is required.`);
    }

    if (isEmpty) {
      return errors;
    }

    const fv = String(value);

    switch (field.type) {
      case 'number':
      case 'decimal':
        if (isNaN(Number(fv))) {
          errors.push(v.number_message || `${field.label || field.key} must be a number.`);
        } else {
          const num = Number(fv);
          const minVal = v.minValue !== undefined ? v.minValue : (v.minimum !== undefined ? v.minimum : v.min);
          const maxVal = v.maxValue !== undefined ? v.maxValue : (v.maximum !== undefined ? v.maximum : v.max);
          if (minVal !== undefined && num < minVal) {
            errors.push(v.minimum_message || v.minValue_message || `${field.label || field.key} must be at least ${minVal}.`);
          }
          if (maxVal !== undefined && num > maxVal) {
            errors.push(v.maximum_message || v.maxValue_message || `${field.label || field.key} cannot exceed ${maxVal}.`);
          }
        }
        break;

      case 'email':
        {
          const emailRe = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
          if (!emailRe.test(fv)) {
            errors.push(v.email_message || 'Enter a valid email address.');
          }
        }
        break;

      case 'phone':
        {
          const phoneRe = /^[\d\s\-\+\(\)\.]{7,20}$/;
          if (!phoneRe.test(fv)) {
            errors.push(v.phone_message || 'Enter a valid phone number.');
          }
        }
        break;
    }

    const minLength = v.minLength !== undefined ? v.minLength : v.min_length;
    const maxLength = v.maxLength !== undefined ? v.maxLength : v.max_length;
    if (minLength !== undefined && fv.length < minLength) {
      errors.push(v.minLength_message || v.min_length_message || `${field.label || field.key} must be at least ${minLength} characters.`);
    }
    if (maxLength !== undefined && fv.length > maxLength) {
      errors.push(v.maxLength_message || v.max_length_message || `${field.label || field.key} cannot exceed ${maxLength} characters.`);
    }
    if (v.pattern) {
      try {
        const re = new RegExp(v.pattern);
        if (!re.test(fv)) {
          errors.push(v.pattern_message || `${field.label || field.key} has an invalid format.`);
        }
      } catch (e) {
        // If the pattern is malformed, we skip regex validation
        // rather than crashing the renderer.
      }
    }

    return errors;
  }

  /**
   * Build a single field element from a field definition.
   * Returns an HTMLElement.
   */
  function renderField(field, formState) {
    const fieldDef = field || {};
    const type = String(fieldDef.type || 'text').toLowerCase();
    const key = fieldDef.key || '';
    const label = fieldDef.label || key;
    const required = !!fieldDef.required;
    const placeholder = fieldDef.placeholder || '';
    const description = fieldDef.description || '';
    const options = fieldDef.options || [];
    const v = fieldDef.validation || {};

    const wrapper = document.createElement('div');
    wrapper.className = 'form-field';
    wrapper.setAttribute('data-field-key', key);

    if (fieldDef.display_order !== undefined) {
      wrapper.style.order = String(fieldDef.display_order);
    }

    const labelEl = document.createElement('label');
    labelEl.setAttribute('for', `form-field-${key}`);
    labelEl.innerHTML = esc(label) + (required ? ' <span class="required-marker" aria-hidden="true">*</span>' : '');
    wrapper.appendChild(labelEl);

    if (description) {
      const descEl = document.createElement('p');
      descEl.className = 'field-description';
      descEl.textContent = description;
      wrapper.appendChild(descEl);
    }

    let inputEl = null;

    function createBaseInput(tag, attrs) {
      const el = document.createElement(tag);
      Object.entries(attrs).forEach(([k, val]) => {
        if (val !== undefined && val !== null && val !== false) {
          el.setAttribute(k, val);
        }
      });
      return el;
    }

    switch (type) {
      case 'textarea':
        inputEl = createBaseInput('textarea', {
          id: `form-field-${key}`,
          name: key,
          placeholder: placeholder,
          rows: '3',
          'aria-required': required ? 'true' : 'false',
        });
        break;

      case 'number':
        inputEl = createBaseInput('input', {
          id: `form-field-${key}`,
          type: 'number',
          name: key,
          placeholder: placeholder,
          step: v.step || 'any',
          'aria-required': required ? 'true' : 'false',
        });
        break;

      case 'decimal':
        inputEl = createBaseInput('input', {
          id: `form-field-${key}`,
          type: 'number',
          name: key,
          placeholder: placeholder,
          step: 'any',
          'aria-required': required ? 'true' : 'false',
        });
        break;

      case 'date':
        inputEl = createBaseInput('input', {
          id: `form-field-${key}`,
          type: 'date',
          name: key,
          'aria-required': required ? 'true' : 'false',
        });
        break;

      case 'datetime':
        inputEl = createBaseInput('input', {
          id: `form-field-${key}`,
          type: 'datetime-local',
          name: key,
          'aria-required': required ? 'true' : 'false',
        });
        break;

      case 'email':
        inputEl = createBaseInput('input', {
          id: `form-field-${key}`,
          type: 'email',
          name: key,
          placeholder: placeholder || 'user@example.com',
          'aria-required': required ? 'true' : 'false',
        });
        break;

      case 'phone':
        inputEl = createBaseInput('input', {
          id: `form-field-${key}`,
          type: 'tel',
          name: key,
          placeholder: placeholder || '(555) 123-4567',
          'aria-required': required ? 'true' : 'false',
        });
        break;

      case 'boolean':
        {
          const checkboxWrap = document.createElement('div');
          checkboxWrap.className = 'checkbox-field';
          const cb = createBaseInput('input', {
            id: `form-field-${key}`,
            type: 'checkbox',
            name: key,
            'aria-required': required ? 'true' : 'false',
          });
          const cbLabel = document.createElement('label');
          cbLabel.setAttribute('for', `form-field-${key}`);
          cbLabel.className = 'checkbox-label';
          cbLabel.innerHTML = esc(label);
          checkboxWrap.appendChild(cb);
          checkboxWrap.appendChild(cbLabel);
          wrapper.appendChild(checkboxWrap);
          inputEl = cb;
        }
        break;

      case 'select':
        inputEl = createBaseInput('select', {
          id: `form-field-${key}`,
          name: key,
          'aria-required': required ? 'true' : 'false',
        });
        if (!v.required) {
          const emptyOpt = document.createElement('option');
          emptyOpt.value = '';
          emptyOpt.textContent = placeholder || '— choose —';
          inputEl.appendChild(emptyOpt);
        }
        options.forEach(opt => {
          const o = document.createElement('option');
          const optVal = typeof opt === 'object' && opt !== null ? (opt.value != null ? opt.value : '') : opt;
          const optLabel = typeof opt === 'object' && opt !== null ? (opt.label != null ? opt.label : String(optVal)) : String(opt);
          o.value = String(optVal);
          o.textContent = optLabel;
          inputEl.appendChild(o);
        });
        wrapper.appendChild(inputEl);
        break;

      case 'multiselect':
        inputEl = createBaseInput('select', {
          id: `form-field-${key}`,
          name: key,
          multiple: 'multiple',
          'aria-required': required ? 'true' : 'false',
        });
        options.forEach(opt => {
          const o = document.createElement('option');
          const optVal = typeof opt === 'object' && opt !== null ? (opt.value != null ? opt.value : '') : opt;
          const optLabel = typeof opt === 'object' && opt !== null ? (opt.label != null ? opt.label : String(optVal)) : String(opt);
          o.value = String(optVal);
          o.textContent = optLabel;
          inputEl.appendChild(o);
        });
        wrapper.appendChild(inputEl);
        break;

      case 'radio':
        options.forEach(opt => {
          const optVal = typeof opt === 'object' && opt !== null ? (opt.value != null ? opt.value : '') : opt;
          const optLabel = typeof opt === 'object' && opt !== null ? (opt.label != null ? opt.label : String(optVal)) : String(optVal);
          const radioId = `form-field-${key}-${optVal}`;

          const radioWrap = document.createElement('div');
          radioWrap.className = 'radio-option';

          const rb = createBaseInput('input', {
            id: radioId,
            type: 'radio',
            name: key,
            value: String(optVal),
            'aria-required': required ? 'true' : 'false',
          });
          rb.checked = formState.getValue(key) === String(optVal);

          const rbLabel = document.createElement('label');
          rbLabel.setAttribute('for', radioId);
          rbLabel.className = 'radio-label';
          rbLabel.innerHTML = esc(optLabel);

          radioWrap.appendChild(rb);
          radioWrap.appendChild(rbLabel);
          wrapper.appendChild(radioWrap);

          if (!inputEl) {
            inputEl = rb;
          }
        });
        break;

      case 'checkbox':
        {
          const checkedValues = formState.getValue(key);
          const checkedSet = Array.isArray(checkedValues) ? new Set(checkedValues.map(String)) : new Set();
          const checkboxWrap = document.createElement('div');
          checkboxWrap.className = 'checkbox-group';

          options.forEach((opt, idx) => {
            const optVal = typeof opt === 'object' && opt !== null ? (opt.value != null ? opt.value : '') : opt;
            const optLabel = typeof opt === 'object' && opt !== null ? (opt.label != null ? opt.label : String(optVal)) : String(optVal);
            const cbId = `form-field-${key}-${idx}`;

            const cb = createBaseInput('input', {
              id: cbId,
              type: 'checkbox',
              name: `${key}[]`,
              value: String(optVal),
              'aria-required': required ? 'true' : 'false',
            });
            cb.checked = checkedSet.has(String(optVal));

            const cbLabel = document.createElement('label');
            cbLabel.setAttribute('for', cbId);
            cbLabel.className = 'checkbox-label';
            cbLabel.innerHTML = esc(optLabel);

            checkboxWrap.appendChild(cb);
            checkboxWrap.appendChild(cbLabel);
          });
          wrapper.appendChild(checkboxWrap);
          inputEl = checkboxWrap;
        }
        break;

      case 'text':
      default:
        inputEl = createBaseInput('input', {
          id: `form-field-${key}`,
          type: 'text',
          name: key,
          placeholder: placeholder,
          'aria-required': required ? 'true' : 'false',
        });
        break;
    }

    // Set default value, if defined
    if (inputEl && fieldDef.default_value !== undefined && !formState.hasValue(key)) {
      if (inputEl.type === 'checkbox' || inputEl.type === 'radio') {
        inputEl.checked = !!fieldDef.default_value;
      } else if (inputEl.tagName === 'SELECT' && inputEl.multiple) {
        const defaults = Array.isArray(fieldDef.default_value) ? fieldDef.default_value : [fieldDef.default_value];
        defaults.forEach(dv => {
          const opt = inputEl.querySelector(`option[value="${String(dv).replace(/"/g, '\\"')}"]`);
          if (opt) opt.selected = true;
        });
        formState.setRawValue(key, defaults);
      } else if (inputEl.tagName === 'SELECT') {
        inputEl.value = fieldDef.default_value;
        formState.setRawValue(key, fieldDef.default_value);
      } else if (inputEl.value !== undefined) {
        inputEl.value = fieldDef.default_value;
        formState.setRawValue(key, fieldDef.default_value);
      }
    }

    // Attach live validation listener
    if (inputEl && inputEl.tagName !== 'DIV') {
      inputEl.addEventListener('blur', () => {
        formState.validateField(key);
      });
      inputEl.addEventListener('input', () => {
        formState.clearFieldError(key);
      });
    }

    // If the field is not a checkbox/radio/checkbox-group, append it to wrapper
    if (inputEl && inputEl.tagName !== 'DIV' && type !== 'boolean' && type !== 'checkbox') {
      wrapper.appendChild(inputEl);
    }
    // For boolean, checkbox group is already appended
    // For radio, we appended each option to wrapper, inputEl is the first radio
    // For checkbox-group type, we appended to wrapper, inputEl is the group div

    return wrapper;
  }

  /**
   * FormState manages form values, validation, and error state.
   */
  function createFormState(formDef, onChangeCallback) {
    const values = {};
    const errors = {};
    const fieldMap = {};

    (formDef.fields || []).forEach(f => { fieldMap[f.key] = f; });

    function getValue(key) {
      const el = document.querySelector(`[data-field-key="${key}"]`);
      if (!el) return values[key] !== undefined ? values[key] : (fieldMap[key]?.default_value ?? '');
      const type = fieldMap[key]?.type || 'text';
      const input = el.querySelector('input, select, textarea');
      if (!input) {
        return values[key] !== undefined ? values[key] : '';
      }
      if (type === 'boolean' || (type === 'checkbox' && !Array.isArray(fieldMap[key]?.options))) {
        return input.checked;
      }
      if (type === 'checkbox' && Array.isArray(fieldMap[key]?.options)) {
        const checkedBoxes = el.querySelectorAll('input[type="checkbox"]:checked');
        return Array.from(checkedBoxes).map(cb => cb.value);
      }
      if (type === 'multiselect') {
        return Array.from(input.selectedOptions).map(o => o.value);
      }
      return input.value;
    }

    function collectValues() {
      const result = {};
      Object.keys(fieldMap).forEach(key => {
        const val = getValue(key);
        if (val !== '' && val !== null && val !== undefined && !(Array.isArray(val) && val.length === 0)) {
          result[key] = val;
        }
      });
      Object.keys(values).forEach(key => {
        if (!(key in result) && values[key] !== undefined) {
          result[key] = values[key];
        }
      });
      return result;
    }

    function setRawValue(key, val) {
      values[key] = val;
    }

    function hasValue(key) {
      return values[key] !== undefined;
    }

    function validateField(key) {
      const field = fieldMap[key];
      if (!field) return;
      const value = getValue(key);
      const fieldErrors = validateFieldFn(field, value);
      errors[key] = fieldErrors;
      renderFieldError(key, fieldErrors);
      return fieldErrors.length === 0;
    }

    function clearFieldError(key) {
      errors[key] = [];
      renderFieldError(key, []);
    }

    function validateFieldFn(field, value) {
      return validateFieldValue(field, value);
    }

    function renderFieldError(key, fieldErrors) {
      const el = document.querySelector(`[data-field-key="${key}"]`);
      if (!el) return;

      // Remove existing error elements
      el.classList.remove('has-error');
      const existing = el.querySelector('.field-error, .error-message, .aria-error');
      if (existing) existing.remove();

      if (fieldErrors.length > 0) {
        el.classList.add('has-error');
        const input = el.querySelector('input, select, textarea');
        if (input) {
          input.setAttribute('aria-invalid', 'true');
        }

        const errorEl = document.createElement('p');
        errorEl.className = 'field-error';
        errorEl.setAttribute('role', 'alert');
        errorEl.textContent = fieldErrors.join(' ');
        el.appendChild(errorEl);
      } else {
        const input = el.querySelector('input, select, textarea');
        if (input) {
          input.setAttribute('aria-invalid', 'false');
        }
      }
    }

    function validateAll() {
      let allValid = true;
      Object.keys(fieldMap).forEach(key => {
        const valid = validateField(key);
        if (!valid) allValid = false;
      });
      return allValid;
    }

    function getErrors() {
      return errors;
    }

    function setServerErrors(fieldErrors) {
      Object.keys(fieldErrors).forEach(key => {
        const field = fieldMap[key];
        if (field) {
          const errs = Array.isArray(fieldErrors[key]) ? fieldErrors[key] : [String(fieldErrors[key])];
          errors[key] = errs;
          renderFieldError(key, errs);

          // Also set aria-invalid on the input
          const el = document.querySelector(`[data-field-key="${key}"]`);
          if (el) {
            const input = el.querySelector('input, select, textarea');
            if (input) {
              input.setAttribute('aria-invalid', 'true');
            }
          }
        }
      });
    }

    function clearAllErrors() {
      Object.keys(errors).forEach(key => {
        errors[key] = [];
        renderFieldError(key, []);
      });
    }

    return {
      getValue,
      collectValues,
      setRawValue,
      hasValue,
      validateField,
      clearFieldError,
      validateAll,
      getErrors,
      setServerErrors,
      clearAllErrors,
      getFieldMap: () => fieldMap,
    };
  }

  /**
   * Render a form definition into a container.
   *
   * @param {object} formDef - The form definition from the API.
   * @param {HTMLElement} container - The container element to render into.
   * @param {object} [options] - Optional configuration.
   * @param {function} [options.onSubmit] - Called with (event, values, formState, formDef) on valid submit.
   * @param {function} [options.onCancel] - Called when the cancel button is clicked.
   * @returns {object} - The form state instance for external control.
   */
  function renderForm(formDef, container, options) {
    if (!container) return null;
    if (!formDef || typeof formDef !== 'object') {
      container.innerHTML = '<p class="form-error">No form definition provided.</p>';
      return null;
    }

    const opts = options || {};
    const onSubmit = opts.onSubmit || opts;
    const onCancel = opts.onCancel;
    const isFn = typeof onSubmit === 'function';

    const fields = Array.isArray(formDef.fields) ? formDef.fields : [];
    // Sort by display_order if present
    const sortedFields = [...fields].sort((a, b) => {
      const oa = a.display_order !== undefined ? a.display_order : 0;
      const ob = b.display_order !== undefined ? b.display_order : 0;
      return oa - ob;
    });

    const formState = createFormState(formDef, null);

    const form = document.createElement('form');
    form.className = 'dynamic-form';
    form.setAttribute('data-form-key', formDef.key || '');
    form.setAttribute('data-form-id', formDef.id || '');
    form.noValidate = true;

    // Build header
    const header = document.createElement('div');
    header.className = 'form-header';
    let headerHTML = '';
    if (formDef.name) {
      headerHTML += `<h2>${esc(formDef.name)}</h2>`;
    }
    if (formDef.description) {
      headerHTML += `<p class="form-description">${esc(formDef.description)}</p>`;
    }
    if (formDef.version !== undefined) {
      headerHTML += `<p class="form-version text-muted" style="font-size:0.8rem">Version ${esc(formDef.version)}</p>`;
    }
    header.innerHTML = headerHTML;
    form.appendChild(header);

    // Render fields
    const fieldsContainer = document.createElement('div');
    fieldsContainer.className = 'form-fields';
    sortedFields.forEach(field => {
      if (field.hidden) return;
      fieldsContainer.appendChild(renderField(field, formState));
    });
    form.appendChild(fieldsContainer);

    // Build action bar
    const actionBar = document.createElement('div');
    actionBar.className = 'form-actions';
    actionBar.style.display = 'flex';
    actionBar.style.gap = '8px';
    actionBar.style.justifyContent = 'flex-end';

    if (onCancel && typeof onCancel === 'function') {
      const cancelBtn = document.createElement('button');
      cancelBtn.type = 'button';
      cancelBtn.className = 'btn secondary';
      cancelBtn.textContent = 'Cancel';
      cancelBtn.addEventListener('click', onCancel);
      actionBar.appendChild(cancelBtn);
    }

    const submitBtn = document.createElement('button');
    submitBtn.type = 'submit';
    submitBtn.className = 'btn';
    submitBtn.innerHTML = '<span class="icon">💾</span> Submit';
    actionBar.appendChild(submitBtn);

    form.appendChild(actionBar);

    // Form-level error/success message container
    const messageEl = document.createElement('div');
    messageEl.className = 'form-message hidden';
    messageEl.setAttribute('role', 'alert');
    form.appendChild(messageEl);

    // Submit handler
    form.addEventListener('submit', (e) => {
      e.preventDefault();
      messageEl.classList.add('hidden');
      messageEl.textContent = '';
      messageEl.className = 'form-message hidden';

      if (!formState.validateAll()) {
        messageEl.className = 'form-message error';
        messageEl.classList.remove('hidden');
        messageEl.textContent = 'Please correct the errors above and try again.';
        return;
      }

      // Let the caller handle submission
      if (isFn) {
        onSubmit(e, formState.collectValues(), formState, formDef);
      }
    });

    container.innerHTML = '';
    container.appendChild(form);

    return formState;
  }

  /**
   * Update a form field's value and re-render its error state.
   */
  function updateField(formState, key, value) {
    formState.setRawValue(key, value);
    formState.clearFieldError(key);
  }

  /**
   * Display server-side validation errors.
   * Expected format: { fieldKey: ["error message", ...] } or { "error": { "field_errors": {...} } }
   */
  function displayServerErrors(formState, errorObj) {
    let fieldErrors = {};

    if (errorObj && errorObj.field_errors) {
      fieldErrors = errorObj.field_errors;
    } else if (errorObj && typeof errorObj === 'object' && !Array.isArray(errorObj)) {
      // Could be { "fieldName": ["error1", "error2"] }
      fieldErrors = errorObj;
    }

    if (Object.keys(fieldErrors).length > 0) {
      formState.setServerErrors(fieldErrors);
    }
  }

  /**
   * Show a generic server error message.
   */
  function showFormError(container, message) {
    if (!container) return;
    let msgEl = container.querySelector('.form-message');
    if (!msgEl) {
      msgEl = document.createElement('div');
      msgEl.className = 'form-message error';
      container.appendChild(msgEl);
    }
    msgEl.textContent = message;
    msgEl.classList.remove('hidden');
  }

  /**
   * Show a success message.
   */
  function showFormSuccess(container, message) {
    if (!container) return;
    let msgEl = container.querySelector('.form-message');
    if (!msgEl) {
      msgEl = document.createElement('div');
      msgEl.className = 'form-message success';
      container.appendChild(msgEl);
    }
    msgEl.textContent = message;
    msgEl.className = 'form-message success';
    msgEl.classList.remove('hidden');
  }

  /**
   * Show a loading state.
   */
  function showFormLoading(container, message) {
    if (!container) return;
    container.innerHTML = `<div class="form-loading" style="text-align:center;padding:40px"><div class="loading-spinner" aria-hidden="true"></div> <span>${esc(message || 'Loading form…')}</span></div>`;
  }

  /**
   * Show an empty state.
   */
  function showFormEmpty(container, message) {
    if (!container) return;
    container.innerHTML = `<div class="form-empty" style="text-align:center;padding:40px"><p class="empty">${esc(message || 'No form is available.')}</p></div>`;
  }

  /**
   * Get the submission payload from form state.
   */
  function getSubmissionPayload(formState) {
    return formState.collectValues();
  }

  // Export to global scope
  global.FormRenderer = {
    SUPPORTED_FIELD_TYPES,
    renderForm,
    createFormState,
    validateField: validateFieldValue,
    updateField,
    displayServerErrors,
    showFormError,
    showFormSuccess,
    showFormLoading,
    showFormEmpty,
    getSubmissionPayload,
  };

})(typeof window !== 'undefined' ? window : globalThis);
