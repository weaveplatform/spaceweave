import { test } from 'node:test';
import assert from 'node:assert/strict';
import { makeView, ISO_ELEVATION, PRESETS } from '../js/projection.js';

const close = (a, b, eps = 1e-9) => assert.ok(Math.abs(a - b) < eps, `${a} ≉ ${b}`);

test('straight down with no azimuth is the plan as drawn', () => {
  const v = makeView({ theta: 0, phi: 90, cx: 2, cy: 3 });
  const [sx, sy] = v.project(5, 7, 1.5);
  close(sx, 3);
  close(sy, 4); // height adds nothing when looking straight down
});

test('true isometric foreshortens all three axes equally', () => {
  const v = makeView(PRESETS.iso);
  const len = ([x, y, z]) => {
    const [sx, sy] = v.project(x, y, z);
    return Math.hypot(sx, sy);
  };
  close(len([1, 0, 0]), len([0, 1, 0]));
  close(len([1, 0, 0]), len([0, 0, 1]));
  close(PRESETS.iso.phi, ISO_ELEVATION);
});

test('a full turn of azimuth is the identity', () => {
  const a = makeView({ theta: 20, phi: 40 }).project(3, -2, 1);
  const b = makeView({ theta: 380, phi: 40 }).project(3, -2, 1);
  a.forEach((v, i) => close(v, b[i]));
});

test('higher points are nearer the camera, and toViewer is a unit vector', () => {
  const v = makeView(PRESETS.iso);
  assert.ok(v.project(1, 1, 2)[2] > v.project(1, 1, 0)[2]);
  close(Math.hypot(...v.toViewer), 1);
});
