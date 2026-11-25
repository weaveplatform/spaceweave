# Validation: Robin (robinpowered.com)

Validated 2026-11-25 by an independent checker. Every ledger URL was fetched again. Help-centre HTML (`/hc/en-us/articles/{id}`) returns 403 to automated fetches, so article bodies were checked through the public Zendesk JSON endpoint (`/api/v2/help_center/en-us/articles/{id}.json`), and the title and `updated_at` were confirmed there. Citations should keep the human-readable `/hc/en-us/articles/{id}` URL. GitHub claims were checked through `gh api`.

## Summary counts
| Verdict | Count |
|---|---|
| ACCEPT | 92 |
| REVISE | 4 (C46, C57, C59, C94) |
| REJECT | 0 ledger claims (7 draft statements are rejected as UNSOURCED or unsupported, listed below) |
| NEW | 9 (N1–N9) |

Flags:
- **STALE (×14):** all ReadMe docs pages with `updatedAt` 2025-11-06, more than 12 months old (C14–C21, C23, C48, C69–C72, C76), plus C61 (2022 blog).
- **Undated official page (×7):** robinpowered.com/security (C57, C58, C60, C62, C63) carries no date. Treat it as possibly stale, because its "all customer data on AWS" wording conflicts with newer sources (see C57).

## Most serious problems
1. **Hosting (C57, §7).** "Cloud provider: AWS" is incomplete. The security page (undated) says "All customer data is stored on AWS services". But the privacy policy (last updated 2025-07-29) lists **both** "Amazon Web Services Inc. – Cloud service provider" **and** "Google LLC – Cloud Services" as sub-processors (N1). The status page lists a "Google Cloud Platform Google Cloud SQL" component (C91), and AI conversations live in GCP us-east4/europe-west3 (C65). Robin is best described as **AWS plus Google Cloud (multi-cloud)**. The split between the two (which workloads run on which) is not publicly documented. The draft's "Discrepancy" framing should become "uses both".
2. **"A full subprocessor list is not publicly readable" (§8) is false.** The privacy policy at https://robinpowered.com/privacy has a sub-processor table (N1).
3. **"Free tier / trial: not publicly documented" (§1) is partly false.** The help centre documents Scheduling and Desks **trials** (N2). There is still no evidence of a free tier.
4. **No outbound webhooks (§11): CONFIRMED.** A full-text help-centre search for "webhook" returns exactly 2 articles. Both describe inbound or Slack-channel uses: the Slack "Channel for webhook" setting and the Universal Presence Connector "Webhook URL" (article 49224863277197). The docs llms.txt index has no webhook entry, the Analytics API is poll-only, and web searches found nothing. The statement stands as "not publicly documented".
5. **Draft-only assertions with no source:**
   - "virtual closet" (§2);
   - "Admin-only and role-scoped restrictions" for meeting rooms (§2);
   - "status page links to updates.robinpowered.com" (§14), which the fetch did not find;
   - "Legacy plans are kept until retired with notice" (§14);
   - "CSV import" for provisioning (§5).

## Claim table
| id | verdict | flags | corrected text (for REVISE) | note |
|---|---|---|---|---|
| C1 | ACCEPT | | | `gh api orgs/robinpowered` gives location "Boston, MA" and public_repos 88. |
| C2 | ACCEPT | | | Quote verbatim on /pricing. |
| C3 | ACCEPT | | | Verbatim. |
| C4 | ACCEPT | | | Verbatim. The same page also says Robin may fit orgs with "more than 150 hybrid employees", so the segment threshold is soft. |
| C5 | ACCEPT | | | Verbatim. |
| C6 | ACCEPT | | | Article title "1) Quick start guide for Robin (Essentials & Business plans)", updated 2026-10-20. |
| C7 | ACCEPT | | | "frees a space license". |
| C8 | ACCEPT | | | Verbatim, updated 2026-11-20. |
| C9 | ACCEPT | | | Requirement and Read/Write scopes confirmed. The article does not distinguish org from personal tokens. |
| C10 | ACCEPT | | | Verbatim. |
| C11 | ACCEPT | | | Verbatim. |
| C12 | ACCEPT | | | Table: hot "Up to 1 day / The same day only"; hotel "1 to 5 days or more / In advance". |
| C13 | ACCEPT | | | Verbatim. |
| C14 | ACCEPT | STALE | | Desk types are arrays: `["hot"]`, `["hoteled"]`, `["assigned"]` (struck through), `["assigned","shared"]`, `["hoteled","shared"]`. updatedAt 2025-11-06. |
| C15 | ACCEPT | STALE | | Verbatim. |
| C16 | ACCEPT | STALE | | `space_id`, `zone_id`, `zone`, `is_reservable`, `disabled_at`, `seat_amenities` all present. |
| C17 | ACCEPT | STALE | | Verbatim, with the RRULE example. |
| C18 | ACCEPT | STALE | | Verbatim. |
| C19 | ACCEPT | STALE | | Verbatim. |
| C20 | ACCEPT | STALE | | `POST /v1.0/seats/{id}/reservations`. The enum spells it `reverse_hotel`, which is inconsistent with `reverse_hoteled` on the Reservation page. `reserver_id` is required for org tokens. The response includes `series_id` and `recurrence_id`. |
| C21 | ACCEPT | STALE | | PUT path, body `user_id` and `device_id`. Response has `seat_reservation_id` and `confirmed_at`. |
| C22 | ACCEPT | | | Note: the request-only option appears only when the org uses meeting room approvals. |
| C23 | ACCEPT | STALE | | Default types include `null`. |
| C24 | ACCEPT | | | Also: "Allow Overlapping Bookings" and role restriction confirmed. |
| C25 | ACCEPT | | | Verbatim. |
| C26 | ACCEPT | | | Template sentence confirmed. **The article does not mention a "virtual closet".** |
| C27 | ACCEPT | | | Verbatim. |
| C28 | ACCEPT | | | Paraphrased. The live article ("Create and manage neighborhoods") says "Neighborhoods organize the office floor plan visually…", "Work Areas created in the Layout tool are Neighborhoods, and you manage them in the Assignments tool." |
| C29 | ACCEPT | | | Verbatim. |
| C30 | ACCEPT | | | Verbatim. |
| C31 | ACCEPT | | | "[12/3/24] Priority desk booking policies", created 2024-12-30. The feature is still current: see the "Priority desk booking feature" article 30135255820557. |
| C32 | ACCEPT | | | Verbatim. |
| C33 | ACCEPT | | | Applies to hot and hotel desks only. |
| C34 | ACCEPT | | | "It uses badge swipes and the office Wi-Fi" also confirmed. |
| C35 | ACCEPT | | | The "Local check-in only" policy is confirmed in the same article. |
| C36 | ACCEPT | | | DWG is not mentioned. A help-centre search for "DWG" returns 0 results. |
| C37 | ACCEPT | | | Verbatim. |
| C38 | ACCEPT | | | Verbatim. |
| C39 | ACCEPT | | | Verbatim. |
| C40 | ACCEPT | | | The article says "Robin's design team starts to convert them". "Manually" is a reasonable reading but is not the article's word. |
| C41 | ACCEPT | | | Snapping and the Shift 15° lock confirmed. |
| C42 | ACCEPT | | | Verbatim. |
| C43 | ACCEPT | | | Verbatim. |
| C44 | ACCEPT | | | 3 months for both. The article does not state whether sync is two-way. |
| C45 | ACCEPT | | | "You need a Microsoft global admin" confirmed. |
| C46 | REVISE | | "Microsoft 365 (Exchange Online) connects through Microsoft Graph API; on-prem Exchange room calendars are accessed via impersonation (UserImpersonation per room calendar), not delegate access." | The article does not use the term "EWS". It is implied by ApplicationImpersonation but not stated. |
| C47 | ACCEPT | | | Okta, Entra ID and OneLogin named. Enterprise attributes confirmed. |
| C48 | ACCEPT | STALE | | `groups` appears only in the response schema, not the request. |
| C49 | ACCEPT | | | Guides: Google Workspace, Okta, Delinea, ADFS, Entra ID and Rippling. |
| C50 | ACCEPT | | | The fee sentence sits under "Custom integration process and service fees". The article does not say whether the 7 listed integrations carry fees. |
| C51 | ACCEPT | | | Verbatim. |
| C52 | ACCEPT | | | Full endpoint and EU host confirmed. |
| C53 | ACCEPT | | | LOCATION or LEVEL only. No space-level or desk-level target. |
| C54 | ACCEPT | | | A personal token of an Admin or Owner also works. An org token is recommended. |
| C55 | ACCEPT | | | Device list confirmed. The suggested status board hardware is Neat Board 50 or a TV. |
| C56 | ACCEPT | | | The third type is "Scheduling status board (list view for spaces only)". |
| C57 | REVISE | undated | "Robin's security page says all customer data is stored on AWS, and customers choose a U.S. or European data center. Its privacy policy (updated 2025-07-29) lists both Amazon Web Services (cloud service provider) and Google LLC (cloud services) as sub-processors, and the status page lists Google Cloud SQL, so hosting spans AWS and Google Cloud; the workload split is not publicly documented." | Combine with N1 and C91. The security page is undated and probably predates the GCP components. |
| C58 | ACCEPT | undated | | Verbatim, but subject to the C57 caveat. |
| C59 | REVISE | | "Robin's primary service cluster is in the US; an EU data server is offered. Not available on EU: Slack Enterprise Grid, the Robin Admin mobile app, SSO/SCIM for Azure AD and OneLogin, and Analytics Executive Insights and AI assistant. EU-hosted orgs also cannot connect AI assistants (MCP) (N5)." | Adds the analytics exclusions that the ledger omitted. |
| C60 | ACCEPT | undated | | Verbatim. |
| C61 | ACCEPT | STALE | | 2022-07-19. SOC 2 type not stated. Note: the security page wording "hosted by data centers with … ISO 27001 and SOC2" refers to the data centres, not Robin. |
| C62 | ACCEPT | | | The trustshare page returned only the title "TrustShare". |
| C63 | ACCEPT | undated | | Verbatim. |
| C64 | ACCEPT | | | Verbatim. |
| C65 | ACCEPT | | | us-east4 and europe-west3 confirmed (Vertex AI). |
| C66 | ACCEPT | | | Admin tool. |
| C67 | ACCEPT | | | Verbatim. |
| C68 | ACCEPT | | | Verbatim. |
| C69 | ACCEPT | STALE | | Verbatim. |
| C70 | ACCEPT | STALE | | Verbatim. |
| C71 | ACCEPT | STALE | | Verbatim. |
| C72 | ACCEPT | STALE | | All three deprecations confirmed. |
| C73 | ACCEPT | | | Fields `client_id`, `client_secret`, `user`. token_type Bearer. |
| C74 | ACCEPT | | | Confirmed. |
| C75 | ACCEPT | | | `page_size` 1–100, default 20. |
| C76 | ACCEPT | STALE | | Defaults page=1, per_page=10. |
| C77 | ACCEPT | | | Also confirmed on reserve-desks. |
| C78 | ACCEPT | | | Headings match. The Overview group also holds Analytics API and Universal Presence Connector. No webhook, level, parking, locker or map entries. |
| C79 | ACCEPT | | | 30 POSTs/min (429). Poll at 10-second intervals. |
| C80 | ACCEPT | | | Types: activity-summary, custom-resources, desks, presence, hybrid-work-policy, people-roster, spaces. |
| C81 | ACCEPT | | | Verbatim. |
| C82 | ACCEPT | | | The page says "archived by the owner on Nov 11, 2026". Last push 2025-11-04. |
| C83 | ACCEPT | | | `PlacesService.java` confirms `/organizations/{id}/locations`, `/locations/{id}/spaces`, and `/spaces/{id}/presence` (GET/POST/DELETE). Space and Location model fields confirmed. |
| C84 | ACCEPT | | | "OAuth 2.0 with the Dynamic discovery type" is stated for Copilot Studio specifically. Clients confirmed: Claude, ChatGPT, Copilot Studio, Gemini (Antigravity CLI / Gemini Enterprise). |
| C85 | ACCEPT | | | Verbatim. |
| C86 | ACCEPT | | | 38 is a count of tool sections, not a number stated on the page. `get_building` and `get_draft_data` return neighborhoods with desk membership but no geometry. |
| C87 | ACCEPT | | | Verbatim. |
| C88 | ACCEPT | | | Verbatim. |
| C89 | ACCEPT | | | VPAT confirmed. |
| C90 | ACCEPT | | | Verbatim. |
| C91 | ACCEPT | | | Component "Google Cloud Platform Google Cloud SQL" is present. No AWS component. Footer "Powered by Atlassian Statuspage". |
| C92 | ACCEPT | | | Product-area list confirmed. Calendars are listed as "Exchange, Microsoft 365 and Google". |
| C93 | ACCEPT | | | Verbatim. |
| C94 | REVISE | | "Robin's Slack app is compatible with Slack and Slack Enterprise Grid. Robin also integrates with Slack and Microsoft Teams for meeting and desk-reservation notifications and booking actions (platform overview, 4416639332621)." | The Slack article (360051881352) does not mention Teams or the Teams store. |
| C95 | ACCEPT | | | Verbatim. |
| C96 | ACCEPT | | | Verbatim. |

## Unsourced statements in the draft (verdict: REJECT unless noted)
| § | Statement | Finding |
|---|---|---|
| §1 | "Free tier / trial: not publicly documented on official pages" | **REJECT as written.** Trials are documented (N2). Rewrite as "no free tier documented; time-limited Scheduling/Desks trials exist". |
| §1 | "The pricing page offers only a demo or tour" | Supported on refetch. Now backed by N7. |
| §1 | SaaSworthy "Basic $1,500/yr" | REJECT. Unverified third-party snippet, and the draft already disclaims it. Remove, or keep only as a labelled pointer. |
| §2 | Meeting rooms: "Admin-only and role-scoped restrictions are available" [C22] | REJECT. C22's article does not say this. |
| §2 | Car parking: "Local check-in supported" | REJECT as cited. Weakly inferable from the QR article ("any resource for check-ins and bookings"), but not stated for parking. |
| §2 | Equipment: "A 'virtual closet' holds mobile items [C26]" | REJECT. Not in the cited article. |
| §2 | Lockers/equipment check-in "yes" for equipment | REJECT. Only lockers (C25) are sourced. |
| §4 | "Drawing a map from scratch in the Map Editor" | No ledger claim, but supported by 11379164575629 ("Draw your own map"). Add a claim. |
| §4 | GitHub forks maplibre-gl-native, fontnik, spritezero, geojson-precision (INFERRED map stack) | Repos verified to exist via `gh api`. All are forks, and all but fontnik are archived. No ledger claim. The inference stays unconfirmed, so keep the INFERRED label and add a ledger row. |
| §5 | Provisioning by "CSV import" | REJECT (no source). |
| §5 | "HRIS: a Rippling integration exists (SAML guide). No other HRIS connectors are documented." | Rippling is documented as a SAML provider, not an HRIS provisioning connector. The platform overview mentions "HRIS directories" as provisioning sources generically, with no named connector. Rewrite accordingly. |
| §7 | "Cloud provider: AWS" plus the "Discrepancy" bullet | REVISE per C57 and N1. |
| §8 | "backups are kept for 30 days (security page)" | Supported on refetch. Now backed by N6. |
| §8 | "A full subprocessor list is not publicly readable" | **REJECT.** Disproved by N1. |
| §13 | `php-ews`, `php-ntlm`, `recurr` in the org [C1] | C1 covers only the location. The repos are verified via `gh api` (php-ews and recurr are archived forks; php-ntlm is an original repo). Add a ledger row. |
| §13 | Microsoft marketplace listing WA200001761 exists | REJECT (403, unverified). |
| §14 | "status page links to updates.robinpowered.com" | REJECT. Not found on status.robinpowered.com when refetched. |
| §14 | "Legacy plans are kept until retired with notice (help centre)" | REJECT. No article cited. |
| §9 | "Org or personal API tokens" | Not in C70. Supported by the Universal Connector doc (personal Admin/Owner token). Cite C54's page. |
| §6 | "Per-desk or per-room sensor occupancy is not publicly documented" | Mostly stands, with two caveats. The platform overview lists Vergesense sensors as an integration (N3), and the archived SDK had `POST /spaces/{id}/presence` (N9). Rewrite: "the current connector resolves only to building/floor; a sensor integration (Vergesense) is listed without documented granularity". |

## Data-model trace issues (§12)
- **Seat Reservation `series_id` and `recurrence_id`:** these come from the reserve-desks response (C20 URL), not the Reservation page. They are traceable, but `[C20]` should be named explicitly for those fields.
- **"Not exposed in the REST API: Neighborhood, parking, locker, custom resource, map/wall geometry" [C28][C24]:** the cited help articles say nothing about the API. Re-cite to C78 (llms.txt index has no such resources), C86 (MCP has no geometry) and N4 ("Not all functionality … is available or documented in the public API"). Also note that Neighborhoods **are** readable through MCP `get_building`/`get_draft_data` (name, colour slot, desk membership; C86 page).
- **Location (legacy) → Organization:** traceable via `/organizations/{id}/locations` and `Location.accountId` (C83). OK.
- **Desk `zone_id` → Zone, and Zone = Neighborhood:** the draft already marks this as inferred. Zone has no entity of its own in any source, so keep it labelled.
- **SCIM User → Groups:** `groups` exists only in the SCIM response schema (C48 page). C47 supports group push. OK.
- All other entities (Desk, Confirmation, Space free-busy, Space legacy, Visit, Presence event, Hierarchy) trace to accepted claims.

## New claims
| id | verdict | claim | url | quote |
|---|---|---|---|---|
| N1 | ACCEPT | Robin's privacy policy (last updated 2025-07-29) publishes a sub-processor list. It names Amazon Web Services Inc. ("Cloud service provider") and Google LLC ("Cloud Services"), plus Sumo Logic, Intercom, SendGrid, Looker, Gainsight, Sigma and Snowflake. All are US entities. | https://robinpowered.com/privacy | Table rows "Amazon Web Services Inc. \| Cloud service provider \| United States" … "Google LLC* \| Cloud Services \| United States" |
| N2 | ACCEPT | Robin runs time-limited trials of its Scheduling and Desks products. | https://support.robinpowered.com/hc/en-us/articles/360004137872 | "Trial updates: You get an email when your Scheduling or Desks trial is about to expire and when it ends." (updated 2026-09-25) |
| N3 | ACCEPT | Robin lists sensor and display integrations including Vergesense (occupancy sensors). Granularity is not described. | https://support.robinpowered.com/hc/en-us/articles/4416639332621 | "Room and desk displays and sensors, including Embrava, Logitech, Crestron, Vergesense and Neat." |
| N4 | ACCEPT | Robin states that its public API does not cover all app functionality. | https://support.robinpowered.com/hc/en-us/articles/4416639332621 | "Not all functionality in Robin's apps is available or documented in the public API." |
| N5 | ACCEPT | EU-hosted organizations cannot connect AI assistants (MCP). | https://support.robinpowered.com/hc/en-us/articles/49768034531085 (also 49474415235213, 35209911294221) | "Organizations hosted on Robin's EU infrastructure cannot connect AI assistants." |
| N6 | ACCEPT (undated) | The security page states that all customer data is stored on AWS and that backups are kept for 30 days. | https://robinpowered.com/security | "All customer data is stored on AWS services, which follows a strict decommissioning policy" / "We retain backups for 30 days, after which time the data will be completely unobtainable." |
| N7 | ACCEPT | The pricing page offers "Book a demo" and "Take a platform tour". It mentions no free trial or prices. | https://robinpowered.com/pricing | CTA labels "Book a demo", "Take a platform tour" |
| N8 | ACCEPT | Robin integrates with Slack and Microsoft Teams for meeting and desk-reservation notifications. Users can also manage bookings (check in, cancel) from these apps. | https://support.robinpowered.com/hc/en-us/articles/4416639332621 | "Slack and Microsoft Teams, for notifications of upcoming meetings and desk reservations" |
| N9 | ACCEPT | The archived Java SDK modelled space-level presence (`GET/POST/DELETE /spaces/{id}/presence`, also on locations). Space-level presence existed in the legacy API; its current status is undocumented. | https://github.com/robinpowered/robin-java-sdk/blob/master/src/main/java/com/robinpowered/sdk/service/PlacesService.java | `@POST("/spaces/{id}/presence")` |

### "Not publicly documented" checks with no disproof found
| Item | Checks run | Result |
|---|---|---|
| Outbound webhooks | help-centre search "webhook", web searches, docs llms.txt | Stands (see problem 4). |
| Zapier / Power Automate / Make | help-centre search "Zapier" | 0 results; stands. |
| Other HRIS connectors | help-centre search "Workday" | No connector found; stands. |
| Per-user booking quotas | help-centre search | Only reservation-length limits, priority booking windows and hybrid-policy minimums; stands. |
| Developer sandbox | help-centre search | Only an LLM planning "sandbox"; stands. |
| OIDC end-user SSO | help-centre search | None; stands. |
| DWG import | help-centre search "DWG" | 0 results; stands. |
| Cloud-workload split (AWS vs GCP) | `dig` | All robinpowered hosts resolve to Cloudflare edge IPs, so DNS is inconclusive. |
