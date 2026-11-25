# Workspace-booking market research

A validated survey of desk, room and parking booking products. It covers features, space
kinds, floor-plan pipelines, hosting, compliance, authentication, REST APIs, webhooks,
data models, SDKs, AI integrations and pricing. It was written to inform spaceweave's
domain model and APIs. Researched and validated on **2026-11-25**.

## Contents

| Profile | What it covers | Data-model diagram |
|---|---|---|
| [Skedda](skedda.md) | Rules, quotas and maps for mid-market offices and schools. Read-only Beta API, signed webhooks | [skedda](diagrams/skedda.drawio.svg) |
| [deskbird](deskbird.md) | Mobile-first hybrid-work app. Public API, read-only MCP server | [deskbird](diagrams/deskbird.drawio.svg) |
| [Robin](robin.md) | Desks and rooms for mid-market and enterprise. DXF map import, MCP server | [robin](diagrams/robin.drawio.svg) |
| [Clearooms](clearooms.md) | Priced per desk, UK-hosted. API docs sit behind login; model built from Zapier | [clearooms](diagrams/clearooms.drawio.svg) |
| [Officely](officely.md) | Slack/Teams-native "classic" app plus Officely Enterprise, with a JSON floor-plan API | [officely](diagrams/officely.drawio.svg) |
| [Envoy](envoy.md) | Workplace platform covering visitors, desks, rooms and deliveries. OAuth2 API, webhooks, MCP | [envoy](diagrams/envoy.drawio.svg) |
| [Eptura](eptura.md) | Condeco → Engage, iOFFICE → Workplace, SpaceIQ, Serraview, Archibus. CAD-heavy | [eptura](diagrams/eptura.drawio.svg) |
| [Archie](archie.md) | Coworking and hybrid offices. OAuth2 client-credentials API, encrypted webhooks | [archie](diagrams/archie.drawio.svg) |
| [Open source](open-source.md) | Seatsurfing, LibreBooking, WARP, MRBS, Mensam. Self-hostable | [five diagrams](diagrams/README.md#open-source) |
| [Common domain model](domain-model.md) | Merged entity model, with design recommendations for spaceweave | [common-domain-model](diagrams/common-domain-model.drawio.svg) |

Each diagram is a draw.io source (`.drawio`) with its SVG export (`.drawio.svg`). The SVG
embeds the diagram, so it also opens in draw.io for editing. They use the same dark-tile
style as the infraweave diagrams. Columns group entities into places, bookables, bookings and
people or integration. Amber cards are enums, and grey dashed cards and lines are inferred
rather than documented. [`diagrams/README.md`](diagrams/README.md) lists them all.

## Method and acceptance criteria

1. **Research.** Each vendor had its own research agent working from a fixed 18-section template.
   - Source priority: API reference > official docs > vendor GitHub > vendor blog > third-party reviews.
   - Every claim went into a ledger with its URL, a verbatim quote, a source tag and the page date.
   - Gaps are recorded as "Not publicly documented" and never guessed.
2. **Independent validation.** A separate agent re-fetched every cited URL and gave every claim a verdict:
   - **ACCEPT:** the source supports the claim as written.
   - **REVISE:** the claim is supportable only with a correction.
   - **REJECT:** the link is dead, or the source does not support the claim.

   The validator also:
   - flagged STALE claims (page more than about 12 months old) and THIRD-PARTY-ONLY claims;
   - tried to disprove each "not publicly documented" gap with fresh searches;
   - checked every data-model entity against an accepted claim.
3. **Acceptance.** The published profiles keep only ACCEPT claims, REVISE claims in their
   corrected form, and new claims the validator found and sourced.
   - Draft statements with no ledger claim were cited, corrected or removed.
   - Stale and third-party claims are labelled inline.
   - The ledgers and validation reports are kept in [`evidence/`](evidence/). Claim ids such as C12 and N3 refer to them.
4. **Spot check.** The lead author re-read 16 randomly sampled claims, two per vendor, against their live pages. All 16 were supported. One wording refinement was made: Archie analytics.

| Profile | Ledger claims | Accepted | Revised | Rejected | New (validator) |
|---|---:|---:|---:|---:|---:|
| Skedda | 91 | 90 | 1 | 0 | 3 |
| deskbird | 102 | 95 | 7 | 0 | 10 |
| Robin | 96 | 92 | 4 | 0 | 9 |
| Clearooms | 88 | 82 | 6 | 0 | 17 |
| Officely | 116 | 114 | 2 | 0 | 20 |
| Envoy | 127 | 125 | 2 | 0 | 6 |
| Eptura | 164 | 161 | 3 | 0 | 9 |
| Archie | 100 | 98 | 2 | 0 | 4 |
| Open source | 72 | 72 | 0 | 0 | 6 |
| **Total** | **956** | **929** | **27** | **0** | **84** |

No ledger claim was rejected outright. Most problems were in the drafts' prose, which the
counts above don't capture:
- statements that had no ledger claim behind them;
- "not publicly documented" gaps that validators disproved, about 20 in total (for example Clearooms' sandbox and audit logs, Eptura's DPA and Archibus MCP server, and Robin's sub-processor list);
- three conflicts inside vendors' own docs, resolved in favour of the dated source (Envoy).

## Comparison matrices

Each cell summarises the matching vendor profile, where the fact is cited. Legend: ✅ supported ·
➖ partial · ❌ not supported or explicitly absent · n/d not publicly documented. "Eptura"
means Engage unless another product is named. "Officely (Ent.)" means Officely Enterprise.

### 1. Space kinds

| Kind | Skedda | deskbird | Robin | Clearooms | Officely | Envoy | Eptura | Archie | Seatsurfing (OSS) |
|---|---|---|---|---|---|---|---|---|---|
| Hot desk | ✅ `bookable` asset, Type "Desk" | ✅ `flexDesk` resource | ✅ `hot` / `hoteled` desk types | ✅ desk pin on plan | ✅ (Ent.) Resource `type=desk` | ✅ Space `type=DESK` | ✅ all-day / hourly personal spaces | ✅ `floating-chair` (mapping inferred) | ✅ untyped "space" |
| Assigned desk | ✅ `assigned` asset, Premier only | ✅ `DEDICATED` accessInfo, autoBook | ✅ `shared` RRULE (`assigned` deprecated) | ✅ incl. part-time allocation | ✅ (Ent.) Assignment on capacity-1 Resource | ✅ bookable only by assignee | ✅ fixed / flexi assignment | ✅ dedicated desk groups | n/d |
| Neighbourhood / zone | ➖ map-only access rules; not in API | ➖ zones with group access; no neighbourhood kind | ➖ visual-only desk groups; MCP only | ✅ drawn polygon, team-restricted | ➖ (Ent.) pool / ResourceGroup (inferred) | ✅ restrict to employees / SCIM groups | ➖ anchor points; Workplace: neighborhoods API | ➖ marketed; no API enum | n/d (areas = floors) |
| Meeting room | ✅ `bookable` asset with capacity | ✅ separate `/rooms` API | ✅ on-demand / scheduled / request-only | ✅ 1:1 with M365/Google calendar | ✅ (Ent.) Resource `type=room` | ✅ `ROOM`, calendar-backed | ✅ self-managed or managed | ✅ `conference-room`, `phone-booth` | ✅ whole rooms |
| Parking | ✅ Type "parking space" | ✅ `parking` resource | ✅ UI; no REST endpoint | ✅ modelled as hot desks | ✅ (Ent.) `parking_bay` in `car_park` | ➖ UI only; not in API | ✅ personal-space workspace type | ✅ `parking`, `parking-level` | ✅ "garage" spaces |
| Lockers | n/d (generic bookable, inferred) | ➖ as "Other" resources | ✅ multi-day, optional check-in | n/d | ✅ (Ent.) `type=locker` | n/d | ✅ personal-space workspace type | ➖ coworking page; `resource` inferred | n/d |
| Event space | n/d | n/d | ➖ via request-only spaces | ➖ internal Event Management | ❌ (Ent.) no event Resource type | n/d | ✅ Engage Events; not in API | ✅ `event-space` with sub-rooms | n/d |
| Equipment / add-ons | ✅ `addOn` assets with quantity cap | ➖ equipment tags; "Other" resources | ✅ template-based custom resources | ❌ desk attributes, not bookable | ✅ (Ent.) `equipment`, `pass`, `session` | n/d | ➖ vendors/catering; delivery points | ➖ marketed; `resource` inferred | n/d |

### 2. API & auth

| Item | Skedda | deskbird | Robin | Clearooms | Officely | Envoy | Eptura | Archie | Seatsurfing (OSS) |
|---|---|---|---|---|---|---|---|---|---|
| Public REST API (read/write?) | ✅ read-only, Beta, 3 resources | ✅ R/W bookings; floors/zones/resources read-only | ➖ R/W desk reservations; narrow coverage | ➖ claimed full parity; docs login-gated | ✅ (Ent.) R/W, dated versions | ✅ R/W desks and reservations | ✅ Engage Admin API R/W, 5 collections | ✅ R/W, ~60 resource groups | ✅ R/W, 99 paths |
| OpenAPI spec available | ✅ OpenAPI 3.1.1 download | ➖ per-endpoint 3.0.1 fragments only | ➖ per-page 3.1.0 fragments only | ➖ exists per G-Cloud; not public | ❌ (Ent.) not found; Slack/Teams API: 3.0.3 | ➖ per-page 3.0.3 fragments only | ➖ behind APIM login; Archibus Swagger public | ➖ per-page 3.1.0 fragments only | ✅ 3.1.0 in repo |
| API auth method | Static per-venue key, Bearer; no OAuth | Company-scoped API key, Bearer | `Access-Token` header; partner OAuth impersonation | Admin API tokens; header n/d | (Ent.) account tokens; OAuth2 PKCE person tokens | OAuth2 (3 grants) or `X-API-Key` | APIM subscription key + Bearer JWT | client id/secret → Bearer, scoped | JWT Bearer, Basic, service-account tokens |
| Webhooks (signed?) | ✅ Standard Webhooks, HMAC-SHA256 | ❌ none; `updatedAt` polling | ❌ no outbound webhooks | ➖ Zapier REST-hooks only; signing n/d | ✅ (Ent.) Standard Webhooks signed | ✅ HMAC-SHA256 `X-Envoy-Signature` | ✅ Listener, secretKey, AES payload; Workplace HMAC | ✅ HMAC-SHA256, AES-GCM payload | n/d |
| MCP server | ❌ none official; Zapier listing | ✅ official, read-only, admin-only | ✅ official; not for EU orgs | n/d | ✅ (Ent.) `mcp.officely.app`, 4 tools | ✅ `mcp.envoy.com`; tool list n/d | ➖ AI Assistant preview; Archibus MCP server | n/d | ✅ MCP server (features page) |
| Official SDKs | ❌ none | ❌ none | ➖ Java SDK, archived | ❌ none | n/d; CLI + agent plugins | ➖ Node.js integrations SDK only | n/d; Postman collection | ❌ none; unofficial TS SDK | n/d |
| End-user SSO protocols | SAML 2.0; OIDC n/d | SAML 2.0; OIDC for Entra only | SAML 2.0; Google/M365 SSO | SAML | (Ent.) Entra ID; protocol not named | SAML 2.0; OIDC n/d | Platform: OIDC + SAML 2.0; Engage: SAML | SAML 2.0 only | OAuth2/OIDC; SAML n/d |
| SCIM (which IdPs) | Entra ID, Okta | Entra ID, Okta | Okta, Entra ID, OneLogin | Entra ID, Okta | (Ent.) Entra ID only | Okta, Entra, OneLogin, Sailpoint, Centrify | Entra ID, Okta | Entra, Okta (SCIM v2) | SCIM 2.0 (Plus Plugin); IdPs n/d |
| Rate limit | 100 req/min per key | 10 req/s per key | Analytics 30 POST/min; global n/d | applies; values n/d | (Ent.) 429 code; numbers n/d | 10 req/s per token | 500/min, 50k/week | 100 req/10 s, then 1-min IP block | n/d |
| Plan that unlocks API | Premier or AllBooked Advanced | Professional+ | "plan with API tokens"; which n/d | every plan | (Ent.) single plan, included | n/d (webhooks row unconfirmed) | unverified; per-customer APIM products | Pro paid add-on; Enterprise custom | free open source |

### 3. Floor-plan pipeline

| Item | Skedda | deskbird | Robin | Clearooms | Officely | Envoy | Eptura | Archie | Seatsurfing (OSS) |
|---|---|---|---|---|---|---|---|---|---|
| Accepted input formats | Any layout: PDF, CAD, PNG, sketch | SVG, PDF, JPG, PNG, WEBP; no CAD | DXF self-serve; PDF/JPG/PNG via team | PNG, SVG, PDF, TIFF; no CAD | (Ent.) n/d; Slack/Teams: PNG link | PNG/JPEG; desk CSV with X/Y | Engage: JPEG/PNG; platform: AutoCAD/Revit plugins | "usually PNG or JPEG" | PNG, JPEG, GIF, SVG or built-in designer |
| Who authors | Vendor service (24–48 h) | Self-serve upload + overlay | Both: DXF/editor or maps team (2–3 days) | Self-serve; drafting service from sketch (48 h) | (Ent.) self-serve draft → publish | Self-serve map editor | Self-serve pins; CAD plugins publish | Self-serve drag and resize | Self-serve drag and drop |
| Auto-detection of desks/rooms | n/d | ➖ "Enhance with AI" image only; marketing claims | ❌ desks/spaces placed by hand | ❌ only assigns placed desks to zones | n/d | ➖ AI desk placement marketed; availability unclear | ➖ from pre-layered CAD only; none from raw | n/d | n/d |
| Geometry exposed via API | ❌ no coordinates in API | ❌ only office lat/long + radius | ❌ none in REST or MCP | n/d | ✅ (Ent.) `floor-manifest/1` rects + polygons | ➖ desk x/y 0–1, create-only | ❌ Engage; Archibus SVG; Public API invite-only | ➖ `map`/`map_meta`/`origin`, keys undocumented | ✅ box x,y,w,h,rotation + map |
| Indoor-map standards (IMDF/IFC etc.) | n/d | n/d | n/d | n/d | n/d | n/d | ➖ IFC in Archibus only; IMDF n/d | n/d | n/d |

### 4. Hosting & compliance

| Item | Skedda | deskbird | Robin | Clearooms | Officely | Envoy | Eptura | Archie | Seatsurfing (OSS) |
|---|---|---|---|---|---|---|---|---|---|
| Cloud provider | Microsoft Azure | GCP (Intelligence feature); "hosted in Germany" | AWS + Google Cloud (split n/d) | AWS (Aurora, multi-AZ) | (Ent.) Cloudflare; Slack app: GCP | AWS, Cloudflare in front | Engage: Azure; older page AWS S3; Serraview AWS | Google Cloud DBs | SaaS hosted in Germany; provider n/d |
| Regions / data residency | US, EU, AU, CA isolated; fixed at signup | "exclusively EU"; US env mentioned, unconfirmed | US or EU; EU loses some features | Ireland/EEA; customer cannot choose | (Ent.) EU, US or FedRAMP DCs; some processing outside | US; EU (Frankfurt) for Visitors; Workplace EU n/d | Engage n/d; Serraview: 5 regions | Canada; custom residency on Enterprise | self-host anywhere; SaaS Germany |
| On-prem option | ❌ SaaS only | n/d | ❌ SaaS only; on-prem n/d | ❌ public cloud only | n/d (appears SaaS only) | n/d | Engage n/d; Archibus: on-prem / partner-hosted | n/d | ✅ self-hosted (Docker, K8s) |
| SOC 2 | ✅ Type 2 (July 2025) | ✅ Type 2 | ✅ type unstated (2022 blog) | ❌ not held (AWS only) | n/d | ✅ Type 2 | ❌ not in trust portal; older: via AWS/Azure | ✅ Type II claimed; no report | n/d |
| ISO 27001 | ❌ "No" (MS attestation) | ✅ ISO/IEC 27001:2022 | ✅ (2022 blog) | ❌ not held; Cyber Essentials Plus | n/d | ✅ trust centre | ✅ trust portal | n/d (infra provider only) | n/d |
| DPA / sub-processor list public | ✅ DPA auto-applies; Trust Center list | ➖ DPA via trust centre; 5 subprocessors | ➖ DPA by email; sub-processor table public | ✅ DPA published; 6 subprocessors | ✅ (Ent.) DPA + list on trust centre | ✅ DPA in trust centre; list public | ✅ DPA + sub-processor list linked | ✅ DPA + list, 10-day objection | n/d |

### 5. Pricing

| Item | Skedda | deskbird | Robin | Clearooms | Officely | Envoy | Eptura | Archie | Seatsurfing (OSS) |
|---|---|---|---|---|---|---|---|---|---|
| Licensing unit | Per space, annual; users unlimited | Per active user | Per space/desk at least partly; users n/d | Per managed space; users unlimited | (Ent.) per bookable space | Per bookable resource; Visitors per location | Per user; Visitor per location | Per resource; Coworking per active booker | Plugin per instance, by user tier |
| Entry price (date) | US Plus $249/mo (35 spaces); EMEA $99/mo (2026-11-25) | USD 3.75/active user/mo, annual, 25+ users (2026-11-25) | Not published; quote only | £60/mo for 1–19 desks, annual (2026-11-25) | (Ent.) $2/space/mo, $500/mo minimum (undated, refs G2 Fall 2026) | $5/resource/mo + unstated platform fee (2026-11-25) | Not published (read 2026-11-25) | Desk Starter $159/mo, 1 location (2026-11-25) | n/d |
| Free tier / trial | No free tier; 14-day trial | Starter ≤15 users after trial | No free tier; time-limited product trials | No free tier; 30-day trial | (Ent.) no free plan; trial n/d | Visitors Basic free; Rooms 60-day trial | n/d | No free tier; 14-day test env after demo | Free self-hosted; SaaS and plugin free ≤10 users |
| SSO/SCIM gating | SCIM Premier; SAML tier unclear | UDM Plus add-on; MS/Google login all tiers | SAML and SCIM need extra entitlements | Every plan | (Ent.) Entra SSO in single plan; SCIM not on pricing | SAML not on Visitors Basic; SCIM Enterprise | Unverified (inferred both tiers) | Pro and above | OIDC in core; SCIM needs Plus Plugin |

## Takeaways for spaceweave

These are analysis, not sourced fact. The supporting detail is in each profile's §17 and in the [domain model](domain-model.md).

1. **Floor-plan ingestion is the gap across the whole market.** Every product starts from one of these:
   - an image with hand-placed pins: deskbird, Envoy, Archie, Clearooms, Engage, Seatsurfing;
   - a human tracing service: Skedda, Robin, Clearooms;
   - CAD that was layered by hand beforehand: Eptura Workplace, Serraview, Archibus and the platform plugins; Robin's DXF import is walls-only.

   No surveyed product documents finding walls, openings, rooms and desks in an unprepared DWG or PDF. That is exactly what spaceweave does.
2. **Almost nobody exposes geometry.** Only Officely Enterprise (`floor-manifest/1`), Seatsurfing (box, rotation and scale) and Archibus (SVG) give a usable spatial model through an API. Envoy exposes desk x/y only, normalised and create-only. A geometry-first, standards-friendly export (IMDF/GeoJSON-like) would be distinctive. Only Archibus touches IFC, and no product documents IMDF.
3. **Most vendor APIs can't take a layout pushed in.** Skedda is read-only, deskbird's layout is read-only, Robin has no floor/space endpoints, and Envoy desk positions are create-only. Officely Enterprise, which publishes floor-plan documents, and Archie, through Area PUT with `map`/`origin`, are the realistic push targets. Elsewhere spaceweave output is a replacement for the vendor's tracing service, or CSV import (for example Envoy's desk CSV with X/Y).
4. **Separate structural space from bookable resource.** Officely's Space/Resource split, and the series-plus-occurrence model used by Skedda, Robin and LibreBooking, are the patterns to copy.
5. **Integration features are now standard:** SAML SSO, SCIM for Entra ID and Okta, two-way M365/Google room sync, signed webhooks and, increasingly, MCP servers (deskbird, Robin, Officely, Envoy, Archibus, Seatsurfing). spaceweave's API should treat Standard Webhooks signing, OpenAPI 3.1 published alongside the docs, and an MCP surface as the baseline.

## Caveats

- Everything here is drawn from public sources as of 2026-11-25. Logged-in developer portals were not accessed: Clearooms, Eptura Engage APIM, the Eptura Public API. Robin's trust centre renders only with JavaScript and could not be read.
- Some sites block automated fetches. Validators used the vendors' own public content APIs instead: Zendesk JSON, MindTouch and Intercom `.md` endpoints. Citations still point at the readable article URLs.
- Prices change often. Re-check them before relying on them.
