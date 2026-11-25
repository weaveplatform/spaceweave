# deskbird validation

Validated 2026-11-25. Method: every cited URL was re-fetched with curl and the quotes were grepped from the raw text. That covers the developer hub `.md` pages, llms.txt, the deskbird.com pages, status.deskbird.com, the GitHub API, and the help-centre articles (read through the public Zendesk JSON API `/api/v2/help_center/en-us/articles/<id>.json`). trust.deskbird.com returns 403 to curl, so it was read with WebFetch only, through a summariser. "Not publicly documented" statements were tested with Zendesk help-centre searches (`/api/v2/help_center/articles/search.json`) and a web search.

## Summary counts
- ACCEPT: 95
- REVISE: 7 (C3, C42, C60, C75, C76, C24, C28)
- REJECT: 0 ledger claims. 11 draft statements are UNSOURCED (see below).
- NEW: 10 (N1–N10)

## Claim table

| id | verdict | flags | corrected text (for REVISE) | note |
|---|---|---|---|---|
| C1 | ACCEPT | | | Verbatim on developer.deskbird.com |
| C2 | ACCEPT | | | |
| C3 | REVISE | | API keys are created by a Global Admin. The developer overview says to use the Integrations section ("Public API section"). The help centre (edited 2026-10-29) says Admin > Company: API keys, choosing type **API** or **SCIM**. | The overview has no key type called "Public API". The two docs name the location differently. |
| C4 | ACCEPT | | | |
| C5 | ACCEPT | | | |
| C6 | ACCEPT | | | |
| C7 | ACCEPT | | | llms.txt lists every group. Floors, Zones and Resources have only GET endpoints, which confirms the draft's read-only statement. |
| C8 | ACCEPT | | | |
| C9 | ACCEPT | | | llms.txt also says create and update are "performed by the API token creator" (supports draft §9) |
| C10 | ACCEPT | | | |
| C11 | ACCEPT | | | |
| C12 | ACCEPT | | | "Typically 1 for desks" is present. zoneId is nullable (used in the data model). |
| C13 | ACCEPT | | | |
| C14 | ACCEPT | | | |
| C15 | ACCEPT | | | "At least one of officeId or ids must be provided — the request returns 400" |
| C16 | ACCEPT | | | Zone status enum is active/inactive (verified) |
| C17 | ACCEPT | | | |
| C18 | ACCEPT | | | |
| C19 | ACCEPT | | | The office schema also says timeZone is IANA, with examples like Europe/Paris |
| C20 | ACCEPT | | | |
| C21 | ACCEPT | | | |
| C22 | ACCEPT | | | History also carries userUuid and completedAt (verified) |
| C23 | ACCEPT | | | |
| C24 | REVISE | | Room bookings have a type of occurrence, exception or single. Create takes a room ID, a start time, a duration in seconds, a title, an optional description, a privacy flag and an optional user ID. Private bookings cannot be updated, cancelled or checked in via the API (403). | The quote is correct, but the draft's data model cites C24 for fields (duration, title, privacy, attendees) that the claim text doesn't state. llms.txt supports them, so the claim text should be widened. |
| C25 | ACCEPT | | | |
| C26 | ACCEPT | | | Service requests use the same updatedAt polling (llms.txt), which supports draft §11 |
| C27 | ACCEPT | | | users.md also lists the fields name, email, role and primaryOfficeId, and the include values externalId, settings, userGroups and officeRoles |
| C28 | REVISE | | The Sensors API lists the monitors detected across the company. Each has an id, manufacturer, serialNumber, productCode, state (ACTIVE/EXCLUDED), lastConnectionAt and a mapping (office/floor/area/desk). Timestamps are UTC. Source: https://developer.deskbird.com/api/sensors/lists-company-monitors.md | The draft cites C28 for monitor fields and for "API timestamps are UTC", which the llms.txt quote does not cover. The endpoint page supports both. |
| C29 | ACCEPT | | | |
| C30 | ACCEPT | | | Scheduling fields (full_day/morning/afternoon, isPrimary, color) verified on the scheduling-options page |
| C31 | ACCEPT | | | |
| C32 | ACCEPT | | | `version: 1.0.0` present. /openapi.json returns 404 (re-checked). |
| C33 | ACCEPT | | | |
| C34 | ACCEPT | | | "You are a **company admin**" |
| C35 | ACCEPT | | | Verbatim. However, connect.us.deskbird.com **does not resolve in DNS** (checked 2026-11-25), so the US environment is unverified beyond the docs. See the hosting conflict below. |
| C36 | ACCEPT | | | Also: 30 days per refresh token, and refresh-token reuse revokes the session |
| C37 | ACCEPT | | | |
| C38 | ACCEPT | | | USD figures. The page also lists EUR 2.75/3.50, CHF, GBP, CAD and AUD. |
| C39 | ACCEPT | | | "Public API access" is in Professional's "Everything in Business, plus" list. The comparison table marks Public API "included" for Professional and Enterprise only. N4 corroborates. |
| C40 | ACCEPT | | | Add-on block verified. Enterprise's own list has "Advanced data privacy setups" but not SAML/SCIM. N5 corroborates. |
| C41 | ACCEPT | | | |
| C42 | REVISE | | Every tier includes unlimited desks and parking, users, admins and locations, plus the MS Teams app, iOS & Android apps, Slack app, web, Outlook add-in, calendar sync, Microsoft Entra ID, Google Workspace and CSV user upload. Public API, access-control integration and multi-tenancy are marked for Professional and Enterprise only. | The draft cites C42 for "native apps/Teams/Outlook/Slack on all tiers" and "access control is Professional+". The page supports both, but the claim text doesn't say either. |
| C43 | ACCEPT | | | Visitors Plus: "Reception kiosk mode Pre-visit registration" |
| C44 | ACCEPT | | | Only Global Admins can generate keys |
| C45 | ACCEPT | | | |
| C46 | ACCEPT | | | |
| C47 | ACCEPT | | | |
| C48 | ACCEPT | | | |
| C49 | ACCEPT | | | |
| C50 | ACCEPT | | | |
| C51 | ACCEPT | | | |
| C52 | ACCEPT | | | |
| C53 | ACCEPT | | | |
| C54 | ACCEPT | | | |
| C55 | ACCEPT | | | "use the arrow keys to nudge it 1px at a time" |
| C56 | ACCEPT | | | |
| C57 | ACCEPT | marketing | | |
| C58 | ACCEPT | marketing | | |
| C59 | ACCEPT | | | |
| C60 | REVISE | | Temporary closures (with an end date) need Desks Professional and above. Closing a floor or area cancels future bookings in the closed period for desks, parking and other resources. Meeting rooms keep their own Active/Inactive toggle, and their bookings are **not** cancelled. | Draft §2 ("Closing cancels future bookings") should carry the meeting-room exception |
| C61 | ACCEPT | | | Desks and parking share one settings set. Meeting rooms have their own. |
| C62 | ACCEPT | | | |
| C63 | ACCEPT | | | |
| C64 | ACCEPT | | | "source of truth" present |
| C65 | ACCEPT | | | |
| C66 | ACCEPT | | | |
| C67 | ACCEPT | | | |
| C68 | ACCEPT | | | |
| C69 | ACCEPT | | | "Granular permission levels by location, floor, room, department, or role" |
| C70 | ACCEPT | | | The page also says "All data is hosted exclusively within the EU, this also applies to our subprocessors", and describes a multi-tenant, logically isolated setup |
| C71 | ACCEPT | | | |
| C72 | ACCEPT | | | |
| C73 | ACCEPT | | | |
| C74 | ACCEPT | | | |
| C75 | REVISE | | The deskbird Intelligence transparency article (Desks Professional+ or Dock Plus) states hosting on Google Cloud Platform, region europe-west3 (Frankfurt), says "Data does not leave the EU", and gives a single endpoint, api.deskbird.com. | The quote is verbatim, but it comes from an article scoped to deskbird Intelligence, not a platform-wide hosting statement. It is consistent with C70 and C100. |
| C76 | REVISE | summariser-only | The trust centre (header "Powered by SafeBase"; footer logo "SafeBase by Drata") lists GDPR, ISO/IEC 27001:2022 and SOC 2 Type 2 under Compliance. Its Legal section names five subprocessors: Mailgun Technologies, Inc., Sentry, Customer.io, Jimo and Mixpanel. No locations are shown, and the "Subprocessors" heading is otherwise empty. | trust.deskbird.com returns 403 to curl, so this is verified only through a WebFetch summariser, like the original. The frameworks and names match. "Built on SafeBase by Drata" is not a verbatim string. Five names is the whole visible list, not "including". |
| C77 | ACCEPT | summariser-only | | "Audit Logging" sits under Product Security (summariser). The page also has "Access Log Management" and "Event & Audit Log Management". These are security-programme controls, not proof of a customer-facing audit-log feature. The draft already says this. |
| C78 | ACCEPT | | | Verified in the raw HTML, not just the summariser: "api.deskbird.com", "app.deskbird.com", "deskbird.com", "All services are online", "Powered by Better Stack" |
| C79 | ACCEPT | | | |
| C80 | ACCEPT | | | |
| C81 | ACCEPT | | | The page title is "Sensor-free occupancy and auto check-ins" |
| C82 | ACCEPT | | | The room-sensor survey is also on this page (supports draft §6) |
| C83 | ACCEPT | | | |
| C84 | ACCEPT | | | |
| C85 | ACCEPT | | | |
| C86 | ACCEPT | marketing | | |
| C87 | ACCEPT | | | |
| C88 | ACCEPT | | | |
| C89 | ACCEPT | | | |
| C90 | ACCEPT | | | The article also lists Claude web, desktop, mobile and Claude Code. It does **not** mention Codex (see N2). |
| C91 | ACCEPT | marketing | | |
| C92 | ACCEPT | | | |
| C93 | ACCEPT | | | |
| C94 | ACCEPT | marketing | | |
| C95 | ACCEPT | marketing | | The full sentence also says "Each API key is scoped per client". That conflicts with the draft's "No OAuth or scopes" (U4). |
| C96 | ACCEPT | STALE | | Edited 2025-06-12 (>12 months). This is a security description, not pricing, so low risk. |
| C97 | ACCEPT | | | All 3 public repos are forks |
| C98 | ACCEPT | THIRD-PARTY-ONLY | | 10 stars, pushed 2022-11-17, Python. Used only as ecosystem colour. |
| C99 | ACCEPT | STALE | | 2024 blog post. A historical date only. |
| C100 | ACCEPT | | | |
| C101 | ACCEPT | | | |
| C102 | ACCEPT | | | |

## Pricing and plan gating (requested focus)
- **Public API on Professional:** confirmed in four places. These are the Professional "plus" list and the comparison table (Professional and Enterprise only) on /pricing, the Public API help article ("Desks Professional and above"), and the "Professional add-ons" article (N4). The developer overview only says "depends on your subscription" (C6). The draft is correct.
- **SAML/SCIM/HRIS in the UDM Plus add-on:** confirmed on /pricing (add-on block), in the SCIM article ("only available with the UDM Plus add-on"), in the SAML article (tagged UDM Plus), and in the UDM Plus setup article (N5). N5 also says Microsoft/Google sign-in is on every plan. The draft is correct.
  - Caveat: the security page (C69) presents SSO/SAML without any plan caveat.
  - Caveat: N5 mentions "an older UDM plan" in some contracts.
  - Caveat: N3 shows Entra SSO can use a customer-owned OIDC app registration.

## Hosting conflict: EU-only vs "US environment" (requested focus)
- **The EU-only statements are strong and repeated:**
  - Security page: "hosted exclusively in the EU … this also applies to our subprocessors" (C70).
  - About page: "hosted securely in Germany" (C100).
  - Pricing page: "Built & hosted in Europe".
  - Intelligence article: europe-west3, "Data does not leave the EU" (C75).
- **The US environment is mentioned in two official places:**
  - Developer MCP overview (C35).
  - Help-centre MCP connector article: "If your company is hosted in the deskbird US environment, use https://connect.us.deskbird.com/mcp" (N1).
- **No other evidence of US hosting:**
  - connect.us.deskbird.com does not resolve in public DNS today.
  - The trust centre says nothing about region.
  - A help-centre search and a web search found nothing on US hosting or data residency.
- **Conclusion:** the draft's "contradiction, US residency implied but not otherwise documented" is accurate. Strengthen it with N1 and the DNS result. The US environment may be planned or private, so do not state that US hosting exists.

## Unsourced statements in the draft (all REJECT as written)
- **U1.** §1: deskbird "calls itself 'Europe's leading workplace management platform'". It is cited [C80], but C80's quote lacks this phrase. **Fix:** cite N10.
- **U2.** §13: "Integration directory: 100+ integrations, mostly HRIS connectors…". This has no ledger claim.
  - The count is supportable: the sitemap has 102 /integration/ URLs, including personio, hibob and bamboohr (N9).
  - "Mostly HRIS" is unverified. Drop it.
- **U3.** §8: "DPA: offered through the trust centre 'Get access' request". This has no ledger claim, and "Get access" was not verified. C75's article does say the DPA is in the Trust Center. **Fix:** reword to "DPA available via the trust centre (C75 article)".
- **U4.** §9: "No OAuth or scopes. A key grants whole-company access." This is an inference, and it contradicts marketing C95 ("Each API key is scoped per client"). C2 supports "company-scoped key". **Fix:** say "no documented OAuth flow or per-endpoint scopes; keys are company-scoped [C2]".
- **U5.** §14: "Recently added groups include Tickets, Visitors, Service requests and Sensors". There is no source for "recently added". Drop it.
- **U6.** §14: "no documented error model". This is unsourced reviewer judgement, and no search backs it. Mark it as ANALYSIS or drop it.
- **U7.** §15: "Supported clients are Claude and the ChatGPT/Codex desktop app [C90]". C90 doesn't say Codex. **Fix:** cite N2, which is titled "deskbird MCP server for ChatGPT (Codex)".
- **U8.** §17: "deskbird has no CAD/DWG ingestion and no wall, opening or room geometry". This states an absence of documentation as fact, while marketing C57 claims plans are ingested "in whatever format you have". **Fix:** reword to "documents no CAD/DWG ingestion…".
- **U9.** §16: "API timestamps are UTC [C19][C23][C28]". None of the cited claims says UTC. Only the monitors endpoint states it. **Fix:** narrow the statement and cite revised C28.
- **U10.** §9: "SCIM: bearer keys of type 'SCIM', generated in Admin > API keys". "Bearer" is not in any source, and the menu path is Admin > Company: API keys. This is minor.
- **U11.** §1/§3: "Mobile, Teams and Slack … on all tiers [C42]" and §6 "Access-control integration is a Professional+ feature [C42]". The page supports both, but the C42 claim text doesn't cover them. **Fix:** use revised C42.

## Data-model trace issues (§12)
- **Room booking.** The fields room, start, duration, title, description, privacy and attendees trace to llms.txt, but not to C24's claim text. **Fix:** widen C24 (see the revise text).
- **User.** The fields name, email, role, primaryOfficeId and the include values externalId, settings, userGroups and officeRoles are not in C27's claim text. They are verified on users.md. Widen C27.
- **Scheduling status / option.** C30 covers only policyRule. The other fields (day, duration enum, optionId, name, type, color, isPrimary, isActive) are verified on the scheduling-options page but not in any claim text. Widen C30.
- **Monitor.** The fields are not in C28's claim text. **Fix:** use revised C28.
- **Booking history.** userUuid and completedAt are not in C22's text (verified on the page). Minor.
- **Resource.** "zoneId (nullable)" and "equipment[{id, name}]" are verified on the page, but not in C12's text. Minor.
- **Room→Zone/Floor and Zone="area".** Both are correctly labelled INFERRED. Keep them out of hard relations. RoomResponse has no zoneId or floorId (confirmed).
- **Every other entity and relation traces to an accepted claim:** Office→C19, Floor→C15, Zone→C16/17/18, Booking→C20/21/22, Network configuration→C29.

## "Not publicly documented" checks
| Statement | Result |
|---|---|
| Webhooks | Confirmed absent: help-centre search "webhook" = 0, and no webhook endpoint in llms.txt |
| DWG/DXF/CAD import | Confirmed absent: searches for "dwg", "dxf" and "CAD" = 0 |
| IMDF/IndoorGML/IFC | Nothing found |
| Downloadable OpenAPI | /openapi.json = 404 (confirmed) |
| Lockers, neighbourhoods | Neighbourhood = 0 results. Lockers **are** documented, but as "Other" resources, not a distinct kind (N6). Accurate as written, but add N6. |
| OIDC SSO | Partly disproved. A manual OIDC setup exists for Microsoft Entra ID (N3). The SAML article contrasts SAML with "OAuth / Microsoft SSO / Google SSO". No generic third-party OIDC was found, so reword. |
| Changelog | Partly disproved. There is a product changelog as an in-app "What's New" feed (N7). There is still no API changelog or deprecation policy. |
| CSV/bulk import of layouts | No layout or floor-plan import was found. The resources UI has a bulk-create modal: N desks with prefix and numbering, no placement (N8). There are CSV imports for users and groups and for license plates only. |
| Zapier/Power Automate | Zapier = 0 results, and the app URL is 404. "Power Automate" returned only unrelated "automated" hits. |
| WCAG/VPAT, sandbox, on-prem, US residency | No evidence found ("accessibility" hits are all about access control) |
| Copilot/Teams agent | Only Teams app articles. No Copilot agent documentation found. |

## New claims
| id | verdict | claim | url | quote |
|---|---|---|---|---|
| N1 | ACCEPT | The help-centre MCP article also references a deskbird US environment endpoint | https://help.deskbird.com/hc/en-us/articles/50620548711441-deskbird-MCP-connector-for-Claude | "If your company is hosted in the deskbird US environment, use https://connect.us.deskbird.com/mcp instead." |
| N2 | ACCEPT | A separate help article covers the MCP server for ChatGPT (Codex). It is desktop app only, and the browser is not supported. | https://help.deskbird.com/hc/en-us/articles/51372253024273 (title "deskbird MCP server for ChatGPT (Codex)", edited 2026-11-10) | "The connection works in the ChatGPT desktop app." … "ChatGPT in the browser (chatgpt.com) does not support this type of MCP server connection." |
| N3 | ACCEPT | Entra ID SSO can be set up as a manual OIDC app registration, as an alternative to the gallery Enterprise Application | https://help.deskbird.com/hc/en-us/articles/14911346851345 (SSO with Microsoft Entra ID, edited 2026-10-09) | "Manual OIDC setup (no Enterprise Application required) If your organization cannot install the deskbird Enterprise Application …" |
| N4 | ACCEPT | Professional and Enterprise add the Public API, temporary closures, geolocation check-in, priority access and ticketing | https://help.deskbird.com/hc/en-us/articles/52408433243281 (Professional add-ons, edited 2026-11-06) | "Desks Professional and Desks Enterprise add features on top of the basic setup: closures with an end date, check-in by geolocation, priority access to areas, workplace ticketing and the Public API." |
| N5 | ACCEPT | UDM Plus bundles SAML SSO, HRIS, SCIM and visibility settings. Microsoft/Google sign-in is on every plan. | https://help.deskbird.com/hc/en-us/articles/52413482256145 (UDM Plus: setup, edited 2026-10-29) | "The steps in this guide need the UDM Plus add-on. If your contract includes an older UDM plan, the same steps apply. Sign-in with Microsoft or Google is available on every plan and doesn't need UDM Plus." |
| N6 | ACCEPT | Lockers, phone booths and equipment are modelled as "Other" resources | https://help.deskbird.com/hc/en-us/articles/47589092863249 (Book a parking spot or other resource, edited 2026-10-12) | "Other resources — any additional bookable item your company has configured, such as lockers, phone booths, or equipment." |
| N7 | ACCEPT | The product changelog is an in-app "What's New" feed | https://help.deskbird.com/hc/en-us/articles/10039231616785 (Get support, edited 2026-10-23) | "The What's New feed is your central place to stay up to date on product updates … You can access it anytime via the bell icon" |
| N8 | ACCEPT | Admins can bulk-create resources in the UI (count, status and naming prefix) | https://help.deskbird.com/hc/en-us/articles/29197912042385-Desks-Parking-Spots-and-Other-Resources | "Add multiple resources at once (e.g. 10 desks) Set their status Customize naming conventions (prefix, starting number)" |
| N9 | ACCEPT | deskbird.com has about 100 integration pages, including Personio, HiBob and BambooHR | https://www.deskbird.com/sitemap.xml | 102 unique `https://www.deskbird.com/integration/…` URLs, incl. `/integration/personio`, `/integration/hibob`, `/integration/bamboohr` |
| N10 | ACCEPT | deskbird describes itself as Europe's leading workplace management platform | https://www.deskbird.com/about | "deskbird is Europe's leading workplace management platform that makes hybrid work an experience your team will love." |
