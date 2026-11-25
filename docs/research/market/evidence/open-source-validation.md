# Validation: open-source (self-hostable booking projects)

Validated 2026-11-25. Every pinned-commit source file was downloaded with `gh api repos/.../contents/PATH?ref=SHA` and searched, and repo metadata was re-read with `gh api`. Docs pages were fetched with WebFetch or curl. HEAD SHAs still match the ledger (607c9c7, fce306f, 64bbcb8, cb0fad5, 9099f4f).

## Summary counts

| Verdict | Count |
|---|---|
| ACCEPT | 72 (C1–C72) |
| REVISE | 0 |
| REJECT (ledger claims) | 0 |
| UNSOURCED draft statements (verdict REJECT by rule) | 14; only 1 is factually wrong (U3) |
| NEW claims | 6 (N1–N6) |

Flags: none of the claims are THIRD-PARTY-ONLY (all sources are repos or official docs). None are STALE (all docs pages are dated Aug–Oct 2026).

## Claim table

| id | verdict | flags | corrected text (REVISE) | note |
|---|---|---|---|---|
| C1 | ACCEPT | | | API: 320, GPL-3.0, Go, pushed 2026-11-25T06:22:50Z |
| C2 | ACCEPT | | | README L27, L31-33 |
| C3 | ACCEPT | | | README L13-18 |
| C4 | ACCEPT | | | README L52, L62, L77 |
| C5 | ACCEPT | | | kubectl apply only; Helm not mentioned; updated Oct 8, 2026 |
| C6 | ACCEPT | | | Verbatim |
| C7 | ACCEPT | | | Verbatim |
| C8 | ACCEPT | | | Verbatim |
| C9 | ACCEPT | | | Verbatim |
| C10 | ACCEPT | | | Verbatim. The page has no SAML or LDAP |
| C11 | ACCEPT | | | Verbatim |
| C12 | ACCEPT | | | Verbatim |
| C13 | ACCEPT | | | |
| C14 | ACCEPT | | | openapi.yaml L1-3 |
| C15 | ACCEPT | | | The schemes are at about L124-135, not L122-132. Cosmetic |
| C16 | ACCEPT | | | Page supports it. The draft misreports it as "per-user API tokens" (see U3) |
| C17 | ACCEPT | | | L105 verbatim |
| C18 | ACCEPT | | | auth-provider-repository.go L19-31 |
| C19 | ACCEPT | | | Nav links to Google and Keycloak; fields present |
| C20 | ACCEPT | | | Exchange, SCIM and Teams quotes match. "SCIM 2.0" is confirmed on the page (see N6) |
| C21 | ACCEPT | | | Whole-instance licence, tiers 25–1000, ≤10 users free, valid one year |
| C22 | ACCEPT | | | Verbatim. The page also says "The Plus Plugin is required!" for self-hosted |
| C23 | ACCEPT | | | location-repository.go L36-44 |
| C24 | ACCEPT | | | L61, L67, L79, L92 |
| C25 | ACCEPT | | | location-floor-plan-repository.go. Also confirms PK/FK to locations |
| C26 | ACCEPT | | | L439-442 verbatim |
| C27 | ACCEPT | | | L2661-2691. GetMapResponse has width/height/scale (L503-511) |
| C28 | ACCEPT | | | No type or kind column in spaces |
| C29 | ACCEPT | | | All migrations present |
| C30 | ACCEPT | | | L753-766 verbatim |
| C31 | ACCEPT | | | |
| C32 | ACCEPT | | | README L22-23 |
| C33 | ACCEPT | | | API matches |
| C34 | ACCEPT | | | README L88, L96 |
| C35 | ACCEPT | | | README L102-110 |
| C36 | ACCEPT | | | README L140-158 |
| C37 | ACCEPT | | | .examples contains docker, kubernetes, podman |
| C38 | ACCEPT | | | README L5-12 |
| C39 | ACCEPT | | | API.rst L5, L40-42. "Disabled by default" is backed by the README (`api.enabled` set to `false`, the default) |
| C40 | ACCEPT | | | Section headings match |
| C41 | ACCEPT | | | Directory listing matches |
| C42 | ACCEPT | | | Oauth2-Configuration.rst L5-16 |
| C43 | ACCEPT | | | create-schema.sql L184-210 |
| C44 | ACCEPT | | | 2.5/schema.sql. The description column is `resource_type_description` |
| C45 | ACCEPT | | | 2.6/schema.sql L104-116 |
| C46 | ACCEPT | | | |
| C47 | ACCEPT | | | API: TypeScript primary; languages include Python 377062 |
| C48 | ACCEPT | | | README L12 (cited as L13; off by one) |
| C49 | ACCEPT | | | README L28 |
| C50 | ACCEPT | | | README L29 |
| C51 | ACCEPT | | | README L38-41 |
| C52 | ACCEPT | | | README L50 |
| C53 | ACCEPT | | | FEATURES L207-208, L758 |
| C54 | ACCEPT | | | schema.sql plan/seat/book |
| C55 | ACCEPT | | | FEATURES L125-126; schema `zone_type integer DEFAULT 10` |
| C56 | ACCEPT | | | FEATURES L756 |
| C57 | ACCEPT | | | README L75, L98 |
| C58 | ACCEPT | | | Commit cb0fad5 is dated 2026-11-23. Repo `pushed_at` is now 2026-11-24T21:18:03Z (a push to another branch), so "last commit" is still correct for HEAD |
| C59 | ACCEPT | | | LICENSE L1 |
| C60 | ACCEPT | | | README.md L4-5, L25 |
| C61 | ACCEPT | | | tables.pg.sql L84-101 |
| C62 | ACCEPT | | | |
| C63 | ACCEPT | | | AUTHENTICATION L127, L196 |
| C64 | ACCEPT | | | Dockerfile L2 |
| C65 | ACCEPT | | | API matches |
| C66 | ACCEPT | | | README.adoc L3, L10, L38 |
| C67 | ACCEPT | | | README.adoc L45, L57 |
| C68 | ACCEPT | | | OpenApi.hs L15-16 |
| C69 | ACCEPT | | | Schema.hs DbDesk, unique (space, name) |
| C70 | ACCEPT | | | Schema.hs DbSpace |
| C71 | ACCEPT | | | Schema.hs DbReservation and TODO comment |
| C72 | ACCEPT | | | 226/AGPL-3.0, 46 (pushed 2026-06-23), 32. Descriptions match |

## Unsourced statements in the draft (verdict REJECT under the brief's rule; most are true and only need a ledger row)

| # | Draft location | Statement | Finding |
|---|---|---|---|
| U1 | L7, L17 | Search methodology, and "Search found no dedicated open-source parking-booking project of note" | Not reproducible from the ledger. It is a negative search result with no cited evidence. Keep it as an author statement or drop it |
| U2 | L23 | "Seatsurfing is an open-core project" | A characterisation. It is implied by C3 and C20 but not cited. Low severity |
| **U3** | **L35** | "API auth uses … **per-user API tokens** [C15][C16]" | **Factually wrong.** API tokens are for **service accounts**: openapi.yaml L40 and L135 ("Service accounts may also use … a long-lived API token … generated via `POST /user/{id}/api-token`"), and C16 itself. Fix it to read "service-account API tokens" (see N3) |
| U4 | L35 | "about 99 paths: `/location`, …" | True: 99 top-level path keys counted. It has no ledger row; add N2 |
| U5 | L58 | LibreBooking has no floor plans and books on a schedule grid ("Schedules" screenshots) | True. The tree has no floor or map code, and README L128 shows the Schedules screenshot. No ledger row |
| U6 | L62 | LibreBooking repo has no OpenAPI or Swagger file | True (tree searched). No ledger row |
| U7 | L74, L90, L138 | No Helm chart for LibreBooking, WARP, MRBS or Mensam | True for the main repo trees (no "helm" or "chart" paths). The LibreBooking/docker repo was not tree-searched. No ledger rows |
| U8 | L78 | WARP was "built during COVID" | True (WARP README L7), but it cites C48 and C52, which do not say this. Add N4 |
| U9 | L86 | WARP has no OpenAPI spec in the tree | True. No ledger row |
| U10 | L94 | MRBS is "long-running" | Unsourced adjective. Low severity |
| U11 | L100 | MRBS has no OIDC in `web/lib/MRBS/Auth/*` | True. The class list is AuthBasic, Cas, Config, Crypt, Db, DbExt, Ext, Idcheck, Imap, ImapPhp, Joomla, Ldap, None, Nw, Pop3, Saml, Wix, Wordpress. Caveat: SimpleSAMLphp can bridge to OIDC via modules, so "no native OIDC" is the precise wording |
| U12 | L102 | MRBS has no REST API; README.sqlapi is a DB abstraction | The README.sqlapi file exists; its content was not checked here. No ledger row |
| U13 | L116, L122 | Mensam auth paths (`Servant/Auth/JWT`, `Auth/Basic.elm`, `Auth/Bearer.elm`) and `final/nixos/module.nix` | True (paths present in tree @9099f4f). No ledger row |
| U14 | L143 | Seatsurfing has a `/location/{locationId}/space/bulk` endpoint, and spaces use an integer pixel box | True (openapi.yaml L3079; spaces x/y/width/height INTEGER). Add it to N2 |

## Data-model trace issues

Items are listed where the draft's data-model tables name an entity, field or relation that no accepted claim's text or quote covers. The source file supports all of them unless stated otherwise.

**Seatsurfing**
- `locations.description` and `locations.enabled` are not in C23 or C24. They are in location-repository.go L50-73.
- In "space attributes", the "values per space/location" and "attribute 1–n values" relation is not covered: C30 only covers the attribute definition. The source has a `space_attribute_values` table with `entity_type` (space-repository.go L327).
- In bookings, the "recurring_bookings 1–n bookings" relation has no claim for a `recurring_bookings` table. C31 covers only the `recurring_id` column.
- `auth_providers.provider_type` and the "org 1–n" relation (`organization_id`) are in source L21-22 but not in C18's quote.
- `location_floor_plans` "(PK, FK)" is not in C25's quote. Source: `location_id uuid PRIMARY KEY REFERENCES locations(id)`.

**LibreBooking**
- `resource_types.description` is wrong: the column is `resource_type_description`.
- `resource_group_assignment` (n–n) is at 2.5/schema.sql L29-35, outside C44's cited L3-27.
- `reservation_resources` (n–n series↔resources) is not covered by any claim.

**MRBS**
- The `mrbs_area` fields (timezone, booking window, slot settings) have no claim. The table is at tables.pg.sql L20 (timezone, resolution, default_duration…).
- `mrbs_repeat` (L105) is outside C62's L138-180 range.

**WARP**
- `zone(id, name, zone_type, zone_group)` is at schema.sql L54-59, outside C54's L71-136 range.
- `seat_assign(sid, login, days_in_advance)` and `plan.dark_filter` are in range but not in C54's claim text.

**Mensam**: no issues.

## "Not publicly documented" checks (2-3 queries each)

- **Seatsurfing check-in:** the features page and admin and booking UI docs do not mention it, and web search found nothing. The repo has `specs/kiosk-mode.md`, which defines kiosk mode as a read-only occupancy display with no booking actions. That supports the statement (N1).
- **Seatsurfing SAML/LDAP:** not in the OpenAPI spec, the repo tree, the features page or the auth-provider docs. Web search found nothing. The statement stands.
- **Seatsurfing CAD/DWG/PDF import:** no dxf or dwg in the tree, and web search found nothing. The statement stands.
- **WARP CAD import and API docs:** no CAD, OpenAPI or Swagger paths in the tree. The statement stands.
- **MRBS floor plans:** no floor, OpenAPI or `/api/` paths in the tree. The statement stands.
- **Mensam import pipeline and OIDC/SAML/LDAP:** none in the tree. The statement stands.
- **SCIM for LibreBooking, WARP, MRBS and Mensam:** zero "scim" paths in any tree. The statement stands.
- **IMDF/IndoorGML/IFC:** none in any tree.

No statement was disproved.

## New claims

| id | verdict | claim | url | quote |
|---|---|---|---|---|
| N1 | ACCEPT | Seatsurfing's kiosk mode is a read-only, per-space occupancy display for wall tablets and e-ink screens. It has no booking actions and no QR codes, so it is not a check-in feature | https://github.com/seatsurfing/seatsurfing/blob/607c9c7dd91ed288723c83d3dfeaadb0b515d179/specs/kiosk-mode.md | "Add a per-space kiosk mode that exposes a lightweight, read-only web page for a single space." … "No booking creation, update, approval, or deletion from the kiosk page." … "No QR code generation requirement in the first version." |
| N2 | ACCEPT | The Seatsurfing OpenAPI spec has 99 paths, including `/location/{id}/map` and `/location/{locationId}/space/bulk` | https://github.com/seatsurfing/seatsurfing/blob/607c9c7dd91ed288723c83d3dfeaadb0b515d179/specs/openapi.yaml#L3079 | "/location/{locationId}/space/bulk:" (99 top-level path keys counted) |
| N3 | ACCEPT | Seatsurfing's long-lived API tokens belong to service accounts, are generated via `POST /user/{id}/api-token` and are stored as SHA-256 hashes | https://github.com/seatsurfing/seatsurfing/blob/607c9c7dd91ed288723c83d3dfeaadb0b515d179/specs/openapi.yaml#L40 | "Service accounts may also use HTTP Basic Authentication … or a long-lived API token (`Authorization: Bearer <token>`) generated via `POST /user/{id}/api-token`. API tokens are stored as SHA-256 hashes" |
| N4 | ACCEPT | WARP was created when an office moved to hybrid hot-desking because of COVID-19 | https://github.com/sebo-b/warp/blob/64bbcb8771b367b012ca218753c183b932ab45ee/README.md | "The story of this project begins when, due to COVID-19, we have converted our regular office into a hybrid of regular and hot-desk assignments." |
| N5 | ACCEPT | LibreBooking's API is off by default (`api.enabled` = false) | https://github.com/LibreBooking/librebooking/blob/fce306faa379137a8c8c9beb1f21fc5075fc3870/README.md | "Disabling the API (`api.enabled` set to `false`, the default) blocks the attack described in the advisory" |
| N6 | ACCEPT | Seatsurfing's SCIM is SCIM 2.0 and needs the Plus Plugin on self-hosted installs | https://seatsurfing.io/docs/scim | "SCIM 2.0. Setup guide and supported operations" … "The Plus Plugin is required!" |
