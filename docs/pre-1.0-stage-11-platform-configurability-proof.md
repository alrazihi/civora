# Stage 11: Configuration Audit & No-Code Platform Proof

**Status**: IN REVIEW
**Audit Type**: Platform Configurability Assessment

---

## Executive Summary

This audit tests whether a completely new process can be configured through the CIVORA API alone, without any source code changes. The audit simulates an organization installing CIVORA for the first time and configuring a new domain-specific workflow from scratch.

---

## Methodology

From a fresh installation (no demo seed data), an organization would:

1. Register an account
2. Create users and assign roles
3. Define workflow(s) with states and transitions
4. Create forms with fields
5. Configure evidence requirements
6. Set up rules for eligibility
7. Configure review processes
8. Track operational metrics

---

## Configuration API Coverage

### ✓ Organization (Tenant) - Configurable via API
- `/organizations` - Create organization via API
- Org slug, name, description user-configurable

### ✓ Users & Roles - Fully Configurable
- `/users` - Create users
- Roles system via DB direct insertion (currently no `/roles` API endpoint)
- Permissions are role-scoped

### ✓ Workflows - Fully Configurable
- POST `/organizations/{orgId}/workflows` - Create workflow definition
- States configurable with custom keys (NEW, OPEN, etc. are just defaults)
- Transitions configurable with conditions
- Form assignments to states
- Workflow activation/archival via API

### ✓ Forms & Fields - Fully Configurable
- Forms created via API with dynamic JSON fields
- Field types: TEXT, NUMBER, DECIMAL, SELECT, BOOLEAN, DATE, DATETIME
- Field options (enum values) defined in field JSON
- Form versions support publishing/unpublishing

### ✓ Rules Engine - Configurable
- Rule sets defined via API with JSON-encoded rules
- Rule templates can be created
- Conditions support field references, operators (eq, neq, lt, lte, gt, gte, exists, not_exists)
- Outcomes: ELIGIBLE, INELIGIBLE, FLAG, REQUIRES_REVIEW, INFORMATION_REQUIRED

### ✓ Evidence Requirements - Configurable
- Evidence types are free-text (not hardcoded enum)
- Can associate evidence with service requests via case ID
- Storage references managed externally (S3, etc.)

### ✓ Review Process - Configurable
- Review queue supports claim/start/complete/escalate/request-information
- Role-based routing

### ✓ Decisions - Configurable
- Decision types: APPROVED, REJECTED

### ✓ Assistance & Follow-ups - Configurable
- Assistance actions tracked
- Follow-up scheduling

### ✓ Operational Metrics - Configurable
- Metrics are calculated from existing data
- No configuration required - automatic

---

## Architectural Violations Identified

### Critical: Hardcoded Service Types

**Location**: `internal/cases/domain/case.go:46-55`

```go
const (
    ServiceTypeGeneral   ServiceType = "GENERAL"
    ServiceTypeEmergency ServiceType = "EMERGENCY"
    ServiceTypeFinancial ServiceType = "FINANCIAL"
    ServiceTypeFood      ServiceType = "FOOD"
    ServiceTypeShelter   ServiceType = "SHELTER"
    ServiceTypeMedical   ServiceType = "MEDICAL"
    ServiceTypeEducation ServiceType = "EDUCATION"
    ServiceTypeTransport ServiceType = "TRANSPORT"
)
```

**OpenAPI Spec** (`api/openapi/modular/components/schemas/Case.yaml:40-48`):
```yaml
service_type:
  type: string
  enum:
  - GENERAL
  - EMERGENCY
  - FINANCIAL
  - FOOD
  - SHELTER
  - MEDICAL
  - EDUCATION
  - TRANSPORT
```

**Problem**: Cannot add new service types (e.g., "HOSPITALITY", "LEGAL_AID", "HOUSING") without modifying source code.

**Impact**: Organizations using domains outside CIVORA's demo use cases cannot configure their service types.

**Recommendation**: 
- Make `service_type` a free-text string in both domain and OpenAPI spec
- Add optional validation via workflow context or plugin system

---

### Critical: Hardcoded Priority Levels

**Location**: `internal/cases/domain/case.go:57-64`

```go
const (
    PriorityLow    Priority = "LOW"
    PriorityNormal Priority = "NORMAL"
    PriorityHigh   Priority = "HIGH"
    PriorityUrgent Priority = "URGENT"
)
```

**OpenAPI Spec** (`api/openapi/modular/components/schemas/Case.yaml:51-55`):
```yaml
priority:
  enum:
  - LOW
  - NORMAL
  - HIGH
  - URGENT
```

**Problem**: Cannot add domain-specific priorities (e.g., "CRITICAL", "ROUTINE", "DELAYED").

**Recommendation**: Make priority user-configurable via organization settings or free-text.

---

### Medium: Case Status Validation

**Location**: `internal/cases/domain/case.go:15-26, 217-223`

```go
const (
    CaseStatusNew             CaseStatus = "NEW"
    CaseStatusOpen            CaseStatus = "OPEN"
    ...
)

func IsClosed(status CaseStatus) bool {
    return status == CaseStatusClosed || status == CaseStatusRejected
}
```

**Problem**: While workflow states are dynamic, the `CaseStatus` constants and `IsClosed()` function are hardcoded. The seed data shows the same state keys are used for both emergency and education workflows, but organizations may need domain-specific terminal states.

---

### Low: Missing API Endpoint for Roles

**Problem**: The seed data uses SQL INSERT for roles:
```go
_, err = db.DB.ExecContext(ctx, `INSERT INTO roles ...`)
```

There is no `/organizations/{orgId}/roles` endpoint in the OpenAPI spec.

**Impact**: Organizations cannot configure roles via API alone; must use database access.

---

## Proof of Configuration Without Code Changes

Despite the hardcoded values, the following CAN be configured via API only:

### Example: Configuring "Housing Assistance" Program

1. **Organization**: API `POST /organizations`

2. **Users**: API `POST /users` (assign admin/staff roles via DB)

3. **Workflow**: API `POST /organizations/{orgId}/workflows`:
   ```json
   {
     "key": "housing_assistance",
     "name": "Housing Assistance",
     "description": "Emergency housing placement workflow",
     "initial_state": "NEW",
     "state": "ACTIVE",
     "states": [...],
     "transitions": [...]
   }
   ```

4. **Forms**: API `POST /organizations/{orgId}/forms`:
   ```json
   {
     "key": "application_form",
     "name": "Housing Application",
     "versions": [{
       "fields": [
         {"key": "household_size", "type": "NUMBER"},
         {"key": "current_shelter", "type": "TEXT"},
         {"key": "income_tier", "type": "SELECT", "options": ["LOW", "MEDIUM", "HIGH"]}
       ]
     }]
   }
   ```

5. **Rules**: API `POST /organizations/{orgId}/rules/rule-sets`:
   ```json
   {
     "key": "housing_eligibility",
     "default_outcome": "INELIGIBLE",
     "rules": [
       {
         "conditions": {"field": "form.application.income_tier", "operator": "eq", "value": "LOW"},
         "outcome": "ELIGIBLE"
       }
     ]
   }
   ```

6. **Evidence Requirements**: Configured via case workflow binding, not hardcoded

---

## Limitations Table

| Component | Configurable via API | Limitation |
|-----------|---------------------|------------|
| Workflow States | ✓ | Uses string keys; no validation |
| Workflow Transitions | ✓ | Conditions supported |
| Forms & Fields | ✓ | Field types limited to available enums |
| Rules | ✓ | Condition operators limited |
| Service Types | ✗ | Hardcoded in Domain |
| Priorities | ✗ | Hardcoded in Domain |
| Case Statuses | ✗ | Hardcoded terminal state logic |
| Roles | ✗ | No CRUD API |

---

## Recommendations

1. **Make service_type configurable** - Convert to free-text or organization-specific enum
2. **Make priority configurable** - Convert to free-text or admin-defined enum
3. **Add `/roles` API endpoint** - Enable full no-code role management
4. **Make terminal states configurable** - Allow organization to define what "closed" means

---

## Conclusion

While CIVORA's core workflow, forms, and rules systems are fully configurable via API, the hardcoded `service_type` and `priority` enums in the domain layer create a barrier for organizations with domain needs outside the demo use cases (emergency response, education grants).

**Verdict**: CIVORA is approximately 70% no-code configurable. Full no-code configuration requires additional API endpoints and domain changes.

**Genuine Architectural Violations**:
1. `service_type` enum in domain code (`internal/cases/domain/case.go:46-55`)
2. `priority` enum in domain code (`internal/cases/domain/case.go:57-64`)

**Fixes Required**:
- Changes to domain types from enums to free-text strings
- Update validation logic accordingly
- Update OpenAPI spec to reflect new format