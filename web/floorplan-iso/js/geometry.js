// Turns plan data into vertical prisms (a convex footprint extruded from z0 to
// z1), and prisms into screen faces drawn back to front.
//
// SVG has no depth buffer, so draw order is everything. Sorting prisms by a
// single depth number fails whenever a long wall sits next to a small chair: the
// wall's centre can be nearer than the chair while its far end is behind it.
// Instead we order each pair whose screen boxes overlap by a separating axis of
// their footprints, then topologically sort. That is exact for disjoint convex
// prisms standing on the floor, which is nearly everything here.

const DEG = Math.PI / 180;

export const DOOR_HEAD = 2.1;
export const LEAF_HEIGHT = 2.05;
export const LEAF_THICKNESS = 0.04;

// ---------------------------------------------------------------- polygons

export function signedArea(poly) {
  let a = 0;
  for (let i = 0; i < poly.length; i++) {
    const [x0, y0] = poly[i];
    const [x1, y1] = poly[(i + 1) % poly.length];
    a += x0 * y1 - x1 * y0;
  }
  return a / 2;
}

export function polygonArea(poly) {
  return Math.abs(signedArea(poly));
}

// Positive signed area, so the outward normal of edge p→q is (dy, −dx).
export function ensurePositive(poly) {
  return signedArea(poly) < 0 ? [...poly].reverse() : poly;
}

export function centroid(poly) {
  let x = 0;
  let y = 0;
  for (const p of poly) {
    x += p[0];
    y += p[1];
  }
  return [x / poly.length, y / poly.length];
}

// w runs along the rotated x axis, d along the rotated y axis.
export function rectPoly(cx, cy, w, d, rotDeg = 0) {
  const c = Math.cos(rotDeg * DEG);
  const s = Math.sin(rotDeg * DEG);
  const hw = w / 2;
  const hd = d / 2;
  return [[-hw, -hd], [hw, -hd], [hw, hd], [-hw, hd]].map(([u, v]) => [cx + u * c - v * s, cy + u * s + v * c]);
}

// Convex, so it still works with the separating-axis ordering. Each corner is
// cut into `segments` straight pieces. The default single chamfer reads as soft
// at plan scale; true arcs multiply the faces drawn (and the frame time) for
// slivers a few pixels wide.
export function roundedRectPoly(cx, cy, w, d, rotDeg = 0, r = 0, segments = 1) {
  if (r <= 1e-6) return rectPoly(cx, cy, w, d, rotDeg);
  const c = Math.cos(rotDeg * DEG);
  const s = Math.sin(rotDeg * DEG);
  const hw = w / 2 - r;
  const hd = d / 2 - r;
  const corners = [[hw, hd, 0], [-hw, hd, 90], [-hw, -hd, 180], [hw, -hd, 270]];
  const out = [];
  for (const [u0, v0, a0] of corners) {
    for (let i = 0; i <= segments; i++) {
      const a = (a0 + (90 * i) / segments) * DEG;
      const u = u0 + r * Math.cos(a);
      const v = v0 + r * Math.sin(a);
      out.push([cx + u * c - v * s, cy + u * s + v * c]);
    }
  }
  return out;
}

export function ngonPoly(cx, cy, r, n = 12, rotDeg = 0) {
  const out = [];
  for (let i = 0; i < n; i++) {
    const a = rotDeg * DEG + (i / n) * 2 * Math.PI;
    out.push([cx + r * Math.cos(a), cy + r * Math.sin(a)]);
  }
  return out;
}

// The separating-axis sort tests every edge of both footprints against every
// vertex, so a 24-sided tub against a 16-sided seat costs ~40× a box pair.
// Ordering only needs the rough shape: sort on at most 8 evenly spaced
// vertices (inscribed, still convex) and draw the full outline.
const SORT_VERTICES = 8;

export function simplify(poly, max = SORT_VERTICES) {
  if (poly.length <= max) return poly;
  return Array.from({ length: max }, (_, i) => poly[Math.floor((i * poly.length) / max)]);
}

export function prism(poly, z0, z1, props = {}) {
  const p = ensurePositive(poly);
  return { poly: p, sortPoly: simplify(p), z0, z1, ...props };
}

// ---------------------------------------------------------------- walls

const COMPASS = { N: [0, -1], S: [0, 1], E: [1, 0], W: [-1, 0] };

function wallFrame(wall, thickness) {
  const [ax, ay] = wall.from;
  const [bx, by] = wall.to;
  const len = Math.hypot(bx - ax, by - ay);
  const ux = (bx - ax) / len;
  const uy = (by - ay) / len;
  const t = thickness[wall.kind];
  // A footprint along the wall from offset s0 to s1, tt thick, centred on the line.
  const band = (s0, s1, tt = t) => {
    const nx = -uy * (tt / 2);
    const ny = ux * (tt / 2);
    const p0 = [ax + ux * s0, ay + uy * s0];
    const p1 = [ax + ux * s1, ay + uy * s1];
    return [[p0[0] + nx, p0[1] + ny], [p1[0] + nx, p1[1] + ny], [p1[0] - nx, p1[1] - ny], [p0[0] - nx, p0[1] - ny]];
  };
  // Opening coordinates are absolute along the wall's axis.
  const offsetOf = (coord) => (Math.abs(ux) > Math.abs(uy) ? (coord - ax) / ux : (coord - ay) / uy);
  return { ax, ay, ux, uy, len, t, band, offsetOf };
}

// Solid intervals of [lo, hi] once the openings are removed.
export function solidRuns(lo, hi, holes) {
  const sorted = [...holes].sort((p, q) => p[0] - q[0]);
  const runs = [];
  let cursor = lo;
  for (const [h0, h1] of sorted) {
    if (h0 > cursor) runs.push([cursor, h0]);
    cursor = Math.max(cursor, h1);
  }
  if (hi > cursor) runs.push([cursor, hi]);
  return runs;
}

export function splitRun([s0, s1], maxPiece) {
  const n = Math.max(1, Math.ceil((s1 - s0) / maxPiece - 1e-9));
  const step = (s1 - s0) / n;
  return Array.from({ length: n }, (_, i) => [s0 + i * step, s0 + (i + 1) * step]);
}

function doorLeaves(wall, frame, s0, s1, opening, H) {
  // In a cutaway the leaves would otherwise stand taller than the walls around them.
  const leafHeight = Math.min(LEAF_HEIGHT, H * 0.96);
  const [sx, sy] = COMPASS[opening.swing];
  const half = frame.t / 2;
  const leaf = (hingeS, width, along) => {
    // The leaf stands open at 90°, its edge against the hinge jamb, on the swing side.
    const inset = (LEAF_THICKNESS / 2 + 0.01) * along;
    const hx = frame.ax + frame.ux * (hingeS + inset) + sx * half;
    const hy = frame.ay + frame.uy * (hingeS + inset) + sy * half;
    const poly = [
      [hx - frame.ux * (LEAF_THICKNESS / 2), hy - frame.uy * (LEAF_THICKNESS / 2)],
      [hx + frame.ux * (LEAF_THICKNESS / 2), hy + frame.uy * (LEAF_THICKNESS / 2)],
      [hx + frame.ux * (LEAF_THICKNESS / 2) + sx * width, hy + frame.uy * (LEAF_THICKNESS / 2) + sy * width],
      [hx - frame.ux * (LEAF_THICKNESS / 2) + sx * width, hy - frame.uy * (LEAF_THICKNESS / 2) + sy * width],
    ];
    return prism(poly, 0, leafHeight, { kind: 'door', mat: 'door', wallId: wall.id });
  };
  const width = s1 - s0;
  if (opening.kind === 'double-door') return [leaf(s0, width / 2, 1), leaf(s1, width / 2, -1)];
  return opening.hinge === 'b' ? [leaf(s1, width, -1)] : [leaf(s0, width, 1)];
}

// Every wall, carved at its openings, as prisms up to wall height H.
// Door leaves are returned separately so they can be toggled.
export function wallPrisms(walls, H, { thickness, width, depth, maxPiece = Infinity }) {
  const solids = [];
  const leaves = [];
  const centre = [width / 2, depth / 2];

  for (const wall of walls) {
    const f = wallFrame(wall, thickness);
    const ext = wall.ext ?? f.t / 2;
    const mat = wall.kind === 'glass' ? 'glass' : 'wall';
    let outward = null;
    if (wall.kind === 'exterior') {
      const mid = [f.ax + (f.ux * f.len) / 2, f.ay + (f.uy * f.len) / 2];
      let [nx, ny] = [-f.uy, f.ux];
      if (nx * (mid[0] - centre[0]) + ny * (mid[1] - centre[1]) < 0) [nx, ny] = [-nx, -ny];
      outward = [nx, ny];
    }
    const base = { wallId: wall.id, outward, kind: 'wall' };

    const openings = (wall.openings ?? []).map((o) => {
      const a = f.offsetOf(o.a);
      const b = f.offsetOf(o.b);
      return { ...o, s0: Math.min(a, b), s1: Math.max(a, b) };
    });

    for (const run of solidRuns(-ext, f.len + ext, openings.map((o) => [o.s0, o.s1]))) {
      for (const [s0, s1] of splitRun(run, maxPiece)) {
        solids.push(prism(f.band(s0, s1), 0, H, { ...base, mat }));
      }
    }

    for (const o of openings) {
      if (o.kind === 'window') {
        const sill = Math.min(o.sill, H);
        const head = Math.min(o.head, H);
        if (sill > 0) solids.push(prism(f.band(o.s0, o.s1), 0, sill, { ...base, mat: 'wall' }));
        if (head > sill) solids.push(prism(f.band(o.s0, o.s1, f.t * 0.35), sill, head, { ...base, mat: 'glass' }));
        if (H > o.head) solids.push(prism(f.band(o.s0, o.s1), o.head, H, { ...base, mat: 'wall' }));
      } else {
        if (H > DOOR_HEAD) solids.push(prism(f.band(o.s0, o.s1), DOOR_HEAD, H, { ...base, mat: 'wall' }));
        leaves.push(...doorLeaves(wall, f, o.s0, o.s1, o, H));
      }
    }
  }
  return { solids, leaves };
}

// ---------------------------------------------------------------- furniture

const DESK_HEIGHT = 0.75;

// How the furniture is shaped, as opposed to coloured. Skins set this: corner
// rounds the soft rectangles (seats, desk tops), facets multiplies the sides of
// round things. The defaults give the original blocky model.
export const BLOCKY = { corner: 0, facets: 1 };

export function furniturePrisms(item, style = BLOCKY) {
  const { x, y } = item;
  const n = (sides) => Math.round(sides * style.facets);
  // A rectangle with corners rounded by up to style.corner, never more than a
  // quarter of its shorter side so thin parts stay rectangles.
  const soft = (cx, cy, w, d, rot = 0) => roundedRectPoly(cx, cy, w, d, rot, Math.min(style.corner, w / 4, d / 4));
  const facing = item.facing ?? 0;
  const fx = Math.cos(facing * DEG);
  const fy = Math.sin(facing * DEG);

  switch (item.type) {
    case 'desk': {
      const rot = item.rot ?? 0;
      const c = Math.cos(rot * DEG);
      const s = Math.sin(rot * DEG);
      const h = item.h ?? DESK_HEIGHT;
      const legOff = item.w / 2 - 0.04;
      return [
        prism(rectPoly(x - c * legOff, y - s * legOff, 0.04, item.d - 0.1, rot), 0, h - 0.04, { mat: 'metal' }),
        prism(rectPoly(x + c * legOff, y + s * legOff, 0.04, item.d - 0.1, rot), 0, h - 0.04, { mat: 'metal' }),
        prism(soft(x, y, item.w, item.d, rot), h - 0.04, h, { mat: 'wood' }),
      ];
    }
    case 'counter':
    case 'cabinet':
    case 'storage':
    case 'copier':
      return [prism(rectPoly(x, y, item.w, item.d, item.rot ?? 0), 0, item.h, { mat: item.type === 'copier' ? 'appliance' : item.type === 'counter' ? 'counter' : 'cabinet' })];
    case 'sink':
      return [
        prism(rectPoly(x, y, item.w, item.d), 0, item.h, { mat: 'counter' }),
        prism(soft(x, y, item.w * 0.7, item.d * 0.6), item.h, item.h + 0.01, { mat: 'steel' }),
      ];
    case 'board':
      return [prism(rectPoly(x, y, item.w, item.d, item.rot ?? 0), item.z0, item.z1, { mat: 'board' })];
    case 'roundTable':
      return [
        prism(ngonPoly(x, y, 0.05, 6), 0, item.h - 0.04, { mat: 'metal' }),
        prism(ngonPoly(x, y, item.r, n(16)), item.h - 0.04, item.h, { mat: 'wood' }),
      ];
    case 'chair': {
      const size = item.size ?? 0.5;
      const back = size / 2 - 0.035;
      return [
        prism(ngonPoly(x, y, 0.035, 6), 0, 0.4, { mat: 'metal' }),
        prism(soft(x, y, size, size, facing), 0.4, 0.47, { mat: 'fabric' }),
        prism(soft(x - fx * back, y - fy * back, 0.07, size, facing), 0.47, 0.95, { mat: 'fabric' }),
      ];
    }
    case 'armchair':
      return [
        prism(soft(x, y, 0.5, 0.52, facing), 0, 0.42, { mat: 'fabricB' }),
        prism(soft(x - fx * 0.2, y - fy * 0.2, 0.12, 0.52, facing), 0.42, 0.78, { mat: 'fabricB' }),
      ];
    case 'tub':
      return [
        prism(ngonPoly(x, y, 0.32, n(12)), 0, 0.42, { mat: 'fabricB' }),
        prism(soft(x - fx * 0.22, y - fy * 0.22, 0.12, 0.5, facing), 0.42, 0.74, { mat: 'fabricB' }),
      ];
    case 'stool':
      return [
        prism(ngonPoly(x, y, 0.03, 6), 0, 0.62, { mat: 'metal' }),
        prism(ngonPoly(x, y, 0.18, n(10)), 0.62, 0.67, { mat: 'fabric' }),
      ];
    case 'plant': {
      const r = item.r;
      if (style.facets > 1) {
        // Smoother skins get a rounder canopy: three tiers that swell then taper.
        const z = 0.32;
        return [
          prism(ngonPoly(x, y, r * 0.42, n(8)), 0, z, { mat: 'pot' }),
          prism(ngonPoly(x, y, r * 0.8, n(9), 10), z, z + r * 0.6, { mat: 'plant' }),
          prism(ngonPoly(x, y, r, n(9), 30), z + r * 0.6, z + r * 1.3, { mat: 'plant' }),
          prism(ngonPoly(x, y, r * 0.6, n(8), 5), z + r * 1.3, z + r * 1.9, { mat: 'plantLight' }),
        ];
      }
      return [
        prism(ngonPoly(x, y, r * 0.45, 8), 0, 0.32, { mat: 'pot' }),
        prism(ngonPoly(x, y, r, 9, 10), 0.32, 0.32 + r * 1.3, { mat: 'plant' }),
        prism(ngonPoly(x, y, r * 0.62, 7, 25), 0.32 + r * 1.3, 0.32 + r * 2.0, { mat: 'plantLight' }),
      ];
    }
    case 'toilet':
      return [
        prism(soft(x - fx * 0.24, y - fy * 0.24, 0.18, 0.42, facing), 0, 0.8, { mat: 'ceramic' }),
        prism(ngonPoly(x + fx * 0.06, y + fy * 0.06, 0.19, n(12)), 0, 0.42, { mat: 'ceramic' }),
      ];
    case 'basin':
      return [
        prism(rectPoly(x, y, 0.16, 0.16, facing), 0, 0.78, { mat: 'ceramic' }),
        prism(soft(x, y - 0.02, 0.5, 0.4, facing), 0.78, 0.88, { mat: 'ceramic' }),
      ];
    default:
      throw new Error(`unknown furniture type ${item.type}`);
  }
}

// ---------------------------------------------------------------- faces

// Projects one prism. Returns its visible faces (sides then top, which never
// overlap each other for a convex prism) and its screen bounding box.
export function projectPrism(p, view, z0 = p.z0, z1 = p.z1) {
  const [vx, vy] = view.toViewer;
  const bottom = p.poly.map(([x, y]) => view.project(x, y, z0));
  const top = p.poly.map(([x, y]) => view.project(x, y, z1));
  const faces = [];
  const n = p.poly.length;
  for (let i = 0; i < n; i++) {
    const [x0, y0] = p.poly[i];
    const [x1, y1] = p.poly[(i + 1) % n];
    const len = Math.hypot(x1 - x0, y1 - y0) || 1;
    const nx = (y1 - y0) / len;
    const ny = -(x1 - x0) / len;
    if (nx * vx + ny * vy > 1e-6) {
      faces.push({ side: i, nx, ny, pts: [bottom[i], bottom[(i + 1) % n], top[(i + 1) % n], top[i]] });
    }
  }
  faces.push({ side: 'top', pts: top });

  let minX = Infinity;
  let minY = Infinity;
  let maxX = -Infinity;
  let maxY = -Infinity;
  for (const pt of bottom.concat(top)) {
    if (pt[0] < minX) minX = pt[0];
    if (pt[0] > maxX) maxX = pt[0];
    if (pt[1] < minY) minY = pt[1];
    if (pt[1] > maxY) maxY = pt[1];
  }
  return { faces, bbox: [minX, minY, maxX, maxY] };
}

// ---------------------------------------------------------------- ordering

function projectOnto(poly, ax, ay) {
  let lo = Infinity;
  let hi = -Infinity;
  for (const [x, y] of poly) {
    const d = x * ax + y * ay;
    if (d < lo) lo = d;
    if (d > hi) hi = d;
  }
  return [lo, hi];
}

// Wall pieces meet at junctions where one tucks into the other by up to a
// thickness. Treating that sliver as a real overlap would fall back to a centroid
// guess and occasionally paint a wall cap over its neighbour's face.
const WALL_TOLERANCE = 0.13;
const TOLERANCE = 0.02;

// Returns -1 if a must be drawn before b, 1 if after, 0 if either order is fine.
export function compareFootprints(a, b, view) {
  const [vx, vy] = view.toViewer;
  const tol = a.kind === 'wall' && b.kind === 'wall' ? WALL_TOLERANCE : TOLERANCE;
  let best = null;
  for (const poly of [a.poly, b.poly]) {
    const n = poly.length;
    for (let i = 0; i < n; i++) {
      const [x0, y0] = poly[i];
      const [x1, y1] = poly[(i + 1) % n];
      const len = Math.hypot(x1 - x0, y1 - y0);
      if (len < 1e-9) continue;
      const ax = (y1 - y0) / len;
      const ay = -(x1 - x0) / len;
      const [aLo, aHi] = projectOnto(a.poly, ax, ay);
      const [bLo, bHi] = projectOnto(b.poly, ax, ay);
      const aBelow = aHi - bLo; // overlap if a is on the low side
      const bBelow = bHi - aLo;
      const pen = Math.min(aBelow, bBelow);
      if (pen > tol) continue;
      const facing = ax * vx + ay * vy;
      // Any clean separation gives the right answer, so take the one most aligned
      // with the view. Failing that, the shallowest junction sliver.
      const score = pen <= 0 ? 10 + Math.abs(facing) : -pen;
      if (!best || score > best.score) best = { score, facing, aLow: aBelow <= bBelow };
    }
  }
  if (best) {
    if (Math.abs(best.facing) < 1e-9) return 0;
    // The polygon on the side the camera looks from is nearer, so it goes later.
    const bNearer = best.aLow === best.facing > 0;
    return bNearer ? -1 : 1;
  }
  return null; // footprints genuinely overlap
}

export function compareItems(a, b, view) {
  const byFootprint = compareFootprints(a, b, view);
  if (byFootprint !== null) return byFootprint;
  // Stacked parts (a seat on its post, a lintel over a door): lower first,
  // because the camera is always above the floor.
  if (a.z0e >= b.z1e - 1e-6) return 1;
  if (b.z0e >= a.z1e - 1e-6) return -1;
  return a.key - b.key;
}

function boxesOverlap(p, q) {
  return p[0] < q[2] && q[0] < p[2] && p[1] < q[3] && q[1] < p[3];
}

// Topological sort of items ({poly, z0e, z1e, bbox, key}) into draw order.
// key is the fallback depth; ties and cycles resolve by it, far first.
export function orderItems(items, view) {
  const n = items.length;
  const succ = Array.from({ length: n }, () => []);
  const indeg = new Array(n).fill(0);
  for (let i = 0; i < n; i++) {
    for (let j = i + 1; j < n; j++) {
      if (!boxesOverlap(items[i].bbox, items[j].bbox)) continue;
      const c = compareItems(items[i], items[j], view);
      if (c < 0) {
        succ[i].push(j);
        indeg[j]++;
      } else if (c > 0) {
        succ[j].push(i);
        indeg[i]++;
      }
    }
  }

  const heap = new MinHeap((i, j) => items[i].key - items[j].key);
  for (let i = 0; i < n; i++) if (indeg[i] === 0) heap.push(i);
  const done = new Uint8Array(n);
  const out = [];
  while (out.length < n) {
    let i = heap.pop();
    if (i === undefined) {
      // A cycle: release the farthest unfinished item and carry on.
      let pick = -1;
      for (let k = 0; k < n; k++) if (!done[k] && (pick < 0 || items[k].key < items[pick].key)) pick = k;
      i = pick;
    }
    if (done[i]) continue;
    done[i] = 1;
    out.push(items[i]);
    for (const j of succ[i]) {
      if (--indeg[j] === 0 && !done[j]) heap.push(j);
    }
  }
  return out;
}

class MinHeap {
  constructor(less) {
    this.less = less;
    this.a = [];
  }

  push(v) {
    const a = this.a;
    a.push(v);
    let i = a.length - 1;
    while (i > 0) {
      const p = (i - 1) >> 1;
      if (this.less(a[i], a[p]) >= 0) break;
      [a[i], a[p]] = [a[p], a[i]];
      i = p;
    }
  }

  pop() {
    const a = this.a;
    if (a.length === 0) return undefined;
    const top = a[0];
    const last = a.pop();
    if (a.length > 0) {
      a[0] = last;
      let i = 0;
      for (;;) {
        const l = 2 * i + 1;
        const r = l + 1;
        let m = i;
        if (l < a.length && this.less(a[l], a[m]) < 0) m = l;
        if (r < a.length && this.less(a[r], a[m]) < 0) m = r;
        if (m === i) break;
        [a[i], a[m]] = [a[m], a[i]];
        i = m;
      }
    }
    return top;
  }
}
