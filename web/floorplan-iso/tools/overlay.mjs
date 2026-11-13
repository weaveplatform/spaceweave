#!/usr/bin/env node
// Draws a traced plan back over its source image so tracing errors are
// visible at a glance: walls (red exterior, blue interior, cyan glass), door and
// window openings (green), room outlines with names (purple), and furniture
// footprints (orange). Output is SVG in source-image pixels; rasterise it to
// look at it, e.g.
//
//   node tools/overlay.mjs js/plans/pt-clinic.js ~/plans/clinic.png /tmp/clinic.svg
//   rsvg-convert /tmp/clinic.svg -o /tmp/clinic.png
//
// The image is embedded as a data URI so the SVG renders anywhere.

import { readFileSync, writeFileSync } from 'node:fs';
import { resolve, extname } from 'node:path';
import { pathToFileURL } from 'node:url';
import { furniturePrisms } from '../js/geometry.js';

const [modulePath, imagePath, outPath, ...flags] = process.argv.slice(2);
if (!modulePath || !imagePath || !outPath) {
  console.error('usage: overlay.mjs <plan-module.js> <source-image> <out.svg> [--grid]');
  process.exit(2);
}

const plan = await import(pathToFileURL(resolve(modulePath)).href);
const { pxPerMetre: k, originPx: [ox, oy], sizePx: [W, H] } = plan.SOURCE;
const X = (m) => (ox + m * k).toFixed(1);
const Y = (m) => (oy + m * k).toFixed(1);
const pts = (poly) => poly.map(([x, y]) => `${X(x)},${Y(y)}`).join(' ');
const esc = (t) => String(t).replace(/[&<>"]/g, (c) => `&#${c.charCodeAt(0)};`);

const mime = { '.png': 'image/png', '.jpg': 'image/jpeg', '.jpeg': 'image/jpeg', '.webp': 'image/webp' }[extname(imagePath).toLowerCase()] ?? 'image/png';
const data = readFileSync(imagePath).toString('base64');

const out = [
  `<svg xmlns="http://www.w3.org/2000/svg" width="${W}" height="${H}" viewBox="0 0 ${W} ${H}" font-family="Helvetica, Arial, sans-serif">`,
  `<image href="data:${mime};base64,${data}" width="${W}" height="${H}" opacity="0.55"/>`,
];

if (flags.includes('--grid')) {
  for (let m = 0; m <= Math.max(plan.WIDTH, plan.DEPTH) + 1; m++) {
    const major = m % 5 === 0;
    out.push(`<line x1="${X(m)}" y1="0" x2="${X(m)}" y2="${H}" stroke="#e0115f" stroke-opacity="${major ? 0.6 : 0.25}" stroke-width="1"/>`);
    out.push(`<line x1="0" y1="${Y(m)}" x2="${W}" y2="${Y(m)}" stroke="#e0115f" stroke-opacity="${major ? 0.6 : 0.25}" stroke-width="1"/>`);
    if (major) out.push(`<text x="${X(m)}" y="${Y(0) - 4}" font-size="${k * 0.4}" fill="#e0115f">${m}</text>`);
  }
}

for (const r of plan.rooms) {
  out.push(`<polygon points="${pts(r.polygon)}" fill="#7b3fe4" fill-opacity="0.08" stroke="#7b3fe4" stroke-width="2" stroke-dasharray="6 4"/>`);
  const [lx, ly] = r.label ?? [r.polygon.reduce((s, p) => s + p[0], 0) / r.polygon.length, r.polygon.reduce((s, p) => s + p[1], 0) / r.polygon.length];
  out.push(`<text x="${X(lx)}" y="${Y(ly)}" font-size="${k * 0.35}" fill="#7b3fe4" text-anchor="middle" font-weight="bold">${esc(r.name)}</text>`);
}

const colour = { exterior: '#d62728', interior: '#1f5fd6', glass: '#17becf' };
for (const w of plan.walls) {
  const t = Math.max(2, (w.thickness ?? plan.THICKNESS[w.kind]) * k);
  out.push(`<line x1="${X(w.from[0])}" y1="${Y(w.from[1])}" x2="${X(w.to[0])}" y2="${Y(w.to[1])}" stroke="${colour[w.kind]}" stroke-width="${t}" stroke-opacity="0.75"/>`);
  const [dx, dy] = [w.to[0] - w.from[0], w.to[1] - w.from[1]];
  const horizontal = Math.abs(dx) >= Math.abs(dy);
  for (const o of w.openings ?? []) {
    // Openings are given along the wall's dominant axis; find the matching points.
    const at = (c) => {
      const s = horizontal ? (c - w.from[0]) / dx : (c - w.from[1]) / dy;
      return [w.from[0] + dx * s, w.from[1] + dy * s];
    };
    const [a, b] = [at(o.a), at(o.b)];
    out.push(`<line x1="${X(a[0])}" y1="${Y(a[1])}" x2="${X(b[0])}" y2="${Y(b[1])}" stroke="#2ca02c" stroke-width="${t + 3}" stroke-opacity="0.9"/>`);
  }
  out.push(`<text x="${X((w.from[0] + w.to[0]) / 2)}" y="${Y((w.from[1] + w.to[1]) / 2) - 3}" font-size="${k * 0.22}" fill="${colour[w.kind]}">${esc(w.id)}</text>`);
}

for (const item of plan.furniture) {
  for (const p of furniturePrisms(item)) {
    out.push(`<polygon points="${pts(p.poly)}" fill="#ff7f0e" fill-opacity="0.15" stroke="#ff7f0e" stroke-width="1.5"/>`);
  }
  if (item.label) out.push(`<text x="${X(item.x)}" y="${Y(item.y)}" font-size="${k * 0.25}" fill="#b35400" text-anchor="middle">${esc(item.label)}</text>`);
}

if (plan.OUTLINE) out.push(`<polygon points="${pts(plan.OUTLINE)}" fill="none" stroke="#000" stroke-width="2" stroke-dasharray="2 4"/>`);

out.push('</svg>');
writeFileSync(outPath, out.join('\n'));
console.log(`wrote ${outPath} (${plan.walls.length} walls, ${plan.rooms.length} rooms, ${plan.furniture.length} furniture items)`);
