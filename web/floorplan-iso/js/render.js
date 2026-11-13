// D3 renderer: projects the prisms for the current view state, orders them,
// and joins the resulting faces to SVG paths. Everything is redrawn from state
// on each frame, so animations only need to tween numbers in the state.

import { makeView } from './projection.js';
import {
  wallPrisms, furniturePrisms, projectPrism, orderItems, centroid, polygonArea, prism, offsetPolygon,
} from './geometry.js';

const d3 = globalThis.d3;

const PX_PER_M = 40;
const CUT_HEIGHT = 0.3; // what the walls nearest the camera are cut down to
const PLINTH = 0.22;
const TILE = 0.3;

// Only big surfaces get pencil hatching: hatching every chair face doubles the
// path count for detail too small to read.
const HATCHED = new Set(['wall', 'cabinet', 'counter', 'appliance', 'door', 'plinth']);

const ZONE_NAMES = { meet: 'Meeting', work: 'Work', social: 'Social', service: 'Service' };

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

export function createScene({ svg: svgEl, tooltip: tooltipEl, styleRoot, plan, onStats }) {
  const svg = d3.select(svgEl);
  const tooltip = d3.select(tooltipEl);
  const W = plan.WIDTH;
  const D = plan.DEPTH;
  const outline = plan.OUTLINE ?? [[0, 0], [W, 0], [W, D], [0, D]];
  const opts = { thickness: plan.THICKNESS, width: W, depth: D, outline: plan.OUTLINE ?? null };
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

  const zoom = d3.zoom().scaleExtent([0.2, 12]).on('zoom', (e) => root.attr('transform', e.transform));
  svg.call(zoom).on('dblclick.zoom', null);

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
    furniture = plan.furniture.flatMap((item) => tag(furniturePrisms(item, shape), {
      kind: 'furniture', delay: 0.42 + 0.38 * distFromEntrance(item.x, item.y),
    }));
  }
  buildFurniture(palette.shape);
  const people = plan.furniture.filter((f) => f.label).map((f) => ({ name: f.label, x: f.x, y: f.y }));

  let wallCache = { H: null, solids: [], leaves: [] };
  function wallsFor(H) {
    if (wallCache.H !== H) {
      const { solids, leaves } = wallPrisms(plan.walls, H, opts);
      const delayed = (list) => tag(list, {}).map((p) => ({ ...p, delay: 0.38 * distFromEntrance(...p.c) }));
      wallCache = { H, solids: delayed(solids), leaves: delayed(leaves).map((p) => ({ ...p, delay: p.delay + 0.3 })) };
    }
    return wallCache;
  }

  // The slab follows the outer face of the exterior walls.
  const plinth = prism(offsetPolygon(outline, plan.THICKNESS.exterior / 2), -PLINTH, 0, { mat: 'plinth' });

  const rooms = plan.rooms.map((r) => ({ ...r, area: polygonArea(r.polygon), c: r.label ?? centroid(r.polygon) }));

  // Tile grid lines, clipped to each tiled rectangle.
  const tileLines = [];
  for (const area of [...plan.rooms, ...plan.floorPatches].filter((r) => r.floor === 'tile')) {
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

  const growth = (build, delay) => d3.easeCubicOut(Math.max(0, Math.min(1, (build - delay) / 0.22)));

  // ---- frame
  let hovered = null;
  let lastView = null;

  function render(state) {
    const t0 = performance.now();
    const view = makeView({ theta: state.theta, phi: state.phi, cx: W / 2, cy: D / 2 });
    lastView = view;
    const [vx, vy] = view.toViewer;
    const vlen = Math.hypot(vx, vy);
    const sinP = Math.sin((state.phi * Math.PI) / 180);
    const cosP = Math.cos((state.phi * Math.PI) / 180);

    // Ground shadow (skins with --shadow-opacity), base slab and floors
    const shadowPts = plinth.poly.map(([x, y]) => {
      const [sx, sy] = view.project(x, y, -PLINTH);
      return [sx + 0.3, sy + 0.55];
    });
    gShadow.selectAll('path').data([0]).join('path').attr('d', pathOf(shadowPts));

    const base = projectPrism(plinth, view);
    gBase.selectAll('path').data(base.faces, (f) => f.side).join('path')
      .attr('d', (f) => pathOf(f.pts.map((p) => [p[0], p[1]])))
      .attr('fill', (f) => (f.side === 'top' ? palette.plinth : shade('plinth', f, view)));

    const floorPts = (poly) => poly.map(([x, y]) => view.project(x, y, 0));
    gFloors.selectAll('path').data(rooms, (r) => r.id).join((enter) => enter.append('path')
      .on('pointerenter', (e, r) => { hovered = r; drawHover(); showTip(e, r); })
      .on('pointermove', (e, r) => showTip(e, r))
      .on('pointerleave', () => { hovered = null; drawHover(); tooltip.attr('hidden', true); })
      .on('click', (e, r) => zoomToRoom(r)))
      .attr('d', (r) => pathOf(floorPts(r.polygon)))
      .attr('fill', (r) => (r.floor === 'tile' ? palette.floor.tile : r.floor === 'wood' ? d3.interpolateRgb(palette.zone[r.zone], palette.floor.wood)(0.45) : palette.zone[r.zone]));

    gTiles.selectAll('path.patch').data(plan.floorPatches, (p) => p.id).join('path').attr('class', 'patch')
      .attr('d', (p) => pathOf(floorPts(p.polygon)))
      .attr('fill', palette.floor.tile);
    gTiles.selectAll('path.grid').data([0]).join('path').attr('class', 'grid')
      .attr('d', tileLines.map(([a, b]) => {
        const p = view.project(a[0], a[1], 0.003);
        const q = view.project(b[0], b[1], 0.003);
        return `M${fmt(p[0])},${fmt(p[1])}L${fmt(q[0])},${fmt(q[1])}`;
      }).join(''));

    // Solids
    const { solids, leaves } = wallsFor(state.wallHeight);
    let active = solids;
    if (state.doors) active = active.concat(leaves);
    if (state.furniture) active = active.concat(furniture);

    const items = [];
    for (const p of active) {
      if (!state.glass && p.mat === 'glass') continue;
      let z0 = p.z0;
      let z1 = p.z1;
      if (state.cutaway && p.outward && vlen > 1e-6 && (p.outward[0] * vx + p.outward[1] * vy) / vlen > 0.3) {
        z1 = Math.min(z1, CUT_HEIGHT);
        if (z0 >= z1) continue;
      }
      const g = growth(state.build, p.delay);
      if (g < 0.002) continue;
      z0 *= g;
      z1 *= g;
      const proj = projectPrism(p, view, z0, z1);
      const [, yr] = view.rotate(p.c[0], p.c[1]);
      items.push({
        p, poly: p.sortPoly ?? p.poly, kind: p.kind, z0e: z0, z1e: z1, bbox: proj.bbox, faces: proj.faces,
        key: yr * cosP + ((z0 + z1) / 2) * sinP,
      });
    }
    const ordered = orderItems(items, view);
    const faces = [];
    for (const it of ordered) {
      for (const f of it.faces) {
        const d = pathOf(f.pts);
        faces.push({ id: `${it.p.id}:${f.side}`, d, fill: shade(it.p.mat, f, view), mat: it.p.mat });
        // Pencil hatching is a second, line-only path over the coloured face,
        // so the watercolour shows through; denser on the side away from the light.
        if (palette.hatch && !state.moving && f.side !== 'top' && HATCHED.has(it.p.mat)) {
          const dark = view.screenNormalX(f.nx, f.ny) < 0;
          faces.push({ id: `${it.p.id}:${f.side}:h`, d, fill: `url(#hatch-${dark ? 'dark' : 'light'})`, mat: 'hatch' });
        }
      }
    }
    gSolids.selectAll('path').data(faces, (f) => f.id).join('path')
      .attr('d', (f) => f.d)
      .attr('fill', (f) => f.fill)
      .attr('class', (f) => f.mat)
      .order();

    // Labels
    const roomLabels = state.labels ? rooms : [];
    gLabels.selectAll('text.room').data(roomLabels, (r) => r.id).join('text').attr('class', 'room')
      .attr('transform', (r) => {
        const [sx, sy] = view.project(r.c[0], r.c[1], 0.05);
        return `translate(${fmt(sx)},${fmt(sy)})`;
      })
      .text((r) => r.name);
    const peopleLabels = state.people && state.furniture ? people : [];
    const lift = 1.2 * growth(state.build, 0.75);
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

    drawHover();
    onStats?.({ prisms: items.length, faces: faces.length, ms: performance.now() - t0 });
  }

  function drawHover() {
    const view = lastView;
    const data = hovered && view ? [hovered] : [];
    for (const g of [gHoverFill, gOverlay]) {
      g.selectAll('path').data(data, (r) => r.id).join('path')
        .attr('d', (r) => pathOf(r.polygon.map(([x, y]) => view.project(x, y, 0.01))));
    }
  }

  function showTip(e, r) {
    const [x, y] = d3.pointer(e, svgEl.parentElement);
    tooltip.attr('hidden', null)
      .style('left', `${x + 14}px`).style('top', `${y + 14}px`)
      .html(`<strong>${r.name}</strong><span>${ZONE_NAMES[r.zone]} · ${r.area.toFixed(1)} m²</span>`);
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
    for (const [x, y] of outline) {
      pts.push(view.project(x, y, -PLINTH), view.project(x, y, state.wallHeight));
    }
    const t = transformToFit(screenBounds(pts));
    (duration ? svg.transition('camera').duration(duration).ease(d3.easeCubicInOut) : svg).call(zoom.transform, t);
  }

  function zoomToRoom(r) {
    const view = lastView;
    const pts = r.polygon.flatMap(([x, y]) => [view.project(x, y, 0), view.project(x, y, 1.2)]);
    svg.transition('camera').duration(750).ease(d3.easeCubicInOut).call(zoom.transform, transformToFit(screenBounds(pts), 0.6));
  }

  function refreshPalette() {
    palette = readPalette(styleRoot);
    shadeCache = new Map();
    buildFurniture(palette.shape);
  }

  function destroy() {
    svg.on('.zoom', null).interrupt('camera');
    svg.selectAll('*').remove();
    tooltip.attr('hidden', true);
  }

  return { render, fit, refreshPalette, destroy, svg };
}
