# CIVORA Pre-1.0 Stage 4: Information Pipeline Hostile Review

**Status:** HEAD `9118b82a6f70c3c2eabbbb0c83ad6a4f80ede223`  
**Pipeline Coverage:** Forms → Submissions → Evidence → Verification → Rules → Evaluations → Decissions → Assistance/Follow-ups

---

## Executive Summary

The information pipeline is **architecturally sound** with strong versioning, immutability guarantees, and deterministic evaluation. However, **three critical gaps** were identified that could allow attackers to cause inconsistent states or denial of service.

---

## 1. Form Submission Pipeline Analysis

### 1.1 Version Pinning - SECURE ✅

**Location:** `internal/cases/application/service.go:878-883`

The `SubmitForm` method correctly:
- Locks form version with `FindByIDForUpdateTx`
- Checks `version.Status == FormVersionStatusPublished`
- Validates `form.Status != FormStatusArchived`

**Code Evidence:**
```go
lockedVersion, err := s.formVersionRepo.FindByIDForUpdateTx(ctx, tx, orgID, formVersionID)
if lockedVersion.Status != formdomain.FormVersionStatusPublished {
    return ErrFormNotPublished
}
```

### 1.2 Immutable Submission History - SECURE ✅

**Location:** `internal/form_submission/domain/submission.go`

Form submissions are append-only and immutable. Once submitted, data cannot be changed.

### 1.3 Unknown Field Detection - SECURE ✅

**Location:** `internal/cases/application/service.go:1316-1343`

The `validateSubmissionData` function rejects unknown fields:
```go
for key := range data {
    if !fieldKeys[key] {
        return fmt.Errorf("%w: unknown field %q", ErrUnknownFields, key)
    }
}
```

### 1.4 Resource Exhaustion - VULNERABLE ⚠️

**Location:** `internal/forms/domain/form.go:119` - Max 100 options per field

**Attack Vector:** A form with `maxFields=100` and each field having `maxOptions=100` could have 10,000 options. Combined with `maxTitleLength=200` and `maxDescriptionLength=2000`, a single form could exceed 200KB just in metadata.

**Missing Protection:** No total form size limit enforced at creation time.

### 1.5 Malformed Metadata - SECURE ✅

**Location:** `internal/evidence/domain/evidence.go:125-142`

Metadata validation uses JSON serialization and enforces 100KB limit.

---

## 2. Evidence Pipeline Analysis

### 2.1 Evidence Verification Split Authority - IDENTIFIED ⚠️

**Location:** `internal/evidence/domain/evidence.go:54-72`

The `Evidence` struct contains both:
- Denormalized fields: `VerificationStatus`, `VerifiedBy`, `VerifiedAt`, `VerificationReason`, `VerificationMethod`
- Separate table: `VerificationRecord` (append-only history)

**Attack Vector:** Direct database manipulation could set `VerificationStatus='VERIFIED'` without creating a `VerificationRecord`.

**Remediation Needed:** Database constraint to ensure verification status matches latest record.

### 2.2 Storage Reference Validation - SECURE ✅

**Location:** `internal/evidence/domain/evidence.go:100-123`

Validates:
- No control characters
- Must be valid URI
- Whitelisted schemes only: `s3`, `gs`, `azureblob`, `civora`
- No credentials/query/fragment in URI

---

## 3. Rules Evaluation Pipeline Analysis

### 3.1 Deterministic Evaluation - SECURE ✅

**Location:** `internal/rules/domain/engine.go:78-96`

Uses `evalContext` with:
- Counter-based trace node IDs (no wall-clock)
- Deterministic sorting of rules by priority
- Immutable input facts snapshot

**Code Evidence:**
```go
type evalContext struct {
    evaluatedAt time.Time
    counter     int64
}

func (ec *evalContext) newTraceNode(nodeType TraceNodeType, description string) TraceNode {
    ec.counter++
    return TraceNode{
        ID:          strconv.FormatInt(ec.counter, 10),
        ...
        Timestamp:   ec.evalatedAt,  // Supplied by caller
    }
}
```

### 3.2 Type Safety - SECURE ✅

**Location:** `internal/rules/domain/engine.go:306-476`

The `compareTyped` function handles:
- Numeric comparisons with `big.Rat` precision
- String operations with regex validation
- Boolean, membership, temporal types
- **Type coercion error** on mismatch (never silently coerce)

### 3.3 Condition Validation - SECURE ✅

**Location:** `internal/rules/domain/rule_set.go:12-66`

Validates:
- Operator whitelisting (18 operators max)
- Priority deduplication
- Condition tree depth limits
- Rules per rule-set limit (MaxRulesPerRuleSet)

### 3.4 Malformed Rule Handling - VULNERABLE ⚠️

**Location:** `internal/rules/domain/engine.go:323-476`

When a condition fails type matching:
```go
case numericOperators[op]:
    return compareNumeric(op, actual, expected)
// ... other operators ...
default:
    return resultError, "unhandled numeric operator"  // Should never happen
```

**Issue:** The `default` case in `compareEquality` (line 471) returns `resultError` but the outer `compareTyped` has no `default`, meaning unknown operators could cause panics in `switch` or return `resultError`.

However, the actual vulnerability is:
- `OpMatches` regex compilation at evaluation time can panic if regex is malformed
- No timeout on regex matching (ReDoS possible)

### 3.5 Facts Snapshot - SECURE ✅

**Location:** `internal/rules/domain/types.go:254`

`FactsSnapshot` is captured and stored with the evaluation, ensuring historical reproducibility.

---

## 4. Decision/Review Pipeline Analysis

### 4.1 Decision Type vs Rule Outcome - NO LINKAGE ⚠️

**Location:** `internal/rules/domain/types.go:21-29`, `internal/decisions/domain/decision.go:13-18`

**Rule Outcomes:** `ELIGIBLE`, `INELIGIBLE`, `REQUIRES_REVIEW`, `INFORMATION_REQUIRED`, `FLAG`, `SCORE`, `ERROR`

**Decision Types:** `APPROVED`, `REJECTED`, `NEEDS_MORE_INFORMATION`, `ESCALATE`

**Gap:** No validation that `DecisionType` aligns with `RuleOutcome`. A rule returning `FLAG` could be interpreted as `APPROVED`.

**Code Evidence:** `ValidateDecisionType` in review_queue does not cross-check with rule outcome.

### 4.2 Missing Information Path - SECURE ✅

**Location:** `internal/decisions/domain/decision.go:90-99`

`ReasonRequired()` correctly requires reason for:
- `REJECTED`
- `ESCALATE`
- `NEEDS_MORE_INFORMATION`
- Not required for `APPROVED`

---

## 5. Cross-Pipeline Attack Vectors

### 5.1 AI Observation → Verified Fact Confusion (IDENTIFIED)

**Location:** `internal/ai/domain/observation.go:60-80`

**Attack Vector:** The `AI Observation` is never automatically promoted to `Verified Fact`. This is by design, but there's no schema constraint to ensure:
- Only reviewers can create `ai_verified_facts`
- AI observations must go through explicit verification

**Status:** Intentional design, but could use guardrails.

### 5.2 Rule Bypass via Direct SQL

**Attack Vector:** Rules require `case_id` (nullable). If `case_id` is NULL:
- Rules can be created without case context
- Evaluations can be created pointing to orphaned rule sets

**Location:** `internal/rules/domain/rule_set.go:138-140`

```go
if caseID != nil && *caseID != uuid.Nil {
    rs.CaseID = caseID
}
```

CaseID is optional. No validation that linked case exists.

---

## 6. Adversarial Integration Tests

### Test 1: Form Submission with Unknown Fields

```go
func TestInformationPipeline_UnknownFieldRejection(t *testing.T) {
    // Submit form with field not defined in form definition
    // Should fail with ErrUnknownFields
}
```

### Test 2: Malformed Rule Causes Evaluation Failure

```go
func TestInformationPipeline_RuleErrorHandling(t *testing.T) {
    // Create rule with malformed regex in pattern
    // Evaluate - should return ERROR status, not panic
}
```

### Test 3: Evidence Status/Record Mismatch

```go
func TestInformationPipeline_EvidenceVerificationIntegrity(t *testing.T) {
    // Create evidence
    // Directly update verification_status via SQL without creating VerificationRecord
    // Verify constraint enforces link
}
```

### Test 4: Old Rule Version Re-evaluation

```go
func TestInformationPipeline_HistoricalEvaluationReproducibility(t *testing.T) {
    // Create rule, make it eligible
    // Evaluate and record evaluation
    // Modify rule to make it ineligible
    // Re-evaluate with same facts - should get different result
    // BUT: Original evaluation should still show ELIGIBLE
}
```

### Test 5: ReDoS on Pattern Comparison

```go
func TestInformationPipeline_ReDoSProtection(t *testing.T) {
    // Create rule with evil regex: ^(a+)+$
    // Match against long string
    // Ensure timeout doesn't allow CPU exhaustion
}
```

---

## 7. Database Constraints Needed

### Missing: Evidence Verification Integrity

```sql
-- Evidence.VerificationStatus must match latest VerificationRecord.Status
-- OR be UNVERIFIED if no record exists

CREATE OR REPLACE FUNCTION check_evidence_verification()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.verification_status != 'UNVERIFIED' THEN
        IF NOT EXISTS (
            SELECT 1 FROM verification_records 
            WHERE evidence_id = NEW.id 
            AND status = NEW.verification_status
        ) THEN
            RAISE EXCEPTION 'verification_status must match verification_records';
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_evidence_verification_check
BEFORE UPDATE ON evidence
FOR EACH ROW
EXECUTE FUNCTION check_evidence_verification();
```

### Missing: Case-Rule Link Validation

```sql
ALTER TABLE rule_sets
ADD CONSTRAINT fk_rule_set_case
FOREIGN KEY (case_id) REFERENCES cases(id) ON DELETE CASCADE;

ALTER TABLE evaluations
ADD CONSTRAINT fk_evaluation_case
FOREIGN KEY (case_id) REFERENCES cases(id) ON DELETE SET NULL;
```

---

## 8. Remediation Priorities

### P0 - Critical (Must fix before 1.0)

1. **Evidence verification integrity constraint** - Prevent mismatched status
2. **Case-rule foreign key constraints** - Ensure referential integrity

### P1 - High (Should fix before 1.0)

3. **Form size limits** - Add total form/metdata byte limits
4. **Regex timeout** - Implement ReDoS protection in `OpMatches`
5. **Decision-outcome alignment validation** - Cross-check between rules and decisions

### P2 - Medium (Nice to have)

6. **Historical evaluation immutability** - Ensure evaluation read-only after creation
7. **Form version compatibility** - Prevent form schema changes invalidating existing submissions

---

## 9. VERIFICATION COMMANDS

```bash
# Run pipeline tests
go test -v -run TestInformationPipeline ./test/integration/...

# Verify form field limits
grep -r "Max" internal/forms/domain/

# Check rule engine type safety
go test -v -run TestRuleEngine ./internal/rules/domain/...

# Build and vet
go build ./...
go vet ./...
gofmt -l .
```

---

## 10. Conclusion

The CIVORA information pipeline is **robustly designed** with:

- ✅ Immutable submissions with version pinning
- ✅ Deterministic, reproducible rule evaluations
- ✅ Type-safe comparisons with no silent coercion
- ✅ Comprehensive field validation

**Weaknesses Identified:**
1. No database constraint for evidence verification integrity
2. Missing foreign keys on case_id in rules/evaluations
3. Potential ReDoS via regex patterns
4. Optional case_id allows orphaned rules

**Risk Level:** MEDIUM - Not critical, but database constraints significantly increase security posture.

---

*Document generated by CIVORA Information Pipeline Hostile Auditor*
*Stage 4 — Forms + Rules + Evidence Pipeline Security Review*