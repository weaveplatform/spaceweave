# Client authentication for public REST consumers

SpaceWeave exposes one supported business REST contract to staff/admin portals,
MCP servers, SDKs, scripts and Terraform. This research explains how to authenticate
those clients without confusing the consumer software with the person or
application whose authority it uses. Sources checked: 19 November 2026.
Protocol facts cite Microsoft, IETF and official MCP documentation; implementation
proposals and unresolved choices are identified separately.
MCP facts use protocol **2026-07-28**, the checked `latest`; its OAuth 2.1
reference is an IETF draft, not a published RFC. [MCP authorization][mcp-auth].

## Application context and authority

The public REST contract serves the UI, MCP servers, SDKs, scripts, tests and
Terraform; workers retain private interfaces. The API permits delegated human
and independent application identities. An application cannot personally check
in a desk assignee. Terraform manages long-lived configuration; first-party MCP
and provider delivery are follow-on work. These product decisions do not select
exact grants or establish identity-provider guarantees.

**MCP authority:** MCP runs in the initiating user's context. Scripts using
machine identity use a separate application-authority channel. Consumer kind,
authentication journey and authority subject are independent dimensions. The
native-client protocol facts below do not select one browser/PKCE journey for
scripts, SDKs or MCP. See the [identity and authority model](access-channels-and-rbac.md).

[ADR 0015](../../adr/0015-use-react-portals-with-go-managed-sessions.md) retains
Go-owned sessions/provider tokens and secure HttpOnly browser cookies.
[ADR 0020](../../adr/0020-share-public-rest-contracts-across-ui-and-automation.md)
retains shared REST semantics and Internet access under applicable Conditional
Access (CA). Read this with the [CA boundaries](conditional-access.md) and
[consumer review](README.md); this appendix does not replace their decisions.

## Verified facts: client type and actor are separate questions

Microsoft distinguishes a public client, which cannot securely keep a client
credential from its users, from a confidential client that can protect one.
Desktop/mobile code is normally public; a server-hosted web application or
middle tier can be confidential. A distributed CLI's embedded secret does not
make it confidential. [Client application types][client-types].

Delegated access represents a signed-in user: both client permission and user
authority matter. App-only access represents the application without a signed-in
user. OAuth delegation does not itself establish SpaceWeave's separate grant
allowing one person to book for another owner. [Permission model][permissions].

| Consumer/profile | Authentication boundary to preserve |
| --- | --- |
| React staff/admin UI | Existing Go backend-for-frontend session boundary; Go handles provider credentials and the browser sends its session cookie. This is the agreed application design. |
| MCP operating for its user | The user's authority remains the business context; hosted HTTP and local STDIO can have different credential boundaries. [MCP authorization][mcp-auth]. |
| Script using machine identity | Non-interactive approved application identity and configurable machine RBAC; no human is implied. [Client credentials][credentials]. |
| SDK/Terraform execution | The actual configured user or machine context determines the journey; the library/provider label does not select interactive login. |

An SDK's host determines its public/confidential and delegated/app-only profile.
This is an architectural inference, not a separate SDK identity or grant.
[Client types][client-types], [permission model][permissions].

## Verified facts: interactive native clients and loopback redirects

RFC 8252 requires native apps to use an external user agent and public native
clients to implement PKCE. The normal external agent is the system browser,
which keeps authentication in a separate security context. Native clients can
use authorization code flow; Microsoft documents code + PKCE/OIDC for desktop,
mobile and server web applications. [RFC 8252 §§4–6][native], [code flow][code].

PKCE binds code redemption to the client's verifier; it is not client identity
proof or business permission. Current OAuth security guidance recommends
non-revealing challenge methods, currently `S256`, and forbids ROPC/password
grant use. Microsoft separately warns that ROPC cannot satisfy MFA.
[RFC 9700 §§2.1.1, 2.4][security], [ROPC limitations][ropc].

RFC 8252 permits a native application that can open a loopback listener to receive
its redirect on an ephemeral local port. HTTP is allowed because this redirect
stays on the device. It recommends opening the listener only for authorization
and closing it after the response. This is a local callback, not an exposed
SpaceWeave API listener. [RFC 8252 §§7.3, 8.3][native].

Microsoft's desktop guidance gives `http://localhost` for system-browser apps.
Its redirect restrictions also prefer `127.0.0.1`, require manifest editing to
register HTTP with that IP literal, and currently exclude IPv6 loopback. Localhost
port matching has documented exceptions; paths and registered platform types
still matter. Therefore RFC support does not prove every redirect works with
Entra, a chosen library or endpoint policy. [Desktop configuration][desktop],
[redirect restrictions][redirects].

**Conditional option, not a selected journey:** if a distinct native user-client
journey is required, authorization code + PKCE and supported loopback/broker
behaviour are options to evaluate against that journey's environment. This is
not the default journey for a machine-identity script or an MCP tool invocation.
Preserve the UI's Go session boundary. The protocol facts do not choose a
client registration, redirect URI, language library or forced reauthentication.

## Verified facts: device code is a conditional alternative

Device authorization lets an input-constrained client ask the user to sign in
through a browser on another device while the originating client polls for
tokens. The resulting access is still delegated user access; it is not a way
to turn a human account into an unattended application. [Device flow][device].

Microsoft recommends allowing device code only where necessary and blocking it
where possible. CA can restrict or block the flow. Protocol tracking can carry
the original device-code state into subsequent token refreshes/resource access,
so switching a later request to another flow is not a policy bypass.
[Authentication-flow controls][flows].

Microsoft documents that device code cannot satisfy the managed-device/device
state grant requirement because authentication occurs on a different device.
Successful browser MFA alone therefore does not prove a headless client will
satisfy the tenant's complete CA policy. [CA grant controls][grants].

**Proposal boundary:** do not assume device code is available as a universal
fallback when loopback/browser sign-in fails. Decide whether to support it after
reviewing the real tenant policy and required endpoint types. Interactive client
support implies neither password collection nor long-lived personal API keys.

## Verified facts: hosted MCP needs two audience boundaries

HTTP MCP authorization treats the MCP server as a resource server. The client
requests tokens for that MCP resource, supplies its `resource` indicator, and
the server must validate its intended audience. The spec forbids accepting or
transiting tokens for other resources. STDIO has a separate environment-credential
model; HTTP authorization requirements do not automatically describe a local
STDIO process. [MCP authorization][mcp-auth].

Official guidance prohibits token passthrough: forwarding tokens meant for another
API crosses audience boundaries and can create a confused deputy. Co-hosting MCP
and REST does not establish a valid shared OAuth resource. [MCP security][mcp-security].

Microsoft OBO is a supported pattern for a confidential middle-tier API calling
a downstream API for a **user**. Incoming token A must target the middle tier;
the middle tier authenticates to Entra and acquires token B for the downstream
API. It cannot redeem a Graph-audience token as though issued to itself. OBO
uses delegated scopes and does not exchange an app-only principal into a user.
[OBO flow and limitations][obo].

OBO also needs consent for the downstream relationship; the middle tier cannot
interactively obtain it during the API call. It is a possible mechanism where
the actual hosted user-context MCP topology supports it, not a universal journey
selected because the consumer is MCP. The middle tier authenticating itself
does not make its delegated downstream action app-only. SpaceWeave's MCP
profile stays in user context; no machine-authority fallback is adopted. Select
topology, consent, challenge propagation and token ownership against that profile.
[OBO consent][obo].

## Verified facts: unattended credentials and consent are not resource grants

Client credentials obtains app-only tokens, issues no refresh token, and supports
certificates or federated credentials as alternatives to shared secrets. These
credentials belong in a confidential workload, not a distributed native binary.
Its `.default` request targets one resource; it does not grant arbitrary
SpaceWeave actions. [Client credentials][credentials].

Workload federation establishes trust in an external issuer and exchanges its
workload token for an Entra access token. Supported environments include GitHub
Actions, Kubernetes and external-cloud workloads; trust configuration and the
issuer/subject/audience matching rules still apply. **Existing direction:**
prefer federation where the selected runtime supports it; a particular desktop
or CI runtime's support must be verified. [Federation concepts][federation].

Delegated scopes describe client access on behalf of a user. Application
permissions/app roles describe app-only access, and application permissions need
administrator consent. Tenant user-consent policy can also require administrator
approval for delegated permissions. Consent approves the client/resource
relationship; it does not define SpaceWeave offices, booking owners, delegation
or current eligibility. [Permission model][permissions], [consent rules][consent].

An API must validate audience, tenant, subject and actor, then the applicable
scopes/app roles. Checking only tenant plus `oid` can accidentally admit service
principals as users. `roles` can occur in user tokens too; claim presence alone
is not a safe actor-type discriminator. [Claim validation][claims],
[scope and role verification][role-checks].

**SpaceWeave requirement/proposal boundary:** the application must distinguish
the real human/application actor and check the current capability, office,
event and owner/delegate grant for the operation. The exact grant model remains
open. No owner ID parameter or Entra admin consent may turn an app-only actor
into the human assignee needed for personal desk check-in.

## Verified facts: cached tokens, sessions and revocation differ

Access tokens are audience-specific credentials validated by their target API;
clients should treat them as opaque and use token-response expiry metadata.
Microsoft documents a variable default lifetime, commonly 60–90 minutes, rather
than a universal fixed hour. A token's validity does not demonstrate that a
SpaceWeave grant is still current. [Access-token guidance][tokens].

For selected user credential journeys, refresh tokens can expire or be revoked
before expiry; recovery follows that user's supported journey, including
interactive sign-in when required, and protects the token cache. Machine-identity
scripts instead use their credential renewal/token reacquisition path; they do
not acquire a human context through an assumed interactive fallback.
Replacing a refresh token does not automatically revoke the previous one.
[Refresh-token behavior][refresh]. Claims challenges can require a new token and
interactive action; capable clients need bounded challenge handling instead of
replaying a rejected cached token. [Claims challenges][challenge].

Entra cannot directly revoke the application's own session cookie. The app
controls that session and when it sends the user back for Entra reevaluation.
Workload CAE currently names Microsoft Graph as its only supported resource
provider; that is not an immediate-revocation guarantee for custom SpaceWeave
REST. [User revocation][revoke], [workload CAE][workload-cae].

**Unresolved inputs:** allowed client registrations/enrollment; staff and API
audiences; user-consent policy; actual CA controls/licences and broker/platform
support; permitted device-code exceptions; native token-cache policy; app-only
capabilities and office/owner scope; local grant/session revocation and cache
freshness bounds; OBO consent/challenge behavior; emergency disable procedure.
Public documentation cannot determine these tenant/product choices. Removing a
credential, consent, Entra session or SpaceWeave grant must have its own stated
effect and propagation bound; no single action above proves instant revocation.

[mcp-auth]: https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization
[client-types]: https://learn.microsoft.com/en-us/entra/identity-platform/msal-client-applications
[permissions]: https://learn.microsoft.com/en-us/entra/identity-platform/permissions-consent-overview
[code]: https://learn.microsoft.com/en-us/entra/identity-platform/v2-oauth2-auth-code-flow
[native]: https://www.rfc-editor.org/rfc/rfc8252
[security]: https://www.rfc-editor.org/rfc/rfc9700
[ropc]: https://learn.microsoft.com/en-us/entra/identity-platform/v2-oauth-ropc
[desktop]: https://learn.microsoft.com/en-us/entra/identity-platform/scenario-desktop-app-configuration
[redirects]: https://learn.microsoft.com/en-us/entra/identity-platform/reply-url
[device]: https://learn.microsoft.com/en-us/entra/identity-platform/v2-oauth2-device-code
[flows]: https://learn.microsoft.com/en-us/entra/identity/conditional-access/concept-authentication-flows
[grants]: https://learn.microsoft.com/en-us/entra/identity/conditional-access/concept-conditional-access-grant
[mcp-security]: https://modelcontextprotocol.io/docs/2026-07-28/tutorials/security/security_best_practices
[obo]: https://learn.microsoft.com/en-us/entra/identity-platform/v2-oauth2-on-behalf-of-flow
[credentials]: https://learn.microsoft.com/en-us/entra/identity-platform/v2-oauth2-client-creds-grant-flow
[federation]: https://learn.microsoft.com/en-us/entra/workload-id/workload-identity-federation
[consent]: https://learn.microsoft.com/en-us/entra/identity-platform/consent-types-developer
[claims]: https://learn.microsoft.com/en-us/entra/identity-platform/claims-validation
[role-checks]: https://learn.microsoft.com/en-us/entra/identity-platform/scenario-protected-web-api-verification-scope-app-roles
[tokens]: https://learn.microsoft.com/en-us/entra/identity-platform/access-tokens
[refresh]: https://learn.microsoft.com/en-us/entra/identity-platform/refresh-tokens
[challenge]: https://learn.microsoft.com/en-us/entra/identity-platform/claims-challenge
[revoke]: https://learn.microsoft.com/en-us/entra/identity/users/users-revoke-access
[workload-cae]: https://learn.microsoft.com/en-us/entra/identity/conditional-access/concept-continuous-access-evaluation-workload
