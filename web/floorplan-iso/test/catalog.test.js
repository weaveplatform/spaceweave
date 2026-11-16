import { test } from 'node:test';
import assert from 'node:assert/strict';
import { loadCatalog, PLANS } from '../js/plans/index.js';

const respond = (status, body) => ({ status, ok: status >= 200 && status < 300, json: async () => body });

function fakeFetch(routes) {
  const calls = [];
  const impl = async (url, init) => {
    calls.push({ url, init });
    return url in routes ? routes[url] : respond(404);
  };
  return { impl, calls };
}

test('no catalog.json means no generated plans', async () => {
  assert.deepEqual(await loadCatalog(fakeFetch({}).impl), []);
});

test('a server error is reported, not mistaken for an empty catalogue', async () => {
  const { impl } = fakeFetch({ 'generated/catalog.json': respond(500) });
  await assert.rejects(loadCatalog(impl), /HTTP 500/);
});

test('lists ready and in-progress plans, hides the rest, and never shadows a built-in plan', async () => {
  const { impl, calls } = fakeFetch({
    'generated/catalog.json': respond(200, {
      plans: [
        { id: 'a-1', title: 'A', status: 'draft', confidence: 0.8, summary: 'Small office' },
        { id: 'b-2', status: 'approved' },
        { id: 'c-3', status: 'tracing' },
        { id: 'd-4', status: 'queued' },
        { id: 'e-5', status: 'rejected' },
        { id: 'f-6', status: 'failed' },
        { id: PLANS[0].id, status: 'draft' },
      ],
    }),
    'generated/a-1/plan.json': respond(200, { WIDTH: 3 }),
  });
  const plans = await loadCatalog(impl);
  assert.deepEqual(plans.map((p) => [p.id, p.status]), [['a-1', 'draft'], ['b-2', 'approved'], ['c-3', 'tracing'], ['d-4', 'queued']]);
  assert.equal(calls[0].init.cache, 'no-store');

  const [a, b] = plans;
  assert.equal(a.name, 'A');
  assert.equal(a.blurb, 'Small office');
  assert.equal(a.confidence, 0.8);
  assert.equal(a.dir, 'generated/a-1/');
  assert.equal(b.name, 'b-2', 'falls back to the id');
  assert.deepEqual(await a.load(), { WIDTH: 3 });
  await assert.rejects(b.load(), /b-2\/plan.json: HTTP 404/);
});

test('a catalogue with no plans key is empty', async () => {
  const { impl } = fakeFetch({ 'generated/catalog.json': respond(200, {}) });
  assert.deepEqual(await loadCatalog(impl), []);
});
