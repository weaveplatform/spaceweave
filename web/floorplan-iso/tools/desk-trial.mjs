#!/usr/bin/env node
// Attach to an isolated Chrome debugging port; open and close only our own tab.
// Outputs desk counts and interaction results, never coordinates or drawings.
import assert from 'node:assert/strict';
import { writeFile } from 'node:fs/promises';

const [site, output] = process.argv.slice(2);
if (!site || !output) {
  console.error('usage: desk-trial.mjs <site-url> <output.json>');
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

// Real pointer events exercise SVG hit testing, D3 pan suppression and focus.
const version = await (await fetch(`${debug}/json/version`)).json();
const browser = await connect(version.webSocketDebuggerUrl);
let page, targetId;
try {
  ({ targetId } = await browser.call('Target.createTarget', { url: 'about:blank' }));
  const tabs = await (await fetch(`${debug}/json/list`)).json();
  page = await connect(tabs.find(tab => tab.id === targetId).webSocketDebuggerUrl);
  const evaluate = async expression => {
    const r = await page.call('Runtime.evaluate', { expression, awaitPromise: true, returnByValue: true });
    if (r.exceptionDetails) throw Error(JSON.stringify(r.exceptionDetails));
    return r.result.value;
  };
  const frames = () => evaluate('new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))');
  const state = async values => {
    await evaluate(`Object.assign(floorplan.state,${JSON.stringify(values)});floorplan.invalidate()`);
    await frames();
  };
  const mouse = (type, point, extra = {}) => page.call('Input.dispatchMouseEvent', { type, ...point, ...extra });
  const click = async point => {
    await mouse('mouseMoved', point);
    await mouse('mousePressed', point, { button: 'left', clickCount: 1 });
    await mouse('mouseReleased', point, { button: 'left', clickCount: 1 });
  };
  const key = async (key, code = key) => {
    const windowsVirtualKeyCode = {Tab:9,Enter:13,Escape:27,Space:32}[code];
    await page.call('Input.dispatchKeyEvent', { type: 'keyDown', key, code, windowsVirtualKeyCode });
    await page.call('Input.dispatchKeyEvent', { type: 'keyUp', key, code, windowsVirtualKeyCode });
  };
  const selected = () => evaluate(`[...new Set([...document.querySelectorAll('.desk-selected')].map(n=>n.dataset.deskId))]`);
  const target = () => evaluate(`(()=>{
    for(const node of document.querySelectorAll('[data-desk-target]')){
      const b=node.getBoundingClientRect(),x=b.left+b.width/2,y=b.top+b.height/2;
      if(x<0||y<0||x>=innerWidth||y>=innerHeight)continue;
      if(document.elementFromPoint(x,y)?.dataset.deskId===node.dataset.deskId)
        return {x,y,id:node.dataset.deskId};
    }
    throw Error('no visible desk target');
  })()`);
  await page.call('Page.enable'); await page.call('Runtime.enable');
  await page.call('Emulation.setDeviceMetricsOverride', { width: 1440, height: 1000, deviceScaleFactor: 1, mobile: false });
  await page.call('Emulation.setTouchEmulationEnabled', { enabled: true });
  const url = new URL(base);
  url.searchParams.set('plan', plans[0].id); url.searchParams.set('skin', 'model');
  await page.call('Page.navigate', { url: url.href });
  let ready = false;
  for (let i=0;i<150;i++) {
    ready = await evaluate('Boolean(globalThis.floorplan)&&floorplan.state.build>=1&&!floorplan.state.moving');
    if(ready)break;
    await new Promise(r=>setTimeout(r,100));
  }
  assert.ok(ready, 'initial floor settles');
  const results = [];
  for (const plan of plans) {
    const source = await (await fetch(new URL(`generated/${plan.id}/plan.json`, base))).json();
    const expected = source.furniture.filter(f=>f.type==='desk').length;
    await evaluate(`(async()=>{await floorplan.loadPlan(${JSON.stringify(plan.id)});d3.timerFlush();d3.select(document.body).interrupt('build')})()`);
    await state({ build: 1, moving: false, theta: -45, phi: 35.26, furniture: true });
    const coverage = await evaluate(`(()=>{
      const nodes=[...document.querySelectorAll('[data-desk-id]')],targets=[...document.querySelectorAll('[data-desk-target]')];
      return {desks:new Set(nodes.map(n=>n.dataset.deskId)).size,targets:targets.length,
        uniqueTargets:new Set(targets.map(n=>n.dataset.deskId)).size,
        accessible:targets.every(n=>n.getAttribute('role')==='button'&&n.getAttribute('tabindex')==='0'&&n.getAttribute('aria-pressed')==='false'&&n.getAttribute('aria-label'))};
    })()`);
    assert.equal(coverage.desks, expected); assert.equal(coverage.targets, expected);
    assert.equal(coverage.uniqueTargets, expected); assert.ok(coverage.accessible);
    assert.deepEqual(await selected(), [], 'new floor clears selection');
    await evaluate(`document.querySelector('[data-desk-target]').focus()`);
    await key('Tab');
    assert.ok(await evaluate(`document.activeElement===document.querySelectorAll('[data-desk-target]')[1]`), 'Tab moves between desks');
    await key('Escape');
    const desk = await target(), point = {x:desk.x,y:desk.y};
    const transform = await evaluate(`document.querySelector('.viewport').getAttribute('transform')`);
    await mouse('mouseMoved', {x:5,y:5}); await frames();
    await evaluate(`globalThis.deskMutations=[];globalThis.deskObserver=new MutationObserver(ms=>deskMutations.push(...ms.map(m=>({id:m.target.dataset.deskId,type:m.type,attribute:m.attributeName}))));deskObserver.observe(document.querySelector('.solids'),{subtree:true,attributes:true,childList:true})`);
    await mouse('mouseMoved', point); await frames();
    assert.ok(await evaluate(`(()=>{const p=[...document.querySelectorAll('[data-desk-id="${desk.id}"]')];return p.length>=3&&p.every(n=>n.classList.contains('desk-hovered'))&&!document.querySelector('.tooltip').hidden})()`), 'whole desk highlights on hover');
    await click(point); await mouse('mouseMoved', {x:5,y:5}); await frames();
    assert.deepEqual(await selected(), [desk.id], 'click persists after pointer leaves');
    assert.equal(await evaluate(`document.querySelector('.viewport').getAttribute('transform')`), transform, 'desk click does not zoom a room');
    const mutations = await evaluate('deskObserver.disconnect();deskMutations');
    assert.ok(mutations.length>0&&mutations.every(m=>m.type==='attributes'&&m.id===desk.id), 'only this desk changes during hover and click');
    await key('Escape'); assert.deepEqual(await selected(), []);
    await evaluate(`document.querySelector('[data-desk-id="${desk.id}"][data-desk-target]').focus()`);
    await key('Enter'); assert.deepEqual(await selected(), [desk.id]);
    await key(' ', 'Space'); assert.deepEqual(await selected(), []);
    await key('Enter');
    await state({theta:135,wallHeight:2.7}); assert.deepEqual(await selected(), [desk.id]);
    await state({theta:0,phi:90}); assert.deepEqual(await selected(), [desk.id]);
    for(const skin of ['clay','blueprint','sketch','neon','pastel','model']){
      await evaluate(`document.querySelector('.stage-wrap').dataset.skin=${JSON.stringify(skin)};floorplan.refreshPalette()`);
      await frames(); assert.deepEqual(await selected(), [desk.id], `${skin} retains desk selection`);
    }
    await state({build:.1,moving:true}); assert.deepEqual(await selected(), []);
    await state({build:1,moving:false}); assert.deepEqual(await selected(), [desk.id], 'rebuild retains source identity');
    await state({furniture:false}); assert.deepEqual(await selected(), []);
    await state({furniture:true}); assert.deepEqual(await selected(), [desk.id]);
    await key('Escape');
    await state({theta:-45,phi:35.26,wallHeight:1.5});
    const again=await target(), againPoint={x:again.x,y:again.y};
    await click(againPoint); await click(againPoint); assert.deepEqual(await selected(), [], 'click twice deselects');
    await click(againPoint); await click({x:30,y:150}); assert.deepEqual(await selected(), [], 'background click deselects');
    // A genuine D3 drag starting on a desk must not also select it.
    await mouse('mouseMoved',againPoint);await mouse('mousePressed',againPoint,{button:'left',buttons:1});
    await mouse('mouseMoved',{x:again.x+30,y:again.y+15},{button:'left',buttons:1});
    await mouse('mouseReleased',{x:again.x+30,y:again.y+15},{button:'left'});
    await frames(); assert.deepEqual(await selected(), [], 'pan gesture does not select');
    const tap = await target();
    await page.call('Input.dispatchTouchEvent', {type:'touchStart',touchPoints:[{x:tap.x,y:tap.y,id:1}]});
    await page.call('Input.dispatchTouchEvent', {type:'touchEnd',touchPoints:[]});
    await frames(); assert.deepEqual(await selected(), [tap.id], 'touch tap selects a desk');
    await key('Escape');
    const room = await evaluate(`(()=>{
      for(const node of document.querySelectorAll('.floors path')){
        const b=node.getBoundingClientRect();
        for(let u=1;u<6;u++)for(let v=1;v<6;v++){
          const x=b.left+b.width*u/6,y=b.top+b.height*v/6;
          if(document.elementFromPoint(x,y)===node)return {x,y};
        }
      }
      throw Error('no exposed room floor');
    })()`);
    await mouse('mouseMoved',room); await frames();
    assert.ok(await evaluate(`!document.querySelector('.tooltip').hidden&&document.querySelectorAll('.hover-fill path').length===1`), 'room hover remains available');
    const beforeRoom = await evaluate(`document.querySelector('.viewport').getAttribute('transform')`);
    await click(room);
    await evaluate(`new Promise(r=>setTimeout(r,850))`);
    assert.notEqual(await evaluate(`document.querySelector('.viewport').getAttribute('transform')`), beforeRoom, 'room click still zooms');
    results.push({floor:plan.title,desks:expected,coverage,interactions:'passed'});
    console.log(JSON.stringify(results.at(-1)));
  }
  assert.deepEqual(page.errors, [], 'no uncaught browser errors');
  await writeFile(output,JSON.stringify({browser:version.Browser,results},null,2)+'\n');
  console.log(`PASS: ${results.reduce((sum,r)=>sum+r.desks,0)} desks, all floors, mouse/keyboard/touch, skins, camera, rebuild, pan and rooms`);
} finally {
  if(targetId)await browser.call('Target.closeTarget',{targetId});
  page?.close();browser.close();
}
