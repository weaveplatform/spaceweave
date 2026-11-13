import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { SKINS, SKIN_TOKENS, DEFAULT_SKIN } from '../js/skins.js';
import { furniturePrisms, roundedRectPoly, polygonArea, signedArea, BLOCKY } from '../js/geometry.js';

const near = (a, b) => assert.ok(Math.abs(a - b) < 1e-9, `${a} vs ${b}`);
const css = readFileSync(new URL('../styles.css', import.meta.url), 'utf8');

// The body of the first rule whose selector is exactly `selector`.
function block(selector) {
  const at = css.indexOf(`${selector} {`);
  assert.ok(at >= 0, `no rule for ${selector}`);
  return css.slice(at, css.indexOf('\n}', at));
}

test('the default skin is in the catalogue and ids are unique', () => {
  assert.ok(SKINS.some((s) => s.id === DEFAULT_SKIN));
  assert.equal(new Set(SKINS.map((s) => s.id)).size, SKINS.length);
});

test('the base tokens define everything the Model skin needs', () => {
  const root = block(':root');
  for (const token of SKIN_TOKENS) assert.match(root, new RegExp(`--${token}:`), `:root is missing --${token}`);
});

test('every other skin sets every token, so none silently inherits Model colours', () => {
  for (const skin of SKINS.filter((s) => s.id !== DEFAULT_SKIN)) {
    const body = block(`.stage-wrap[data-skin="${skin.id}"]`);
    for (const token of SKIN_TOKENS) assert.match(body, new RegExp(`--${token}:`), `${skin.id} is missing --${token}`);
  }
});

test('the sort footprint is a small inscribed subset of the drawn one', () => {
  const tub = furniturePrisms({ type: 'tub', x: 0, y: 0, facing: 0 }, { corner: 0, facets: 2 })[0];
  assert.equal(tub.sortPoly.length, 8);
  assert.ok(tub.sortPoly.every((v) => tub.poly.includes(v)));
});

test('rounded rectangles stay convex and lose only their corners', () => {
  const poly = roundedRectPoly(0, 0, 1, 0.6, 30, 0.1, 8);
  const area = polygonArea(poly);
  const ideal = 1 * 0.6 - (4 - Math.PI) * 0.1 * 0.1;
  assert.ok(Math.abs(area - ideal) < 0.002, `${area} vs ${ideal}`);
  // The default is a single 45° chamfer: each corner loses a triangle of r²/2.
  near(polygonArea(roundedRectPoly(0, 0, 1, 0.6, 0, 0.1)), 0.6 - 4 * 0.005);
  assert.ok(signedArea(poly) > 0);
  // Convex: every turn has the same sign.
  for (let i = 0; i < poly.length; i++) {
    const [a, b, c] = [poly[i], poly[(i + 1) % poly.length], poly[(i + 2) % poly.length]];
    const cross = (b[0] - a[0]) * (c[1] - b[1]) - (b[1] - a[1]) * (c[0] - b[0]);
    assert.ok(cross >= -1e-12);
  }
});

test('smoother skins add facets and corners without changing the blocky default', () => {
  const tub = { type: 'tub', x: 1, y: 1, facing: 0 };
  assert.equal(furniturePrisms(tub)[0].poly.length, 12);
  assert.equal(furniturePrisms(tub, BLOCKY)[0].poly.length, 12);
  assert.equal(furniturePrisms(tub, { corner: 0.05, facets: 2 })[0].poly.length, 24);
  assert.equal(furniturePrisms(tub, { corner: 0.05, facets: 1.5 })[0].poly.length, 18);
  const chair = { type: 'chair', x: 1, y: 1, facing: 90 };
  assert.equal(furniturePrisms(chair)[1].poly.length, 4);
  assert.ok(furniturePrisms(chair, { corner: 0.05, facets: 1 })[1].poly.length > 4);
});
