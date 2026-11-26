# Architecture research

spaceweave is deployed for one organisation, in UK data centres on Azure or Google Cloud. Microsoft
Entra ID is the only identity provider, and Exchange Online holds room calendars. Every diagram has an
editable `.drawio` source next to its `.drawio.svg` in [`diagrams/`](diagrams/).

| Document | What it covers |
|---|---|
| [Domain model](../research/market/domain-model.md) | The nine pillars and their entities ([diagram](diagrams/common-domain-model.drawio.svg)), and how space allocation works ([diagram](diagrams/space-allocation-lifecycle.drawio.svg)) |
| [Integration architecture](integration-architecture.md) | Every system-to-system integration: single view, system context and containers |
| [Network and trust boundaries](network.md) | Edge, private origin, private endpoints, fixed egress, trust boundaries TB1–TB4 |
| [Identity and Conditional Access](identity.md) | Each kind of caller, its Entra registration and credential, the Conditional Access policy that applies (or why none does), and what spaceweave checks itself |
| [Deployment and UK residency](deployment.md) | Environments, UK regions, disaster recovery, where each store lives |
| [State machines](state-machines.md) | BookingOccurrence, SpaceRequest, WaitlistEntry and the versioned artefacts |
| [Integration sequences](sequences.md) | Room panel through Exchange, assistant through MCP, outbox to webhook, Entra SCIM |
| [Personal data and privacy](personal-data.md) | Each category of personal data, retention, who sees it, DPIA notes |
| [Floor-plan pipeline](floor-plan-pipeline.md) | From a drawing to a published revision without breaking bookings |
| [Threat model](threat-model.md) | STRIDE threats per element and boundary, with mitigations |
| [CI/CD and supply chain](supply-chain.md) | Quality gate, release, signing, provenance and admission control |
