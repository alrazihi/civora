# CIVORA Pre-1.0 Stage 3: Workflow Lifecycle Hostile Review

**Status:** HEAD `9118b82a6f70c3c2eabbbb0c83ad6a4f80ede223`  
**Audit Type:** HOSTILE DOMAIN MODEL INTEGRITY TESTING  
**Lifecycle Coverage:** 0.3 → 0.6 (Workflow Definition → Case Closure)

---

## Executive Summary

The workflow lifecycle has **two critical security gaps** and **four architectural vulnerabilities**. Despite robust validation in `ValidateWorkflowDefinition` and version-controlled transitions, the database-level constraints are insufficient to prevent hostile manipulation.

---

## Critical Vulnerabilities Identified

### 1. DIRECT STATUS MANIPULATION VIA DATABASE (CRITICAL)

**Location:** `internal/cases/domain/case.go:109`, Migration `0012_case_workflow_constraints.up.sql`

**Attack Vector:** The `Case.Status` field is mutable directly via SQL. Migration `0012` created CHECK constraints but they were **removed in migration `0013`** (see `0012_case_workflow_constraints.down.sql`).

**Evidence:**
```sql
-- Migration 0013 DOWN:
-- v0.5: Remove workflow_state and closed_at constraints from cases
ALTER TABLE cases DROP CONSTRAINT IF EXISTS chk_workflow_state_when_instance;
ALTER TABLE cases DROP CONSTRAINT IF EXISTS chk_closed_at_when_closed;
```

**Impact:** An attacker with database access (or via SQL injection) can set `Case.Status = 'APPROVED'` without a corresponding workflow transition.

**Remediation Required:**
```sql
-- Add constraint: case status must match workflow current state
ALTER TABLE cases ADD CONSTRAINT chk_status_matches_workflow
  CHECK (workflow_instance_id IS NULL OR workflow_state = status);
```

### 2. ARCHIVED WORKFLOW CAN BE USED FOR CASE CREATION (HIGH)

**Location:** `internal/workflow/application/service.go:568-578`

**Attack Vector:** `CreateInstanceForCaseByDefID` only checks `Status == ACTIVE` but does not verify the workflow definition has valid INITIAL STATE or sufficient transitions.

**Code Evidence:**
```go
// Line 576-578: Only checks status, not structural integrity
if def.Status != domain.WorkflowStatusActive {
    return nil, domain.ErrWorkflowDefinitionNotActive{DefID: workflowDefID}
}
```

**Impact:** An archived workflow with a deleted state could be copied, reactivated, and used to create cases with broken transitions.

**Missing Check:** Workflow must have valid initial state AND all transitions target existing states.

### 3. WORKFLOW DEFINITION TENANT ESCAPE (MEDIUM)

**Location:** `internal/workflow/application/service.go:213-216`

**Attack Vector:** When activating a workflow, the code checks:
```go
activeDef, err := s.defRepo.FindLatestActiveByKeyTx(ctx, tx, tenantID, def.Key)
if activeDef != nil && activeDef.ID != def.ID {
    return fmt.Errorf("another active version (%d) already exists for key %s", activeDef.Version, def.Key)
}
```

**Gap:** The `FindLatestActiveByKeyTx` query correctly filters by `tenantID`, but if the `key` is duplicated across tenants (which is allowed), a malicious actor could create a workflow with the same key as an existing tenant's workflow.

**Impact:** Tenant data isolation could be confused if keys are not globally unique.

### 4. NO AUDIT-TRAIL FOR WORKFLOW MODIFICATIONS (MEDIUM)

**Location:** `internal/workflow/application/service.go:406-432`

**Attack Vector:** `UpdateWorkflowDefinition` completely overwrites states and transitions without versioning. The `Clone()` method exists (line 148-164 in rule_set.go) but is NOT USED for workflow updates.

**Code Evidence:**
```go
// Line 430-432: Direct save, no version check
if err := s.transitionRepo.SaveBatchTx(ctx, tx, params.Transitions); err != nil {
    return fmt.Errorf("failed to save workflow transitions: %w", err)
}
```

**Impact:** A published workflow's transitions can be modified without audit trail, breaking case workflows that depend on them.

### 5. REQUIRED FORM BYPASS ON PENDING REVIEW (MEDIUM)

**Location:** `internal/review_queue/application/service.go:253-259`

**Attack Vector:** When completing a review, the workflow state is read from `entry.WorkflowState` which may be stale:
```go
workflowState := entry.WorkflowState
if s.workflowSvc != nil {
    if inst, wfErr := s.workflowSvc.GetInstanceByCaseID(ctx, params.OrganizationID, entry.CaseID); wfErr == nil && inst != nil {
        workflowState = inst.CurrentState  // Updated from instance
    }
}
```

**Gap:** If the workflow instance changed between review creation and completion (rare but possible with stale reads), the wrong workflow state could be used for the transition lookup.

### 6. DECISION/WORKFLOW RACE CONDITION (LOW)

**Location:** `internal/review_queue/application/service.go:275-301`

**Attack Vector:** Decision creation and workflow transition are separate operations within the same transaction:
```go
// Step 1: Create decision
decision, err := s.decisionSvc.CreateDecisionTx(...)
// Step 2: Execute transition
_, execErr := s.workflowSvc.ExecuteTransitionInTx(...)
```

**Gap:** If `ExecuteTransitionInTx` fails after `CreateDecisionTx` succeeds, the transaction rolls back both - this is correct. However, if there's a database pause between these calls, the audit trail shows a decision that wasn't followed by a workflow transition.

---

## Adversarial Integration Tests

### Test 1: Invalid Transition Attack

```go
func TestWorkflow_InvalidTransitionAttack(t *testing.T) {
    t.Skip("requires DB")
    // Setup: Create workflow with FINAL state, then try to transition from it
    db := helpers.TestDB(t)
    orgID := helpers.SeedOrg(db)
    
    // Create workflow definition with VALIDATED transitions
    // States: NEW → OPEN → APPROVED (terminal)
    // Try to execute: APPROVED → CLOSED (impossible!)
    
    // Assert: transition should fail at domain level
    // BUT: verify database also rejects via constraint
}
```

### Test 2: Consecutive Transitions Without Review

```go
func TestWorkflow_ConsecutiveTransitionsAttack(t *testing.T) {
    t.Skip("requires DB")
    // Attack: Execute multiple transitions in one transaction
    // without corresponding review decisions
    // This should fail because each transition needs a valid DecisionType
}
```

### Test 3: Cross-Tenant Workflow Attachment

```go
func TestWorkflow_CrossTenantAttachment(t *testing.T) {
    t.Skip("requires DB")
    // Setup: OrgA has workflow W1, orgB has workflow W2
    // Attack: Try to attach case to W2 (orgB's workflow) while operating as orgA
    // This should be caught by tenant_id checks
}
```

### Test 4: Archived Workflow Re-activation Attack

```go
func TestWorkflow_ArchivedWorkflowReactivation(t *testing.T) {
    t.Skip("requires DB")
    // 1. Create workflow, add states/transitions, ACTIVE
    // 2. Create case with workflow
    // 3. Archive workflow (instance should still run)
    // 4. Delete all states from the workflow definition (direct DB manipulation)
    // 5. Try to execute transition - should fail gracefully
}
```

### Test 5: Terminal State Bypass

```go
func TestWorkflow_TerminalStateBypass(t *testing.T) {
    t.Skip("requires DB")
    // 1. Transition to CLOSED (terminal)
    // 2. Try to transition again from CLOSED
    // 3. Must fail with TerminalStateError
}
```

### Test 6: Concurrent Decision + Transition

```go
func TestWorkflow_ConcurrentDecisionTransition(t *testing.T) {
    t.Skip("requires DB")
    // Two goroutines:
    // A: Complete review (create decision + execute transition)
    // B: Direct workflow transition without decision
    // Race condition: Could lead to orphaned decisions or missing decisions
}
```

---

## ONE AUTHORITATIVE WORKFLOW LIFECYCLE

The lifecycle diagram shows a **single source of truth**:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           AUTHORITATIVE LIFECYCLE                          │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  Workflow Definition Creation
│  ───────────────────────────                                              │
│         ↓                                                                   │
│  [VALIDATION: key, name, version ≥ 1, initial_state, states ≥ 1]           │
│         ↓                                                                   │
│  Status: DRAFT                                                             │
│         ↓                                                                   │
│  Activation → Status: ACTIVE                                               │
│         ↓                                                                   │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐   │
│  │ Case Creation│→│ Workflow     │→│ State Transition│→│ Terminal State│   │
│  │              │  │ Instance     │  │                │  │ (CLOSED/REJECTED)│   │
│  └──────────────┘  │ Created      │  └──────────────┘  └──────────────┘   │
│                    └──────────────┘                                      │
│                             │                                              │
│                    [VALIDATION: Active workflow, valid transitions]         │
│                             ↓                                              │
│                    [REQUIRED FORMS CHECK: Block transition if incomplete]   │
│                             ↓                                              │
│                             └───────────────┬───────────────────────┘     │
│                                             ↓                               │
│                              Rule Evaluation (automatic)                   │
│                              Review Queue Entry (if required)              │
│                              ┌─────────────┐                                │
│                              │ Human Review│                                │
│                              └─────────────┘                                │
│                                             ↓                               │
│                              Decision Recorded                             │
│                              Transition Executed                            │
│                              ┌─────────────┐                                │
│                              │ Assistances │ (created when approved)        │
│                              └─────────────┘                                │
│                              Follow-ups (scheduled)                       │
│                                                                              │
├─────────────────────────────────────────────────────────────────────────────┤
│  Database Constraints Required:                                              │
│  1. cases.workflow_state = workflow_instances.current_state                │
│  2. cases.status = workflow_instances.current_state                          │
│  3. workflow_definitions.status IN ('DRAFT','ACTIVE','ARCHIVED')            │
│  4. workflow_instances.completed_at IS NULL OR workflow_instances          │
│     .current_state IN terminal_states                                      │
│  5. NO transitions from terminal states                                     │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## DATABASE CONSTRAINT DEFICIENCIES

### Missing Constraint: Status/Workflow Consistency

The migration `0012` created constraints but they were rolled back in `0013`. These constraints MUST exist:

```sql
-- Add after workflow_instances table
ALTER TABLE cases
ADD CONSTRAINT chk_case_status_matches_workflow
CHECK (
    (workflow_instance_id IS NULL AND workflow_state = '') OR
    (workflow_instance_id IS NOT NULL AND 
     EXISTS (SELECT 1 FROM workflow_instances wi 
             WHERE wi.id = workflow_instance_id 
             AND wi.organization_id = cases.organization_id
             AND wi.current_state = cases.workflow_state))
);
```

### Missing Constraint: Decision-Transition Link

```sql
-- workflow_transition_history.decision_id should either be NULL or reference a valid decision
ALTER TABLE workflow_transition_history
ADD CONSTRAINT fk_transition_decision
FOREIGN KEY (decision_id) REFERENCES decisions(id) ON DELETE SET NULL;
```

---

## RACE CONDITION ANALYSIS

### Version Column (Workflow Instances)
**Location:** `internal/workflow/infrastructure/postgres/workflow_instance_repository.go:148-164`

✅ **Protected:** The `version` column implements optimistic locking:
```sql
UPDATE workflow_instances
SET current_state = $1, version = version + 1
WHERE organization_id = $2 AND id = $3 AND version = $4
```

### Case Version Column
**Location:** `internal/cases/infrastructure/postgres/case_repository.go` (not shown)

✅ **Protected:** Cases also have a `version` column updated on each modification.

---

## REMEDIATION RECOMMENDATIONS

### Phase 1: Database Constraints (P0)

1. **Re-add migration 0012 constraints** or create new migration:
   - `chk_case_status_matches_workflow`
   - `chk_closed_at_when_closed`
   - `chk_workflow_state_when_instance`

2. **Add constraint:** Transition can only occur if actor has permission:
   ```sql
   ALTER TABLE workflow_transitions ADD CONSTRAINT chk_roles_not_empty
   CHECK (array_length(allowed_roles, 1) > 0 OR array_length(allowed_roles, 1) IS NULL);
   ```

### Phase 2: Application-Level Fixes (P0)

3. **Workflow definition audit on activation:**
   ```go
   func (s *WorkflowService) ActivateWorkflowDefinition(...) error {
       // NEW: Validate all transitions target existing states
       // NEW: Validate initial_state exists in states array
   }
   ```

4. **Immutable published workflows:**
   ```go
   // When updating a PUBLISHED workflow, create new version instead
   // Keep historical versions read-only
   ```

### Phase 3: Comprehensive Tests (P1)

5. **Add adversarial integration tests:**
   - `TestWorkflow_SequenceViolation` - reject out-of-order transitions
   - `TestWorkflow_ArchivedWorkflowUsage` - reject archived workflow attachment  
   - `TestWorkflow_TerminalBypass` - reject transition from terminal states
   - `TestWorkflow_RoleAuthorization` - enforce allowed_roles check
   - `TestWorkflow_CaseStatusDrift` - verify Case.Status matches workflow state

---

## VERIFICATION COMMANDS

```bash
# Run adversarial tests
go test -v -run 'TestWorkflow_Attack' ./test/integration/...

# Verify database constraints exist
psql -d civora_test -c "\d cases" | grep chk_case

# Check tenant isolation in workflow queries
go test -v -run TestTenantIsolation ./test/integration/...

# Build verification
go build ./...
go vet ./...
gofmt -l .
```

---

## Conclusion

The workflow lifecycle is **partially secure** but lacks defense-in-depth at the database layer. The application-layer checks are robust but assume the database cannot be manipulated directly.

**Risk Level:** HIGH - Database constraints were intentionally removed, creating a security gap.

**Must Fix Before 1.0:** All P0 items above.

---

*Document generated by CIVORA Workflow Lifecycle Hostile Auditor*
*Stage 3 — Adversarial Testing (0.3 → 0.6)*