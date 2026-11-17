import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  packageRows, ROLES, canAddSpaces, canChangePackage, spaceFormats, SPACE_FORMATS, spacePlanProblem,
} from '../js/pipeline.js';
import { check } from '../js/formats.js';
import { createApi, REQUEST_HEADER } from '../js/api.js';
import { loadCatalog } from '../js/plans/index.js';

test('a floor package lists the files beside the drawing, each with its role and link', () => {
  const entry = {
    id: 'f-1', dir: 'generated/f-1/', generated: true, status: 'draft', format: 'dwg',
    files: [
      { role: 'architecture', upload: 'original.dwg', sha256: 'a', name: 'Floor 02.dwg' },
      { role: 'spaces', upload: 'files/spaces-bbbbbb.dwg', sha256: 'b', name: 'Floor 02 space.dwg' },
      { role: 'reference', upload: 'files/reference-cccccc.png', sha256: 'c', name: 'print.png' },
      { role: 'later', upload: 'files/later-d.png', sha256: 'd', name: 'x.png' },
    ],
  };
  assert.deepEqual(packageRows(entry), [
    { sha: 'b', name: 'Floor 02 space.dwg', role: 'spaces', label: 'Space plan', href: 'generated/f-1/files/spaces-bbbbbb.dwg' },
    { sha: 'c', name: 'print.png', role: 'reference', label: 'Reference', href: 'generated/f-1/files/reference-cccccc.png' },
    { sha: 'd', name: 'x.png', role: 'later', label: 'later', href: 'generated/f-1/files/later-d.png' },
  ]);
  assert.deepEqual(packageRows({}), [], 'a plan from before packages has nothing beside its drawing');
  assert.deepEqual(packageRows(undefined), []);
  assert.equal(packageRows({ files: [{ role: 'spaces', upload: 'files/s.dwg' }] })[0].href, 'files/s.dwg');
  assert.equal(ROLES.architecture, 'Drawing');
});

test('a space plan joins a drawing that is not being traced or approved', () => {
  const plan = (status, format = 'dxf') => ({ generated: true, status, format });
  for (const status of ['queued', 'draft', 'failed']) assert.equal(canAddSpaces(plan(status)), true, status);
  for (const status of ['converting', 'tracing', 'approved']) {
    assert.equal(canAddSpaces(plan(status)), false, status);
    assert.equal(canChangePackage(plan(status)), false, status);
  }
  assert.equal(canAddSpaces(plan('draft', 'png')), false, 'an image has no coordinates to place it on');
  assert.equal(canChangePackage(plan('draft', 'png')), true, 'its references can still go');
  assert.equal(canAddSpaces({ status: 'draft', format: 'dxf' }), false, 'a built-in plan has no package');
  assert.equal(canAddSpaces(undefined), false);
  assert.equal(canChangePackage(undefined), false);
});

test('a space plan is checked against the formats it is read from', () => {
  const formats = [
    { id: 'png', label: 'PNG', exts: ['.png'], mime: ['image/png'] },
    { id: 'dxf', label: 'DXF', exts: ['.dxf'], mime: [] },
    { id: 'dwg', label: 'DWG', exts: ['.dwg'], mime: [] },
  ];
  assert.deepEqual(spaceFormats(formats).map((f) => f.id), ['dxf', 'dwg']);
  assert.deepEqual([...SPACE_FORMATS], ['dxf', 'dwg']);
  const pngBytes = new Uint8Array([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]);
  const dwgBytes = new TextEncoder().encode('AC1032\0\0\0\0\0\0');
  assert.equal(spacePlanProblem('spaces.png', check('spaces.png', pngBytes, formats)),
    'spaces.png is a PNG file; a space plan is read from a DXF or DWG drawing.');
  assert.match(spacePlanProblem('spaces.dwg', check('spaces.dwg', pngBytes, formats)), /Rename it to end in .png/);
  assert.equal(spacePlanProblem('spaces.dwg', check('spaces.dwg', dwgBytes, formats)), '');
});

function fakeFetch(status, body) {
  const calls = [];
  const impl = async (url, init) => {
    calls.push({ url, init });
    return { ok: status < 300, status, json: async () => body };
  };
  return { impl, calls };
}

test('the API client attaches and detaches package files with the request header', async () => {
  const { impl, calls } = fakeFetch(201, { plan: { id: 'a b' }, duplicate: false });
  const api = createApi(impl);
  const file = new File([new Uint8Array([65, 67])], 'Floor 02 space.dwg');
  assert.deepEqual(await api.attach('a b', file, 'spaces'), { plan: { id: 'a b' }, duplicate: false });
  assert.equal(calls[0].url, 'api/plans/a%20b/files');
  assert.equal(calls[0].init.method, 'POST');
  assert.equal(calls[0].init.headers[REQUEST_HEADER], '1');
  assert.equal(calls[0].init.body.get('role'), 'spaces');
  assert.equal(calls[0].init.body.get('file').name, 'Floor 02 space.dwg');
  await api.attach('a b', file, 'reference', 'renamed.dwg');
  assert.equal(calls[1].init.body.get('file').name, 'renamed.dwg');
  await api.detach('a b', 'ab/c');
  assert.equal(calls[2].url, 'api/plans/a%20b/files/ab%2Fc');
  assert.equal(calls[2].init.method, 'DELETE');
  assert.equal(calls[2].init.headers[REQUEST_HEADER], '1');
});

test('the catalogue hands each plan its package, empty for one from before packages', async () => {
  const files = [{ role: 'architecture', upload: 'original.dwg', sha256: 'a', name: 'F.dwg' }];
  const impl = async () => ({
    status: 200, ok: true, json: async () => ({ plans: [{ id: 'p-1', status: 'draft', files }, { id: 'p-2', status: 'draft' }] }),
  });
  const [withFiles, without] = await loadCatalog(impl);
  assert.deepEqual(withFiles.files, files);
  assert.deepEqual(without.files, []);
});
