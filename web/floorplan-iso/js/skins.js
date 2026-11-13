// The switcher's catalogue. A skin's look lives in styles.css as a block of
// custom properties on .stage-wrap[data-skin="…"], so a new skin is a CSS block
// plus an entry here. The swatch colours only draw the button preview.
// test/skins.test.js checks every skin's CSS block sets every token, because a
// missing one silently falls back to the Model colour.

export const SKINS = [
  { id: 'model', name: 'Model', blurb: 'Shaded materials, follows light/dark', swatch: ['#2e3338', '#c99b6d', '#3d6f8f'] },
  { id: 'clay', name: 'Clay', blurb: 'White architectural massing model', swatch: ['#f3f0ea', '#d9d3c9', '#e07a5f'] },
  { id: 'blueprint', name: 'Blueprint', blurb: 'White linework on blue', swatch: ['#0f3b73', '#e8f1ff', '#2c64a8'] },
  { id: 'sketch', name: 'Sketch', blurb: 'Ink, hatching and watercolour', swatch: ['#f5efe2', '#3b342c', '#e6c49a'] },
  { id: 'neon', name: 'Neon', blurb: 'Glowing edges in the dark', swatch: ['#07080d', '#38e1ff', '#ff3ea5'] },
  { id: 'pastel', name: 'Pastel', blurb: 'Flat isometric illustration', swatch: ['#6c5ce7', '#ffd6dc', '#3ccf91'] },
];

export const DEFAULT_SKIN = 'model';

// Every token a skin must define. Colours feed the renderer through
// getComputedStyle; the numeric ones are read with parseFloat.
export const SKIN_TOKENS = [
  'bg-stage',
  'wall', 'wall-cap', 'plinth', 'glass', 'door', 'wood', 'metal', 'fabric', 'fabric-b',
  'cabinet', 'counter', 'steel', 'appliance', 'board', 'ceramic', 'pot', 'plant', 'plant-light',
  'zone-meet', 'zone-work', 'zone-social', 'zone-service', 'floor-wood', 'floor-tile', 'tile-line',
  'edge', 'edge-width', 'edge-opacity', 'floor-edge', 'glass-opacity',
  'shade-min', 'shade-max', 'hatch', 'corner', 'facets',
  'solids-filter', 'shadow-opacity', 'label-font', 'label-ink', 'label-halo', 'label-case', 'label-size',
];
