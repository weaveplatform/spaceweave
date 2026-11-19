# Own booking state and durable work in PostgreSQL

The Booking module owns local space availability and booking-state transitions;
other modules use its interface. PostgreSQL holds durable application state with
module-owned tables and migrations. Commit booking changes and their background
work records together, then perform external calls after commit using retry-safe
workers and a transactional outbox.

This preserves atomic booking rules and durable follow-up work across API and
worker replicas. Drawings and generated assets use blob storage with Azure and
GCP support, referenced by metadata in PostgreSQL.

Initially use PostgreSQL-backed durable jobs and outbox processing, with leased
claims, retry backoff and operator-visible failures. Production PostgreSQL is a
managed platform service operated separately from application workloads, with
its own availability, backup and restore plan.

Allow only one overlapping desk allocation per person. Office moves secure
replacement capacity and release the previous allocation in one transaction,
including required linked parking.

Required API retry receipts commit with command acceptance as recorded in
[the retry receipt decision](0022-commit-retry-receipts-with-command-acceptance.md).
