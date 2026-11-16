// The page's side of `spaceweave serve`: dropping, choosing or pasting an
// image uploads it, the pipeline list follows each plan through the tracer,
// and a draft can be approved or sent back with a note. Under a plain static
// server the API isn't there, and the page says how to start it instead.
import { createApi } from './api.js';
import { describe, inPipeline } from './pipeline.js';

const d3 = globalThis.d3;

// Must match inbox.Eligible in the Go worker.
const ACCEPT = /\.(png|jpe?g|webp)$/i;
const EXT = { 'image/png': '.png', 'image/jpeg': '.jpg', 'image/webp': '.webp' };

// Pasted screenshots arrive as "image.png"; the tracer names the plan from
// the drawing anyway, but the file name is its first guess.
function uploadName(file) {
  if (ACCEPT.test(file.name) && file.name !== 'image.png') return file.name;
  return EXT[file.type] ? `Pasted plan${EXT[file.type]}` : file.name;
}

/**
 * @param {object} hooks
 * @param {() => object[]} hooks.plans   every plan the page knows
 * @param {() => string} hooks.current   the id of the plan on show
 * @param {() => Promise<void>} hooks.refresh  re-read the catalogue
 * @param {(id: string) => void} hooks.open    show a plan
 */
export function initWorkbench({ plans, current, refresh, open }) {
  const api = createApi();
  const $ = (id) => document.getElementById(id);
  const dropzone = $('dropzone');
  const fileInput = $('upload-file');
  const terms = $('upload-terms');
  const offline = $('upload-offline');
  const status = $('upload-status');
  const list = $('pipeline');
  const review = $('review');
  const approve = $('approve');
  const sendBack = $('send-back');
  const rejectForm = $('reject-form');
  const note = $('reject-note');

  let info = null; // the server's answer to /api/info; null without the API
  const uploading = new Map(); // files on their way, shown before the server has an id
  const mine = new Set(); // plans sent from this page, announced when they're done
  let seq = 0;

  function say(message, isError = false) {
    status.textContent = message;
    status.classList.toggle('error', isError);
  }

  async function act(fn) {
    try {
      await fn();
    } catch (err) {
      say(err.message, true);
    }
    await refresh();
  }

  async function uploadFiles(files) {
    for (const file of files) {
      const name = uploadName(file);
      if (!ACCEPT.test(name)) {
        say(`${file.name} isn't a PNG, JPEG or WebP image.`, true);
        continue;
      }
      if (file.size > info.maxUploadBytes) {
        say(`${file.name} is over the ${Math.round(info.maxUploadBytes / 2 ** 20)} MB upload limit.`, true);
        continue;
      }
      const key = `upload-${++seq}`;
      uploading.set(key, { id: key, name, status: 'uploading' });
      update();
      try {
        const { plan, duplicate } = await api.upload(file, name);
        if (duplicate) {
          say(`That image was uploaded before, as “${plan.title}” (${plan.status}).`);
        } else {
          mine.add(plan.id);
          say(`“${plan.title}” is queued. Tracing usually takes a few minutes.`);
        }
      } catch (err) {
        say(`${name}: ${err.message}`, true);
      } finally {
        uploading.delete(key);
      }
      await refresh();
    }
  }

  // ---- getting files in: the file picker, a drop anywhere on the page, paste
  fileInput.addEventListener('change', () => {
    const files = [...fileInput.files];
    fileInput.value = ''; // so choosing the same file again still fires
    uploadFiles(files);
  });

  const carriesFiles = (e) => info && [...(e.dataTransfer?.types ?? [])].includes('Files');
  let depth = 0; // dragenter/leave fire for every child crossed
  document.addEventListener('dragenter', (e) => {
    if (!carriesFiles(e)) return;
    depth++;
    document.body.classList.add('dragging');
  });
  document.addEventListener('dragleave', (e) => {
    if (!carriesFiles(e)) return;
    depth = Math.max(0, depth - 1);
    if (depth === 0) document.body.classList.remove('dragging');
  });
  document.addEventListener('dragover', (e) => {
    if (!carriesFiles(e)) return;
    e.preventDefault(); // otherwise the browser opens the file instead
    e.dataTransfer.dropEffect = 'copy';
  });
  document.addEventListener('drop', (e) => {
    if (!carriesFiles(e)) return;
    e.preventDefault();
    depth = 0;
    document.body.classList.remove('dragging');
    uploadFiles([...e.dataTransfer.files]);
  });
  document.addEventListener('paste', (e) => {
    if (!info || e.target.closest?.('input, textarea')) return;
    const files = [...(e.clipboardData?.files ?? [])].filter((f) => EXT[f.type]);
    if (files.length === 0) return;
    e.preventDefault();
    uploadFiles(files);
  });

  // ---- review
  const shown = () => plans().find((p) => p.id === current());
  approve.addEventListener('click', () => act(async () => {
    const entry = shown();
    await api.review(entry.id, 'approve');
    say(`Approved “${entry.name}”.`);
  }));
  sendBack.addEventListener('click', () => {
    rejectForm.hidden = false;
    note.focus();
  });
  $('reject-cancel').addEventListener('click', () => {
    rejectForm.hidden = true;
  });
  rejectForm.addEventListener('submit', (e) => {
    e.preventDefault();
    act(async () => {
      const entry = shown();
      await api.review(entry.id, 'reject', note.value.trim());
      mine.add(entry.id);
      note.value = '';
      rejectForm.hidden = true;
      say(`Sent “${entry.name}” back for another pass.`);
    });
  });

  function renderReview() {
    const entry = shown();
    const reviewable = Boolean(info && entry?.generated && (entry.status === 'draft' || entry.status === 'approved'));
    review.hidden = !reviewable;
    if (!reviewable) rejectForm.hidden = true;
    approve.hidden = entry?.status !== 'draft';
  }

  // ---- the pipeline list
  function renderPipeline() {
    const rows = [...uploading.values(), ...inPipeline(plans())];
    list.hidden = rows.length === 0;
    d3.select(list).selectAll('li.job')
      .data(rows, (d) => d.id)
      .join((enter) => {
        const li = enter.append('li').attr('class', 'job');
        const head = li.append('div').attr('class', 'job-head');
        head.append('span').attr('class', 'job-name');
        head.append('span').attr('class', 'job-attempt');
        li.append('ol').attr('class', 'steps').attr('aria-label', 'Progress');
        li.append('p').attr('class', 'job-detail');
        const actions = li.append('div').attr('class', 'job-actions');
        actions.append('button').attr('type', 'button').attr('class', 'open').text('Open')
          .on('click', (e, d) => open(d.id));
        actions.append('button').attr('type', 'button').attr('class', 'retry').text('Retry')
          .on('click', (e, d) => act(async () => {
            await api.review(d.id, 'retry');
            mine.add(d.id);
            say(`Retrying “${d.name}”.`);
          }));
        return li;
      })
      .each(function show(d) {
        const v = describe(d, info?.maxUsd);
        const row = d3.select(this).attr('data-status', d.status);
        row.select('.job-name').text(d.name);
        row.select('.job-attempt').text(v.attempt);
        row.select('.steps').selectAll('li').data(v.steps).join('li')
          .attr('class', (s) => s.state)
          .attr('aria-current', (s) => (s.state === 'current' ? 'step' : null))
          .text((s) => s.label);
        row.select('.job-detail').text(v.detail);
        row.select('.job-actions').attr('hidden', v.action ? null : '');
        row.select('.open').attr('hidden', v.action === 'open' ? null : '');
        row.select('.retry').attr('hidden', v.action === 'retry' ? null : '');
      });
  }

  // Tell whoever sent a plan when it's done, without them watching the list.
  function announce() {
    for (const id of mine) {
      const entry = plans().find((p) => p.id === id);
      if (entry?.status === 'draft') {
        say(`“${entry.name}” is ready for review.`);
        mine.delete(id);
      } else if (entry?.status === 'failed') {
        say(`“${entry.name}” couldn't be traced: ${entry.error}`, true);
        mine.delete(id);
      }
    }
  }

  function update() {
    renderReview();
    renderPipeline();
    announce();
  }

  api.info().then((i) => {
    info = i;
    dropzone.hidden = false;
    terms.textContent = `PNG, JPEG or WebP, up to ${Math.round(i.maxUploadBytes / 2 ** 20)} MB. `
      + `The image is sent to the Claude API (${i.model}); each plan costs up to $${i.maxUsd}.`;
    update();
  }).catch(() => {
    offline.hidden = false;
  });

  return { update };
}
