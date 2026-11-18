#!/usr/bin/env node
// Attach to an isolated Chrome debugging port; open and close only our own tab.
// Outputs cut-wall counts and interaction results, never coordinates or drawings.
import assert from 'node:assert/strict';
import { connect } from './browser.mjs';
import { writeFile } from 'node:fs/promises';

const [site, output] = process.argv.slice(2);
if (!site || !output) {
  console.error('usage: cutaway-trial.mjs <site-url> <output.json>');
  process.exit(2);
}
const debug = `http://127.0.0.1:${process.env.SPACEWEAVE_CHROME_DEBUG_PORT || 9267}`;
const base = new URL(site);
const catalogue = await (await fetch(new URL('generated/catalog.json', base))).json();
const plans = catalogue.plans.filter(p => ['draft', 'approved'].includes(p.status));
assert.ok(plans.length, 'need at least one generated floor');


// Execute the real renderer and UI. The geometric cases live in cutaway.test.js;
// this trial checks that wall assemblies, controls and camera changes obey them.
async function exerciseFloor(id) {
  const {createRoomCutaway}=await import('./js/cutaway.js');
  const {makeView}=await import('./js/projection.js');
  const {polygonArea}=await import('./js/geometry.js');
  const plan=await (await fetch(`generated/${id}/plan.json`)).json();
  const check=(ok,message)=>{if(!ok)throw Error(message)};
  const frames=()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));
  const set=async values=>{Object.assign(floorplan.state,values);floorplan.invalidate();await frames()};
  await floorplan.loadPlan(id);
  d3.timerFlush();d3.select(document.body).interrupt('build');
  const room=plan.rooms.filter(r=>r.kind!=='zone'&&polygonArea(r.polygon)<150)
    .sort((a,b)=>polygonArea(b.polygon)-polygonArea(a.polygon))[0]??plan.rooms[0];
  check(room,'floor has a focus room');
  check(floorplan.state.activeRoom===null,'floor change clears room focus');
  const picker=document.querySelector('#room-focus');
  check(picker.options.length===plan.rooms.length+1,'every room is selectable');
  await set({build:1,moving:false,wallHeight:2.7,cutHeight:.3,cutaway:true,cutawayMode:'room',furniture:true});
  picker.value=room.id;picker.dispatchEvent(new Event('change'));
  check(floorplan.state.activeRoom===null,'room selection alone does not change cutaways');
  document.querySelector('#selection-focus').click();
  await new Promise(r=>setTimeout(r,850));await frames();
  check(floorplan.state.activeRoom===room.id,'picker activates its room');
  const changes=[];
  const observer=new MutationObserver(list=>changes.push(...list));
  observer.observe(document.querySelector('#stage'),{attributes:true,childList:true,subtree:true});
  floorplan.invalidate();await frames();observer.disconnect();
  check(changes.length===0,'settled room focus does not rewrite SVG');
  const stage=document.querySelector('#stage'),box=stage.getBoundingClientRect(),zoom=d3.zoomTransform(stage);
  const camera=makeView({...floorplan.state,cx:plan.WIDTH/2,cy:plan.DEPTH/2});
  for(const [x,y] of room.polygon)for(const z of [0,2.7]){
    const [sx,sy]=camera.project(x,y,z),[px,py]=zoom.apply([sx*40,sy*40]);
    check(px>=0&&px<=box.width&&py>=0&&py<=box.height,'room zoom fits full-height rear walls');
  }
  const expectedCuts=createRoomCutaway(plan.walls,plan.THICKNESS);
  const wallHeights=()=>{
    const map=new Map();
    for(const n of document.querySelectorAll('.solids path')){
      const f=n.__data__;if(f.wallId)map.set(f.wallId,Math.max(map.get(f.wallId)??0,f.z1));
    }
    return map;
  };
  const fingerprint=(onlyFurniture=false)=>JSON.stringify([...document.querySelectorAll('.solids path')]
    .filter(n=>!onlyFurniture||!n.__data__.wallId)
    .map(n=>[n.__data__.id,n.getAttribute('d'),n.getAttribute('fill')])
    .sort((a,b)=>a[0].localeCompare(b[0])));
  const views=[];
  for(const [theta,phi] of [[-45,35.26],[45,35.26],[135,35.26],[-135,35.26],[0,90]]){
    await set({theta,phi,cutaway:false});
    const original=wallHeights(),before=fingerprint(),furniture=fingerprint(true);
    const cuts=expectedCuts(room.polygon,makeView({theta,phi}),2.7);
    await set({cutaway:true,cutHeight:.3});
    const clipped=wallHeights();
    for(const [id,height] of original){
      if(cuts.has(id))check(!clipped.has(id)||clipped.get(id)<=.300001,'cut walls and doors stay below the cut height');
      else check(Math.abs(clipped.get(id)-height)<1e-6,'nonblocking and rear walls keep their height');
    }
    check(fingerprint(true)===furniture,'furniture and guardrails are unaffected');
    await set({cutHeight:0});
    const hidden=wallHeights();
    for(const id of cuts)check(!hidden.has(id),'zero height hides the entire wall assembly');
    check(fingerprint(true)===furniture,'hidden walls do not change furniture');
    await set({cutHeight:3});check(fingerprint()===before,'cut height cannot raise walls');
    await set({cutHeight:.3,cutaway:false});check(fingerprint()===before,'turning cutaways off restores the exact wall geometry');
    views.push({theta,phi,cutWalls:cuts.size,retainedWalls:[...original.keys()].filter(id=>!cuts.has(id)).length});
  }
  check(views.slice(0,4).some(v=>v.cutWalls>0),'room cutaway removes an obstruction');
  check(views.at(-1).cutWalls===0,'top-down Plan keeps all walls');
  await set({theta:-45,phi:35.26,cutaway:true,cutHeight:.3});
  const slider=document.querySelector('#cut-height');
  slider.value='0';slider.dispatchEvent(new Event('input'));
  await frames();check(floorplan.state.cutHeight===0&&document.querySelector('#cut-height-out').textContent==='Hidden','cut slider supports hiding');
  const toggle=document.querySelector('#cutaway-mode');
  toggle.value='off';toggle.dispatchEvent(new Event('change'));
  await frames();check(floorplan.state.cutaway===false,'checkbox disables cutaways');
  const height=document.querySelector('#wall-height');
  height.value='1.2';height.dispatchEvent(new Event('input'));await frames();
  check(floorplan.state.wallHeight===1.2&&+slider.max===1.2,'main wall height remains independent and bounds the cut height');
  toggle.value='room';toggle.dispatchEvent(new Event('change'));
  document.querySelector('#reset').click();await new Promise(r=>setTimeout(r,650));
  check(floorplan.state.activeRoom===room.id,'Fit preserves the cutaway target');
  document.querySelector('#whole-floor').click();await new Promise(r=>setTimeout(r,650));
  check(floorplan.state.activeRoom===null,'Whole floor clears room focus');
  document.querySelector('#selection-clear').click();
  return {rooms:plan.rooms.length,views};
}

const version=await(await fetch(`${debug}/json/version`)).json();
const browser=await connect(version.webSocketDebuggerUrl);
let page,targetId;
try{
  ({targetId}=await browser.call('Target.createTarget',{url:'about:blank'}));
  const tabs=await(await fetch(`${debug}/json/list`)).json();
  page=await connect(tabs.find(t=>t.id===targetId).webSocketDebuggerUrl);
  const evaluate=async expression=>{
    const r=await page.call('Runtime.evaluate',{expression,awaitPromise:true,returnByValue:true});
    if(r.exceptionDetails)throw Error(JSON.stringify(r.exceptionDetails));
    return r.result.value;
  };
  await page.call('Page.enable');await page.call('Runtime.enable');
  await page.call('Emulation.setDeviceMetricsOverride',{width:1440,height:1000,deviceScaleFactor:1,mobile:false});
  const url=new URL(base);url.searchParams.set('plan',plans[0].id);url.searchParams.set('skin','model');
  await page.call('Page.navigate',{url:url.href});
  let ready=false;
  for(let i=0;i<150;i++){
    ready=await evaluate('Boolean(globalThis.floorplan)&&floorplan.state.build>=1&&!floorplan.state.moving');
    if(ready)break;await new Promise(r=>setTimeout(r,100));
  }
  assert.ok(ready,'initial floor settles');
  const results=[];
  for(const plan of plans){
    const row={floor:plan.title,...await evaluate(`(${exerciseFloor})(${JSON.stringify(plan.id)})`)};
    // Actual hit testing: clicking an exposed room must activate that room.
    const point=await evaluate(`(()=>{
      for(const n of document.querySelectorAll('.floors path')){
        const b=n.getBoundingClientRect();
        for(let u=1;u<6;u++)for(let v=1;v<6;v++){
          const x=b.x+b.width*u/6,y=b.y+b.height*v/6;
          if(document.elementFromPoint(x,y)===n)return {x,y,id:n.__data__.id};
        }
      }throw Error('no exposed room floor');
    })()`);
    const {x,y}=point;
    await page.call('Input.dispatchMouseEvent',{type:'mouseMoved',x,y});
    await page.call('Input.dispatchMouseEvent',{type:'mousePressed',x,y,button:'left',clickCount:1});
    await page.call('Input.dispatchMouseEvent',{type:'mouseReleased',x,y,button:'left',clickCount:1});
    assert.equal(await evaluate('floorplan.selection.id'),point.id,'floor click selects the room');
    assert.equal(await evaluate('floorplan.state.activeRoom'),null,'room selection does not activate a cutaway');
    await evaluate(`document.querySelector('#selection-focus').click()`);
    assert.equal(await evaluate('floorplan.state.activeRoom'),point.id,'Focus activates room cutaway');
    assert.equal(await evaluate('document.querySelector("#room-focus").value'),point.id);
    results.push(row);console.log(JSON.stringify(row));
  }
  assert.deepEqual(page.errors,[],'no uncaught browser errors');
  await writeFile(output,JSON.stringify({browser:version.Browser,results},null,2)+'\n');
  console.log(`PASS: ${results.length} floors, 25 camera poses, wall assemblies, furniture, controls and restoration`);
}finally{
  if(targetId)await browser.call('Target.closeTarget',{targetId});
  page?.close();browser.close();
}
