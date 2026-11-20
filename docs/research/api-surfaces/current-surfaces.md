# Current-system API and consumer inventory

The implemented SpaceWeave application is a local floor-plan conversion,
tracing and review workbench. The wider workplace-booking platform introduces
staff identity, capacity management, enterprise integrations and distributed
operation. This inventory establishes which current HTTP, file, browser, CLI
and worker contracts must be preserved or deliberately replaced during that work.

Source baseline: 19 November 2026. Route and behaviour claims cite this application's
source code. Proposed destinations map the existing responsibilities into each
[architecture candidate](../architecture-options/README.md); a destination path
is separate from an implemented booking service.

## Implementation baseline

| Constraint | Current baseline | Evidence |
| --- | --- | --- |
| Go package layout | Public importable packages in `pkg`, project-only packages in `internal`, entrypoints in `cmd`. | [Engineering conventions](../../../CLAUDE.md) |
| Runtime/tooling | Chi HTTP, Cobra CLI without Viper, strict YAML configuration. | [Engineering conventions](../../../CLAUDE.md) |
| Public SDK | `pkg` contains guidance; it does not yet publish a floor-plan client or booking SDK. | [Public-package guidance](../../../pkg/README.md) |

## Implemented HTTP ingress

All application route registration lives in `Server.Handler`, not in OpenAPI or
module API packages ([server.go](../../../internal/server/server.go#L85)).
Seven explicit API route patterns produce ten concrete operations when the four
review actions are counted separately. Profile routes require a non-nil library.

| Method | Path | Existing behaviour and response | Source |
| --- | --- | --- | --- |
| GET | `/api/info` | `200` runtime version/tracer, optional Claude model/spend cap, max upload bytes, accepted formats, active profile and profile capability. | [info](../../../internal/server/server.go#L192) |
| POST | `/api/plans` | Multipart field `image`; accepts image **or drawing**; ingest/deduplicate by SHA-256; `201 {plan,duplicate:false}` or `200 {plan,duplicate:true}`; wakes worker for new input. | [upload](../../../internal/server/server.go#L216) |
| POST | `/api/plans/{id}/files` | Multipart `file` plus `role=spaces\|reference`; adds/replaces attachment and queues retrace; `201 {plan,duplicate:false}` or duplicate `200`. | [attach](../../../internal/server/files.go#L124) |
| DELETE | `/api/plans/{id}/files/{sha}` | Removes attachment by full SHA-256 and queues retrace; `200 {plan,duplicate:false}`; primary architecture file cannot be removed. | [detach](../../../internal/server/files.go#L155), [store rule](../../../internal/catalog/package.go#L112) |
| POST | `/api/plans/{id}/approve` | Body may contain `note`; only `draft → approved`; `200 {plan,was}`. | [review handler](../../../internal/server/server.go#L313), [action table](../../../internal/catalog/catalog.go#L399) |
| POST | `/api/plans/{id}/reject` | `draft` or `approved → queued`; keeps reviewer note for next attempt; same reply envelope. | [Review](../../../internal/catalog/catalog.go#L414) |
| POST | `/api/plans/{id}/retry` | `failed → queued`; clears error; same reply envelope. | [review actions](../../../internal/catalog/catalog.go#L399) |
| POST | `/api/plans/{id}/retrace` | `draft` or `failed → queued`; clears error and note; same reply envelope. | [Retrace](../../../internal/catalog/catalog.go#L390) |
| GET | `/api/profiles/{name}` | Conditional route; `200 {name,builtin,exists,active,profile,roles}`. Missing valid client profile is `exists:false` with empty rules, not `404`. | [getProfile](../../../internal/server/profiles.go#L48) |
| PUT | `/api/profiles/{name}` | Conditional route; strict JSON or YAML profile body, max 1 MiB; whole-file replacement; `200` profile reply. Built-ins cannot be overwritten (`403`). | [putProfile](../../../internal/server/profiles.go#L67) |

`{action}` is one generic route, with the four accepted values checked by the
catalogue. Unknown action or plan maps to `404`; invalid transition to `409`.
There is no implemented `GET /api/plans`, `GET /api/plans/{id}`, plan-delete,
booking, login, SCIM, health or versioned `/v1` application route.

| Other mounted HTTP surface | Routing/detail | Evidence |
| --- | --- | --- |
| `/generated/*` | `Handle` mount, not constrained to a verb by Chi; `StripPrefix` plus `http.FileServer` rooted at `Store.Dir`, independently of site root. Browser consumers use reads. | [mount](../../../internal/server/server.go#L103) |
| `/*` | `Handle` catch-all for microsite HTML/CSS/JS/assets rooted at `Server.Site`; same file wrapper. | [site mount](../../../internal/server/server.go#L105) |
| File visibility | Path segments beginning `.` get `404`; no status, user or office filter. Ordinary source/artifact/profile files in the output directory remain directly addressable. | [files](../../../internal/server/server.go#L129) |
| `/api` misses | API-specific JSON `404 {error:"no such API"}`; application error envelope generally `{error:string}`. | [NotFound](../../../internal/server/server.go#L99), [writeError](../../../internal/server/server.go#L345) |
| Separate static development server | Python `SimpleHTTPRequestHandler`, `127.0.0.1:8000`, `Cache-Control:no-store`; no upload/review API. | [serve.py](../../../web/floorplan-iso/tools/serve.py#L14) |
| Separate test provider | Chi `POST /v1/messages` in `internal/fakeclaude`; an `httptest` Anthropic stand-in, not a SpaceWeave product endpoint. | [fakeclaude.New](../../../internal/fakeclaude/fakeclaude.go#L67) |

## Existing wire behaviour and access assumptions

| Concern | Actual behaviour | Evidence |
| --- | --- | --- |
| Host guard | All Go-server requests require `localhost`, `*.localhost`, loopback IP or configured extra host; otherwise `421`. `serve` can add its explicit bound address. | [allowHost](../../../internal/server/server.go#L153), [serve wiring](../../../internal/cli/cli.go#L430) |
| Mutation guard | API methods other than GET/HEAD require `X-Spaceweave: 1`; supplied `Origin` must have the same `Host` as request. Missing Origin is accepted; comparison checks host, not a full scheme-origin tuple. | [sameOrigin](../../../internal/server/server.go#L172) |
| Authentication | These are local-browser/DNS-rebinding protections. No user login, session store, bearer verification, role/capability or office-scope authorisation is implemented in the handler. | [complete middleware/routes](../../../internal/server/server.go#L85) |
| CORS/cache | No CORS approval mechanism; requests passing the host guard receive `Cache-Control: no-store`. The custom header is not an identity credential. | [package rationale](../../../internal/server/server.go#L5), [noStore](../../../internal/server/server.go#L144) |
| Upload eligibility | PNG, JPEG, WebP, DXF, DWG and SVG. Extension/content agreement checked; drawings defer deeper conversion validation to worker. | [Formats](../../../internal/convert/formats.go#L30), [check](../../../internal/server/server.go#L253) |
| Upload limits | Default 40 MiB request body; image limit 100 million pixels; multipart text fields bounded at 1 KiB. | [limits](../../../internal/server/server.go#L42), [receive](../../../internal/server/files.go#L41) |
| Attachment rules | Space plan and parent must both be DXF/DWG; one space plan replaces previous; references append. Change allowed when queued/draft/failed; active/approved input is refused except unchanged duplicate content. | [Attach](../../../internal/catalog/package.go#L25), [requeue](../../../internal/catalog/package.go#L145) |
| Reference limits | Reference role uses the same accepted-format list as upload. Comments mention PDF prints, but PDF is absent from `convert.Formats` and is currently rejected. | [reference comment](../../../internal/catalog/catalog.go#L121), [Formats](../../../internal/convert/formats.go#L77) |
| Review body | Optional JSON `{note:string}`, 64 KiB reader cap; note trimmed. The decoder does not call `DisallowUnknownFields` or check for a second JSON value. | [review](../../../internal/server/server.go#L313) |
| Catalogue errors | `404` missing plan/action; `400` bad role; `415` unsupported format; `422` incompatible package; `409` state/primary-file conflict; `503` lock timeout; otherwise `500`. | [storeError](../../../internal/server/files.go#L174) |
| Transport coupling | `server.Info`, private reply structs, `catalog.Entry`, `convert.Format` and profile structs are JSON shapes used directly; no separate generated DTO layer. | [Info](../../../internal/server/server.go#L50), [uploadReply](../../../internal/server/server.go#L209), [profileReply](../../../internal/server/profiles.go#L19) |

## Direct JavaScript consumers and file contracts

The existing portal is native JavaScript/D3 with no build step, not the proposed
React/TypeScript staff/admin applications ([package.json](../../../web/floorplan-iso/package.json#L1)).

| Consumer | Contract it actually consumes | Evidence |
| --- | --- | --- |
| `js/api.js` | Relative `api/...` fetches, `cache:no-store`; mutating calls set `X-Spaceweave:1`; no explicit credentials/token/session configuration. | [createApi](../../../web/floorplan-iso/js/api.js#L16) |
| API methods | `info`, `upload`, `attach`, `detach`, `review`, `profile`, `saveProfile`; handwritten body/URL handling; failures expose JSON `error` or fallback HTTP status. | [client methods](../../../web/floorplan-iso/js/api.js#L29) |
| Workbench discovery | Calls `/api/info` once. Success reveals upload UI and derives formats/size/tracer text; failure shows offline UI. A plain static server can still display plans. | [discovery](../../../web/floorplan-iso/js/workbench.js#L499) |
| `js/plans/index.js` | Reads `generated/catalog.json` directly. `404` means empty; JSON `{plans}` becomes picker entries; excludes IDs shadowing built-ins. | [loadCatalog](../../../web/floorplan-iso/js/plans/index.js#L24) |
| Plan loading | Reads `generated/{id}/plan.json` directly on demand; built-ins use dynamic JS imports. This JSON is a browser contract even without an `/api` path. | [load closure](../../../web/floorplan-iso/js/plans/index.js#L55), [built-ins](../../../web/floorplan-iso/js/plans/index.js#L4) |
| Picker state | Queued/converting/tracing/draft/approved listed; only draft/approved loadable. Approval changes badge/status, not an HTTP asset ACL. | [LISTED/READY](../../../web/floorplan-iso/js/plans/index.js#L13), [picker](../../../web/floorplan-iso/js/main.js#L283) |
| Polling | Catalogue reread every 2 seconds while busy, otherwise 10 seconds; hidden document postpones reads. Updated completed trace reloads by `updatedAt`. No push/status job endpoint. | [schedule/refresh](../../../web/floorplan-iso/js/main.js#L300) |
| Review evidence | Workspace fetches report JSON and links overlay, report and original upload; source/converted drawing and package links are ordinary generated-directory URLs. | [workspace](../../../web/floorplan-iso/js/workspace.js#L97), [package links](../../../web/floorplan-iso/js/pipeline.js#L117) |
| Layer correction | Fetches report and profile roles; reads named profile, saves changed profile, then separately posts `retrace`. Profile save can succeed before retrace fails. | [layer read](../../../web/floorplan-iso/js/workbench.js#L254), [apply](../../../web/floorplan-iso/js/workbench.js#L335) |
| UI attachment support | API wrapper supports spaces/reference; current add-space flow uploads role `spaces`; existing attachments get direct links and detach actions. | [attach client](../../../web/floorplan-iso/js/api.js#L36), [add flow](../../../web/floorplan-iso/js/workbench.js#L369) |

| Stored artifact | Existing producer/schema and visibility implication |
| --- | --- |
| `catalog.json` | Versioned catalogue envelope with `plans:[]`; entries include status, original/upload/source, files, hash, progress, feedback, costs, timestamps. Directly served. [types](../../../internal/catalog/catalog.go#L66) |
| Per-plan source/package files | Images retain `source.<ext>`; drawings retain `original.<ext>` then converted assets; attachments use role/hash filenames. [Ingest](../../../internal/catalog/catalog.go#L482), [Attach](../../../internal/catalog/package.go#L71) |
| `trace.json`, `plan.json`, `overlay.png`, `report.json` | Worker rewrites fixed filenames for a new attempt; report holds validation/provenance, layers and errors. Existing storage is not immutable revision history. [filenames](../../../internal/worker/worker.go#L33), [publish](../../../internal/worker/worker.go#L313) |
| `transcript.jsonl` | Optional Claude trace transcript in the plan directory; may be read through static mount when present, although not a dedicated API route. [creation](../../../internal/worker/worker.go#L261) |
| `profiles/{name}.yaml`, `.active` | Client profile files beside plans; selected profile persists through restart. `.active` is hidden by dotfile guard; ordinary YAML files are not. [Library](../../../internal/autotrace/profile/library.go#L14) |

## CLI and non-HTTP ingress

The CLI calls Go objects and the local store directly; it is not an HTTP client.
The root registers five command families ([New](../../../internal/cli/cli.go#L92)).

| Existing command/entry | Actual work | Source |
| --- | --- | --- |
| `spaceweave watch` | Inbox watcher plus conversion/tracing workers until cancellation; no HTTP listener. | [watchCmd](../../../internal/cli/cli.go#L387) |
| `spaceweave serve` | Same watcher/workers plus microsite, generated files and API. Flags `--addr`, `--site`, repeatable `--allow-host`. | [serveCmd](../../../internal/cli/cli.go#L404) |
| `spaceweave trace <image or drawing>` | Ingest once, reject duplicate retracing, immediately transition/process input; failed trace produces failure exit. | [traceCmd](../../../internal/cli/cli.go#L481) |
| `spaceweave convert <drawing>` | Local conversion to PNG/SVG without catalogue/job; `-o/--output`, `--svg`, `--max-px`. | [convertCmd](../../../internal/cli/cli.go#L587) |
| `spaceweave plans list [--json]` | Reads catalogue; table or entries array to stdout. JSON output is another existing consumer-visible shape. | [plansCmd](../../../internal/cli/cli.go#L664) |
| `spaceweave plans approve <id>` | Calls store Review with approve. | [approve](../../../internal/cli/cli.go#L704) |
| `spaceweave plans reject <id> [--note ...]` | Calls store Review with reject/feedback. | [reject](../../../internal/cli/cli.go#L713) |
| `spaceweave plans retry <id>` | Calls store Review with retry. There is no explicit CLI `retrace`, attachment or profile command. | [retry/registration](../../../internal/cli/cli.go#L726) |
| Shared settings | Persistent config/inbox/output/model/max-usd/workers/profile/verbose flags; flag > environment > strict YAML > defaults. Default listener `127.0.0.1:8000`. | [flags](../../../internal/cli/cli.go#L111), [config](../../../internal/config/config.go#L102) |
| Inbox filesystem ingress | Stabilises size/mtime before callback; fsnotify plus 30-second rescan; matching space filenames attach to architecture file, with hold/fallback rules. | [Watcher](../../../internal/inbox/inbox.go#L32), [naming](../../../internal/inbox/inbox.go#L4) |

Automatic tracing is default and constructs no Anthropic client. Claude mode is
explicit configuration and obtains provider credentials through the SDK on the
Go side; the browser never supplies a Claude API key ([newWorker](../../../internal/cli/cli.go#L206)).

## Local queue versus proposed durable work

| Property | Existing implementation | Proposed production baseline |
| --- | --- | --- |
| Work persistence | Queue state survives process restart in local `catalog.json`; not merely in memory. | PostgreSQL durable records, not pod scratch. |
| Mutation/claim | Process mutex plus exclusive lock file, bounded wait/stale-lock expiry; atomically renamed catalogue; oldest queued `updatedAt` claimed. | Transactional claim/lease/fencing plus owner-restricted storage. |
| Wake/retry | Coalesced in-memory wake signal; workers also poll every 2 seconds; CLI retries can be detected across processes sharing output. | Persisted due work/outbox and retry classification/backoff. |
| Restart | Worker startup requeues **all** converting/tracing entries; cancellation requeues its current plan. | Lease/attempt-aware recovery prevents stale workers publishing. |
| Publication | Writes fixed artifact files before changing catalogue to draft; separate file renames, no multi-file/database transaction. | Immutable revisions/manifests plus verified recovery copies before acknowledgement. |
| Failure visibility | Failed status/error/report; manual retry/retrace. No generic leased job table, transactional outbox or per-consumer receipts. | Explicit visible failed jobs, outbox receipts and safe replay. |

Existing evidence: [store locking](../../../internal/catalog/catalog.go#L261),
[Claim](../../../internal/catalog/catalog.go#L339), [Recover](../../../internal/catalog/catalog.go#L464),
[worker loop](../../../internal/worker/worker.go#L125), [publish](../../../internal/worker/worker.go#L337).
Proposed baseline: [common requirements](../architecture-options/README.md).
Inference: startup-wide recovery is a local-daemon design, not safe evidence of
independent cross-pod ownership; there is no lease identity to distinguish another active daemon.

## Existing project-private Go seams

`internal` packages and exported Go identifiers are importable within the project
subject to Go's internal rules; neither makes an interface an Internet API.

| Existing seam | Kind and responsibility | Evidence |
| --- | --- | --- |
| `plan.Validator` | Interface `Validate(context.Context, []byte) ([]string,error)`; renderer invariant check. `NodeValidator` writes temporary JSON and invokes JS validator. | [validate.go](../../../internal/plan/validate.go#L18) |
| `worker.TraceFunc` | Function type, not interface; common call signature for deterministic `autotrace.Run` or optional `trace.Run`. | [TraceFunc](../../../internal/worker/worker.go#L47), [automatic implementation](../../../internal/autotrace/autotrace.go#L40) |
| `trace.Options`, `trace.Job`, `trace.Result` | Concrete processing DTOs, including SDK client, local image/path/profile inputs, feedback and artifacts; not transport-independent domain ports. | [Options/Job](../../../internal/trace/agent.go#L57) |
| `Server.Store`, `Worker.Store` | Concrete `*catalog.Store`, not repository interfaces. HTTP handlers and workers directly share catalogue types. | [Server](../../../internal/server/server.go#L68), [Worker](../../../internal/worker/worker.go#L50) |
| `profile.Library` | Concrete filesystem profile store used separately by server/worker over same directory; active profile read for each trace. | [CLI profiles](../../../internal/cli/cli.go#L247), [worker profile read](../../../internal/worker/worker.go#L253) |
| `inbox.Watcher.Ingest/Attach` | Function callbacks isolate watcher from catalogue implementation; role strings deliberately match without catalogue import. | [Watcher](../../../internal/inbox/inbox.go#L44) |
| `cli.App` | Dependency injection struct for output, environment, Anthropic options and optional validator; not a published client. | [App](../../../internal/cli/cli.go#L47) |
| `cad.Entity`, `vector.Item` | Drawing representation interfaces: `Base()` and bounds/transform operations. Pure processing polymorphism. | [Entity](../../../internal/cad/cad.go#L83), [Item](../../../internal/vector/vector.go#L119) |
| `dxf.source` | Unexported parser interface for ASCII/binary pair sources; implementation detail. | [source](../../../internal/dxf/reader.go#L58) |

## Proposed placement across the five alternatives

Every path in the next tables is a **proposal**. Wire schemas, generated server
contracts and handwritten ingress are separate from business-facing Go contracts.
In particular, `internal/<module>/api` is an in-process, project-private seam even
when its folder says `api` and its Go names are exported; it is not an Internet API.

| Layout | External wire source and generated types/clients | Ingress/controller placement | Private Go interfaces and composition |
| --- | --- | --- | --- |
| 1 — Layered | `api/src/{staff,admin,scim}/`, bundled `api/*.openapi.yaml`; Go `internal/transport/http/gen/{staff,admin,scim}`; optional `pkg/client/{staff,admin}`; TS `web/shared/api/generated`. | Chi `internal/transport/http`; staff/admin/plans/SCIM handlers and static portal adapter; `internal/transport/{cli,inbox,trigger,jobs}` are other drivers. | Services in `internal/application/*`; storage/provider interfaces in `internal/persistence/{store,blob,calendar,...}` beside implementations; shared business values in `internal/model`; `internal/app` wires them. |
| 2 — Hexagonal | `api/src/{staff,admin,scim}.yaml`, bundles/configs under `api`; Go `internal/adapter/driving/http/gen/*`; optional `pkg/client/{staff,admin}`; TS `web/shared/src/api/generated`. | `internal/adapter/driving/http/{plans,booking,session,scim,...}`; CLI/jobs/scheduler/inbox/web siblings. Handlers hold driving ports. | Central `internal/core/port/driving` and `driven`; model/rules/services in core; consumer-owned sibling interfaces where needed; `internal/app` assigns implementations. |
| 3 — Clean | `api/src/{staff,admin,visitor}.yaml`; Go `internal/adapter/http/gen/{staff,admin,visitor}`; TS `web/packages/api/*.gen.ts`; SCIM has protocol-specific shapes, not staff DTO aliases. | `internal/adapter/http/controller/{staff,admin,visitor}`, presenter/session/protocol adapters; Chi assembly in `internal/infrastructure/router`; lifecycle in `httpserver`. | Each `internal/usecase/<operation>/port.go` owns consumed interfaces; only shared `unitofwork/event/job` contract exceptions; `internal/entity` stays pure; `internal/app` wires use cases/controllers. |
| 4 — DDD | `api/src/app/<context>`, `api/src/provisioning`; bundle `api/generated/app.openapi.yaml`; Go `internal/shared/generated/{app,provisioning}`; TS `web/shared/generated`. | Each `internal/<context>/adapters/http`; common `shared/httpkit`; `internal/app/routes.go` dispatches generated contracts. | Eleven context `api` seams publish commands/queries/facts; repository/provider contracts in context domain/application; implementation in adapters; factories composed by `app/contexts.go`. |
| 5 — Modular monolith | Single `api/openapi.yaml` with capability fragments and `generation.yaml`; Go `internal/transport/gen/rest`; optional `pkg/client/rest`; TS `web/generated/{transport,client}.ts`. | Large modules `internal/<module>/adapters/http`; small modules `<module>/http`; read-composing `internal/gateway/{portal,operations}`; `internal/app/router.go` mounts module handlers. | Ten representative `<module>/api` seams; large modules have private `core/port`, small ones may stay flat; `module.go` exposes narrow API/route/job bundles; `app/modules.go` composes. |

Placement evidence: [layered tree](../architecture-options/1-layered.md),
[layered generation](../architecture-options/1-layered.md),
[hexagonal ports/adapters](../architecture-options/2-hexagonal.md),
[hexagonal generation](../architecture-options/2-hexagonal.md),
[clean rings](../architecture-options/3-clean.md),
[clean transport](../architecture-options/3-clean.md),
[DDD contracts](../architecture-options/4-ddd.md),
[DDD generation](../architecture-options/4-ddd.md),
[modular transport](../architecture-options/5-modular-monolith.md).

| Floor-plan responsibility | Layered | Hexagonal | Clean | DDD | Modular monolith |
| --- | --- | --- | --- | --- | --- |
| Upload/review/profile HTTP | `transport/http/plans` | `adapter/driving/http/plans` | Admin drawing controller; explicit drawing use cases | `plans/adapters/http/plans.go` | `plans/adapters/http/revisions.go` |
| Workflow contracts | Application Plans arguments/model; persistence facets | `core/port/driving/plans.go`, driven plan/blob/tracing/validator contracts | `usecase/{drawingupload,drawingprocess,drawingreview}/port.go` | `plans/api/plans.go`; domain revision/blob/repository contracts | `plans/api/revisions.go`; `plans/core/port/{blobs,store}.go` |
| Processing implementation | Pure `support` engines; I/O in persistence adapters | Pure `core/geometry`; I/O/tracing in driven adapters | `adapter/drawingengine` plus conversion/validator/Claude gateways | Retained existing packages behind `plans/adapters/tracing/pipeline.go` | `plans/processing/*`; local catalogue migration in `plans/adapters/localimport` |
| Durable work owner | Application Plans plus jobs/persistence work mechanism | Core Plans plus work ports and driving jobs runner | Drawing use cases, job port and runner/queue adapters | Plans context plus shared jobs/outbox | Plans registers jobs; platform supplies queue/outbox |

Evidence: [layered Plans](../architecture-options/1-layered.md),
[hexagonal pipeline](../architecture-options/2-hexagonal.md),
[clean drawing operations](../architecture-options/3-clean.md),
[DDD Plans](../architecture-options/4-ddd.md),
[modular Plans tree](../architecture-options/5-modular-monolith.md).

## Option 5's ten representative owners

This candidate is not the eleven-context DDD layout or the earlier blueprint.
Its explicit list is [Identity, Workplace, Booking, Events, Planning, Visitors,
Plans, Notifications, Attendance and Policy](../architecture-options/5-modular-monolith.md).
The Events–Booking boundary is agreed; the other placements remain proposed.

| Proposed module | Owned responsibility and private published seam |
| --- | --- |
| Identity | Combines workforce/directory projection, verified source correlation, sessions, capabilities and delegation; `identity/api/{access,people,delegation,events}.go`. |
| Workplace | Versioned offices/buildings/floors/spaces/calendars and baseline neighbourhood definitions; `workplace/api/{definitions,events}.go`; no claim writes. |
| Booking | Dated inventory/claims, desk use/waitlists, rooms, Exchange reconciliation state, venue claims and outage protection; `booking/api/{claims,allocation,desks,rooms,venues,operations}.go`. No separate Exchange module in this candidate. |
| Events | Event catalogue/audience/registration, ticket capacity, named tickets and admission; `events/api/{events,tickets}.go`. Coordinates venue changes through Booking's participant; owns its event/ticket SQL and migrations. |
| Planning | Attendance dates/travel choices, cycles, groups, solver/proposals/run status; `planning/api/planning.go`; computes proposals while Booking validates/commits capacity. |
| Visitors | Hosted/restricted visits, verification/reception/retention; `visitors/api/{visits,ticket_visits}.go`; proposed named `VisitTicketIssuance` transaction with Events. |
| Plans | Drawing-to-approved-revision workflow and assets, including blob port/Azure/GCS adapters and retained processing algorithms; `plans/api/revisions.go`. |
| Notifications | In-app/email delivery, message catalogue/preferences, optional private booking-raised Teams delivery; `notifications/api/notifications.go`. |
| Attendance | Provenance-labelled attendance/occupancy projections, reports and retention; `attendance/api/reports.go`; no booking authority. |
| Policy | Immutable versions, validation/publication and permitted office overrides; `policy/api/policies.go`. |

Ownership evidence: [published contracts](../architecture-options/5-modular-monolith.md),
[Events and venue ownership](../architecture-options/5-modular-monolith.md),
[Exchange ownership](../architecture-options/5-modular-monolith.md),
[Plans ownership](../architecture-options/5-modular-monolith.md).

## Factual gaps to preserve in later API work

| Gap | Implication for subsequent design |
| --- | --- |
| No implemented booking/session/provisioning services | Do not present suggested endpoints, auth scopes, idempotency or status codes as existing behaviour. Proposed names do not define accepted wire contracts. |
| No OpenAPI/generated application clients found | Wire compatibility currently depends on handwritten Go/JS/file shapes. The five alternatives choose different generation locations, not existing tooling. |
| Generated-directory reads bypass application decisions | A production asset contract must address direct catalogue/report/original/profile access as well as named API routes; protecting only POST routes is incomplete. |
| Lock visibility comment differs from code | `files` says it hides the catalogue lock, but the actual filename is non-dot `catalog.lock`; the dotfile guard does not hide it while present. [comment/guard](../../../internal/server/server.go#L129), [lock path](../../../internal/catalog/catalog.go#L286) |
| Mutable attempt files and global profile selection | Immutable revision IDs, expected versions and scoped profile semantics are proposed work, not current guarantees. Saving a profile and retracing are separate writes. |
| CLI/HTTP asymmetry | HTTP has retrace/attachment/profile operations absent from explicit CLI commands; browser lists through file JSON while CLI reads store directly. Do not invent parity. |
| Local queue versus durable target | Persisted local status has useful recovery behaviour but no lease/attempt fencing, outbox or distributed ownership protocol. |
| Mapping correction during this review | Earlier candidate mappings described `internal/server/files.go` as asset serving; the mappings now identify its multipart attachment/detach role. Static generated-directory serving originates in `server.go`. [mapping](../architecture-options/3-clean.md), [actual](../../../internal/server/files.go#L37) |
| Standard protocols remain distinct | Proposed SCIM/provider callbacks are separately authenticated wire surfaces; published Go module/context APIs are ordinary in-process calls, with no HTTP address implied. |
| Package privacy enforcement is proposed | Root `internal` protects the project boundary, not arbitrary sibling-module implementation imports; context/module API-only rules also need the proposed import/ownership checks. [DDD rule](../architecture-options/4-ddd.md), [module enforcement](../architecture-options/5-modular-monolith.md) |
