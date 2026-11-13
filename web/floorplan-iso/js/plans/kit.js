// Shared vocabulary for traced plans. A plan module exports:
//
//   SOURCE     { url, credit, title, pxPerMetre, originPx } — where it came from and
//              how image pixels map to metres (used by tools/overlay.mjs)
//   WIDTH, DEPTH   bounding box of the wall centrelines, metres
//   OUTLINE    optional polygon of the exterior wall centrelines when the
//              building isn't a rectangle (L-shapes, slanted facades)
//   ENTRANCE   [x, y] the build animation grows outward from
//   THICKNESS  { exterior, interior, glass } metres
//   walls, rooms, floorPatches, furniture
//
// Coordinates are metres with the origin at the top-left of the bounding box,
// x to the right and y DOWN, like the source image, so the top-down view
// overlays the original without a flip.

export const THICKNESS = { exterior: 0.15, interior: 0.1, glass: 0.05 };

// Image pixel → metres for a plan's SOURCE calibration.
export const fromPx = (source) => (px, py) => [
  +((px - source.originPx[0]) / source.pxPerMetre).toFixed(3),
  +((py - source.originPx[1]) / source.pxPerMetre).toFixed(3),
];

export const FEET = 0.3048;
export const ft = (feet, inches = 0) => (feet + inches / 12) * FEET;

// ---- openings: a and b are absolute coordinates along the wall's dominant
// axis (x for a mostly horizontal wall, y for a mostly vertical one).
// swing: compass side the leaf opens to; hinge: 'a' = low end, 'b' = high end.
export const door = (a, b, swing, hinge = 'a') => ({ a, b, kind: 'door', swing, hinge });
export const doubleDoor = (a, b, swing) => ({ a, b, kind: 'double-door', swing });
export const win = (a, b, sill = 0.9, head = 2.1) => ({ a, b, kind: 'window', sill, head });
// A gap with no leaf, e.g. an archway or the mouth of an open alcove.
export const opening = (a, b) => ({ a, b, kind: 'opening' });

// A structural column, as a short thick wall so it follows the wall-height
// slider (a fixed-height block would poke through a cutaway).
export const column = (id, x, y, size = 0.45) => ({
  id, kind: 'interior', from: [x - size / 2, y], to: [x + size / 2, y], thickness: size, openings: [],
});

export const rect = (x0, y0, x1, y1) => [[x0, y0], [x1, y0], [x1, y1], [x0, y1]];

// ---- furniture. Facing is the bearing a seated person looks along, in degrees
// in plan space (y down): 0 = east, 90 = south, 180 = west, 270 = north.
export const E = 0;
export const S = 90;
export const W = 180;
export const N = 270;
export const toward = (x, y, tx, ty) => (Math.atan2(ty - y, tx - x) * 180) / Math.PI;

export const desk = (x, y, w, d, rot = 0) => ({ type: 'desk', x, y, w, d, rot });
export const chair = (x, y, facing, label) => ({ type: 'chair', x, y, facing, label });
export const tub = (x, y, facing) => ({ type: 'tub', x, y, facing });
export const plant = (x, y, r = 0.3) => ({ type: 'plant', x, y, r });
export const roundTable = (x, y, r, h = 0.74) => ({ type: 'roundTable', x, y, r, h });
export const sofa = (x, y, w, facing) => ({ type: 'sofa', x, y, w, facing });
export const toilet = (x, y, facing) => ({ type: 'toilet', x, y, facing });
export const basin = (x, y, facing) => ({ type: 'basin', x, y, facing });

// Axis-aligned box by corners: counter, cabinet, storage, copier, sink, desk.
export const box = (type, x0, y0, x1, y1, h, extra = {}) => ({
  type, x: (x0 + x1) / 2, y: (y0 + y1) / 2, w: x1 - x0, d: y1 - y0, rot: 0, h, ...extra,
});

// Anything else: a block of any material (see the palette in render.js:
// wall, wood, metal, fabric, fabricB, cabinet, counter, steel, appliance,
// board, ceramic, pot, plant, door). z0 lifts it off the floor.
export const block = (x, y, w, d, h, mat, { rot = 0, z0 = 0 } = {}) => ({ type: 'block', x, y, w, d, rot, h, z0, mat });

// A straight flight rising along `facing` (the direction you walk up).
export const stairs = (x, y, w, d, facing, rise = 1.5, steps = 9) => ({ type: 'stairs', x, y, w, d, facing, rise, steps });
