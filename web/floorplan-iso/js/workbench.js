// The page's side of `spaceweave serve`: dropping, choosing or pasting a
// plan uploads it, the pipeline list follows each plan through conversion and
// the tracer, and a draft can be approved or sent back with a note. A
// drawing's layer roles are staged together, then saved and retraced once.
// A drawing's floor package can take a space plan, which is
// placed on the drawing and traced with it. Under a plain static server the
// API isn't there, and the page says how to start it instead.
import { createApi } from './api.js';
import { layerRows, nameProblem, setLayerRole, sourceLabel, suggestName } from './layers.js';
import {
  canAddSpaces, canChangePackage, describe, inPipeline, packageRows, spaceFormats, spacePlanProblem,
} from './pipeline.js';
import { acceptAttr, checkFile, names, pastedName, refusesDrag } from './formats.js';

const d3 = globalThis.d3;

/**
 * @param {object} hooks
 * @param {() => object[]} hooks.plans   every plan the page knows
 * @param {() => string} hooks.current   the id of the plan on show
 * @param {() => Promise<void>} hooks.refresh  re-read the catalogue
 * @param {(id: string) => void} hooks.open    show a plan
 */
export function initWorkbench({ plans, current, refresh, open, onImportStarted, onLayers, onTraceQueued }) {
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
  const layersPanel = $('layers');
  const layersIntro = $('layers-intro');
  const layerBody = $('layer-rows');
  const packagePanel = $('package');
  const packageIntro = $('package-intro');
  const packageFiles = $('package-files');
  const spaceAdd = $('space-add');
  const spaceInput = $('space-file');

  let info = null; // the server's answer to /api/info; null without the API
  let drawings = new Set(); // ids of the formats converted before tracing
  const uploading = new Map(); // files on their way, shown before the server has an id
  const mine = new Set(); // plans sent from this page, announced when they're done
  let seq = 0;
  function track(id) { mine.add(id); onTraceQueued?.(id); }

  // One line per message, so every refused file in a batch is named.
  function sayAll(lines, isError = false) {
    status.replaceChildren(...lines.map((l) => {
      const line = document.createElement('span');
      line.textContent = l;
      return line;
    }));
    status.classList.toggle('error', isError);
    const inDialog = $('import-status');
    inDialog.replaceChildren(...[...status.children].map(node => node.cloneNode(true)));
    inDialog.classList.toggle('error', isError);
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
    if (ok.length) onImportStarted?.();
    for (const { file, format } of ok) {
      const { name } = file;
      const key = `upload-${++seq}`;
      uploading.set(key, { id: key, name, status: 'uploading', format: format.id });
      update();
      try {
        const { plan, duplicate } = await api.upload(file, name);
        if (!duplicate) track(plan.id);
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
    if (!info || !$('import-dialog').open || e.target.closest?.('input, textarea, select')) return;
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
  $('retrace').addEventListener('click', () => act(async () => {
    const entry = shown();
    await api.review(entry.id, 'retrace');
    track(entry.id);
    say(`Retracing “${entry.name}” using its source files and saved layer roles.`);
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
      track(entry.id);
      note.value = '';
      rejectForm.hidden = true;
      say(`Sent “${entry.name}” back for another pass.`);
    });
  });

  function renderReview() {
    const entry = shown();
    const reviewable = Boolean(info && entry?.generated && (entry.status === 'draft' || entry.status === 'approved'));
    review.hidden = !reviewable;
    const pending = Boolean(drafts.get(entry?.id)?.changes.size);
    approve.disabled = applying || pending;
    $('retrace').disabled = applying || pending;
    if (!reviewable || info?.tracer !== 'claude') rejectForm.hidden = true;
    sendBack.hidden = info?.tracer !== 'claude';
    $('review-guidance').textContent = pending ? drafts.get(entry.id).profileSaved
      ? 'Finish or clear the pending retrace before approving this floor.'
      : 'Apply or discard the staged layer changes before approving or retracing this floor.' : info?.tracer === 'claude'
      ? 'Request changes sends your notes to the tracer for its next pass.'
      : 'Automatic tracing uses source files and saved layer roles. Retracing unchanged inputs produces the same result; review notes are not applied.';
    approve.hidden = entry?.status !== 'draft';
  }

  // ---- layers: what each of a drawing's layers was read as, and a menu to
  // say otherwise. Drafts stay with their floor across navigation; applying
  // saves one profile and queues one trace after all chosen roles are ready.
  let layersFor = null; // a failed drawing picked from the pipeline list
  let layersShown = ''; // the plan and version whose report is on show
  let layersEntry = null; // the plan whose layers are on show
  const drafts = new Map();
  let applying = false;
  window.addEventListener('beforeunload', event => {
    if (![...drafts.values()].some(draft => draft.changes.size)) return;
    event.preventDefault(); event.returnValue = '';
  });
  let roles = []; // the roles a layer can have, from the profile API

  function layersTarget() {
    const picked = plans().find((p) => p.id === layersFor && (p.status === 'failed' || p.status === 'draft'));
    if (picked) return picked;
    const entry = shown();
    return entry?.generated && entry.status === 'draft' && drawings.has(entry.format) ? entry : null;
  }

  async function renderLayers() {
    const entry = info?.profiles ? layersTarget() : null;
    if (!entry) {
      layersPanel.hidden = true;
      layersShown = '';
      return;
    }
    // A retrace changes updatedAt and rewrites report.json.
    const key = `${entry.id}@${entry.updatedAt}`;
    if (key === layersShown) return;
    layersShown = key;
    let rows = [];
    try {
      if (roles.length === 0) roles = (await api.profile(info.profile || 'generic')).roles;
      const res = await fetch(`${entry.dir}report.json`, { cache: 'no-store' });
      if (res.ok) rows = layerRows(await res.json());
    } catch (err) {
      console.warn('layers unavailable:', err);
    }
    if (layersShown !== key) return; // a later render won
    layersEntry = entry;
    layersPanel.querySelector('summary').textContent = `Layer roles · ${entry.name}`;
    layersPanel.hidden = rows.length === 0;
    const guessed = rows.filter((r) => r.guessed).length;
    layersIntro.textContent = `“${entry.name}” has ${rows.length} layers`
      + (guessed ? `; ${guessed} match no profile and were read by what is drawn on them` : '')
      + '. Stage changes below, then Apply and retrace once. The named profile is also used for future drawings.';
    const defaultName = info.profile && !nameProblem(info.profile) ? info.profile : suggestName(entry.name);
    $('profile-name').value = drafts.get(entry.id)?.name ?? defaultName;
    d3.select(layerBody).selectAll('tr')
      .data(rows, (r) => r.name)
      .join((enter) => {
        const tr = enter.append('tr');
        tr.append('td').attr('class', 'layer-name');
        tr.append('td').attr('class', 'num');
        const td = tr.append('td');
        td.append('select').on('change', (e, r) => stageRole(layersEntry, r, e.target.value));
        td.append('span').attr('class', 'source');
        return tr;
      })
      .classed('guessed', (r) => r.guessed)
      .each(function show(r) {
        const tr = d3.select(this);
        tr.select('.layer-name').text(r.name);
        tr.select('.num').text(r.entities);
        tr.select('.source').text(sourceLabel(r.source));
        const select = tr.select('select').attr('aria-label', `Role of layer ${r.name}`);
        // The first option keeps what the built-ins or a guess say; picking
        // it takes back a role this page saved.
        const options = [{ role: '', label: r.mine ? 'Built-in / guess' : `${r.role} (${r.guessed ? 'guess' : 'built-in'})` },
          ...roles.map((x) => ({ role: x.role, label: x.label }))];
        select.selectAll('option').data(options).join('option')
          .attr('value', (o) => o.role).attr('title', (o) => roles.find((x) => x.role === o.role)?.help ?? null)
          .text((o) => o.label);
        const pending = drafts.get(entry.id)?.changes;
        select.property('value', pending?.has(r.name) ? pending.get(r.name) : r.mine ? r.role : '');
        tr.classed('pending', pending?.has(r.name) ?? false);
      });
    renderLayerChanges();
  }

  function renderLayerChanges() {
    const draft = drafts.get(layersEntry?.id), count = draft?.changes.size ?? 0;
    const saved = Boolean(draft?.profileSaved && count);
    $('layer-changes').textContent = applying ? 'Saving and queuing a new trace…'
      : saved ? 'Profile saved; retrace pending.'
        : count ? `${count} staged layer ${count === 1 ? 'change' : 'changes'} for ${layersEntry.name}.` : 'No staged changes.';
    $('layers-apply').disabled = applying || !count;
    $('layers-apply').textContent = saved ? 'Retry retrace' : 'Apply and retrace';
    $('layers-discard').disabled = applying || !count;
    $('layers-discard').textContent = saved ? 'Clear staged retry' : 'Discard changes';
    $('profile-name').disabled = applying || saved;
    layerBody.querySelectorAll('select').forEach(select => { select.disabled = applying || saved; });
    renderReview();
  }
  function stageRole(entry, row, role) {
    if (!drafts.has(entry.id)) drafts.set(entry.id, { name: $('profile-name').value, changes: new Map() });
    const draft = drafts.get(entry.id);
    if (role === (row.mine ? row.role : '')) draft.changes.delete(row.name);
    else draft.changes.set(row.name, role);
    for (const tr of layerBody.rows) tr.classList.toggle('pending', draft.changes.has(tr.querySelector('.layer-name').textContent));
    renderLayerChanges();
  }
  $('profile-name').addEventListener('input', () => {
    const draft = drafts.get(layersEntry?.id); if (draft) draft.name = $('profile-name').value;
  });
  $('layers-discard').addEventListener('click', () => {
    const saved = drafts.get(layersEntry?.id)?.profileSaved;
    say(saved ? 'Cleared the staged retry. The saved profile is kept; use Retrace when ready.' : `Discarded staged layer changes for “${layersEntry.name}”.`);
    drafts.delete(layersEntry?.id); layersShown = ''; renderLayers();
  });
  $('layers-apply').addEventListener('click', async () => {
    const entry = layersEntry, draft = drafts.get(entry?.id);
    if (applying || !draft?.changes.size) return;
    const name = $('profile-name').value.trim();
    const problem = nameProblem(name);
    if (problem) { say(problem, true); $('profile-name').focus(); return; }
    applying = true; renderLayerChanges();
    try {
      if (!draft.profileSaved) {
        const { profile } = await api.profile(name);
        let next = profile;
        for (const [layer, role] of draft.changes) next = setLayerRole(next, layer, role);
        await api.saveProfile(name, next);
        info.profile = name; draft.profileSaved = true;
      }
      await api.review(entry.id, 'retrace');
      drafts.delete(entry.id); track(entry.id);
      say(`Saved layer roles for “${entry.name}”; one new trace is queued.`);
    } catch (err) {
      say(draft.profileSaved
        ? `Profile saved, but retracing “${entry.name}” failed: ${err.message}. Use Retry retrace to queue the saved roles.`
        : `Could not save changes to “${entry.name}”: ${err.message}. Your staged changes are kept.`, true);
    } finally {
      applying = false; layersShown = ''; await refresh(); renderLayerChanges();
    }
  });

  // ---- the floor package: a draft's space plan and other files beside its
  // drawing. Adding or removing one traces the plan again from the drawing
  // already converted.
  spaceAdd.addEventListener('click', () => spaceInput.click());
  spaceInput.addEventListener('change', () => {
    const [file] = spaceInput.files;
    spaceInput.value = ''; // so choosing the same file again still fires
    const entry = shown();
    if (!file || !entry) return;
    act(async () => {
      // The same check as an upload, then the formats a space plan is read
      // from, so a wrong file is turned away here rather than by the server.
      if (file.size > info.maxUploadBytes) {
        throw new Error(`${file.name}: over the ${Math.round(info.maxUploadBytes / 2 ** 20)} MB upload limit.`);
      }
      const problem = spacePlanProblem(file.name, await checkFile(file, info.formats));
      if (problem) throw new Error(problem);
      const { duplicate } = await api.attach(entry.id, file, 'spaces');
      if (duplicate) {
        say(`“${file.name}” is already part of “${entry.name}”.`);
        return;
      }
      track(entry.id);
      say(`“${file.name}” is the space plan of “${entry.name}” now; tracing it again.`);
    });
  });

  function renderPackage() {
    const entry = shown();
    const rows = packageRows(entry);
    if (entry?.generated && entry.upload) rows.unshift({ sha: 'drawing', name: entry.files?.find(f => f.role === 'architecture')?.name ?? entry.name,
      href: `${entry.dir}${entry.upload}`, label: 'Drawing', locked: true });
    const adding = Boolean(info) && canAddSpaces(entry) && entry.status === 'draft';
    packagePanel.hidden = !info || !entry?.generated || (!adding && rows.length === 0);
    if (packagePanel.hidden) return;
    spaceAdd.hidden = !adding;
    spaceInput.accept = acceptAttr(spaceFormats(info.formats));
    packageIntro.textContent = rows.length
      ? `“${entry.name}” is traced from its drawing with these files.`
      : 'A space plan from the same floor (DXF or DWG) gives the rooms their outlines, numbers and names.';
    d3.select(packageFiles).selectAll('li')
      .data(rows, (r) => r.sha)
      .join((enter) => {
        const li = enter.append('li');
        li.append('a').attr('target', '_blank').attr('rel', 'noopener');
        li.append('span').attr('class', 'role');
        li.append('button').attr('type', 'button').text('Remove')
          .on('click', (e, r) => removeFile(shown(), r));
        return li;
      })
      .each(function show(r) {
        const li = d3.select(this);
        li.select('a').attr('href', r.href).text(r.name);
        li.select('.role').text(r.label);
        li.select('button').attr('aria-label', `Remove ${r.name}`).attr('hidden', !r.locked && canChangePackage(entry) ? null : '');
      });
  }

  function removeFile(entry, row) {
    act(async () => {
      await api.detach(entry.id, row.sha);
      track(entry.id);
      say(`Took “${row.name}” out of “${entry.name}”; tracing it again.`);
    });
  }

  // ---- the pipeline list
  function renderPipeline() {
    const rows = [...uploading.values(), ...inPipeline(plans())];
    list.hidden = rows.length === 0;
    $('processing-empty').hidden = rows.length !== 0;
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
        actions.append('button').attr('type', 'button').attr('class', 'layers-open').text('Layers')
          .on('click', (e, d) => {
            layersFor = d.id; onLayers?.();
            renderLayers().then(() => layersPanel.scrollIntoView?.({ block: 'nearest' }));
          });
        actions.append('button').attr('type', 'button').attr('class', 'retry').text('Retry')
          .on('click', (e, d) => act(async () => {
            await api.review(d.id, 'retry');
            track(d.id);
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
        // A drawing that failed may have its walls on a layer no profile
        // names: its layers are where to say so.
        row.select('.layers-open')
          .attr('hidden', info?.profiles && d.status === 'failed' && drawings.has(d.format) ? null : '');
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

  let lastCurrent = current();
  function update() {
    if (lastCurrent !== current()) { layersFor = null; lastCurrent = current(); }
    renderReview();
    renderPipeline();
    renderLayers();
    renderPackage();
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
