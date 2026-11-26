# Integration architecture

Every system-to-system integration spaceweave has, which service owns it, the protocol,
and which side starts the call. Azure and GCP options are shown side by side; the product
deploys to either.

Out of scope: Google Workspace Calendar, Slack, and identity providers other than Microsoft Entra ID.

There are two forms of the same architecture:
- **Single view:** every external system around the Kubernetes services, with the managed
  cloud services below.
- **Two pages:** a system context (spaceweave as one box) and a containers view (the
  services inside it and how they are wired). Both pages are in one editable file,
  [`integration-architecture-2-page.drawio`](diagrams/integration-architecture-2-page.drawio), with a
  stacked rendering in [`integration-architecture-2-page.drawio.svg`](diagrams/integration-architecture-2-page.drawio.svg).

## Single view

![Integration architecture, single view](diagrams/integration-architecture.drawio.svg)

Source: [`diagrams/integration-architecture.drawio`](diagrams/integration-architecture.drawio)

## Page 1: system context

![Integration architecture, system context](diagrams/integration-context.drawio.svg)

## Page 2: containers

![Integration architecture, containers](diagrams/integration-containers.drawio.svg)

## Two points of direction

- **SCIM is inbound.** Microsoft Entra ID, the only identity provider, *pushes* users and groups to
  spaceweave's SCIM 2.0 endpoint; spaceweave does not pull. Sign-in (OIDC or SAML) is a
  separate flow from provisioning.
- **AI agents act through MCP and are told through webhooks.** An assistant calls the MCP
  server, which acts as the signed-in user through the public API. Webhooks run the other
  way: spaceweave pushes events out. Agent and workflow platforms (Copilot Studio, Power
  Automate, n8n, Zapier) subscribe to webhooks to trigger a flow, then act back through the
  API or MCP.

## Inventory

### Identity & workforce data

| Integration | Direction | Protocol | Owner | Purpose |
|---|---|---|---|---|
| Microsoft Entra ID sign-in | in | OIDC / SAML, Conditional Access | Web app & API | Staff and admin sign-in; tokens held server-side (see [identity](identity.md)) |
| Microsoft Entra ID provisioning | in (push) | SCIM 2.0 | SCIM endpoint | User and group create, update, deactivate, delete |
| HR system (e.g. Workday) | in (pulled) | REST or file feed | Directory & HR sync | Teams, managers, usual office, leaver dates, which Entra often lacks |
| Public-holiday calendars | in (pulled) | ICS or API | Directory & HR sync | Closures per country and office |

### Calendars & messaging

| Integration | Direction | Protocol | Owner | Purpose |
|---|---|---|---|---|
| Exchange Online room mailboxes and calendars | two-way | Microsoft Graph REST, delta queries, change notifications | Calendar sync | Room booking sync; delegate grants feed `Delegation` |
| Microsoft Places | two-way | Graph Places API | Calendar sync | Rooms and desks known to Outlook and Teams |
| Microsoft Teams | out | Graph / Bot Framework, Adaptive Cards | Notifications | Booking raised, waitlist offer, no-show warning |
| Email | out | Graph sendMail or an SMTP relay | Notifications | Default channel, guest invites |
| Web push | out | Web Push (VAPID) | Notifications | Mobile-web nudges, such as a check-in reminder |

### Channels and automation

| Integration | Direction | Protocol | Owner | Purpose |
|---|---|---|---|---|
| Web and mobile web | in | HTTPS, OIDC session | Web app & API | Staff and admin UI |
| Teams app, Outlook add-in | in | App manifests calling the public API with the user's token | Web app & API | Book where people already work |
| spaceweave displays and kiosks | in | Public API, device credential | Web app & API | spaceweave's own room and desk screens: status, check-in, QR codes |
| Scripts, SDKs, Terraform, workflow tools | in | REST + OpenAPI, OAuth2 client credentials | Web app & API | Automation and infrastructure as code |
| AI assistants (Claude, Microsoft Copilot, ChatGPT) | in | MCP over streamable HTTP, the user's OAuth grant | MCP server | Assistant booking and admin, with published skills |

### Room panels

Third-party room panels never call spaceweave. They read and write the room's **Exchange
room mailbox**, and spaceweave's two-way calendar sync keeps that mailbox and spaceweave in
step. A booking made on a panel reaches spaceweave as a Graph change notification; a booking
made in spaceweave appears on the panel through the mailbox.

| Panel | Where | Calendar path | Notes |
|---|---|---|---|
| Cisco Room Navigator, in room booking mode | Meeting rooms | Webex Hybrid Calendar, configured in Control Hub against Exchange Online | Shared-mode rooms need a resource mailbox of type Room. Hybrid Calendar relies on meeting comments, so Exchange must not strip them. A standalone Navigator needs RoomOS 11.3 or later |
| Crestron scheduling panels (TSS / TSW) | Event spaces | Microsoft Graph to Exchange Online, or through Crestron Fusion | One mailbox per room. Set the panel's time zone before enabling scheduling |

What this asks of spaceweave:
- **Every panelled room or event space is calendar-backed.** It has an Exchange room
  mailbox, and `KindProfile.calendar_backed` is set, so Calendar sync owns it.
- **Panel bookings are external bookings.** They arrive through sync with
  `source: calendar`, and spaceweave's rules (hours, closures, approvals) still apply when it
  reconciles them.
- **Check-in on a panel** (Cisco's room-in-use, Crestron's check-in) is visible to spaceweave
  only as the calendar changes. Treating it as a `CheckIn` would need each vendor's own API
  (the Webex device or workspace APIs, Crestron Fusion), which is not in scope yet.
- **The mix of hardware stays the customer's choice.** Because the panels meet spaceweave
  only at the mailbox, swapping Crestron for Cisco or for anything else that reads Exchange
  changes nothing in spaceweave.

Sources: [Webex: enable room booking for shared-mode devices](https://help.webex.com/en-us/article/nzp31jb/Enable-room-booking-for-shared-mode-Board-and-Room-Series-devices);
[Webex: Room Navigator as a standalone device](https://help.webex.com/iq6aw6/);
[Crestron room scheduling panels operations guide](https://de.crestron.com/getmedia/302bea62-d755-4073-819a-48e33644c244/mg_og_crestron_room_scheduling_panels);
[Crestron Fusion with Exchange Online](https://help.crestron.com/fusion/Content/Topics/Setup/Configuring-Crestron-Fusion-Rooms-for-Exchange-Online.htm).

### Outbound integration

| Integration | Direction | Protocol | Owner | Purpose |
|---|---|---|---|---|
| Webhook subscribers | out | Standard Webhooks (HMAC signatures), retries, dead-letter | Webhook dispatcher | ITSM, HR, workflow and agent platforms react to bookings, check-ins, allocations |
| Event bus (optional) | out | CloudEvents to Azure Event Grid / Google Pub/Sub | Webhook dispatcher | Estates whose integration bus won't take webhooks |
| Reporting & BI | out | Scheduled export to Microsoft Fabric / BigQuery, or an OData feed | Export & retention | Attendance and utilisation in Power BI or Looker |

### Physical workplace

| Integration | Direction | Protocol | Owner | Purpose |
|---|---|---|---|---|
| Occupancy sensors (e.g. Butlr, VergeSense) | in | Vendor webhooks or API polling | Signal ingest | Vacancy evidence for release |
| Badge and access control (e.g. HID, Genetec, Paxton) | in | Vendor events | Signal ingest | Badge-in counts as check-in |
| Wi-Fi presence (e.g. Cisco Spaces, Meraki) | in | Vendor API | Signal ingest | Office-level presence |
| Drawings | in | Upload from the local file system through the web UI, the CLI or the REST API | Floor-plan processing | DWG, DXF or PDF. There is no SharePoint, OneDrive or IWMS connector; other systems use the REST API |

Sensors are optional. Check-in works without them.

### Cloud platform

| Component | Azure | GCP | Used for |
|---|---|---|---|
| Hosting | AKS | GKE | Serving pods (web app & API, MCP, SCIM) and worker pods |
| Edge | Front Door + WAF | Cloud Load Balancer + Cloud Armor | Every inbound HTTPS call, including SCIM pushes and callbacks; DNS and TLS |
| Database | Azure Database for PostgreSQL Flexible Server | Cloud SQL for PostgreSQL | State, job queue, outbox |
| Object storage | Blob Storage | Cloud Storage | Drawings, renders, exports, archives |
| Secrets | Key Vault | Secret Manager | Graph app credentials, webhook signing secrets; read through workload identity, no static keys |
| Registry | Azure Container Registry | Artifact Registry | Signed images and Helm charts from CI, in the UK region |
| Observability | Azure Monitor | Google Cloud Observability | OTLP from every pod through an in-cluster OpenTelemetry Collector; Grafana or Datadog also work |
| SIEM | Microsoft Sentinel | Google Security Operations | Sign-in, admin and API audit |
| Disaster recovery | Geo-redundant backups or a replica, and geo-replicated storage, in UK West | Three zones in europe-west2, the only UK region; a second region would be outside the UK | See [deployment](deployment.md) |

## How the services fit together

- **Serving pods answer every inbound call,** including Graph change notifications and sensor
  webhooks. They validate the call, write state, and enqueue work.
- **One PostgreSQL holds state, the job queue and the outbox.** Every state change writes a
  `DomainEvent` and an outbox row in the same transaction, so a webhook, notification or
  export is never sent for a change that rolled back, and never lost for one that committed.
- **Worker pods claim jobs from PostgreSQL** (`SKIP LOCKED`) and own the outbound
  integrations: calendar sync, notifications, webhook dispatch, exports, signal processing,
  floor-plan processing and the allocation scheduler.
- **No message broker is needed** at this scale. The optional event bus is an outbound
  integration for customers, not internal plumbing.
