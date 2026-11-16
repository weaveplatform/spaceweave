// What the "Add a plan" list says about each generated plan: where it is in
// upload → queue → trace → review, and what can be done about it. Pure, so
// node can test it.

export const STEPS = ['Uploaded', 'Queued', 'Tracing', 'Review'];

// The step each status is on. 'uploading' is the page's own state while the
// file is on its way; approved is past the last step.
const AT = { uploading: 0, queued: 1, tracing: 2, failed: 2, draft: 3, approved: 4 };

const money = (n) => `$${(n ?? 0).toFixed(2)}`;
const plural = (n, word) => `${n} ${word}${n === 1 ? '' : 's'}`;

export function describe(entry, maxUsd) {
  const at = AT[entry.status] ?? 0;
  const steps = STEPS.map((label, i) => {
    if (i < at) return { label, state: 'done' };
    if (i > at) return { label, state: 'todo' };
    return { label, state: entry.status === 'failed' ? 'failed' : 'current' };
  });
  const cap = Number.isFinite(maxUsd) ? ` of ${money(maxUsd)}` : '';
  let detail;
  let action = null;
  switch (entry.status) {
    case 'uploading':
      detail = 'Uploading…';
      break;
    case 'queued':
      detail = entry.note ? `Waiting for the tracer, with your note: “${entry.note}”` : 'Waiting for the tracer';
      break;
    case 'tracing': {
      const p = entry.progress;
      detail = p
        ? `Turn ${p.turn} · ${plural(p.submits, 'submission')} · ${money(p.costUsd)}${cap}`
        : 'Starting…';
      break;
    }
    case 'draft':
      detail = ['Ready for review',
        Number.isFinite(entry.confidence) ? `confidence ${Math.round(entry.confidence * 100)}%` : null,
        money(entry.costUsd)].filter(Boolean).join(' · ');
      action = 'open';
      break;
    case 'failed':
      detail = `Failed: ${entry.error || 'no reason given'} · ${money(entry.costUsd)}`;
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

// True while the tracer has work, so the page can poll faster.
export const busy = (plans) => plans.some((p) => p.generated && (p.status === 'queued' || p.status === 'tracing'));
