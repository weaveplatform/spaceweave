// New Urban Suites, third floor, traced by hand from the architect's sheet
// (2550×3300 px PNG). Coordinates are metres on wall centrelines, origin at the
// top-left of the exterior centreline box, x right and y down like the image.
//
// Calibration: the graphic scale bar (0'–16') runs from px x = 1680 to 2130.5,
// 450 px for 4.877 m = 92.27 px/m; its 4' ticks land every 112.6 px, so the bar
// is linear. Door gaps measure 0.85–0.93 m at that scale, which matches the usual
// 3'-0" leaf. The origin is the centre of the north wall band (px y 204–232) and
// of the west wall band (px x 412–438).
//
// The patio is open to the sky but sits inside OUTLINE, because the plan needs
// every room inside the footprint. That leaves the thick walls between the patio
// and the building with no outline side to face, so they run from their east or
// south end: the renderer then flips their outward normal onto the patio.
// Reversing a wall swaps which end a door's hinge 'a' names; see those walls.

import {
  THICKNESS, door, win, opening, column, rect, E, S, W, N, toward,
  desk, chair, plant, roundTable, toilet, basin, box, block, stairs,
} from './kit.js';

export const SOURCE = {
  title: 'Urban Suites',
  url: 'https://i.pinimg.com/originals/45/48/50/4548508ef40e615e64ad3b9b6535654b.png',
  credit: 'New Urban Suites, third floor (architect drawing)',
  sizePx: [2550, 3300],
  pxPerMetre: 92.27,
  originPx: [425, 218],
};

// The south-east block (Suites 302/301) has a deeper facade than the rest of the
// east side, and Stair A's south wall sits further out than the breakroom's.
const EAST = 19.13;
const EAST_LOW = 19.33;
const STEP_Y = 18.7;
const SOUTH = 28.08;
const SOUTH_STAIR = 28.33;
const STAIR_A_W = 9.6;

export const WIDTH = EAST_LOW;
export const DEPTH = SOUTH_STAIR;

export const OUTLINE = [
  [0, 0], [EAST, 0], [EAST, STEP_Y], [EAST_LOW, STEP_Y], [EAST_LOW, SOUTH_STAIR],
  [STAIR_A_W, SOUTH_STAIR], [STAIR_A_W, SOUTH], [0, SOUTH],
];

// Lobby 330, by the lift at the foot of the building.
export const ENTRANCE = [14.9, 25.0];

export { THICKNESS };

const EXT = 0.3;
// Both long facades repeat the same bay: 1.48 m of glass between 1.67 m piers.
const glazing = (spans) => spans.map(([a, b]) => win(a, b, 0.6, 2.4));

// Suite 302's west wall is a shallow arc; three chords stay within 3 cm of it.
const CURVE = [[13.4, 18.62], [13.53, 19.4], [13.68, 20.1], [14.36, 22.38]];

export const walls = [
  // ---- exterior
  { id: 'north', kind: 'exterior', from: [0, 0], to: [EAST, 0], thickness: EXT, ext: EXT / 2, openings: [] },
  {
    id: 'west', kind: 'exterior', from: [0, 0], to: [0, 22.25], thickness: EXT, ext: 0,
    openings: glazing([[0.66, 2.14], [3.81, 5.29], [6.96, 8.43], [10.11, 11.59], [13.25, 14.74], [16.41, 17.88], [19.56, 21.04]]),
  },
  {
    id: 'east', kind: 'exterior', from: [EAST, 0], to: [EAST, STEP_Y], thickness: EXT, ext: 0,
    openings: glazing([[3.81, 5.29], [6.96, 8.43], [10.11, 11.59], [13.25, 14.74], [16.41, 17.88]]),
  },
  {
    id: 'east-low', kind: 'exterior', from: [EAST_LOW, STEP_Y], to: [EAST_LOW, SOUTH_STAIR], thickness: 0.45, ext: 0,
    openings: glazing([[19.73, 21.16], [22.89, 24.3], [26.04, 27.45]]),
  },
  {
    id: 'south-stair', kind: 'exterior', from: [STAIR_A_W, SOUTH_STAIR], to: [EAST_LOW, SOUTH_STAIR], thickness: 0.4, ext: 0.22,
    openings: glazing([[10.44, 11.89], [13.74, 15.19], [17.04, 18.49]]),
  },
  {
    id: 'south-breakroom', kind: 'exterior', from: [2.855, SOUTH], to: [STAIR_A_W, SOUTH], thickness: EXT, ext: EXT / 2,
    openings: glazing([[3.88, 5.34], [6.98, 8.44]]),
  },
  // Patio-facing walls run backwards (see the header): hinge 'a' is the end
  // nearest `from`, i.e. the east end of patio-north and the south end of
  // breakroom-west.
  { id: 'patio-north', kind: 'exterior', from: [2.855, 22.25], to: [0, 22.25], thickness: EXT, ext: EXT / 2, openings: [door(1.25, 2.17, 'S', 'a')] },
  { id: 'breakroom-west', kind: 'exterior', from: [2.855, SOUTH], to: [2.855, 22.25], thickness: EXT, ext: 0, openings: [door(26.41, 27.33, 'W', 'a')] },
  // Patio 336 railing
  { id: 'patio-rail-west', kind: 'glass', from: [0, 22.25], to: [0, SOUTH] },
  { id: 'patio-rail-south', kind: 'glass', from: [0, SOUTH], to: [2.855, SOUTH] },
  column('patio-column', 0.32, 27.78, 0.7),
  column('patio-pier', 2.55, 27.78, 0.6),

  // ---- north strip
  {
    id: 'north-strip', kind: 'interior', from: [0, 3.05], to: [13.85, 3.05], thickness: 0.18,
    openings: [door(3.27, 4.14, 'N', 'b'), door(4.63, 5.48, 'N', 'a'), opening(8.79, 10.26)],
  },
  { id: 'suite308-east', kind: 'interior', from: [4.385, 0], to: [4.385, 3.05], thickness: 0.18 },
  { id: 'elect-east', kind: 'interior', from: [6.385, 0], to: [6.385, 3.05], thickness: 0.25 },
  { id: 'mens-east', kind: 'interior', from: [8.37, 0], to: [8.37, 3.05], thickness: 0.18, openings: [door(1.46, 2.33, 'E', 'a')] },
  { id: 'womens-west', kind: 'interior', from: [10.68, 0], to: [10.68, 3.05], thickness: 0.18, openings: [door(1.46, 2.33, 'W', 'a')] },
  { id: 'womens-east', kind: 'interior', from: [12.71, 0], to: [12.71, 3.05], thickness: 0.3 },
  { id: 'shaft-divider', kind: 'interior', from: [12.71, 1.56], to: [13.85, 1.56] },
  { id: 'stair-west', kind: 'interior', from: [13.85, 0], to: [13.85, 3.05], thickness: 0.25 },
  { id: 'stair-south', kind: 'interior', from: [13.85, 2.96], to: [EAST, 2.96], thickness: 0.35, openings: [door(14.19, 15.06, 'N', 'a')] },

  // ---- west suites, casual meeting room, workroom
  { id: 'suite309-310', kind: 'interior', from: [0, 6.125], to: [4.27, 6.125], thickness: 0.17, openings: [door(3.22, 4.07, 'S', 'b')] },
  { id: 'cmr-west', kind: 'interior', from: [4.27, 4.65], to: [4.27, 9.0], thickness: 0.17 },
  { id: 'cmr-north', kind: 'interior', from: [4.27, 4.65], to: [7.155, 4.65], thickness: 0.17 },
  { id: 'corridor-door', kind: 'interior', from: [7.155, 3.05], to: [7.155, 4.425], thickness: 0.17, openings: [door(3.22, 4.14, 'W', 'a')] },
  { id: 'workroom-west', kind: 'interior', from: [7.155, 4.425], to: [7.155, 9.0], thickness: 0.17 },
  { id: 'workroom-north', kind: 'interior', from: [7.155, 4.425], to: [11.9, 4.425], thickness: 0.17, openings: [win(8.13, 10.58, 0.9, 2.1)] },
  { id: 'workroom-east', kind: 'interior', from: [11.9, 4.425], to: [11.9, 9.0], thickness: 0.18 },
  {
    id: 'lounge-north', kind: 'interior', from: [0, 9.0], to: [11.9, 9.0], thickness: 0.17,
    openings: [door(6.11, 6.99, 'N', 'b'), opening(9.09, 9.96)],
  },
  column('workroom-column', 9.53, 6.55, 0.62),

  // ---- business club, copy, bathroom
  { id: 'club-east', kind: 'glass', from: [11.8, 9.0], to: [11.8, 15.0], openings: [door(11.02, 11.94, 'W', 'b')] },
  { id: 'lounge-south', kind: 'interior', from: [0, 15.01], to: [7.16, 15.01], thickness: 0.18 },
  { id: 'copy-north', kind: 'interior', from: [7.16, 15.365], to: [11.9, 15.365], thickness: 0.17 },
  { id: 'copy-west', kind: 'interior', from: [7.16, 15.01], to: [7.16, 20.18], thickness: 0.2, openings: [win(16.31, 17.18, 0.9, 2.1)] },
  {
    id: 'corridor-east-wall', kind: 'interior', from: [11.82, 15.0], to: [11.82, 20.18], thickness: 0.3,
    openings: [opening(16.35, 17.18), opening(18.77, 19.64)],
  },
  { id: 'copy-south', kind: 'interior', from: [7.16, 17.95], to: [9.31, 17.95], thickness: 0.25 },
  { id: 'shaft-north', kind: 'interior', from: [9.31, 17.365], to: [11.82, 17.365], thickness: 0.17 },
  { id: 'shaft-west', kind: 'interior', from: [9.31, 17.365], to: [9.31, 18.555], thickness: 0.2 },
  { id: 'shaft-south', kind: 'interior', from: [9.31, 18.555], to: [11.82, 18.555], thickness: 0.17 },
  { id: 'bath-east', kind: 'interior', from: [10.39, 18.555], to: [10.39, 20.18], thickness: 0.17, openings: [door(18.64, 19.8, 'W', 'a')] },
  { id: 'bath-south', kind: 'interior', from: [7.16, 20.18], to: [11.82, 20.18], thickness: 0.18 },
  column('copy-column', 9.51, 15.77, 0.5),

  // ---- suites 313, 314 and the breakroom
  { id: 'suite313-314', kind: 'interior', from: [0, 18.7], to: [5.68, 18.7], thickness: 0.18 },
  { id: 'suite313-door', kind: 'interior', from: [5.68, 18.3], to: [7.16, 18.3], thickness: 0.3, openings: [door(5.9, 6.76, 'S', 'a')] },
  { id: 'suite314-east', kind: 'interior', from: [5.68, 18.3], to: [5.68, 22.25], thickness: 0.18, openings: [door(19.42, 20.29, 'W', 'a')] },
  { id: 'breakroom-north', kind: 'interior', from: [2.855, 22.25], to: [5.68, 22.25], thickness: 0.3 },
  { id: 'breakroom-north-2', kind: 'interior', from: [5.68, 21.52], to: [8.72, 21.52], thickness: 0.17, openings: [door(5.77, 6.73, 'S', 'a')] },
  { id: 'breakroom-ne', kind: 'interior', from: [8.72, 21.52], to: [8.72, 22.24], thickness: 0.17 },
  { id: 'basin-alcove', kind: 'interior', from: [8.72, 22.24], to: [10.05, 22.24], thickness: 0.17 },

  // ---- storage, Stair A, lift
  { id: 'storage-west', kind: 'interior', from: [9.535, 22.24], to: [9.535, 23.6], thickness: 0.17 },
  { id: 'storage-nw', kind: 'interior', from: [10.05, 21.52], to: [10.05, 22.24], thickness: 0.17 },
  { id: 'storage-north', kind: 'interior', from: [10.05, 21.52], to: [11.335, 21.52], thickness: 0.17, openings: [door(10.13, 11.24, 'S', 'b')] },
  { id: 'storage-east', kind: 'interior', from: [11.335, 21.52], to: [11.335, 23.6], thickness: 0.2 },
  { id: 'stair-a-north', kind: 'interior', from: [STAIR_A_W, 23.6], to: [13.13, 23.6], thickness: 0.3, openings: [door(11.92, 12.9, 'S', 'b')] },
  { id: 'stair-a-west', kind: 'interior', from: [STAIR_A_W, 22.24], to: [STAIR_A_W, SOUTH_STAIR], thickness: 0.3 },
  { id: 'stair-a-east', kind: 'interior', from: [13.13, 23.6], to: [13.13, SOUTH_STAIR], thickness: 0.42 },
  { id: 'lift-north', kind: 'interior', from: [13.13, 26.2], to: [15.885, 26.2], thickness: 0.3, openings: [opening(14.4, 15.43)] },

  // ---- east suites
  { id: 'suite307-west', kind: 'interior', from: [15.45, 2.96], to: [15.45, 6.125], thickness: 0.17, openings: [door(3.24, 4.12, 'E', 'a')] },
  { id: 'suite307-306', kind: 'interior', from: [13.37, 6.125], to: [EAST, 6.125], thickness: 0.17 },
  { id: 'suite306-305', kind: 'interior', from: [13.37, 9.275], to: [EAST, 9.275], thickness: 0.17 },
  { id: 'suite305-304', kind: 'interior', from: [13.37, 12.41], to: [EAST, 12.41], thickness: 0.17 },
  {
    id: 'east-suites-west', kind: 'interior', from: [13.37, 6.125], to: [13.37, 18.62], thickness: 0.17,
    openings: [door(6.34, 7.21, 'E', 'a'), door(9.46, 10.34, 'E', 'a'), door(13.93, 14.77, 'E', 'b')],
  },
  { id: 'suite303-pier-w', kind: 'interior', from: [13.37, 15.16], to: [14.27, 15.16], thickness: 0.42 },
  { id: 'suite303-pier-e', kind: 'interior', from: [17.94, 15.16], to: [EAST, 15.16], thickness: 0.42 },
  { id: 'suite303-glass', kind: 'glass', from: [14.27, 15.365], to: [17.94, 15.365], openings: [opening(15.23, 16.15)] },
  { id: 'suite303-302', kind: 'interior', from: [13.37, 18.565], to: [EAST, 18.565], thickness: 0.17 },
  { id: 'lobby-door', kind: 'interior', from: [11.82, 18.7], to: [13.4, 18.7], thickness: 0.2, openings: [door(12.32, 13.18, 'S', 'b')] },
  ...CURVE.slice(1).map((to, i) => ({
    id: `suite302-curve-${i + 1}`, kind: 'interior', from: CURVE[i], to, thickness: 0.26,
    openings: i === 2 ? [door(21.0, 21.8, 'E', 'b')] : [],
  })),
  { id: 'suite302-301', kind: 'interior', from: [14.36, 22.38], to: [EAST_LOW, 22.38], thickness: 0.2, openings: [opening(14.5, 15.75)] },
  { id: 'suite301-west', kind: 'interior', from: [15.885, 22.38], to: [15.885, SOUTH_STAIR], thickness: 0.17, openings: [door(22.5, 23.4, 'E', 'a')] },
];

// Corridors are split where the drawing changes direction, so each piece is a
// simple polygon.
export const rooms = [
  // North strip
  { id: 'suite308', name: 'Suite 308', zone: 'work', floor: 'carpet', polygon: rect(0, 0, 4.385, 3.05) },
  { id: 'elect', name: 'Elect', zone: 'service', floor: 'tile', polygon: rect(4.385, 0, 6.385, 3.05) },
  { id: 'mens', name: "Men's", zone: 'service', floor: 'tile', polygon: rect(6.385, 0, 8.37, 3.05) },
  { id: 'vest', name: 'Vest.', zone: 'service', floor: 'tile', polygon: rect(8.37, 0, 10.68, 3.05) },
  { id: 'womens', name: "Women's", zone: 'service', floor: 'tile', polygon: rect(10.68, 0, 12.71, 3.05) },
  { id: 'shaft-1', name: 'Shaft', zone: 'service', floor: 'tile', polygon: rect(12.71, 0, 13.85, 1.56) },
  { id: 'shaft-2', name: 'Shaft', zone: 'service', floor: 'tile', polygon: rect(12.71, 1.56, 13.85, 3.05) },
  { id: 'stair', name: 'Stair', zone: 'service', floor: 'tile', polygon: rect(13.85, 0, EAST, 2.96) },

  // West
  { id: 'suite309', name: 'Suite 309', zone: 'work', floor: 'carpet', polygon: rect(0, 3.05, 3.22, 6.125) },
  { id: 'suite310', name: 'Suite 310', zone: 'work', floor: 'carpet', polygon: rect(0, 6.125, 4.27, 9.0) },
  { id: 'casual', name: 'Casual Meeting Room', zone: 'meet', floor: 'carpet', polygon: rect(4.27, 4.65, 7.155, 9.0) },
  { id: 'workroom', name: 'Workroom', zone: 'work', floor: 'tile', polygon: rect(7.155, 4.425, 11.9, 9.0) },
  { id: 'lounge', name: 'Lounge', zone: 'social', floor: 'wood', polygon: rect(0, 9.0, 7.155, 15.01) },
  { id: 'club', name: 'Business Club', zone: 'meet', floor: 'carpet', polygon: rect(7.155, 9.0, 11.8, 15.365) },
  {
    id: 'suite313', name: 'Suite 313', zone: 'work', floor: 'carpet',
    polygon: [[0, 15.01], [7.16, 15.01], [7.16, 18.3], [5.68, 18.3], [5.68, 18.7], [0, 18.7]],
  },
  { id: 'suite314', name: 'Suite 314', zone: 'work', floor: 'carpet', polygon: rect(0, 18.7, 5.68, 22.25) },
  { id: 'patio', name: 'Patio', zone: 'social', floor: 'wood', polygon: rect(0, 22.25, 2.855, SOUTH) },
  {
    id: 'breakroom', name: 'Breakroom', zone: 'social', floor: 'tile',
    polygon: [[2.855, 22.25], [5.68, 22.25], [5.68, 21.52], [8.72, 21.52], [8.72, 22.24], [STAIR_A_W, 22.24], [STAIR_A_W, SOUTH], [2.855, SOUTH]],
    label: [5.5, 25.0],
  },

  // Centre
  {
    id: 'copy', name: 'Copy', zone: 'work', floor: 'tile',
    polygon: [[7.16, 15.365], [11.82, 15.365], [11.82, 17.365], [9.31, 17.365], [9.31, 17.95], [7.16, 17.95]],
  },
  { id: 'shaft-3', name: 'Shaft', zone: 'service', floor: 'tile', polygon: rect(9.31, 17.365, 11.82, 18.555) },
  {
    id: 'bathroom', name: 'Bathroom', zone: 'service', floor: 'tile',
    polygon: [[7.16, 17.95], [9.31, 17.95], [9.31, 18.555], [10.39, 18.555], [10.39, 20.18], [7.16, 20.18]],
  },
  {
    id: 'storage', name: 'Storage', zone: 'service', floor: 'tile',
    polygon: [[10.05, 21.52], [11.335, 21.52], [11.335, 23.6], [9.535, 23.6], [9.535, 22.24], [10.05, 22.24]],
  },
  { id: 'stair-a', name: 'Stair A', zone: 'service', floor: 'tile', polygon: rect(STAIR_A_W, 23.6, 13.13, SOUTH_STAIR) },
  { id: 'lift', name: 'Lift', zone: 'service', floor: 'tile', polygon: rect(13.13, 26.2, 15.885, SOUTH_STAIR) },

  // East
  { id: 'suite307', name: 'Suite 307', zone: 'work', floor: 'carpet', polygon: rect(15.45, 2.96, EAST, 6.125) },
  { id: 'suite306', name: 'Suite 306', zone: 'work', floor: 'carpet', polygon: rect(13.37, 6.125, EAST, 9.275) },
  { id: 'suite305', name: 'Suite 305', zone: 'work', floor: 'carpet', polygon: rect(13.37, 9.275, EAST, 12.41) },
  { id: 'suite304', name: 'Suite 304', zone: 'work', floor: 'carpet', polygon: rect(13.37, 12.41, EAST, 15.365) },
  { id: 'suite303', name: 'Suite 303', zone: 'work', floor: 'carpet', polygon: rect(13.37, 15.365, EAST, 18.565) },
  {
    id: 'suite302', name: 'Suite 302', zone: 'work', floor: 'carpet',
    polygon: [...CURVE, [EAST_LOW, 22.38], [EAST_LOW, STEP_Y], [EAST, STEP_Y], [EAST, 18.565], [13.4, 18.565]],
    label: [16.4, 20.4],
  },
  { id: 'suite301', name: 'Suite 301', zone: 'work', floor: 'carpet', polygon: rect(15.885, 22.38, EAST_LOW, SOUTH_STAIR) },
  {
    id: 'lobby', name: 'Lobby', zone: 'social', floor: 'tile',
    polygon: [[11.82, 18.7], ...CURVE, [15.885, 22.38], [15.885, 26.2], [13.13, 26.2], [13.13, 23.6], [11.335, 23.6], [11.335, 20.18], [11.82, 20.18]],
    label: [14.5, 24.2],
  },

  // Corridors
  {
    id: 'corridor-north', name: 'Corridor', zone: 'service', floor: 'carpet',
    polygon: [[3.22, 3.05], [15.45, 3.05], [15.45, 6.125], [13.37, 6.125], [13.37, 4.425], [7.155, 4.425], [7.155, 4.65], [4.27, 4.65], [4.27, 6.125], [3.22, 6.125]],
    label: [9.5, 3.75],
  },
  { id: 'corridor-east', name: 'Corridor', zone: 'service', floor: 'carpet', polygon: rect(11.9, 4.425, 13.37, 18.7), label: [12.63, 12.0] },
  {
    id: 'corridor-south', name: 'Corridor', zone: 'service', floor: 'carpet',
    polygon: [[5.68, 18.3], [7.16, 18.3], [7.16, 20.18], [11.335, 20.18], [11.335, 21.52], [10.05, 21.52], [10.05, 22.24], [8.72, 22.24], [8.72, 21.52], [5.68, 21.52]],
    label: [8.3, 20.85],
  },
  {
    id: 'bath-vestibule', name: 'Corridor', zone: 'service', floor: 'tile', polygon: rect(10.39, 18.555, 11.82, 20.18),
  },
];

export const floorPatches = [];

const armchair = (x, y, facing) => ({ type: 'armchair', x, y, facing });

// The reception counter is a quarter ring (outer r 1.68 m, 0.6 m deep) whose
// centre sits on the lounge's south wall; three chords stand in for the arc.
function receptionDesk() {
  const [cx, cy, r] = [5.33, 14.95, 1.38];
  return [-75, -45, -15].map((deg) => {
    const a = (deg * Math.PI) / 180;
    return block(cx + r * Math.cos(a), cy + r * Math.sin(a), 0.74, 0.6, 1.05, 'counter', { rot: deg + 90 });
  });
}

// Business Club: four lounge chairs on an arc with side tables between them.
const CLUB_FOCUS = [9.4, 14.6];
const clubChair = (x, y) => armchair(x, y, toward(x, y, ...CLUB_FOCUS));

export const furniture = [
  // Suite 308
  box('cabinet', 0.16, 0.16, 1.95, 0.89, 0.75),
  box('desk', 1.95, 0.16, 2.66, 2.4, 0.75),
  box('cabinet', 0.16, 1.59, 0.65, 2.4, 0.75),
  armchair(1.55, 1.48, E), chair(3.74, 1.54, W),

  // Men's 339, Women's 338
  toilet(6.9, 0.62, E), basin(6.67, 1.86, E),
  toilet(11.71, 0.6, W), basin(12.35, 1.89, W),

  // Stair: the flight by the corridor climbs east to the landing, the other returns west.
  stairs(16.64, 2.24, 1.08, 2.8, E), stairs(16.64, 0.69, 1.02, 2.8, W),

  // Suite 309 (drawn greyed out: an open alcove off the corridor)
  box('cabinet', 0.16, 3.27, 0.9, 5.88, 0.5),
  box('cabinet', 0.94, 3.17, 2.46, 3.92, 0.75),
  box('desk', 0.94, 5.23, 2.46, 5.98, 0.75),
  armchair(1.73, 4.95, S),

  // Suite 310
  box('desk', 1.79, 6.63, 2.55, 8.15, 0.75),
  box('cabinet', 0.16, 6.63, 0.65, 8.15, 0.75),
  box('cabinet', 0.16, 8.15, 2.55, 8.9, 0.75),
  armchair(1.25, 7.39, E), chair(3.16, 7.2, W),

  // Casual Meeting Room
  box('cabinet', 4.44, 4.77, 6.94, 5.17, 0.75),
  armchair(4.71, 6.25, E), armchair(6.66, 6.25, W), armchair(4.71, 7.39, E), armchair(6.66, 7.39, W),
  box('cabinet', 4.39, 6.63, 5.04, 6.96, 0.5), box('cabinet', 6.34, 6.63, 6.99, 6.96, 0.5),

  // Workroom: counters on both long walls, two low screens on the west run.
  box('counter', 7.24, 4.52, 7.86, 8.92, 0.92),
  box('counter', 10.93, 4.52, 11.8, 8.92, 0.92),
  block(7.98, 6.08, 1.5, 0.05, 1.2, 'board'), block(7.98, 7.48, 1.5, 0.05, 1.2, 'board'),
  chair(8.08, 5.19, W), chair(8.08, 6.71, W), chair(8.08, 8.22, W),
  chair(10.73, 5.35, E), chair(10.75, 6.71, E), chair(10.75, 8.0, E),

  // Lounge: bar along the windows, dining table, café tables, plants, armchairs.
  box('counter', 0.27, 9.67, 1.05, 14.44, 1.05),
  ...[10.16, 11.4, 12.54, 13.75].map((y) => chair(1.25, y, W)),
  desk(3.1, 12.0, 1.06, 1.73),
  chair(3.09, 10.92, S), chair(3.09, 13.08, N),
  chair(2.33, 11.56, E), chair(2.33, 12.32, E), chair(3.85, 11.56, W), chair(3.85, 12.32, W),
  desk(3.09, 9.45, 0.76, 0.76), chair(2.44, 9.45, E), chair(3.68, 9.45, W),
  desk(3.09, 14.49, 0.76, 0.76), chair(2.44, 14.49, E), chair(3.68, 14.49, W),
  box('cabinet', 4.82, 10.99, 5.31, 11.92, 0.45),
  plant(5.72, 10.64), plant(5.72, 11.46), plant(5.72, 12.27),
  armchair(6.6, 10.59, S), box('cabinet', 6.38, 11.16, 6.84, 11.78, 0.5), armchair(6.6, 12.32, N),
  ...receptionDesk(),
  chair(5.77, 14.44, -49),

  // Business Club
  desk(8.18, 9.58, 0.76, 1.03), chair(7.64, 9.6, E), chair(8.85, 9.6, W),
  desk(10.89, 9.58, 0.76, 1.03), chair(10.4, 9.6, E), chair(11.33, 9.96, W),
  clubChair(7.69, 13.11), clubChair(8.72, 12.58), clubChair(9.92, 12.55), clubChair(11.09, 13.11),
  roundTable(8.13, 12.71, 0.22, 0.5), roundTable(10.62, 12.76, 0.22, 0.5),
  box('cabinet', 8.18, 15.03, 10.84, 15.27, 1.0),

  // Suite 313
  box('cabinet', 0.27, 15.1, 5.2, 15.85, 0.75),
  box('desk', 1.95, 15.85, 2.71, 17.36, 0.75), box('desk', 2.8, 15.85, 3.58, 17.36, 0.75),
  armchair(1.46, 16.58, E), armchair(4.06, 16.58, W),
  roundTable(6.39, 15.73, 0.54),
  chair(5.56, 15.58, toward(5.56, 15.58, 6.39, 15.73)), chair(6.5, 16.55, toward(6.5, 16.55, 6.39, 15.73)),

  // Copy
  box('counter', 7.3, 15.45, 9.25, 16.0, 0.92),
  box('copier', 9.86, 15.47, 10.35, 16.22, 1.1),
  box('counter', 10.35, 15.45, 11.65, 16.0, 0.92),
  box('counter', 7.3, 17.2, 9.2, 17.82, 0.92),

  // Bathroom, and the two hand basins in the alcove outside the breakroom
  toilet(7.66, 18.42, S), basin(8.81, 18.23, S),
  basin(9.05, 22.0, N), basin(9.54, 22.0, N),

  // Suite 314
  box('cabinet', 0.16, 19.0, 0.85, 22.05, 0.5),
  box('cabinet', 1.14, 18.82, 2.76, 19.1, 0.75),
  box('desk', 1.14, 19.1, 2.76, 19.53, 0.75), box('desk', 2.76, 18.82, 3.52, 21.05, 0.75),
  armchair(2.28, 20.29, E),
  chair(4.5, 21.61, toward(4.5, 21.61, 3.2, 20.5)),
  box('cabinet', 4.99, 21.05, 5.55, 22.08, 0.75),

  // Breakroom 335
  box('counter', 6.12, 23.49, 7.37, 26.85, 0.92),
  box('sink', 6.85, 24.2, 7.3, 24.75, 0.93), box('sink', 6.85, 25.57, 7.3, 26.12, 0.93),
  box('counter', 7.26, 21.62, 8.56, 22.2, 0.92),
  box('counter', 8.6, 22.35, 9.43, 23.75, 0.92),
  box('storage', 8.6, 23.8, 9.43, 26.6, 2.1),
  box('counter', 8.6, 26.65, 9.43, 27.9, 0.92),

  // Stair A: up the east flight southwards, then back north on the west flight.
  stairs(12.35, 26.0, 1.1, 2.0, S), stairs(10.3, 26.0, 1.1, 2.0, N),

  // Suite 307, and the open desk in the corridor beside it
  box('desk', 16.84, 3.76, 17.59, 5.28, 0.75),
  box('cabinet', 18.46, 3.76, 18.97, 5.28, 0.75),
  box('cabinet', 16.53, 5.28, 18.97, 6.0, 0.75),
  armchair(17.99, 4.57, W),
  box('desk', 13.42, 4.23, 14.06, 6.0, 0.75), box('desk', 14.06, 5.44, 15.28, 6.0, 0.75),
  chair(14.48, 4.93, W),

  // Suites 306–304 share one layout: desk, wall credenza, and a return.
  ...[[6.93, 8.45, 9.18], [10.1, 11.56, 12.32]].flatMap(([y0, y1, y2]) => [
    box('desk', 16.55, y0, 17.31, y1, 0.75),
    box('cabinet', 18.43, y0, 18.93, y1, 0.75),
    box('cabinet', 16.55, y1, 18.93, y2, 0.75),
  ]),
  chair(15.25, 7.67, E), armchair(17.83, 7.65, W),
  chair(15.88, 11.18, E), armchair(17.83, 10.84, W),
  box('cabinet', 16.58, 12.5, 18.93, 13.24, 0.75),
  box('desk', 16.58, 13.24, 17.34, 14.87, 0.75),
  box('cabinet', 18.46, 13.24, 18.93, 14.6, 0.75),
  chair(15.88, 13.86, E), armchair(17.84, 14.0, W),

  // Suite 303
  box('desk', 15.82, 16.22, 16.58, 17.74, 0.75),
  box('desk', 14.6, 17.74, 17.8, 18.05, 0.75),
  box('cabinet', 14.6, 18.05, 17.8, 18.45, 0.75),
  armchair(15.33, 16.98, E), armchair(17.12, 16.98, W),

  // Suite 302
  box('cabinet', 16.53, 18.66, 18.91, 19.37, 0.75),
  box('desk', 16.58, 19.37, 17.34, 20.89, 0.75),
  box('cabinet', 18.48, 19.37, 18.95, 20.89, 0.75),
  armchair(17.88, 20.13, W), chair(15.93, 19.88, E),

  // Suite 301
  box('desk', 16.83, 25.65, 18.37, 26.41, 0.75),
  box('cabinet', 18.37, 25.65, 19.07, 28.05, 0.75),
  armchair(17.58, 26.92, N), chair(17.85, 25.0, S),
];
