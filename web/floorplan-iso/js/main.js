import { PRESETS } from './projection.js';
import { PLANS, DEFAULT_PLAN, READY, loadCatalog } from './plans/index.js';
import { createScene } from './render.js';
import { SKINS, DEFAULT_SKIN } from './skins.js';

const d3 = globalThis.d3;

const state = {
  ...PRESETS.iso,
  // Cutaway height: at full 2.7 m the interior walls hide most of the furniture
  // from an isometric camera. The slider goes back up to full height.
  wallHeight: 1.5,
  cutaway: true,
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
let scene = null;

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

function syncOutputs() {
  elevation.value = state.phi;
  elevationOut.textContent = `${Math.round(state.phi)}°`;
  wallHeight.value = state.wallHeight;
  wallHeightOut.textContent = `${state.wallHeight.toFixed(1)} m`;
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
  return transition.on('end.motion', settle).on('interrupt.motion', settle);
}

function animateView(target, duration = 1100) {
  const from = { theta: state.theta, phi: state.phi };
  const to = { ...from, ...target };
  const theta = d3.interpolateNumber(from.theta, to.theta);
  const phi = d3.interpolateNumber(from.phi, to.phi);
  scene?.fit({ ...state, ...to }, duration);
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
document.querySelector('#reset').addEventListener('click', () => scene?.fit(state, 600));

elevation.addEventListener('input', () => {
  state.phi = +elevation.value;
  syncOutputs();
  invalidate();
});
wallHeight.addEventListener('input', () => {
  state.wallHeight = +wallHeight.value;
  syncOutputs();
  invalidate();
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
window.addEventListener('resize', () => scene?.fit(state));

// ---- plan picker
const planBlurb = document.querySelector('#plan-blurb');
const sourceLink = document.querySelector('#source-link');
const planName = document.querySelector('#plan-name');

// Built-in plans plus the tracer's catalogue, which is re-read while the page
// is open so a newly traced plan appears without a reload.
let plans = PLANS;
let current = null;

function renderPicker() {
  d3.select('#plans').selectAll('label')
    .data(plans, (p) => p.id)
    .join((enter) => {
      const label = enter.append('label');
      label.append('input').attr('type', 'radio').attr('name', 'plan').attr('value', (p) => p.id)
        .on('change', (e, p) => loadPlan(p.id));
      label.append('span').attr('class', 'plan-name');
      label.append('span').attr('class', 'badge');
      return label;
    })
    .attr('title', (p) => p.blurb)
    .classed('generated', (p) => p.generated)
    .classed('pending', (p) => p.generated && !READY.has(p.status))
    .call((label) => {
      label.select('input')
        .property('checked', (p) => p.id === current)
        .property('disabled', (p) => p.generated && !READY.has(p.status));
      label.select('.plan-name').text((p) => p.name);
      label.select('.badge')
        .text((p) => ({ draft: 'Draft', queued: 'Queued', tracing: 'Tracing…' })[p.status] ?? '')
        .attr('hidden', (p) => (['draft', 'queued', 'tracing'].includes(p.status) ? null : ''));
    });
}

// A generated plan's blurb carries its review status and links to the
// tracer's evidence, so a draft can be judged without leaving the page.
function renderBlurb(entry) {
  const blurb = d3.select(planBlurb).text(entry.blurb);
  if (!entry.generated) return;
  const facts = [entry.status === 'draft' ? 'Draft, not yet reviewed' : 'Auto-traced'];
  if (Number.isFinite(entry.confidence)) facts.push(`confidence ${Math.round(entry.confidence * 100)}%`);
  blurb.append('br');
  blurb.append('span').text(`${facts.join(' · ')} · `);
  blurb.append('a').attr('href', `${entry.dir}overlay.png`).attr('target', '_blank').text('overlay');
  blurb.append('span').text(' · ');
  blurb.append('a').attr('href', `${entry.dir}report.json`).attr('target', '_blank').text('report');
}

async function refreshCatalog() {
  try {
    plans = [...PLANS, ...(await loadCatalog())];
  } catch (err) {
    console.warn('plan catalogue unavailable:', err);
    return;
  }
  renderPicker();
  const entry = plans.find((p) => p.id === current);
  if (entry) renderBlurb(entry);
}

let loading = 0;
async function loadPlan(id) {
  const entry = plans.find((p) => p.id === id && (!p.generated || READY.has(p.status))) ?? PLANS[0];
  const ticket = ++loading;
  current = entry.id;
  renderPicker();
  renderBlurb(entry);
  let plan;
  try {
    plan = await entry.load();
  } catch (err) {
    console.error(err);
    planBlurb.textContent = `${entry.name} couldn't be loaded.`;
    return;
  }
  if (ticket !== loading) return; // a later click won
  scene?.destroy();
  scene = createScene({
    styleRoot: stageWrap,
    svg: document.querySelector('#stage'),
    tooltip: document.querySelector('#tooltip'),
    plan,
    onStats: ({ prisms, faces, ms }) => {
      stats.textContent = `${prisms} prisms · ${faces} faces · ${ms.toFixed(1)} ms/frame`;
    },
  });
  sourceLink.href = plan.SOURCE.url;
  sourceLink.textContent = plan.SOURCE.credit;
  planName.textContent = plan.SOURCE.title;
  document.querySelector('#stage').setAttribute('aria-label', `Isometric model of the ${plan.SOURCE.title} floor plan`);
  remember('plan', entry.id);
  state.build = 0;
  scene.render(state);
  scene.fit(state);
  build();
}
syncOutputs();
await refreshCatalog();
loadPlan(remembered('plan', (id) => plans.some((p) => p.id === id), DEFAULT_PLAN));
// The tracer takes minutes per plan, so a 10 s poll is plenty; also catch up
// as soon as the tab is looked at again.
setInterval(() => { if (!document.hidden) refreshCatalog(); }, 10_000);
document.addEventListener('visibilitychange', () => { if (!document.hidden) refreshCatalog(); });

// Exposed for poking at the scene from the console.
globalThis.floorplan = {
  state, invalidate, animateView, build, loadPlan,
  refreshPalette: () => { scene?.refreshPalette(); invalidate(); },
};
