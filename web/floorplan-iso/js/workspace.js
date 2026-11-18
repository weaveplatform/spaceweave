export function statusLabel(entry) {
  if (!entry?.generated) return 'Example';
  return { draft: 'Needs review', approved: 'Approved', queued: 'Queued', converting: 'Converting',
    tracing: 'Tracing', failed: 'Failed' }[entry.status] ?? 'Unavailable';
}

export function initWorkspace({ onFocus, onClear, onWholeFloor, onModelShown }) {
  const $ = id => document.getElementById(id);
  const tabs = [...document.querySelectorAll('[role="tab"][data-workspace]')];
  const dialog = $('import-dialog');
  let mode = 'view', evidence = 'model', entry = null, plan = null, room = null, reportKey = '';

  function showEvidence() {
    const visual = mode === 'view' ? 'model' : evidence;
    const wasHidden = $('stage').hidden;
    $('stage').hidden = visual !== 'model';
    if (wasHidden && visual === 'model') requestAnimationFrame(() => onModelShown?.());
    $('evidence-view').hidden = visual === 'model';
    $('model-tools').hidden = visual !== 'model';
    $('model-legend').hidden = visual !== 'model';
    $('focus-banner').hidden = !room || visual !== 'model';
    $('evidence-open').hidden = visual === 'model';
    for (const button of document.querySelectorAll('[data-evidence]')) button.setAttribute('aria-pressed', String(button.dataset.evidence === visual));
    if (visual === 'model') return;
    const image = $('evidence-image');
    const url = visual === 'overlay' && entry?.generated ? `${entry.dir}overlay.png` : plan?.SOURCE.url;
    const failed = image.dataset.failed === url;
    $('evidence-error').hidden = Boolean(url) && !failed;
    image.hidden = !url || failed;
    $('evidence-open').hidden = !url;
    if (url) $('evidence-open').href = url;
    if (url && image.getAttribute('src') !== url) {
      image.alt = `${entry?.name ?? 'Floor'} · ${visual === 'overlay' ? 'trace overlay' : 'source drawing'}`;
      image.src = url;
    }
  }
  $('evidence-image').addEventListener('error', () => {
    const image = $('evidence-image'); image.dataset.failed = image.getAttribute('src');
    $('evidence-error').hidden = false; image.hidden = true;
  });
  $('evidence-image').addEventListener('load', () => {
    delete $('evidence-image').dataset.failed; $('evidence-error').hidden = true; $('evidence-image').hidden = false;
  });
  for (const button of document.querySelectorAll('[data-evidence]')) button.addEventListener('click', () => {
    evidence = button.dataset.evidence; showEvidence();
  });

  function setMode(next) {
    mode = next;
    document.body.dataset.workspace = mode;
    for (const tab of tabs) {
      const active = tab.dataset.workspace === mode;
      tab.setAttribute('aria-selected', String(active)); tab.tabIndex = active ? 0 : -1;
      $(tab.getAttribute('aria-controls')).hidden = !active;
    }
    $('evidence-tools').hidden = mode !== 'review';
    showEvidence();
  }
  for (const tab of tabs) {
    tab.addEventListener('click', () => setMode(tab.dataset.workspace));
    tab.addEventListener('keydown', event => {
      if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return;
      event.preventDefault();
      const next = event.key === 'Home' ? tabs[0] : event.key === 'End' ? tabs.at(-1) : tabs[(tabs.indexOf(tab) + 1) % tabs.length];
      next.focus(); setMode(next.dataset.workspace);
    });
  }
  $('import-open').addEventListener('click', () => { $('import-status').replaceChildren(); dialog.showModal(); });
  $('import-close').addEventListener('click', () => dialog.close());
  dialog.addEventListener('click', event => {
    if (event.target !== dialog) return;
    const box = dialog.getBoundingClientRect();
    if (event.clientX < box.left || event.clientX > box.right || event.clientY < box.top || event.clientY > box.bottom) dialog.close();
  });
  $('selection-focus').addEventListener('click', onFocus);
  $('selection-clear').addEventListener('click', onClear);
  $('whole-floor').addEventListener('click', onWholeFloor);

  function setSelection(selection) {
    $('selection-empty').hidden = Boolean(selection);
    $('selection-info').hidden = !selection;
    $('selection-name').textContent = selection?.label ?? '';
    $('selection-detail').textContent = selection?.detail ?? '';
    $('room-focus').value = selection?.type === 'room' ? selection.id : '';
  }
  function setRoomFocus(value) {
    room = value;
    $('focus-name').textContent = room?.name ?? '';
    showEvidence();
  }
  async function setPlan(value, data) {
    entry = value; plan = data;
    const status = $('plan-status');
    status.textContent = statusLabel(entry); status.dataset.status = entry.status ?? 'example';
    const processing = ['queued', 'converting', 'tracing'].includes(entry.status);
    $('plan-blurb').textContent = entry.blurb + (processing && plan ? ' Showing the previous result while the new trace is processed.' : '');
    $('report-confidence').textContent = Number.isFinite(entry.confidence)
      ? `Tracer confidence: ${Math.round(entry.confidence * 100)}%. This is not a measured accuracy score.` : '';
    const links = [];
    if (entry.generated) {
      links.push(['Trace overlay', 'overlay.png'], ['Trace report', 'report.json']);
      if (entry.drawing) links.push(['Rendered drawing', entry.drawing]);
      if (entry.upload) links.push(['Original file', entry.upload]);
    }
    $('evidence-links').replaceChildren(...links.map(([label, file]) => {
      const a = document.createElement('a'); a.textContent = label; a.href = `${entry.dir}${file}`; a.target = '_blank'; a.rel = 'noopener'; return a;
    }));
    document.querySelector('[data-evidence="overlay"]').disabled = !entry.generated;
    if (!entry.generated && evidence === 'overlay') evidence = 'model';
    showEvidence();
    const key = `${entry.id}@${entry.updatedAt ?? ''}`;
    if (key === reportKey) return;
    reportKey = key;
    $('review-warnings').replaceChildren();
    if (!entry.generated) { $('checks-summary').textContent = 'Example plan. Import your own drawing to review a trace.'; return; }
    $('checks-summary').textContent = 'Loading trace checks…';
    try {
      const response = await fetch(`${entry.dir}report.json`, { cache: 'no-store' });
      if (!response.ok) throw Error('Trace report unavailable.');
      const report = await response.json();
      if (reportKey !== key) return;
      const caveats = report.caveats ?? [];
      $('checks-summary').textContent = caveats.length ? `${caveats.length} items to check against the source drawing.` : 'No warnings reported. Compare the model with the source before approval.';
      $('review-warnings').replaceChildren(...caveats.map(text => { const li = document.createElement('li'); li.textContent = text; return li; }));
    } catch {
      if (reportKey === key) $('checks-summary').textContent = 'Trace checks are unavailable for this floor.';
    }
  }
  setMode('view');
  return { setMode, setPlan, setSelection, setRoomFocus,
    importStarted() { if (dialog.open) dialog.close(); setMode('review'); $('tab-review').focus(); },
    showLayers() { setMode('review'); $('layers').open = true; },
  };
}
