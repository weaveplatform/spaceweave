#!/usr/bin/env node
// Attach to an isolated Chrome debugging port; open and close only our own tab.
// Outputs timings and SVG hashes, never plan coordinates or source drawings.
import assert from 'node:assert/strict';
import { readFile, writeFile } from 'node:fs/promises';

const [site, output, baseline] = process.argv.slice(2);
if (!site || !output) {
  console.error('usage: animation-trial.mjs <site-url> <output.json> [baseline.json]');
  process.exit(2);
}
const debug = `http://127.0.0.1:${process.env.SPACEWEAVE_CHROME_DEBUG_PORT || 9267}`;
const base = new URL(site);
const catalogue = await (await fetch(new URL('generated/catalog.json', base))).json();
const plans = catalogue.plans.filter(p => ['draft', 'approved'].includes(p.status));
assert.ok(plans.length, 'need at least one generated floor');

async function connect(url) {
  const ws = new WebSocket(url), pending = new Map(), errors = [];
  let serial = 0;
  await new Promise((resolve, reject) => {
    ws.addEventListener('open', resolve, { once: true });
    ws.addEventListener('error', reject, { once: true });
  });
  ws.addEventListener('message', e => {
    const message = JSON.parse(e.data);
    if (message.id) {
      const p = pending.get(message.id);
      pending.delete(message.id);
      if (message.error) p.reject(Error(JSON.stringify(message.error)));
      else p.resolve(message.result);
    } else if (message.method === 'Runtime.exceptionThrown') {
      errors.push(message.params.exceptionDetails.text);
    }
  });
  return {
    call: (method, params = {}) => new Promise((resolve, reject) => {
      const id = ++serial;
      pending.set(id, { resolve, reject });
      ws.send(JSON.stringify({ id, method, params }));
    }),
    close: () => ws.close(), errors,
  };
}

// This function runs inside the page, with its actual D3 renderer and timer.
async function measureFloor(id) {
  const state = floorplan.state;
  Object.assign(state, { theta: -45, phi: 35.26, furniture: true });
  const longTasks = [], renders = [], intervals = [];
  let loaded = Infinity;
  const tasks = new PerformanceObserver(list => longTasks.push(...list.getEntries().map(e => ({ start: e.startTime, duration: e.duration }))));
  tasks.observe({ entryTypes: ['longtask'] });
  const stats = new MutationObserver(() => {
    const match = document.querySelector('#stats').textContent.match(/([\d.]+) ms\/frame/);
    if (match && performance.now() >= loaded) renders.push(+match[1]);
  });
  stats.observe(document.querySelector('#stats'), { childList: true });
  let stopped = false, previous;
  const frame = now => {
    if (now > previous) intervals.push(now - previous);
    previous = now;
    if (!stopped) requestAnimationFrame(frame);
  };
  const start = performance.now();
  await floorplan.loadPlan(id);
  loaded = performance.now(); previous = loaded;
  requestAnimationFrame(frame);
  await new Promise((resolve, reject) => {
    const check = () => {
      if (performance.now() - start > 15000) return reject(Error('animation did not settle'));
      if (state.build >= 1 && !state.moving) resolve();
      else requestAnimationFrame(check);
    };
    requestAnimationFrame(check);
  });
  await new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)));
  stopped = true; tasks.disconnect(); stats.disconnect();
  const percentile = (values, q) => [...values].sort((a, b) => a - b)[Math.floor((values.length - 1) * q)] ?? 0;
  const duringBuild = longTasks.filter(e => e.start >= loaded);
  return { loadMs: loaded - start, settledMs: performance.now() - start, animationMs: performance.now() - loaded,
    frames: intervals.length, frameMsP95: percentile(intervals, .95),
    renderMsP95: percentile(renders, .95), longTasks: duringBuild.length,
    maxLongTaskMs: Math.max(0, ...duringBuild.map(e => e.duration)), paths: document.querySelectorAll('#stage path').length };
}

async function fingerprint(state) {
  Object.assign(floorplan.state, state);
  floorplan.invalidate();
  await new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)));
  const nodes = [...document.querySelectorAll('#stage path,#stage text,#stage .person line')];
  const serialised = JSON.stringify(nodes.map(e => [e.tagName,
    [...e.attributes].map(a => [a.name, a.value]).sort((a, b) => a[0].localeCompare(b[0])), e.textContent]));
  const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(serialised));
  return { hash: [...new Uint8Array(digest)].map(x => x.toString(16).padStart(2, '0')).join(''), nodes: nodes.length };
}

const version = await (await fetch(`${debug}/json/version`)).json();
const browser = await connect(version.webSocketDebuggerUrl);
let page, targetId;
try {
  ({ targetId } = await browser.call('Target.createTarget', { url: 'about:blank' }));
  const tabs = await (await fetch(`${debug}/json/list`)).json();
  page = await connect(tabs.find(tab => tab.id === targetId).webSocketDebuggerUrl);
  const evaluate = async expression => {
    const result = await page.call('Runtime.evaluate', { expression, awaitPromise: true, returnByValue: true });
    if (result.exceptionDetails) throw Error(JSON.stringify(result.exceptionDetails));
    return result.result.value;
  };
  await page.call('Page.enable'); await page.call('Runtime.enable');
  await page.call('Emulation.setDeviceMetricsOverride', { width: 1440, height: 1000, deviceScaleFactor: 1, mobile: false });
  const url = new URL(base);
  url.searchParams.set('plan', plans[0].id); url.searchParams.set('skin', 'model');
  await page.call('Page.navigate', { url: url.href });
  let ready = false;
  for (let i = 0; i < 150; i++) {
    ready = await evaluate('Boolean(globalThis.floorplan)&&floorplan.state.build>=1&&!floorplan.state.moving');
    if (ready) break;
    await new Promise(resolve => setTimeout(resolve, 100));
  }
  assert.ok(ready, 'initial floor must settle');
  const metrics = [], snapshots = [];
  for (const plan of plans) {
    const row = { floor: plan.title, ...await evaluate(`(${measureFloor})(${JSON.stringify(plan.id)})`) };
    metrics.push(row); console.log(JSON.stringify(row));
    assert.ok(row.settledMs >= 2700, 'keep the complete 2.8-second build');
  }
  for (const plan of plans) {
    await evaluate(`(async()=>{await floorplan.loadPlan(${JSON.stringify(plan.id)});d3.timerFlush();d3.select(document.body).interrupt('build')})()`);
    for (const [theta, phi] of [[-45, 35.26], [135, 35.26], [0, 90]]) for (const build of [.35, .65, 1]) {
      const state = { theta, phi, build, moving: build < 1 };
      snapshots.push({ floor: plan.title, theta, phi, build,
        ...await evaluate(`(${fingerprint})(${JSON.stringify(state)})`) });
    }
  }
  const checks = await evaluate(`(async()=>{
    let attributes=0,children=0;
    const observer=new MutationObserver(list=>{for(const m of list){if(m.type==='attributes')attributes++;else children++}});
    observer.observe(document.querySelector('#stage'),{attributes:true,childList:true,subtree:true});
    floorplan.invalidate();await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));observer.disconnect();
    floorplan.build();d3.select(document.body).interrupt('build');await new Promise(r=>setTimeout(r,40));
    return {unchangedSvgAttributes:attributes,unchangedSvgChildren:children,cancelledMotionSettled:!floorplan.state.moving};
  })()`);
  await writeFile(output, JSON.stringify({ browser: version.Browser, viewport: [1440, 1000], skin: 'model', metrics, snapshots, checks }, null, 2)+'\n');
  if (baseline) {
    const before = JSON.parse(await readFile(baseline, 'utf8'));
    assert.deepEqual(snapshots, before.snapshots, 'fixed-stage SVG output must stay identical');
  }
  assert.equal(checks.unchangedSvgAttributes, 0, 'settled SVG should not be rewritten');
  assert.equal(checks.unchangedSvgChildren, 0, 'settled SVG should not be reordered');
  assert.equal(checks.cancelledMotionSettled, true, 'pending cancellation must release motion');
  assert.deepEqual(page.errors, [], 'no uncaught browser errors');
  console.log(`PASS: ${snapshots.length} SVG fingerprints, retained animation, unchanged SVG and cancellation`);
} finally {
  if (targetId) await browser.call('Target.closeTarget', { targetId });
  page?.close(); browser.close();
}
