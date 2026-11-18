#!/usr/bin/env node
// All API writes are intercepted: this trial never changes the review floors.
import assert from 'node:assert/strict';
import { writeFile } from 'node:fs/promises';
import { connect } from './browser.mjs';

const [site, output] = process.argv.slice(2);
if (!site || !output) throw Error('usage: workspace-trial.mjs <site-url> <output.json>');
const base = new URL(site), debug = `http://127.0.0.1:${process.env.SPACEWEAVE_CHROME_DEBUG_PORT || 9267}`;
const catalogue = await (await fetch(new URL('generated/catalog.json', base))).json();
const plans = catalogue.plans.filter(p => ['draft', 'approved'].includes(p.status));
assert.ok(plans.length, 'need a review floor');
const info = await (await fetch(new URL('api/info', base))).json();
const generic = await (await fetch(new URL('api/profiles/generic', base))).json();
const version = await (await fetch(`${debug}/json/version`)).json();
const browser = await connect(version.webSocketDebuggerUrl);
let page, targetId;
const writes = [], profiles = new Map();
const testCatalogue = structuredClone(catalogue);
let failSave = false, failQueue = false, tracer = 'auto', imported = false;
try {
  ({ targetId } = await browser.call('Target.createTarget', { url: 'about:blank' }));
  const tabs = await (await fetch(`${debug}/json/list`)).json();
  page = await connect(tabs.find(t => t.id === targetId).webSocketDebuggerUrl);
  const evaluate = async expression => {
    const r = await page.call('Runtime.evaluate', { expression, awaitPromise: true, returnByValue: true });
    if (r.exceptionDetails) throw Error(JSON.stringify(r.exceptionDetails));
    return r.result.value;
  };
  const frames = () => evaluate('new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))');
  const waitFor = async expression => {
    for (let i = 0; i < 200; i++) {
      if (page.errors.length) throw Error(page.errors.join('\n'));
      if (await evaluate(expression)) return;
      await new Promise(r => setTimeout(r, 75));
    }
    throw Error(`Timed out: ${expression}`);
  };
  const clickAt = async ({ x, y }) => {
    await page.call('Input.dispatchMouseEvent', { type: 'mouseMoved', x, y });
    await page.call('Input.dispatchMouseEvent', { type: 'mousePressed', x, y, button: 'left', clickCount: 1 });
    await page.call('Input.dispatchMouseEvent', { type: 'mouseReleased', x, y, button: 'left', clickCount: 1 });
  };
  const click = async selector => {
    const point = await evaluate(`(()=>{const n=document.querySelector(${JSON.stringify(selector)});n.scrollIntoView({block:'nearest'});const b=n.getBoundingClientRect();return{x:b.x+b.width/2,y:b.y+b.height/2}})()`);
    await clickAt(point); await frames();
  };
  const change = async (selector, value) => {
    await evaluate(`(()=>{const n=document.querySelector(${JSON.stringify(selector)});n.value=${JSON.stringify(value)};n.dispatchEvent(new Event('change',{bubbles:true}))})()`); await frames();
  };
  const key = async (key, code = key) => {
    const windowsVirtualKeyCode = { Escape: 27, Tab: 9, Enter: 13, ArrowRight: 39 }[code];
    for (const type of ['keyDown', 'keyUp']) await page.call('Input.dispatchKeyEvent', { type, key, code, windowsVirtualKeyCode });
  };
  const visible = selector => evaluate(`(()=>{
    for(const n of document.querySelectorAll(${JSON.stringify(selector)})){
      const b=n.getBoundingClientRect(),x=b.x+b.width/2,y=b.y+b.height/2;
      if(document.elementFromPoint(x,y)===n)return{x,y,id:n.dataset.deskId??n.__data__.id};
    }throw Error('No visible object');
  })()`);
  const snapshot = async name => {
    if (!process.env.SPACEWEAVE_SCREENSHOT_DIR) return;
    const shot = await page.call('Page.captureScreenshot', { format: 'png' });
    await writeFile(`${process.env.SPACEWEAVE_SCREENSHOT_DIR}/workspace-${name}.png`, Buffer.from(shot.data, 'base64'));
  };
  page.on('Fetch.requestPaused', async ({ requestId, request }) => {
    const url = new URL(request.url), path = url.pathname, method = request.method;
    const reply = async (body, responseCode = 200) => page.call('Fetch.fulfillRequest', {
      requestId, responseCode, responseHeaders: [{ name: 'Content-Type', value: 'application/json' }],
      body: Buffer.from(JSON.stringify(body)).toString('base64'),
    });
    if (path.endsWith('/generated/catalog.json')) return reply({ ...testCatalogue, plans: imported ? [...testCatalogue.plans, { id: 'trial-import', title: 'Trial import', status: 'queued', format: 'png' }] : testCatalogue.plans });
    if (path.endsWith('/api/info')) return reply({ ...info, tracer });
    if (method === 'GET') {
      if (path.includes('/api/profiles/')) {
        const name = decodeURIComponent(path.split('/').at(-1));
        return reply({ ...generic, name, profile: profiles.get(name) ?? { rules: [] } });
      }
      return page.call('Fetch.continueRequest', { requestId });
    }
    // Every non-GET API request ends here, including unexpected mutations.
    writes.push({ path, method, body: request.postData });
    if (method === 'PUT' && path.includes('/api/profiles/')) {
      if (failSave) return reply({ error: 'Trial save failure' }, 503);
      profiles.set(decodeURIComponent(path.split('/').at(-1)), JSON.parse(request.postData));
    } else if (path.endsWith('/api/plans')) imported = true;
    else if (path.endsWith('/retrace')) {
      if (failQueue) return reply({ error: 'Trial queue failure' }, 503);
      const floor = testCatalogue.plans.find(p => p.id === decodeURIComponent(path.split('/').at(-2)));
      if (floor) { floor.status = 'queued'; floor.updatedAt = new Date().toISOString(); }
    }
    return reply({ plan: { id: 'trial-import', status: 'queued' }, duplicate: false });
  });
  await page.call('Fetch.enable', { patterns: [{ urlPattern: `${base.origin}/*api/*` }, { urlPattern: `${base.origin}/*generated/catalog.json` }] });
  await page.call('Page.enable'); await page.call('Runtime.enable');
  await page.call('Emulation.setDeviceMetricsOverride', { width: 1440, height: 1000, deviceScaleFactor: 1, mobile: false });
  const url = new URL(base); url.searchParams.set('plan', plans[Math.min(1, plans.length - 1)].id); url.searchParams.set('skin', 'model');
  await page.call('Page.navigate', { url: url.href });
  await waitFor('Boolean(globalThis.floorplan)&&floorplan.state.build===1&&!floorplan.state.moving');
  assert.equal(await evaluate('document.body.dataset.workspace'), 'view');
  assert.equal(await evaluate('document.querySelector("#floor-select").options.length'), plans.length + 1);
  assert.ok(await evaluate('document.querySelector("#review-panel").hidden&&document.querySelector("#wall-height").getBoundingClientRect().bottom<innerHeight'));
  await snapshot('view');

  const room = await visible('.floors path');
  const before = await evaluate('document.querySelector(".viewport").getAttribute("transform")');
  await clickAt(room); await frames();
  assert.equal(await evaluate('floorplan.selection.type'), 'room');
  assert.equal(await evaluate('floorplan.state.activeRoom'), null, 'selection does not change cutaways');
  assert.equal(await evaluate('document.querySelector(".viewport").getAttribute("transform")'), before, 'selection does not zoom');
  await click('#selection-focus');
  await evaluate('new Promise(r=>setTimeout(r,850))');
  assert.equal(await evaluate('floorplan.state.activeRoom'), room.id);
  assert.equal(await evaluate('floorplan.state.cutawayMode'), 'room');
  assert.ok(await evaluate('!document.querySelector("#focus-banner").hidden'));
  await click('#reset');
  assert.equal(await evaluate('floorplan.state.activeRoom'), room.id, 'Fit leaves cutaway target intact');
  await change('#cutaway-mode', 'off'); assert.ok(await evaluate('document.querySelector("#cutaway-settings").hidden'));
  await click('#whole-floor'); await evaluate('new Promise(r=>setTimeout(r,650))');
  assert.equal(await evaluate('floorplan.state.activeRoom'), null);
  await change('#cutaway-mode', 'exterior');
  const desk = await visible('[data-desk-target]'); await clickAt(desk); await frames();
  assert.equal(await evaluate('floorplan.selection.id'), desk.id);
  await click('#import-open'); assert.ok(await evaluate('document.querySelector("#import-dialog").open'));
  await key('Escape'); await frames();
  assert.ok(await evaluate('!document.querySelector("#import-dialog").open'));
  assert.equal(await evaluate('floorplan.selection.id'), desk.id, 'modal Escape does not clear canvas selection');
  await click('#selection-clear'); assert.equal(await evaluate('floorplan.selection'), null);

  await click('#tab-review');
  await waitFor('!document.querySelector("#review").hidden&&document.querySelectorAll("#layer-rows select").length>1');
  assert.ok(await evaluate('document.querySelector("#send-back").hidden&&document.querySelector("#review-guidance").textContent.includes("not applied")'));
  await click('[data-evidence="overlay"]');
  await waitFor('document.querySelector("#evidence-image").naturalWidth>0&&!document.querySelector("#evidence-image").hidden');
  assert.ok(await evaluate('document.querySelector("#stage").hidden'));
  await snapshot('review');
  await click('[data-evidence="source"]');
  await waitFor('document.querySelector("#evidence-image").complete&&document.querySelector("#evidence-image").naturalWidth>0');
  await click('#tab-view'); assert.ok(await evaluate('!document.querySelector("#stage").hidden'));
  await click('#tab-review');
  await evaluate('document.querySelector("#layers").open=true');
  await evaluate(`(()=>{
    const profile=document.querySelector('#profile-name');profile.value='workspace-trial';profile.dispatchEvent(new Event('input'));
    for(const select of [...document.querySelectorAll('#layer-rows select')].slice(0,2)){
      select.value=[...select.options].find(o=>o.value&&o.value!==select.value).value;select.dispatchEvent(new Event('change'));
    }
  })()`);
  assert.equal(writes.length, 0, 'staging edits has no API writes');
  assert.ok(await evaluate('document.querySelector("#layer-changes").textContent.includes("2 staged")'));
  assert.ok(await evaluate('document.querySelector("#approve").disabled&&document.querySelector("#retrace").disabled'), 'review actions wait for staged changes');
  assert.ok(await evaluate(`(()=>{const e=new Event('beforeunload',{cancelable:true});dispatchEvent(e);return e.defaultPrevented})()`), 'unsaved edits guard against leaving the page');
  if (plans.length > 1) {
    await change('#floor-select', plans[0].id);
    await waitFor(`floorplan.state.build===1&&!floorplan.state.moving&&document.querySelector('#floor-select').value===${JSON.stringify(plans[0].id)}`);
    assert.ok(await evaluate('document.querySelector("#layer-changes").textContent==="No staged changes."'));
    await change('#floor-select', plans[1].id);
    await waitFor('floorplan.state.build===1&&!floorplan.state.moving&&document.querySelector("#layer-changes").textContent.includes("2 staged")');
    assert.equal(await evaluate('document.querySelector("#profile-name").value'), 'workspace-trial');
    assert.equal(writes.length, 0, 'switching floors preserves drafts without saving');
  }
  await click('#layers-apply');
  await waitFor('document.querySelector("#layer-changes").textContent==="No staged changes."');
  assert.equal(writes.filter(r => r.method === 'PUT').length, 1);
  assert.equal(writes.filter(r => r.path.endsWith('/retrace')).length, 1);
  assert.equal(profiles.get('workspace-trial').rules.length, 2);
  assert.equal(await evaluate('document.querySelector("#plan-status").textContent'), 'Queued');
  const finished = testCatalogue.plans.find(p => p.id === plans[Math.min(1, plans.length - 1)].id);
  finished.status = 'draft'; finished.updatedAt = new Date().toISOString();
  await evaluate('floorplan.refreshCatalog()');
  await waitFor('floorplan.state.build===1&&!floorplan.state.moving&&!document.querySelector("#layers").hidden');
  assert.equal(await evaluate('document.querySelector("#plan-status").textContent'), 'Needs review', 'completed retrace refreshes the model and review state');
  await evaluate(`(()=>{const n=document.querySelector('#layer-rows select');n.value=[...n.options].find(o=>o.value&&o.value!==n.value).value;n.dispatchEvent(new Event('change'))})()`);
  failSave = true; await click('#layers-apply');
  await waitFor('!document.querySelector("#layers-apply").disabled&&document.querySelector("#upload-status").classList.contains("error")');
  assert.ok(await evaluate('document.querySelector("#layer-changes").textContent.includes("1 staged")'));
  assert.equal(writes.filter(r => r.path.endsWith('/retrace')).length, 1, 'failed save never retraces');
  await click('#layers-discard'); failSave = false;
  await waitFor('document.querySelector("#layer-changes").textContent==="No staged changes."');
  await evaluate(`(()=>{const n=document.querySelector('#layer-rows select');n.value=[...n.options].find(o=>o.value&&o.value!==n.value).value;n.dispatchEvent(new Event('change'))})()`);
  failQueue = true; await click('#layers-apply');
  await waitFor('document.querySelector("#layer-changes").textContent==="Profile saved; retrace pending."');
  assert.ok(await evaluate('document.querySelector("#layer-rows select").disabled&&document.querySelector("#approve").disabled'));
  const putsBeforeRetry = writes.filter(r => r.method === 'PUT').length;
  failQueue = false; await click('#layers-apply');
  await waitFor('document.querySelector("#layer-changes").textContent==="No staged changes."');
  assert.equal(writes.filter(r => r.method === 'PUT').length, putsBeforeRetry, 'queue retry does not resave the profile');
  finished.status = 'draft'; finished.updatedAt = new Date().toISOString();
  await evaluate('floorplan.refreshCatalog()');
  await waitFor('floorplan.state.build===1&&!floorplan.state.moving&&!document.querySelector("#layers").hidden');

  tracer = 'claude'; await page.call('Page.reload');
  await waitFor('Boolean(globalThis.floorplan)&&floorplan.state.build===1&&!floorplan.state.moving&&!document.querySelector("#send-back").hidden');
  await click('#tab-review'); await click('#send-back');
  await evaluate('document.querySelector("#reject-note").value="Trial correction note"');
  await click('#reject-form button[type="submit"]');
  await waitFor('document.querySelector("#reject-form").hidden');
  assert.equal(JSON.parse(writes.find(r => r.path.endsWith('/reject')).body).note, 'Trial correction note');

  await click('#import-open');
  await evaluate(`(()=>{const d=new DataTransfer();d.items.add(new File(['x'],'unsupported.txt',{type:'text/plain'}));const n=document.querySelector('#upload-file');n.files=d.files;n.dispatchEvent(new Event('change'))})()`);
  await waitFor('document.querySelector("#import-status").classList.contains("error")');
  assert.ok(await evaluate('document.querySelector("#import-dialog").open'));
  await evaluate(`(()=>{const d=new DataTransfer();d.items.add(new File([new Uint8Array([137,80,78,71,13,10,26,10])],'trial.png',{type:'image/png'}));const n=document.querySelector('#upload-file');n.files=d.files;n.dispatchEvent(new Event('change'))})()`);
  await waitFor('document.body.dataset.workspace==="review"&&!document.querySelector("#import-dialog").open&&document.querySelector("#pipeline").textContent.includes("Trial import")');
  assert.equal(writes.filter(r => r.path.endsWith('/api/plans')).length, 1);

  tracer = 'auto';
  await page.call('Emulation.setDeviceMetricsOverride', { width: 390, height: 844, deviceScaleFactor: 1, mobile: true });
  await page.call('Page.reload');
  await waitFor('Boolean(globalThis.floorplan)&&floorplan.state.build===1&&!floorplan.state.moving');
  assert.ok(await evaluate('document.documentElement.scrollWidth<=innerWidth'), 'mobile layout must not overflow horizontally');
  await snapshot('mobile');
  await click('#import-open'); assert.ok(await evaluate('document.querySelector("#import-dialog").getBoundingClientRect().width<=innerWidth'));
  await key('Escape'); await click('#tab-view'); await key('ArrowRight');
  assert.equal(await evaluate('document.body.dataset.workspace'), 'review', 'workspace tabs support keyboard arrows');
  assert.deepEqual(page.errors, [], 'no uncaught browser errors');
  await writeFile(output, JSON.stringify({ browser: version.Browser, floors: plans.length, desktop: [1440,1000], mobile: [390,844], interceptedWrites: writes.length, checks: 'selection, focus, cutaways, evidence, batch save, failed save, tracer capability, import, responsive layout and keyboard passed' }, null, 2)+'\n');
  console.log('PASS: consolidated workspace, mocked API writes, desktop/mobile and keyboard');
} finally {
  if (targetId) await browser.call('Target.closeTarget', { targetId });
  page?.close(); browser.close();
}
