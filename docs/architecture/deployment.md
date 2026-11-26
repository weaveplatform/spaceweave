# Deployment, environments and UK residency

spaceweave is deployed for **one organisation**, not run as multi-tenant SaaS. It runs in UK data
centres on Azure or Google Cloud. This page covers where each piece runs and what disaster recovery
can look like without data leaving the UK.

![Deployment, environments and UK residency](diagrams/deployment-residency.drawio.svg)

Source: [`diagrams/deployment-residency.drawio`](diagrams/deployment-residency.drawio)

## Environments

| Environment | Isolation | Data | Purpose |
|---|---|---|---|
| Development | Its own subscription (Azure) or project (GCP) | Synthetic only | Day-to-day work; one zone, small node pools |
| Staging | Its own subscription or project | No live personal data | Release candidates and acceptance runs, in the same shape as production |
| Production | Its own subscription or project | Live personal data | UK region across three zones; every deploy needs an approval |

Images are promoted between environments by digest, never rebuilt (see [supply chain](supply-chain.md)).

## UK regions

| | Azure | Google Cloud |
|---|---|---|
| Deployment target | **UK South** (London, 3 availability zones). Everything runs here | **europe-west2** (London, zones a, b and c). Everything runs here |
| UK regions | UK South and **UK West** (Cardiff, no availability zones) are a region pair. UK West holds only DR copies | europe-west2 is the only Google Cloud region in the UK |
| In-region resilience | AKS node pools across zones 1–3; zone-redundant PostgreSQL HA; zone-redundant storage | GKE regional cluster across three zones; Cloud SQL HA with a standby in a second zone; a regional bucket |
| Disaster recovery inside the UK | UK West: geo-redundant PostgreSQL backups or a cross-region replica, and geo-replicated storage. The cluster is rebuilt from code | **Not available as a second region.** Losing europe-west2 means restoring from backups once it returns |

### Azure: a pair is not a failover

Microsoft says deploying to a region pair gives no automatic resilience, high availability or
failover. DR to UK West is a plan someone runs:
- promote the database replica or restore the geo-backup;
- rebuild the cluster from infrastructure code;
- move DNS at the edge.

UK West has no availability zones. Front Door Private Link is offered only in regions that have them,
so after a failover, the private origin needs re-planning. One option is a public origin locked to Front
Door until UK South returns.

### Google Cloud: decide before go-live

With one UK region, the choice is:
1. **Stay in the UK.** Three zones cover the loss of a data centre. Loss of the whole region is accepted
   as a risk, with recovery from backups held in europe-west2.
2. **Standby outside the UK,** for example europe-west1 in Belgium. This holds personal data outside the
   UK, so the data protection officer must decide it, and it must be recorded (see [personal data](personal-data.md)).

**Watch the Cloud SQL backup location.** By default Cloud SQL stores backups in a multi-region
location, which can be outside the UK. Set a custom backup location of europe-west2.

## Where each store lives

| Store | Contents | Azure | Google Cloud |
|---|---|---|---|
| PostgreSQL | All state, the job queue and the outbox | UK South, replica or backups in UK West | europe-west2, with the backup location pinned |
| Object storage | Drawings, renders, exports, archives | UK South, geo-replica in UK West | Regional bucket, europe-west2 |
| Secrets | Graph credentials, webhook signing secrets | Key Vault, UK South | Secret Manager regional secrets, europe-west2 |
| Images | Signed container images | Container Registry, UK South | Artifact Registry, europe-west2 |
| Logs and audit | Telemetry and the SIEM | The organisation's Azure Monitor and Sentinel workspace | Cloud Observability and Security Operations, in its configured location |

Microsoft 365 data (Exchange calendars, Entra ID) stays in the organisation's Microsoft 365 tenant.
spaceweave holds only what it copies in (see [personal data](personal-data.md)).

## Sources

- [Azure region pairs](https://learn.microsoft.com/en-us/azure/reliability/regions-paired):
  "doesn't automatically make them more resilient"
- [Azure regions and availability zones](https://learn.microsoft.com/en-us/azure/reliability/regions-list)
- [Front Door Private Link regions](https://learn.microsoft.com/en-us/azure/frontdoor/private-link)
- [Google Cloud regions and zones](https://docs.cloud.google.com/compute/docs/regions-zones)
- [Cloud SQL backups](https://docs.cloud.google.com/sql/docs/postgres/backup-recovery/backups);
  [backup location](https://docs.cloud.google.com/sql/docs/postgres/backup-recovery/backing-up)
