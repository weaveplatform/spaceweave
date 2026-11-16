// Every plan, hand-traced or generated, must pass test/invariants.mjs. A
// built-in plan whose module doesn't exist yet is skipped, so plans can land
// one at a time; generated plans are checked when generated/catalog.json exists.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { PLANS } from '../js/plans/index.js';
import { checkPlan } from './invariants.mjs';

const generated = new URL('../generated/', import.meta.url);

async function generatedPlans() {
  let catalog;
  try {
    catalog = JSON.parse(await readFile(new URL('catalog.json', generated), 'utf8'));
  } catch (e) {
    if (e.code === 'ENOENT') return [];
    throw e;
  }
  return catalog.plans
    .filter((p) => p.status === 'draft' || p.status === 'approved')
    .map((p) => ({ id: `generated/${p.id}`, load: async () => JSON.parse(await readFile(new URL(`${p.id}/plan.json`, generated), 'utf8')) }));
}

for (const entry of [...PLANS, ...(await generatedPlans())]) {
  let plan;
  try {
    plan = await entry.load();
  } catch (e) {
    if (e.code === 'ERR_MODULE_NOT_FOUND') {
      test(`${entry.id}: not traced yet`, { skip: true }, () => {});
      continue;
    }
    throw e;
  }
  test(`${entry.id}: meets every plan invariant`, () => {
    assert.deepEqual(checkPlan(plan), []);
  });
}
