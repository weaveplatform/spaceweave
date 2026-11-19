# Entra group facts for configurable SpaceWeave RBAC

SpaceWeave maps explicitly configured SCIM-provisioned group membership to
scoped application roles. This research explains the Microsoft Entra and SCIM
semantics needed to implement that mapping correctly: stable identity, membership
scope, nesting, freshness and revocation. It separates provider facts from
SpaceWeave's application policy. Sources were checked on 19 November 2026.

## Product context and terminology

SpaceWeave RBAC is configurable. MCP acts in the signed-in user's context;
machine-identity scripts use a separate channel. Public REST is shared by its
consumers, while worker APIs remain private. App-only facts below concern machine
identities or a backend's directory lookup, not an app-only MCP product profile.
Read with [client authentication](client-authentication.md) and
[provider contracts](provider-contracts.md); this note chooses no grants or SDKs.

The [glossary](../../../GLOSSARY.md) distinguishes externally maintained teams
used for proximity allocation from allocation groups formed inside SpaceWeave.
**Product boundary:** neither becomes an authorisation group merely because it
exists or is provisioned. A configured mapping must explicitly give a particular
external group an authorisation purpose; reusing a team is a policy choice.

SCIM does not supply that policy: its Group schema represents membership but
explicitly leaves resulting authorisation semantics to the service provider.
SCIM's ability to represent nested groups does not establish an Entra connector's
support for provisioning them. [RFC 7643 §§4.1.2, 4.2][scim-schema].

## Verified facts: roles, groups and actor identity are distinct

| Concept | Provider meaning and consequence |
| --- | --- |
| Delegated permission / `scp` | Client access on behalf of a signed-in user; both client permission and user authority matter. It does not itself assign a SpaceWeave role. [Permission model][permissions]. |
| App-only permission | An application acts without a signed-in user; application permissions use app roles. A script does not inherit a person's group authority by naming that person. [Permission model][permissions]. |
| Entra app role / `roles` | A role defined for an app/API and assigned to users, groups of users, or application service principals. Its configured value appears in the appropriate token. [App roles][app-roles]. |
| Entra group / `groups` | Directory membership evidence, optionally emitted in tokens; it is not an app role definition. [Claims reference][claims]. |
| Entra directory role / `wids` | Tenant-wide Entra role identifiers, distinct from application roles and configurable SpaceWeave roles. [Claims reference][claims]. |

User tokens can contain `roles` as well as `scp`; `roles` presence alone does not
identify an app-only request. Microsoft recommends distinct user/application role
definitions where both are supported. [Scope and role validation][role-checks].
Also, putting a service principal in a group and assigning that group an app role
does **not** cause Entra to emit that role in the principal's tokens.
[App-role service-principal limitation][app-roles].

An API must validate audience, tenant, subject and client actor. Tenant plus `oid`
alone can accidentally authorise service principals as users. Microsoft documents
the optional `idtyp` claim for validating app-only identity when authorising by
client application ID. These checks precede applying an RBAC mapping.
[Claim validation][claim-validation].

## Verified facts: stable identifiers and SCIM correlation

Microsoft recommends group ObjectID for in-app authorisation: it is immutable,
unique and available for all groups. Names and synced on-premises attributes have
different availability/collision properties. [Group identifier guidance][group-claims].
For users, `oid` identifies a user or service principal; a person represented in
multiple tenants has a different object ID in each. Use tenant context when
correlating identity; Microsoft recommends `tid` plus `oid` for application data.
[Token identifiers][claims], [tenant/subject validation][claim-validation].

SCIM `id` is issued by the target service provider, stable and non-reassignable;
`externalId` is issued by the provisioning client and scoped to its provisioning
domain. They are not automatically equal to an Entra object ID. Correlation must
follow the configured mappings. [RFC 7643 §3.1][scim-schema].
**Proposed representation:** key each external authorisation mapping by its
trusted Entra tenant and group ObjectID, retaining names as labels and explicit
links to any SCIM resource. Renames must not change the selected authority.

## Verified facts: direct and transitive membership differ by surface

For a user directly in group B, where B is a member of group A:

| Surface | Documented membership behaviour |
| --- | --- |
| Ordinary configured group claims | Can include both A and B through nesting, subject to configured group types, filters and overage. [Group claims][group-claims]. |
| `Groups assigned to the application` claim mode | Includes assigned groups of which the user is a **direct** member; nesting does not qualify A. [Assigned-group claims][token-groups]. |
| Enterprise-app/group-to-app-role assignment | Assignment does not cascade through nested groups; assign the user or a group containing them directly. Group-based app assignment requires P1/P2. [Assignment rules][assignments]. |
| Entra outbound provisioning scope | With assigned-user/group scope, provisioning reads immediate users of explicitly assigned groups; it cannot read/provision users through nested groups. [Provisioning behaviour][provisioning]. |
| Graph `memberOf` | Lists direct memberships only; results can include groups, directory roles and administrative units. [Direct membership][graph-direct]. |
| Graph `transitiveMemberOf` | Lists direct and transitive memberships, also including non-group directory objects. [Transitive membership][graph-transitive]. |
| Graph `checkMemberGroups` | Tests a supplied set of group IDs **transitively**, at most 20 per request; it is not a direct-membership check. [Membership test][graph-check]. |

Consequently, substituting transitive Graph results for direct app-assignment or
SCIM evidence can broaden membership. This is an inference from the documented
contracts above, not an Entra equivalence guarantee. **Proposed requirement:** a
mapping records the selected membership semantics and preserves them on fallback.

The agreed SCIM Users/Groups contract remains intact. Provisioning scope, group
attribute mappings and an authorisation mapping serve different purposes;
provisioning a group alone grants no SpaceWeave administrative capability.
[Existing provider contract](provider-contracts.md#entra-to-spaceweave-scim-20-users-and-mapped-groups),
[SCIM authorisation boundary][scim-schema].

## Verified facts: token overage and Graph lookup constraints

JWT group emission is limited to 200 groups, including nested groups; SAML uses
150. Over the limit, Entra omits the groups array and signals overage instead.
An absent `groups` claim is therefore not sufficient evidence of no memberships.
[Group limits][group-claims], [overage reference][claims].
Microsoft documents detecting `hasgroups` or `_claim_names.groups` and retrieving
membership through Microsoft Graph. The `_claim_sources` URL can still reference
legacy Azure AD Graph; do not blindly follow that value. Narrowing claims to
assigned groups changes nesting semantics as described above.
[Overage handling and assigned groups][token-groups].

Graph lookup needs a Graph-audience token and permissions for the selected
operation; a SpaceWeave-audience token does not grant Graph access. APIs accept
tokens intended for themselves. [Audience ownership][tokens].
`/me/memberOf` and `/me/transitiveMemberOf` require delegated identity; the
documented least permission for the signed-in user is `User.Read`. Looking up a
different user or using application identity has different permission tables.
[Direct lookup permissions][graph-direct], [transitive lookup permissions][graph-transitive].
Graph can return only IDs/types for resource types the caller cannot fully read;
hidden-membership access has additional requirements. Some membership checks can
omit groups the caller cannot evaluate. A successful response therefore needs its
documented visibility limits understood. [Membership visibility][graph-transitive],
[hidden-group evaluation][graph-check]. Enumerations can require multiple pages;
follow `@odata.nextLink` to completion. [Graph paging][paging].

## Verified facts: freshness and revocation are separate boundaries

| Evidence or action | Documented limit |
| --- | --- |
| Group claims in an acquired token | Membership is a snapshot at token acquisition; existing token contents do not update with directory changes. [Token group freshness][token-groups]. |
| Access-token expiry | Default Entra lifetime varies, commonly 60–90 minutes; this is not a universal one-hour bound or a SpaceWeave grant-freshness guarantee. [Token lifetime][tokens]. |
| Graph advanced membership queries | Search/filter/cast queries use `ConsistencyLevel: eventual`; the index might not reflect recent changes. A lookup is not an instantaneous propagation guarantee. [Direct][graph-direct], [transitive][graph-transitive]. |
| SCIM provisioning | Initial and periodic incremental cycles apply; the non-gallery guide describes approximately 40-minute subsequent cycles. Duration varies with workload. This is not a maximum revocation SLA. [SCIM schedule][scim-guide], [duration factors][provisioning-status]. |
| Disable an app-role definition | Existing assignments remain and the role can continue in tokens; assignments must be removed as well. [App-role disable behaviour][app-roles]. |
| Revoke Entra sessions | Entra cannot directly revoke an application's own cookie; the application controls its session authorisation. [Revocation boundary][revocation]. |
| Continuous access evaluation | Requires participating services and supported events; published Microsoft-service behaviour does not establish immediate group-removal enforcement in custom SpaceWeave REST. [CAE scope][cae]. |

**Inference:** directory propagation, token acquisition, SCIM delivery, local
membership caching, role mapping and application sessions can each delay removal
of authority. None of these sources establishes an end-to-end SpaceWeave bound.
**Proposed requirement:** state a freshness/revocation contract for each chosen
source and for local mapping changes, with an explicit emergency-disable path.

## Selected source and alternative membership mechanisms

Configured group-to-role mappings use **SCIM-provisioned membership**. Global
administration manages those mappings; roles compose grants within complete
capability/scope pairings. Self-permission discovery informs clients while
commands revalidate current authority. Direct grants retain independent source
and lifecycle information. See [configuration requirements](access-channels-and-rbac.md#configuration-and-operational-requirements).

The additional provider mechanisms researched above are facts, not extra authority
sources in the application design:

| Alternative not adopted as another mapping source | Boundary |
| --- | --- |
| Entra groups assigned to Entra app roles | App-role token mechanics do not replace the selected SCIM-backed local mappings. |
| Token group claims or Graph membership lookup | Their nesting, visibility and freshness differ; do not silently substitute them for provisioned membership. |
| Hybrid membership sources | No automatic combination/fallback is approved. Retain the selected source semantics and explicit local grants. |

Verify concrete tenant/connector scope, stable group correlation and membership
delivery against the cited provider prerequisites during integration. Do not
infer nested membership that the connector has not delivered, or treat stale/
missing data as an authoritative empty group. Source observation/freshness and
local grant invalidation remain distinct. Exact deployment settings are evidence
to establish during integration, and should be validated before production rollout.

[permissions]: https://learn.microsoft.com/en-us/entra/identity-platform/permissions-consent-overview
[app-roles]: https://learn.microsoft.com/en-us/entra/identity-platform/howto-add-app-roles-in-apps
[claims]: https://learn.microsoft.com/en-us/entra/identity-platform/access-token-claims-reference
[claim-validation]: https://learn.microsoft.com/en-us/entra/identity-platform/claims-validation
[role-checks]: https://learn.microsoft.com/en-us/entra/identity-platform/scenario-protected-web-api-verification-scope-app-roles
[group-claims]: https://learn.microsoft.com/en-us/entra/identity/hybrid/connect/how-to-connect-fed-group-claims
[token-groups]: https://learn.microsoft.com/en-us/security/zero-trust/develop/configure-tokens-group-claims-app-roles
[assignments]: https://learn.microsoft.com/en-us/entra/identity/enterprise-apps/assign-user-or-group-access-portal
[provisioning]: https://learn.microsoft.com/en-us/entra/identity/app-provisioning/how-provisioning-works
[scim-schema]: https://www.rfc-editor.org/rfc/rfc7643.html
[graph-direct]: https://learn.microsoft.com/en-us/graph/api/user-list-memberof?view=graph-rest-1.0
[graph-transitive]: https://learn.microsoft.com/en-us/graph/api/user-list-transitivememberof?view=graph-rest-1.0
[graph-check]: https://learn.microsoft.com/en-us/graph/api/directoryobject-checkmembergroups?view=graph-rest-1.0
[paging]: https://learn.microsoft.com/en-us/graph/paging
[tokens]: https://learn.microsoft.com/en-us/entra/identity-platform/access-tokens
[scim-guide]: https://learn.microsoft.com/en-us/entra/identity/app-provisioning/use-scim-to-provision-users-and-groups
[provisioning-status]: https://learn.microsoft.com/en-us/entra/identity/app-provisioning/application-provisioning-when-will-provisioning-finish-specific-user
[revocation]: https://learn.microsoft.com/en-us/entra/identity/users/users-revoke-access
[cae]: https://learn.microsoft.com/en-us/entra/identity/conditional-access/concept-continuous-access-evaluation
