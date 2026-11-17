// How the renderer paints a space's floor: a room solid by its use and
// finish, a zone as a tint of its use with a dashed edge (styles.css's
// .floors path.zone) and no tile grid. The scene itself needs a DOM and d3,
// so these are the pure parts it is built from.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { execFile } from 'node:child_process';
import { mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { promisify } from 'node:util';
import { isZone, spaceClass, spaceFill, spaceTip } from '../js/render.js';

const palette = {
  zone: { meet: '#a', work: '#b', social: '#c', service: '#d' },
  floor: { wood: '#w', tile: '#t' },
};
const mix = (a, b, t) => `${a}+${b}@${t}`;

test('a room is filled by its finish, or by its use', () => {
  const room = { zone: 'social', floor: 'carpet' };
  assert.equal(isZone(room), false);
  assert.equal(spaceClass(room), 'room');
  assert.equal(spaceFill(room, palette, mix), '#c');
  assert.equal(spaceFill({ ...room, floor: 'tile' }, palette, mix), '#t');
  assert.equal(spaceFill({ ...room, floor: 'wood' }, palette, mix), '#c+#w@0.45');
  assert.equal(spaceFill({ ...room, kind: 'room', floor: 'wood' }, palette, mix), '#c+#w@0.45');
});

test('a zone is a tint of its use whatever its finish', () => {
  const zone = { kind: 'zone', zone: 'work', floor: 'wood', area: 123.45 };
  assert.equal(isZone(zone), true);
  assert.equal(spaceClass(zone), 'zone');
  assert.equal(spaceFill(zone, palette, mix), '#b');
  assert.equal(spaceFill({ ...zone, floor: 'tile' }, palette, mix), '#b');
  assert.equal(spaceTip(zone), 'Work · open plan · 123.5 m²');
  assert.equal(spaceTip({ ...zone, kind: undefined }), 'Work · 123.5 m²');
});

test('the overlay tool draws a zone apart from a room', async () => {
  const dir = await mkdtemp(join(tmpdir(), 'overlay-'));
  try {
    const kit = new URL('../js/plans/kit.js', import.meta.url).href;
    await writeFile(join(dir, 'plan.mjs'), `
      export { THICKNESS } from '${kit}';
      export const SOURCE = { url: 'x.png', credit: 't', title: 't', pxPerMetre: 10, originPx: [0, 0], sizePx: [100, 100] };
      export const WIDTH = 10, DEPTH = 10;
      export const walls = [];
      export const rooms = [
        { id: 'r', name: 'Office', zone: 'work', floor: 'carpet', polygon: [[0, 0], [4, 0], [4, 4], [0, 4]] },
        { id: 'z', name: 'Open plan', kind: 'zone', zone: 'work', floor: 'carpet', polygon: [[4, 0], [10, 0], [10, 10], [4, 10]] },
      ];
      export const furniture = [];
    `);
    await writeFile(join(dir, 'x.png'), Buffer.from('89504e470d0a1a0a', 'hex'));
    const tool = fileURLToPath(new URL('../tools/overlay.mjs', import.meta.url));
    await promisify(execFile)(process.execPath, [tool, join(dir, 'plan.mjs'), join(dir, 'x.png'), join(dir, 'out.svg')]);
    const svg = await readFile(join(dir, 'out.svg'), 'utf8');
    assert.match(svg, /stroke="#7b3fe4" stroke-width="2" stroke-dasharray="6 4"/);
    assert.match(svg, /stroke="#e377c2" stroke-width="2" stroke-dasharray="2 4"/);
    assert.match(svg, /fill="#e377c2"[^>]*font-style="italic">Open plan</);
  } finally {
    await rm(dir, { recursive: true, force: true });
  }
});
