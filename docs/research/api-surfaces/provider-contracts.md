# Provider protocols and integration boundaries

SpaceWeave is an enterprise workplace booking platform for desks, meeting rooms,
parking, internal events and hosted visitors. It obtains staff identity and
membership from Microsoft Entra, workforce facts from Workday, synchronises room
bookings with Exchange Online, and stores versioned floor-plan assets in Azure
Blob Storage or Google Cloud Storage. Staff and administrators use web portals;
external event visitors use narrowly scoped verified-email access.

This research explains the provider-owned protocols that constrain the API and
worker design. It covers inbound SCIM provisioning, Graph change notifications,
optional personal Teams booking messages, Workday retrieval, signed storage
transfers and Kubernetes/telemetry interfaces. Each integration has its own
caller identity, protocol response rules, retries and reachability requirements.
Facts cite primary specifications/provider documentation; hostnames, credentials
and tenant deployment settings remain implementation choices. Sources checked:
19 November 2026.

## Reachability, caller identity and application authority are separate

An Internet-reachable HTTPS endpoint can still require authenticated,
authorised requests. Conversely, a private network route does not establish a
caller's application permissions. SCIM authenticates a provisioning client;
Graph validates callback ownership and notification authenticity; Teams uses
bot service authentication. These are distinct from the staff browser session.
[SCIM authentication](https://www.rfc-editor.org/rfc/rfc7644.html#section-2),
[Graph callback validation](https://learn.microsoft.com/en-us/graph/change-notifications-delivery-webhooks#notificationurl-validation),
[Bot Connector authentication](https://learn.microsoft.com/en-us/azure/bot-service/rest-api/bot-framework-rest-connector-authentication).

| Surface | Direction relative to SpaceWeave | Protocol consequence |
| --- | --- | --- |
| Entra provisioning | Inbound SCIM client requests | Authenticate the configured provisioning client; preserve SCIM semantics. |
| Graph notifications/lifecycle | Inbound provider callbacks | Reachable HTTPS callbacks, challenge handling and notification validation. |
| Native room check-in | Outbound Graph create/read calls on a user's behalf | Supported delegated user permission and native eligibility; no assumed application-permission check-in reader or independent room-expiry service. |
| Teams personal delivery | Outbound messages plus inbound bot activities | Bot installation/conversation/authentication contract, separate from Graph callbacks. |
| Workday HR retrieval | Outbound provider API calls if retrieval is selected | SOAP WWS is verified; REST coverage and tenant access need selection. |
| Asset capability issuance | App request followed by client-to-storage transfer | App authority is checked before delegating a limited storage capability. |
| Probes/telemetry | Kubelet/collector traffic and application export | Operational access paths with deployment-controlled exposure. |

The table is a classification of the cited contracts below, not a decision
about route names, hostnames, listeners, Kubernetes Services or gateways.

## Entra to SpaceWeave: SCIM 2.0 users and mapped groups

Entra acts as a SCIM client for app provisioning. Entra's separate inbound
SCIM-provider capabilities do not describe this direction. The SpaceWeave provisioning contract includes both Users and explicitly mapped Groups even
though Entra permits applications that support users alone.
[Entra SCIM roles](https://learn.microsoft.com/en-us/entra/identity/app-provisioning/scim-support-in-entra-id),
[lifecycle requirements](../../architecture.md#verified-scim-lifecycle-constraints).

The SCIM resource endpoints are `/Users` and `/Groups`; operations include
POST creation, GET by ID/query, PATCH changes and DELETE. SCIM JSON uses
`application/scim+json`. Query responses use the SCIM `ListResponse` schema,
with `Resources`, `totalResults` and pagination fields. An unmatched valid
filter returns a successful empty list rather than a missing-resource error.
[SCIM methods, queries and media type](https://www.rfc-editor.org/rfc/rfc7644.html#section-3).

Errors use `urn:ietf:params:scim:api:messages:2.0:Error`, required string
`status`, and optional `scimType`/`detail`. Examples include invalid filters
and uniqueness conflicts. SCIM PATCH uses a `PatchOp` envelope with ordered
`Operations` (`add`, `remove`, `replace`), and is not RFC 6902 JSON Patch.
One PATCH request is atomic: failure restores its original resource.
DELETE returns 204 and makes the resource unavailable through the interface;
retention of internal history is a separate concern.
[SCIM PATCH](https://www.rfc-editor.org/rfc/rfc7644.html#section-3.5.2),
[deletion](https://www.rfc-editor.org/rfc/rfc7644.html#section-3.6),
[SCIM errors](https://www.rfc-editor.org/rfc/rfc7644.html#section-3.12).

Entra-specific interoperability requirements include HTTPS, PATCH, known-ID
reads, user listing/pagination, matching-attribute filters using `eq`/`and`,
and group queries supporting `excludedAttributes=members`. Group display
names must be unique for this integration. Structural PATCH operation values
must tolerate Entra's `Add`, `Replace`, `Remove` capitalisation. Disabled
users remain readable and support `active=false` followed by `active=true`.
[Entra implementation profile](https://learn.microsoft.com/en-us/entra/identity/app-provisioning/use-scim-to-provision-users-and-groups#understand-the-microsoft-entra-scim-implementation).

Microsoft also documents historical compliance differences and feature flags.
Record and verify the tenant connector's configured behaviour; a generic
application PATCH handler or a copied historical sample is insufficient
evidence of compatibility.
[Entra SCIM compliance issues](https://learn.microsoft.com/en-us/entra/identity/app-provisioning/application-provisioning-config-problem-scim-compatibility).

SCIM itself does not prescribe a unique authentication scheme. Current Entra
guidance supports OAuth client credentials and workload identity federation
for gallery/non-gallery provisioning. Its table retains long-lived bearer
tokens for existing/non-gallery apps, while rejecting them for new gallery
onboarding. WIF requires a token endpoint and verification of Entra's signed
assertion; the SCIM resource call then carries an access token.
[Current Entra authentication guidance](https://learn.microsoft.com/en-us/entra/identity/app-provisioning/use-scim-to-provision-users-and-groups#authorization-to-provisioning-connectors-in-the-application-gallery).
[Gallery onboarding requirements](https://learn.microsoft.com/en-us/entra/identity/enterprise-apps/app-gallery-user-provisioning-requirements#scim-authentication-requirements)
explicitly exclude basic authentication and long-lived bearer tokens for new
gallery submissions; gallery publication is not an agreed SpaceWeave requirement.

Therefore authentication is required, but a static secret, OAuth token endpoint
or WIF exchange must not be silently chosen in this research. Microsoft also
warns that its automatically generated blank-secret-field token is primarily
for testing. Actual connector/profile support must be checked for the tenant.
[Entra endpoint authentication](https://learn.microsoft.com/en-us/entra/identity/app-provisioning/use-scim-to-provision-users-and-groups#handling-endpoint-authentication).

SCIM needs connectivity from its client, not universal Internet exposure as
an intrinsic property of the protocol. Microsoft supports an on-premises
SCIM provisioning-agent route with target connectivity and outbound Microsoft
connectivity. Direct cloud provisioning and an agent-mediated private route
are operational alternatives; neither makes SCIM anonymous or a staff API.
[On-premises SCIM provisioning prerequisites](https://learn.microsoft.com/en-us/entra/identity/app-provisioning/on-premises-scim-provisioning#prerequisites).

## Microsoft Graph change and lifecycle notifications

Graph webhook delivery requires a publicly accessible HTTPS endpoint.
Network/firewall restrictions can narrow inbound reachability to documented
Microsoft Graph sender ranges. The provider requirement does not determine
whether the application backend itself has a public address behind that edge.
[Webhook reachability and firewall guidance](https://learn.microsoft.com/en-us/graph/change-notifications-delivery-webhooks#considerations-for-a-webhook-endpoint).

Subscription creation triggers a POST with a query-string `validationToken`.
Return its URL-decoded opaque value as a plain-text body with HTTP 200 and
`Content-Type: text/plain` within ten seconds. This validates callback control;
it is not the authenticity check for later resource notifications. A configured
`lifecycleNotificationUrl` is validated too, including when both URLs coincide.
[Callback ownership challenge](https://learn.microsoft.com/en-us/graph/change-notifications-delivery-webhooks#notificationurl-validation),
[rich-notification endpoint validation](https://learn.microsoft.com/en-us/graph/change-notifications-with-resource-data#subscription-request).

Basic change notifications carry subscription/resource information and a
`clientState` secret that must match the subscription's value. The resource
model marks `clientState` optional, whereas webhook guidance requires setting
it: configure and validate it for this callback profile. Its presence is not
a general signed-body/JWT guarantee.
[Notification fields](https://learn.microsoft.com/en-us/graph/api/resources/changenotification?view=graph-rest-1.0),
[basic versus rich validation](https://learn.microsoft.com/en-us/graph/change-notifications-with-resource-data#validate-the-authenticity-of-notifications).

Rich notifications add resource data, certificate-based encryption and JWT
`validationTokens`. Verify token signatures, expiry, audience and the documented
Graph publisher identity, and retain the corresponding private keys for
decryption/rotation. These JWTs are distinct from the query-string challenge.
Basic and rich notification modes must not be treated as interchangeable
authentication contracts; mode selection remains open.
[Rich notification security](https://learn.microsoft.com/en-us/graph/change-notifications-with-resource-data#how-to-validate),
[encryption and key rotation](https://learn.microsoft.com/en-us/graph/change-notifications-with-resource-data#decrypting-resource-data-from-change-notifications).

For delivery, return 200 when work completes promptly or 202 after queueing
within the documented three-second initial window. Return 5xx if neither
processing nor queueing succeeds. Delivery retries back off for up to four
hours; slow/drop states can cause unrecoverable notification loss. An HTTP
acknowledgement does not prove application reconciliation completed.
[Acknowledgement and retry contract](https://learn.microsoft.com/en-us/graph/change-notifications-delivery-webhooks#http-codes-and-retry-logic).

Rich-notification guidance additionally says to acknowledge immediately before
JWT validation, then ignore invalid notifications. Reconcile that guidance
with durable bounded intake before implementation; acknowledgement must never
be mistaken for authorisation to apply a notification's business effects.
[Rich acknowledgement guidance](https://learn.microsoft.com/en-us/graph/change-notifications-with-resource-data#how-to-validate).

Lifecycle events are `reauthorizationRequired`, `subscriptionRemoved` and
`missed`; support depends on resource type. Outlook events support all three.
Removal requires restoring a subscription and recovering missed changes;
`missed` calls for resynchronisation, such as delta query. Lifecycle delivery
does not replace ordinary renewal or reconciliation.
[Lifecycle support and recovery](https://learn.microsoft.com/en-us/graph/change-notifications-lifecycle-events).

Calendar-view delta is specific to one calendar and a fixed start/end range.
Follow opaque `nextLink` URLs to completion and persist the resulting
`deltaLink` for the next round. Each calendar needs its own progress. This is
state reconciliation, not a replayable log of every intermediate notification.
[Calendar-view delta contract](https://learn.microsoft.com/en-us/graph/delta-query-events).

The retry/loss contracts require repeat-safe processing and a recovery path;
they establish no exactly-once delivery or unconditional ordering guarantee.
Neither notifications nor delta establish Graph event compare-and-swap.
The latter remains explicitly unverified in the existing architecture notes.
[Webhook retries](https://learn.microsoft.com/en-us/graph/change-notifications-delivery-webhooks#http-codes-and-retry-logic),
[existing Exchange constraints](../../architecture.md#verified-exchange-constraints).

## Exchange-native room policy and check-in

Native Exchange room admission/check-in/autorelease governs managed rooms across
Outlook and SpaceWeave. The supported native check-in create/read APIs require
user-delegated permission; calendar application permissions do not imply access
to those operations. Native claims lack a named-person field, so keep application
request audit separate from attendance inference. The
[native-policy research](outlook-room-policy-enforcement.md) cites the exact
contracts, configuration limitations, eligibility and release effects.

Configure native room policy through supported native administration/integration.
Document how effective settings are obtained/refreshed; do not assume Graph
exposes every Exchange administrative setting. A desired local configuration
record is not proof of native activation. The
[room contract](recurring-rooms-and-leavers.md) excludes independent application
timers/counters and records unverified requested behaviour as capability gaps.

## Teams: private proactive personal booking-raised message

The product permission is narrow: one private personal booking-raised message
type, with the individual's opt-in. Tenant installation, SSO, bot access and
provider API permission establish technical prerequisites; none establishes
that individual product preference. Other Teams messages, installation welcome
messages and shared-channel posting are outside the agreed scope.
[Agreed Teams scope](../../architecture.md#verified-personal-teams-messaging-constraints).

Personal proactive delivery uses a Teams app containing a bot/agent. Addressing
by Entra object ID requires personal-scope installation; missing installation
can produce 403. A conversation needs recipient ID, tenant ID and service URL;
retain its ID/reference for later messages. Email/UPN is not a supported
proactive-message address.
[Personal proactive prerequisites](https://learn.microsoft.com/en-us/microsoftteams/platform/bots/how-to/conversations/send-proactive-messages).

App installation can be performed through Graph or administrative setup
policies. Proactive Graph installation needs the appropriate installation
permissions and an app available in the organisation's catalog or Teams Store.
This is an installation API, not general app-only chat posting.
[Proactive installation](https://learn.microsoft.com/en-us/microsoftteams/platform/graph-api/proactive-bots-and-messages/graph-proactive-bots-and-messages).

The bot has its own registered, reachable HTTPS messaging endpoint. Incoming
activity handling includes conversation and
installation lifecycle events; `installationUpdate` reports add/remove and
`conversationUpdate` supplies conversation context. Treat those as a distinct
provider activity surface, even if one deployment shares an ingress.
[Teams endpoint registration](https://learn.microsoft.com/en-us/microsoftteams/platform/agents-in-teams/registering-agent-teams-platform),
[HTTPS messaging endpoint](https://learn.microsoft.com/en-us/microsoftteams/platform/concepts/authentication/auth-flow-bot),
[conversation/installation events](https://learn.microsoft.com/en-us/microsoftteams/platform/bots/how-to/conversations/subscribe-to-conversation-events).

Current Teams registration guidance supports Teams-managed or Azure-managed
Agent Communication Service registration; an Azure AI Bot Service resource is
one option, not an intrinsic requirement of a private personal message.
Registration/SDK choice and its current authentication profile need validation
before implementation; application staff SSO does not choose that profile.
[Registration alternatives](https://learn.microsoft.com/en-us/microsoftteams/platform/agents-in-teams/choose-agent-registration-type).

Bot Connector requests carry signed JWTs in the Authorization header.
Validate the documented issuer, audience, lifetime and signing keys; outbound
bot requests use service-to-service authentication. This is separate from
Graph's validationToken/clientState and from end-user authentication.
[Connector authentication and validation](https://learn.microsoft.com/en-us/azure/bot-service/rest-api/bot-framework-rest-connector-authentication).

The ordinary Graph `POST /chats/{id}/messages` permission table provides
delegated `ChatMessage.Send` and application `Teamwork.Migrate.All` for
migration. It supplies no general application-only background-posting grant.
An activity-feed notification is another product surface and does not satisfy
the agreed private-chat requirement by substitution.
[Chat posting permissions](https://learn.microsoft.com/en-us/graph/api/chat-post-messages?view=graph-rest-1.0),
[activity-feed surface](https://learn.microsoft.com/en-us/graph/teams-send-activityfeednotifications).

## Workday: separate HR provider contract

Workday's current production directory identifies WWS v47.1 / 2026R2 as
SOAP services with WSDL/XSD. Human Resources includes employee, contingent
worker and organisation data; `Get_Workers` retrieves worker information.
An outbound SpaceWeave-to-Workday request can therefore retrieve inbound HR
data without requiring a Workday-to-SpaceWeave callback.
[WWS directory](https://community.workday.com/sites/default/files/file-hosting/productionapi/index.html),
[Human Resources operations](https://community.workday.com/sites/default/files/file-hosting/productionapi/Human_Resources/v47.1/Human_Resources.html).

Workday also publishes a separate REST API directory. That proves REST is
available as an API family, not that one REST resource has all required HR
fields, incremental/effective-date semantics or permissions. SOAP versus REST,
API version and the tenant's permitted operations remain choices.
[Workday REST directory](https://community.workday.com/sites/default/files/file-hosting/restapi/index.html),
[Workday integration API overview](https://forms.workday.com/content/dam/web/en-us/documents/datasheets/datasheet-workday-cloud-connect-platform.pdf).

Microsoft's Workday provisioning connector specifically uses WWS
`Get_Workers`, basic authentication and a tenant integration-system user with
configured security-domain access. That documents Microsoft's connector;
it does not mandate the credential method for SpaceWeave's own adapter or
prove that this tenant has supplied credentials/visibility.
[Microsoft's Workday connector contract](https://learn.microsoft.com/en-us/entra/identity/app-provisioning/workday-integration-reference#establishing-connectivity),
[integration-system-user/security setup](https://learn.microsoft.com/en-us/entra/identity/saas-apps/workday-inbound-tutorial#configure-integration-system-user-in-workday).

`Get_Workers` distinguishes entry/update criteria and effective/as-of dates.
Preserve the agreed field ownership and effective-date rules; neither the WWS
directory nor the Entra SCIM contract establishes Workday SCIM delivery into
SpaceWeave. Tenant mappings and the HR transport must be verified separately.
[Get_Workers contract](https://community.workday.com/sites/default/files/file-hosting/productionapi/Human_Resources/v47.1/Get_Workers.html),
[agreed source ownership](../../architecture.md#verified-workday-and-entra-source-constraints).

## Private Azure/GCS assets and signed transfer capabilities

Azure SAS delegates bounded resource/permission/time access. Microsoft
recommends user-delegation SAS where possible; acquiring its key requires
`generateUserDelegationKey` authority, in addition to appropriate data access.
Service/account SAS instead rely on account-key authority. A server identity's
ability to read an object alone does not establish its signing authority.
[Azure SAS types and signing](https://learn.microsoft.com/en-us/azure/storage/common/storage-sas-overview),
[user-delegation permissions](https://learn.microsoft.com/en-us/rest/api/storageservices/create-user-delegation-sas).

GCS signed URLs delegate specific resource/action/time access and are usable
by anyone holding the URL. V4 signed URLs use XML API endpoints, with a
maximum seven-day expiry. Signing requires a suitable private key or signing
permission such as `iam.serviceAccounts.signBlob`, as well as permission for
the requested storage action.
[GCS signed URL contract](https://docs.cloud.google.com/storage/docs/access-control/signed-urls),
[signing permissions](https://docs.cloud.google.com/storage/docs/access-control/signing-urls-with-helpers).

The issuance endpoint must authenticate the requesting app user and check
their authority over the asset and operation before returning a capability.
The storage transfer then uses that capability, rather than the user's app
cookie. Scope download/upload permissions and lifetime to the intended
operation; a private object need not have a publicly readable bucket/container.
[Azure authenticated issuer pattern](https://learn.microsoft.com/en-us/azure/storage/common/storage-sas-overview#when-to-use-a-shared-access-signature),
[GCS application-specific access](https://docs.cloud.google.com/storage/docs/access-control/signed-urls#when-should-you-use-a-signed-url).

A GCS resumable-upload session URI is itself a bearer capability after
initiation; subsequent chunks do not reuse the initiating signed URL.
Browser direct transfers also require compatible CORS methods/headers/origins.
Neither signing nor CORS proves uploaded content is validated, published,
durable across recovery regions or tied to a committed database revision.
Those are application workflows retained from the agreed asset requirements.
[GCS resumable sessions](https://docs.cloud.google.com/storage/docs/access-control/signed-urls#using-signed-urls-with-resumable-uploads),
[browser CORS](https://docs.cloud.google.com/storage/docs/cross-origin),
[agreed asset/recovery scope](../../architecture.md#verified-azure-and-gcp-blob-constraints).

## Private operational interfaces

Liveness failure can restart a container; readiness failure removes it from
normal Service routing without killing it; startup probes defer the other
probes until startup succeeds. Kubernetes HTTP probes GET a configured Pod
port/path; 200–399 indicates success. Kubernetes does not prescribe
`/livez`, `/readyz`, their response JSON or dependencies for SpaceWeave.
[Kubernetes probe semantics](https://kubernetes.io/docs/concepts/workloads/pods/probes/).

OTLP is telemetry transport, with gRPC and HTTP variants. Default ports are
4317/4318; OTLP/HTTP uses signal paths such as `/v1/traces` and `/v1/metrics`.
These are receiver contracts, usually on a Collector receiving application
exports. They do not require a browser-facing application OTLP intake route.
[OTLP specification](https://opentelemetry.io/docs/specs/otlp/).

Collector pipelines configure receivers, processors and exporters; Prometheus
scraping/export and OTLP export are separate configurations. Selecting
OpenTelemetry does not require a public `/metrics` endpoint or decide whether
an application exposes a Prometheus scrape endpoint at all.
[Collector components and pipelines](https://opentelemetry.io/docs/collector/configuration/).

Operational topology must explicitly set listeners, routing and access policy.
OpenTelemetry recommends restricting receiver/extension binding and configuring
authentication/TLS where needed. Kubernetes Service/Ingress exposure is
deployment configuration. A path name, gRPC choice or localhost example alone
does not establish a private network boundary.
[Collector exposure guidance](https://opentelemetry.io/docs/security/config-best-practices/),
[Collector authentication/TLS](https://opentelemetry.io/docs/collector/configuration/#authentication),
[Kubernetes Services](https://kubernetes.io/docs/concepts/services-networking/service/).

## Operational choices still open

- SCIM connector/profile, provisioning credentials or federation exchange,
  mapping/filter details, and direct-cloud versus agent-mediated reachability.
- Graph basic/rich mode, subscription scope/renewal, callback ingress controls,
  durable intake limits, certificate lifecycle and reconciliation windows.
- Teams app identity/registration, personal installation policy and permitted
  Graph installation permissions, activity endpoint and conversation retention.
  Individual booking-raised opt-in remains mandatory under the agreed scope.
- Workday tenant/version, SOAP or sufficient REST operations, security domains,
  authentication, effective-date queries and reconciliation completeness.
- Blob proxy/direct-transfer choice, signer permissions, capability lifetime,
  object preconditions, CORS and post-upload validation/publication steps.
- Probe definitions per runtime role, private listener/Service layout, telemetry
  authentication/TLS, Collector placement and metrics export/scrape choice.

These sources establish protocol prerequisites, not achieved latency,
availability, freshness, regional RPO/RTO or exactly-once business outcomes.
The agreed targets still require deployment-specific workload/recovery evidence.
