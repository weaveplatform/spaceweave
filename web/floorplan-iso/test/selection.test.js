import { test } from 'node:test';
import assert from 'node:assert/strict';
import { describeSelection } from '../js/selection.js';
import { statusLabel } from '../js/workspace.js';

const plan = {
  rooms: [
    { id: 'zone', name: 'Work area', kind: 'zone', polygon: [[0, 0], [10, 0], [10, 10], [0, 10]] },
    { id: 'room', name: 'Office', polygon: [[1, 1], [4, 1], [4, 3], [1, 3]] },
  ],
  furniture: [{ type: 'chair', x: 2, y: 2 }, { type: 'desk', x: 2, y: 2, w: 1.4, d: .7, label: 'A' }],
};

test('rooms and zones share selection details without changing their source geometry', () => {
  const before = JSON.stringify(plan);
  assert.deepEqual(describeSelection(plan, { type: 'room', id: 'room' }), {
    type: 'room', id: 'room', label: 'Office', roomId: 'room', detail: 'Room · 6.0 m²',
  });
  assert.equal(describeSelection(plan, { type: 'room', id: 'zone' }).detail, 'Open-plan zone · 100.0 m²');
  assert.equal(JSON.stringify(plan), before);
});

test('desk selection keeps source identity and focuses the smaller containing room', () => {
  assert.deepEqual(describeSelection(plan, { type: 'desk', id: 'desk-1' }), {
    type: 'desk', id: 'desk-1', label: 'Desk 1 · A', roomId: 'room', detail: '1.40 × 0.70 m · Office',
  });
  assert.equal(describeSelection({ rooms: [], furniture: [{ type: 'desk', x: 20, y: 20 }] }, { type: 'desk', id: 'desk-0' }).detail, 'Desk');
});

test('clearing a selection or loading a different floor cannot retain stale object details', () => {
  for (const selection of [null, { type: 'room', id: 'missing' }, { type: 'desk', id: 'desk-0' }, { type: 'wall', id: 'x' }]) {
    assert.equal(describeSelection(plan, selection), null);
  }
  assert.equal(describeSelection(null, { type: 'room', id: 'room' }), null);
});

test('floor status reports review and processing state without presenting confidence as accuracy', () => {
  assert.equal(statusLabel({ generated: true, status: 'draft', confidence: .99 }), 'Needs review');
  assert.equal(statusLabel({ generated: true, status: 'approved' }), 'Approved');
  assert.equal(statusLabel({ generated: true, status: 'failed' }), 'Failed');
  assert.equal(statusLabel({ generated: true, status: 'unknown' }), 'Unavailable');
  assert.equal(statusLabel({ id: 'office' }), 'Example');
});
