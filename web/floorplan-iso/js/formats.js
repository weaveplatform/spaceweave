// The page's check on a file before it's uploaded: is it a format the tracer
// takes, and does its content agree with its name? The list of formats comes
// from the server (/api/info); the rules for recognising content mirror
// internal/convert/formats.go and must agree with it, or the page would
// refuse files the server accepts, or upload ones it then refuses. The
// server checks again either way.

// How much of a file sniff needs; convert.SniffLen.
export const SNIFF_LEN = 8 * 1024;

const ascii = (bytes, from = 0, to = bytes.length) => String.fromCharCode(...bytes.subarray(from, Math.min(to, bytes.length)));
const startsWith = (bytes, s) => bytes.length >= s.length && ascii(bytes, 0, s.length) === s;
const isDigit = (c) => c >= 0x30 && c <= 0x39;

// AutoCAD's "AC10nn" version string, or the older "AC1.40" style: any DWG,
// whether or not its version is one the tracer reads.
const isDWG = (b) => b.length >= 6 && startsWith(b, 'AC') && (b[2] === 0x31 || b[2] === 0x32)
  && (isDigit(b[3]) || b[3] === 0x2e) && isDigit(b[4]) && isDigit(b[5]);

// The AutoCAD release behind each DWG version string, and whether the
// tracer reads it: convert.dwgReleases, which must agree.
const DWG_RELEASES = {
  'AC1.40': ['AutoCAD Release 1.4'],
  'AC1.50': ['AutoCAD Release 2.05'],
  'AC2.10': ['AutoCAD Release 2.10'],
  AC1001: ['AutoCAD Release 2.2'],
  AC1002: ['AutoCAD Release 2.5'],
  AC1003: ['AutoCAD Release 2.6'],
  AC1004: ['AutoCAD Release 9'],
  AC1006: ['AutoCAD Release 10'],
  AC1009: ['AutoCAD Release 11/12'],
  AC1012: ['AutoCAD Release 13'],
  AC1014: ['AutoCAD Release 14'],
  AC1015: ['AutoCAD 2000', true],
  AC1018: ['AutoCAD 2004', true],
  AC1021: ['AutoCAD 2007'],
  AC1024: ['AutoCAD 2010', true],
  AC1027: ['AutoCAD 2013', true],
  AC1032: ['AutoCAD 2018', true],
};

// A DWG of a version the tracer reads, or why one of another is refused.
function sniffDWG(bytes) {
  const code = ascii(bytes, 0, 6);
  const [name = `AutoCAD ${code}`, read = false] = DWG_RELEASES[code] ?? [];
  if (read) return { id: 'dwg' };
  return { refused: `This DWG is ${name} format, which isn't supported. Save it as DWG 2018 or 2013, or as DXF.` };
}

// Formats people try that the tracer doesn't read, with what to do instead.
const OTHER = [
  ['PDF', 'Export the page as PNG, or the drawing as DXF, DWG or SVG.', (b) => {
    const i = ascii(b, 0, 1024 + 5).indexOf('%PDF-');
    return i >= 0 && i < 1024;
  }],
  ['DWF', 'Export the drawing as DWG or DXF instead.', (b) => startsWith(b, '(DWF V')],
  ['TIFF', 'Export it as PNG.', (b) => startsWith(b, 'II*\x00') || startsWith(b, 'MM\x00*')],
  ['GIF', 'Export it as PNG.', (b) => startsWith(b, 'GIF8')],
  ['BMP', 'Export it as PNG.', (b) => startsWith(b, 'BM')],
  ['HEIC', 'Export it as JPEG.', (b) => b.length >= 12 && ascii(b, 4, 8) === 'ftyp' && ascii(b, 8, 11) === 'hei'],
  ['ZIP archive', 'Unzip it and upload the drawing inside.', (b) => startsWith(b, 'PK\x03\x04')],
];

const text = (bytes) => new TextDecoder().decode(bytes).replace(/^﻿/, '');

function asciiDXF(bytes) {
  const lines = text(bytes).split('\n');
  for (let i = 0; i + 1 < lines.length; i += 2) {
    const code = lines[i].trim();
    if (code === '999') continue;
    return code === '0' && lines[i + 1].trim() === 'SECTION';
  }
  return false;
}

function isSVG(bytes) {
  let s = text(bytes).trim();
  const after = (end) => {
    const i = s.indexOf(end);
    s = i < 0 ? '' : s.slice(i + end.length).trim();
  };
  for (;;) {
    if (s.startsWith('<?')) after('?>');
    else if (s.startsWith('<!--')) after('-->');
    else if (s.startsWith('<!')) {
      const b = s.indexOf('[');
      if (b >= 0 && b < s.indexOf('>')) after(']');
      after('>');
    } else return /^<svg[\s>/]/.test(s);
    if (s === '') return false;
  }
}

/**
 * Identifies a file from its first bytes.
 * @returns {{id: string} | {refused: string}} a format id ('png', 'dxf', ...),
 *   or why the file is refused
 */
export function sniff(bytes, formats) {
  let id = null;
  if (startsWith(bytes, '\x89PNG\r\n\x1a\n')) id = 'png';
  else if (bytes[0] === 0xff && bytes[1] === 0xd8 && bytes[2] === 0xff) id = 'jpeg';
  else if (bytes.length >= 12 && ascii(bytes, 0, 4) === 'RIFF' && ascii(bytes, 8, 12) === 'WEBP') id = 'webp';
  else if (startsWith(bytes, 'AutoCAD Binary DXF\r\n\x1a\x00') || asciiDXF(bytes)) id = 'dxf';
  else if (isDWG(bytes)) return sniffDWG(bytes);
  else if (isSVG(bytes)) id = 'svg';
  if (id) return { id };
  for (const [name, advice, match] of OTHER) {
    if (match(bytes)) return { refused: `${name} files aren't supported. ${advice}` };
  }
  return { refused: `This isn't a file spaceweave reads. Use ${names(formats)}.` };
}

/** "PNG, JPEG, WebP, DXF, DWG or SVG" */
export function names(formats) {
  const labels = formats.map((f) => f.label);
  return labels.length < 2 ? labels.join('') : `${labels.slice(0, -1).join(', ')} or ${labels.at(-1)}`;
}

const extOf = (name) => {
  const i = name.lastIndexOf('.');
  return i > 0 ? name.slice(i).toLowerCase() : '';
};

// "an SVG", "a PNG": by how the acronym is spoken.
const article = (label) => (label === label.toUpperCase() && 'AEFHILMNORSX'.includes(label[0]) ? `an ${label}` : `a ${label}`);

/**
 * Checks a file's name against its first bytes; the same rules and wording
 * as convert.Check.
 * @returns {{ok: true, format: object} | {ok: false, reason: string}}
 */
export function check(name, bytes, formats) {
  const ext = extOf(name);
  const byName = formats.find((f) => f.exts.includes(ext));
  const byContent = sniff(bytes, formats);
  if (!byName) {
    if (!ext) return { ok: false, reason: `${name} has no file extension. Use ${names(formats)}.` };
    const actual = formats.find((f) => f.id === byContent.id);
    if (actual) return { ok: false, reason: `${name} is ${article(actual.label)} file. Rename it to end in ${actual.exts[0]}.` };
    // The content says what it is: "PDF files aren't supported" is more use
    // than ".pdf files aren't".
    if (!byContent.refused.startsWith("This isn't")) return { ok: false, reason: byContent.refused };
    return { ok: false, reason: `${ext} files aren't supported. Use ${names(formats)}.` };
  }
  if (byContent.refused) {
    // A DWG of a version we don't read: "plan.dwg doesn't look like a DWG
    // file" would be untrue.
    if (byName.id === 'dwg' && isDWG(bytes)) return { ok: false, reason: byContent.refused };
    return { ok: false, reason: `${name} doesn't look like ${article(byName.label)} file. ${byContent.refused}` };
  }
  if (byContent.id !== byName.id) {
    const actual = formats.find((f) => f.id === byContent.id);
    return {
      ok: false,
      reason: `${name} is named like ${article(byName.label)} file but is ${article(actual.label)} file. Rename it to end in ${actual.exts[0]}.`,
    };
  }
  return { ok: true, format: byName };
}

/** Reads the start of a File and checks it. */
export async function checkFile(file, formats) {
  const head = new Uint8Array(await file.slice(0, SNIFF_LEN).arrayBuffer());
  return check(file.name, head, formats);
}

/** The file picker's accept attribute. */
export const acceptAttr = (formats) => [...formats.flatMap((f) => f.exts), ...formats.flatMap((f) => f.mime)].join(',');

/**
 * Whether a drag can be refused before the drop, from the MIME types the
 * browser reveals. Only a type that is known and not ours refuses: DXF
 * often has no type at all, so an empty or generic type is let through to
 * the check after the drop.
 */
export function refusesDrag(types, formats) {
  const ours = new Set(formats.flatMap((f) => f.mime));
  const known = types.filter((t) => t && t !== 'application/octet-stream');
  return types.length > 0 && known.length === types.length && known.every((t) => !ours.has(t));
}

/** A name for a pasted file, which arrives as "image.png" whatever it is. */
export function pastedName(file, formats) {
  const f = formats.find((x) => x.mime.includes(file.type));
  if (file.name && file.name !== 'image.png' && extOf(file.name)) return file.name;
  return f ? `Pasted plan${f.exts[0]}` : file.name || 'Pasted file';
}
