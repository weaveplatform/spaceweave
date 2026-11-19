# Documentation

Design notes, decision records and operator guides for this repository live
here. Decision records live in `docs/adr/`.

- [Product purpose and scope](../README.md#product-scope) and
  [domain glossary](../GLOSSARY.md): the agreed product scope and terminology.

- [Architecture and policy design](architecture.md): the current application,
  design tree, open decisions and verified integration constraints.

- [Consolidated architecture blueprint](architecture-blueprint.md): agreed
  product/runtime decisions and the detailed module ownership proposed for review.

- [Operational policy defaults](policy-defaults.md): the agreed, configurable
  initial values and allocation priority rules.

- [Technical research](research/README.md): an entry point for architects and
  engineers, with architecture/API reading paths and evidence conventions.

- [Five detailed architecture options](research/architecture-options/README.md):
  full repository/package layouts, dependency rules, ownership/transaction deep
  dives, common walkthroughs, migration maps and tradeoffs for side-by-side review.

- [Architecture decision analysis](research/architecture-options-spaceweave.md):
  the compact comparison of consistency, modularity, operations and validation.

- [API surfaces, users and consumers](research/api-surfaces/README.md):
  public/private contracts, network exposure, protocols, operations, access,
  compatibility and placement across the five layouts, with verified appendices.

- [REST retries, concurrency and cost](research/api-surfaces/request-behaviour.md):
  application request requirements, retention tradeoffs and primary-source
  research on HTTP/PostgreSQL idempotency, admission and API contract tooling.

- [Allocate advance desk intents together](adr/0001-allocate-advance-desk-intents-together.md):
  consider attendance demand together to place groups by team proximity.

- [Model neighbourhoods as flexible space bundles](adr/0002-model-neighbourhoods-as-flexible-space-bundles.md):
  assign groups to flexible bundles that can balance capacity across office days.

- [Require assignee check-in to mark a desk taken](adr/0003-require-assignee-check-in-to-mark-a-desk-taken.md):
  distinguish an identified desk claim from anonymous physical occupancy.

- [Synchronise room bookings bidirectionally with Exchange Online](adr/0004-synchronise-room-bookings-bidirectionally-with-exchange-online.md):
  reflect bookings, changes and cancellations made in either system in the other.

- [Design for one organisation across many offices](adr/0005-design-for-one-organisation-across-many-offices.md):
  plan for 100,000 staff and 60,000 desks within one enterprise.

- [Keep room requests pending until Exchange accepts](adr/0006-keep-room-requests-pending-until-exchange-accepts.md):
  confirm room requests after the room's acceptance response.

- [Reconcile compatible calendar changes automatically](adr/0007-reconcile-compatible-calendar-changes-automatically.md):
  resolve routine synchronisation automatically and escalate incompatible intent.

- [Separate booking ownership from the acting delegate](adr/0008-separate-booking-ownership-from-the-acting-delegate.md):
  retain the booking owner's identity and the delegate's actions.

- [Use a domain-first modular monolith](adr/0009-use-a-domain-first-modular-monolith.md):
  organise domain ownership with ports and adapters inside substantial modules.

- [Keep app-managed delegation local](adr/0010-keep-app-managed-delegation-local.md):
  grant app-mediated booking authority while synchronising booking outcomes.

- [Own booking state and durable work in PostgreSQL](adr/0011-own-booking-state-and-durable-work-in-postgresql.md):
  use one booking authority and atomically commit state plus follow-up work.

- [Use configurable revisioned blob storage](adr/0012-use-configurable-revisioned-blob-storage.md):
  support Azure/GCP through one interface and preserve immutable plan revisions.

- [Separate workplace definitions from dated booking inventory](adr/0013-separate-workplace-definitions-from-dated-booking-inventory.md):
  plan from snapshots and revalidate capacity when committing bookings.

- [Use explicit allocation cycles and atomic group placement](adr/0014-use-explicit-allocation-cycles-and-atomic-group-placement.md):
  coordinate attendance cohorts and commit linked placements atomically.

- [Use React portals with Go-managed sessions](adr/0015-use-react-portals-with-go-managed-sessions.md):
  share frontend capabilities while keeping authentication tokens server-side.

- [Version runtime policies separately from deployment configuration](adr/0016-version-runtime-policies-separately-from-deployment-config.md):
  control policy changes and preserve existing confirmations' rules.

- [Use one write region with warm regional recovery](adr/0017-use-one-write-region-with-warm-regional-recovery.md):
  keep booking writes consistent and exercise end-to-end regional failover.

- [Correlate staff with explicit source authority](adr/0018-correlate-staff-with-explicit-source-authority.md):
  link identities deliberately and retain ownership of directory and app fields.

- [Provision Entra users and groups through SCIM](adr/0019-provision-entra-users-and-groups-through-scim.md):
  support an explicit provisioning contract while retaining source field authority.

- [Share public REST contracts across UI and automation](adr/0020-share-public-rest-contracts-across-ui-and-automation.md):
  support UI, MCP, SDK, script, testing and Terraform consumers while keeping
  platform-worker interfaces private.

- [Separate human and application authority](adr/0021-separate-human-and-application-authority.md):
  retain actual actor/client/owner identity and independent machine grants.

- [Commit retry receipts with command acceptance](adr/0022-commit-retry-receipts-with-command-acceptance.md):
  recover accepted outcomes after response loss without repeating the mutation
  or durable work submission.
