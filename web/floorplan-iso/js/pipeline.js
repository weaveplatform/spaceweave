// What the "Add a plan" list says about each generated plan: where it is in
// upload → queue → (convert) → trace → review, and what can be done about
// it. Pure, so node can test it.

export const STEPS = ['Uploaded', 'Queued', 'Tracing', 'Review'];
// A drawing (DXF, SVG) is turned into an image before it's traced.
export const DRAWING_STEPS = ['Uploaded', 'Queued', 'Converting', 'Tracing', 'Review'];

// The step each status is on. 'uploading' is the page's own state while the
// file is on its way; approved is past the last step. A failure sits on the
// step that failed.
const AT = { uploading: 0, queued: 1, tracing: 2, failed: 2, draft: 3, approved: 4 };
const DRAWING_AT = { uploading: 0, queued: 1, converting: 2, tracing: 3, failed: 3, draft: 4, approved: 5 };

const money = (n) => `$${(n ?? 0).toFixed(2)}`;
// A plan traced automatically costs nothing, and saying "$0.00" on every
// row would suggest a bill that doesn't exist.
const spent = (n) => (n > 0 ? money(n) : null);
const plural = (n, word) => `${n} ${word}${n === 1 ? '' : 's'}`;

// Whether a drawing's conversion has run: its image exists once it has.
const converted = (entry) => Boolean(entry.source);

/**
 * @param {object} entry   a catalogue entry, or the page's own uploading row
 * @param {number} [maxUsd] the per-plan spending cap, when known
 * @param {Set<string>} [drawings] ids of the formats converted before tracing
 */
export function describe(entry, maxUsd, drawings = new Set()) {
  const drawing = drawings.has(entry.format);
  const labels = drawing ? DRAWING_STEPS : STEPS;
  let at = (drawing ? DRAWING_AT : AT)[entry.status] ?? 0;
  if (drawing && entry.status === 'failed' && !converted(entry)) at = 2;
  const steps = labels.map((label, i) => {
    if (i < at) return { label, state: 'done' };
    if (i > at) return { label, state: 'todo' };
    return { label, state: entry.status === 'failed' ? 'failed' : 'current' };
  });
  const cap = Number.isFinite(maxUsd) ? ` of ${money(maxUsd)}` : '';
  const kind = String(entry.format ?? '').toUpperCase();
  const scale = entry.scale?.pxPerMetre > 0 && !entry.scale.assumed ? 'scale from the drawing' : null;
  let detail;
  let action = null;
  switch (entry.status) {
    case 'uploading':
      detail = 'Uploading…';
      break;
    case 'queued':
      detail = entry.note ? `Waiting for the tracer, with your note: “${entry.note}”` : 'Waiting for the tracer';
      break;
    case 'converting':
      detail = `Turning the ${kind} drawing into an image…`;
      break;
    case 'tracing': {
      const p = entry.progress;
      detail = p
        ? [`Turn ${p.turn}`, plural(p.submits, 'submission'), `${money(p.costUsd)}${cap}`, scale].filter(Boolean).join(' · ')
        : 'Tracing…';
      break;
    }
    case 'draft':
      detail = ['Ready for review',
        Number.isFinite(entry.confidence) ? `confidence ${Math.round(entry.confidence * 100)}%` : null,
        spent(entry.costUsd), drawing ? `from ${kind}` : null, scale].filter(Boolean).join(' · ');
      action = 'open';
      break;
    case 'failed':
      detail = [`Failed: ${entry.error || 'no reason given'}`, spent(entry.costUsd)].filter(Boolean).join(' · ');
      action = 'retry';
      break;
    case 'approved':
      detail = 'Approved';
      action = 'open';
      break;
    default:
      detail = entry.status;
  }
  return { steps, detail, action, attempt: entry.attempts > 1 ? `attempt ${entry.attempts}` : '' };
}

// The plans the list shows: everything generated that still needs something
// to happen or someone to look at it, newest first.
export function inPipeline(plans) {
  return plans
    .filter((p) => p.generated && p.status !== 'approved')
    .sort((a, b) => String(b.updatedAt ?? '').localeCompare(String(a.updatedAt ?? '')));
}

// Statuses with work in hand, while the page polls faster.
const WORKING = new Set(['queued', 'converting', 'tracing']);

// True while the tracer has work, so the page can poll faster.
export const busy = (plans) => plans.some((p) => p.generated && WORKING.has(p.status));
