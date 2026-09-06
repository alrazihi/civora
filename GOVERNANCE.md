# CIVORA Governance

> **Status**: Draft — initial governance model for the foundation phase.

---

## Overview

CIVORA is a community-driven, independent, public-interest open-source
project. It is governed by its community of contributors, with
leadership that is earned through sustained contribution rather than
appointed from outside.

This document describes the governance model: how decisions are made,
how leadership is established, and how the project evolves.

---

## Roles

### Contributors

Anyone who contributes to the project in any way is a **contributor**:
code, documentation, tests, issue reports, design feedback, bug reports,
answering questions, organizing events, etc.

All contributors are covered by the [Code of Conduct](CODE_OF_CONDUCT.md).

### Committers

**Committers** are contributors who have demonstrated sustained,
high-quality contributions and who have been granted write access to the
repository. They can:

- Review and approve pull requests.
- Merge changes into the main branch.
- Participate in architectural decisions.

Committers are expected to act in the best interest of the project and
its community, following the principles in [docs/principles.md](docs/principles.md)
and [docs/vision.md](docs/vision.md).

### Stewards

**Stewards** are the project's core leadership. They are selected from
among committers and have additional responsibilities:

- Setting technical direction and roadmap priorities.
- Approving or requesting changes to Architecture Decision Records.
- Managing releases.
- Resolving conflicts that cannot be resolved by consensus.
- Maintaining the threat model and security posture.
- Onboarding new committers.

The initial stewards are the project's founding engineers. As the
project grows, additional stewards are chosen from the committer pool
by consensus among existing stewards.

### Benevolent Dictator (interim)

For the foundation phase, the project operates under a **Benevolent
Dictator For Life (BDFL)** model with the founding engineering agent as
the BDFL. This provides clear decision-making in the early stages.

The BDFL role will be **replaced** by a voting steward model once the
project has at least 3 stewards and 10 active committers, as defined
below.

---

## Decision-making process

### Small decisions

Small, non-controversial decisions (bug fixes, small features,
documentation improvements) are made by individual committers through
pull requests, following standard code review.

### Significant decisions

Significant decisions — anything that affects architecture, API
surface, security posture, governance, or long-term direction — follow
this process:

1. **Proposal**: An Architecture Decision Record (ADR) or a GitHub
   issue is created describing the proposal.
2. **Discussion**: At least 7 days for community comment and
   discussion.
3. **Consensus**: Stewards seek consensus among active contributors.
4. **Escalation**: If consensus cannot be reached, stewards vote. A
   simple majority of stewards decides. The rationale is recorded.

### Voting

When consensus is not possible:

- Only **stewards** vote.
- A simple majority wins.
- Votes are recorded publicly in the relevant GitHub issue or PR.
- The decision and its rationale are documented.

### Reversibility

All decisions are reversible. If evidence emerges that a decision was
wrong, a new proposal can be made to reverse it, following the same
process.

---

## Adding committers

A contributor becomes a committer by:

1. Being nominated by an existing steward or committer.
2. Demonstrating sustained, high-quality contributions (at least 5
   merged contributions).
3. Passing a vote of the stewards (simple majority).
4. Being invited to join and accepting.

---

## Adding stewards

A committer becomes a steward by:

1. Being nominated by an existing steward or committer.
2. Demonstrating leadership in the project (architectural decisions,
   mentorship, conflict resolution).
3. Passing a vote of the existing stewards (supermajority: 2/3 if 3+
   stewards).
4. Being invited to join and accepting.

---

## Conflict resolution

1. Try to resolve conflicts through discussion and consensus.
2. If unresolved, a steward mediates.
3. If still unresolved, the stewards vote.
4. In rare cases where the stewards are themselves the conflict, the
   issue is escalated to the broader committer group for a vote.

---

## Roadmap and milestones

The roadmap is maintained in [ROADMAP.md](ROADMAP.md). Milestones are
set collaboratively by the stewards and community. Dates are not
promised; priorities and scope are committed to.

---

## Budget and resources

CIVORA operates as a volunteer community project. The project does not
have a formal budget. Any project funds (e.g., from sponsorships or
grants) are managed by the stewards in a transparent, documented manner.

Infrastructure costs (hosting of CI, package registries, etc.) are
borne by the community or donated. Sponsors are acknowledged but do
not gain decision-making power beyond that of any other contributor.

---

## Trademarks and branding

The "CIVORA" name and logo are trademarks of the project. Use of the
name and logo is governed by a trademark policy (to be created). The
project reserves the right to enforce its trademarks against misuse.

---

## Changes to this document

Changes to this governance document follow the same process as
significant technical decisions: proposal, discussion, consensus (or
steward vote), and public documentation of the change.

---

Last updated: 2026-09-06
