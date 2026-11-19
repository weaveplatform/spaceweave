# Conditional Access boundaries for Internet-accessible APIs

Research checked: 19 November 2026. Status: factual appendix plus explicitly
labelled SpaceWeave policy/deployment choices; no implementation is supplied.
Sources below are Microsoft-owned documentation, checked against current pages.

**Application exposure:** public REST, staff and administration endpoints are
Internet accessible under the applicable Microsoft Entra Conditional Access policy.
This research examines which identities, resources and sessions those policies
actually govern, and what SpaceWeave must enforce itself.
The Go server owns provider tokens; browsers use secure cookie sessions.
Scripts, MCP, SDKs and Terraform can consume the public REST contract; workers
use private application interfaces. These are product decisions, not Microsoft
guarantees. See the [surface review](README.md) and
[session ADR](../../adr/0015-use-react-portals-with-go-managed-sessions.md).
MCP runs in its user's context; machine-identity scripts use a separate
authority channel. The policy facts here do not prescribe
a common authentication journey for client labels.

## Verified facts: what a Conditional Access policy protects

Conditional Access (CA) evaluates identity and contextual signals to permit,
require controls, or block access. It operates after first-factor authentication;
it is not a denial-of-service defence for an Internet endpoint. User CA generally
requires Entra ID P1; user/sign-in risk policies require P2. Related Intune and
Workload ID capabilities have their own licensing requirements. [CA overview][ca].

CA targets resources/services, including registered custom applications. A
public/native client is not itself the target resource: policy applies when it
requests the protected service. A confidential web client requesting an ID token
can additionally be subject to policy on that web application. Microsoft Graph
access can encounter policy on its underlying datasets/services, such as Exchange
and Teams. Protecting a Graph request does not establish protection of a separate
SpaceWeave API audience. [Resource targeting][resources], [developer guide][dev].

**Design implication:** enumerate the actual SpaceWeave web-app and API resource
registrations and map policies to them. A hostname, `/api/v1/admin/` prefix, SDK,
Terraform provider or MCP label does not create an independently targeted Entra
resource. Stronger requirements for administrative actions need an explicit
resource/user assignment or application-enforced authentication-context design.
This inference follows the resource model; it does not choose the registrations.

Authentication context allows an application to associate sensitive actions with
tenant-managed context IDs and check the token's `acrs` claim, challenging when
the required context is missing. The application must implement that check and
map actions to contexts. Context presence alone does not prove a particular MFA
policy exists: Microsoft says a requested context can be issued when no policy
is assigned to it. [Authentication-context developer guidance][context].

## Verified facts: users, applications and managed identities differ

| Caller identity | Applicable boundary |
| --- | --- |
| Staff/admin user signing in or calling with delegated access | User CA can apply to the requested resource and selected users/groups; the app still authorises the actor and operation. [Resources][resources], [claims validation][claims]. |
| Unattended application using app-only access | Separate CA for workload identities can cover eligible service principals; user MFA controls are not machine controls. [Workload CA][workload]. |
| Azure managed identity | Excluded from CA for workload identities and workload CAE; use its supported identity and resource access controls. [Workload CA][workload], [workload CAE][workload-cae]. |
| Federated credential used by an app registration | A client-credential authentication option, not a claim that all workload identity types support the same CA controls. [Client credentials][credentials]. |

Workload CA covers owned, single-tenant service principals registered in the
tenant. Microsoft applications, third-party SaaS/multitenant apps and managed
identities are outside that coverage. Policy must target the service principal
directly; assigning a group containing it does not enforce that policy on it.
The relevant object ID is the enterprise application's service-principal object
ID, not the app-registration object ID. [Workload CA][workload].

Workload CA requires Workload Identities Premium to create/modify policies. Its
documented controls block token requests by public IP location or service-principal
risk; **Block access is its available grant control**. Service principals cannot
perform MFA. Do not translate “protected by CA” into a universal MFA requirement
or claim every service principal is covered. [Workload CA][workload].

There is a documentation boundary to resolve before relying on app-only context
integration: the workload CA/resource pages list authentication contexts, whereas
the authentication-context developer guide still describes user-sign-in-only
prerequisites and excludes apps authenticating as themselves. User action
step-up is documented; universal app-only step-up is not established by these
pages. Validate the intended workload scenario before selecting it.
[Workload CA][workload], [resource targeting][resources], [context guide][context].

## Verified facts: grants and interactive-client compatibility

User CA can require MFA/authentication strength, compliant devices or hybrid
joined devices. Administrators can combine selected grant controls with AND or
OR. The supported platform, browser, device-registration and broker conditions
matter; a control available for users is not automatically usable by every API
client. Device-code flow cannot satisfy the documented managed-device/device
state grant requirement. [Grant controls][grants].

Authorization code with PKCE/OIDC supports server web applications and native
clients as well as SPAs; this does not require SpaceWeave to move tokens into its
browser. Device-code authentication can be restricted or blocked by tenant CA.
ROPC/password collection is incompatible with MFA and Microsoft recommends
against it. A headless user-password script is therefore not a reliable way to
satisfy staff CA. [Authorization code][code], [authentication-flow controls][flows],
[ROPC limitations][ropc].

Client credentials obtains app-only access without a signed-in user. Microsoft
supports certificates and federated credentials as alternatives to shared
secrets; this flow does not issue refresh tokens. Its resource must authorise
the application. These facts do not mandate static API keys, choose SpaceWeave's
machine credentials, or permit an application to impersonate any staff owner.
[Client credentials][credentials].

## Verified facts: challenges and continuous evaluation are specific capabilities

An API claims challenge uses `401` and `WWW-Authenticate: Bearer` with
`error="insufficient_claims"`, an authorization URI and a base64-encoded claims
request. The delegated client obtains a fresh token with the requested claims;
interactive user action can be required. Clients must handle challenges rather
than indefinitely replay the rejected cached token. [Claims challenges][challenge].

Client capability `cp1` declares claims-challenge support. An API can request the
optional `xms_cc` claim to detect that capability; it is not a general permission
grant. A Go web app calling downstream resources can encounter CA challenges,
including during silent token acquisition or on-behalf-of exchanges. It must
return the user to the appropriate authentication flow when required.
[Claims challenges][challenge], [CA developer guidance][dev].

User Continuous Access Evaluation (CAE) requires cooperating resource providers
and capable clients; Microsoft's support matrix describes Exchange, SharePoint,
Teams and Graph scenarios and does not establish SpaceWeave as a CAE resource.
Even supported critical-event evaluation can have propagation latency of up to
15 minutes. Ordinary JWT validation or declaring `cp1` does not create that
resource-side event enforcement. [User CAE][cae].

Workload CAE currently supports **Microsoft Graph as resource provider only**,
eligible single-tenant service principals, selected disable/delete/high-risk
events and location/risk policies. It requires client opt-in and claims-challenge
handling, and excludes managed identities. Therefore it cannot be cited as an
immediate revocation guarantee for SpaceWeave's custom REST resource or cookies.
[Workload CAE scope and limitations][workload-cae].

## Verified facts: application sessions and API validation remain necessary

Microsoft distinguishes Entra's browser session from the application's own
cookie. Once an app issues its session, it controls access; Entra cannot directly
revoke an app-issued session token. Sending the user back to Entra allows its
policies to be reevaluated. An indefinitely accepted app cookie can outlive the
opportunity for that reevaluation. [Revoking user access][revoke].

Sign-in frequency depends on the application's authentication/session behaviour
and redirects to Entra. Microsoft's “Every time” setting means every session
evaluation, not every HTTP request; it includes documented prompt/clock-skew
behaviour. Setting tenant policy alone does not install validation into a Go
session middleware. [Session-lifetime controls][sessions].

A protected API must still validate the access token's signature, intended
audience, trusted issuer and validity period. It must validate the tenant,
subject and permitted client, then the delegated scopes or application roles
needed by the operation. A token for Graph or an ID token is not a substitute
for a SpaceWeave API access token. [Protected API configuration][validation],
[token validation][tokens], [scope/app-role checks][permissions].

Only checking tenant and object ID can accidentally admit service principals
alongside users. Microsoft recommends immutable identity keys and warns against
using email/UPN-like display claims for authorisation. SpaceWeave's per-office,
event, owner/delegate and global-capability decisions remain application policy,
not a consequence of satisfying CA. [Claims validation][claims].

## Provider callbacks and email-verified visitors keep separate boundaries

The email-verified visitor session remains a SpaceWeave decision. It is not an
Entra staff sign-in. The design does not require Entra B2B guest accounts or
establish that staff Conditional Access governs visitor email verification. Its invitation, event and visit
scope must still be enforced by the application.

Internet-reachable provider endpoints remain provider-validated. Graph requires
public HTTPS webhook connectivity, notification-URL validation and notification
validation such as matching the subscription's `clientState`. Teams/Bot Connector
activities require the connector's service authentication/JWT validation. SCIM
uses its configured provisioning authentication. A staff interactive sign-in
requirement cannot replace these protocols. See [provider contracts](provider-contracts.md),
[Graph webhooks][graph], [Bot authentication][bot], [Entra SCIM integration][scim].

## Proposed SpaceWeave policies and deployment inputs

The following deployment choices require explicit configuration and validation;
Microsoft's platform behaviour does not select them for the application:

- Map staff web, admin web and REST audiences to actual tenant app/service
  principals; choose resource separation or user authentication contexts where
  stronger administrative requirements are wanted, with no unprotected alternate
  route to the same action. Map SpaceWeave capabilities separately from Entra roles.
- Select user/group assignments, exclusions, grant combinations, allowed device
  and authentication flows, named locations, licensing and policy ownership.
  Evaluate actual user journeys, including MCP's user context, against user
  policy; evaluate machine-identity scripts against the applicable workload
  controls. SDK/provider labels do not select an interactive journey.
- For each unattended caller, choose an eligible identity, credential mechanism,
  resource permissions and app grants. Decide whether workload CA is required
  and supported; managed identity/federation are not interchangeable CA promises.
- Define cookie idle/absolute lifetimes, a bounded Entra revalidation cadence,
  challenge recovery and behaviour during identity-provider failure. Validate
  session expiry/revocation and current application authority on each request;
  do not confuse this local check with Entra reevaluation on every request.
- Connect staff deactivation and grant removal to session/API eligibility checks;
  document the resulting propagation bound, cache expiry and emergency revocation
  procedure. Require fresh applicable authentication before selected sensitive
  actions if that is the chosen policy. Deny locally observed effective disables
  immediately; distinguish that enforcement from upstream observation latency.
- Keep visitor, provisioning, Graph and Teams authentication independently
  specified and reachable. Keep worker and operational interfaces private as
  designed. Public reachability changes neither provider verification nor business
  authorisation, and creates no requirement for static general-purpose API keys.

[ca]: https://learn.microsoft.com/en-us/entra/identity/conditional-access/overview
[resources]: https://learn.microsoft.com/en-us/entra/identity/conditional-access/concept-conditional-access-cloud-apps
[dev]: https://learn.microsoft.com/en-us/entra/identity-platform/v2-conditional-access-dev-guide
[context]: https://learn.microsoft.com/en-us/entra/identity-platform/developer-guide-conditional-access-authentication-context
[workload]: https://learn.microsoft.com/en-us/entra/identity/conditional-access/workload-identity
[grants]: https://learn.microsoft.com/en-us/entra/identity/conditional-access/concept-conditional-access-grant
[code]: https://learn.microsoft.com/en-us/entra/identity-platform/v2-oauth2-auth-code-flow
[flows]: https://learn.microsoft.com/en-us/entra/identity/conditional-access/concept-authentication-flows
[ropc]: https://learn.microsoft.com/en-us/entra/identity-platform/v2-oauth-ropc
[credentials]: https://learn.microsoft.com/en-us/entra/identity-platform/v2-oauth2-client-creds-grant-flow
[challenge]: https://learn.microsoft.com/en-us/entra/identity-platform/claims-challenge
[cae]: https://learn.microsoft.com/en-us/entra/identity/conditional-access/concept-continuous-access-evaluation
[workload-cae]: https://learn.microsoft.com/en-us/entra/identity/conditional-access/concept-continuous-access-evaluation-workload
[revoke]: https://learn.microsoft.com/en-us/entra/identity/users/users-revoke-access
[sessions]: https://learn.microsoft.com/en-us/entra/identity/conditional-access/concept-session-lifetime
[validation]: https://learn.microsoft.com/en-us/entra/identity-platform/scenario-protected-web-api-app-configuration
[tokens]: https://learn.microsoft.com/en-us/entra/identity-platform/access-tokens
[permissions]: https://learn.microsoft.com/en-us/entra/identity-platform/scenario-protected-web-api-verification-scope-app-roles
[claims]: https://learn.microsoft.com/en-us/entra/identity-platform/claims-validation
[graph]: https://learn.microsoft.com/en-us/graph/change-notifications-delivery-webhooks
[bot]: https://learn.microsoft.com/en-us/azure/bot-service/rest-api/bot-framework-rest-connector-authentication
[scim]: https://learn.microsoft.com/en-us/entra/identity/app-provisioning/use-scim-to-provision-users-and-groups
