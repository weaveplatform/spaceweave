#!/usr/bin/env node
// Geometric renderer diagnostics on a fixed plan. --geometry can point to a
// prior geometry.js revision, keeping input evidence identical in comparisons.
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { pathToFileURL } from 'node:url';
import * as current from '../js/geometry.js';

export function renderingMetrics(plan, geometry = current) {
  const { leaves } = geometry.wallPrisms(plan.walls, 2.7, {
    thickness: plan.THICKNESS, width: plan.WIDTH, depth: plan.DEPTH,
    outline: plan.OUTLINE, footprint: plan.FOOTPRINT,
  });
  let nonPerpendicularLeaves = 0, nonRectangularLeaves = 0, maxLeafAngleErrorDeg = 0;
  for (const leaf of leaves) {
    const wall = plan.walls.find((w) => w.id === leaf.wallId);
    const ux = wall.to[0] - wall.from[0], uy = wall.to[1] - wall.from[1];
    const edges = leaf.poly.map((p, i) => [leaf.poly[(i + 1) % leaf.poly.length][0] - p[0], leaf.poly[(i + 1) % leaf.poly.length][1] - p[1]]);
    const longest = edges.reduce((a, b) => Math.hypot(...a) >= Math.hypot(...b) ? a : b);
    const cosine = Math.abs(longest[0] * ux + longest[1] * uy) / Math.hypot(...longest) / Math.hypot(ux, uy);
    const error = Math.asin(Math.min(1, cosine)) * 180 / Math.PI;
    if (error > .01) nonPerpendicularLeaves++;
    maxLeafAngleErrorDeg = Math.max(maxLeafAngleErrorDeg, error);
    if (edges.some((a, i) => {
      const b = edges[(i + 1) % edges.length];
      return Math.abs(a[0] * b[0] + a[1] * b[1]) / Math.hypot(...a) / Math.hypot(...b) > 1e-6;
    })) nonRectangularLeaves++;
  }
  const supplied = Boolean(geometry.floorFootprint && plan.FOOTPRINT?.length);
  const polygons = geometry.floorFootprint ? geometry.floorFootprint(plan) : [[
    geometry.offsetPolygon(plan.OUTLINE ?? [[0, 0], [plan.WIDTH, 0], [plan.WIDTH, plan.DEPTH], [0, plan.DEPTH]], plan.THICKNESS.exterior / 2),
    ...(plan.VOIDS ?? []),
  ]];
  const slabAreaM2 = polygons.reduce((sum, [outer, ...holes]) => sum + current.polygonArea(outer) - holes.reduce((s, h) => s + current.polygonArea(h), 0), 0);
  return { doorLeaves: leaves.length, nonPerpendicularLeaves, nonRectangularLeaves, maxLeafAngleErrorDeg,
    suppliedFootprint: supplied, slabAreaM2, slabComponents: polygons.length,
    slabHoles: polygons.reduce((sum, polygon) => sum + polygon.length - 1, 0) };
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  const [planPath, flag, modulePath] = process.argv.slice(2);
  if (!planPath || (flag && (flag !== '--geometry' || !modulePath))) {
    console.error('usage: render-quality.mjs <plan.json> [--geometry <geometry-module.js>]');
    process.exitCode = 2;
  } else {
    const geometry = modulePath ? await import(pathToFileURL(resolve(modulePath)).href) : current;
    console.log(JSON.stringify(renderingMetrics(JSON.parse(readFileSync(planPath, 'utf8')), geometry)));
  }
}
