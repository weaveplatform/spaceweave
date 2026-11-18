import { overlapArea, rectPoly } from './geometry.js';

const bounds = poly => {
  const xs = poly.map(p => p[0]), ys = poly.map(p => p[1]);
  return [Math.min(...xs), Math.min(...ys), Math.max(...xs), Math.max(...ys)];
};
const overlaps = (a, b) => a[0] < b[2] && b[0] < a[2] && a[1] < b[3] && b[1] < a[3];

function hull(points) {
  const sorted = [...points].sort((a, b) => a[0] - b[0] || a[1] - b[1]);
  const cross = (a, b, c) => (b[0] - a[0]) * (c[1] - a[1]) - (b[1] - a[1]) * (c[0] - a[0]);
  const half = list => {
    const out = [];
    for (const p of list) {
      while (out.length > 1 && cross(out.at(-2), out.at(-1), p) <= 0) out.pop();
      out.push(p);
    }
    return out.slice(0, -1);
  };
  return half(sorted).concat(half(sorted.reverse()));
}

// A wall obscures floor points behind it along the actual parallel camera ray.
// Sweeping its footprint by -H * (vx, vy) / vz gives that shadow exactly. The
// footprint's own overlap is subtracted: room boundaries often follow wall
// centrelines, and that half-thickness alone must not remove the rear wall.
export function createRoomCutaway(walls, thickness) {
  const footprints = walls.map(wall => {
    const [ax, ay] = wall.from, [bx, by] = wall.to;
    const t = wall.thickness ?? thickness[wall.kind] ?? thickness.interior;
    const poly = rectPoly((ax + bx) / 2, (ay + by) / 2,
      Math.hypot(bx - ax, by - ay) + 2 * (wall.ext ?? t / 2), t,
      Math.atan2(by - ay, bx - ax) * 180 / Math.PI);
    return { id: wall.id, poly, box: bounds(poly) };
  });
  let target, targetBox, touching;
  let previous = {}, cuts = new Set();
  return (polygon, view, height) => {
    if (previous.polygon === polygon && previous.theta === view.theta && previous.phi === view.phi && previous.height === height) return cuts;
    previous = { polygon, theta: view.theta, phi: view.phi, height };
    cuts = new Set();
    const [vx, vy, vz] = view.toViewer;
    if (!polygon || vz <= 0 || height <= 0 || Math.hypot(vx, vy) < 1e-6) return cuts;
    if (target !== polygon) {
      target = polygon; targetBox = bounds(polygon);
      touching = new Map(footprints.map(w => [w.id, overlaps(w.box, targetBox) ? overlapArea(w.poly, polygon) : 0]));
    }
    const dx = -height * vx / vz, dy = -height * vy / vz;
    for (const wall of footprints) {
      const [x0, y0, x1, y1] = wall.box;
      if (!overlaps([x0 + Math.min(0, dx), y0 + Math.min(0, dy), x1 + Math.max(0, dx), y1 + Math.max(0, dy)], targetBox)) continue;
      const shadow = hull(wall.poly.concat(wall.poly.map(([x, y]) => [x + dx, y + dy])));
      if (overlapArea(shadow, polygon) - touching.get(wall.id) > 1e-6) cuts.add(wall.id);
    }
    return cuts;
  };
}
