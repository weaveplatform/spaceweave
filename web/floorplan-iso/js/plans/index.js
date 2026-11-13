// Plan catalogue for the picker: the hand-traced plans below, plus whatever
// the spaceweave tracer has written to generated/catalog.json. Modules and
// plan files load on demand so the page only fetches the plan it shows.
export const PLANS = [
  { id: 'office', name: 'Office', blurb: 'RoomSketcher sample · 19 × 11 m', load: () => import('./roomsketcher-office.js') },
  { id: 'clinic', name: 'Clinic', blurb: 'Physical therapy practice (SmartDraw sample)', load: () => import('./pt-clinic.js') },
  { id: 'suites', name: 'Suites', blurb: 'New Urban Suites, third floor', load: () => import('./urban-suites.js') },
  { id: 'corporate', name: 'Corporate', blurb: 'Open-plan floor with private offices', load: () => import('./corporate-floor.js') },
];

export const DEFAULT_PLAN = 'office';

// Statuses the picker lists. queued and tracing show as disabled placeholders
// so a dropped image is visibly on its way; rejected plans are waiting for
// another pass and failed ones have nothing to show.
const LISTED = new Set(['queued', 'tracing', 'draft', 'approved']);
export const READY = new Set(['draft', 'approved']);

// Must match the output directory the Go tracer writes (internal/catalog).
export const GENERATED = 'generated/';

// Reads generated/catalog.json. A missing file (no tracer has run) is an
// empty catalogue, not an error.
export async function loadCatalog(fetchImpl = globalThis.fetch) {
  const res = await fetchImpl(`${GENERATED}catalog.json`, { cache: 'no-store' });
  if (res.status === 404) return [];
  if (!res.ok) throw new Error(`catalog.json: HTTP ${res.status}`);
  const { plans = [] } = await res.json();
  const builtIn = new Set(PLANS.map((p) => p.id));
  return plans
    .filter((p) => LISTED.has(p.status) && !builtIn.has(p.id))
    .map((p) => {
      const dir = `${GENERATED}${p.id}/`;
      return {
        id: p.id,
        name: p.title || p.id,
        blurb: p.summary || 'Traced automatically',
        status: p.status,
        confidence: p.confidence,
        generated: true,
        dir,
        load: async () => {
          const r = await fetchImpl(`${dir}plan.json`, { cache: 'no-store' });
          if (!r.ok) throw new Error(`${dir}plan.json: HTTP ${r.status}`);
          return r.json();
        },
      };
    });
}
