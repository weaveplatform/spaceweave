import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { sniff, check, names, acceptAttr, refusesDrag, pastedName, checkFile, SNIFF_LEN } from '../js/formats.js';

// Shared with internal/convert/formats_test.go: both checks must give the
// same verdict, in the same words, on the same bytes.
const cases = JSON.parse(readFileSync(new URL('./sniff-cases.json', import.meta.url)));
const { formats } = cases;
const bytes = (s) => Uint8Array.from(s, (c) => c.charCodeAt(0));

test('the shared cases list the formats', () => {
  assert.ok(formats?.length >= 5, 'run go test ./internal/convert -run Shared -update');
});

for (const c of cases.sniff) {
  test(`sniff: ${c.name}`, () => {
    assert.deepEqual(sniff(bytes(c.head), formats), c.want);
  });
}

for (const c of cases.check) {
  test(`check: ${c.file}`, () => {
    const got = check(c.file, bytes(c.head), formats);
    if (c.want.ok) {
      assert.equal(got.ok, true, got.reason);
      assert.equal(got.format.id, c.want.id);
    } else {
      assert.deepEqual(got, { ok: false, reason: c.want.reason });
    }
  });
}

test('names and accept come from the list', () => {
  assert.equal(names(formats), 'PNG, JPEG, WebP, DXF, DWG or SVG');
  assert.equal(names(formats.slice(0, 1)), 'PNG');
  const accept = acceptAttr(formats).split(',');
  for (const want of ['.png', '.jpeg', '.dxf', '.dwg', '.svg', 'image/svg+xml', 'image/vnd.dwg']) assert.ok(accept.includes(want), want);
});

test('a drag is refused only when every type is known and not ours', () => {
  assert.equal(refusesDrag(['application/pdf'], formats), true);
  assert.equal(refusesDrag(['image/png', 'application/pdf'], formats), false);
  assert.equal(refusesDrag([''], formats), false, 'DXF often has no type');
  assert.equal(refusesDrag(['application/octet-stream'], formats), false);
  assert.equal(refusesDrag([], formats), false);
});

test('pasted files get a name with the right extension', () => {
  assert.equal(pastedName({ name: 'image.png', type: 'image/png' }, formats), 'Pasted plan.png');
  assert.equal(pastedName({ name: '', type: 'image/jpeg' }, formats), 'Pasted plan.jpg');
  assert.equal(pastedName({ name: 'office.dxf', type: '' }, formats), 'office.dxf');
  assert.equal(pastedName({ name: '', type: 'image/vnd.dwg' }, formats), 'Pasted plan.dwg');
  assert.equal(pastedName({ name: '', type: 'application/pdf' }, formats), 'Pasted file');
});

test('checkFile reads only the start of the file', async () => {
  const big = new Uint8Array(SNIFF_LEN * 4);
  big.set(bytes('  0\nSECTION\n'));
  let sliced = null;
  const file = { name: 'plan.dxf', slice: (a, b) => { sliced = [a, b]; return new Blob([big.subarray(a, b)]); } };
  const got = await checkFile(file, formats);
  assert.equal(got.ok, true);
  assert.deepEqual(sliced, [0, SNIFF_LEN]);
});
