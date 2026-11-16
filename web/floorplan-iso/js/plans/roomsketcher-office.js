// Office floor plan traced by hand from the RoomSketcher sample plan
// (800×460 px JPEG, linked from the README). All coordinates are metres on wall
// centrelines: x runs left→right, y runs top→bottom, matching the source image,
// so the top-down view lines up with the original without any flip.
//
// Calibration: the dimension chains put tick marks at px x = 86.7 + 34.3·m and
// px y = 40 + 34.3·m. The bottom chain (4.08 + 6.56 + 4.18 + 2.30 + 2.22) lands on
// every interior wall it names to within one source pixel (≈3 cm), so the
// structural lines below use the labelled dimensions and the furniture uses
// measured pixel positions converted with the same scale.

import {
  THICKNESS, door, doubleDoor, win, rect, E, S, W, N, toward, desk, chair, tub, plant, box,
} from './kit.js';

export const SOURCE = {
  title: 'Office',
  url: 'https://wpmedia.roomsketcher.com/content/uploads/2021/12/09160214/RoomSketcher-Office-Floor-Plan-PID3529710-2D-bw-with-Labels.jpg',
  credit: 'RoomSketcher office floor plan sample',
  sizePx: [800, 460],
  pxPerMetre: 34.3,
  originPx: [86.7, 40],
};

export const WIDTH = 19.34; // 4.08 + 6.56 + 4.18 + 2.30 + 2.22
export const DEPTH = 10.65; // 3.32 + 3.47 + 3.86, and 3.17 + 3.61 + 2.75 + 1.11

// The exterior door on the west wall; the build animation grows outward from here.
export const ENTRANCE = [0, 7.4];

export { THICKNESS };

// The north facade is glazed almost wall to wall. The solid pieces between the
// windows are the mullions and partition junctions visible in the source.
const NORTH_GLAZING = [
  [0.1, 2.05], [2.15, 4.05], [4.15, 6.05], [6.25, 8.2], [8.3, 10.28],
  [10.42, 12.45], [12.55, 14.5], [15.1, 17.05], [17.3, 19.25],
].map(([a, b]) => win(a, b, 0.1, 2.5));

const EXT_HALF = THICKNESS.exterior / 2;

export const walls = [
  // Exterior. North and south run past the corners by half a thickness so the
  // corners close. West and east stop at the centreline and tuck inside them.
  { id: 'north', kind: 'exterior', from: [0, 0], to: [WIDTH, 0], ext: EXT_HALF, openings: NORTH_GLAZING },
  { id: 'south', kind: 'exterior', from: [0, DEPTH], to: [WIDTH, DEPTH], ext: EXT_HALF, openings: [win(12.38, 14.35, 0.9, 2.1)] },
  { id: 'west', kind: 'exterior', from: [0, 0], to: [0, DEPTH], ext: 0, openings: [door(6.9, 7.87, 'E', 'a')] },
  { id: 'east', kind: 'exterior', from: [WIDTH, 0], to: [WIDTH, DEPTH], ext: 0, openings: [] },

  // Meetings / Creative Space
  { id: 'meet-east', kind: 'interior', from: [6.15, 0], to: [6.15, 3.32] },
  { id: 'meet-glass', kind: 'glass', from: [0, 3.32], to: [4.07, 3.32] },
  { id: 'meet-south', kind: 'interior', from: [4.07, 3.32], to: [6.15, 3.32], openings: [door(4.69, 5.69, 'N', 'b')] },

  // Conference / Lab
  { id: 'conf-east', kind: 'interior', from: [4.07, 3.32], to: [4.07, 6.79], openings: [doubleDoor(4.19, 6.0, 'E')] },

  // Corridor wall with the 1.90 m double door
  { id: 'corridor', kind: 'interior', from: [0, 6.79], to: [14.82, 6.79], openings: [doubleDoor(5.45, 7.35, 'S')] },

  // East partition: Support double door, Retreat 1.40 m double door
  { id: 'partition', kind: 'interior', from: [14.82, 0], to: [14.82, DEPTH], openings: [doubleDoor(3.4, 5.0, 'W'), doubleDoor(6.88, 8.28, 'W')] },

  // Mike and Sarah
  { id: 'offices-south', kind: 'interior', from: [14.82, 3.17], to: [WIDTH, 3.17], openings: [door(16.02, 17.0, 'N', 'b'), door(17.35, 18.35, 'N', 'a')] },
  { id: 'offices-divider', kind: 'interior', from: [17.15, 0], to: [17.15, 3.17] },

  // Server, File
  { id: 'server-north', kind: 'interior', from: [16.31, 6.79], to: [WIDTH, 6.79] },
  { id: 'server-west', kind: 'interior', from: [16.31, 6.79], to: [16.31, 8.24] },
  { id: 'server-south', kind: 'interior', from: [16.31, 8.24], to: [18.03, 8.24], openings: [door(16.75, 17.5, 'N', 'a')] },
  { id: 'file-west', kind: 'interior', from: [18.03, 6.79], to: [18.03, 9.53], openings: [door(8.42, 9.22, 'E', 'b')] },

  // Washrooms
  { id: 'wc-north', kind: 'interior', from: [14.82, 9.53], to: [WIDTH, 9.53], openings: [door(16.31, 17.08, 'S', 'a'), door(17.22, 18.0, 'S', 'b')] },
  { id: 'wc-divider', kind: 'interior', from: [17.15, 9.53], to: [17.15, DEPTH] },
];


// zone drives the floor tint; floor drives the material (tile gets a grid).
export const rooms = [
  { id: 'meetings', name: 'Meetings / Creative Space', zone: 'meet', floor: 'carpet', polygon: rect(0, 0, 6.15, 3.32), label: [3.4, 1.75] },
  { id: 'conference', name: 'Conference / Lab', zone: 'meet', floor: 'carpet', polygon: rect(0, 3.32, 4.07, 6.79), label: [2.3, 6.35] },
  {
    id: 'open-office', name: 'Open office & Break', zone: 'work', floor: 'wood',
    polygon: [[6.15, 0], [14.82, 0], [14.82, 6.79], [4.07, 6.79], [4.07, 3.32], [6.15, 3.32]],
    label: [6.9, 5.1],
  },
  { id: 'mike', name: 'Office', zone: 'work', floor: 'carpet', polygon: rect(14.82, 0, 17.15, 3.17), label: [15.6, 2.3] },
  { id: 'sarah', name: 'Office', zone: 'work', floor: 'carpet', polygon: rect(17.15, 0, WIDTH, 3.17), label: [18.0, 2.5] },
  { id: 'support', name: 'Support', zone: 'work', floor: 'carpet', polygon: rect(14.82, 3.17, WIDTH, 6.79), label: [15.55, 6.3] },
  { id: 'coffee', name: 'Coffee, Kitchen & Stuff', zone: 'social', floor: 'wood', polygon: rect(0, 6.79, 10.64, DEPTH), label: [5.2, 7.75] },
  { id: 'retreat', name: 'Retreat', zone: 'social', floor: 'carpet', polygon: rect(10.64, 6.79, 14.82, DEPTH), label: [12.6, 8.25] },
  {
    id: 'lobby', name: 'Lobby', zone: 'service', floor: 'wood',
    polygon: [[14.82, 6.79], [16.31, 6.79], [16.31, 8.24], [18.03, 8.24], [18.03, 9.53], [14.82, 9.53]],
    label: [15.55, 8.9],
  },
  { id: 'server', name: 'Server', zone: 'service', floor: 'tile', polygon: rect(16.31, 6.79, 18.03, 8.24) },
  { id: 'file', name: 'File', zone: 'service', floor: 'tile', polygon: rect(18.03, 6.79, WIDTH, 9.53) },
  { id: 'womens', name: "Women's", zone: 'service', floor: 'tile', polygon: rect(14.82, 9.53, 17.15, DEPTH) },
  { id: 'mens', name: "Men's", zone: 'service', floor: 'tile', polygon: rect(17.15, 9.53, WIDTH, DEPTH) },
];

// Tiled floor patches that sit on top of a room's own floor.
export const floorPatches = [
  { id: 'kitchen-tiles', name: 'Kitchen', polygon: rect(5.0, 8.2, 9.0, DEPTH), floor: 'tile' },
];


function conferenceChairs() {
  // Three chairs on each long side of the angled table.
  const cx = 1.83, cy = 4.88, rot = 44, half = 0.6 + 0.34;
  const ux = Math.cos((rot * Math.PI) / 180), uy = Math.sin((rot * Math.PI) / 180);
  const nx = -uy, ny = ux;
  const out = [];
  for (const side of [-1, 1]) {
    for (const along of [-0.8, 0, 0.8]) {
      const x = cx + ux * along + nx * half * side;
      const y = cy + uy * along + ny * half * side;
      out.push(chair(x, y, toward(x, y, cx + ux * along, cy + uy * along)));
    }
  }
  return out;
}

function ericJohnDesks() {
  // Two desks pushed together as a 1.44 m square, turned 45°.
  const cx = 9.5, cy = 1.6, off = 0.36 * Math.SQRT1_2;
  return [
    desk(cx + off, cy - off, 1.44, 0.72, 45),
    desk(cx - off, cy + off, 1.44, 0.72, 45),
    chair(10.26, 0.93, toward(10.26, 0.93, 9.75, 1.35), 'Eric'),
    chair(8.84, 2.35, toward(8.84, 2.35, 9.25, 1.85), 'John'),
    { type: 'cabinet', x: 10.26, y: 2.33, w: 0.75, d: 0.5, rot: 45, h: 0.6 },
  ];
}

export const furniture = [
  // Meetings / Creative Space
  ...[[0.42, 0.43, 0], [1.02, 0.51, 0], [1.64, 0.43, -12], [2.35, 0.43, 0], [3.07, 0.43, 15], [3.76, 0.43, 0]]
    .map(([x, y, r]) => ({ type: 'armchair', x, y, facing: S + r })),
  plant(4.8, 0.42, 0.3), plant(5.6, 0.55, 0.42),
  { type: 'board', x: 0.16, y: 1.45, w: 0.05, d: 1.7, rot: 0, z0: 0.8, z1: 2.0 },
  box('counter', 0.12, 2.44, 1.39, 3.22, 0.9), box('counter', 1.43, 2.44, 2.74, 3.22, 0.9), box('counter', 2.78, 2.44, 4.0, 3.22, 0.9),

  // Break
  { type: 'roundTable', x: 7.06, y: 1.73, r: 0.52, h: 0.74 },
  tub(7.24, 0.73, toward(7.24, 0.73, 7.06, 1.73)),
  tub(7.93, 1.24, toward(7.93, 1.24, 7.06, 1.73)),
  tub(6.8, 2.86, toward(6.8, 2.86, 7.06, 1.73)),
  plant(6.58, 0.32, 0.28), plant(8.08, 0.51, 0.33), plant(6.4, 3.75, 0.3),

  // Open office
  ...ericJohnDesks(),
  desk(12.235, 1.075, 0.69, 1.49), desk(12.935, 1.075, 0.69, 1.49), desk(12.59, 2.165, 1.4, 0.69),
  chair(11.52, 1.22, E, 'Tina'), chair(13.58, 1.25, W, 'Liv'), chair(12.58, 2.88, N, 'Nicole'),
  desk(9.48, 4.595, 1.42, 0.73), desk(9.125, 5.685, 0.71, 1.45), desk(9.835, 5.685, 0.71, 1.45),
  chair(9.48, 3.86, S, 'Sebastian'), chair(8.4, 5.55, E), chair(10.53, 5.55, W, 'Thomas'),
  desk(12.59, 4.61, 1.4, 0.7), desk(12.27, 5.685, 0.66, 1.45), desk(12.945, 5.685, 0.69, 1.45),
  chair(12.63, 3.85, 65, 'Nathan'), chair(11.52, 5.55, E, 'Linda'), chair(13.63, 5.55, W, 'Kate'),
  box('cabinet', 14.27, 0.58, 14.74, 1.77, 1.2), box('cabinet', 14.27, 1.81, 14.74, 2.81, 1.2),
  plant(14.45, 0.3, 0.26),
  box('copier', 14.27, 5.6, 14.74, 6.2, 1.1),
  plant(8.15, 6.27, 0.4), plant(13.72, 6.3, 0.35),

  // Conference / Lab
  { type: 'desk', x: 1.83, y: 4.88, w: 2.45, d: 1.2, rot: 44 },
  ...conferenceChairs(),
  { type: 'board', x: 0.14, y: 5.2, w: 0.04, d: 1.6, rot: 0, z0: 1.0, z1: 2.3 },

  // Coffee, Kitchen & Stuff
  box('storage', 0.1, 8.18, 0.57, 9.36, 2.0), box('storage', 0.1, 9.4, 0.57, 10.56, 2.0),
  desk(1.9, 8.905, 0.84, 1.39), desk(2.73, 9.345, 0.82, 1.39),
  box('storage', 4.03, 8.18, 4.52, 9.36, 2.0), box('storage', 4.03, 9.4, 4.52, 10.56, 2.0),
  box('counter', 4.54, 8.18, 5.71, 8.7, 0.92), box('counter', 4.54, 8.72, 4.98, 10.56, 0.92),
  box('sink', 5.0, 8.72, 5.45, 9.3, 0.92),
  box('counter', 7.0, 8.75, 7.6, 10.55, 0.92),
  box('counter', 7.72, 8.75, 8.08, 10.53, 1.05),
  ...[9.1, 9.65, 10.2].map((y) => ({ type: 'stool', x: 8.3, y })),
  desk(9.9, 9.65, 0.58, 1.8),
  ...[9.1, 9.65, 10.2].flatMap((y) => [
    { type: 'chair', x: 9.4, y, facing: E, size: 0.4 },
    { type: 'chair', x: 10.4, y, facing: W, size: 0.4 },
  ]),

  // Retreat
  plant(10.3, 7.36, 0.55),
  { type: 'roundTable', x: 12.16, y: 7.21, r: 0.3, h: 0.45 },
  tub(11.43, 7.25, E), tub(12.89, 7.25, W),
  { type: 'roundTable', x: 12.01, y: 9.58, r: 0.3, h: 0.45 },
  ...[[11.54, 8.96], [11.54, 10.17], [12.56, 10.17]].map(([x, y]) => tub(x, y, toward(x, y, 12.01, 9.58))),
  { type: 'roundTable', x: 14.24, y: 9.47, r: 0.3, h: 0.45 },
  ...[[14.24, 8.78], [14.27, 10.22], [13.47, 9.84]].map(([x, y]) => tub(x, y, toward(x, y, 14.24, 9.47))),

  // Mike and Sarah
  box('desk', 16.42, 0.5, 17.08, 2.02), box('desk', 17.22, 0.5, 17.94, 2.02),
  chair(16.33, 1.18, 15, 'Mike'), chair(18.32, 1.27, W, 'Sarah'),
  plant(18.79, 0.51, 0.4), plant(15.33, 2.7, 0.33),
  tub(18.86, 2.59, 225),

  // Support
  ...[15.98, 17.09, 18.19].flatMap((x0, i) => {
    const x1 = [17.09, 18.19, 19.25][i];
    return [box('desk', x0, 4.37, x1, 4.99), box('desk', x0, 4.99, x1, 5.6)];
  }),
  chair(16.62, 3.99, S, 'Ryan'), chair(17.71, 3.99, S, 'Ned'), chair(18.75, 3.99, S, 'Cece'),
  chair(16.6, 5.94, N, 'Dana'), chair(17.66, 5.94, N, 'Pearl'), chair(18.72, 5.94, N, 'Frank'),
  { type: 'board', x: 18.13, y: 6.66, w: 1.66, d: 0.04, rot: 0, z0: 0.9, z1: 2.0 },

  // Washrooms
  { type: 'toilet', x: 15.44, y: 10.25, facing: N },
  { type: 'basin', x: 16.42, y: 10.35, facing: N },
  { type: 'basin', x: 18.14, y: 10.35, facing: N },
  { type: 'toilet', x: 19.0, y: 10.02, facing: W },
];
