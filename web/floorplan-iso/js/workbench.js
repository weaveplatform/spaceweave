// The page's side of `spaceweave serve`: dropping, choosing or pasting a
// plan uploads it, the pipeline list follows each plan through conversion and
// the tracer, and a draft can be approved or sent back with a note. Under a
// plain static server the API isn't there, and the page says how to start it
// instead.
import { createApi } from './api.js';
import { describe, inPipeline } from './pipeline.js';
import { acceptAttr, checkFile, names, pastedName, refusesDrag } from './formats.js';

const d3 = globalThis.d3;

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
  let drawings = new Set(); // ids of the formats converted before tracing
  const uploading = new Map(); // files on their way, shown before the server has an id
  const mine = new Set(); // plans sent from this page, announced when they're done
  let seq = 0;

  // One line per message, so every refused file in a batch is named.
  function sayAll(lines, isError = false) {
    status.replaceChildren(...lines.map((l) => {
      const line = document.createElement('span');
      line.textContent = l;
      return line;
    }));
    status.classList.toggle('error', isError);
  }
  function say(message, isError = false) {
    sayAll([message], isError);
  }

  async function act(fn) {
    try {
      await fn();
    } catch (err) {
      say(err.message, true);
    }
    await refresh();
  }

  // Checks every file before uploading any, so a batch with one bad file
  // says which and why while the good ones go ahead.
  async function vet(files) {
    const ok = [];
    const refused = [];
    const limit = info.maxUploadBytes;
    for (const file of files) {
      const name = pastedName(file, info.formats);
      const named = name === file.name ? file : new File([file], name, { type: file.type });
      if (file.size > limit) {
        refused.push(`${name}: over the ${Math.round(limit / 2 ** 20)} MB upload limit.`);
        continue;
      }
      const verdict = await checkFile(named, info.formats);
      if (verdict.ok) ok.push({ file: named, format: verdict.format });
      else refused.push(verdict.reason.startsWith(name) ? verdict.reason : `${name}: ${verdict.reason}`);
    }
    return { ok, refused };
  }

  async function uploadFiles(files) {
    const { ok, refused } = await vet(files);
    if (refused.length) sayAll(refused, true);
    for (const { file, format } of ok) {
      const { name } = file;
      const key = `upload-${++seq}`;
      uploading.set(key, { id: key, name, status: 'uploading', format: format.id });
      update();
      try {
        const { plan, duplicate } = await api.upload(file, name);
        if (!duplicate) mine.add(plan.id);
        // Refusals stay on screen; the good files' progress shows in the list.
        if (refused.length === 0) {
          if (duplicate) say(`That file was uploaded before, as “${plan.title}” (${plan.status}).`);
          else if (format.drawing) say(`“${plan.title}” is queued. It's converted from ${format.label} first; tracing usually takes a few minutes.`);
          else say(`“${plan.title}” is queued. Tracing usually takes a few minutes.`);
        }
      } catch (err) {
        refused.push(`${name}: ${err.message}`);
        sayAll(refused, true);
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
  // Browsers show a dragged file's MIME type, but not its name, until the
  // drop; a known type that isn't ours can be refused straight away.
  const refusedDrag = (e) => refusesDrag([...(e.dataTransfer?.items ?? [])]
    .filter((i) => i.kind === 'file').map((i) => i.type), info.formats);
  let depth = 0; // dragenter/leave fire for every child crossed
  document.addEventListener('dragenter', (e) => {
    if (!carriesFiles(e)) return;
    depth++;
    document.body.classList.add('dragging');
    document.body.classList.toggle('drag-refused', refusedDrag(e));
  });
  document.addEventListener('dragleave', (e) => {
    if (!carriesFiles(e)) return;
    depth = Math.max(0, depth - 1);
    if (depth === 0) document.body.classList.remove('dragging', 'drag-refused');
  });
  document.addEventListener('dragover', (e) => {
    if (!carriesFiles(e)) return;
    e.preventDefault(); // otherwise the browser opens the file instead
    e.dataTransfer.dropEffect = document.body.classList.contains('drag-refused') ? 'none' : 'copy';
  });
  document.addEventListener('drop', (e) => {
    if (!carriesFiles(e)) return;
    e.preventDefault();
    depth = 0;
    document.body.classList.remove('dragging', 'drag-refused');
    uploadFiles([...e.dataTransfer.files]);
  });
  document.addEventListener('paste', (e) => {
    if (!info || e.target.closest?.('input, textarea')) return;
    const files = [...(e.clipboardData?.files ?? [])];
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
        const v = describe(d, info?.maxUsd, drawings);
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
    drawings = new Set(i.formats.filter((f) => f.drawing).map((f) => f.id));
    fileInput.accept = acceptAttr(i.formats);
    dropzone.hidden = false;
    terms.textContent = `${names(i.formats)}, up to ${Math.round(i.maxUploadBytes / 2 ** 20)} MB. `
      + 'Drawings are turned into an image here first. '
      + (i.tracer === 'claude'
        ? `The image is sent to the Claude API (${i.model}); each plan costs up to $${i.maxUsd}.`
        : 'Plans are traced on this server; nothing is sent anywhere.');
    update();
  }).catch(() => {
    offline.hidden = false;
  });

  return { update };
}
