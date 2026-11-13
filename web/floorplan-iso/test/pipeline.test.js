import { test } from 'node:test';
import assert from 'node:assert/strict';
import { describe, inPipeline, busy, STEPS } from '../js/pipeline.js';
import { createApi, REQUEST_HEADER, ApiError } from '../js/api.js';

const states = (entry) => describe(entry, 15).steps.map((s) => s.state);

test('each status sits on its step of the pipeline', () => {
  assert.deepEqual(STEPS, ['Uploaded', 'Queued', 'Tracing', 'Review']);
  assert.deepEqual(states({ status: 'uploading' }), ['current', 'todo', 'todo', 'todo']);
  assert.deepEqual(states({ status: 'queued' }), ['done', 'current', 'todo', 'todo']);
  assert.deepEqual(states({ status: 'tracing' }), ['done', 'done', 'current', 'todo']);
  assert.deepEqual(states({ status: 'failed' }), ['done', 'done', 'failed', 'todo']);
  assert.deepEqual(states({ status: 'draft' }), ['done', 'done', 'done', 'current']);
  assert.deepEqual(states({ status: 'approved' }), ['done', 'done', 'done', 'done']);
});

test('the detail line says what is happening and what it has cost', () => {
  assert.equal(describe({ status: 'tracing' }, 15).detail, 'Starting…');
  assert.equal(
    describe({ status: 'tracing', progress: { turn: 14, submits: 1, costUsd: 2.314 } }, 15).detail,
    'Turn 14 · 1 submission · $2.31 of $15.00',
  );
  assert.equal(
    describe({ status: 'tracing', progress: { turn: 3, submits: 0, costUsd: 0 } }).detail,
    'Turn 3 · 0 submissions · $0.00',
    'no cap known without the API',
  );
  assert.equal(describe({ status: 'queued' }).detail, 'Waiting for the tracer');
  assert.match(describe({ status: 'queued', note: 'Add the kitchen' }).detail, /with your note: “Add the kitchen”/);
  assert.equal(describe({ status: 'draft', confidence: 0.82, costUsd: 0.768 }).detail, 'Ready for review · confidence 82% · $0.77');
  assert.equal(describe({ status: 'draft' }).detail, 'Ready for review · $0.00');
  assert.equal(describe({ status: 'failed', error: 'tracer stopped', costUsd: 1 }).detail, 'Failed: tracer stopped · $1.00');
  assert.equal(describe({ status: 'failed' }).detail, 'Failed: no reason given · $0.00');
  assert.equal(describe({ status: 'uploading' }).detail, 'Uploading…');
  assert.equal(describe({ status: 'approved' }).detail, 'Approved');
  assert.equal(describe({ status: 'mystery' }).detail, 'mystery');
});

test('only drafts can be opened from the list and only failures retried', () => {
  assert.equal(describe({ status: 'draft' }).action, 'open');
  assert.equal(describe({ status: 'failed' }).action, 'retry');
  assert.equal(describe({ status: 'tracing' }).action, null);
  assert.equal(describe({ status: 'draft', attempts: 2 }).attempt, 'attempt 2');
  assert.equal(describe({ status: 'draft', attempts: 1 }).attempt, '');
});

test('the list holds unfinished generated plans, newest first', () => {
  const plans = [
    { id: 'office' },
    { id: 'a', generated: true, status: 'approved', updatedAt: '2026-11-13T10:00:00Z' },
    { id: 'b', generated: true, status: 'draft', updatedAt: '2026-11-13T09:00:00Z' },
    { id: 'c', generated: true, status: 'failed', updatedAt: '2026-11-13T11:00:00Z' },
    { id: 'd', generated: true, status: 'queued' },
  ];
  assert.deepEqual(inPipeline(plans).map((p) => p.id), ['c', 'b', 'd']);
  assert.equal(busy(plans), true);
  assert.equal(busy(plans.slice(0, 4)), false);
});

function fakeFetch(status, body) {
  const calls = [];
  const impl = async (url, init) => {
    calls.push({ url, init });
    return { ok: status < 300, status, json: async () => (body === undefined ? JSON.parse('<html>') : body) };
  };
  return { impl, calls };
}

test('the API client sends the request header on every POST', async () => {
  const { impl, calls } = fakeFetch(201, { plan: { id: 'x' }, duplicate: false });
  const api = createApi(impl);
  const file = new File([new Uint8Array([137, 80, 78, 71])], 'image.png', { type: 'image/png' });
  assert.deepEqual(await api.upload(file, 'Pasted plan.png'), { plan: { id: 'x' }, duplicate: false });
  assert.equal(calls[0].url, 'api/plans');
  assert.equal(calls[0].init.headers[REQUEST_HEADER], '1');
  assert.equal(calls[0].init.body.get('image').name, 'Pasted plan.png');

  await api.review('a b', 'reject', 'Add the kitchen');
  assert.equal(calls[1].url, 'api/plans/a%20b/reject');
  assert.equal(calls[1].init.headers[REQUEST_HEADER], '1');
  assert.equal(calls[1].init.headers['Content-Type'], 'application/json');
  assert.deepEqual(JSON.parse(calls[1].init.body), { note: 'Add the kitchen' });

  await api.info();
  assert.equal(calls[2].url, 'api/info');
  assert.equal(calls[2].init.headers, undefined, 'a GET needs no header');
  assert.equal(calls[2].init.cache, 'no-store');
});

test('API errors carry the server message, or the status when there is none', async () => {
  await assert.rejects(createApi(fakeFetch(409, { error: 'x is draft' }).impl).review('x', 'retry'),
    (err) => err instanceof ApiError && err.status === 409 && err.message === 'x is draft');
  await assert.rejects(createApi(fakeFetch(404).impl).info(), /HTTP 404/, 'a static server has no API');
});
