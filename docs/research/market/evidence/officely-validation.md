# Validation: officely

Validated 2026-11-25. All ledger URLs were fetched (curl + text extraction; TechCrunch via WebFetch after a 403).

## Priority check: are getofficely.com and officely.app the same company?

**Yes, same company. No WRONG-VENDOR rejections.** Evidence:
- DPA at https://trust.officely.app/data-processing-agreement: "Hotelflex Limited, trading as Officely ("Officely"), provides Officely Enterprise to the customer"; objections go to security@getofficely.com.
- Privacy policy at https://getofficely.com/privacy-policy: "Hotelflex Limited, trading as Officely, is the data controller"; terms at https://getofficely.com/terms-of-service: "HotelFlex Ltd (trading as Officely)".
- Slack Marketplace listing of the classic app: "Developer HotelFlex Limited", ToS link https://getofficely.com/terms-of-service.
- getofficely.com pages (home, /pricing, /api, /integrations, /status) link directly to docs.officely.app, trust.officely.app, api.officely.app and mcp.officely.app; /api says "The full reference is at docs.officely.app".
- The Enterprise help article lives on help.getofficely.com (C31); the status page on getofficely.com tracks both "Officely Slack App" and "Officely Web App".
- Company number was not found on the fetched pages (not needed given the above).

## Summary counts
- ACCEPT: 114
- REVISE: 2 (C8, C94)
- REJECT: 0 ledger claims (plus 8 unsourced draft statements rejected, see below)
- NEW: 20 (N1-N20)

## Ledger verdicts

| id | verdict | flags | corrected text (for REVISE) | note |
|---|---|---|---|---|
| C1 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C2 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C3 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C4 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C5 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C6 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C7 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C8 | REVISE |  | Pricing page says "All 35 integrations are ready-made and included"; the integrations page itself lists 34 (HR and leave 30, Calendar 2, Directory and SSO 1, Where people book 1). Cite as "35 per pricing page (34 listed on /integrations)". | Count mismatch between two vendor pages. |
| C9 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C10 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C11 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C12 | ACCEPT | THIRD-PARTY-ONLY |  | Official corroboration: privacy policy gives a UK postal address (Temple Guiting, Cheltenham area). YC page also oddly shows "Founded: 2021, Batch: Summer 2017". |
| C13 | ACCEPT | THIRD-PARTY-ONLY |  | Page title text. |
| C14 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C15 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C16 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C17 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C18 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C19 | ACCEPT |  |  | "Developer HotelFlex Limited"; consistent with privacy policy and DPA entity (Hotelflex Limited trading as Officely). |
| C20 | ACCEPT | THIRD-PARTY-ONLY, STALE |  | curl got 403; WebFetch confirmed "$2 million seed round" and "This round was led by TEN13". |
| C21 | ACCEPT | THIRD-PARTY-ONLY, STALE |  | 2021 article; draft correctly labels as historical. |
| C22 | ACCEPT | THIRD-PARTY-ONLY, STALE |  | 2021 article; historical. |
| C23 | ACCEPT | STALE |  | Help article dated 2025-07-15 (>12 months). |
| C24 | ACCEPT | STALE |  | 2025-07-15. |
| C25 | ACCEPT | STALE |  | 2025-07-15. |
| C26 | ACCEPT | STALE |  | 2025-07-15. |
| C27 | ACCEPT | STALE |  | 2025-05-29. |
| C28 | ACCEPT | STALE |  | 2025-05-29. |
| C29 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C30 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C31 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C32 | ACCEPT |  |  | getofficely.com/v2 now redirects to /why-officely; cite the final URL. |
| C33 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C34 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C35 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C36 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C37 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C38 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C39 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C40 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C41 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C42 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C43 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C44 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C45 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C46 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C47 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C48 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C49 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C50 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C51 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C52 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C53 | ACCEPT |  |  | Quote is in the errors table (infrastructure category). No numeric limits found in any fetched docs page. |
| C54 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C55 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C56 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C57 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C58 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C59 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C60 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C61 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C62 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C63 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C64 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C65 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C66 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C67 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C68 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C69 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C70 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C71 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C72 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C73 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C74 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C75 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C76 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C77 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C78 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C79 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C80 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C81 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C82 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C83 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C84 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C85 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C86 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C87 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C88 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C89 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C90 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C91 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C92 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C93 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C94 | REVISE |  | Core database can be located in the EU, the US, or Cloudflare's FedRAMP Moderate data centres, agreed at account setup; otherwise Officely chooses per customer. | Ledger claim omits the third option the same quote sentence names. Note: this is a data-centre location choice, not a FedRAMP authorisation of Officely. |
| C95 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C96 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C97 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C98 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C99 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C100 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C101 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C102 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C103 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C104 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C105 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C106 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C107 | ACCEPT |  |  | Supports the claim as written; but the draft §3 misuses it for Enterprise (see unsourced U6). |
| C108 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C109 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C110 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C111 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C112 | ACCEPT |  |  | Same misuse in draft §3. |
| C113 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C114 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C115 | ACCEPT |  |  | Quote found verbatim (or near-verbatim) on fetched page. |
| C116 | ACCEPT |  |  | "The four tools the Officely MCP server gives an agent" on /building/start/agents; names search/docs/read/write appear on /building/other-mcp. |

## Statements the draft admits without a ledger row (specifically requested)

| statement | result |
|---|---|
| New subprocessors get 20 days' notice | VERIFIED -> N1 |
| Core DB option "Cloudflare FedRAMP-Moderate" | VERIFIED -> N2 (also fixes C94). Wording: it is a choice of Cloudflare's FedRAMP Moderate data centres, not an Officely FedRAMP authorisation. |
| ~30 HR and leave integrations incl. Workday, SAP SuccessFactors, Personio, HiBob, BambooHR, Rippling | VERIFIED -> N3 (exactly 30 tagged "HR and leave"; all six named vendors listed) |
| Webhook secret rotation gives a 24-hour overlap | VERIFIED -> N4 |

## Unsourced statements (no ledger claim) — verdict REJECT unless re-cited

Statements below have no ledger row. Where I verified them, they become NEW claims and the draft just needs the citation; the REJECT stands for the uncited text as written.

| # | draft location | statement | verdict | note |
|---|---|---|---|---|
| U1 | §8 | 20 days' subprocessor notice | REJECT (as uncited) | Verified, cite N1 |
| U2 | §7 | FedRAMP-Moderate hosting option | REJECT (as uncited) | Verified, cite N2 |
| U3 | §5 | ~30 HR integrations, named vendors, leave-aware attendance | REJECT (as uncited) | Verified, cite N3 |
| U4 | §11 | 24h secret overlap | REJECT (as uncited) | Verified, cite N4 |
| U5 | §11 | 12 attempts, auto-disable, deliveries stored and resendable | REJECT (as uncited) | Verified, cite N5 |
| U6 | §3 Analytics | "Enterprise has attendance reports and a separate workspace reporting API [C107][C112]" | REJECT | **Misattribution.** C107 says the workspace API token is created "in Officely for Slack or Microsoft Teams"; it is the classic product's API, not Enterprise's. Rewrite as: classic product has a workspace reporting API; Enterprise attendance reporting is uncited. |
| U7 | §11 | "Every change is an Event ... readable via GET /events" | REJECT | evt_ and sequence appear in example payloads (webhook-endpoint page) but GET /events was not found in the fetched pages; needs a citation to the Event reference page. |
| U8 | §11 | Event catalogue examples (booking.created, floor_plan.published, visit.arrived, signal.recorded ...) | REJECT | Only booking.checkin_closed was seen (booking page). Needs per-object citations. |
| U9 | §10 | Idempotency keys supported | REJECT (as uncited) | Verified, cite N6 |
| U10 | §10 | 36 objects and 5 lookups | REJECT (as uncited) | Verified, cite N7 |
| U11 | §4 | Last 3 published versions kept; resource goes in smallest containing Space | REJECT (as uncited) | Verified, cite N8, N9 |
| U12 | §9 | Permissions per operation; delegation headers | REJECT (as uncited) | Verified, cite N10 |
| U13 | §5 | Entra deactivation ends bookings and assignments | REJECT (as uncited) | Verified, cite N11 |
| U14 | §5 | Working location and building mapping (Google Workspace / Microsoft Places) | REJECT (as uncited) | Verified, cite N12 |
| U15 | §10 | Classic API endpoints list, CSV export, read-only GET only | REJECT (as uncited) | Verified, cite N13 |
| U16 | §14 | Staging server api.development.getofficely.com | REJECT (as uncited) | Verified, cite N14 (the "likely internal" remains inference) |
| U17 | §13 | CLI `officely integrations teams setups prepare` | REJECT (as uncited) | Verified, cite N15 |
| U18 | §15 | MCP auth via OAuth as person or Account API Token | REJECT (as uncited) | Verified, cite N16 |
| U19 | §15 | Booking rules still apply to agent bookings | REJECT (as uncited) | Verified, cite N17 |
| U20 | §8 | Trust centre contents list | REJECT (as uncited) | Verified, cite N18 |
| U21 | §2 | Check-in only within a window; occupancy informational only | REJECT (as uncited) | Verified, cite N19, N20 |
| U22 | §2 | BookingRule settings list (access/approval, check-in, granularity, open hours, waitlist, booking window) | REJECT | Plausible (booking-rule page mentions access, booking window, open hours, booking lengths, waitlist) but no ledger row; add one. |
| U23 | §8 | AuditLogEntry source "api/web/mcp/slack/scim" | REJECT | Incomplete: the Source Type values table also includes `internal` (Officely staff or Officely itself). Add a ledger row and include `internal`. |
| U24 | §13 | "A Boost.space connector page appeared in search but was not fetched" | REJECT | Unverified; remove. |
| U25 | §3 | "The product runs in Teams and the web [C106]" | REJECT (as cited) | C106 covers Teams only. "Web" is supported by the getofficely.com nav ("Book the right room in Teams or the web") and the status page "Officely Web App" (C33); re-cite. |
| U26 | §1, §14, §17 | Segment, docs-quality and spaceweave-relevance analysis | n/a | Labelled as analysis; fine. |
| U27 | Intro | Label "classic" for the Slack/Teams product | note | Vendor does not use "classic"; it is the drafter's label. Keep in quotes or say "Officely for Slack/Teams". |

## Data-model trace issues (§12)

Every entity exists in the API reference (field names checked on the fetched pages), but many fields and relations cite a claim that does not cover them:
- **Account**: cited [C44] (ID prefixes only). "owns Organisations, People" not traced. `acc_` prefix appears on the Place page, not the intro text.
- **Organisation**: cited [C70] (booking-rule precedence). "owns Places and BookingRules" not traced; `org_` prefix not traced.
- **Place**: address, status, belongs-to-Organisation not traced (present on /reference/place: "account ... The Account that owns the Place", "organisation"). Note Place is also owned by an Account.
- **Space**: `parent_space`, `level_index`, `status` not traced (present on /reference/space).
- **Resource**: `amenities`, `occupancy`, ResourceGroup membership not traced (present on /reference/resource).
- **FloorPlan**: `revision`, `document_digest`, `status`, "maps one level Space" not traced (present on /reference/floor-plan: "A level can have only one active FloorPlan").
- **Booking**: `booked_for`, `booked_by`, `pending_change`, `starts_at/ends_at`, and relations to Guest, Approval, Hold not traced (all present on /reference/booking).
- **BookingRule**: `position`, `audience`, `targets` not traced (present on /reference/booking-rule).
- **Signal**: `trace_id`, SignalType JSON-schema typing, optional Person/Place/Space relations not traced.
- **Automation**: `trigger`, `source` field names not traced.
- **AuditLogEntry**: `outcome`, `targets`, source values not traced; source list missing `internal` (see U23).
- **WebhookEndpoint**: WebhookDelivery relation not traced (verified via N5).
Fix: add ledger rows citing each object's reference page; nothing in the table was found to be wrong apart from the AuditLogEntry source list.

## "Not publicly documented" checks (2-3 queries each)
- SOC 2 / ISO 27001 / Cyber Essentials: no evidence found (trust centre, data-handling, DPA, getofficely.com, web search). Stands. Related: the classic Slack listing self-declares "HIPAA compliant yes" (a marketplace self-attestation, not a certification). Not added as a claim; the drafter may mention it with that caveat.
- Mobile apps: no official App Store or Google Play listing found. GetApp (third-party) lists iPhone/iPad/Android, likely via Slack/Teams clients. Stands.
- WCAG/VPAT: nothing found. Stands.
- Numeric rate limits for the Enterprise API: none in any fetched docs page. Stands.
- Audit-log retention: not found. Stands.
- Zapier/Power Automate: not on the integrations page. Stands.

## New claims

| id | claim | url | quote | verdict |
|---|---|---|---|---|
| N1 | New subprocessors: at least 20 days' email notice, with an objection right | https://trust.officely.app/data-processing-agreement | "at least 20 days before a new subprocessor processes Customer Personal Data, identifying the provider and its role" | ACCEPT |
| N2 | Core DB location options include Cloudflare's FedRAMP Moderate data centres | https://trust.officely.app/data-handling | "Customers may opt to locate their core database in the European Union, the United States or Cloudflare's FedRAMP Moderate data centres." | ACCEPT |
| N3 | Integrations page lists 30 HR and leave integrations (incl. Workday, SAP SuccessFactors, Personio, HiBob, BambooHR, Rippling) of 34 total | https://getofficely.com/integrations | "All 34 HR and leave 30 Calendar 2 Directory and SSO 1 Where people book 1" | ACCEPT |
| N4 | Webhook secret rotation keeps the previous secret signing for 24 hours | https://docs.officely.app/reference/webhook-endpoint | "The previous secret keeps signing for 24 hours, so each request carries both signatures" | ACCEPT |
| N5 | 12 automatic delivery attempts; endpoint disabled after 3 days of failures; deliveries readable and resendable | https://docs.officely.app/reference/webhook-endpoint | "Officely disables an endpoint whose requests have failed for 3 days. Each Event sent to an endpoint is a WebhookDelivery you can read and resend." / "the 12 automatic attempts" | ACCEPT |
| N6 | Idempotency keys supported on writes | https://docs.officely.app/reference/booking | "idempotency_key string Optional Key that makes the request safe to retry." | ACCEPT |
| N7 | API reference has 36 objects and 5 lookups | https://docs.officely.app/reference/introduction | "Objects · 36 ... Lookups · 5 Availability Booking Policy Me Place Day Schedules" | ACCEPT |
| N8 | Only the 3 most recent published floor-plan versions are kept | https://docs.officely.app/reference/floor-plan | "Only the 3 most recent versions are kept." | ACCEPT |
| N9 | Published desk goes in the smallest Space containing its box centre | https://docs.officely.app/reference/floor-plan | "The Resource goes in the smallest Space on the map that contains the centre of its box" | ACCEPT |
| N10 | Per-operation permissions; delegation via Officely-Delegation and Officely-On-Behalf-Of headers | https://docs.officely.app/reference/authentication | "A request made under a granted Delegation carries two headers together: Officely-Delegation ... and Officely-On-Behalf-Of" | ACCEPT |
| N11 | Entra inactive -> person deactivated, bookings/assignments ended | https://docs.officely.app/using/directory-sync | "When Microsoft marks someone inactive, Officely deactivates them and automatically ends their current and upcoming bookings, waitlist places, desk assignments, group memberships and roles." | ACCEPT |
| N12 | Optional calendar working-location feature and building mapping | https://docs.officely.app/reference/integrations/calendar | "office days as each person's working location, and naming the mapped building in it" | ACCEPT |
| N13 | Classic workspace API is GET-only: offices, users, teams, work-statuses, insights; CSV supported | https://api.getofficely.com/openapi.json | paths /v1/offices, /v1/users, /v1/teams, /v1/work-statuses, /v1/insights/* all `get` only; `text/csv` response type present | ACCEPT |
| N14 | Classic spec lists a staging server | https://api.getofficely.com/openapi.json | "url":"https://api.development.getofficely.com","description":"Staging" | ACCEPT |
| N15 | Teams setup via CLI command | https://docs.officely.app/reference/integrations/teams | "Use officely integrations teams setups prepare from a trusted terminal client." | ACCEPT |
| N16 | MCP: OAuth as a person, or Account API Token for unattended use | https://docs.officely.app/building/other-mcp | "To sign in as a person: leave authentication on OAuth ... To connect without a person: send Authorization" | ACCEPT |
| N17 | Booking rules apply to agent bookings | https://getofficely.com/api | "Your booking rules still apply When Officely refuses a booking, the agent gets the reason back as an error." | ACCEPT |
| N18 | Trust centre: security overview, privacy policy, ToS, DPA, subprocessors | https://trust.officely.app | "Security overview ... Privacy Policy ... Terms of Service ... Data Processing Agreement ... Subprocessors" | ACCEPT |
| N19 | Check-in only while check-in window is open | https://docs.officely.app/reference/booking | "Works only while the Booking's check-in window is open." | ACCEPT |
| N20 | Resource occupancy is informational | https://docs.officely.app/reference/resource | "Recommended number of people. For information only; it doesn't limit Bookings." | ACCEPT |
