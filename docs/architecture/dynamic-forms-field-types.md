# Dynamic Forms — Field Type Vocabulary & Configuration Independence

## Field Type Vocabulary

CIVORA's dynamic form system supports a **fixed set of 13 field types** defined
in the backend domain layer (`internal/forms/domain/form.go`) and rendered by
the frontend runtime (`web/js/form-renderer.js`).

| # | Type | Backend constant | HTML element | Client-side validation |
|---|------|-----------------|--------------|----------------------|
| 1 | Text | `TEXT` | `<input type="text">` | Required check only |
| 2 | Textarea | `TEXTAREA` | `<textarea>` | Required check only |
| 3 | Number | `NUMBER` | `<input type="number">` | Numeric check, optional min/max/step |
| 4 | Decimal | `DECIMAL` | `<input type="number" step="any">` | Numeric check, optional min/max |
| 5 | Date | `DATE` | `<input type="date">` | Required check only |
| 6 | DateTime | `DATETIME` | `<input type="datetime-local">` | Required check only |
| 7 | Boolean | `BOOLEAN` | `<input type="checkbox">` | Required check (must be checked) |
| 8 | Select | `SELECT` | `<select>` | Required check, value in options |
| 9 | MultiSelect | `MULTISELECT` | `<select multiple>` | Required check (at least one) |
| 10 | Radio | `RADIO` | `<input type="radio">[]` | Required check, value in options |
| 11 | Checkbox | `CHECKBOX` | `<input type="checkbox">[]` | Required check (at least one) |
| 12 | Email | `EMAIL` | `<input type="email">` | Email format regex |
| 13 | Phone | `PHONE` | `<input type="tel">` | Phone format regex (7-20 chars, digits/spaces/dashes/parens) |

### Why a fixed vocabulary

The 13 types are intentionally fixed at the platform level. They map directly to
HTML input semantics and cover the universal data-collection needs across all
service domains. Adding a new type requires coordinated changes in three places:

1. **Backend domain** — add constant and register in `validFieldTypes` map
2. **Backend validation** — add server-side validation in
   `internal/cases/application/service.go`
3. **Frontend renderer** — add rendering and client-side validation in
   `web/js/form-renderer.js`

This is a deliberate constraint: field types are platform infrastructure, not
user-configurable. Organizations configure *which fields to use* and *how they
are arranged*, not the underlying input mechanics.

### Field configuration options

Each field in a form definition supports:

- `key` — unique identifier within the form (lowercase, underscores)
- `label` — human-readable display label
- `type` — one of the 13 types above
- `required` — whether the field must be filled
- `placeholder` — hint text for text-like inputs
- `options` — available choices for select/multiselect/radio/checkbox
- `validation` — type-specific rules (min, max, step, pattern, messages)
- `default_value` — pre-filled value
- `display_order` — sort order within the form
- `hidden` — if true, field is not rendered but value is still collected
- `description` — help text shown below the label

---

## Configuration Independence

### Can an organization configure a completely different workflow and forms
### without modifying CIVORA source code?

**Yes.** CIVORA is designed so that all workflow and form configuration is
stored in the database and managed through the API. No source code changes are
needed to:

- Create entirely new workflows with custom states and transitions
- Define forms with any combination of the 13 field types
- Assign forms to specific workflow states (required or optional)
- Set validation rules, field options, and display ordering
- Version forms independently (publish new versions without breaking existing submissions)

### What is configured via the API (no code changes)

| Capability | Mechanism |
|-----------|-----------|
| Workflow definition | `POST /api/v1/workflows` — define states, transitions, terminal states |
| Form definition | `POST /api/v1/forms` — define fields, types, validation, options |
| Form assignment | `POST /api/v1/workflows/{id}/states/{state}/forms` — pin form version to state |
| Form versioning | Publish new versions; old submissions remain tied to their version |
| Field ordering | `display_order` on each field |
| Conditional display | `hidden` flag on fields (future: rules engine) |

### What requires source code changes

| Change | Why |
|--------|-----|
| New field type (e.g., `FILE_UPLOAD`) | Needs backend validation + frontend renderer |
| New workflow trigger mechanism | Core workflow engine behavior |
| Custom business logic on submission | Requires rules engine or integration hooks |
| UI theming/branding | CSS changes in the frontend |

### Frontend domain-agnosticism

The frontend renderer (`web/js/form-renderer.js`) is fully domain-agnostic. It:

- Renders any form definition from the API without hardcoded field assumptions
- Uses generic section titles (configurable via workflow metadata)
- Does not map workflows to specific service types or domains
- Displays service banners generically from the workflow/case metadata
- Handles all error states (400, 401, 403, 404, 409, 429) with user-friendly messages

The frontend contains no service-specific logic — all behavior is driven by the
workflow definition, form definitions, and their assignments retrieved from the API.

---

## Architecture summary

```
Organization Admin
       |
       v
  [API: Create Workflow] --> [Workflow Definition] --> [States + Transitions]
       |
       v
  [API: Create Form] --> [Form Definition] --> [Fields (13 types)]
       |
       v
  [API: Assign Form to State] --> [WorkflowStateFormAssignment]
       |
       v
  [Case Worker uses Frontend]
       |
       v
  [Frontend: fetch workflow forms] --> [Render form] --> [Submit] --> [Backend validates]
```

The separation between platform (field types, workflow engine, form renderer) and
configuration (workflow definitions, form definitions, assignments) is the key
architectural boundary that enables configuration independence.
