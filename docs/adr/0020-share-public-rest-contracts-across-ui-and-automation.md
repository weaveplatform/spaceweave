# Share public REST contracts across UI and automation

Expose supported public REST contracts for the UI, MCP servers, SDKs, scripts,
test clients, Terraform and other authorised organisation applications. Keep
platform-worker coordination and application implementation interfaces private.
This gives clients the same business semantics and prevents a UI-only contract
fork, at the cost of supporting independently upgraded clients and explicit
public API compatibility.

Version and support the public REST contract independently of UI releases.
Changes within a major version remain compatible; breaking changes require a
new major version and an explicit migration/deprecation policy. The support
window initially requires 180 days' notice once replacement/migration guidance
is available. Future notice periods are configured organisation-wide; honour
already published support dates.

User-facing access is Internet-accessible and governed by Conditional Access
rules/policies alongside API authentication and resource/capability/delegation
checks. Preserve Go-managed browser sessions; credential transport can differ
between browser and programmatic clients without changing the business contract.
Visitor/provider and non-interactive identity profiles require their own actual
validation mechanisms; they are not silently converted into staff sign-in.
