import * as plan from './plan.js';
import { PRESETS } from './projection.js';
import { createScene } from './render.js';

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
};

const stats = document.querySelector('#stats');
const scene = createScene({
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

function animateView(target, duration = 1100) {
  const from = { theta: state.theta, phi: state.phi };
  const to = { ...from, ...target };
  const theta = d3.interpolateNumber(from.theta, to.theta);
  const phi = d3.interpolateNumber(from.phi, to.phi);
  scene.fit({ ...state, ...to }, duration);
  d3.select(document.body).transition('view').duration(duration).ease(d3.easeCubicInOut)
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
  d3.select(document.body).transition('build').duration(duration).ease(d3.easeLinear)
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
