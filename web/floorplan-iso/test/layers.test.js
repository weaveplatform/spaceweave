import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  escapeGlob, layerRows, nameProblem, setLayerRole, sourceLabel, suggestName, PROFILE_NAME, BUILTIN_PROFILES,
} from '../js/layers.js';
import { createApi, REQUEST_HEADER } from '../js/api.js';

const report = {
  layers: [
    { name: 'A-WALL', role: 'wall', entities: 120, source: 'builtin:aia' },
    { name: 'Q-TRENNWAND', role: 'unknown', entities: 8, source: 'guess' },
    { name: 'L1', role: 'wall', entities: 40, source: 'guess' },
    { name: 'Partitions', role: 'wall', entities: 12, source: 'profile:acme' },
  ],
};

test('rows keep the report’s order and say which roles were guessed', () => {
  const rows = layerRows(report);
  assert.deepEqual(rows.map((r) => r.name), ['A-WALL', 'Q-TRENNWAND', 'L1', 'Partitions']);
  assert.deepEqual(rows.map((r) => r.guessed), [false, true, true, false]);
  assert.deepEqual(rows.map((r) => r.mine), [false, false, false, true], 'only a client rule can be taken back');
  assert.equal(rows[1].entities, 8);
  assert.deepEqual(layerRows({}), [], 'an image, or a report from before layers');
  assert.deepEqual(layerRows(undefined), []);
});

test('sources read as words', () => {
  assert.equal(sourceLabel('guess'), 'guessed');
  assert.equal(sourceLabel('builtin:uniclass'), 'built-in uniclass');
  assert.equal(sourceLabel('profile:acme'), 'your profile “acme”');
  assert.equal(sourceLabel('other'), 'other');
  assert.equal(sourceLabel(), '');
});

test('a layer’s role becomes the first rule of the profile, replacing the page’s earlier one', () => {
  const before = {
    description: 'Acme',
    rules: [
      { role: 'door', layer: 'Q-TRENNWAND' },
      { role: 'furniture', words: ['FITTINGS'] },
      { role: 'glass', layer: 'Q-TRENNWAND', note: 'written by hand: kept' },
    ],
  };
  const after = setLayerRole(before, 'Q-TRENNWAND', 'wall');
  assert.deepEqual(after.rules, [
    { role: 'wall', layer: 'Q-TRENNWAND' },
    { role: 'furniture', words: ['FITTINGS'] },
    { role: 'glass', layer: 'Q-TRENNWAND', note: 'written by hand: kept' },
  ]);
  assert.equal(after.description, 'Acme');
  assert.equal(before.rules.length, 3, 'the profile it was given is left alone');

  const back = setLayerRole(after, 'Q-TRENNWAND', '');
  assert.deepEqual(back.rules.map((r) => r.role), ['furniture', 'glass'], 'no role takes the page’s rule out');
  assert.deepEqual(setLayerRole(undefined, 'X', 'wall').rules, [{ role: 'wall', layer: 'X' }]);
});

test('a layer name with glob characters means only itself', () => {
  assert.equal(escapeGlob('Walls*new?'), 'Walls\\*new\\?');
  assert.equal(escapeGlob('a\\b'), 'a\\\\b');
  assert.equal(escapeGlob('A-WALL'), 'A-WALL');
  const p = setLayerRole({ rules: [{ role: 'door', layer: 'W\\*' }] }, 'W*', 'wall');
  assert.deepEqual(p.rules, [{ role: 'wall', layer: 'W\\*' }], 'the escaped rule is the one replaced');
});

test('profile names are file names, and not the built-ins’', () => {
  assert.equal(nameProblem('acme'), '');
  assert.equal(nameProblem('acme-2_b'), '');
  assert.match(nameProblem('Acme Ltd'), /lower-case/);
  assert.match(nameProblem('../x'), /lower-case/);
  assert.match(nameProblem('generic'), /built-in/);
  assert.equal(suggestName('Floor 02 — Acme HQ'), 'floor-02-acme-hq');
  assert.equal(suggestName('—'), 'client');
  assert.equal(suggestName('Generic'), 'client');
  assert.equal(suggestName(), 'client');
  assert.ok(PROFILE_NAME.test(suggestName('x'.repeat(100))));
  assert.deepEqual(BUILTIN_PROFILES, ['uniclass', 'aia', 'cafm', 'generic']);
});

test('the API reads profiles, and saves them with the request header', async () => {
  const calls = [];
  const api = createApi(async (url, init) => {
    calls.push({ url, init });
    return { ok: true, status: 200, json: async () => ({ name: 'acme', profile: { rules: [] }, roles: [] }) };
  });
  await api.profile('acme');
  assert.equal(calls[0].url, 'api/profiles/acme');
  assert.equal(calls[0].init.headers, undefined, 'a GET needs no header');
  await api.saveProfile('acme', { rules: [{ role: 'wall', layer: 'X' }] });
  assert.equal(calls[1].init.method, 'PUT');
  assert.equal(calls[1].init.headers[REQUEST_HEADER], '1', 'a PUT changes something, so it carries the header');
  assert.deepEqual(JSON.parse(calls[1].init.body), { rules: [{ role: 'wall', layer: 'X' }] });
  await api.review('p', 'retrace');
  assert.equal(calls[2].url, 'api/plans/p/retrace');
});
