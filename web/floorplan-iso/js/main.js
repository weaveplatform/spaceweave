import { PRESETS } from './projection.js';
import { PLANS, DEFAULT_PLAN, LISTED, READY, loadCatalog } from './plans/index.js';
import { busy } from './pipeline.js';
import { initWorkbench } from './workbench.js';
import { createScene } from './render.js';
import { SKINS, DEFAULT_SKIN } from './skins.js';
import { initWorkspace } from './workspace.js';
import { describeSelection } from './selection.js';

const d3 = globalThis.d3;

const state = {
  ...PRESETS.iso,
  // Cutaway height: at full 2.7 m the interior walls hide most of the furniture
  // from an isometric camera. The slider goes back up to full height.
  wallHeight: 1.5,
  cutaway: true,
  cutawayMode: 'exterior',
  cutHeight: 0.3,
  activeRoom: null,
  furniture: true,
  doors: true,
  glass: true,
  labels: true,
  people: true,
  build: 0,
  moving: false,
};

const stats = document.querySelector('#stats');
const stageWrap = document.querySelector('.stage-wrap');

// Read ?key= and fall back to the viewer's last choice, then the default.
// Storage can throw (private mode, blocked site data).
function remembered(key, known, fallback) {
  const fromUrl = new URLSearchParams(location.search).get(key);
  if (known(fromUrl)) return fromUrl;
  try {
    const saved = localStorage.getItem(`floorplan-${key}`);
    if (known(saved)) return saved;
  } catch { /* fall through */ }
  return fallback;
}

function remember(key, value) {
  try {
    localStorage.setItem(`floorplan-${key}`, value);
  } catch { /* the choice just won't persist */ }
  const url = new URL(location.href);
  url.searchParams.set(key, value);
  history.replaceState(null, '', url);
}

stageWrap.dataset.skin = remembered('skin', (id) => SKINS.some((s) => s.id === id), DEFAULT_SKIN);

// The scene is rebuilt from scratch when the plan changes; everything else
// (camera, skin, toggles) carries over.
let scene = null, loadedPlan = null, selection = null, cameraFocus = null;
let loadedRevision = null;
const awaitingTrace = new Set();

// Several tweens can run at once (camera, build, sliders); coalesce them into
// one render per animation frame.
let pending = false;
function invalidate() {
  if (pending) return;
  pending = true;
  requestAnimationFrame(() => {
    pending = false;
    scene?.render(state);
  });
}

const elevation = document.querySelector('#elevation');
const elevationOut = document.querySelector('#elevation-out');
const wallHeight = document.querySelector('#wall-height');
const wallHeightOut = document.querySelector('#wall-height-out');
const cutHeight = document.querySelector('#cut-height');
const cutHeightOut = document.querySelector('#cut-height-out');
const roomFocus = document.querySelector('#room-focus');

const workspace = initWorkspace({
  onClear: () => selectObject(null), onFocus: focusSelection, onWholeFloor: wholeFloor,
  onModelShown: () => frameView(state),
});

function selectObject(value, toggle = false) {
  if (toggle && selection?.type === value?.type && selection?.id === value?.id) value = null;
  selection = describeSelection(loadedPlan, value);
  scene?.setSelection(selection);
  workspace.setSelection(selection);
}

function setRoomFocus(id) {
  const room = loadedPlan?.rooms.find(r => r.id === id) ?? null;
  state.activeRoom = room?.id ?? null;
  workspace.setRoomFocus(room);
  syncCutaways();
  invalidate();
}

function syncCutaways() {
  const mode = state.cutawayMode;
  document.querySelector('#cutaway-mode').value = mode;
  document.querySelector('#cutaway-settings').hidden = mode === 'off';
  const room = loadedPlan?.rooms.find(r => r.id === state.activeRoom);
  document.querySelector('#cutaway-help').textContent = mode === 'exterior'
    ? 'Near exterior walls are lowered. Interior walls remain.'
    : room ? `Walls blocking ${room.name} are lowered; rear walls stay up. At 0 m, cut walls and their doors are hidden.`
      : 'Select a room, then choose Focus to reveal interior walls.';
}

function setCutawayMode(mode) {
  state.cutawayMode = mode; state.cutaway = mode !== 'off';
  if (mode === 'room' && !state.activeRoom && selection?.type === 'room') setRoomFocus(selection.id);
  syncCutaways(); invalidate();
}

function frameView(target, duration = 0) {
  if (cameraFocus) scene?.focusObject(cameraFocus, target, duration);
  else scene?.fit(target, duration);
}

function focusSelection() {
  if (!selection) return;
  cameraFocus = { type: selection.type, id: selection.id };
  setRoomFocus(selection.roomId);
  if (selection.roomId && state.cutawayMode !== 'off') setCutawayMode('room');
  frameView(state, 750);
}

function wholeFloor() {
  cameraFocus = null; setRoomFocus(null);
  if (state.cutawayMode === 'room') setCutawayMode('exterior');
  scene?.fit(state, 600);
}

function syncOutputs() {
  elevation.value = state.phi;
  elevationOut.textContent = `${Math.round(state.phi)}°`;
  wallHeight.value = state.wallHeight;
  wallHeightOut.textContent = `${state.wallHeight.toFixed(1)} m`;
  cutHeight.max = state.wallHeight;
  cutHeight.value = state.cutHeight;
  const height = Math.min(state.cutHeight, state.wallHeight);
  cutHeightOut.textContent = height === 0 ? 'Hidden' : `${height.toFixed(1)} m`;
}

// SVG filters (Sketch's wobble, Neon's glow, the soft ground shadow) are
// re-rasterised on every frame, which halves the frame rate while the camera
// moves. They are dropped for the duration of a camera or build tween; the
// eye doesn't miss them in motion and they return when it settles.
let moving = 0;
function motion(transition) {
  moving++;
  stageWrap.classList.add('moving');
  state.moving = true;
  const settle = () => {
    moving = Math.max(0, moving - 1);
    if (moving === 0) {
      stageWrap.classList.remove('moving');
      state.moving = false;
      invalidate(); // bring back what was skipped in motion (hatching)
    }
  };
  return transition.on('end.motion', settle).on('interrupt.motion', settle).on('cancel.motion', settle);
}

function animateView(target, duration = 1100) {
  const from = { theta: state.theta, phi: state.phi };
  const to = { ...from, ...target };
  const theta = d3.interpolateNumber(from.theta, to.theta);
  const phi = d3.interpolateNumber(from.phi, to.phi);
  frameView({ ...state, ...to }, duration);
  motion(d3.select(document.body).transition('view').duration(duration).ease(d3.easeCubicInOut))
    .tween('view', () => (t) => {
      state.theta = theta(t);
      state.phi = phi(t);
      syncOutputs();
      invalidate();
    })
    .on('end', () => {
      // Keep θ in (-180, 180] so repeated rotations don't wind up.
      state.theta = ((((to.theta + 180) % 360) + 360) % 360) - 180;
    });
}

function build(duration = 2800) {
  motion(d3.select(document.body).transition('build').duration(duration).ease(d3.easeLinear))
    .tween('build', () => (t) => {
      state.build = t;
      invalidate();
    });
}

function setPreset(name) {
  document.querySelectorAll('[data-preset]').forEach((b) => b.setAttribute('aria-pressed', b.dataset.preset === name));
  animateView(PRESETS[name]);
}

document.querySelectorAll('[data-preset]').forEach((b) => b.addEventListener('click', () => setPreset(b.dataset.preset)));
document.querySelector('#rotate-left').addEventListener('click', () => animateView({ theta: state.theta - 90 }, 900));
document.querySelector('#rotate-right').addEventListener('click', () => animateView({ theta: state.theta + 90 }, 900));
document.querySelector('#rebuild').addEventListener('click', () => build());
document.querySelector('#reset').addEventListener('click', () => { cameraFocus = null; scene?.fit(state, 600); });

elevation.addEventListener('input', () => {
  state.phi = +elevation.value;
  document.querySelectorAll('[data-preset]').forEach(button => {
    button.setAttribute('aria-pressed', String(Math.abs(PRESETS[button.dataset.preset].phi - state.phi) < 0.5));
  });
  frameView(state);
  syncOutputs();
  invalidate();
});
wallHeight.addEventListener('input', () => {
  state.wallHeight = +wallHeight.value;
  syncOutputs();
  invalidate();
});
cutHeight.addEventListener('input', () => {
  state.cutHeight = +cutHeight.value;
  syncOutputs();
  invalidate();
});
roomFocus.addEventListener('change', () => selectObject(roomFocus.value ? { type: 'room', id: roomFocus.value } : null));
document.querySelector('#cutaway-mode').addEventListener('change', event => setCutawayMode(event.target.value));
document.addEventListener('keydown', event => {
  if (event.key === 'Escape' && !event.defaultPrevented && !document.querySelector('dialog[open]')) selectObject(null);
});
document.querySelectorAll('[data-toggle]').forEach((input) => {
  input.checked = state[input.dataset.toggle];
  input.addEventListener('change', () => {
    state[input.dataset.toggle] = input.checked;
    invalidate();
  });
});

// ---- style picker
const skinBlurb = document.querySelector('#skin-blurb');
const picker = d3.select('#skins').selectAll('label').data(SKINS).join('label');
picker.append('input').attr('type', 'radio').attr('name', 'skin')
  .attr('value', (s) => s.id)
  .property('checked', (s) => s.id === stageWrap.dataset.skin);
picker.append('span').attr('class', 'chip')
  .style('background', ({ swatch: [a, b, c] }) => `linear-gradient(135deg, ${a} 0 40%, ${b} 40% 70%, ${c} 70%)`);
picker.append('span').text((s) => s.name);
picker.attr('title', (s) => s.blurb);

function applySkin(id) {
  stageWrap.dataset.skin = id;
  skinBlurb.textContent = SKINS.find((s) => s.id === id).blurb;
  scene?.refreshPalette();
  invalidate();
  remember('skin', id);
}
picker.select('input').on('change', (e, s) => applySkin(s.id));
skinBlurb.textContent = SKINS.find((s) => s.id === stageWrap.dataset.skin).blurb;
// The handwriting font may arrive after the first frame; labels re-measure for free,
// but re-render so nothing waits on the next interaction.
document.fonts?.ready.then(invalidate);

const dark = window.matchMedia('(prefers-color-scheme: dark)');
dark.addEventListener('change', () => {
  scene?.refreshPalette();
  invalidate();
});
window.addEventListener('resize', () => frameView(state));

// ---- plan picker
const planBlurb = document.querySelector('#plan-blurb');
const sourceLink = document.querySelector('#source-link');
const traceMethod = document.querySelector('#trace-method');
const HAND_TRACED = 'Example plan traced by hand from its published source.';
const planName = document.querySelector('#plan-name');

// Built-in plans plus the tracer's catalogue, which is re-read while the page
// is open so a newly traced plan appears without a reload.
let plans = PLANS;
let current = null;
let workbench = null;

function renderPicker() {
  const generated = plans.filter(p => p.generated && LISTED.has(p.status));
  const floors = document.querySelector('#floor-select');
  d3.select(floors).selectAll('option.floor').data(generated, p => p.id).join('option')
    .attr('class', 'floor').attr('value', p => p.id).property('disabled', p => !READY.has(p.status))
    .text(p => `${p.name}${READY.has(p.status) ? '' : ` · ${p.status}`}`);
  floors.value = generated.some(p => p.id === current) ? current : '';
  floors.options[0].textContent = generated.length ? 'Your floors' : 'No imported floors';
  d3.select('#example-select').selectAll('option.example').data(PLANS, p => p.id).join('option')
    .attr('class', 'example').attr('value', p => p.id).text(p => p.name);
  document.querySelector('#example-select').value = PLANS.some(p => p.id === current) ? current : '';
}
document.querySelector('#floor-select').addEventListener('change', event => { if (event.target.value) loadPlan(event.target.value); });
document.querySelector('#example-select').addEventListener('change', event => { if (event.target.value) loadPlan(event.target.value); });

function renderBlurb(entry) { workspace.setPlan(entry, loadedPlan); }

// The tracer takes minutes per plan, so a 10 s poll is plenty while it's
// idle; while it has work, poll faster so the pipeline's turn and cost
// counters move.
let poll = 0;
function schedule() {
  clearTimeout(poll);
  poll = setTimeout(() => { if (!document.hidden) refreshCatalog(); else schedule(); }, busy(plans) ? 2_000 : 10_000);
}

async function refreshCatalog() {
  try {
    plans = [...PLANS, ...(await loadCatalog())];
  } catch (err) {
    console.warn('plan catalogue unavailable:', err);
    schedule();
    return;
  }
  renderPicker();
  const entry = plans.find((p) => p.id === current);
  if (entry) renderBlurb(entry);
  workbench?.update();
  if (entry?.generated && ['queued', 'converting', 'tracing'].includes(entry.status)) awaitingTrace.add(entry.id);
  if (!loadingPlan && entry && READY.has(entry.status) && awaitingTrace.has(entry.id) && loadedRevision !== entry.updatedAt) await loadPlan(entry.id);
  schedule();
}

let loading = 0;
let loadingPlan = null;
async function loadPlan(id) {
  const entry = plans.find((p) => p.id === id && (!p.generated || READY.has(p.status))) ?? PLANS[0];
  const ticket = ++loading;
  loadingPlan = entry.id;
  document.querySelector('#stage').setAttribute('aria-busy', 'true');
  let plan;
  try {
    plan = await entry.load();
  } catch (err) {
    console.error(err);
    if (ticket !== loading) return;
    loadingPlan = null;
    renderPicker();
    document.querySelector('#stage').removeAttribute('aria-busy');
    document.querySelector('#upload-status').textContent = `${entry.name} couldn't be loaded. Your previous floor is still open.`;
    return;
  }
  if (ticket !== loading) return; // a later click won
  loadingPlan = null;
  scene?.destroy();
  current = entry.id; loadedPlan = plan; selection = cameraFocus = null;
  loadedRevision = entry.updatedAt; awaitingTrace.delete(entry.id);
  workspace.setSelection(null);
  setRoomFocus(null);
  renderPicker(); renderBlurb(entry); workbench?.update();
  const wholeFloor = new Option('Choose a room…', '');
  roomFocus.replaceChildren(wholeFloor, ...plan.rooms.map(room =>
    new Option(`${room.name}${room.number && room.number !== room.name ? ` · ${room.number}` : ''}`, room.id)));
  scene = createScene({
    styleRoot: stageWrap,
    svg: document.querySelector('#stage'),
    tooltip: document.querySelector('#tooltip'),
    plan,
    onSelect: value => selectObject(value, true),
    onStats: ({ prisms, faces, ms }) => {
      stats.textContent = `${prisms} prisms · ${faces} faces · ${ms.toFixed(1)} ms/frame`;
    },
  });
  sourceLink.href = plan.SOURCE.url;
  sourceLink.textContent = plan.SOURCE.credit;
  traceMethod.textContent = !entry.generated ? HAND_TRACED
    : entry.tracer === 'claude'
      ? 'Traced with the Claude tracer.' : 'Traced locally with the automatic tracer.';
  planName.textContent = plan.SOURCE.title;
  document.querySelector('#stage').setAttribute('aria-label', `Isometric model of the ${plan.SOURCE.title} floor plan`);
  remember('plan', entry.id);
  document.querySelector('#stage').removeAttribute('aria-busy');
  state.build = 0;
  scene.render(state);
  scene.fit(state);
  build();
}
syncOutputs(); syncCutaways();
workbench = initWorkbench({
  plans: () => plans,
  current: () => current,
  refresh: refreshCatalog,
  open: (id) => { workspace.setMode('review'); return loadPlan(id); },
  onImportStarted: () => workspace.importStarted(),
  onLayers: () => workspace.showLayers(),
  onTraceQueued: id => awaitingTrace.add(id),
});
await refreshCatalog();
loadPlan(remembered('plan', id => plans.some(p => p.id === id && (!p.generated || READY.has(p.status))), plans.find(p => p.generated && READY.has(p.status))?.id ?? DEFAULT_PLAN));
document.addEventListener('visibilitychange', () => { if (!document.hidden) refreshCatalog(); });

// Exposed for poking at the scene from the console.
globalThis.floorplan = {
  state, invalidate, animateView, build, loadPlan, refreshCatalog,
  get selection() { return selection; },
  refreshPalette: () => { scene?.refreshPalette(); invalidate(); },
};
