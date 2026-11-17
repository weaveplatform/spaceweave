import { test } from 'node:test';
import assert from 'node:assert/strict';
import * as geometry from '../js/geometry.js';
import { renderingMetrics } from '../tools/render-quality.mjs';

const plan = {
  WIDTH: 10, DEPTH: 8, THICKNESS: { exterior: .2, interior: .1 },
  walls: [{ id: 'w', kind: 'interior', from: [1, 1], to: [5, 4], openings: [{ a: 2, b: 2.8, kind: 'door', swing: 'S' }] }],
  FOOTPRINT: [[[[0, 0], [4, 0], [4, 4], [0, 4]], [[1, 1], [2, 1], [2, 2], [1, 2]]]],
};

test('render diagnostics measure angled leaves and net slab area', () => {
  const result = renderingMetrics(plan);
  assert.equal(result.doorLeaves, 1);
  assert.equal(result.nonPerpendicularLeaves, 0);
  assert.equal(result.nonRectangularLeaves, 0);
  assert.ok(result.maxLeafAngleErrorDeg < 1e-10);
  assert.equal(result.slabAreaM2, 15);
  assert.equal(result.slabHoles, 1);
  assert.equal(result.suppliedFootprint, true);
});

test('render diagnostics detect an independently supplied skewed leaf', () => {
  const bad = { ...geometry, wallPrisms: () => ({ leaves: [{ wallId: 'w', poly: [[2, 2], [2.04, 2], [2.14, 3], [2.1, 3]] }] }) };
  const result = renderingMetrics(plan, bad);
  assert.equal(result.nonPerpendicularLeaves, 1);
  assert.equal(result.nonRectangularLeaves, 1);
  assert.ok(result.maxLeafAngleErrorDeg > 30);
});
