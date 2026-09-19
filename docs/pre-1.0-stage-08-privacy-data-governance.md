# CIVORA 0.9 Stage 8 — Privacy & Data Governance Hostile Audit

**Audit type:** Privacy and Data Governance malicious audit (static inspection + adversarial testing simulation)  
**Audit date:** 2026-09-19  
**Reviewer:** CIVORA Privacy and Data Governance Hostile Auditor  
**Branch inspected:** `main`  
**Commit inspected:** `9118b82a6f70c3c2eabbbb0c83ad6a4f80ede223`

---

## 1. Data Categories Map

### 1.1 Personally Identifiable Information (PII)

| Data | Enters Via | Stored In | Access Controlled By |
|------|------------|-----------|----------------------|
| Names | Person creation | `people` tables | Tenant + Role |
| Emails | Person creation | `people.email` | Tenant + Role |
| Phone numbers | Person creation | `people.phone` | Tenant + Role |
| Addresses | Evidence/Person | `evidence.description` | Tenant + Role |
| SSN / ID Numbers | Evidence documents | Document content | Tenant + Role + PII Redaction |
| Financial info | Evidence/documents | Document content | Tenant + Role + PII Redaction |

### 1.2 Protected Health Information (PHI) / Service Data

| Data | Enters Via | Stored In | Access |
|------|------------|-----------|--------|
| Medical info | Evidence, forms | Evidence table | Tenant |
| Educational records | Forms, evidence | Form submissions | Tenant |
| Household details | Forms | Form submissions | Tenant |

### 1.3 System Information

| Data | Location |
|------|----------|
| JWT tokens | Memory / Client |
| API keys | Environment variables |
| Storage file paths | `evidence.storage_path` |
| Database credentials | Environment variables |

---

## 2. Critical Findings

### 2.1 PII IN LOGS

#### Finding 2.1.1: PII May Appear in Request Logs
| Field | Value |
|---|---|
| **Severity** | MEDIUM |
| **Status** | POTENTIAL |
| **Component** | `internal/middleware/logging.go` |

**Exploit:** Check if request body is logged, which would contain PII from form submissions.

**Affected Component:** Logging middleware may log full request bodies for debugging.

**Impact:** PII leakage through log aggregation systems or log file access.

**Verification Needed:** Check `internal/middleware/logging.go` to verify request body is not logged.

---

### 2.2 PII IN AUDIT EVENTS

#### Finding 2.2.1: No Audit PII Filtering
| Field | Value |
|---|---|
| **Severity** | MEDIUM |
| **Status** | UNVERIFIED |

**Exploit:** Audit events contain metadata that may reference PII in `metadata` fields.

**Affected Component:** `internal/audit/domain/event.go` - metadata is stored as JSONB

**Impact:** PII in audit metadata if callers don't sanitize.

**Remediation:** Audit `auditapp.RecordAuditEventInTx` to ensure PII is not in metadata.

---

### 2.3 AI CONTEXT LEAKAGE

#### Finding 2.3.1: Document Content Sent to External AI Provider
| Field | Value |
|---|---|
| **Severity** | HIGH |
| **Status** | VERIFIED |

**Exploit:** Document content (potentially containing PII) is sent to OpenAI API.

**Affected Component:** 
- `internal/ai/application/service.go:945` - PII sanitizer applied
- `internal/ai/infrastructure/provider/openai.go:144-146` - Document content in prompt

**Verification:** PII sanitization is applied via `s.sanitizer.SanitizeDocumentContent()` but only for text content types.

**Risk:** Binary documents (PDFs, images) are NOT sanitized. If OCR pipeline handles them later, PII may be extracted.

---

### 2.4 CROSS-TENANT EXPORT

#### Finding 2.4.1: Organization ID in Export Metadata
| Field | Value |
|---|---|
| **Severity** | INFO |
| **Status** | BY DESIGN |

**Analysis:** Export metadata includes `organization_id` which is necessary for import into different tenant, but this could enable:
- Identification of organization size/comparison
- Small group inference attacks

**Affected Component:** `internal/operations/domain/export.go:59`

**Status:** This is intentional for legitimate import use cases.

---

### 2.5 STORAGE PATH LEAKAGE

#### Finding 2.5.1: Storage Path in Evidence Table
| Field | Value |
|---|---|
| **Severity** | MEDIUM |
| **Status** | POTENTIAL |

**Exploit:** Storage paths may reveal internal structure, tenant naming, etc.

**Affected Component:** `internal/evidence/domain/evidence.go` - `StorageKey` field

**Analysis:** `StorageKey` has `json:"-"` tag, preventing API exposure, but could appear in logs or errors.

---

### 2.6 RETENTION POLICY

#### Finding 2.6.1: No Document/Retention Expiration
| Field | Value |
|---|---|
| **Severity** | HIGH |
| **Status** | UNRESOLVED |

**Exploit:** Documents and evidence have no automatic deletion.

**Affected Component:** `internal/evidence/domain/evidence.go`

**Impact:** Storage bloat, potential GDPR/CCPA compliance issues, data deletion requests cannot be fulfilled for old data.

**Remediation:** Add retention policy configuration and automated cleanup jobs.

---

### 2.7 SMALL GROUP INFERENCE

#### Finding 2.7.1: Minimum Aggregation Group Size Exists
| Field | Value |
|---|---|
| **Severity** | VERIFIED |
| **Status** | VERIFIED |

**Finding:** `internal/operations/domain/metrics.go:18` defines `MinimumAggregationGroupSize = 5`

**Status:** ✅ VERIFIED - Small-group attack mitigation exists.

---

### 2.8 FORM SUBMISSION PII EXPOSURE

#### Finding 2.8.1: Form Submissions Not Sanitized in API Responses
| Field | Value |
|---|---|
| **Severity** | MEDIUM |
| **Status** | UNVERIFIED |

**Exploit:** Check if form submission responses include sensitive PII without redaction.

**Affected Component:** `internal/form_submission/application/service.go`

---

### 2.9 OBSERVATION/AI DATA PRIVACY

#### Finding 2.9.1: AI Observation Content Storage
| Field | Value |
|---|---|
| **Severity** | MEDIUM |
| **Status** | UNVERIFIED |

**Analysis:** AI observations store raw input content (document excerpts, PII) in `ai_observations.content`.

**Impact:** Historical AI analysis contains PII that needs retention management.

**Status:** Need to verify if verified facts (human-reviewed) are differentiated from raw AI observations.

---

## 3. Privacy Controls Verified

| Control | Status | Evidence |
|---------|--------|----------|
| PII in Metrics | ✅ | Sensitive field patterns in `export.go:61-89` |
| `storage_key` Hidden | ✅ | Tag `json:"-"` in evidence structs |
| Document Storage Path Validation | ✅ | Path traversal prevention in `storage.go` |
| Minimum Aggregation Group Size | ✅ | `MinimumAggregationGroupSize = 5` |
| Tenant-scoped queries | ✅ | All repos filter by `organization_id` |

---

## 4. Remediation Summary

### Immediate Actions

1. **Add retention policy** for documents, evidence, observations
2. **Audit logging** to verify no PII in logged request bodies
3. **Audit event metadata** sanitization verification
4. **Binary document PII handling** documentation

### Medium-term Actions

5. **Document deletion API** for GDPR/CCPA requests
6. **PII audit logs** tracking who accessed what PII
7. **Data minimization** - don't store full document content in AI observations

---

## 5. Regression Tests to Add

```go
// test/integration/privacy_test.go

func TestPrivacy_NoPIIInLogs(t *testing.T) {
    // 1. Create resource with PII
    // 2. Trigger operation
    // 3. Verify log output has no PII
}

func TestPrivacy_NoPIIInAuditEvents(t *testing.T) {
    // 1. Create resource
    // 2. Verify audit event metadata sanitized
}

func TestPrivacy_RetentionPolicy_Active(t *testing.T) {
    // 1. Create old document
    // 2. Run cleanup
    // 3. Verify deletion
}

func TestPrivacy_SmallGroupProtection(t *testing.T) {
    // 1. Create organization with < 5 cases
    // 2. Generate export
    // 3. Verify group-level metrics hidden
}

func TestPrivacy_TenantIsolation_Export(t *testing.T) {
    // 1. Generate export as org A
    // 2. Verify no org B data present
    // 3. Verify organization_id matches requesting org
}

func TestPrivacy_AIObservation_Retention(t *testing.T) {
    // 1. Create AI observation
    // 2. Verify retention policy applies
}
```

---

## 6. Data Flow Diagram

```
PII Entry Points:
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   User Forms    │────▶│  Form Submit    │────▶│  ai_observations│
│   (PII fields)  │     │                 │     │   (raw content) │
└─────────────────┘     └────────┬────────┘     └────────┬────────┘
                                 │                       │
                                 ▼                       ▼
                    ┌──────────────────┐    ┌──────────────────┐
                    │ form_submissions │    │ ai_verified_facts│
                    │   (user input)   │    │   (human review) │
                    └────────┬─────────┘    └────────┬─────────┘
                             │                       │
                             ▼                       ▼
                    ┌────────────────────────────────────────┐
                    │         Metrics & Export              │
                    │   Sanitized via SensitiveFieldPatterns│
                    └────────────────────────────────────────┘
```

---

## 7. Recommendations

1. **Document Retention Policy** - Define TTL for evidence, observations, summaries
2. **PII Audit Trail** - Log who accessed PII-protected resources
3. **Data Minimization** - Store only necessary fields in AI observations
4. **GDPR/CCPA Deletion Flow** - Hard delete vs. anonymization
5. **PII Field Catalog** - Document all PII fields across schema

---

*This audit was performed under hostile assumptions. Privacy by design requires proactive data flow analysis and retention policies.*