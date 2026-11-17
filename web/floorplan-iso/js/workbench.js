// The page's side of `spaceweave serve`: dropping, choosing or pasting a
// plan uploads it, the pipeline list follows each plan through conversion and
// the tracer, and a draft can be approved or sent back with a note. A
// drawing's layers can be given roles, saved in the client's profile and
// traced again. A drawing's floor package can take a space plan, which is
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

  // ---- layers: what each of a drawing's layers was read as, and a menu to
  // say otherwise. A change is saved in the client's profile, which every
  // later trace reads, and this plan is traced again with it.
  let layersFor = null; // a failed drawing picked from the pipeline list
  let layersShown = ''; // the plan and version whose report is on show
  let layersEntry = null; // the plan whose layers are on show
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
    layersPanel.hidden = rows.length === 0;
    const guessed = rows.filter((r) => r.guessed).length;
    layersIntro.textContent = `“${entry.name}” has ${rows.length} layers`
      + (guessed ? `; ${guessed} match no profile and were read by what is drawn on them` : '')
      + '. Choosing a role saves it in '
      + (info.profile ? `the “${info.profile}” profile` : 'a client profile you name')
      + ' and traces the plan again.';
    d3.select(layerBody).selectAll('tr')
      .data(rows, (r) => r.name)
      .join((enter) => {
        const tr = enter.append('tr');
        tr.append('td').attr('class', 'layer-name');
        tr.append('td').attr('class', 'num');
        const td = tr.append('td');
        td.append('select').on('change', (e, r) => changeRole(layersEntry, r, e.target.value));
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
        select.property('value', r.mine ? r.role : '');
      });
  }

  function changeRole(entry, row, role) {
    // Whatever happens, the menus are drawn again from report.json after:
    // the new role once the retrace is done, or the old one if it failed.
    layersShown = '';
    act(async () => {
      let name = info.profile;
      if (!name) {
        name = (globalThis.prompt('Name the client profile to keep layer roles in. Every drawing traced here '
          + 'is read with it.', suggestName(entry.name)) ?? '').trim();
        if (!name) return;
        const problem = nameProblem(name);
        if (problem) throw new Error(problem);
      }
      const { profile } = await api.profile(name);
      await api.saveProfile(name, setLayerRole(profile, row.name, role));
      info.profile = name;
      await api.review(entry.id, 'retrace');
      mine.add(entry.id);
      say(role
        ? `Layer “${row.name}” is ${role} in profile “${name}”; tracing “${entry.name}” again.`
        : `Layer “${row.name}” is back to the built-in reading; tracing “${entry.name}” again.`);
    });
  }

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
      mine.add(entry.id);
      say(`“${file.name}” is the space plan of “${entry.name}” now; tracing it again.`);
    });
  });

  function renderPackage() {
    const entry = shown();
    const rows = packageRows(entry);
    const adding = Boolean(info) && canAddSpaces(entry) && entry.status === 'draft';
    packagePanel.hidden = !info || !entry?.generated || entry.status !== 'draft' || (!adding && rows.length === 0);
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
        li.select('button').attr('aria-label', `Remove ${r.name}`).attr('hidden', canChangePackage(entry) ? null : '');
      });
  }

  function removeFile(entry, row) {
    act(async () => {
      await api.detach(entry.id, row.sha);
      mine.add(entry.id);
      say(`Took “${row.name}” out of “${entry.name}”; tracing it again.`);
    });
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
        actions.append('button').attr('type', 'button').attr('class', 'layers-open').text('Layers')
          .on('click', (e, d) => {
            layersFor = d.id;
            renderLayers().then(() => layersPanel.scrollIntoView?.({ block: 'nearest' }));
          });
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

  function update() {
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
