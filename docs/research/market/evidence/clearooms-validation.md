# Validation: Clearooms

Validated 2026-11-25. I fetched every ledger URL (57 unique, all returned HTTP 200) and compared each claim against the page text. The extra sources used were https://clearooms.com/ai-info/ (the vendor's own fact sheet for AI assistants, "Last updated: 3 November 2026"), https://clearooms.com/llms.txt, https://clearooms.com/product/feature-list/ and the G-Cloud 15 service page https://www.applytosupply.digitalmarketplace.service.gov.uk/g-cloud/services/644555930164125 (official supplier answers, dated 2026-01-29). The draft says G-Cloud pages failed to load, but this one loads.

## Summary counts

| Verdict | Count |
|---|---|
| ACCEPT | 82 |
| REVISE | 6 (C21, C25, C26, C28, C63, C82) |
| REJECT | 0 ledger claims (the 24 unsourced draft statements below are REJECT as written) |
| NEW | 17 (N1-N17) |

**Most serious problems**

1. **Several "not publicly documented" statements are wrong.** The vendor's G-Cloud 15 listing says:
   - API documentation exists in OpenAPI (Swagger) format, and an API sandbox or test environment exists.
   - Rate limiting applies.
   - User and supplier audit data is kept for at least 12 months.
   - Data export is in CSV.
   - Customers cannot choose where their data is stored.

   The integrations page says the API is "REST-based, fully documented and available on every plan". The draft headline ("the API reference is not public") is still true, but sections 1, 7, 8, 10 and 14 understate what is documented.
2. **The draft mixes up two Zapier sources.** Its Zapier-derived entity and event lists come from the vendor KB page (C25, C26). That page describes Visitor triggers, Create Visitor, Find Visitor and Find Desk Recommendation. **The live public Zapier app (zapier.com, integration v1.0.92, labelled "Beta") has none of these.** It does have Create User, Find User, Find Room, Find Meeting and Find-or-Create actions that the KB omits. Sections 10, 11 and 12 should say which source each item comes from.
3. **Trigger mechanism and Zapier auth are documented.** All 24 public triggers are marked "Instant" on zapier.com ("isHook": true in the page data). The KB FAQ says "All Clearooms triggers fire in real time". The Zapier connection's authType is "oauth". The draft says both are not documented.
4. **Compliance wording is misleading.** "ISO 27001 and SOC 2 are not publicly documented" should become "Clearooms states it does not hold ISO 27001 or SOC 2; those apply to its AWS infrastructure" (N7). A DPA exists (clearooms.com/dpa/, N10).
5. **Other gaps are disproved:**
   - Occupancy sensors: the vendor says there is no native sensor hardware and no sensor partnerships (N8).
   - EV parking bays and permits are documented (N12).
   - The founding year is documented as 2020, with HQ in Huddersfield (N9).
   - The AI section misses two shipped AI features (N11).

**Zapier framing check (requested):** mostly good. Section 10 ("Zapier exposes…"), section 11 ("de facto event catalogue") and the section 12 heading ("from docs and Zapier only, no API schema") are clearly framed as Zapier-derived and not as the vendor API. The headline sentence "Everything about the API's shape below comes from the Zapier integration" should now also mention the G-Cloud API capability list (N2). Every Zapier list should be tagged "per KB Zapier guide" or "per zapier.com listing" because the two differ (problem 2).

## Claim table

| id | verdict | flags | corrected text (REVISE) | note |
|---|---|---|---|---|
| C1 | ACCEPT | | | Quote verbatim on pricing page. |
| C2 | ACCEPT | | | Plan 1 £66/£60 and Plan 7 £1155/£1050 confirmed. The pricing FAQ repeats the bands. |
| C3 | ACCEPT | | | "Enterprise 500+ Desks CONTACT US". |
| C4 | ACCEPT | | | Tier 1 £11/£10 to Tier 5 £7/£6. Rooms Enterprise starts at 300+. |
| C5 | ACCEPT | | | Parking Plans 1-6 match desk bands, with "Enterprise 400+ Spaces". |
| C6 | ACCEPT | | | The FAQ also lists Teams booking, SSO, SCIM, visitor management and the iOS/Android apps. |
| C7 | ACCEPT | | | Verbatim. |
| C8 | ACCEPT | | | Verbatim. "Monthly billing costs 10% more". |
| C9 | ACCEPT | | | Verbatim. |
| C10 | ACCEPT | | | Blog published 24 Jul 2026. |
| C11 | ACCEPT | THIRD-PARTY-ONLY (no; it is a vendor blog) | | The integrations page FAQ says the same and is the better official source. |
| C12 | ACCEPT | | | Verbatim. |
| C13 | ACCEPT | | | Verbatim. |
| C14 | ACCEPT | | | Both quotes present on the homepage. |
| C15 | ACCEPT | | | Module names from the site navigation. |
| C16 | ACCEPT | | | The quote is verbatim. The same page also says "The API is REST-based, fully documented and available on every plan" (see N1). |
| C17 | ACCEPT | | | Verbatim. The path is "Settings > Account > Apps & integrations> APIs". |
| C18 | ACCEPT | | | The page text is "You will also receive an expiry notification before the token expires", so the quote can be made verbatim. The page does not say tokens are admin-only. |
| C19 | ACCEPT | | | The link "here" has href https://portal.clearooms.com/settings/account/api. |
| C20 | ACCEPT | | | Title is "Clearooms - Portal" (SPA shell). |
| C21 | REVISE | | "Clearooms says it has an open API; for integrations it points to its integrations page and says to 'contact us to request access to additional documentation'." | The "additional documentation" sentence is about integrations generally, not specifically API docs. Also, G-Cloud now says the API docs are OpenAPI/Swagger (N2). |
| C22 | ACCEPT | | | Root serves `<title>Clearooms API</title>`. /docs, /swagger, /openapi.json and /v1 return 404. |
| C23 | ACCEPT | | | "This organization has no public repositories." |
| C24 | ACCEPT | | | GitHub API: created 2023-02-08, OCaml, 1 commit. |
| C25 | REVISE | | "The vendor's KB Zapier guide lists create/update/delete triggers for Bookings (desks and parking), Meetings, Visitors, Users, Desks, Offices, Rooms and Service Requests, plus Booking Checked In/Checked Out/Not Attended. The public zapier.com listing (v1.0.92, Beta) has 24 triggers matching these **except it has no Visitor triggers**." | The KB says bookings cover "Desks & Parking Spaces". The KB also says "New Office - fires when a new office or floor is added". |
| C26 | REVISE | | "KB lists actions Create Booking/Invitation/Meeting/Team/Visitor and searches incl. Find Desk Recommendation, Find Custom Field, Find Desk, Find Meeting, Find Room, Find User, Find Visitor. The public zapier.com listing instead has actions Create Booking/Invitation/Meeting/Team/User, searches Find Desk/Meeting/Room/User, Find-or-Create Meeting/User, and a hidden Find Custom Field; **no Create Visitor, Find Visitor or Find Desk Recommendation**." | KB and live app differ (see N15). |
| C27 | ACCEPT | | | Verbatim tip. The KB's "Before you begin" says "admin or manager-level access", which is inconsistent. |
| C28 | REVISE | | "The public Zapier app (v1.0.92, labelled Beta) has 24 instant triggers, 5 actions (Create Booking, Invitation, Meeting, Team, User), 4 searches and 2 find-or-create actions." | The counts come from the page's embedded data, not page prose. The tag should be "third-party marketplace", not "review". The description quote is verbatim. |
| C29 | ACCEPT | | | All six IdPs are listed. |
| C30 | ACCEPT | | | Verbatim. |
| C31 | ACCEPT | | | Titles present in the Security index. |
| C32 | ACCEPT | | | Verbatim. |
| C33 | ACCEPT | | | Verbatim. |
| C34 | ACCEPT | | | Verbatim. "Set and managed by Microsoft". |
| C35 | ACCEPT | | | Verbatim. |
| C36 | ACCEPT | | | Verbatim. |
| C37 | ACCEPT | | | Verbatim. |
| C38 | ACCEPT | | | Verbatim. |
| C39 | ACCEPT | | | Verbatim. |
| C40 | ACCEPT | | | Verbatim. |
| C41 | ACCEPT | | | Supported. The Tenant ID is also entered at portal.clearooms.com/settings/account/microsoft-teams. |
| C42 | ACCEPT | | | Verbatim. |
| C43 | ACCEPT | | | Verbatim. |
| C44 | ACCEPT | | | Verbatim. The page also lists the Clearooms production IPs. |
| C45 | ACCEPT | | | Verbatim. Note that the page also documents matching on **Employee ID**, which the User entity relies on. |
| C46 | ACCEPT | | | Verbatim. |
| C47 | ACCEPT | | | Same page: "draft plan back to you with 48hrs". |
| C48 | ACCEPT | | | Present in the site navigation on the homepage and pricing page. |
| C49 | ACCEPT | | | Steps confirmed: "Click anywhere on your plan to add a desk". |
| C50 | ACCEPT | | | Verbatim. |
| C51 | ACCEPT | | | August 2026 entry, verbatim. |
| C52 | ACCEPT | | | Verbatim. The same page says "Clearooms automatically detects which desks fall within the boundary" and "Each desk can belong to only one neighbourhood". |
| C53 | ACCEPT | | | Verbatim. |
| C54 | ACCEPT | | | Verbatim. |
| C55 | ACCEPT | | | Verbatim. |
| C56 | ACCEPT | | | Verbatim. |
| C57 | ACCEPT | | | Verbatim. |
| C58 | ACCEPT | | | Verbatim. |
| C59 | ACCEPT | | | Verbatim. Exceptions can be set by Room, Role or Team, and administrators are exempt. |
| C60 | ACCEPT | | | All six titles are present in the /general index. |
| C61 | ACCEPT | | | Verbatim. |
| C62 | ACCEPT | | | All five titles are present in the /settings index. |
| C63 | REVISE | | Claim text is OK; replace the quote with the actual Nov 2026 entry: "Clearooms uses the user's location to check them in to their desk booking when they arrive at the office. It also records a step-out when they leave". | The ledger quote is not on the page, and it says "check-out", which is wrong (the feature records a step-out). |
| C64 | ACCEPT | | | Verbatim. |
| C65 | ACCEPT | | | Verbatim. |
| C66 | ACCEPT | | | Kiosk, future visitor logging and email notifications are confirmed. |
| C67 | ACCEPT | | | Verbatim, including "automatically scheduled". |
| C68 | ACCEPT | | | Verbatim. |
| C69 | ACCEPT | | | Verbatim, including the match score. |
| C70 | ACCEPT | | | Verbatim. |
| C71 | ACCEPT | | | Both quotes are present. v1.23.8 was released "5d ago" (about 20 Nov 2026). |
| C72 | ACCEPT | | | Verbatim. |
| C73 | ACCEPT | | | Verbatim. |
| C74 | ACCEPT | | | Verbatim. |
| C75 | ACCEPT | | | Verbatim. |
| C76 | ACCEPT | | | Verbatim. |
| C77 | ACCEPT | | | Verbatim. |
| C78 | ACCEPT | | | Verbatim. Infrastructure FAQ: "recovered on request for up to 35 days following deletion". |
| C79 | ACCEPT | | | All six are listed. |
| C80 | ACCEPT | | | Verbatim. Changelog dates the certification to Aug 2025. |
| C81 | ACCEPT | | | Verbatim. "We do not answer excel based assessments." |
| C82 | REVISE | | "The KB FAQ lists 7 portal languages (EN, FR, DE, ES, IT, NL, Welsh); the current feature-list page lists 9, adding Portuguese and Polish." | The undated KB conflicts with https://clearooms.com/product/feature-list/. See N17. |
| C83 | ACCEPT | | | The Oct 2026 Zapier entry exists. The quote is paraphrased (the actual text is "Connect Clearooms to 7,000+ apps…"). Entries run month by month. |
| C84 | ACCEPT | | | Verbatim. |
| C85 | ACCEPT | | | Verbatim. Note that the integrations page says 9,000+ apps and the KB says 7,000+. |
| C86 | ACCEPT | | | Verbatim. |
| C87 | ACCEPT | | | All titles are present in /settings. |
| C88 | ACCEPT | | | Scope list and the "add Meeting rooms to your subscription" sentence are confirmed. The quote is weak, so use the scope URLs instead. |

STALE: none of the pricing or feature claims comes from a page known to be more than 12 months old. Most KB pages are undated ("© 2025"), so their age cannot be confirmed. THIRD-PARTY-ONLY: none material. C28 and C71 are marketplace or store listings, used appropriately.

## Unsourced statements in the draft (verdict: REJECT as written; most are true and only need a ledger claim)

1. §1 "Founding year, headcount and street-address HQ are not publicly documented on the About page." This is true for the About page only. The ai-info page gives "founded 2020, HQ Huddersfield" and Company No. 12855479 (N9). Rewrite it.
2. §1 "The homepage lists education, health, professional services and the public sector, and claims customers in 30+ countries." This is true (navigation and footer) but there is no ledger claim. The ai-info page says "500+ organisations across 35 countries" (N9).
3. §1 "The pricing page does not mention API or Slack. API availability by plan is not publicly documented." **This is wrong.** The integrations page says the API is "available on every plan" and the FAQ covers "API access" (N1).
4. §2 meeting-room rules "Buffer times, min notice, max length, approvals, linked multi-room meetings, attendees ≤ capacity (KB titles…)". These are in the /settings and /general indexes, but no claim covers them.
5. §2 parking "Misuse reporting (Sep 2026 release, changelog)". True (Sep 2026 entry) but unledgered.
6. §2 parking "EV charging: not publicly documented". **This is wrong** (N12).
7. §2 equipment "Meeting-room 'services' (catering etc.) exist as KB topics". True but unledgered. The feature list also says rooms can be filtered by equipment.
8. §3 "Android is implied by the pricing page". The pricing FAQ states "the iOS and Android apps" outright, so it is stated, not implied.
9. §5 "The blog says desk and visitor bookings also sync to Outlook." True ("Every Clearooms booking, whether for a desk, a meeting room, or a visitor, syncs directly with Outlook") but no claim covers it.
10. §6 "Occupancy sensors: not publicly documented." **This is wrong** (N8).
11. §6 "Desk QR stickers can be ordered (Oct 2026 changelog)". True but unledgered.
12. §7 "SaaS only. Private or on-prem hosting is not publicly documented." G-Cloud gives the deployment model as "Public cloud" (N5).
13. §7 "Data-residency choice: not publicly documented." **This is wrong.** G-Cloud answers "User control over data storage and processing locations: No" (N5).
14. §8 "ISO 27001 and SOC 2 are not publicly documented…" This is misleading. The vendor states that it does not hold them (N7).
15. §8 "status.clearooms.com and trust.clearooms.com did not resolve." Confirmed (no response), but this is an observation, not a claim.
16. §8 "Audit logs: not publicly documented." **This is wrong** (N3).
17. §8 "The GDPR page does not name a DPA document." The scope is accurate, but a DPA exists (N10).
18. §8 "A 'Users - Last Seen' feature exists (Settings index)." True but unledgered.
19. §9 "The header format, scopes and OAuth2 are not publicly documented … The OAuth flow behind that is not documented." The zapier.com page data shows authType "oauth" (N14), so the Zapier connection uses OAuth. The token header format remains undocumented.
20. §10 "Not publicly documented: OpenAPI/Swagger, Postman, pagination, rate limits and versioning." **This is partly wrong.** G-Cloud gives the API documentation format as "Open API (also known as Swagger)" and says the API "is subject to rate limiting" (N2). Postman, pagination and versioning remain undocumented.
21. §11 "Whether these are polling or REST-hook triggers is not publicly documented." **This is wrong.** The triggers are Instant/REST-hook (N13).
22. §14 "Sandbox: none documented." **This is wrong.** G-Cloud answers "API sandbox or test environment: Yes" (N2).
23. §15 "Smart Booking" as the only AI feature, and "The Teams blog has no AI mention". The latter is true, but the section is incomplete (N11).
24. §16 "WCAG/VPAT: not publicly documented. A web search found no accessibility statement." This needs nuance. G-Cloud gives accessibility standards as "None or don't know", describes the accessibility work and testing, and says the documentation meets WCAG 2.2 AA (N4).

Not-documented statements I could **not** disprove after 2-3 searches each: lockers, DWG/CAD import, IMDF/IndoorGML/IFC, map geometry via API, customer webhook catalogue and signing, Power Automate/Make, MCP/Copilot agent, pagination/versioning, deprecation policy, Postman, HRIS-native connectors. The meeting-zones page mentions "automatic desk detection", but it refers to assigning desks to a neighbourhood boundary, not to detecting desks from a floor plan, so §4's statement stands.

## Data-model trace issues (§12)

- **Office:** "working hours" has no claim (settings title only). "Geofence boundary" is on the C64 page, but the C64 claim text omits it. The containment relation is marked INFERRED, but C50 and C54 directly support desk, room and parking pins on an office plan. Add the KB Zapier guide's statement that Office = "office or floor".
- **Desk:** "name, office" is cited to C28, but the C28 claim text has no field detail. Add a claim for zapier.com "Find Desk Finds a desk by name and optionally by office" (N15). "Notes" has no claim (KB title "Desk Notes"). Neighbourhood membership is marked INFERRED but is documented: desks are auto-detected within the boundary and each belongs to only one neighbourhood (C52 page). Upgrade it and add the text to the claim.
- **Room:** "capacity" (KB title) has no claim. The feature list and zapier.com Find Room ("by name and office") are better. "name, office" again relies on C28 without field text.
- **Booking:** the "User → Desk" relation and fields are not in any accepted claim. zapier.com booking trigger fields are User, User Teams, Type, Booked By User, Desk, Office, and Create Booking takes Guest Name/Email and **Vehicle IDs** (N15). Bookings cover desks **and parking** (KB Zapier guide), which the table omits.
- **Meeting:** C25 gives no fields. "External attendees" is unledgered (Oct 2026 changelog) and "multi-room" is unledgered (settings titles). zapier.com Create Meeting fields are Subject, Description, Organiser, Private, Start/End Time, **Time Zone**, Rooms, Attendees (Name, Email, Attendance Type) (N15).
- **Visitor:** **the entity is not in the live Zapier app**, only in the KB guide (C25 needs revising). "Custom fields" has no claim (KB title "Visitor Management - Custom Fields"). C66 does not mention custom fields.
- **User:** C28 has no field text. "Employee ID" is on the C45 page but not in the claim text. "Manager" and "roles" come from zapier.com Find User ("filters for manager status, teams, and roles") and Create User (Name, Email, Office, Teams, Attributes, Roles). Add these via N15.
- **Team:** zapier.com Create Team has Name, **Manager, Parent Team**, Roles (N15). That gives teams a hierarchy, which the model omits.
- **Service Request:** the Meeting relation is labelled INFERRED. OK.
- **Missing entities visible in the Zapier app metadata** (hidden dynamic-dropdown triggers, Zapier-derived): Building, Role, Attribute, Custom Field, Invitation, Room Resource, Timezone, Scope, and Vehicle (via Create Booking). Add them if the model is meant to be complete, and label them Zapier-derived.

## New claims

| id | claim | url | quote | verdict |
|---|---|---|---|---|
| N1 | The vendor says the API is REST-based, documented and on every plan | https://clearooms.com/integrations/ | "The API is REST-based, fully documented and available on every plan." / "That covers SSO, SCIM, Outlook, Teams, Google Calendar, Slack, Paxton, Salto, and API access." | ACCEPT |
| N2 | API docs are in OpenAPI/Swagger format, a sandbox exists, and rate limiting applies (G-Cloud 15 supplier answers, 2026-01-29). The API scope covers desks, rooms, open spaces, bookings, users/attributes and events | https://www.applytosupply.digitalmarketplace.service.gov.uk/g-cloud/services/644555930164125 | "API documentation formats: Open API (also known as Swagger)" / "API sandbox or test environment: Yes" / "is subject to rate limiting" / "Trigger and consume events and status updates for third-party systems" | ACCEPT |
| N3 | Audit data for users and the supplier is available in real time and kept for at least 12 months. Exports are audit-logged | same G-Cloud URL | "Users have access to real-time audit information" / "How long user audit data is stored for: At least 12 months" / "All exports are subject to role-based access controls and audit logging" | ACCEPT |
| N4 | Accessibility: no conformance standard is claimed for the service. Testing uses Accessibility Insights. Documentation meets WCAG 2.2 AA | same G-Cloud URL | "Accessibility standards: None or don't know" / "Assessment reports using Accessibility Insights for Web 2.46.0 (axe-core 4.10.2)" / "Documentation accessibility standard: WCAG 2.2 AA" | ACCEPT |
| N5 | Public cloud only. Data is stored in the EEA with no customer choice of location | same G-Cloud URL | "Cloud deployment model: Public cloud" / "Data storage and processing locations: European Economic Area (EEA)" / "User control over data storage and processing locations: No" | ACCEPT |
| N6 | Data export and import use CSV. Bulk extraction is also possible via the Open API | same G-Cloud URL | "Data export formats: CSV" / "Data import formats: CSV" / "extract data using the Clearooms Open API" | ACCEPT |
| N7 | Clearooms itself does not hold ISO 27001 or SOC 2 (only AWS does) | https://clearooms.com/ai-info/ | "its AWS infrastructure carries ISO 27001 and SOC 2, but Clearooms itself does not" | ACCEPT |
| N8 | There are no native occupancy sensors or sensor partnerships. Sensor data can come in via the open API | https://clearooms.com/ai-info/ | "Clearooms has no native occupancy sensor hardware or sensor partnerships; sensor data can connect via the open API" | ACCEPT |
| N9 | Founded 2020. HQ is Huddersfield. Company No. 12855479. 500+ organisations in 35 countries | https://clearooms.com/ai-info/ | "UK company (Clearooms Limited, No. 12855479), founded 2020, HQ Huddersfield, England" / "500+ organisations across 35 countries" | ACCEPT |
| N10 | A Data Processing Agreement is published | https://clearooms.com/llms.txt (links https://clearooms.com/dpa/, HTTP 200) | "[Data processing agreement](https://clearooms.com/dpa/)" / ai-info: "A Data Processing Agreement is available" | ACCEPT |
| N11 | Other shipped AI features: AI placement in Team Planner, and an AI assistant in the ArrivalsHQ visitor journey builder | https://clearooms.com/ai-info/ | "Smartbooking recommendations, intelligent Team Planner placement and an AI assistant in the ArrivalsHQ journey builder" | ACCEPT |
| N12 | Parking supports EV, Blue Badge and permit bays, waitlists and pool cars | https://www.clearooms.com/pricing | "Permits, EV and visitor bays included." / "Permits: Blue Badge, EV, contractor" / "Conflict prevention & waitlists" | ACCEPT |
| N13 | All Clearooms Zapier triggers are instant (REST-hook), not polling | https://zapier.com/apps/clearooms/integrations (also https://knowledgebase.clearooms.com/zapier) | "Instant — This event starts a Zap instantly." (all 24 visible triggers; page data "isHook":true) / KB: "All Clearooms triggers fire in real time" | ACCEPT |
| N14 | The Zapier connection uses OAuth (Zapier-derived, from page-embedded metadata, not vendor API docs) | https://zapier.com/apps/clearooms/integrations | `"authType":"oauth"`, `"selectedApi":"App232439CLIAPI@1.0.92"` | ACCEPT |
| N15 | Zapier-derived field-level data model from the live listing (Beta): booking trigger fields; Create Booking/Meeting/User/Team inputs; Find Desk/Room keys; no Visitor or Desk-Recommendation operations; hidden lookups for Building, Role, Attribute, Custom Field, Invitation, Room Resource, Timezone, Scope | https://zapier.com/apps/clearooms/integrations | "Booking Updated … User, User Teams, Type, Booked By User, Booked By User Teams, Desk, Office" / "Create Team … Name Required, Manager, Parent Team, Roles" / "Create Booking … Guest Name, Guest Email, Book All Available, Vehicle IDs" / "Find Room Finds a room by name and office." | ACCEPT |
| N16 | Reports export as CSV/PDF | https://clearooms.com/ai-info/ | "utilisation and efficiency reporting with CSV/PDF export" | ACCEPT |
| N17 | The current feature list gives 9 languages, adding Portuguese and Polish | https://clearooms.com/product/feature-list/ | "English, French, German, Spanish, Italian, Dutch, Portuguese, Polish and Welsh" | ACCEPT |

Caveat on N7-N11 and N16: ai-info is vendor marketing written for LLMs. It is official and dated 3 Nov 2026, but it is self-description. Cite it as "vendor states".
