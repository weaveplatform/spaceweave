# Separate human and application authority

Support delegated human identities and independently authorised application
identities on the public REST API. Preserve the actual actor, client and booking
owner separately; an application grant does not inherit a human's delegation or
administrative permissions. This supports unattended consumers without shared
human identities or machine impersonation, while requiring explicit grant and
audit models. App-only credentials cannot perform personal desk check-in.

SpaceWeave RBAC is configurable. MCP acts in the initiating user's context;
scripts using machine identity use a separate machine-authority channel.
Consumer type, authentication journey and authority subject are different
dimensions. No single interactive login journey is selected for all clients,
and missing user context does not authorise MCP to switch to machine authority.

Existing authorised global administration configures roles and scoped assignments.
Support explicit group-to-role mappings from SCIM-provisioned membership, retaining
source provenance separately from direct grants and team/allocation membership.
Roles initially compose grants within complete capability/scope pairings, without
a discretionary deny layer. Mandatory identity, eligibility and domain rules
still apply. Clients can discover their own effective capabilities/scopes; each
command revalidates current authority.
