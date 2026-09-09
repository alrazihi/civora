# CIVORA

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![CI](https://img.shields.io/github/actions/workflow/status/alrazihi/civora/ci.yml?branch=main&label=CI)](https://github.com/alrazihi/civora/actions)
[![Code of Conduct](https://img.shields.io/badge/Contributor%20Covenant-2.1-4baaaa)](./CODE_OF_CONDUCT.md)

CIVORA is an open-source platform for building, operating, and auditing
service-delivery processes in the public interest.

## Mission

Governments, NGOs, humanitarian organizations, social organizations, and
other public-interest institutions should be able to create, operate, audit,
and improve complex service-delivery processes without proprietary lock-in.

CIVORA provides the open digital infrastructure to make that possible.

## Status

CIVORA is in **early development** (Milestones 0.x). The project is not yet
production-ready. See the [roadmap](./ROADMAP.md) and
[governance](./GOVERNANCE.md) for details on how to follow along or
contribute.

Current focus: **Milestone 0.2** — Service delivery lifecycle (eligibility,
assistance, follow-up modules).

## Quick Start

```bash
# Start PostgreSQL
docker compose up -d db

# Run the server
go run ./cmd/civora

# Seed demo data (in another terminal)
go run ./cmd/seed
```

Open http://localhost:8080 and sign in with the demo credentials printed by the seed command.

## Demo Scenario: Emergency Food and Shelter Assistance

The following scenario demonstrates the complete CIVORA workflow from request through closure.

### Organization Setup

An NGO called **"Emergency Response Org"** registers on CIVORA with slug `emergency-demo`. The first registered user becomes the organization administrator. Additional staff members are invited and assigned the `staff` role.

### Step 1: Create a Person

A field worker registers **Amina Hassan**, a 34-year-old mother of four who was displaced by flooding. Her preferred language is set to English, and her contact details are recorded.

### Step 2: Create an Assistance Request

The field worker creates a new **Emergency Food and Shelter Assistance** case:

- **Service type:** Emergency
- **Priority:** Urgent
- **Linked person:** Amina Hassan
- **Description:** "Family of 4 displaced by flooding, needs immediate food and shelter support"

The case is assigned case number `CAS-20260909-DEMO001` and enters status `NEW`.

### Step 3: Open and Review

The case worker opens the case (`NEW → OPEN`) and moves it into review (`OPEN → IN_REVIEW`). During review, the worker:

- **Adds eligibility assessment:** records criteria (displacement verified, income below threshold) and explanation.
- **Attaches evidence:** uploads identity document and proof-of-residence with storage references.

### Step 4: Assessment

The case moves to `ASSESSMENT`. A case manager records a needs assessment:

- **Findings:** Household of 4 displaced by flood. Verified identity and residence documents. Income below threshold.
- **Needs identified:** Emergency food, temporary shelter, clothing
- **Recommendation:** Approve emergency shelter placement and food package

### Step 5: Human Decision

The case enters `DECISION_PENDING`. A human reviewer (not an automated system) records the final decision:

- **Decision:** APPROVED
- **Reason:** "Meets all eligibility criteria. Assessment supports immediate shelter and food assistance."

**CIVORA enforces that AI cannot automatically approve, reject, or distribute assistance.** Every decision must be explicitly recorded by an authorized human actor and is immutable once finalized.

### Step 6: Assistance

After approval, the case moves to `IN_PROGRESS`. The organization creates an assistance record:

- **Type:** Shelter
- **Description:** Emergency shelter placement at City Shelter Center for 30 days
- **Responsible staff:** Assigned case manager
- **Status:** Planned → In Progress → Completed

Assistance records what was provided, who was responsible, and when it was delivered.

### Step 7: Follow-up

A follow-up is scheduled for 2026-10-15. After the visit, the case worker records the outcome:

- **Outcome:** Family stably housed and receiving ongoing support
- **Notes:** Weekly check-ins scheduled with case manager

### Step 8: Closure

Once the follow-up is complete and all obligations are satisfied, the case is moved to `FOLLOW_UP` and then closed (`CLOSED`). Closed cases cannot be reopened or transitioned to any other status.

### Audit Trail

Every state transition, evidence addition, assessment, decision, assistance action, and follow-up is recorded in an immutable, hash-chained audit log. The audit trail can be reviewed at any time to verify who did what, when, and why.

## Documentation

| Topic | Location |
|-------|----------|
| Vision and mission | [README](./), [docs/vision.md](./docs/vision.md) |
| Project principles | [docs/principles.md](./docs/principles.md) |
| Architecture overview | [ARCHITECTURE.md](./ARCHITECTURE.md) |
| Architecture decision records | [docs/decisions](./docs/decisions) |
| Threat model | [docs/threat-model.md](./docs/threat-model.md) |
| Hostile review | [docs/hostile-review.md](./docs/hostile-review.md) |
| Development roadmap | [ROADMAP.md](./ROADMAP.md) |
| Changelog | [CHANGELOG.md](./CHANGELOG.md) |
| Contribution guide | [CONTRIBUTING.md](./CONTRIBUTING.md) |
| Governance model | [GOVERNANCE.md](./GOVERNANCE.md) |
| Security policy | [SECURITY.md](./SECURITY.md) |
| Code of conduct | [CODE_OF_CONDUCT.md](./CODE_OF_CONDUCT.md) |

## License

[Apache License 2.0](./LICENSE)

Copyright 2026 CIVORA contributors.
