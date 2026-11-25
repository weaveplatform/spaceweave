# Validation: archie (archieapp.co)

Validator run 2026-11-25. Every cited URL was fetched again (curl + WebFetch). All 45 sources resolve (HTTP 200). The exception is SourceForge, which returns 403 to curl but loads through WebFetch. The pricing comparison table was parsed from the raw HTML, where each row's check, minus or label icon sits in a fixed column order: Starter | Pro | Enterprise. OpenAPI fragments were parsed as JSON.

## Summary counts
- ACCEPT: 98 (C1–C6, C8–C89, C91–C100)
- REVISE: 2 (C7, C90)
- REJECT: 0 ledger claims (but 9 draft statements are rejected as unsourced or wrong; see below)
- NEW: 4 (N1–N4)
- STALE-flagged: 26 (developer-portal guide pages last updated 2025-09-08, and help articles dated 2024 to Aug 2025)
- THIRD-PARTY-ONLY: 1 (C97)

## Claim table

| id | verdict | flags | corrected text (for REVISE) | note |
|---|---|---|---|---|
| C1 | ACCEPT | | | Privacy policy, Last Updated April 17, 2026. |
| C2 | ACCEPT | | | Homepage quote verified; four products listed. |
| C3 | ACCEPT | | | |
| C4 | ACCEPT | | | Desk booking Starter, $159/month, 1 location. |
| C5 | ACCEPT | | | |
| C6 | ACCEPT | | | Enterprise also has "Custom HRIS integration" and "Custom cloud hosting location" (both "Custom") in the comparison table. |
| C7 | REVISE | | Desk-booking comparison table: "API access + Webhooks" is **not included on Starter (minus icon), a paid Add-on on Pro, and "Custom" on Enterprise**. "Premium API support" is Enterprise-only (Custom). On the Coworking plans, "API access + Webhooks" is listed as an Enterprise key feature. | The column mapping *is* legible in the HTML (row cells: minus / "Add-on" / "Custom"). The draft's "likely paid add-on below Enterprise; confirm with sales" should state this as fact: Starter has no API access. |
| C8 | ACCEPT | | | FAQ text. |
| C9 | ACCEPT | | | |
| C10 | ACCEPT | | | |
| C11 | ACCEPT | | | |
| C12 | ACCEPT | | | |
| C13 | ACCEPT | | | Comparison table also shows Single Sign-On and SCIM are minus on Starter and check on Pro/Enterprise. |
| C14 | ACCEPT | | | Enum matches exactly (13 values). |
| C15 | ACCEPT | | | Path `/spaces/{spaceDomain}/conferenceRooms/...`. |
| C16 | ACCEPT | | | Fields present. **But** `map_meta` is typed `object<string,string>` and `origin` is `object<string,number>` (additionalProperties typed). So the draft's "untyped" is wrong; see draft issues. |
| C17 | ACCEPT | | | Also has `allow_hourly_bookings`, `allow_daily_bookings`, `cancellable`, `cancellation_delay`. |
| C18 | ACCEPT | | | Policy also carries `entity_uuid`/`entity_name`. |
| C19 | ACCEPT | | | The llms.txt quote only shows the endpoints. The field list (`prices`, `available_times`, hourly/half-day/daily price + billing_mode) is in `types.ConferenceRoomRevisionRest` on the Get Area page. Better URL: https://developers.archieapp.co/reference/get_spaces-spacedomain-conferencerooms-areauuidorslug.md |
| C20 | ACCEPT | | | |
| C21 | ACCEPT | | | |
| C22 | ACCEPT | | | Both live in `types.BookingConfigurationRest` (Space.booking_configuration). |
| C23 | ACCEPT | | | `spaces.BookingDisplay` enum = list, map, map-list, list-map; admin variants also exist. Fields are on `booking_configuration`, not directly on Space (the draft §2 says "Space has"). |
| C24 | ACCEPT | | | |
| C25 | ACCEPT | | | |
| C26 | ACCEPT | | | |
| C27 | ACCEPT | | | `bookings.ItemType` enum = `area, bookable, all` (relevant to data model). |
| C28 | ACCEPT | | | |
| C29 | ACCEPT | | | Frequency enum daily/weekly/monthly/yearly. |
| C30 | ACCEPT | | | Cancel, reschedule, end early, extend, approve, import/export CSV all present. |
| C31 | ACCEPT | | | ItemType enum is longer: area, conference-room, printing, bookable, buyable, day-pass, plan, day-access, segment, role, printing-account. |
| C32 | ACCEPT | | | |
| C33 | ACCEPT | | | |
| C34 | ACCEPT | | | API enum has no Tapkey even though marketing lists it. |
| C35 | ACCEPT | STALE | | getting-started updatedAt 2025-09-08. |
| C36 | ACCEPT | STALE | | |
| C37 | ACCEPT | | | Endpoint is `POST /authenticate` with JSON `{client_id, client_secret}`. The securityScheme is `http bearer`, not an `oauth2` type. The draft's "OAuth2 client-credentials token endpoint" is loose wording. |
| C38 | ACCEPT | | | |
| C39 | ACCEPT | | | |
| C40 | ACCEPT | STALE | | |
| C41 | ACCEPT | | | Also `openapi: 3.1.0`. |
| C42 | ACCEPT | STALE | | Doc inconsistency: the OpenAPI fragments model errors as `sdkerr.HTTPErrorBody{error{message,template_key,params,errors[]},message,request_uuid}`, not `message_code/type`. This is worth noting in §14 docs quality. |
| C43 | ACCEPT | | | `limit` default 40 on Get Bookings. |
| C44 | ACCEPT | | | Also stated for payments/purchases ("first 40 items"). |
| C45 | ACCEPT | STALE | | |
| C46 | ACCEPT | STALE | | webhooks.md updatedAt 2025-09-08. |
| C47 | ACCEPT | | | 21 events, 7 types × created/updated/deleted. |
| C48 | ACCEPT | STALE | | The "GSM" typo is verified. |
| C49 | ACCEPT | STALE | | |
| C50 | ACCEPT | STALE | | Recipe: "verify that it is maximum 2 minutes old". The 5 vs 2 min conflict is confirmed. |
| C51 | ACCEPT | STALE | | Configurable in the developer portal. |
| C52 | ACCEPT | STALE | | |
| C53 | ACCEPT | | | |
| C54 | ACCEPT | | | |
| C55 | ACCEPT | STALE | | Article dated February 4, 2025. |
| C56 | ACCEPT | | | Blog, December 16, 2025. |
| C57 | ACCEPT | STALE | | The quote is paraphrased; the page has "Hot desks" / "Dedicated desks" options and "private office". |
| C58 | ACCEPT | STALE | | |
| C59 | ACCEPT | STALE | | |
| C60 | ACCEPT | STALE | | |
| C61 | ACCEPT | | | |
| C62 | ACCEPT | | | |
| C63 | ACCEPT | | | |
| C64 | ACCEPT | STALE | | Nov 18, 2024. |
| C65 | ACCEPT | | | Aug 28, 2026. |
| C66 | ACCEPT | | | |
| C67 | ACCEPT | STALE | | Nov 12, 2024. |
| C68 | ACCEPT | | | |
| C69 | ACCEPT | | | |
| C70 | ACCEPT | STALE | | Aug 5, 2025. |
| C71 | ACCEPT | | | |
| C72 | ACCEPT | | | |
| C73 | ACCEPT | STALE | | Feb 25, 2025. |
| C74 | ACCEPT | | | "available exclusively with Enterprise contracts" verified. Service runs on Mac and Windows. |
| C75 | ACCEPT | | | |
| C76 | ACCEPT | STALE | | Nov 6, 2024. |
| C77 | ACCEPT | | | |
| C78 | ACCEPT | | | |
| C79 | ACCEPT | | | Ubiquiti/Aruba/Cisco Meraki appear as items next to IronWifi. Whether they are "via IronWifi" or separate Wi-Fi items is ambiguous; the draft only says IronWifi, which is fine. |
| C80 | ACCEPT | STALE | | Nov 22, 2024. |
| C81 | ACCEPT | | | |
| C82 | ACCEPT | STALE | | Nov 12, 2024. |
| C83 | ACCEPT | | | |
| C84 | ACCEPT | STALE | | Nov 13, 2024. |
| C85 | ACCEPT | | | |
| C86 | ACCEPT | STALE | | Feb 21, 2025. |
| C87 | ACCEPT | | | |
| C88 | ACCEPT | | | |
| C89 | ACCEPT | | | |
| C90 | REVISE | | Subprocessors: Google Cloud (CA), Google LLC/Google Workspace (CA), Google Analytics (US), Cloudflare (US), Intercom (US), HubSpot (US), Stripe (US), MongoDB (CA), OneSignal (US), PandaDoc (US), Twilio SendGrid (US); 10-business-day objection window. | The ledger omits Google Workspace and Google Analytics. |
| C91 | ACCEPT | | | |
| C92 | ACCEPT | | | |
| C93 | ACCEPT | | | Linked from the archieapp.co footer ("Status"), so it is official. |
| C94 | ACCEPT | | | Inconsistency: the bookings.created page's `constants.Language` has only 5 values (no tr-TR, he-IL). |
| C95 | ACCEPT | | | The full list also includes ironwifi, unifi, zoom, xero, quickbooks, stripe, hubspot, papercut, etc. |
| C96 | ACCEPT | | | The repo description is "Unofficial Archie API sdk with TypeScript" (user-owned, 0 stars, last push 2026-02-15). Both draft and ledger correctly label it community/unofficial. |
| C97 | ACCEPT | THIRD-PARTY-ONLY | | Via WebFetch: Archie = Arcanum AI (NZ); Archie AI = archielabs.com. Acceptable for name disambiguation only. |
| C98 | ACCEPT | | | |
| C99 | ACCEPT | | | |
| C100 | ACCEPT | STALE | | |

## Unsourced or incorrect statements in the draft (verdict REJECT unless noted)
1. §1 "The page shows no prices for Rooms, Visitors or Coworking." **Wrong.** The pricing page data has Room booking Starter $159/mo and Pro $249/mo; Visitors Starter $109 and Pro $185 per location/month; Coworking Starter $192/mo monthly or $165/mo yearly, and Pro $299/mo monthly or $257/mo yearly. See N1.
2. §1 "Treat API access as a likely paid add-on below Enterprise; confirm with sales." Replace with the C7 correction (Starter: none; Pro: add-on; Enterprise: custom).
3. Identity check: "An 'Archie CRM' iPad app also appeared in search results but was not fetched." Unsourced; drop it.
4. Identity check: the GitHub `archieApp` account (created 2026-03-30, 0 repos) has no ledger claim. Now verified as N2.
5. §4 "Their schemas are untyped in the published OpenAPI" (`map_meta`, `origin`). Wrong: `map_meta` is `object<string,string>` and `origin` is `object<string,number>`; only their keys are undocumented.
6. §4 "No endpoint to upload or edit a map was found" and §17 "the API has no map write endpoint, so that would need vendor cooperation". REVISE. There is no dedicated upload endpoint, but `PUT .../conferenceRooms/{areaUUIDOrSlug}` (Update Area) takes `types.ConferenceRoomRest` as its body, which includes `map`, `map_meta` and `origin`. So writing placement may be possible through the API (untested). See N4.
7. §5 "The help articles only describe pushing bookings to the calendar plus that optional inbound toggle." Interpretive and unsourced; soften it or cite C65/C67.
8. §7 "SaaS only. No on-prem offering was found." and §8 "No downloadable report or trust portal was found". These are absence claims without a ledger entry. Acceptable as "Not publicly documented", but not as stated facts.
9. §13 "zapier.com/apps/archie → 404" and "AppSource listings ... returned 403". Not in the ledger. Note that the Teams help article (C82 page) links an official AppSource listing: https://appsource.microsoft.com/en-us/product/office/wa200006189
10. §15 "G2's 'AI' category tags are third-party taxonomy". Unsourced.
11. §2 Car parking: "Space has `parking_booking_users_display`". Minor: the field is on `Space.booking_configuration` (BookingConfigurationRest).
12. §14 "Credentials are self-serve from the Developer Portal". This needs qualifying with C7: API access is not included on Starter.

## Data-model trace issues (§12)
- **User row**: cited [C70][C27], but neither ledger claim covers User fields. The fields do exist in `types.UserRest` (on the bookings.created page): uuid, email, external_id, status (`onboard|not-onboard`), type (`regular|visitor`). However, the **`groups` relation is not in UserRest**, so it is untraced. A new claim is needed for the User fields; drop or source the groups relation (e.g. through the Groups API).
- **Booking item → Area (INFERRED)**: `bookings.ItemType` = `area | bookable | all`, so the item is an Area *or* a bookable resource. It is not inferred; correct the relation to item_type-discriminated (see N3).
- **Area `children`**: the Area schema has no `children`/`parent` field. The hierarchy exists only through the children endpoints (C15). The relation is fine, but it should say "via endpoints".
- **Area `door_access_configurations`**: the field exists, but no ledger claim covers it (C34 covers only the provider enum). Minor.
- **Space key fields** `domain`, `slug`, `language`, `booking_configuration`, `integrations`: present in the SpaceRest schema, but only `integrations` (C95) and the C25 fields are in claims. Minor.
- **Presence log → User**: traced only to an llms.txt endpoint description (C32). It is weak but acceptable.
- All other rows (Campus, Area revision, Area policy, Quota, Space booking config) trace to accepted claims.

## "Not publicly documented" checks
These were searched in the help-centre index (all 50 Desk & Rooms articles), llms.txt (59 resource groups + 21 webhook events), the security/DPA pages and the web:
- CAD/DWG/DXF/PDF/SVG import, auto-detection, IMDF/IFC: no evidence found. The status stands.
- Occupancy sensors: none in llms.txt, the integrations page or the help centre. The status stands.
- OIDC, audit log, Postman, changelog (/changelog 404 on both domains), deprecation policy, MCP, WCAG/VPAT (/accessibility 404), Power Automate/Make: no evidence. The status stands.
- ISO 27001: the DPA says the *infrastructure provider* has "SOC 2 Type II report and EN ISO 27001 compliance". Archie itself does not claim ISO 27001, so the status stands, but the draft should mention this nuance.

## New claims

| id | verdict | claim | url | quote |
|---|---|---|---|---|
| N1 | ACCEPT | Published prices for other products: Room booking Starter from $159/mo, Pro from $249/mo; Visitor management Starter $109 and Pro $185 per location/month; Coworking Starter $192/mo (monthly) or $165/mo (yearly), Pro $299/mo (monthly) or $257/mo (yearly), Pro limited to 2 locations and 200 active users, Starter to 1 location and 100 active users; Enterprise custom for all | https://archieapp.co/pricing | "visitor.pricing-boxes.starter.monthly":"$109 per location/month" … "coworking.pricing-boxes.pro.monthly":"$299/month","…pro.yearly":"$257/month" … "room.pricing-boxes.starter.monthly":"Starting at $159/month" |
| N2 | ACCEPT | The GitHub account `archieApp` was created 2026-03-30, has 0 public repos and no profile name or blog link to the vendor | https://api.github.com/users/archieApp | "created_at": "2026-03-30T16:00:21Z", "public_repos": 0 |
| N3 | ACCEPT | Booking `item_type` enum is `area`, `bookable`, `all`; `item_uuid` therefore references an Area or a bookable resource | https://developers.archieapp.co/reference/post_bookings-created.md | "bookings.ItemType": enum "area", "bookable", "all" |
| N4 | ACCEPT | Update Area (`PUT /spaces/{spaceDomain}/conferenceRooms/{areaUUIDOrSlug}`) takes `types.ConferenceRoomRest` as its request body, which includes `map` (string), `map_meta` (object of strings) and `origin` (object of numbers) | https://developers.archieapp.co/reference/put_spaces-spacedomain-conferencerooms-areauuidorslug.md | requestBody schema "$ref": "#/components/schemas/types.ConferenceRoomRest"; "map_meta": {"additionalProperties": {"type": "string"}}, "origin": {"additionalProperties": {"type": "number"}} |
