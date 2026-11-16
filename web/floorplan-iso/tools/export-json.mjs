#!/usr/bin/env node
// Writes a hand-traced plan module out as the plan.json the Go tracer
// produces, e.g.
//
//   node tools/export-json.mjs js/plans/roomsketcher-office.js > /tmp/office.json
//
// The output is the fixture that shows a JSON plan and its module render alike,
// and the reference internal/plan's conversion is tested against.
import { resolve } from 'node:path';
import { pathToFileURL } from 'node:url';

const KEYS = ['SOURCE', 'WIDTH', 'DEPTH', 'OUTLINE', 'ENTRANCE', 'THICKNESS', 'walls', 'rooms', 'floorPatches', 'furniture'];

const [modulePath] = process.argv.slice(2);
if (!modulePath) {
  console.error('usage: export-json.mjs <plan-module.js>');
  process.exit(2);
}
const plan = await import(pathToFileURL(resolve(modulePath)).href);
const out = Object.fromEntries(KEYS.filter((k) => plan[k] !== undefined).map((k) => [k, plan[k]]));
process.stdout.write(`${JSON.stringify(out, null, 1)}\n`);
