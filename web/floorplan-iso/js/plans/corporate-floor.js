// Corporate office floor test-fit traced by hand from the architect's drawing
// (2202×1426 px). The floor is an L: a slanted west facade, a north wing on the
// right, a central core of lifts, stairs and restrooms, and private offices on
// three sides of two open workstation fields.
//
// Calibration: the room labels give clear spans that disagree by axis. Measured
// vertically, Conference 32'-2" (430 px), Office 11'-10" x 14'-0" (188 px),
// Support Staff and Storage 12'-3" (165 px), TBD 17'-10" (239 px) and Server Room
// 7'-5" (100 px) all give 43.9–45.1 px/m. Measured horizontally, Support Staff
// 18'-11" (224 px), Storage 7'-1" (83 px), Server Room 9'-2" (108 px), both TBD
// rooms and Office 23 give 38.4–38.8 px/m, so the published JPEG was squeezed
// about 13% across. The plan uses separate scales, 38.6 px/m in x and 44 in y,
// so labelled spans come out right on both axes. Angles are taken in metres
// for the same reason: the squeeze changes them.
//
// Everything below is traced in source pixels and converted with fromPx, so a
// coordinate can be checked against the image directly.

import {
  THICKNESS as KIT_THICKNESS, fromPx, door, doubleDoor, win, opening, column, box, block, stairs,
  desk, chair, toilet, basin, tub, E, S, W, N,
} from './kit.js';

export const SOURCE = {
  title: 'Corporate floor',
  url: 'https://images.squarespace-cdn.com/content/v1/52f9dfd4e4b067fa9e2bb2cb/1394745812848-0EXC0B4JMUTENEE479E1/Picture3.jpg?format=2500w',
  credit: 'Corporate office test-fit (architect drawing)',
  sizePx: [2202, 1426],
  pxPerMetre: [38.6, 44],
  // The west facade's centreline at the north wall (y 390.5) and the wing's
  // north wall centreline (y 68.5).
  originPx: [40.6, 68.5],
};

const toM = fromPx(SOURCE);
const X = (px) => toM(px, 0)[0];
const Y = (py) => toM(0, py)[1];
const PT = (px, py) => toM(px, py);
const [SX, SY] = SOURCE.pxPerMetre;
const Lx = (px) => +(px / SX).toFixed(3);
const Ly = (px) => +(px / SY).toFixed(3);
// Length of a pixel segment, in metres.
const Lseg = (x0, y0, x1, y1) => +Math.hypot((x1 - x0) / SX, (y1 - y0) / SY).toFixed(3);

// Exterior wall centrelines, px. The west facade leans in: its centreline runs
// x = 0.2317·y − 49.85, fitted to the outer edge over the full height.
const NORTH_Y = 390.5;
const WING_Y = 68.5;
const SOUTH_Y = 1353;
const EAST_X = 2154;
const WING_X = 1313;
const westX = (py) => 0.2317 * py - 49.85;

export const WIDTH = X(EAST_X);
export const DEPTH = Y(SOUTH_Y);

export const OUTLINE = [
  PT(westX(NORTH_Y), NORTH_Y), PT(WING_X, NORTH_Y), PT(WING_X, WING_Y),
  PT(EAST_X, WING_Y), PT(EAST_X, SOUTH_Y), PT(westX(SOUTH_Y), SOUTH_Y),
];

// The lift lobby side of reception, where visitors arrive.
export const ENTRANCE = PT(960, 910);

// The drawn facade is 17 px (0.39 m) of masonry.
export const THICKNESS = { ...KIT_THICKNESS, exterior: 0.38 };
const EXT_HALF = THICKNESS.exterior / 2;

// A wall in source pixels. Openings are given in pixels along the wall's
// dominant axis and converted on the same axis.
function wall(id, kind, [x0, y0], [x1, y1], openings = [], extra = {}) {
  const c = Math.abs(x1 - x0) >= Math.abs(y1 - y0) ? X : Y;
  return {
    id, kind, from: PT(x0, y0), to: PT(x1, y1),
    openings: openings.map((o) => ({ ...o, a: c(o.a), b: c(o.b) })), ...extra,
  };
}
const inner = (id, a, b, openings, extra) => wall(id, 'interior', a, b, openings, extra);
const thin = (id, a, b, openings = []) => wall(id, 'interior', a, b, openings, { thickness: 0.05 });
const col = (id, px, py, sizePx) => column(id, X(px), Y(py), +((Lx(sizePx) + Ly(sizePx)) / 2).toFixed(3));
// A rectangular pier that isn't square, as one thick wall.
const pier = (id, x0, y0, x1, y1) => ({
  id, kind: 'interior', from: PT(x0, (y0 + y1) / 2), to: PT(x1, (y0 + y1) / 2), thickness: Ly(y1 - y0), openings: [],
});

// Glazing measured as the light runs along each facade centreline.
const glaze = (runs) => runs.map(([a, b]) => win(a, b, 0.8, 2.4));
const NORTH_GLAZING = glaze([
  [178, 229], [248, 300], [344, 382], [401, 442], [460, 502], [550, 604], [624, 676], [728, 775],
  [790, 835], [853, 899], [953, 991], [1009, 1050], [1069, 1110], [1133, 1184], [1201, 1252],
]);
const SOUTH_GLAZING = glaze([
  [298, 350], [413, 467], [518, 569], [623, 676], [728, 781], [832, 885], [953, 1007], [1074, 1122],
  [1177, 1231], [1286, 1337], [1402, 1457], [1523, 1576], [1628, 1681], [1735, 1786], [1839, 1892],
  [1947, 1998], [2062, 2112],
]);
const WEST_GLAZING = glaze([
  [424, 475], [550, 602], [664, 715], [779, 834], [894, 951], [1010, 1064], [1125, 1182], [1257, 1310],
]);

// A lift car as an enclosed shaft with its door gap on one side.
function lift(id, x0, y0, x1, y1, side) {
  const gap = (a, b) => [opening((a + b) / 2 - 20, (a + b) / 2 + 20)];
  return [
    thin(`${id}-n`, [x0, y0], [x1, y0], side === 'N' ? gap(x0, x1) : []),
    thin(`${id}-s`, [x0, y1], [x1, y1], side === 'S' ? gap(x0, x1) : []),
    thin(`${id}-w`, [x0, y0], [x0, y1], side === 'W' ? gap(y0, y1) : []),
    thin(`${id}-e`, [x1, y0], [x1, y1], side === 'E' ? gap(y0, y1) : []),
  ];
}

// South office partitions, west to east; 09 and 10 share one room.
const SOUTH_PARTITIONS = [473.5, 699.5, 809.5, 919.5, 1028.5, 1136.5, 1256, 1369, 1479.5, 1598.5, 1715.5, 1811.5, 1906.5, 2003.5];
const CORRIDOR_Y = 1154;

// Facade-side ends of the slanted partitions, where each line meets the west facade.
const W0607 = [96.6, 632.2];
const W07S = [123.5, 748.2];
const W08N = [226.5, 1192.8];

export const walls = [
  // ---- exterior
  wall('north', 'exterior', [westX(NORTH_Y), NORTH_Y], [WING_X, NORTH_Y], NORTH_GLAZING, { ext: EXT_HALF }),
  wall('wing-west', 'exterior', [WING_X, WING_Y], [WING_X, NORTH_Y], [], { ext: 0 }),
  wall('wing-north', 'exterior', [WING_X, WING_Y], [EAST_X, WING_Y], [], { ext: EXT_HALF }),
  wall('east', 'exterior', [EAST_X, WING_Y], [EAST_X, SOUTH_Y], [], { ext: 0 }),
  wall('south', 'exterior', [westX(SOUTH_Y), SOUTH_Y], [EAST_X, SOUTH_Y], SOUTH_GLAZING, { ext: EXT_HALF }),
  wall('west', 'exterior', [westX(NORTH_Y), NORTH_Y], [westX(SOUTH_Y), SOUTH_Y], WEST_GLAZING, { ext: 0 }),

  // ---- structural columns (centre, side in px)
  ...[
    [325, 412, 33], [531, 410, 32], [699, 412, 37], [919, 412, 31], [1133, 413, 26],
    [113, 626, 50], [159, 856, 38], [211, 1085, 36], [273, 1330, 40],
    [532, 608, 23], [915, 612, 30], [1134, 612, 24], [1308, 611, 22], [1540, 611, 22], [1741, 588, 21], [1932, 589, 23], [2135, 588, 34],
    [426, 850, 23], [701, 847, 24], [915, 849, 29], [1308, 848, 22], [1741, 835, 21], [1933, 834, 21], [2135, 835, 34],
    [915, 971, 29], [1741, 973, 21],
    [473, 1084, 20], [697, 1086, 23], [914, 1086, 26], [1133, 1086, 23], [1741, 1086, 21], [1933, 1086, 22], [2135, 1080, 34],
    [491, 1330, 42], [702, 1330, 42], [921, 1330, 42], [1150, 1330, 42], [1259, 1330, 42], [1489, 1330, 44], [1708, 1330, 44], [1920, 1330, 44],
    [1540, 88, 30], [1746, 88, 34], [1937, 88, 40], [2136, 84, 36], [1331, 158, 40],
    [1745, 335, 28], [1928, 335, 34], [1534, 338, 30], [2142, 334, 38],
  ].map(([x, y, s], i) => col(`col-${i + 1}`, x, y, s)),
  // Office 07's corner column is turned with the facade.
  { ...col('col-07', 374, 611, 34), from: PT(358, 616), to: PT(390, 606) },
  pier('pier-02', 662, 589, 743, 626),

  // ---- north offices 01–06 and the 33'-10" TBD room
  inner('o06-east', [313, 399], [313, 562], [door(515, 555, 'W', 'b')]),
  inner('o06-07', [314, 562], W0607),
  inner('o07-east-1', [278, 571], [282, 616]),
  inner('o07-east-2', [282, 616], [305, 614]),
  inner('o07-east-3', [305, 614], [308, 642]),
  inner('o07-door', [308, 642], [318, 688], [door(645, 684, 'W', 'b')]),
  inner('o07-south', [318, 688], W07S),
  inner('o04-05', [395.5, 399], [395.5, 621]),
  inner('o04-door', [395.5, 621], [443, 621], [door(403, 439, 'N', 'a')]),
  inner('o04-jamb', [443, 591], [443, 621]),
  inner('o04-south', [443, 591], [505.5, 591]),
  inner('o03-04', [505.5, 424], [505.5, 591]),
  inner('o03-04-head', [505.5, 424], [520, 424]),
  inner('o03-south', [505.5, 591], [690, 591], [door(552, 586, 'N', 'a')]),
  inner('o02-03', [690, 431], [690, 591]),
  inner('o02-south', [690, 591], [816, 591], [door(776, 812, 'N', 'b')]),
  inner('o01-02', [816, 420], [816, 623]),
  inner('o01-02-head', [816, 420], [842, 420]),
  inner('o01-south', [816, 623], [900, 623]),
  inner('o01-east', [928, 431], [928, 597], [door(550, 589, 'W', 'b')]),
  inner('tbd-south', [928, 535.5], [1446, 535.5], [doubleDoor(1095, 1165, 'N')]),
  inner('tbd-east', [1330, 312], [1330, 535.5]),

  // ---- north wing: closet, break, kitchen, closets, conference
  inner('closet-nw-south', [WING_X, 155], [1446, 155]),
  inner('closet-nw-east', [1446, WING_Y], [1446, 329], [door(81, 123, 'E', 'a')]),
  thin('shaft-nw-e', [1340, WING_Y], [1340, 140]),
  thin('shaft-nw-s', [WING_X, 140], [1340, 140]),
  inner('kitchen-jamb', [1446, 522], [1446, 535.5]),
  inner('closets-w-head', [1491, WING_Y], [1491, 126]),
  inner('closets-w-foot', [1491, 462], [1491, 540]),
  inner('closets-spine', [1520, WING_Y], [1520, 510]),
  inner('conf-south', [1491, 510], [1755, 510]),
  inner('closet-s-end', [1683, 540], [1703, 510]),
  inner('conf-east', [1733, WING_Y], [1733, 335]),
  inner('conf-door', [1755, 335], [1755, 510], [door(359, 401, 'W', 'a')]),

  // ---- north wing: TBD rooms, vestibule, server room
  inner('tbd1-south', [1745, 320], [1928, 320], [door(1768, 1805, 'S', 'a')]),
  inner('tbd1-2', [1939, WING_Y], [1939, 248]),
  inner('tbd2-south', [1939, 248], [2035, 248], [doubleDoor(1952, 2023, 'N')]),
  inner('vest-west', [1944, 248], [1944, 426]),
  inner('server-west', [2035, 248], [2035, 426], [door(322, 363, 'E', 'a')]),
  inner('server-north', [2035, 317], [EAST_X, 317]),
  inner('server-south', [1936, 426], [EAST_X, 426], [door(1968, 2003, 'N', 'b')]),

  // ---- core: restrooms
  inner('core-west', [990, 601], [990, 866], [door(606, 648, 'E', 'a'), door(815, 857, 'E', 'b')]),
  inner('core-north', [990, 601], [1447, 601], [door(1083, 1117, 'S', 'b'), door(1211, 1245, 'S', 'b')]),
  inner('wc-a-east', [1049, 601], [1049, 743]),
  inner('wc-a-south', [990, 718], [1049, 718]),
  inner('wc-divider', [1128, 601], [1128, 758]),
  inner('wc-back', [1049, 743], [1282, 743]),
  inner('wc-w-east', [1252, 664], [1252, 743]),
  inner('wc-w-alcove', [1252, 664], [1278, 664]),
  inner('wc-w-alcove-e', [1278, 601], [1278, 664]),
  thin('stall-m-front', [1049, 674], [1128, 674], [door(1053, 1083, 'N', 'a'), door(1090, 1124, 'N', 'b')]),
  thin('stall-m-1', [1086, 674], [1086, 743]),
  thin('stall-w-front', [1128, 674], [1252, 674], [door(1132, 1165, 'N', 'a'), door(1173, 1207, 'N', 'a'), door(1214, 1248, 'N', 'a')]),
  thin('stall-w-1', [1169, 674], [1169, 743]),
  thin('stall-w-2', [1211, 674], [1211, 743]),
  thin('shaft-w-east', [1038, 718], [1038, 773]),
  inner('janitor-north', [990, 773], [1072, 773]),
  inner('janitor-east', [1072, 773], [1072, 866]),
  inner('janitor-south', [990, 862], [1072, 862]),

  // ---- core: north stair, lift banks, east lobby
  inner('stair-n-west', [1282, 627], [1282, 755]),
  inner('stair-n-jog', [1447, 601], [1447, 627]),
  inner('stair-n-north', [1282, 627], [1545, 627], [door(1487, 1522, 'S', 'b')]),
  inner('stair-n-south', [1282, 755], [1545, 755]),
  inner('lifts-e-west', [1545, 589], [1545, 1097]),
  inner('core-ne-north', [1545, 589], [1729, 589]),
  inner('shaft-ne-south', [1545, 700], [1642, 700]),
  inner('wc-e-west', [1642, 589], [1642, 700]),
  inner('wc-e-south', [1642, 700], [1729, 700]),
  inner('lobby-e-east', [1729, 589], [1729, 903], [door(655, 693, 'W', 'b'), door(706, 748, 'W', 'a')]),
  inner('lobby-e-south', [1642, 903], [1729, 903]),
  ...[[1083, 1142], [1162, 1220], [1235, 1293], [1324, 1382], [1396, 1455], [1476, 1534]]
    .flatMap(([x0, x1], i) => lift(`lift-${i + 1}`, x0, 767, x1, 862, 'S')),
  ...[[717, 796], [809, 888], [901, 980], [996, 1077]]
    .flatMap(([y0, y1], i) => lift(`lift-${i + 7}`, 1553, y0, 1637, y1, 'E')),

  // ---- core: corridor doors, office 24, south and east stairs
  inner('core-corridor-door', [1100, 868], [1100, 950], [doubleDoor(872, 948, 'E')]),
  inner('o24-west', [993, 950], [993, 1097]),
  inner('core-south-top', [993, 950], [1476, 950], [door(1203, 1240, 'S', 'a')]),
  inner('o24-east', [1148, 950], [1148, 1097]),
  inner('core-south', [993, 1097], [1729, 1097], [door(1008, 1042, 'N', 'a')]),
  inner('stair-s-vest', [1198, 950], [1198, 1008]),
  inner('stair-s-vest-s', [1198, 1008], [1296, 1008]),
  inner('stair-s-west', [1296, 1008], [1296, 1097]),
  inner('stair-s-east', [1476, 950], [1476, 1097]),
  inner('shaft-s-north', [1476, 975], [1545, 975]),
  inner('stair-e-west', [1648, 903], [1648, 1097]),
  inner('stair-e-east', [1729, 903], [1729, 1097]),
  inner('stair-e-rail', [1688, 912], [1688, 1048], [], { thickness: 0.12 }),

  // ---- reception back wall, the 1-station enclosure and the stair top beside it
  inner('recep-back', [806, 780], [806, 1078]),
  inner('station-1-south', [806, 780], [925, 780]),
  inner('station-1-east', [925, 676], [925, 780]),
  inner('stair-top-north', [806, 1037], [925, 1037]),
  inner('stair-top-east', [925, 1037], [925, 1078]),
  inner('stair-top-south', [806, 1078], [925, 1078]),

  // ---- support staff and storage
  inner('support-north', [1832, 986], [EAST_X, 986]),
  inner('support-west', [1832, 986], [1832, CORRIDOR_Y], [door(1103, 1145, 'W', 'b')]),
  inner('support-storage', [2060, 986], [2060, CORRIDOR_Y], [door(1103, 1145, 'E', 'b')]),

  // ---- south offices 08–23
  inner('o08-north', W08N, [421, 1138]),
  inner('o08-jog', [421, 1138], [429, CORRIDOR_Y]),
  inner('corridor-south-w', [429, CORRIDOR_Y], [879, CORRIDOR_Y], [
    door(432, 466, 'S', 'b'), door(594, 630, 'S', 'b'), door(766, 800, 'S', 'b'), door(817, 853, 'S', 'a'),
  ]),
  inner('recess-12-w', [879, CORRIDOR_Y], [879, 1172]),
  inner('recess-12-s', [879, 1172], [919.5, 1172]),
  inner('corridor-south-e', [919.5, CORRIDOR_Y], [EAST_X, CORRIDOR_Y], [
    door(985, 1021, 'S', 'b'), door(1093, 1129, 'S', 'b'), door(1144, 1180, 'S', 'a'), door(1326, 1361, 'S', 'b'),
    door(1437, 1471, 'S', 'b'), door(1555, 1591, 'S', 'b'), door(1672, 1706, 'S', 'b'), door(1736, 1771, 'S', 'a'),
    door(1863, 1897, 'S', 'b'), door(1960, 1996, 'S', 'b'), door(2013, 2047, 'S', 'a'),
  ]),
  ...SOUTH_PARTITIONS.map((x, i) => inner(`south-part-${i + 1}`, [x, CORRIDOR_Y], [x, SOUTH_Y])),
];

// ---- rooms
const room = (id, name, zone, floor, pts, label) => ({
  id, name, zone, floor, polygon: pts.map(([x, y]) => PT(x, y)), ...(label ? { label: PT(...label) } : {}),
});
const rectPx = (x0, y0, x1, y1) => [[x0, y0], [x1, y0], [x1, y1], [x0, y1]];

const SOUTH_OFFICES = [
  ['09–10', 473.5, 699.5], ['11', 699.5, 809.5], ['12', 809.5, 919.5], ['13', 919.5, 1028.5], ['14', 1028.5, 1136.5],
  ['15', 1136.5, 1256], ['16', 1256, 1369], ['17', 1369, 1479.5], ['18', 1479.5, 1598.5], ['19', 1598.5, 1715.5],
  ['20', 1715.5, 1811.5], ['21', 1811.5, 1906.5], ['22', 1906.5, 2003.5], ['23', 2003.5, EAST_X],
];

export const rooms = [
  room('o06', 'Office 06', 'work', 'carpet', [[westX(NORTH_Y), NORTH_Y], [313, NORTH_Y], [313, 562], W0607], [210, 500]),
  room('o05', 'Office 05', 'work', 'carpet', [[313, NORTH_Y], [395.5, NORTH_Y], [395.5, 600], [350, 600], [313, 562]], [355, 540]),
  room('o04', 'Office 04', 'work', 'carpet', [[395.5, NORTH_Y], [505.5, NORTH_Y], [505.5, 591], [443, 591], [443, 621], [395.5, 621]], [450, 545]),
  room('o03', 'Office 03', 'work', 'carpet', rectPx(505.5, NORTH_Y, 690, 591), [627, 545]),
  room('o02', 'Office 02', 'work', 'carpet', rectPx(690, NORTH_Y, 816, 591), [743, 545]),
  room('o01', 'Office 01', 'work', 'carpet', rectPx(816, NORTH_Y, 928, 623), [872, 545]),
  room('o07', 'Office 07', 'work', 'carpet', [W0607, [278, 571], [282, 616], [305, 614], [308, 642], [318, 688], W07S], [240, 650]),
  room('tbd-north', 'TBD', 'work', 'carpet', rectPx(928, NORTH_Y, 1330, 535.5), [1130, 470]),
  room('corridor-north', 'Corridor', 'service', 'carpet', [
    [443, 623], [928, 623], [928, 535.5], [1491, 535.5], [1491, 555], [1683, 555], [1729, 589],
    [1545, 589], [1545, 627], [1447, 627], [1447, 601], [990, 601], [990, 660], [443, 660],
  ], [1200, 570]),
  room('ws-west', 'Workstations (west)', 'work', 'carpet', [
    W07S, [318, 688], [318, 660], [806, 660], [806, 1154], [429, 1154], [421, 1138], W08N,
  ], [600, 1120]),
  room('station-1', 'Workstation', 'work', 'carpet', rectPx(806, 676, 925, 780), [866, 700]),
  room('reception', 'Reception', 'social', 'wood', rectPx(806, 780, 990, 1154), [950, 1000]),

  // North wing
  room('closet-nw', 'Closet', 'service', 'wood', rectPx(1340, WING_Y, 1446, 155)),
  room('break', 'Break', 'social', 'tile', rectPx(WING_X, 155, 1446, 330), [1390, 250]),
  room('kitchen', 'Kitchen', 'social', 'tile', rectPx(1330, 330, 1446, 535.5), [1390, 385]),
  room('closet-passage', 'Closet passage', 'service', 'wood', rectPx(1446, 155, 1491, 535.5)),
  room('closets', 'Closets', 'service', 'wood', rectPx(1491, WING_Y, 1520, 510)),
  room('conference', 'Conference', 'meet', 'carpet',
    [[1520, WING_Y], [1733, WING_Y], [1733, 335], [1755, 335], [1755, 510], [1520, 510]], [1641, 110]),
  room('closet-s', 'Closet', 'service', 'wood', [[1491, 510], [1703, 510], [1683, 540], [1491, 540]]),
  room('tbd-1', 'TBD', 'work', 'carpet', rectPx(1733, WING_Y, 1939, 320), [1835, 110]),
  room('tbd-2', 'TBD', 'work', 'carpet', [[1939, WING_Y], [EAST_X, WING_Y], [EAST_X, 317], [2035, 317], [2035, 248], [1939, 248]], [2045, 110]),
  room('vestibule', 'Vestibule', 'service', 'carpet', rectPx(1944, 248, 2035, 426)),
  room('server', 'Server Room', 'service', 'tile', rectPx(2035, 317, EAST_X, 426), [2090, 345]),

  // Core
  room('wc-access', 'Restroom', 'service', 'tile', rectPx(990, 601, 1049, 718)),
  room('wc-men', 'Restrooms', 'service', 'tile', rectPx(1049, 601, 1128, 743)),
  room('wc-women', 'Restrooms', 'service', 'tile', [[1128, 601], [1278, 601], [1278, 664], [1252, 664], [1252, 743], [1128, 743]]),
  room('janitor', 'Janitor', 'service', 'tile', rectPx(990, 773, 1072, 862)),
  room('stair-n', 'Stair', 'service', 'tile', rectPx(1282, 627, 1545, 755)),
  room('lifts-w', 'Lifts', 'service', 'tile', rectPx(1078, 758, 1545, 866)),
  room('core-corridor', 'Core', 'service', 'tile', rectPx(990, 866, 1545, 950), [1300, 910]),
  room('lifts-e', 'Lifts', 'service', 'tile', rectPx(1545, 700, 1642, 1097)),
  room('shaft-ne', 'Core', 'service', 'tile', rectPx(1545, 589, 1642, 700)),
  room('wc-east', 'Restroom', 'service', 'tile', rectPx(1642, 589, 1729, 700)),
  room('lobby-e', 'Lift lobby', 'service', 'tile', rectPx(1642, 700, 1729, 903)),
  room('o24', 'Office 24', 'work', 'carpet', rectPx(993, 950, 1148, 1097), [1070, 1050]),
  room('core-store', 'Core', 'service', 'tile', [[1148, 950], [1198, 950], [1198, 1008], [1296, 1008], [1296, 1097], [1148, 1097]]),
  room('stair-s', 'Stair', 'service', 'tile', [[1198, 950], [1476, 950], [1476, 1097], [1296, 1097], [1296, 1008], [1198, 1008]]),
  room('shaft-s', 'Core', 'service', 'tile', rectPx(1476, 975, 1545, 1097)),
  room('stair-e', 'Stair', 'service', 'tile', rectPx(1648, 903, 1729, 1097)),

  // East field, support, south corridor and offices
  room('production', 'Production & Operations', 'work', 'carpet', [
    [1755, 320], [1944, 320], [1944, 426], [EAST_X, 426], [EAST_X, 986], [1832, 986], [1832, CORRIDOR_Y],
    [1729, CORRIDOR_Y], [1729, 589], [1683, 555], [1703, 510], [1755, 510],
  ], [1990, 890]),
  room('support', 'Support Staff', 'work', 'carpet', rectPx(1832, 986, 2060, CORRIDOR_Y), [1946, 1130]),
  room('storage', 'Storage', 'service', 'wood', rectPx(2060, 986, EAST_X, CORRIDOR_Y), [2100, 1020]),
  room('corridor-south', 'Corridor', 'service', 'carpet', rectPx(990, 1097, 1729, CORRIDOR_Y), [1400, 1125]),
  room('o08', 'Office 08', 'work', 'carpet',
    [W08N, [421, 1138], [429, CORRIDOR_Y], [473.5, CORRIDOR_Y], [473.5, SOUTH_Y], [westX(SOUTH_Y), SOUTH_Y]], [380, 1210]),
  // Office 12 loses its north-east corner to the corridor recess.
  ...SOUTH_OFFICES.map(([n, x0, x1]) => room(`o${n.slice(0, 2)}`, `Office ${n}`, 'work', 'carpet',
    n === '12'
      ? [[x0, CORRIDOR_Y], [879, CORRIDOR_Y], [879, 1172], [x1, 1172], [x1, SOUTH_Y], [x0, SOUTH_Y]]
      : rectPx(x0, CORRIDOR_Y, x1, SOUTH_Y),
    [(x0 + x1) / 2, 1210])),
];

export const floorPatches = [];

// ---- furniture
const pbox = (type, x0, y0, x1, y1, h, extra) => box(type, X(x0), Y(y0), X(x1), Y(y1), h, extra);
const pchair = (x, y, facing) => chair(X(x), Y(y), facing);
const pdesk = (x0, y0, x1, y1) => pbox('desk', x0, y0, x1, y1, 0.75);
const board = (x0, y0, x1, y1) => {
  const [cx, cy] = PT((x0 + x1) / 2, (y0 + y1) / 2);
  return x1 - x0 >= y1 - y0
    ? block(cx, cy, Lx(x1 - x0), 0.05, 1.2, 'board')
    : block(cx, cy, 0.05, Ly(y1 - y0), 1.2, 'board');
};
// A workstation: the chair where the drawing puts it, its worksurface 0.6 m
// ahead (centre to centre), long side across the facing direction.
function station(x, y, facing, len = 1.4) {
  const [cx, cy] = PT(x, y);
  const r = (facing * Math.PI) / 180;
  return [
    desk(+(cx + Math.cos(r) * 0.6).toFixed(3), +(cy + Math.sin(r) * 0.6).toFixed(3), len, 0.66, facing + 90),
    chair(cx, cy, facing),
  ];
}
// Toward a pixel point, for chairs turned at a table.
const at = (x, y, tx, ty) => (Math.atan2((ty - y) / SY, (tx - x) / SX) * 180) / Math.PI;

// The west cubicles lean with the facade (x = 0.2317·y in px), so their
// panels run off the grid by the facade's angle in metres, about 15°.
const SKEW = -Math.round((Math.atan((0.2317 * SY) / SX) * 180) / Math.PI);

const southOffices = [
  [479, 535, 518], [638, 697, 656], [702, 763, 745], [858, 917, 877], [922, 981, 965],
  [1033, 1090, 1074], [1196, 1253, 1213], [1259, 1317, 1301], [1372, 1430, 1414], [1482, 1541, 1525],
  [1601, 1660, 1645], [1749, 1808, 1766], [1814, 1865, 1855], [1910, 1968, 1953], [2086, 2142, 2102],
].flatMap(([x0, x1, cx]) => [pdesk(x0, 1233, x1, 1270), pchair(cx, 1295, N)]);

export const furniture = [
  // North offices
  pdesk(148, 419, 185, 494), pdesk(114, 419, 148, 435), pchair(128, 471, E), pchair(205, 430, at(205, 430, 166, 456)),
  desk(...PT(198, 563), 1.0, 0.6, -17),
  pchair(192, 541, 73), pchair(204, 586, 253), pchair(172, 571, -17), pchair(225, 556, 163),
  pdesk(316, 432, 353, 512), pchair(371, 455, W),
  pdesk(432, 473, 498, 512), pdesk(485, 432, 500, 473), pchair(450, 450, S), pchair(488, 532, at(488, 532, 465, 500)),
  pdesk(617, 473, 686, 513), pdesk(671, 434, 686, 473), pchair(636, 445, S),
  pdesk(695, 473, 765, 513), pdesk(695, 434, 713, 473), pchair(746, 447, S),
  pdesk(856, 473, 924, 513), pdesk(908, 434, 924, 473), pchair(875, 447, S),
  desk(...PT(192, 648), 0.75, 1.75, -15), pchair(166, 672, -15),
  pbox('cabinet', 446, 595, 519, 616, 1.1), pbox('cabinet', 586, 595, 658, 617, 1.1),

  // Office 08: desk, guest chair and a four-seat table
  desk(...PT(322, 1212), 0.75, 1.7, -15), pchair(283, 1236, -15), tub(X(363), Y(1172), at(363, 1172, 322, 1212)),
  pdesk(400, 1238, 462, 1278),
  pchair(413, 1226, S), pchair(448, 1226, S), pchair(413, 1290, N), pchair(448, 1290, N),

  // South offices 09–23, with guest chairs where drawn
  ...southOffices,
  ...[1618, 1789, 1833, 1927, 2124].map((x) => tub(X(x), Y(1180), S)),
  pbox('cabinet', 479, 1158, 587, 1176, 1.0), pbox('cabinet', 643, 1158, 754, 1172, 1.0),

  // Office 24
  pdesk(1073, 958, 1101, 1025), pchair(1115, 990, W),

  // Reception
  pbox('counter', 902, 863, 930, 956, 1.05), pchair(880, 909, E),
  pbox('cabinet', 808, 834, 844, 986, 1.0),
  pdesk(810, 752, 923, 778), pdesk(896, 680, 922, 752), pchair(838, 735, S),
  stairs(...PT(866, 1057), 0.75, 2.5, E),

  // West workstations: three spines of back-to-back worksurfaces
  ...[
    [407, 715, E], [486, 715, W], [486, 842, W], [486, 920, W],
    [586, 713, E], [667, 726, W], [767, 726, E],
    [586, 842, E], [667, 810, W], [767, 810, E],
    [586, 923, E], [667, 950, W], [767, 950, E],
    [586, 1044, E], [667, 1034, W], [767, 1034, E],
  ].flatMap(([x, y, f]) => station(x, y, f, 1.5)),
  board(447, 676, 447, 1000), board(628, 676, 628, 1095), board(776, 676, 776, 845), board(776, 990, 776, 1095),
  board(449, 676, 509, 676), board(565, 676, 660, 676), board(383, 780, 509, 780), board(565, 780, 657, 780),
  board(449, 885, 509, 885), board(565, 885, 806, 885), board(447, 990, 509, 990), board(565, 990, 657, 990),
  board(565, 1095, 657, 1095),

  // West workstations along the slanted facade
  ...[
    [304, 731, 90 + SKEW], [213, 790, 270 + SKEW],
    [318, 851, 90 + SKEW], [210, 886, 90 + SKEW], [345, 967, 270 + SKEW], [237, 1001, 270 + SKEW],
    [287, 1106, 90 + SKEW], [387, 1098, 90 + SKEW],
  ].flatMap(([x, y, f]) => station(x, y, f, 1.4)),
  ...[
    [253, 719, 273, 814], [226, 832, 347, 798], [192, 954, 363, 904], [263, 862, 292, 990],
    [353, 835, 384, 967], [279, 1059, 400, 1025], [329, 1048, 345, 1120],
  ].map(([x0, y0, x1, y1]) => {
    const [cx, cy] = PT((x0 + x1) / 2, (y0 + y1) / 2);
    return block(cx, cy, Lseg(x0, y0, x1, y1), 0.05, 1.2, 'board', { rot: at(x0, y0, x1, y1) });
  }),

  // Production & Operations: 16 stations in back-to-back rows plus the facade run
  pdesk(1800, 569, 1890, 600), pdesk(1890, 569, 1984, 600), pdesk(1984, 569, 2047, 600),
  pdesk(1800, 600, 1890, 638), pdesk(1890, 600, 1950, 638),
  pdesk(1800, 785, 1890, 822), pdesk(1890, 785, 1950, 822),
  pdesk(1800, 822, 1890, 854), pdesk(1890, 822, 1984, 854), pdesk(1984, 822, 2047, 854),
  pdesk(1800, 946, 1891, 978), pdesk(1891, 946, 2045, 978), pdesk(2045, 946, 2142, 978),
  pdesk(1984, 600, 2013, 711), pdesk(1984, 711, 2013, 822),
  pdesk(2047, 430, 2142, 463), pdesk(2116, 463, 2142, 569), pdesk(2116, 606, 2142, 818), pdesk(2116, 852, 2142, 946),
  pdesk(1799, 982, 1828, 1073),
  ...[
    [1837, 553, S], [1987, 553, S], [2100, 516, E],
    [1840, 655, N], [1916, 655, N], [1840, 768, S], [1916, 768, S],
    [2026, 635, W], [2026, 769, W], [2100, 638, E], [2100, 772, E],
    [1840, 870, N], [1987, 870, N], [1840, 930, S], [1987, 930, S], [2100, 899, E],
  ].map(([x, y, f]) => pchair(x, y, f)),
  board(1800, 600, 2047, 600), board(1800, 822, 2047, 822), board(1984, 600, 1984, 822),
  board(1800, 554, 1800, 654), board(1800, 778, 1800, 880),

  // Support Staff: a U of worksurfaces
  pdesk(1863, 988, 2030, 1008), pdesk(1836, 988, 1863, 1073), pdesk(2030, 988, 2057, 1073),
  pchair(1934, 1020, N), pchair(1876, 1039, W), pchair(2018, 1039, E),

  // Break and kitchen: counters either side of the 5'-0" aisle, closets on the passage
  pbox('counter', 1322, 177, 1358, 310, 0.9), pbox('counter', 1425, 158, 1443, 312, 0.9),
  pbox('counter', 1333, 330, 1358, 522, 0.9),
  pbox('storage', 1420, 330, 1444, 465, 2.1),
  pbox('storage', 1494, 126, 1517, 322, 2.1), pbox('storage', 1494, 350, 1517, 462, 2.1),
  pbox('storage', 1497, 513, 1683, 538, 2.1),

  // Conference: a 6 m table with seven a side and one at each end
  desk(...PT(1640.5, 291), Lx(61), Ly(270)),
  ...[176, 212, 248, 284, 320, 356, 392].flatMap((y) => [pchair(1600, y, E), pchair(1683, y, W)]),
  pchair(1641, 140, S), pchair(1641, 444, N),
  pbox('storage', 1523, 85, 1552, 320, 2.1), pbox('storage', 1523, 352, 1552, 505, 2.1),
  pbox('cabinet', 1731, 425, 1752, 498, 0.9),

  // Wing stair top, duct riser in TBD, server rack
  stairs(...PT(1866, 340), 0.8, Lx(92), W),
  block(...PT(1911, 285), Lx(70), Ly(66), 2.6, 'wall'),
  pbox('cabinet', 2083, 376, 2107, 417, 2.0),

  // Core stairs
  stairs(...PT(1432, 659), 1.3, Lx(100), W), stairs(...PT(1432, 730), 1.0, Lx(100), E),
  stairs(...PT(1387, 979), 1.05, Lx(85), W), stairs(...PT(1387, 1066), 1.15, Lx(85), E),
  stairs(...PT(1668, 1003), 0.8, Ly(80), S), stairs(...PT(1708, 1003), 0.8, Ly(80), N),

  // Restrooms
  toilet(X(1032), Y(700), W), basin(X(1034), Y(622), W),
  basin(X(1062), Y(620), S), toilet(X(1068), Y(722), N), toilet(X(1106), Y(722), N),
  toilet(X(1148), Y(722), N), toilet(X(1190), Y(722), N), toilet(X(1231), Y(722), N),
  basin(X(1264), Y(620), W), basin(X(1264), Y(647), W),
  toilet(X(1709), Y(610), S), basin(X(1666), Y(606), S),
];
