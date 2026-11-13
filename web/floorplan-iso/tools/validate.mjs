#!/usr/bin/env node
// Checks a plan.json against test/invariants.mjs and prints {"ok", "errors"} on
// stdout. The Go tracer (internal/plan) runs this on every submission. Exit
// status is 0 whenever a verdict was printed, valid or not; anything else is a
// usage or I/O failure.
import { readFile } from 'node:fs/promises';
import { checkPlan } from '../test/invariants.mjs';

const [path] = process.argv.slice(2);
if (!path) {
  console.error('usage: validate.mjs <plan.json>');
  process.exit(2);
}

let plan;
try {
  plan = JSON.parse(await readFile(path, 'utf8'));
} catch (e) {
  console.error(`validate.mjs: ${e.message}`);
  process.exit(2);
}
const errors = checkPlan(plan);
process.stdout.write(`${JSON.stringify({ ok: errors.length === 0, errors })}\n`);
