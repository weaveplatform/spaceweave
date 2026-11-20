---
status: proposed
---

# Consider a domain-first modular monolith

The repository-architecture decision remains pending across the five candidates.
A domain-first modular monolith is the preferred candidate, with
published module interfaces and owned data, using ports and adapters inside
complex modules. This keeps consistency rules local and isolates external
systems, at the cost of enforcing module dependencies and maintaining explicit
cross-module workflow contracts. The [decision analysis](../research/architecture-options-spaceweave.md)
records the reasoning. The [full five-option review set](../research/architecture-options/README.md)
compares detailed repository layouts and tradeoffs. Accepted core boundaries
combine [Identity](0026-combine-workforce-identity-and-access-ownership.md), keep
[planning in Booking](0027-keep-attendance-planning-inside-booking.md), and separate
[Exchange](0028-separate-exchange-synchronisation-from-booking.md) and
[Events](0023-separate-events-from-venue-booking.md). Concrete package placement
and remaining evidence/publication boundaries still require validation. The
agreed logical owners apply to every comparison layout; they do not select a
repository architecture.

One versioned application image supports independently scaled Kubernetes
deployments for HTTP, allocation, Exchange synchronisation, tracing and
maintenance roles.
