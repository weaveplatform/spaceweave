# Technical research

SpaceWeave is an enterprise workplace booking platform for approximately 100,000
staff and 60,000 desks across multiple offices, buildings and floors. Its scope
includes advance desk allocation, direct space booking, meeting rooms, parking,
internal events, hosted visits and attendance reporting. Staff and administrators
use separate web portals, including mobile web access.

This research is for technical architects and engineers evaluating the system's
structure, consistency, integrations and operating model. It explains the problem
and relevant technology principles without requiring prior design discussions.

## Architecture reading path

1. [Architecture foundations](architecture-options/architecture-foundations.md):
   primary-source explanations of layered, hexagonal, Clean, domain-driven and
   modular-monolith approaches, plus Go, PostgreSQL and Kubernetes boundaries.
2. [Decision analysis](architecture-options-spaceweave.md): the booking invariants,
   integration failures and operational requirements driving the comparison.
3. [Five detailed candidates](architecture-options/README.md): repository layouts,
   dependency rules, state ownership, transactions, migration and common workflows.

The repository-architecture choice is pending. A modular monolith remains the
preferred candidate; agreed logical owners apply across all five layouts, while
concrete package placement remains subject to validation; a candidate tree illustrates an option,
rather than proving its correctness or performance.

## API reading path

1. [API catalogue](api-surfaces/README.md): consumers, public/private contracts,
   identity profiles, operations, disclosure and compatibility.
2. [Request contract](api-surfaces/request-behaviour.md): required booking retry,
   configuration concurrency, request-budget and operation lifecycle behaviour.
3. [Idempotency and concurrency](api-surfaces/idempotency-and-concurrency.md):
   HTTP/PostgreSQL foundations, atomic acceptance, replay, privacy and retention.
4. [API contract engineering](api-surfaces/api-contract-engineering.md): OpenAPI,
   generation, enforcement, rate admission, backpressure and consumer validation.
5. [Event workflows](api-surfaces/event-workflows.md): registration, venue recovery,
   reconfirmation, visitor eligibility, cancellation and optional event waitlists.
6. [Workplace lifecycle](api-surfaces/workplace-lifecycle.md): scheduled retirement,
   identity-preserving reactivation, protected confirmations and guarded deletion.
7. [Plan publication](api-surfaces/plan-publication.md): asset approval, Workplace
   mapping activation and Booking dated-inventory validation across all layouts.
8. [Operational incidents](api-surfaces/operational-incidents.md): confirmed outage
   declarations, guarded expiry and business-owned deadline protection.

The catalogue links specialised research on authentication, role-based access,
Entra groups, Conditional Access, MCP/Terraform and provider protocols. The
[current-system inventory](api-surfaces/current-surfaces.md) identifies implemented
floor-plan surfaces and migration dependencies; proposed booking APIs are separate.

## Evidence and decision conventions

- **Source facts** describe standards, framework behaviour or provider contracts,
  with citations to the primary specification, maintainer or official documentation.
- **Application requirements** describe SpaceWeave's product and operational rules.
  A framework citation does not independently establish those requirements.
- **Design proposals** identify a concrete mechanism, its tradeoffs and alternatives.
  Package counts and directory names are descriptive, not capacity estimates.
- **Unresolved choices** state the missing requirement or validation evidence.
  Numerical defaults need an application-specific rationale. Draft specifications
  and provider-specific conventions are identified separately from published standards.

The [product scope](../../README.md#product-scope), [glossary](../../GLOSSARY.md),
[decision records](../README.md) and [policy defaults](../policy-defaults.md)
provide the application baseline. The current implementation is a floor-plan
processing tool; the research does not imply that the booking platform or its
production targets have already been demonstrated.
