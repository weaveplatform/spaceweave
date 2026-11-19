# Protocol and identity facts for the API surface review

SpaceWeave provides a supported business REST API for web/mobile-web portals,
MCP servers, SDKs, scripts, tests and Terraform. Workers use private application
interfaces; SCIM and provider callbacks retain their own protocols. This research
establishes the HTTP, identity and compatibility constraints on those surfaces
for technical architects and engineers. Primary-source facts are separated from
proposed application conventions. Sources checked: 19 November 2026.

The established direction is contract-first REST with OpenAPI and generated Go
and TypeScript transport types; separate staff and admin React entrypoints;
Go-owned provider tokens and secure HttpOnly sessions with backend authorisation
and CSRF controls; verified-email visitor access; and PostgreSQL durable work.
These are repository decisions, not conclusions drawn from HTTP standards.
See [architecture decisions](../../architecture.md), [portal/session ADR](../../adr/0015-use-react-portals-with-go-managed-sessions.md)
and [durable-work ADR](../../adr/0011-own-booking-state-and-durable-work-in-postgresql.md).
Authenticated Entra SCIM provisioning is already selected in
[ADR 0019](../../adr/0019-provision-entra-users-and-groups-through-scim.md).
The [surface catalogue](README.md) records the current agreed consumer set and
qualified internal-event visitor scope; the generic protocol facts below do not
reintroduce a separate private UI business API.

## Vocabulary for comparing the five options

**Proposed review vocabulary:** record the following dimensions separately.
They avoid giving “public API” three different meanings in one comparison.

| Dimension | Meaning in this review | Does not establish |
| --- | --- | --- |
| Supported consumer | UI, MCP, SDK, script, test, Terraform, provider, worker or another Go package, using the appropriate contract. | Extra permissions derived from the consumer label. |
| Public REST contract | The supported business contract consumed by UI and authorised programmatic clients. | Anonymous access or access to every operation/record. |
| Browser authentication profile | Go-managed session and CSRF requirements on the public REST operations. | A separate private UI business contract or permission for arbitrary origins. |
| Private application contract | A Go interface/call contract within this application, including worker calls. | A remotely reachable HTTP endpoint. |
| Reachability | Internet, organisation network, cluster, or in-process; decide independently for each listener/route. | Authentication, authorisation, or compatibility promises. |
| Surface kind | Resource read/write, command, asynchronous operation monitor, protocol adapter, or private interface. | One HTTP handler per Go method or one service per module. |

Go export visibility and `internal` import restrictions control source access.
They do not publish a network interface: exported identifiers are accessible to
importing packages, while an `internal` directory restricts eligible importers.
[Go specification][go-spec], [Go command documentation][go-internal].
Kubernetes separately distinguishes cluster-internal Services from external
exposure through Service types, Ingress or Gateway. A source-level module API
therefore does not decide network reachability. [Kubernetes Service docs][k8s-service].

OpenAPI describes HTTP paths, operations, parameters, schemas, responses, headers
and security requirements. It can describe authenticated surfaces; it does not
grant access or implement authorisation. An empty Security Requirement Object
permits anonymous access in the description. These are specification facts, not
an instruction to expose anonymous endpoints. [OpenAPI 3.2.1][oas].
The cited version is the current published specification at research time, not a
decision to select that version or a particular generator.

## Verified HTTP semantics

The short statements below summarise the relevant standards; the application
must still define which operations use them.

| Mechanism | Standard meaning |
| --- | --- |
| Safe methods | GET and HEAD request read-only semantics. [RFC 9110 §9.2.1][http-safe] |
| `ETag` / `If-Match` | A representation validator; `If-Match` uses strong comparison and guards against lost updates. [RFC 9110 §13.1.1][http-match] |
| `412` | A supplied precondition failed; the mutation is not performed, subject to the specified already-applied exception. [RFC 9110 §13.1.1][http-match] |
| `428` | The server requires a conditional request; explain how to resubmit. [RFC 6585 §3][preconditions] |
| `409` | Conflict with current resource state. [RFC 9110 §15.5.10][http-conflict] |
| `422` | Understood content type and valid syntax, but unprocessable instructions. [RFC 9110 §15.5.21][http-422] |
| `202` | Processing accepted but incomplete; completion is not guaranteed. The response ought to identify current status and a status monitor. [RFC 9110 §15.3.3][http-202] |
| `If-None-Match` | Supports conditional retrieval, including `304` when the representation matches. [RFC 9110 §13.1.2][http-none-match] |
| Retrying `POST` | Automatic retry needs known idempotent semantics or evidence the original was unapplied. [RFC 9110 §9.2.2][http-retry] |

**Proposed SpaceWeave distinction:** a supplied resource revision detects a stale
edit; a retry key identifies another attempt at the same command; the booking
transaction enforces competing-capacity rules. None replaces either of the
others. A resource ETag alone cannot reserve a desk or make desk/parking changes
atomic. This is a design inference from the selected consistency requirements,
not an extra HTTP guarantee.

For first-party mutable resources, consider strong validators and documented
`If-Match` requirements. Specify which representations/revisions they cover.
Distinguish stale edits from capacity/state conflicts with separate problem
types. Do not describe `If-Match: *` as a specific revision check.
The required operations and missing-precondition policy remain open.

## Retrying commands without duplicating their effects

The IETF `Idempotency-Key` work remains an **active Internet-Draft**, revision
`-08`, dated 22 October 2026, at research time. It is not a published RFC or a
header that every HTTP server necessarily honours. [IETF document status][retry-status].
Its model includes client-generated keys, resource-defined uniqueness and
expiration policy, optional request fingerprints, handling completed retries
and concurrent attempts, and published server behaviour. A client cannot assume
support merely by sending the header. [Draft §§2.2–2.7][retry-draft].

**Proposed SpaceWeave convention, subject to contract review:** document a retry
key only for commands whose duplicate execution matters. The contract would say:

- Which operation requires it, its wire name/format, and which caller identity
  and operation form the uniqueness scope.
- What makes two payloads equivalent, and how reusing a key with different
  command content is rejected.
- How an in-progress attempt is represented, and what a completed retry returns.
- How long keys/results are retained, what happens after expiry, and which
  failures are retained rather than retried as a fresh attempt.
- How durable command acceptance and its retry record become consistent, so a
  crash after commit cannot create a second accepted command on replay.
- How authorisation is checked on each attempt and how a replay result avoids
  disclosing data after the caller's rights change.

The durable record/transaction and replay-result rules above are application
design proposals. The draft does not supply a PostgreSQL implementation or
promise exactly-once execution across external providers. An incoming HTTP key,
an internal job identity and a provider's retry mechanism require explicit
mapping; they are not automatically the same contract.

## Asynchronous acceptance and visible business state

**Proposed application convention:** where work cannot finish during the request,
return an authorised operation reference only after the agreed durable acceptance
boundary. Specify how to read progress and terminal outcome, retention/expiry,
polling behaviour, and the reference to any resulting domain resource. Do not
expose internal queue payloads as the client contract.

An operation monitor's successful completion must name the completed activity.
For example, persisting a room request and scheduling synchronisation is not
proof of Exchange acceptance. SpaceWeave already requires room requests to remain
pending until Exchange accepts them; see [ADR 0006](../../adr/0006-keep-room-requests-pending-until-exchange-accepts.md).
Likewise, returning an operation reference is not a desk confirmation. These
distinctions preserve existing product semantics without choosing new routes or
new asynchronous features. An operation monitor is needed only where selected
operations actually use asynchronous acceptance.

## Error contracts

RFC 9457 defines `application/problem+json`, with `type` as the primary machine
identifier and optional `title`, `status`, `detail`, `instance` and extensions.
The body's `status`, when present, must agree with the HTTP response status;
human-readable `detail` is not a machine parsing contract. New problem types
document their URI, title and status. The RFC warns against exposing sensitive
implementation details through problems or occurrence links. [RFC 9457 §§3–5][problems].

**Proposed first-party convention:** use stable problem types for validation,
stale revision, capacity/state conflict, access denial and temporary failure.
Document any field-error and correlation extensions, retry advice and redaction.
The definitive catalogue and status mapping remain design work.

SCIM already defines a different error schema and its own `scimType` values.
Its protocol adapter should preserve that contract; RFC 9457 explicitly allows
existing domain-specific error formats. [RFC 7644 §3.12][scim-errors],
[RFC 9457 §4][problems]. This does not require internal Go errors to be HTTP
problem objects: adapters can map application outcomes to each wire protocol.

## Pagination, validators and sensitive data

SCIM's index pagination uses `startIndex` and `count`, with `totalResults`,
`itemsPerPage` and `startIndex` in list responses. Its resource-versioning section
describes optional ETag support; it does **not** establish that every SCIM service
must implement conditional writes. SCIM also illustrates weak ETags, which must
not be copied into an ordinary HTTP `If-Match` design that requires strong
comparison. Treat the SCIM integration profile separately from first-party
resource concurrency. [RFC 7644 §§3.4.2.4, 3.14][scim], [RFC 9110 §13.1.1][http-match].

**Proposed first-party list contract:** choose and document page bounds, ordering
with a stable tie-breaker, filters, continuation expiry and behaviour while data
changes. Cursor versus offset, exact totals and snapshot guarantees are open.
SCIM's paging requirements do not choose pagination for the portal endpoints.
If continuation tokens are used, bind their meaning to the original query and
authorisation scope; opaque tokens do not themselves authorise access.

HTTP caching controls are distinct from concurrency control. `private` excludes
shared-cache storage but allows private-cache storage; `no-cache` requires
validation before reuse, while `no-store` prohibits storage by compliant HTTP
caches. `no-store` alone is not a privacy guarantee. A `Set-Cookie` response
field does not itself inhibit caching. [RFC 9111 §§3, 5.2.2, 7.3][caching].

**Proposed data-handling convention:** explicitly classify personal booking,
visitor, directory, session and operation-status responses before enabling
caching. Consider `no-store` for sensitive responses and intentional conditional
retrieval for suitably classified revisioned data/assets. Define cache variation
and re-authorisation as part of each representation contract. No blanket rule
here requires ETags for every read, requires them for SCIM, or makes a cached
availability view authoritative for a booking transaction.

## Browser sessions and OIDC

RFC 10017, published August 2026, now provides the browser OAuth best-current-
practice reference. Its BFF keeps OAuth responsibilities and tokens in a
confidential backend, uses the authorization-code flow, and associates browser
requests with a cookie session. Its cookie guidance requires `Secure` and
`HttpOnly`, recommends `SameSite=Strict` and avoiding `Domain`, and requires CSRF
defence. Same-site subdomains can still be different origins; SameSite alone
does not resolve that risk. CORS-based CSRF protection needs the documented
preflight/custom-header conditions; simply enabling CORS is insufficient.
Malicious JavaScript can still send requests using the victim's browser.
[RFC 10017 §§6.1.1–6.1.4][bff].

This supports the agreed Go-managed session direction; it does not require a
separate BFF process or a second API hop inside the Go application. The RFC
explicitly permits colocating the internal resource server with the BFF.
[RFC 10017 §6.1.2.5][bff].

The OAuth redirect flow has its own CSRF requirement, separate from protection of
cookie-authenticated application writes. PKCE, OIDC nonce and session-bound
state have defined roles in that flow. [RFC 9700 §§2.1.1, 4.7.1][oauth-security].
OIDC ID-token validation includes issuer, client audience, expiry, applicable
signature/algorithm rules and matching a sent nonce. [OIDC Core §3.1.3.7][oidc].
Choosing portal origins, session boundaries, logout/revocation behaviour and the
exact CSRF mechanism remains necessary; separate bundles alone cannot enforce
admin authorisation. Verified-email visitor access remains its agreed separate
identity flow, not proof of an Entra workforce identity.

## Entra identities and API permissions

Entra delegated access acts for a signed-in user; application access acts as the
application without a signed-in user. Delegated permissions are scopes;
application permissions are app roles. These identify different authority
contexts, not interchangeable ways to describe “logged in”.
[Microsoft permissions overview][entra-permissions].
The `scp` claim identifies delegated scopes. `roles` can carry application
permissions **or user-assigned app roles**, so its presence alone does not prove
an app-only caller. [Microsoft access-token claims][entra-claims].

For an API accepting Entra access tokens, validate the token for that API:
signature/issuer and intended audience, then the permitted tenant, subject,
client and operation authority. For v2 access tokens the audience is the API's
client-ID GUID; v1 can use its App ID URI. A Graph access token is not a
SpaceWeave access token. Checking only `tid` and `oid` can unintentionally admit
service principals as well as users. [Microsoft token validation][entra-tokens],
[Microsoft claims validation][entra-validation].
Access-token privileges should be restricted to the minimum required for the use
case. [RFC 9700 §2.3][oauth-security].

**Application boundary:** browser operations use staff/visitor session
authorisation on the shared public REST contract. Token clients use the API's
explicit Entra resource audience and delegated scopes/application roles, mapped
to current domain authority. OAuth consent
alone does not specify SpaceWeave office scope or booking delegation policy.
Do not add generic app-only booking authority merely because a background Graph
integration uses application credentials. SCIM authentication also needs its selected
Entra provisioning profile; this appendix does not silently choose a new token
scheme for it.

## Decisions still open for the surface design

| Decision | What the comparison must make reviewable |
| --- | --- |
| Consumer grants | UI/MCP/SDK/script/test/Terraform are supported audiences; exact client grants and allowed operations remain to define. |
| Publication/reachability | Supported contract audience, listener/route exposure, and authentication independently. |
| Concurrency | Required previously read versions for configuration edits; exact validator representation and precondition responses. |
| Retry policy | Booking/ticket/work creation requires keys and atomic receipts; resolve wire equivalence, retention and expiry/reconciliation details. |
| Asynchronous contract | Allocation/plan processing/export monitors are required; define schemas, progress freshness, cancellation races and result expiry. |
| Errors and lists | Problem catalogue, protocol-specific mappings, pagination and consistency guarantees. |
| Browser security | Origins, session separation/lifecycle and concrete redirect/write CSRF controls. |
| Token profiles | Define audiences and concrete grants for the separate human/application authority profiles. |

[go-spec]: https://go.dev/ref/spec#Exported_identifiers
[go-internal]: https://pkg.go.dev/cmd/go#hdr-Internal_Directories
[k8s-service]: https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types
[oas]: https://spec.openapis.org/oas/v3.2.1.html
[http-safe]: https://www.rfc-editor.org/rfc/rfc9110.html#section-9.2.1
[http-match]: https://www.rfc-editor.org/rfc/rfc9110.html#section-13.1.1
[preconditions]: https://www.rfc-editor.org/rfc/rfc6585.html#section-3
[http-conflict]: https://www.rfc-editor.org/rfc/rfc9110.html#section-15.5.10
[http-422]: https://www.rfc-editor.org/rfc/rfc9110.html#section-15.5.21
[http-202]: https://www.rfc-editor.org/rfc/rfc9110.html#section-15.3.3
[http-none-match]: https://www.rfc-editor.org/rfc/rfc9110.html#section-13.1.2
[http-retry]: https://www.rfc-editor.org/rfc/rfc9110.html#section-9.2.2
[retry-status]: https://datatracker.ietf.org/doc/draft-ietf-httpapi-idempotency-key-header/
[retry-draft]: https://datatracker.ietf.org/doc/html/draft-ietf-httpapi-idempotency-key-header-08
[problems]: https://www.rfc-editor.org/rfc/rfc9457.html
[scim-errors]: https://www.rfc-editor.org/rfc/rfc7644.html#section-3.12
[scim]: https://www.rfc-editor.org/rfc/rfc7644.html
[caching]: https://www.rfc-editor.org/rfc/rfc9111.html
[bff]: https://www.rfc-editor.org/rfc/rfc10017.html
[oauth-security]: https://www.rfc-editor.org/rfc/rfc9700.html
[oidc]: https://openid.net/specs/openid-connect-core-1_0.html#IDTokenValidation
[entra-permissions]: https://learn.microsoft.com/en-us/entra/identity-platform/permissions-consent-overview
[entra-claims]: https://learn.microsoft.com/en-us/entra/identity-platform/access-token-claims-reference
[entra-tokens]: https://learn.microsoft.com/en-us/entra/identity-platform/access-tokens#validate-tokens
[entra-validation]: https://learn.microsoft.com/en-us/entra/identity-platform/claims-validation
