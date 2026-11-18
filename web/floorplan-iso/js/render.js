// D3 renderer: projects the prisms for the current view state, orders them,
// and joins the resulting faces to SVG paths. Everything is redrawn from state
// on each frame, so animations only need to tween numbers in the state.

import { makeView } from './projection.js';
import { deskTargets, createDeskHighlight } from './desks.js';
import { createRoomCutaway } from './cutaway.js';
import {
  wallPrisms, balustradePrisms, furniturePrisms, projectPrism, createItemOrderer, centroid, polygonArea, floorFootprint, projectFootprint,
} from './geometry.js';

const d3 = globalThis.d3;

const PX_PER_M = 40;
const PLINTH = 0.22;
const TILE = 0.3;

// Only big surfaces get pencil hatching: hatching every chair face doubles the
// path count for detail too small to read.
const HATCHED = new Set(['wall', 'cabinet', 'counter', 'appliance', 'door', 'plinth']);

const ZONE_NAMES = { meet: 'Meeting', work: 'Work', social: 'Social', service: 'Service' };

// A zone is open plan: a tint of its use's colour over the slab, edged with a
// dash and no walls, so it reads as an area rather than a room whose walls
// went missing. A room is filled solid by its use and floor finish.
export const isZone = (r) => r.kind === 'zone';
export function spaceFill(r, palette, mix) {
  if (isZone(r)) return palette.zone[r.zone];
  if (r.floor === 'tile') return palette.floor.tile;
  if (r.floor === 'wood') return mix(palette.zone[r.zone], palette.floor.wood, 0.45);
  return palette.zone[r.zone];
}
export const spaceClass = (r) => (isZone(r) ? 'zone' : 'room');
export const spaceTip = (r) => `${ZONE_NAMES[r.zone]}${isZone(r) ? ' · open plan' : ''} · ${r.area.toFixed(1)} m²`;

// nodes is the layer's complete child list, in paint order. D3's order uses
// compareDocumentPosition for every pair; that is costly across thousands of
// SVG furniture faces. Immediate sibling identity gives the same final order.
export function orderSiblings(parent, nodes) {
  let next = null;
  for (let i = nodes.length - 1; i >= 0; i--) {
    const node = nodes[i];
    if (node.nextSibling !== next) parent.insertBefore(node, next);
    next = node;
  }
}

const fmt = (v) => (Math.round(v * PX_PER_M * 10) / 10).toString();
const pathOf = (pts) => `M${pts.map((p) => `${fmt(p[0])},${fmt(p[1])}`).join('L')}Z`;

// Skins are CSS custom properties on the stage (see styles.css and skins.js),
// so the renderer reads its colours and knobs from there on every skin change.
function readPalette(el) {
  const css = getComputedStyle(el);
  const v = (name) => css.getPropertyValue(`--${name}`).trim();
  const num = (name, fallback) => {
    const n = parseFloat(v(name));
    return Number.isFinite(n) ? n : fallback;
  };
  return {
    shadeMin: num('shade-min', 0.1),
    shadeMax: num('shade-max', 0.65),
    hatch: num('hatch', 0) > 0,
    shape: { corner: num('corner', 0), facets: Math.max(1, num('facets', 1)) },
    wall: v('wall'), wallCap: v('wall-cap'), glass: v('glass'), door: v('door'),
    wood: v('wood'), metal: v('metal'), fabric: v('fabric'), fabricB: v('fabric-b'),
    cabinet: v('cabinet'), counter: v('counter'), steel: v('steel'), appliance: v('appliance'),
    board: v('board'), ceramic: v('ceramic'), pot: v('pot'), plant: v('plant'), plantLight: v('plant-light'),
    plinth: v('plinth'),
    zone: { meet: v('zone-meet'), work: v('zone-work'), social: v('zone-social'), service: v('zone-service') },
    floor: { wood: v('floor-wood'), tile: v('floor-tile') },
  };
}

export function createScene({ svg: svgEl, tooltip: tooltipEl, styleRoot, plan, onStats, onSelect }) {
  const svg = d3.select(svgEl);
  const tooltip = d3.select(tooltipEl);
  const W = plan.WIDTH;
  const D = plan.DEPTH;
  const footprint = floorFootprint(plan);
  const opts = { thickness: plan.THICKNESS, width: W, depth: D, outline: plan.OUTLINE ?? null, footprint: plan.FOOTPRINT ?? null };
  const voids = plan.VOIDS ?? [];
  const maxDist = Math.hypot(W, D);
  const distFromEntrance = (x, y) => Math.hypot(x - plan.ENTRANCE[0], y - plan.ENTRANCE[1]) / maxDist;

  let palette = readPalette(styleRoot);
  let shadeCache = new Map();

  // Filters and patterns that skins switch on through CSS (--solids-filter) or
  // the hatch flag. Their colours come from the same custom properties.
  svg.append('defs').html(`
    <filter id="sketch" x="-5%" y="-5%" width="110%" height="110%">
      <feTurbulence type="fractalNoise" baseFrequency="0.035" numOctaves="2" seed="7" result="noise"/>
      <feDisplacementMap in="SourceGraphic" in2="noise" scale="2.6" xChannelSelector="R" yChannelSelector="G"/>
    </filter>
    <filter id="glow" x="-10%" y="-10%" width="120%" height="120%">
      <feGaussianBlur in="SourceGraphic" stdDeviation="2.2" result="blur"/>
      <feMerge><feMergeNode in="blur"/><feMergeNode in="SourceGraphic"/></feMerge>
    </filter>
    <filter id="soft-shadow" x="-20%" y="-20%" width="140%" height="140%">
      <feGaussianBlur stdDeviation="14"/>
    </filter>
    <pattern id="hatch-light" width="6" height="6" patternUnits="userSpaceOnUse" patternTransform="rotate(55)">
      <line x1="0" y1="0" x2="0" y2="6" style="stroke: var(--edge); stroke-opacity: .35; stroke-width: .7"/>
    </pattern>
    <pattern id="hatch-dark" width="3.5" height="3.5" patternUnits="userSpaceOnUse" patternTransform="rotate(55)">
      <line x1="0" y1="0" x2="0" y2="3.5" style="stroke: var(--edge); stroke-opacity: .55; stroke-width: .8"/>
    </pattern>`);

  const root = svg.append('g').attr('class', 'viewport');
  const gShadow = root.append('g').attr('class', 'shadow');
  const gBase = root.append('g').attr('class', 'base');
  const gFloors = root.append('g').attr('class', 'floors');
  const gTiles = root.append('g').attr('class', 'tiles');
  // The hover tint sits under the solids so furniture stays legible; only the
  // outline is drawn on top.
  const gHoverFill = root.append('g').attr('class', 'hover-fill');
  const gSolids = root.append('g').attr('class', 'solids');
  const gOverlay = root.append('g').attr('class', 'overlay');
  const gLabels = root.append('g').attr('class', 'labels');
  tooltip.selectAll('*').remove();
  const tipTitle = tooltip.append('strong'), tipDetail = tooltip.append('span');
  let tipName, tipText;
  const targets = deskTargets(plan.furniture);
  const deskByIndex = new Map(targets.map(desk => [desk.index, desk]));
  const desks = createDeskHighlight({
    container: gSolids.node(), svg: svgEl, targets,
    show(event, desk, selected) {
      if (hovered) { hovered = null; drawHover(); }
      showTooltip(event, desk.label, selected ? 'Selected · Click again or Esc to clear' : 'Click to select · Esc clears selection');
    },
    hide() { tooltip.attr('hidden', true); },
    onSelect(desk) { onSelect?.(desk ? { type: 'desk', id: desk.id } : null); },
  });

  // Labels keep a constant on-screen size: CSS divides their font size by the
  // zoom scale, so a 46 m clinic fitted to the window stays as readable as a
  // 19 m office.
  const zoom = d3.zoom().scaleExtent([0.2, 12]).on('zoom', (e) => {
    root.attr('transform', e.transform);
    gLabels.style('--zoom-k', e.transform.k);
  });
  svg.call(zoom).on('dblclick.zoom', null);
  svg.on('click.selection', event => {
    if (!event.defaultPrevented && !event.target.closest('.floors path,[data-desk-id]')) onSelect?.(null);
  });

  // ---- static geometry, built once (walls rebuild when their height changes)
  let id = 0;
  const tag = (list, extra) => list.map((p) => {
    const c = centroid(p.poly);
    return { ...p, ...extra, id: id++, c };
  });

  // Furniture shape depends on the skin (rounded corners, facet count), so it is
  // rebuilt when a skin changes it.
  let furniture = [];
  let furnitureShape = null;
  function buildFurniture(shape) {
    if (furnitureShape && furnitureShape.corner === shape.corner && furnitureShape.facets === shape.facets) return;
    furnitureShape = shape;
    furniture = plan.furniture.flatMap((item, index) => tag(furniturePrisms(item, shape), {
      kind: 'furniture', delay: 0.42 + 0.38 * distFromEntrance(item.x, item.y),
      deskId: deskByIndex.get(index)?.id,
    }));
  }
  buildFurniture(palette.shape);
  const people = plan.furniture.filter((f) => f.label).map((f) => ({ name: f.label, x: f.x, y: f.y }));

  let wallCache = { H: null, solids: [], leaves: [] };
  function wallsFor(H) {
    if (wallCache.H !== H) {
      const { solids: walls, leaves } = wallPrisms(plan.walls, H, opts);
      const solids = walls.concat(balustradePrisms(voids, H));
      const delayed = (list) => tag(list, {}).map((p) => ({ ...p, delay: 0.38 * distFromEntrance(...p.c) }));
      wallCache = { H, solids: delayed(solids), leaves: delayed(leaves).map((p) => ({ ...p, delay: p.delay + 0.3 })) };
    }
    return wallCache;
  }

  const holes = footprint.flatMap((polygon) => polygon.slice(1));

  const rooms = plan.rooms.map((r) => ({ ...r, area: polygonArea(r.polygon), c: r.label ?? centroid(r.polygon) }));
  const roomsById = new Map(rooms.map(r => [r.id, r]));
  const roomCutaways = createRoomCutaway(plan.walls, plan.THICKNESS);
  let exteriorCuts = new Set(), exteriorAt = {};
  function exteriorCutaways(solids, view) {
    if (exteriorAt.solids !== solids || exteriorAt.theta !== view.theta || exteriorAt.phi !== view.phi) {
      const [vx, vy] = view.toViewer, length = Math.hypot(vx, vy);
      exteriorCuts = new Set(solids.filter(p => p.outward && length > 1e-6 &&
        (p.outward[0] * vx + p.outward[1] * vy) / length > 0.3).map(p => p.wallId));
      exteriorAt = { solids, theta: view.theta, phi: view.phi };
    }
    return exteriorCuts;
  }

  // Tile grid lines, clipped to each tiled rectangle. A zone is a tint, not a
  // finish, so it gets none.
  const tileLines = [];
  for (const area of [...plan.rooms.filter((r) => !isZone(r)), ...plan.floorPatches].filter((r) => r.floor === 'tile')) {
    const xs = area.polygon.map((p) => p[0]);
    const ys = area.polygon.map((p) => p[1]);
    const [x0, x1, y0, y1] = [Math.min(...xs), Math.max(...xs), Math.min(...ys), Math.max(...ys)];
    for (let x = x0 + TILE; x < x1 - 0.05; x += TILE) tileLines.push([[x, y0], [x, y1]]);
    for (let y = y0 + TILE; y < y1 - 0.05; y += TILE) tileLines.push([[x0, y], [x1, y]]);
  }

  // ---- shading
  function shade(mat, face, view) {
    if (face.side === 'top') return mat === 'wall' ? palette.wallCap : palette[mat];
    // Light from the camera's right: faces turned that way are brightest.
    const nx = view.screenNormalX(face.nx, face.ny);
    const { shadeMin, shadeMax } = palette;
    const k = Math.round((shadeMin + (shadeMax - shadeMin) * (1 - (nx + 1) / 2)) * 50) / 50;
    const key = `${mat}|${k}`;
    let c = shadeCache.get(key);
    if (!c) {
      c = d3.color(palette[mat]).darker(k).formatHex();
      shadeCache.set(key, c);
    }
    return c;
  }

  // Ground geometry stays fixed while the build changes prism heights.
  function drawGround(view) {
    // Ground shadow (skins with --shadow-opacity), base slab and floors
    const shadowPath = footprint.map(([ring]) => pathOf(ring.map(([x, y]) => {
      const [sx, sy] = view.project(x, y, -PLINTH);
      return [sx + 0.3, sy + 0.55];
    }))).join('');
    gShadow.selectAll('path').data([0]).join('path').attr('d', shadowPath);

    // A void is a hole through the slab's top. Under the hole, drawn first so
    // the slab hides all but what shows through it: the hole's sides at slab
    // level, then its floor a slab's depth down, darker, so it reads as depth.
    const voidAt = (z) => holes.map((v) => pathOf(v.map(([x, y]) => view.project(x, y, z)))).join('');
    const baseFaces = [
      ...(holes.length ? [
        { id: 'void-floor', d: voidAt(-PLINTH), fill: d3.color(palette.plinth).darker(1.2).formatHex() },
      ] : []),
      ...projectFootprint(footprint, view, -PLINTH, 0).map((f) => ({
        id: f.id,
        d: f.rings ? f.rings.map(pathOf).join('') : pathOf(f.pts),
        fill: f.side === 'top' ? palette.plinth : shade('plinth', f, view),
      })),
    ];
    gBase.selectAll('path').data(baseFaces, (f) => f.id).join('path')
      .attr('d', (f) => f.d)
      .attr('fill-rule', 'evenodd')
      .attr('fill', (f) => f.fill)
      .order();

    const floorPts = (poly) => poly.map(([x, y]) => view.project(x, y, 0));
    gFloors.selectAll('path').data(rooms, (r) => r.id).join((enter) => enter.append('path')
      .on('pointerenter', (e, r) => { hovered = r; drawHover(); showTip(e, r); })
      .on('pointermove', (e, r) => showTip(e, r))
      .on('pointerleave', () => { hovered = null; drawHover(); tooltip.attr('hidden', true); })
      .attr('role', 'button').attr('tabindex', '0').attr('aria-label', r => r.name).attr('aria-pressed', 'false')
      .on('click', (e, r) => onSelect?.({ type: 'room', id: r.id }))
      .on('keydown', (e, r) => {
        if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); if (!e.repeat) onSelect?.({ type: 'room', id: r.id }); }
      }))
      .attr('d', (r) => pathOf(floorPts(r.polygon)))
      .attr('class', spaceClass)
      .attr('fill', (r) => spaceFill(r, palette, (a, b, t) => d3.interpolateRgb(a, b)(t)));

    gTiles.selectAll('path.patch').data(plan.floorPatches, (p) => p.id).join('path').attr('class', 'patch')
      .attr('d', (p) => pathOf(floorPts(p.polygon)))
      .attr('fill', palette.floor.tile);
    gTiles.selectAll('path.grid').data([0]).join('path').attr('class', 'grid')
      .attr('d', tileLines.map(([a, b]) => {
        const p = view.project(a[0], a[1], 0.003);
        const q = view.project(b[0], b[1], 0.003);
        return `M${fmt(p[0])},${fmt(p[1])}L${fmt(q[0])},${fmt(q[1])}`;
      }).join(''));
  }

  const growth = (build, delay) => d3.easeCubicOut(Math.max(0, Math.min(1, (build - delay) / 0.22)));

  // ---- frame
  let hovered = null, selectedRoom = null, activeRoom = null;
  let lastView = null;
  const orderItems = createItemOrderer();
  const painted = new WeakMap();
  let lastFaceIds = [];
  // Most parts finish before the stagger ends. Reuse their projection and
  // painted paths while the remaining parts grow, releasing them on a turn.
  let projected = new WeakMap(), projectedAt = null;
  let groundPalette = null, lastLabels = null;
  let lastPeople = null, lastLift = null;
  let hoverAt = {};

  function render(state) {
    const t0 = performance.now();
    const view = makeView({ theta: state.theta, phi: state.phi, cx: W / 2, cy: D / 2 });
    const sameView = projectedAt?.theta === state.theta && projectedAt?.phi === state.phi;
    if (!sameView) {
      projected = new WeakMap();
      projectedAt = { theta: state.theta, phi: state.phi };
    }
    lastView = view;
    activeRoom = roomsById.get(state.activeRoom) ?? null;
    const sinP = Math.sin((state.phi * Math.PI) / 180);
    const cosP = Math.cos((state.phi * Math.PI) / 180);

    if (!sameView || groundPalette !== palette) {
      drawGround(view);
      groundPalette = palette;
    }

    // Solids
    const { solids, leaves } = wallsFor(state.wallHeight);
    const cutHeight = Math.max(0, Math.min(state.wallHeight, state.cutHeight ?? 0.3));
    let cutWalls = null;
    if (state.cutaway && cutHeight < state.wallHeight) {
      const mode = state.cutawayMode ?? (activeRoom ? 'room' : 'exterior');
      if (mode === 'room' && activeRoom) cutWalls = roomCutaways(activeRoom.polygon, view, state.wallHeight);
      else if (mode === 'exterior') cutWalls = exteriorCutaways(solids, view);
    }
    let active = solids;
    if (state.doors) active = active.concat(leaves);
    if (state.furniture) active = active.concat(furniture);

    const items = [];
    for (const p of active) {
      if (!state.glass && p.mat === 'glass') continue;
      let z0 = p.z0;
      let z1 = p.z1;
      if (p.wallId && cutWalls?.has(p.wallId)) {
        z1 = Math.min(z1, cutHeight);
        if (z0 >= z1) continue;
      }
      const g = growth(state.build, p.delay);
      if (g < 0.002) continue;
      z0 *= g;
      z1 *= g;
      let item = projected.get(p);
      if (!item || item.z0e !== z0 || item.z1e !== z1) {
        const proj = projectPrism(p, view, z0, z1);
        const [, yr] = view.rotate(p.c[0], p.c[1]);
        item = {
          p, poly: p.sortPoly ?? p.poly, kind: p.kind, z0e: z0, z1e: z1, bbox: proj.bbox, faces: proj.faces,
          key: yr * cosP + ((z0 + z1) / 2) * sinP,
        };
        projected.set(p, item);
      }
      items.push(item);
    }
    const ordered = orderItems(items, view);
    const faces = [];
    const hatch = palette.hatch && !state.moving;
    for (const it of ordered) {
      if (it.paintedPalette !== palette || it.paintedHatch !== hatch) {
        it.drawn = [];
        for (const f of it.faces) {
          const d = pathOf(f.pts);
          it.drawn.push({ id: `${it.p.id}:${f.side}`, d, fill: shade(it.p.mat, f, view), mat: it.p.mat,
            deskId: it.p.deskId, deskTarget: Boolean(it.p.deskId && it.p.mat === 'wood' && f.side === 'top'),
            wallId: it.p.wallId, z0: it.z0e, z1: it.z1e });
          // Pencil hatching is a second, line-only path over the coloured face.
          if (hatch && f.side !== 'top' && HATCHED.has(it.p.mat)) {
            const dark = view.screenNormalX(f.nx, f.ny) < 0;
            it.drawn.push({ id: `${it.p.id}:${f.side}:h`, d, fill: `url(#hatch-${dark ? 'dark' : 'light'})`, mat: 'hatch' });
          }
        }
        it.paintedPalette = palette; it.paintedHatch = hatch;
      }
      faces.push(...it.drawn);
    }
    const bound = gSolids.selectAll('path').data(faces, (f) => f.id);
    bound.exit().each(function () { desks.unbind(this); }).remove();
    const paths = bound.enter().append('path').merge(bound);
    paths.each(function (f) {
      const previous = painted.get(this);
      if (previous?.d !== f.d) this.setAttribute('d', f.d);
      if (previous?.fill !== f.fill) this.setAttribute('fill', f.fill);
      if (previous?.mat !== f.mat) this.setAttribute('class', f.mat);
      if (f.deskId || previous?.deskId) {
        if (previous?.deskId !== f.deskId || previous?.deskTarget !== f.deskTarget || previous?.mat !== f.mat) desks.bind(this, f);
        else if (previous?.fill !== f.fill) desks.refresh(this, f);
      }
      painted.set(this, f);
    });
    // D3's join orders the DOM, and an explicit order did it a second time.
    // Growth usually keeps the same face order; don't walk thousands of DOM
    // siblings or invalidate paths that already have their final attributes.
    if (faces.length !== lastFaceIds.length || faces.some((f, i) => f.id !== lastFaceIds[i])) {
      orderSiblings(gSolids.node(), paths.nodes());
      lastFaceIds = faces.map((f) => f.id);
    }

    // Labels
    if (!sameView || lastLabels !== state.labels) {
      const roomLabels = state.labels ? rooms : [];
      gLabels.selectAll('text.room').data(roomLabels, (r) => r.id).join('text')
        .attr('class', (r) => `room${isZone(r) ? ' zone' : ''}`)
        .attr('transform', (r) => {
          const [sx, sy] = view.project(r.c[0], r.c[1], 0.05);
          return `translate(${fmt(sx)},${fmt(sy)})`;
        })
        .text((r) => r.name);
      lastLabels = state.labels;
    }
    const showPeople = state.people && state.furniture;
    const lift = 1.2 * growth(state.build, 0.75);
    if (!sameView || lastPeople !== showPeople || lastLift !== lift) {
      const peopleLabels = showPeople ? people : [];
      gLabels.selectAll('g.person').data(peopleLabels, (p) => p.name).join((enter) => {
        const g = enter.append('g').attr('class', 'person');
        g.append('line');
        g.append('text');
        return g;
      })
        .attr('transform', (p) => {
          const [sx, sy] = view.project(p.x, p.y, lift);
          return `translate(${fmt(sx)},${fmt(sy)})`;
        })
        .attr('opacity', lift > 0.1 ? 1 : 0)
        .call((g) => g.select('line').attr('y2', fmt(lift * Math.cos((state.phi * Math.PI) / 180) * 0.62)))
        .call((g) => g.select('text').text((p) => p.name));
      lastPeople = showPeople; lastLift = lift;
    }

    drawHover();
    onStats?.({ prisms: items.length, faces: faces.length, ms: performance.now() - t0 });
  }

  function drawHover() {
    const view = lastView;
    const room = hovered ?? selectedRoom ?? activeRoom;
    if (hoverAt.room === room && hoverAt.theta === view?.theta && hoverAt.phi === view?.phi) return;
    hoverAt = { room, theta: view?.theta, phi: view?.phi };
    const data = room && view ? [room] : [];
    for (const g of [gHoverFill, gOverlay]) {
      g.selectAll('path').data(data, (r) => r.id).join('path')
        .attr('d', (r) => pathOf(r.polygon.map(([x, y]) => view.project(x, y, 0.01))));
    }
  }

  function showTip(e, r) {
    showTooltip(e, r.name, spaceTip(r));
  }

  function showTooltip(e, title, detail) {
    let x, y;
    if (Number.isFinite(e.clientX) && Number.isFinite(e.clientY)) [x, y] = d3.pointer(e, svgEl.parentElement);
    else {
      const box = e.target.getBoundingClientRect(), parent = svgEl.parentElement.getBoundingClientRect();
      x = box.left + box.width / 2 - parent.left;
      y = box.top - parent.top;
    }
    if (tipName !== title) { tipTitle.text(title); tipName = title; }
    if (tipText !== detail) { tipDetail.text(detail); tipText = detail; }
    tooltip.attr('hidden', null)
      .style('left', `${x + 14}px`).style('top', `${y + 14}px`);
  }

  // ---- camera
  function size() {
    const { width, height } = svgEl.getBoundingClientRect();
    return [width || 800, height || 600];
  }

  function transformToFit(bounds, pad = 0.9) {
    const [w, h] = size();
    const [[x0, y0], [x1, y1]] = bounds;
    const k = Math.min((w * pad) / (x1 - x0), (h * pad) / (y1 - y0));
    return d3.zoomIdentity.translate(w / 2, h / 2).scale(k).translate(-(x0 + x1) / 2, -(y0 + y1) / 2);
  }

  function screenBounds(pts) {
    const xs = pts.map((p) => p[0] * PX_PER_M);
    const ys = pts.map((p) => p[1] * PX_PER_M);
    return [[Math.min(...xs), Math.min(...ys)], [Math.max(...xs), Math.max(...ys)]];
  }

  function fit(state, duration = 0) {
    const view = makeView({ theta: state.theta, phi: state.phi, cx: W / 2, cy: D / 2 });
    const pts = [];
    for (const [x, y] of [...footprint.flat(2), ...plan.walls.flatMap((w) => [w.from, w.to])]) {
      pts.push(view.project(x, y, -PLINTH), view.project(x, y, state.wallHeight));
    }
    const t = transformToFit(screenBounds(pts));
    (duration ? svg.transition('camera').duration(duration).ease(d3.easeCubicInOut) : svg).call(zoom.transform, t);
  }

  function focusObject(selection, state, duration = 750) {
    const view = makeView({ ...state, cx: W / 2, cy: D / 2 });
    const room = selection?.type === 'room' ? roomsById.get(selection.id) : null;
    const parts = selection?.type === 'desk' ? furniture.filter(p => p.deskId === selection.id) : [];
    const pts = room ? room.polygon.flatMap(([x, y]) => [view.project(x, y, 0), view.project(x, y, state.wallHeight)])
      : parts.flatMap(p => p.poly.flatMap(([x, y]) => [view.project(x, y, p.z0), view.project(x, y, p.z1)]));
    if (!pts.length) return;
    const transform = transformToFit(screenBounds(pts), room ? 0.6 : 0.25);
    (duration ? svg.transition('camera').duration(duration).ease(d3.easeCubicInOut) : svg).call(zoom.transform, transform);
  }

  function setSelection(selection) {
    const before = selectedRoom;
    selectedRoom = selection?.type === 'room' ? roomsById.get(selection.id) : null;
    desks.setSelected(selection?.type === 'desk' ? selection.id : null);
    if (before !== selectedRoom) {
      gFloors.selectAll('path').filter(r => r === before || r === selectedRoom)
        .attr('aria-pressed', r => String(r === selectedRoom));
      drawHover();
    }
  }

  function refreshPalette() {
    palette = readPalette(styleRoot);
    shadeCache = new Map();
    buildFurniture(palette.shape);
  }

  function destroy() {
    desks.destroy();
    svg.on('.zoom', null).on('.selection', null).interrupt('camera');
    svg.selectAll('*').remove();
    tooltip.attr('hidden', true);
  }

  return { render, fit, refreshPalette, destroy, svg, focusObject, setSelection };
}
