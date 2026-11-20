# Documentation

Design notes, decision records and operator guides for this repository live
here. Decision records live in `docs/adr/`.

- [Product purpose and scope](../README.md#product-scope) and
  [domain glossary](../GLOSSARY.md): the agreed product scope and terminology.

- [Architecture and policy design](architecture.md): the current application,
  design tree, open decisions and verified integration constraints.

- [Consolidated architecture blueprint](architecture-blueprint.md): agreed
  product/runtime and core ownership decisions, with representative package placement.

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

- [API diagrams](diagrams/README.md): the proposed consumer/interface map and
  component architecture, supplied as editable draw.io sources and SVGs with
  embedded diagram data.

- [REST retries, concurrency and cost](research/api-surfaces/request-behaviour.md):
  application request requirements, retention tradeoffs and primary-source
  research on HTTP/PostgreSQL idempotency, admission and API contract tooling.

- [Event registration and ticket lifecycle](research/api-surfaces/event-workflows.md):
  venue recovery, attendee reconfirmation, visitor eligibility, cancellation and
  optional event waitlists across the supported API consumers.

- [Workplace retirement, reactivation and deletion](research/api-surfaces/workplace-lifecycle.md):
  publish effective retirement schedules, preserve confirmations, validate
  reactivation and report removal dependencies to administrators and automation.

- [Plan approval and mapping publication](research/api-surfaces/plan-publication.md):
  distinguish approved immutable assets from operational geometry mappings and
  validate dated-inventory impact before activation.

- [Linked desk and parking lifecycle](research/api-surfaces/linked-bookings.md):
  cancel a both-required pair atomically before either starts; automatic desk
  release preserves parking until its own end or explicit release.

- [Confirmed outages and deadline protection](research/api-surfaces/operational-incidents.md):
  share operational declarations while keeping deadline state with Booking and
  Events and configurable rules with Policy.

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

- [Consider a domain-first modular monolith](adr/0009-use-a-domain-first-modular-monolith.md):
  proposed architecture direction; the repository-style decision remains pending
  across all five candidates.

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

- [Separate Events from venue booking](adr/0023-separate-events-from-venue-booking.md):
  keep registration, tickets and admission in Events while coordinating venue
  capacity changes with Booking through a narrow local transaction.

- [Create event visitor visits with tickets atomically](adr/0024-create-event-visitor-visits-with-tickets-atomically.md):
  roll back a visit created solely for failed registration while retaining an
  existing valid visit reused by the attempt.

- [Retire workplaces before removing protected resources](adr/0025-retire-workplaces-before-removing-protected-resources.md):
  schedule withdrawal from new use, validate reactivation and separate
  dependency-checked deletion from protected commitments.

- [Combine workforce identity and access ownership](adr/0026-combine-workforce-identity-and-access-ownership.md):
  keep source identity, sessions, permissions and delegation in one Identity owner.

- [Keep attendance planning inside Booking](adr/0027-keep-attendance-planning-inside-booking.md):
  isolate the solver while keeping demand and capacity validation together.

- [Separate Exchange synchronisation from Booking](adr/0028-separate-exchange-synchronisation-from-booking.md):
  own provider integration independently while Booking remains the claim authority.

- [Keep operational occupancy evidence in Booking](adr/0029-keep-operational-occupancy-evidence-in-booking.md):
  retain evidence used for release with the claim owner; project committed facts
  into Attendance reports with separately configured retention.

- [Separate plan approval from operational mapping activation](adr/0030-separate-plan-approval-from-operational-mapping-activation.md):
  Plans approves assets, Workplace publishes mappings and Booking validates
  dated-inventory and commitment impact.

- [Share incident declarations with owning deadline workflows](adr/0031-share-incident-declarations-with-owning-deadline-workflows.md):
  give operational facilities custody of confirmed outage scope/times, with
  Booking and Events retaining their own transition authority.

- [Separate linked cancellation from automatic desk release](adr/0032-separate-linked-cancellation-from-automatic-desk-release.md):
  keep deliberate paired cancellation atomic while retaining parking when desk
  evidence cannot establish vehicle departure.
