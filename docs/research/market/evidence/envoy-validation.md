# Envoy validation

Validator: independent re-fetch on 2026-11-25. All 68 cited URLs resolved (HTTP 200). Developer-hub pages were read from their `.md` source. GitHub claims were checked with `gh api`. Quotes were string-matched against each page, and every miss was checked by hand (all misses were markdown-link or whitespace artifacts). The OpenAPI fragments were parsed to check the schemas, fields and enums.

## Summary

| ACCEPT | REVISE | REJECT | NEW |
|---|---|---|---|
| 125 | 2 (C55, C111) | 0 | 6 (N1–N6) |

Flags: 7 STALE (C50, C78, C79, C80, C106, C110, C111). No THIRD-PARTY-ONLY flags: every claim cites an official Envoy or GitHub source.

### The three flagged conflicts

| Conflict | Verdict | Authoritative source |
|---|---|---|
| Token endpoint: C5 (`api.envoy.com/oauth2/token`) vs C55 (`app.envoy.com/a/auth/v0/token`, "private apps use client_credentials") | **C5 wins.** The llms.txt URL is the legacy v3 Visitor API's `tokenUrl` (OpenAPI on retrieve-devices). Both URLs return an identical 401 `{"error":"authorization_required"}` to an unauthenticated POST, so they probably share one OAuth server. llms.txt is wrong about the grant: the dated guide and the app-type table say private apps use the password grant or `X-API-Key`. | getting-an-access-and-refresh (2025-06-05), overview-building-apps |
| Reservation window: 30 days (C22) vs 31 days (llms.txt) | **30 days.** The OpenAPI operation description (updatedAt 2026-08-03) says 30. llms.txt is undated and hand-edited (it carries an `envoy-llms-txt-custom-v1` marker comment). Note that the default query window is ±14 days. | reference/reservations-1 |
| Kiosk languages: 30+ (C111) vs 40+ (C112) | **40+.** The dedicated languages article was updated 2026-07-23 and lists 41 languages; the accessibility article dates from 2024-10-22. | eu.envoy.help 9944722 |

General point: the hub's llms.txt is a hand-maintained file and contradicts the dated reference pages in two places. Treat it as lower-authority than the reference and guide pages.

### Notes-specific checks
- **Space type enum:** confirmed as `DESK` and `ROOM` only. It appears in `components.schemas.SpaceType`, in the `type` filter on /v1/spaces and /v1/reservations, and in `CreateReservationInput.spaceType`, which defaults to DESK.
- **Desk xPos/yPos:** the 0–1 normalisation is stated only on `CreateDeskInput`. The GET Desk schema says only "x/y coordinate ... on the Map" (Float). **`UpdateDeskInput` has no xPos/yPos** (N2), so the API cannot reposition a desk after it is created.
- **MCP server (envoy/mcp-plugin):** verified with gh api. The repo was created 2026-09-24T17:35:20Z, is MIT-licensed, and contains README, plugin.json and mcp.json. Its transport is streamable-http to `https://mcp.envoy.com/mcp`. The server is live: it publishes OAuth authorization-server metadata (authorization_code with PKCE S256, refresh_token, dynamic client registration) and returns 401 to an unauthenticated POST (N3). No tool list is published. The SPA bundle at mcp.envoy.com contains identifiers such as `reserve_desk`, `assign_desks`, `invite_visitor`, `list_locations` and `room_calendar`, but this is not documentation and is not given as a claim.

## Claim table

| id | verdict | flags | corrected text (for REVISE) | note |
|---|---|---|---|---|
| C1 | ACCEPT |  |  |  |
| C2 | ACCEPT |  |  |  |
| C3 | ACCEPT |  |  |  |
| C4 | ACCEPT |  |  |  |
| C5 | ACCEPT |  |  | AUTHORITATIVE over C55 for v1. Dated guide (2025-06-05) with curl example; token response shows expires_in 86400 and refresh_token_expires_in 2592000 (30 d). Live probe: POST api.envoy.com/oauth2/token -> 401 {"error":"authorization_required"} (endpoint exists). |
| C6 | ACCEPT |  |  | Also consistent with the app-type table on overview-building-apps (Listed = client credentials, Unlisted = authorization code, Private = password grant). Contradicts llms.txt (C55). |
| C7 | ACCEPT |  |  |  |
| C8 | ACCEPT |  |  |  |
| C9 | ACCEPT |  |  |  |
| C10 | ACCEPT |  |  |  |
| C11 | ACCEPT |  |  | Page updatedAt 2025-10-02. |
| C12 | ACCEPT |  |  |  |
| C13 | ACCEPT |  |  |  |
| C14 | ACCEPT |  |  |  |
| C15 | ACCEPT |  |  |  |
| C16 | ACCEPT |  |  |  |
| C17 | ACCEPT |  |  |  |
| C18 | ACCEPT |  |  | Verified in OpenAPI: parameter `type` enum ["DESK","ROOM"]; components.schemas.SpaceType = string, anyOf enum ["DESK","ROOM"]; Space.type refs SpaceType. Also the reservations list `type` filter and CreateReservationInput.spaceType ("DESK" or "ROOM", default DESK). Enum is DESK and ROOM only. |
| C19 | ACCEPT |  |  |  |
| C20 | ACCEPT |  |  |  |
| C21 | ACCEPT |  |  |  |
| C22 | ACCEPT |  |  | AUTHORITATIVE over llms.txt "31 days". OpenAPI operation description (updatedAt 2026-08-03) says 30 days; startTime/endTime default to 14 days before/after now. llms.txt is undated and hand-edited (custom marker comment). |
| C23 | ACCEPT |  |  |  |
| C24 | ACCEPT |  |  |  |
| C25 | ACCEPT |  |  |  |
| C26 | ACCEPT |  |  |  |
| C27 | ACCEPT |  |  |  |
| C28 | ACCEPT |  |  |  |
| C29 | ACCEPT |  |  |  |
| C30 | ACCEPT |  |  |  |
| C31 | ACCEPT |  |  | GET /v1/desks Desk schema describes xPos/yPos only as "x/y coordinate ... on the Map" (type Float); the 0-1 normalisation is stated only on CreateDeskInput (C32). |
| C32 | ACCEPT |  |  | Verified: CreateDeskInput {locationId, floorId, name, neighborhoodId, assignedTo, enabled, xPos, yPos}, required [locationId, floorId, name]; xPos/yPos "normalized ... decimal value between 0 and 1". The docs say "on the Map", not explicitly "relative to the floor image". |
| C33 | ACCEPT |  |  | But UpdateDeskInput is only {name, neighborhoodId, assignedTo, enabled}: no xPos/yPos, so a desk cannot be repositioned by API after creation (see N2). |
| C34 | ACCEPT |  |  |  |
| C35 | ACCEPT |  |  |  |
| C36 | ACCEPT |  |  |  |
| C37 | ACCEPT |  |  |  |
| C38 | ACCEPT |  |  |  |
| C39 | ACCEPT |  |  |  |
| C40 | ACCEPT |  |  |  |
| C41 | ACCEPT |  |  |  |
| C42 | ACCEPT |  |  |  |
| C43 | ACCEPT |  |  |  |
| C44 | ACCEPT |  |  |  |
| C45 | ACCEPT |  |  |  |
| C46 | ACCEPT |  |  |  |
| C47 | ACCEPT |  |  |  |
| C48 | ACCEPT |  |  |  |
| C49 | ACCEPT |  |  |  |
| C50 | ACCEPT | STALE |  | Beta feature; page updatedAt 2025-06-05. |
| C51 | ACCEPT |  |  | llms.txt also calls the Node.js SDK "the officially supported client". |
| C52 | ACCEPT |  |  | package.json on main: name @envoy/envoy-integrations-sdk, version 2.5.8; pushed_at 2026-11-23T23:44:32Z (gh api). |
| C53 | ACCEPT |  |  | gh api: repo created 2026-09-24T17:35:20Z, last push same day, MIT, description "Agent Plugins package for the Envoy remote MCP server". README: Cursor plugin in Agent Plugins 1.0.0 format; mcp.json transport is streamable-http. Live: mcp.envoy.com/.well-known/oauth-authorization-server is served (authorization_code + refresh_token, PKCE S256, dynamic client registration at /register, scopes openid/offline_access/mcp); POST /mcp unauthenticated -> 401. See N3. |
| C54 | ACCEPT |  |  | plugin.json v1.0.0 description verbatim. No tool list is published in the repo. |
| C55 | REVISE |  | The hub's llms.txt (undated, hand-maintained) says tokens come from `POST https://app.envoy.com/a/auth/v0/token` and that private apps use `client_credentials`. That URL is the tokenUrl of the legacy Visitor API (v3) OpenAPI (retrieve-devices page). It is not the documented v1 endpoint, and the grant statement contradicts the dated guides [C5][C6] and the app-type table [C8]. For v1, treat `https://api.envoy.com/oauth2/token` as authoritative. | Both URLs answer an unauthenticated POST with an identical 401 {"error":"authorization_required"}, so they probably share one OAuth server. The real conflict is the grant type, and llms.txt is wrong there. The v1 OpenAPI securitySchemes give tokenUrl "/token", but api.envoy.com/token returns a GraphQL-style UNAUTHENTICATED error, not an OAuth error, so that relative URL is a spec artifact. |
| C56 | ACCEPT |  |  | A statement about llms.txt; its content is lower-authority (see C55). |
| C57 | ACCEPT |  |  |  |
| C58 | ACCEPT |  |  |  |
| C59 | ACCEPT |  |  | Its OpenAPI securitySchemes also give tokenUrl https://app.envoy.com/a/auth/v0/token, which explains C55. |
| C60 | ACCEPT |  |  |  |
| C61 | ACCEPT |  |  |  |
| C62 | ACCEPT |  |  |  |
| C63 | ACCEPT |  |  |  |
| C64 | ACCEPT |  |  |  |
| C65 | ACCEPT |  |  |  |
| C66 | ACCEPT |  |  |  |
| C67 | ACCEPT |  |  | Maps article updated 2026-11-23. |
| C68 | ACCEPT |  |  |  |
| C69 | ACCEPT |  |  |  |
| C70 | ACCEPT |  |  |  |
| C71 | ACCEPT |  |  |  |
| C72 | ACCEPT |  |  |  |
| C73 | ACCEPT |  |  |  |
| C74 | ACCEPT |  |  |  |
| C75 | ACCEPT |  |  |  |
| C76 | ACCEPT |  |  |  |
| C77 | ACCEPT |  |  |  |
| C78 | ACCEPT | STALE |  | Space-saver article dateModified 2025-08-01. |
| C79 | ACCEPT | STALE |  | dateModified 2025-08-01. |
| C80 | ACCEPT | STALE |  | dateModified 2025-08-01. |
| C81 | ACCEPT |  |  |  |
| C82 | ACCEPT |  |  |  |
| C83 | ACCEPT |  |  |  |
| C84 | ACCEPT |  |  |  |
| C85 | ACCEPT |  |  |  |
| C86 | ACCEPT |  |  |  |
| C87 | ACCEPT |  |  |  |
| C88 | ACCEPT |  |  |  |
| C89 | ACCEPT |  |  |  |
| C90 | ACCEPT |  |  |  |
| C91 | ACCEPT |  |  |  |
| C92 | ACCEPT |  |  |  |
| C93 | ACCEPT |  |  |  |
| C94 | ACCEPT |  |  |  |
| C95 | ACCEPT |  |  |  |
| C96 | ACCEPT |  |  |  |
| C97 | ACCEPT |  |  |  |
| C98 | ACCEPT |  |  |  |
| C99 | ACCEPT |  |  |  |
| C100 | ACCEPT |  |  | $362 and Free are in the visible table and the JSON-LD; Basic is "Up to 100 entries/month"; Reservations, Screens and Protect prices "exclude platform fee". |
| C101 | ACCEPT |  |  | The visible table columns are Basic, Premium, Enterprise and Critical: SAML "No Yes Yes Yes", SCIM "No No Limited Limited". |
| C102 | ACCEPT |  |  |  |
| C103 | ACCEPT |  |  |  |
| C104 | ACCEPT |  |  | The page also lists CCPA and CPRA (so the draft's addition is supported). |
| C105 | ACCEPT |  |  |  |
| C106 | ACCEPT | STALE |  | dateModified 2024-11-08. |
| C107 | ACCEPT |  |  | Footnotes: * "At the election of the customer" (Anthropic, Merge.dev); ** Stream.io "For emergency notification two-way communications only". The draft calls Stream.io "optional", which is wrong. |
| C108 | ACCEPT |  |  |  |
| C109 | ACCEPT |  |  |  |
| C110 | ACCEPT | STALE |  | dateModified 2024-10-22. |
| C111 | REVISE | STALE | Kiosk supports 40+ languages (41 listed) per the dedicated language article [C112, updated 2026-07-23]. The accessibility article (updated 2024-10-22) says "30+" and is outdated. The mobile app's 7 languages are sourced only from that 2024 article. | RESOLVED: C112 is authoritative (newer, dedicated, lists each language). |
| C112 | ACCEPT |  |  | AUTHORITATIVE. dateModified 2026-07-23; lists 41 languages. |
| C113 | ACCEPT |  |  | Components also include Auto Check-in via Access Control and via Wifi Integrations. |
| C114 | ACCEPT |  |  | Corroborated by the official help article C70. |
| C115 | ACCEPT |  |  |  |
| C116 | ACCEPT |  |  | In the JSON-LD Organization address; Denver and London offices are also listed. |
| C117 | ACCEPT |  |  | Verified with gh api orgs/envoy/repos (the org HTML page renders the list client-side): envoy-platform-sdk-nodejs (created 2016, pushed 2025-10-03, not archived); scim-kit archived, "SCIM 2.0 SDK for Ruby." |
| C118 | ACCEPT |  |  |  |
| C119 | ACCEPT |  |  |  |
| C120 | ACCEPT |  |  |  |
| C121 | ACCEPT |  |  |  |
| C122 | ACCEPT |  |  |  |
| C123 | ACCEPT |  |  |  |
| C124 | ACCEPT |  |  |  |
| C125 | ACCEPT |  |  |  |
| C126 | ACCEPT |  |  |  |
| C127 | ACCEPT |  |  |  |

## Unsourced or mis-cited statements in the draft (verdict: REJECT as written)

1. **§17 "there is no CAD/PDF import or auto-detection"** and **§4 "Auto-detection ... not documented as current".** An Envoy blog post dated 2026-09-02 says "Envoy Workplace uses computer vision AI to detect and place multiple seats" (N1). Its linked help article (8828469-automatic-map-placement) now returns 404, and the current maps article (updated 2026-11-23) does not mention the feature. Rewrite to: "AI desk auto-placement from the floor image was marketed (blog, Sep 2026); its help article is now 404, so current availability is unclear." The CAD/PDF part still stands.
2. **§8 "Retention is not publicly documented".** The security page has a "Data retention" section (N4), and the pricing table lists "Data retention policy" (Premium) and "Configurable field-level data retention policy" (Enterprise). Rewrite.
3. **§8 "Anthropic, Merge.dev and Stream.io are optional".** Stream.io is "for emergency notification two-way communications only". Only Anthropic and Merge.dev are "at the election of the customer" [C107].
4. **§10 "Entries, Invites, RecurringInvites, Blocklists, Flows, Deliveries [C39]".** C39 supports Deliveries only. The others are named in llms.txt (the C56 URL) and in the events page [C120]. Re-cite.
5. **§10 "WorkSchedules (list, create, check in, check out, delete) [C36][C37]".** Check-out and delete were not in the ledger. They are now supported by N5.
6. **§14 "the quickstart takes about 10 minutes [C5]".** The C5 page does not say this. It comes from make-your-first-api-request ("Time to complete: 10 minutes"). Re-cite (N6). The same page backs the "/rest/v1 vs /api/v1" inconsistency in §14 (verified).
7. **§3 "Mobile apps: iOS and Android".** No ledger claim covers this.
8. **§4 "bulk select".** No ledger claim covers this, and the maps article does not mention it. "Undo and redo" is supported by the maps article but is not in the ledger.
9. **§13 "Third parties (Truto, StackOne, Pipedream, Zapier, Boomi) offer connectors and MCP servers".** This is self-declared as search-only and unverified. Drop it or source it. (Pipedream alone is covered by C95.)
10. **§15 "envoy.com/products/ai-for-envoy now redirects to mcp.envoy.com".** Not in the ledger, but verified: 301 to https://mcp.envoy.com/. Add as a claim.
11. **§1 Segment and §14 "GraphQL backend".** Both are labelled as inference, which is acceptable. Supporting evidence for the GraphQL inference: the OpenAPI scalar schemas are named ID, String, Boolean, Float and DateTime, and `POST api.envoy.com/token` returns `{"errors":[{"message":"No authorization token was found","extensions":{"code":"UNAUTHENTICATED"}}]}`.
12. Minor: §4 and §17 say xPos/yPos are normalised "relative to the floor map image". The docs say "on the Map". This is a reasonable reading but is not stated verbatim.

The draft says the "Errors"/"Status Codes" titles are swapped. This was verified: errors-1 is titled "# Status Codes" and status-codes is titled "# Errors".

## Data-model trace issues (§12)

- Location, Space, Desk, Reservation, ReservationInput, WorkSchedule, Employee and Delivery: every field listed matches the parsed OpenAPI schemas. Location.address has latitude and longitude; WorkSchedule.scheduledFor is UserView {id, type, name, email}; Delivery references DeliveryArea and Carrier schemas. OK.
- **Floor → Location** is marked INFERRED. It has no direct claim; Space and Desk require locationId and floorId together. Acceptable as labelled.
- **Location → Company:** supported only by the `companyId` field [C35]. OK.
- **Desk:** add that position is create-only via the API (N2). Desk input also has `enabled` [C32]. OK.
- **ReservationInput:** required fields are [locationId, spaceType, userEmail, startTime]. The draft does not mark which are required.
- No Floor, Neighborhood or Room endpoints exist: the reference slugs floors-1, floors, neighborhoods, rooms and rooms-1 all return 404, so the draft's statement holds.

## New claims

| id | claim | url | quote | tag | page date | verdict |
|---|---|---|---|---|---|---|
| N1 | Envoy marketed AI auto-placement of desks from the map image (its help article now 404s) | https://envoy.com/space-management/space-planning-envoy-workplace-maps | "Envoy admins can now add multiple desks with a simple drag and drop. Envoy Workplace uses computer vision AI to detect and place multiple seats in just a few clicks." | blog | 2026-09-02 | ACCEPT |
| N2 | The desk update input has no position fields, so desks cannot be moved by API | https://developers.envoy.com/hub/reference/updatedesk | UpdateDeskInput properties: "name", "neighborhoodId", "assignedTo", "enabled" (no xPos/yPos) | api-ref | 2026-08-03 | ACCEPT |
| N3 | The Envoy MCP server uses OAuth with PKCE and dynamic client registration, over streamable HTTP | https://github.com/envoy/mcp-plugin/blob/main/mcp.json (and https://mcp.envoy.com/.well-known/oauth-authorization-server) | "\"type\": \"streamable-http\", \"url\": \"https://mcp.envoy.com/mcp\"" … "\"code_challenge_methods_supported\":[\"S256\"]" … "\"registration_endpoint\":\"https://mcp.envoy.com/register\"" | github | repo created 2026-09-24 | ACCEPT |
| N4 | Retention of sign-in data is configurable; Premium and Enterprise can save or purge responses; purged data is deleted within 24 h | https://envoy.com/security | "Companies on Premium and Enterprise plans can choose to save or purge the responses to both their visitor sign-in and employee health check." … "all data, except for the screening result (approved or denied) is deleted within 24 hours" | official-docs | none shown | ACCEPT |
| N5 | WorkSchedules can be checked out and deleted through the API | https://developers.envoy.com/hub/reference/checkoutworkschedule and https://developers.envoy.com/hub/reference/deleteworkschedule | "# Check out a WorkSchedule" … "/v1/work-schedules/{id}/checkout" ; "# Delete a WorkSchedule" … "/v1/work-schedules/{id}" | api-ref | — | ACCEPT |
| N6 | The quickstart is billed as 10 minutes and mixes /api/v1 and /rest/v1 for locations | https://developers.envoy.com/hub/docs/make-your-first-api-request | "Time to complete: 10 minutes" … "call the locations API `/api/v1/locations`" … "GET 'https://api.envoy.com/rest/v1/locations'" | official-docs | — | ACCEPT |

Searches that found nothing to overturn "not publicly documented": room capacity, lockers or equipment as bookable kinds, CAD/DWG/PDF map import, and a developer sandbox (2–3 queries each).
