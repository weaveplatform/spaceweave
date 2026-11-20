# Access channels and configurable RBAC

SpaceWeave serves staff, booking delegates, administrators, event visitors and
organisation-authorised applications. This document defines how a caller's
identity becomes permission to read or change a particular resource. It is an
application design for architects and engineers; the [protocol research](client-authentication.md)
and [Entra group research](authorization-group-facts.md) establish provider behaviour.

Role-based access control (RBAC) grants implemented capabilities within office,
resource or owner scopes. MCP operates in the initiating user's context, while
unattended scripts can use separately approved application identities. Consumer,
login mechanism and authority remain separate dimensions.

## Four dimensions to keep separate

| Dimension | What it means | Examples |
| --- | --- | --- |
| Consumer | Software initiating/transporting a request. | UI, MCP server, SDK, script, Terraform provider, test client. |
| Authentication journey | How the relevant execution context establishes or continues its credentials. | Existing UI SSO/session, MCP host's delegated user context, unattended workload authentication. |
| Authority subject/channel | Whose current authority the API evaluates. | User actor; application actor; private worker execution; restricted visitor; verified provider. |
| Protocol mechanism | How credentials/messages cross a particular interface. | Session cookie, correctly addressed user access token, application access token, provider callback validation. |

Neither an SDK language nor a transport's `Bearer` header determines the actor.
A confidential MCP service can authenticate itself while obtaining a delegated
token representing a user; that does not turn the business action into app-only
automation. See [client authentication facts](client-authentication.md).

## Identity and authority matrix

| Execution context | Initiating actor | Effective authority | Journey/credential consequence |
| --- | --- | --- | --- |
| Staff/admin UI | The signed-in user. | Current user capabilities/scopes, owner relationship and human delegation. | Keep the agreed Go-owned SSO/session/token handling and browser CSRF controls. |
| MCP operating for a user | The current user; retain MCP client/server provenance separately. | That user's current roles, office scopes and applicable owner/delegate grants, subject to client restrictions. | Use the host/topology's valid user context for downstream REST. Do not prescribe a fresh system-browser login per tool call or fall back to machine permissions. |
| Script using machine identity | The application actor. | Independently approved, configurable application RBAC and office/owner/data scope. | Non-interactive machine/workload authentication; no human or personal delegate authority is implied. |
| SDK or Terraform code | Determined by its actual execution context. | User context when genuinely acting as that user; machine context when using its own approved identity. | The library/provider label does not choose a journey. Machine automation does not launch an assumed interactive login. |
| Platform worker | Preserve the accepted operation's initiating actor/client where applicable; record the executing workload separately. | Private owning-interface/workflow authority and current domain constraints. | Private Go/job contracts and scoped runtime credentials; not a public-client escape hatch. |
| Internal-event visitor | The verified visitor. | The associated internal event, visit/ticket and allowed window. | Existing invitation/email verification; guest speakers do not acquire staff roles through the invitation. |
| Graph/Teams callback | The verified provider/source. | Its narrow observation/activity contract. | Native callback validation, distinct from user or machine business-API permission. |

MCP is a user-context consumer in the agreed product profile. Generic protocol
possibilities do not add app-only MCP here. If user context is unavailable,
expired or insufficient, follow the supported user-context recovery/denial path;
do not silently retry with a broader application grant. Missing authority never
becomes permission through tool annotations, a model instruction or an owner ID.

MCP and REST token audiences still require correct validation. A hosted middle
tier may need a delegated downstream exchange; a local STDIO host can have a
different credential boundary. The hosting topology chooses that mechanism.
None of those facts selects a universal journey. The
[MCP/automation research](automation-consumers.md) and
[Conditional Access facts](conditional-access.md) record the relevant constraints.

## Configurable SpaceWeave RBAC

SpaceWeave uses configurable RBAC and global approval of machine capabilities,
office scope and permitted owner populations. Application RBAC is SpaceWeave's
authorisation model; it is distinct from an Entra token's app-role claim and
from a particular SDK or client registration.

Proposed configuration records make the agreed behaviour reviewable:

| Record | Meaning | Important limit |
| --- | --- | --- |
| Capability catalogue | The supported actions understood by the application. | A custom role selects implemented capabilities; it cannot invent public endpoints or expose private worker functions. |
| Role definition | Stable role identity/name and configurable capability set. | Authorised global administration manages RBAC. |
| Scoped role assignment | A role granted to the appropriate person/application with its office/resource/owner scope and provenance. | Scope belongs to that assignment; assignments are not flattened into an unrelated capability/office cross-product. |
| Application approval/grant | Approved machine identity, actions and permitted office/owner population. | No inherited human role or PA relationship. Global approval is required; exact fields and role linkage remain detailed design. |
| SCIM-backed source mapping | Explicit mapping from configured provisioned group membership to scoped roles. | SCIM supplies the configured membership evidence. Preserve source provenance; no automatic team/allocation-group permission mapping or new token/Graph authority source. |
| Change/audit record | Who changed a definition/assignment, effective policy version and previous/current state. | Authorisation changes are distinct from pinned booking rules or public API version support. |

Entra registration, consent and Conditional Access remain identity-management
controls; they do not replace SpaceWeave's configurable capabilities/scopes.
An in-app global capability does not create an Entra administrator role.
Public clients do not submit authoritative role, actor or channel assertions in
their JSON bodies. Derive identity/profile from validated credentials and load
current applicable policy/grants.

Role configuration belongs with authorisation ownership in each candidate,
not with floor definitions or a new public copy of worker internals:

| Candidate | Proposed RBAC configuration owner |
| --- | --- |
| Layered | Identity application service, distinct People/Access/Delegation facets and owner-labelled Identity persistence. |
| Hexagonal | Identity core service with distinct driving/storage facets; pure access rules remain in the core. |
| Clean | Identity operation family and owned repositories; pure authority/staff models remain inward. |
| DDD | Identity context owns source membership, current eligibility, sessions and roles/assignments under one owner. |
| Modular monolith | Identity module owns roles/assignments, source correlations, sessions and delegation through distinct published interfaces. |

These placements show how each architecture candidate can own the same access
model. Library selection and final module boundaries remain separate decisions.

## Evaluation and scope preservation

The following evaluation order is a proposal explaining the agreed separation:

1. Validate the surface's credential/protocol and derive the actual subject,
   client, authority channel and relevant authenticated context.
2. Check current staff/visitor/application eligibility and the hard restrictions
   for that subject/surface. A locally known inactive staff member stays denied.
3. Resolve current role assignments and, where required, client/application
   approval within that channel. MCP resolves the user's roles, not a machine
   role substituted because the user lacks authority.
4. Match the requested action/resource against a complete applicable scoped
   grant. Apply owner/delegation/source-freshness checks where relevant.
5. Apply business constraints: current booking state, capacity, attendance
   choices, required parking, event cap, check-in ownership and policy versions.
6. Record the initiating actor, client, owner, applicable grant/policy provenance
   and execution context at the appropriate audit boundary.

Example of why scopes must stay paired:

| Assignment | Effective grant |
| --- | --- |
| Reporter role in office A | Report capability in A. |
| Space editor role in office B | Space-edit capability in B. |

The combination grants neither reporting in B nor space editing in A. A role's
capability and another role's office scope cannot be combined to authorise a
request. Similar pairing applies to owner populations, event/resource scopes
and approved machine capabilities.

The initial role model uses grant-only composition: assignments may grant
capabilities in scope; no configurable discretionary deny-rule layer. This does
not remove mandatory eligibility, identity-channel, privacy or domain checks.
For example, a configured machine role cannot perform personal desk check-in,
and a user with a broad role cannot skip a required owner/delegate relationship.

Role/assignment changes apply to new authorisation decisions after local commit.
A browser session is an authentication context, not a permanent permissions
snapshot. Proposed caches use relevant policy/assignment versions or invalidation;
authoritative command checks do not trust an old UI/MCP capabilities response.
Upstream group-change observation has its own freshness/reconciliation limits.

This differs from existing booking confirmations retaining their operational
policy version: a booking's no-show rule can be pinned without pinning an actor's
old permissions forever. Revoking permission also does not itself delete every
booking that the actor previously created for a staff owner.

## Public API implications

The shared REST contract needs distinct, authorised operations for role/catalogue
queries, role-definition changes, scoped assignments and application approvals.
Exact paths/schemas are proposals; the role-management administrative audience
uses the same public REST contract under the appropriate authority.

The public boundary still excludes private worker primitives such as allocation
commit internals, lease renewal and raw provider-delta application. An authorised
public request may start a supported business operation and observe its status;
the worker executes the private workflow. Configurable RBAC does not make every
private method a selectable public action.

UI, MCP and other clients can query an authorised self-permissions view.
It exposes the caller's effective scoped capabilities and current context
version, plus appropriate resource-level `allowedActions` where useful. It
does not expose tokens, every user's role assignments or raw directory group
data. Tool/action availability is a hint at read time; execution still checks
current permission, state and capacity.

## Configuration and operational requirements

Global administration configures roles, scoped assignments and explicit mappings
from SCIM-provisioned group membership. Direct grants and provisioned grants
retain independent source/lifecycle information. Team membership used for
allocation supplies no implicit administrative authority.

Role composition grants capabilities only within each complete assignment scope.
Mandatory identity, eligibility, privacy and booking rules remain enforced.
Permission discovery is restricted to the caller's effective authority, and every
command revalidates permissions against current resource state.

Version role/assignment changes, audit their source and actor, and test revocation
against cached sessions, permission discovery and repeated commands. The
[request contract](request-behaviour.md) defines configuration concurrency and
retry behaviour; API projections must not expose raw group membership or grants
belonging to other users.
