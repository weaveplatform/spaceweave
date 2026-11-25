# Validation: eptura

Validated 2026-11-25. All 83 distinct ledger URLs resolved: 71 knowledge.eptura.com pages were re-read via the public MindTouch API (contents + date.edited); ioffice.github.io, developer.spaceiq.com, help.archibus.com, security.eptura.com, status.eptura.com, developer-portal.condecosoftware.com and the GitHub API returned HTTP 200 to curl; the three eptura.com pages return Cloudflare 403 to curl and were re-read with WebFetch. Every quote was matched against the fetched text (string search), not just skimmed.

## Summary counts
- ACCEPT: 161
- REVISE: 3 (C13, C51, C151)
- REJECT: 0
- NEW: 9 (N1-N9, all ACCEPT)
- Flags: STALE x47. These are documentation pages last edited before 2025-11-25, mostly mechanical; the substantive ones are C61-C64 (Workplace CAD standards, 2023), C116/C118 (Engage SSO/SCIM, 2023), C131/C133 (Workplace OAuth/webhooks, 2023) and C151-C153 (2024 security page, superseded by the 2026 trust portal). THIRD-PARTY-ONLY: none (C11-C15 are official but rest on a WebFetch summary because of the 403).
- Unsourced draft statements: 13 REJECT (listed below), several of which are now disproved by new claims.

## Claim table
| id | verdict | flags | corrected text (for REVISE) | note |
|---|---|---|---|---|
| C1 | ACCEPT |  |  | eptura.com 403 to curl; verified via WebFetch summary of /resource/eptura-history/ (incl. "In 2018, Archibus merged with Serraview under the JMI Equity portfolio"). |
| C2 | ACCEPT |  |  | eptura.com 403 to curl; verified via WebFetch summary of /resource/eptura-history/ (incl. "In 2018, Archibus merged with Serraview under the JMI Equity portfolio"). |
| C3 | ACCEPT |  |  | eptura.com 403 to curl; verified via WebFetch summary of /resource/eptura-history/ (incl. "In 2018, Archibus merged with Serraview under the JMI Equity portfolio"). |
| C4 | ACCEPT |  |  | eptura.com 403 to curl; verified via WebFetch summary of /resource/eptura-history/ (incl. "In 2018, Archibus merged with Serraview under the JMI Equity portfolio"). |
| C5 | ACCEPT |  |  | eptura.com 403 to curl; verified via WebFetch summary of /resource/eptura-history/ (incl. "In 2018, Archibus merged with Serraview under the JMI Equity portfolio"). |
| C6 | ACCEPT |  |  | Source resolves and supports the claim. |
| C7 | ACCEPT |  |  | Source resolves and supports the claim. |
| C8 | ACCEPT |  |  | Verified via WebFetch of /our-platform/ (exact strings returned). Tag "blog" is a mis-tag: this is an official marketing page. |
| C9 | ACCEPT | STALE |  | Source resolves and supports the claim. |
| C10 | ACCEPT |  |  | Link to ioffice.github.io confirmed (page 2026-06-16). The INFERRED "Workplace = former iOFFICE" is now directly sourced: see N1 (KC "iOffice is Eptura Workplace", July 2023) and N2 (eptura.com history: "In 2023 the iOffice product was renamed Eptura Workplace"). Cite N1/N2 for the rename. |
| C11 | ACCEPT |  |  | Fetch-summary only (403). Summary returned the sentence verbatim. |
| C12 | ACCEPT |  |  | Fetch-summary only (403). "Our agreements are no less than one year" returned verbatim. |
| C13 | REVISE |  | The pricing page lists two tiers, Advanced and Power, for every product (Asset, Engage, Visitor, Workplace, Serraview), with no prices. Tier columns are unlabelled and tick images have no alt text; the WebFetch summariser inferred (not read) that "Access to the extensive API and Developer Portal" is ticked in both Engage tiers. Parts of the page copy are "Lorem ipsum" placeholders. | Re-fetched via WebFetch (403 to curl). Summariser explicitly said the tick-to-tier mapping is an inference. Treat tier-gating as unverified. |
| C14 | ACCEPT |  |  | Fetch-summary only; rows confirmed, both two-tick (inferred both tiers). |
| C15 | ACCEPT |  |  | Fetch-summary only. Both rows show a single tick in an unlabelled column, so they are probably tier-gated to one tier (unknown which). Claim as written ("rows include") is fine; draft must not imply availability in both tiers. |
| C16 | ACCEPT |  |  | Source resolves and supports the claim. |
| C17 | ACCEPT |  |  | Source resolves and supports the claim. |
| C18 | ACCEPT |  |  | Source resolves and supports the claim. |
| C19 | ACCEPT |  |  | Source resolves and supports the claim. |
| C20 | ACCEPT |  |  | Source resolves and supports the claim. |
| C21 | ACCEPT |  |  | Source resolves and supports the claim. |
| C22 | ACCEPT |  |  | Source resolves and supports the claim. |
| C23 | ACCEPT |  |  | Source resolves and supports the claim. |
| C24 | ACCEPT |  |  | Source resolves and supports the claim. |
| C25 | ACCEPT | STALE |  | Source resolves and supports the claim. |
| C26 | ACCEPT | STALE |  | Source resolves and supports the claim. |
| C27 | ACCEPT |  |  | Source resolves and supports the claim. |
| C28 | ACCEPT |  |  | Source resolves and supports the claim. |
| C29 | ACCEPT |  |  | Page adds nuance: Open Space Types are shown as Engage workspace types, but their spaces are pulled in only if the function is changed to Bookable/Assignable/Flexi. |
| C30 | ACCEPT |  |  | Source resolves and supports the claim. |
| C31 | ACCEPT |  |  | Page also lists Portfolio "Region" as "Not used" in Engage. |
| C32 | ACCEPT |  |  | Source resolves and supports the claim. |
| C33 | ACCEPT |  |  | Source resolves and supports the claim. |
| C34 | ACCEPT |  |  | Source resolves and supports the claim. |
| C35 | ACCEPT |  |  | Source resolves and supports the claim. |
| C36 | ACCEPT |  |  | Source resolves and supports the claim. |
| C37 | ACCEPT |  |  | Source resolves and supports the claim. |
| C38 | ACCEPT |  |  | Source resolves and supports the claim. |
| C39 | ACCEPT |  |  | Source resolves and supports the claim. |
| C40 | ACCEPT |  |  | Source resolves and supports the claim. |
| C41 | ACCEPT |  |  | Source resolves and supports the claim. |
| C42 | ACCEPT |  |  | Source resolves and supports the claim. |
| C43 | ACCEPT |  |  | Source resolves and supports the claim. |
| C44 | ACCEPT |  |  | Source resolves and supports the claim. |
| C45 | ACCEPT |  |  | Source resolves and supports the claim. |
| C46 | ACCEPT |  |  | Source resolves and supports the claim. |
| C47 | ACCEPT |  |  | Source resolves and supports the claim. |
| C48 | ACCEPT |  |  | Source resolves and supports the claim. |
| C49 | ACCEPT |  |  | Source resolves and supports the claim. |
| C50 | ACCEPT |  |  | Source resolves and supports the claim. |
| C51 | REVISE |  | The Eptura Public API page lists Asset, Authentication, Floor Plan (building/floor info, publish floor-plan data), HR (Person and Group management), Space and Visitor APIs. No Booking API is listed. | Claim itself is accurate but incomplete; the page title is "Eptura APIs OLD" (2026-11-09), so it may be superseded. Draft §0 adds a "Booking" package that is not on the page. |
| C52 | ACCEPT |  |  | Same "OLD" page as C51. |
| C53 | ACCEPT |  |  | Source resolves and supports the claim. |
| C54 | ACCEPT |  |  | Source resolves and supports the claim. |
| C55 | ACCEPT |  |  | Source resolves and supports the claim. |
| C56 | ACCEPT |  |  | Source resolves and supports the claim. |
| C57 | ACCEPT |  |  | Source resolves and supports the claim. |
| C58 | ACCEPT |  |  | Page also shows the "Send to iOffice" button text cited by the draft. |
| C59 | ACCEPT |  |  | Source resolves and supports the claim. |
| C60 | ACCEPT |  |  | Source resolves and supports the claim. |
| C61 | ACCEPT | STALE |  | Page also lists !FURN, !WKST, !PANELS, !FILES, supporting draft §4. Page last edited 2023-06-26 but is linked from the 2026 SoftSpace page. |
| C62 | ACCEPT | STALE |  | Source resolves and supports the claim. |
| C63 | ACCEPT | STALE |  | Source resolves and supports the claim. |
| C64 | ACCEPT | STALE |  | Supported: "All workspace numbers should be located on a single layer" and must match the occupant data spreadsheet. |
| C65 | ACCEPT | STALE |  | Source resolves and supports the claim. |
| C66 | ACCEPT |  |  | Source resolves and supports the claim. |
| C67 | ACCEPT |  |  | Source resolves and supports the claim. |
| C68 | ACCEPT |  |  | Source resolves and supports the claim. |
| C69 | ACCEPT |  |  | Source resolves and supports the claim. |
| C70 | ACCEPT | STALE |  | Source resolves and supports the claim. |
| C71 | ACCEPT | STALE |  | Source resolves and supports the claim. |
| C72 | ACCEPT | STALE |  | Source resolves and supports the claim. |
| C73 | ACCEPT | STALE |  | Source resolves and supports the claim. |
| C74 | ACCEPT | STALE |  | Source resolves and supports the claim. |
| C75 | ACCEPT |  |  | Source resolves and supports the claim. |
| C76 | ACCEPT |  |  | help.archibus.com Swagger HTML resolves (HTTP 200); buildingId/floorId are required query params. |
| C77 | ACCEPT |  |  | help.archibus.com Swagger HTML resolves (HTTP 200); buildingId/floorId are required query params. |
| C78 | ACCEPT |  |  | help.archibus.com Swagger HTML resolves (HTTP 200); buildingId/floorId are required query params. |
| C79 | ACCEPT |  |  | Source resolves and supports the claim. |
| C80 | ACCEPT |  |  | Source resolves and supports the claim. |
| C81 | ACCEPT |  |  | Source resolves and supports the claim. |
| C82 | ACCEPT | STALE |  | Source resolves and supports the claim. |
| C83 | ACCEPT | STALE |  | Source resolves and supports the claim. |
| C84 | ACCEPT | STALE |  | Source resolves and supports the claim. |
| C85 | ACCEPT |  |  | Page HTML title "Microsoft Azure API Management - developer portal"; Eptura Engage text confirmed. |
| C86 | ACCEPT |  |  | Source resolves and supports the claim. |
| C87 | ACCEPT |  |  | Source resolves and supports the claim. |
| C88 | ACCEPT | STALE |  | Source resolves and supports the claim. |
| C89 | ACCEPT | STALE |  | Source resolves and supports the claim. |
| C90 | ACCEPT |  |  | Source resolves and supports the claim. |
| C91 | ACCEPT |  |  | Source resolves and supports the claim. |
| C92 | ACCEPT |  |  | Source resolves and supports the claim. |
| C93 | ACCEPT |  |  | Source resolves and supports the claim. |
| C94 | ACCEPT |  |  | Source resolves and supports the claim. |
| C95 | ACCEPT |  |  | Source resolves and supports the claim. |
| C96 | ACCEPT |  |  | Source resolves and supports the claim. |
| C97 | ACCEPT |  |  | Source resolves and supports the claim. |
| C98 | ACCEPT |  |  | Source resolves and supports the claim. |
| C99 | ACCEPT |  |  | Source resolves and supports the claim. |
| C100 | ACCEPT | STALE |  | C# sample: DecryptText(DynamicParamA, clientId.ToUpper().Substring(0,16), clientId.ToUpper()) with AesCryptoServiceProvider; key and IV are the upper-cased ClientId. |
| C101 | ACCEPT |  |  | Source resolves and supports the claim. |
| C102 | ACCEPT |  |  | Resource payload: ResourceActionType 0 CREATE / 1 UPDATE / ... for Room/Desk. |
| C103 | ACCEPT | STALE |  | "Paid" in draft §6 is an inference from "Subscription to Eptura Engage Sensor Integration". |
| C104 | ACCEPT | STALE |  | Source resolves and supports the claim. |
| C105 | ACCEPT | STALE |  | Source resolves and supports the claim. |
| C106 | ACCEPT | STALE |  | Source resolves and supports the claim. |
| C107 | ACCEPT | STALE |  | Source resolves and supports the claim. |
| C108 | ACCEPT | STALE |  | Source resolves and supports the claim. |
| C109 | ACCEPT | STALE |  | Source resolves and supports the claim. |
| C110 | ACCEPT | STALE |  | Source resolves and supports the claim. |
| C111 | ACCEPT |  |  | Source resolves and supports the claim. |
| C112 | ACCEPT |  |  | Source resolves and supports the claim. |
| C113 | ACCEPT | STALE |  | Source resolves and supports the claim. |
| C114 | ACCEPT | STALE |  | Source resolves and supports the claim. |
| C115 | ACCEPT | STALE |  | Production values; UAT uses ssouat.condecosoftware.com / PING-CONDECO-UAT. |
| C116 | ACCEPT | STALE |  | Source resolves and supports the claim. |
| C117 | ACCEPT | STALE |  | Source resolves and supports the claim. |
| C118 | ACCEPT | STALE |  | Source resolves and supports the claim. |
| C119 | ACCEPT |  |  | Source resolves and supports the claim. |
| C120 | ACCEPT |  |  | Source resolves and supports the claim. |
| C121 | ACCEPT |  |  | Source resolves and supports the claim. |
| C122 | ACCEPT |  |  | Source resolves and supports the claim. |
| C123 | ACCEPT |  |  | Source resolves and supports the claim. |
| C124 | ACCEPT |  |  | Source resolves and supports the claim. |
| C125 | ACCEPT |  |  | Source resolves and supports the claim. |
| C126 | ACCEPT |  |  | Source resolves and supports the claim. |
| C127 | ACCEPT |  |  | Source resolves and supports the claim. |
| C128 | ACCEPT |  |  | Source resolves and supports the claim. |
| C129 | ACCEPT |  |  | Source resolves and supports the claim. |
| C130 | ACCEPT |  |  | Source resolves and supports the claim. |
| C131 | ACCEPT | STALE |  | Source resolves and supports the claim. |
| C132 | ACCEPT |  |  | Source resolves and supports the claim. |
| C133 | ACCEPT | STALE |  | Page says "HMAC-256" and (typo) "HMAC-SHA265"; HMAC-SHA256 is the reasonable reading. Methods are GET or POST. |
| C134 | ACCEPT | STALE |  | Source resolves and supports the claim. |
| C135 | ACCEPT | STALE |  | Source resolves and supports the claim. |
| C136 | ACCEPT |  |  | Source resolves and supports the claim. |
| C137 | ACCEPT |  |  | Source resolves and supports the claim. |
| C138 | ACCEPT |  |  | Source resolves and supports the claim. |
| C139 | ACCEPT |  |  | Source resolves and supports the claim. |
| C140 | ACCEPT | STALE |  | Source resolves and supports the claim. |
| C141 | ACCEPT | STALE |  | Source resolves and supports the claim. |
| C142 | ACCEPT | STALE |  | Source resolves and supports the claim. |
| C143 | ACCEPT |  |  | Source resolves and supports the claim. |
| C144 | ACCEPT |  |  | Source resolves and supports the claim. |
| C145 | ACCEPT |  |  | Source resolves and supports the claim. |
| C146 | ACCEPT |  |  | Source resolves and supports the claim. |
| C147 | ACCEPT |  |  | Source resolves and supports the claim. |
| C148 | ACCEPT | STALE |  | Source resolves and supports the claim. |
| C149 | ACCEPT | STALE |  | Source resolves and supports the claim. |
| C150 | ACCEPT |  |  | Source resolves and supports the claim. |
| C151 | REVISE | STALE | Older KC security page (2024-02-06): ISO 27001 "Certified"; SSAE 16 SOC 2 listed as "SOC 2 certification provided by AWS and Azure"; GDPR "Compliant". The page does not say whether Eptura holds its own SOC 2 report. | The claim and draft §8 ("i.e. Eptura holds no SOC 2 report of its own") overstate the page. The 2026 trust portal (C150) lists no SOC 2 at all. A quick search found no Eptura SOC 2 report either way. |
| C152 | ACCEPT | STALE |  | 2024 page; Engage analytics is on Azure (C149), so the "AWS S3" backup statement may not apply to every product. |
| C153 | ACCEPT | STALE |  | Source resolves and supports the claim. |
| C154 | ACCEPT |  |  | Live status page; components confirmed, including Teem and SpaceIQ, and Serraview sub-components Space Planning, Core Services, Engage, Insights, Public API, Floorplan. |
| C155 | ACCEPT |  |  | Webhook, Slack, RSS and Atom all present. |
| C156 | ACCEPT |  |  | Source resolves and supports the claim. |
| C157 | ACCEPT |  |  | Source resolves and supports the claim. |
| C158 | ACCEPT |  |  | Table check: the mobile app has zh-CN, en-US, fr, de, it, ja, pt-BR, es (no en-GB). Room Screen v3 adds ar-SA. |
| C159 | ACCEPT |  |  | 49 of 71 repos are forks; most recent push 2023-12-15. |
| C160 | ACCEPT |  |  | Created 2023-02-10, 0 public repos. |
| C161 | ACCEPT |  |  | MIT, pushed 2026-08-31, 12 stars. |
| C162 | ACCEPT | STALE |  | Source resolves and supports the claim. |
| C163 | ACCEPT | STALE |  | Source resolves and supports the claim. |
| C164 | ACCEPT |  |  | Source resolves and supports the claim. |

## Unsourced draft statements (all REJECT)
1. §0 table, Workplace "Heritage: iOFFICE (+ SpaceIQ heritage)". No source links SpaceIQ to Workplace. The history page says the iOffice + SpaceIQ *company* rebranded, but only the iOffice *product* was renamed Workplace (N2). SpaceIQ is still a separate product (C140, C154). Drop "+ SpaceIQ heritage".
2. §0 paragraph "No single page states the rename outright." Disproved by N1 and N2. Replace it with a citation to N1/N2.
3. §0 table, Serraview "with its own 'Serraview Engage' booking module" and "a Core API". Not in the ledger. A "Serraview Engage" KC tree exists (N8), but no "Core API" was found (the status page has "Core Services"). Source the first part with N8 and drop "Core API".
4. §0 table, platform row "Public API ... Floor Plan, Space, HR, Visitor, Asset, Booking packages". There is no Booking API on the page (C51 revised). Also, "Directory, Device Hub" as platform components: Directory is supported by the platform-services page, but Device Hub is an Engage tool (C112) and is not listed as a platform service.
5. §0 table, Engage heritage "(+ Teem room screens on the status page)". The status page lists Teem as a separate component, and nothing ties Teem room screens to Engage.
6. §1 "Positioning: 'Transform the way you manage your physical world.' [C8]". The phrase is not in C8's quote. A search shows it is the /our-platform/ headline, so the statement is TRUE, but it needs a ledger entry or a new quote under C8.
7. §1 "Segment: enterprise". This is analysis with no source; label it as analysis.
8. §8 "i.e. Eptura holds no SOC 2 report of its own". This overstates C151 (see the revision).
9. §8 "Not publicly documented: DPA, subprocessor list". Disproved: the trust portal links a Data Processing Addendum and a Sub-processors list (N3).
10. §4 "PDF import is not publicly documented" and "Auto-detection ... not publicly documented in any product. Every pipeline needs pre-layered, closed-polyline CAD". Partly disproved: SpaceIQ accepts .DWG/.PDF/.PNG floor-plan sources and Teem has a PDF floor-plan submission flow (N4). These are service- or raster-based, not auto-detection, so the auto-detection statement can stay, but "every pipeline needs pre-layered CAD" is false for SpaceIQ, Teem and Engage raster.
11. §13 "Zapier, Power Automate, Make: not publicly documented". Disproved for Zapier: Teem (an Eptura product still on the status page) has a documented Zapier integration (N5).
12. §15 "A customer-facing MCP server endpoint is not publicly documented". Disproved: Archibus documents a deployable single-tenant read-only MCP server with 69 tools over Streamable HTTP (N6).
13. §7 "Regions / data residency: not publicly documented beyond the westeurope Device Hub hostname". Disproved for Serraview, which documents customer-selectable AWS hosting regions (N7). It is still undocumented for Engage and Workplace as far as checked.

Minor citation and wording fixes (not rejects):
- §2 Equipment cites `Vendor_GetVendorItems_V1` to [C90]. The page supports it, but C90's claim text does not mention it.
- §2 Meeting rooms "seating styles in Engage [C32]": the page supports it (resource admin seating styles), but C32's text does not mention it.
- §6 "Sensor Integration is a paid subscription": "paid" is inferred.
- §6 Kiosks/displays "Room Screen v2/v3": only v3 appears in the C158 table.
- §5 "Workplace has Automated User Provisioning (page title only)", §13 ServiceNow pages and §14 "SpaceIQ API changes last logged in 2021 (page titles only)" are unledgered. A SiQ_API_Changes_2021July08.pdf file exists in the KC search results, which is consistent with the 2021 date.
- §10 "Engage OpenAPI rendered in APIM but not publicly downloadable" was not verified (the portal needs login). Label it as an assumption.

## Data-model trace issues (§12)
- Platform row: "Usage (Me/We/Common/None)" and "Space function (... Retained/Shared)" go beyond C28/C29 as written. The page does support Common/None and lists "Retained Shared" fields, so widen C28/C29 or add a claim. The "Region→Site" relation is supported by the page (Region is the parent level; "Not used" in Engage), not by C31's quote. Add it to C31.
- Engage Resource: the fields "status, capacity, attributes" are not in C30/C32/C101 (only the name, space type, default capacity, usage and ResourceItemID are). The "Group→Resource (booking rules)" relation is implied by C23 but not cited.
- Engage Visitor: "VisitorId, Action (create...undeparted)" is supported by the 030-Notification_messages page (ARRIVED/UNARRIVED/DEPARTED...), which is not the cited C90/C93. Cite the C101/C102 page. "Visitor→Booking" is INFERRED and labelled as such. That is OK, but it is not an accepted relation.
- Workplace Room "type" field: C128 shows only id/floor/reservable/name. Drop "type" or cite the API page's query params.
- Workplace Reservation: "id, notes, dateUpdated" are not in C129 (the page shows notes; dateUpdated appears on User, not Reservation). Drop dateUpdated.
- Workplace Neighborhood fields "code, color, borderColor, fillOpacity, owner, center" are on the API page (example JSON), but C130 cites only the endpoint. Widen C130.
- SpaceIQ Floor "id, name, code, areaSize, spaces[]": present on developer.spaceiq.com (example query), but C138 cites only url/interactiveMap. Widen C138.
- All other entities and relations trace to accepted claims.

## New claims
| id | verdict | claim | url | quote |
|---|---|---|---|---|
| N1 | ACCEPT | iOffice was renamed Eptura Workplace in the July 2023 Workplace release (released 19 July 2023). | https://knowledge.eptura.com/Workplace/Product_Information/Product_Releases_Archive/2023_Product_Releases/2023_July_Eptura_Workplace_Product_Release | "July's release includes: iOffice is Eptura Workplace Released July 19th To unify your Eptura platform experience..." (page edited 2025-02-26) |
| N2 | ACCEPT | Eptura's history page states the rename directly. | https://eptura.com/resource/eptura-history/ | "In 2023 the iOffice product was renamed Eptura Workplace." (via WebFetch; 403 to curl) |
| N3 | ACCEPT | The trust portal publishes a Data Processing Addendum and a Sub-processors list. | https://security.eptura.com/ | tiles "Data Processing Addendum" -> https://eptura.com/terms/dpa/ ; "Sub-processors" -> https://eptura.com/terms/subprocessors/ (created 2026-07-22) |
| N4 | ACCEPT | SpaceIQ floor plans can be prepared from .DWG, .PDF or .PNG and are published as PNG; Teem floor plans are submitted as PDF. | https://knowledge.eptura.com/SpaceIQ/Admins/Manage_Overview/Manage_Buildings%2F%2FFloors/006_Upload_a_Floorplan ; https://knowledge.eptura.com/Teem/Admin/Floorplans_Admin/Prepare_and_Submit_a_Floor_Plan | "The floor plan will be published as a PNG image uploaded into SpaceIQ... source file: .DWG .PDF .PNG" / "source the floor plan file in a PDF format" (2025-02-26 / 2022-10-10, STALE) |
| N5 | ACCEPT | Teem has a documented Zapier integration. | https://knowledge.eptura.com/Teem/Integrations/Custom_Integration/Zapier_Integration | "Using Zapier, Teem users can automate a number of tasks that they now perform manually." (2024-11-03, STALE) |
| N6 | ACCEPT | Archibus ships a deployable single-tenant, read-only MCP server (69 tools) that proxies to WebCentral REST /api/v1/mcp/... via OAuth2 client credentials. | https://knowledge.eptura.com/Archibus/Archibus_System_Help/Installation/Deploy_AI_Integration | "mcp-server - a single-tenant, read-only MCP server. It exposes 69 read-only tools over Archibus/WebCentral data" (2026-07-22) |
| N7 | ACCEPT | Serraview is hosted on AWS with customer-selectable regions: Sydney, Canada Central, Frankfurt, Singapore and US East. | https://knowledge.eptura.com/Serraview/Product_Information/Our_Technology/Explore_the_Serraview_Technology_Platform/003Serraview_Cloud_Hosting | "We allow customers to nominate the Hosting Region that the Serraview system is hosted from" (2022-08-24, STALE) |
| N8 | ACCEPT | Serraview has a "Serraview Engage" module and a Serraview-to-Eptura Engage integration that makes Serraview the authority for HR and location data. | https://knowledge.eptura.com/Serraview/Integrations/Eptura_Engage_Integration | "This integration makes Serraview the authoritative source for HR and location data" (2026-09-07); the KC also has a /Serraview/Serraview_Engage/ tree |
| N9 | ACCEPT | "Transform the way you manage your physical world" is the headline on the Eptura platform page. | https://eptura.com/our-platform/ | search snippet: "Transform the way you manage your physical world. Connect people, places, assets, and work on a single platform." |
