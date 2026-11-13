import * as plan from './plan.js';
import { PRESETS } from './projection.js';
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

// Skin choice: ?skin= wins so a link can show a particular look, then the
// viewer's last pick. Storage can throw (private mode, blocked site data).
function initialSkin() {
  const known = (id) => SKINS.some((s) => s.id === id);
  const fromUrl = new URLSearchParams(location.search).get('skin');
  if (known(fromUrl)) return fromUrl;
  try {
    const saved = localStorage.getItem('floorplan-skin');
    if (known(saved)) return saved;
  } catch { /* fall through to the default */ }
  return DEFAULT_SKIN;
}
stageWrap.dataset.skin = initialSkin();

const scene = createScene({
  styleRoot: stageWrap,
  svg: document.querySelector('#stage'),
  tooltip: document.querySelector('#tooltip'),
  plan,
  onStats: ({ prisms, faces, ms }) => {
    stats.textContent = `${prisms} prisms · ${faces} faces · ${ms.toFixed(1)} ms/frame`;
  },
});

// Several tweens can run at once (camera, build, sliders); coalesce them into
// one render per animation frame.
let pending = false;
function invalidate() {
  if (pending) return;
  pending = true;
  requestAnimationFrame(() => {
    pending = false;
    scene.render(state);
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
  scene.fit({ ...state, ...to }, duration);
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
document.querySelector('#reset').addEventListener('click', () => scene.fit(state, 600));

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
  scene.refreshPalette();
  invalidate();
  try {
    localStorage.setItem('floorplan-skin', id);
  } catch { /* the choice just won't persist */ }
  const url = new URL(location.href);
  url.searchParams.set('skin', id);
  history.replaceState(null, '', url);
}
picker.select('input').on('change', (e, s) => applySkin(s.id));
skinBlurb.textContent = SKINS.find((s) => s.id === stageWrap.dataset.skin).blurb;
// The handwriting font may arrive after the first frame; labels re-measure for free,
// but re-render so nothing waits on the next interaction.
document.fonts?.ready.then(invalidate);

const dark = window.matchMedia('(prefers-color-scheme: dark)');
dark.addEventListener('change', () => {
  scene.refreshPalette();
  invalidate();
});
window.addEventListener('resize', () => scene.fit(state));

syncOutputs();
scene.render(state);
scene.fit(state);
build();

// Exposed for poking at the scene from the console.
globalThis.floorplan = {
  state, invalidate, animateView, build,
  refreshPalette: () => { scene.refreshPalette(); invalidate(); },
};
