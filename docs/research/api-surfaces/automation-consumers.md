# Automation consumers of the supported REST APIs

SpaceWeave's public business API serves its web applications and external clients
including Model Context Protocol (MCP) servers, SDKs, scripts, tests and Terraform.
The API manages workplace definitions, attendance intent, bookings, hosted visits
and internal-event registration for one organisation. Each consumer needs stable
contracts, appropriate authority and a way to recover ambiguous outcomes.

This document explains how those consumer requirements affect API design. MCP
acts in a signed-in user's context; unattended automation can use an independently
approved application identity. Terraform initially manages long-lived workplace
and permitted configuration resources. First-party MCP/provider implementations
follow the core platform and Go/TypeScript SDKs, but their contract requirements
apply from the first supported API release.

Protocol/framework claims below cite primary sources. Proposed tool/resource
names and exact schemas remain design choices. The public API has independent
version support, an initial 180-day retirement notice, configurable scoped RBAC
and normal version checks for UI/API changes to Terraform-managed configuration.
The dated MCP links identify a researched specification snapshot; implementation
versions must be validated with the selected toolchain. Sources were checked on
19 November 2026.

## One contract, several consumer roles

**Proposal:** treat these as consumers of supported REST capabilities, with
authentication suitable for each caller and the same domain authority checks.
Shared contracts do not require every consumer to receive every capability.

| Consumer | Contract implication | Authority boundary |
| --- | --- | --- |
| UI | Use the published business REST schemas and operation semantics. | Go-owned sessions and CSRF protection; server derives the current person. |
| MCP server | Translate the user's tools/resources into supported REST calls. | Preserve that user's context and client provenance; no app-only fallback or model-supplied grant. |
| SDK | Offer an ergonomic mapping of the same schemas, errors and outcomes. | Credentials and scopes constrain the caller; an SDK does not confer trust. |
| Script/CLI | Support bounded lists, retries and noninteractive outcomes. | An explicit user or machine identity with permitted actions and office scope. |
| Contract/integration test | Exercise the supported wire behaviour with controlled fixtures. | Test identities and isolation; no production authorisation exemption. |
| Terraform provider | Reconcile selected managed configuration through REST. | Explicit configuration-management grants, subject to normal domain rules. |
| Platform worker | Use the private owning-module/job contracts. | Scoped runtime identity and validated durable work; no public caller can select this path. |

**Proposal:** keep business commands, transitions and authorisation independent
of browser, REST-client, MCP-tool and Terraform-provider representations. Each
adapter maps authenticated context into the same actor/owner/action checks.
Private worker interfaces remain implementation contracts, not an alternative
integration API or a bypass for a difficult public operation.

## MCP: two boundaries, not an automatic SpaceWeave endpoint

**Fact:** MCP servers expose tools that can call external APIs. Tool input and
output schemas describe that MCP interface; they are not an HTTP API contract.
The tools specification requires input validation, access controls, rate limits
and sanitised outputs. [MCP tools][mcp-tools].

**Design implication:** an MCP server consumes the supported REST contract.
Its protocol endpoint, hosting and tool catalogue are separate from the
application's business REST routes. A tool may compose authorised reads, but a mutation
must preserve the REST command's atomicity, deadlines and pending outcomes.

**Fact:** MCP's core authorisation specification addresses HTTP transports.
When supported, HTTP implementations should conform to it; stdio implementations
should instead obtain credentials from the environment. A protected MCP server
is an OAuth resource server, and its MCP client is an OAuth client. Discovery
and access-token requirements belong to that connection. [MCP authorisation][mcp-auth].

**Fact:** an MCP server must validate that incoming access tokens were issued
for that server. When it calls an upstream API, it may act as an OAuth client
using a separate token issued for that API; forwarding the MCP client's token
unchanged is forbidden. MCP clients must identify the intended resource in
authorisation and token requests. [MCP security requirements][mcp-security].

**Proposal:** model the possible chain explicitly:

```text
MCP client -- token for MCP server --> MCP server
MCP server -- credential for SpaceWeave REST --> SpaceWeave
```

**Proposal:** retain separate audiences and credential validation at both
boundaries. A token for Microsoft Graph, the MCP server, or an OIDC login is not
automatically a SpaceWeave API credential. Reusing the UI's business API does
not require exporting its session/provider tokens to an MCP client.
Entra resource registration, Conditional Access applicability and the precise
downstream mechanism follow the actual user journey/hosting topology. MCP is
user-context in this product; it does not select a universal OBO journey or
permit a fallback to application authority. Machine-identity scripts are a
different channel.

**Application authority:** distinguish a human-authorised call from unattended automation.
For a human call, preserve the authenticated person's actor identity and any
separate booking owner, with current eligibility and delegation checked by
SpaceWeave. For an application call, preserve an application actor and explicit
machine grants. Neither an OAuth consent screen nor a tool argument creates
SpaceWeave delegation, office scope, or permission to check in another person.
The same distinction applies to SDKs, scripts and Terraform automation.

**Fact:** MCP tool annotations, including `readOnlyHint`, `destructiveHint` and
`idempotentHint`, are descriptive hints whose accuracy is not guaranteed.
Clients must treat annotations from untrusted servers accordingly.
[MCP annotation schema][mcp-annotations], [MCP tools][mcp-tools].

**Proposal:** annotations may guide a host's presentation or confirmation
behaviour; they must never authorise a SpaceWeave action. A tool labelled
idempotent still needs the underlying command's retry contract. Tool visibility,
user confirmation and a valid MCP token do not replace REST authorisation.
Keep operation IDs, request correlation and the authenticated actor available
across the adapter boundary without placing access tokens in logs or results.

## Terraform: REST must support reconciliation

**Fact:** a managed resource maps creation, reading, updating and deletion into
Terraform's lifecycle. Read refreshes remote state during plan/apply/refresh
and after import; it is not just a helper following a write.
[Framework resources][tf-resources], [Framework Read][tf-read].

**Fact:** Framework resource identity must resolve to at most one remote object,
allow existence and state discovery at import, and remain stable for the
object's lifetime. Import must provide enough state for a subsequent Read, or
return an error. Composite lookup information can be supported.
[Resource identity][tf-identity], [Resource import][tf-import].

**Proposal:** expose durable, opaque identifiers and an authorised lookup that
returns managed fields without requiring the original create payload. Keep IDs
independent of mutable display names and folder/layout positions. Document
whether office/organisation scope forms part of lookup and import identity.
This benefits other API clients even before a provider exists.

| Lifecycle need | Proposed SpaceWeave REST property |
| --- | --- |
| Create | Return resource identity and committed/accepted outcome; support recovering an uncertain result without duplicate objects. |
| Read/refresh | Return authoritative managed fields and relevant computed status; provide an explicit absence outcome. |
| Import | Resolve an existing object by documented identity using ordinary read permissions; no privileged import endpoint is implied. |
| Update | Distinguish mutable fields from changes requiring a new object; protect concurrent edits and retain domain validation. |
| Delete | Define removal, retirement and dependency conflicts; report actual completion for asynchronous deletion. |
| Drift | Expose out-of-band changes, source ownership and effective values accurately enough for the provider to compare state. |

**Fact:** HashiCorp recommends refreshing all possible values so that Terraform
can detect drift and simplify import. A confirmed missing resource is removed
from Terraform state; read errors retain prior state. Deletion treats an already
absent object as success. [Framework Read][tf-read], [Framework Delete][tf-delete].

**Proposal:** document absence separately from forbidden access, source outage
and temporary replication delay. If concealment intentionally uses the same
response as absence, provider-safe lookup behaviour needs a deliberate design.
Do not make loss of read authority look like permission to recreate an object.
An import/read API must not substitute a successful cached value for an outage
without making its freshness and authority clear.

**Fact:** Create and Update response state must preserve known planned values;
unknown values must be resolved to known values or null. Unanticipated server
normalisation can therefore cause provider consistency errors. Semantic equality
can address inconsequential representation differences.
[Framework Create][tf-create], [Framework Update][tf-update], [Custom types][tf-equality].

**Proposal:** define defaults, canonical forms, null/omitted semantics, ordering
and computed fields in the REST contract. Keep configured policy inputs separate
from derived effective policy. A provider must not hide a genuine UI/policy
change as formatting noise or silently fight a source-owned SCIM/HR value.
Choose a clear management boundary before promising Terraform ownership.

**Fact:** Framework CRUD implementations must account for known API delays and
can expose configurable timeouts. HTTP `202 Accepted` records acceptance, not
successful completion, and should describe or link to status monitoring.
[Framework timeouts][tf-timeouts], [HTTP 202][http-202].

**Proposal:** an asynchronous mutation needs a durable operation identifier,
authorised status reads, terminal success/failure, result identity and a defined
read-after-write visibility point. A provider can poll within a bounded timeout
and resume discovery after interruption. It must not equate queue acceptance
with convergence, poll by repeating creation, or retry every error indefinitely.
Specify whether cancellation stops waiting, requests cancellation of work, or
leaves the accepted operation running; these are different outcomes.

**Initial Terraform scope:** durable workplace definitions and permitted managed
configuration are initial Terraform candidates, with useful read-only data
sources. Personal attendance intents, check-in, tickets and transient bookings
are outside initial managed-resource scope. Exact resource schemas remain open.
Reconciliation must not fabricate or repeatedly recreate real-world evidence.
Here “short-lived” describes domain data, not
Terraform's separate ephemeral-resource feature. The provider resource/data-source
catalogue, dependency behaviour and deletion policy remain open.

## SDKs, scripts and tests: common client requirements

**Fact:** HTTP advises against automatically retrying non-idempotent requests
unless the client knows the semantics are idempotent or can determine that the
first attempt was not applied. Strong validators with `If-Match` can protect
against lost updates. [HTTP retry semantics][http-retry], [HTTP preconditions][http-match].

**Proposal:** describe the following once in the common OpenAPI contract and
supporting behaviour documentation; generated code alone cannot supply them.

| Requirement | Consumer-visible contract |
| --- | --- |
| Authentication | Accepted caller types, credential audience, required scopes/grants and current domain-authority checks. |
| Reliable writes | Covered commands, idempotency-key scope/lifetime, payload-equivalence rules and authorised replay behaviour. |
| Safe concurrency | Applicable resource revision/validator, stale-write error and explicit re-read/re-plan behaviour. |
| Bounded reads | Pagination cursor and expiry, limits, stable ordering, filters and consistency while data changes. |
| Durable outcomes | Resource/operation IDs, deadlines, pending versus terminal status and correlation identifiers. |
| Stable representations | Field presence, nulls, timezones, enum evolution, errors and compatibility/deprecation policy. |

**Proposal:** SDKs should preserve cancellation, error details, pagination and
operation status rather than hiding them behind success booleans. Scripts need
machine-readable output and documented retry behaviour. Neither requires a
separate API dialect or permission to use private worker commands.

**Proposal:** contract tests should verify these observable promises through
REST, including scope denial, pagination, repeated writes, stale updates and
asynchronous failure. Domain tests can exercise transport-neutral Go commands;
worker tests can exercise private contracts. Those test seams do not change the
supported client boundary or introduce test-only production bypass routes.

## Detailed design and validation

| Topic | Required behaviour | Remaining engineering work |
| --- | --- | --- |
| Machine authority | Independent application grants, without personal desk check-in. | Define exact capability, office, owner and data projections per operation. |
| Authentication | MCP retains user context; machine scripts use application identity. | Select concrete credential flows and deployment ownership for enrolled clients. |
| Terraform | Long-lived workplace/permitted configuration and useful data sources. | Define resource identities, schemas, import, impact and deletion behaviour. |
| Drift and concurrency | Authorised UI edits remain valid; updates require the version read. | Test refresh/update behaviour against concurrent changes and source-owned records. |
| Long-running work | Public operation references and cancellation requested until execution stops, ordered against commits/publication. | Define concrete completion/progress/result schemas and validate cancellation/commit ordering. |
| Retry and load | Atomic acceptance receipts and actor budgets across clients. | Specify fingerprint/retention details and validate bursts, polling and backpressure. |
| Compatibility | Public support independent of portal release; initial 180-day retirement notice. | Exercise generated SDK/provider clients against supported mixed versions. |

[mcp-auth]: https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization
[mcp-security]: https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/security-considerations
[mcp-tools]: https://modelcontextprotocol.io/specification/2026-07-28/server/tools
[mcp-annotations]: https://modelcontextprotocol.io/specification/2026-07-28/schema#toolannotations
[tf-resources]: https://developer.hashicorp.com/terraform/plugin/framework/resources
[tf-read]: https://developer.hashicorp.com/terraform/plugin/framework/resources/read
[tf-identity]: https://developer.hashicorp.com/terraform/plugin/framework/resources/identity
[tf-import]: https://developer.hashicorp.com/terraform/plugin/framework/resources/import
[tf-create]: https://developer.hashicorp.com/terraform/plugin/framework/resources/create
[tf-update]: https://developer.hashicorp.com/terraform/plugin/framework/resources/update
[tf-delete]: https://developer.hashicorp.com/terraform/plugin/framework/resources/delete
[tf-equality]: https://developer.hashicorp.com/terraform/plugin/framework/handling-data/types/custom#semantic-equality
[tf-timeouts]: https://developer.hashicorp.com/terraform/plugin/framework/resources/timeouts
[http-202]: https://www.rfc-editor.org/rfc/rfc9110.html#section-15.3.3
[http-retry]: https://www.rfc-editor.org/rfc/rfc9110.html#section-9.2.2
[http-match]: https://www.rfc-editor.org/rfc/rfc9110.html#section-13.1.1
