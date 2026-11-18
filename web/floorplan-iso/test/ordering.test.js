import { test } from 'node:test';
import assert from 'node:assert/strict';
import { compareItems, orderItems, overlapCandidates, createItemOrderer, prism, rectPoly, projectPrism } from '../js/geometry.js';
import { makeView } from '../js/projection.js';

// The pre-optimisation pair scan and queue, retained as a differential oracle.
// Equal depths and cycles make a merely topologically valid order insufficient.
function legacyOrder(items, view) {
  const n = items.length;
  const succ = Array.from({ length: n }, () => []), indeg = new Array(n).fill(0);
  for (let i = 0; i < n; i++) for (let j = i + 1; j < n; j++) {
    const a = items[i].bbox, b = items[j].bbox;
    if (!(a[0] < b[2] && b[0] < a[2] && a[1] < b[3] && b[1] < a[3])) continue;
    const c = compareItems(items[i], items[j], view);
    if (c < 0) { succ[i].push(j); indeg[j]++; }
    else if (c > 0) { succ[j].push(i); indeg[i]++; }
  }
  const heap = [];
  const less = (i, j) => items[i].key - items[j].key;
  const push = (v) => {
    heap.push(v);
    for (let i = heap.length - 1; i > 0;) {
      const p = (i - 1) >> 1;
      if (less(heap[i], heap[p]) >= 0) break;
      [heap[i], heap[p]] = [heap[p], heap[i]]; i = p;
    }
  };
  const pop = () => {
    if (!heap.length) return undefined;
    const top = heap[0], last = heap.pop();
    if (heap.length) {
      heap[0] = last;
      for (let i = 0;;) {
        const l = 2 * i + 1, r = l + 1;
        let m = i;
        if (l < heap.length && less(heap[l], heap[m]) < 0) m = l;
        if (r < heap.length && less(heap[r], heap[m]) < 0) m = r;
        if (m === i) break;
        [heap[i], heap[m]] = [heap[m], heap[i]]; i = m;
      }
    }
    return top;
  };
  for (let i = 0; i < n; i++) if (!indeg[i]) push(i);
  const done = new Uint8Array(n), out = [];
  while (out.length < n) {
    let i = pop();
    if (i === undefined) {
      let pick = -1;
      for (let k = 0; k < n; k++) if (!done[k] && (pick < 0 || items[k].key < items[pick].key)) pick = k;
      i = pick;
    }
    if (done[i]) continue;
    done[i] = 1; out.push(items[i]);
    for (const j of succ[i]) if (--indeg[j] === 0 && !done[j]) push(j);
  }
  return out;
}

function randomScene() {
  let seed = 982451653;
  const random = () => ((seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0) / 2 ** 32);
  return Array.from({ length: 180 }, (_, i) => prism(
    rectPoly(random() * 30, random() * 30, i % 9 ? .6 : 20, .3 + random(), random() * 180),
    i % 3 ? 0 : .5, .7 + random() * 2, { kind: i % 9 ? 'furniture' : 'wall' },
  ));
}
function projected(prisms, view, growth) {
  return prisms.map((p, i) => {
    const z0e = p.z0 * growth, z1e = p.z1 * growth;
    return { poly: p.sortPoly, kind: p.kind, z0e, z1e,
      bbox: projectPrism(p, view, z0e, z1e).bbox, key: i % 7 };
  });
}

test('screen sweep returns exactly the old overlap pairs in input order', () => {
  const boxes = [[0, 0, 1, 1], [1, 0, 2, 1], [-1, .5, 2, .7], [.5, .2, .5, .8], [-2, 2, 3, 3]];
  const items = boxes.map(bbox => ({ bbox }));
  assert.deepEqual(overlapCandidates(items), [[2, 3], [2], [3], [], []]);
  assert.deepEqual(overlapCandidates([]), []);
  const separated = Array.from({ length: 2000 }, (_, i) => ({ bbox: [i * 2, 0, i * 2 + 1, 1] }));
  assert.equal(overlapCandidates(separated).flat().length, 0);
});

test('cached and uncached ordering match the old algorithm across growth and views', () => {
  const prisms = randomScene(), cached = createItemOrderer();
  for (const theta of [-135, -45, 0, 45, 135, -45]) for (const phi of [35.26, 90]) {
    const view = makeView({ theta, phi });
    for (const growth of [.1, .4, 1, .7, 1]) {
      const items = projected(prisms, view, growth);
      const expected = legacyOrder(items, view);
      assert.deepEqual(orderItems(items, view), expected);
      assert.deepEqual(cached(items, view), expected);
      const subset = items.filter((_, i) => i % 3);
      assert.deepEqual(cached(subset, view), legacyOrder(subset, view));
    }
  }
});

test('cached overlap comparisons retain dynamic stack heights and wall tolerance', () => {
  const cached = createItemOrderer(), view = makeView({ theta: 0, phi: 35 });
  const a = prism(rectPoly(0, 0, 1, 1), 0, 1), b = prism(rectPoly(0, 0, 1, 1), .4, .6);
  const items = projected([a, b], view, 1);
  assert.deepEqual(cached(items, view), legacyOrder(items, view));
  items[0].z0e = 1; items[0].z1e = 2;
  assert.deepEqual(cached(items, view), legacyOrder(items, view));
  items[0].kind = 'wall'; items[1].kind = 'wall';
  assert.deepEqual(cached(items, view), legacyOrder(items, view));
  assert.deepEqual(cached([], view), []);

  // A 5 cm footprint overlap is genuine overlap for furniture but falls
  // within the wall-junction allowance. Reusing the furniture result here
  // would reverse which wall face appears in front.
  const junctionView = makeView({ theta: -90, phi: 35 });
  const junction = projected([
    prism(rectPoly(0, 0, 1, 1), 0, 1),
    prism(rectPoly(.95, 0, 1, 1), 0, 1),
  ], junctionView, 1);
  junction[0].key = 0; junction[1].key = 1;
  assert.deepEqual(cached(junction, junctionView).map(i => i.key), [0, 1]);
  assert.deepEqual(cached(junction, junctionView).map(i => i.key), [0, 1]);
  junction.forEach(i => { i.kind = 'wall'; });
  assert.deepEqual(cached(junction, junctionView).map(i => i.key), [1, 0]);
});
