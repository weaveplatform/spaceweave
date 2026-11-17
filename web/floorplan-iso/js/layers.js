// The layers panel's logic: what report.json says about a drawing's layers,
// and how a role chosen for one becomes a rule in the client's profile
// (internal/autotrace/profile). Pure, so node can test it; workbench.js
// draws it.

// A profile's name is a file name on the server: it must match validName
// in internal/autotrace/profile/library.go.
export const PROFILE_NAME = /^[a-z0-9][a-z0-9_-]{0,63}$/;

// Built-in profiles can't be saved over (profile.BuiltinNames).
export const BUILTIN_PROFILES = ['uniclass', 'aia', 'cafm', 'generic'];

// A glob matching exactly one layer name. It must agree with EscapeGlob in
// internal/autotrace/profile/profile.go: a layer called "A*" would
// otherwise give every layer starting with A the role.
export function escapeGlob(name) {
  return name.replace(/[*?\\]/g, (c) => `\\${c}`);
}

// Where a layer's role came from, in words.
export function sourceLabel(source = '') {
  if (source === 'guess') return 'guessed';
  const [kind, name] = source.split(':');
  if (kind === 'profile') return `your profile “${name}”`;
  if (kind === 'builtin') return `built-in ${name}`;
  return source;
}

/**
 * The panel's rows: report.json's layers, in its order (by name), with what
 * the page needs to show and change each.
 * @param {object} report  report.json
 */
export function layerRows(report) {
  return (report?.layers ?? []).map((l) => ({
    name: l.name,
    entities: l.entities,
    role: l.role,
    source: l.source,
    guessed: l.source === 'guess',
    // A role from a client rule can be taken back; a built-in's can only be
    // overridden.
    mine: String(l.source).startsWith('profile:'),
  }));
}

// A rule the page wrote: one layer, by its exact name, and nothing else.
const pageRule = (rule, layer) => rule.layer === escapeGlob(layer)
  && Object.keys(rule).every((k) => ['layer', 'role'].includes(k));

/**
 * The profile with layer given role, or with the page's rule for it taken
 * out when role is '' (back to what the built-ins or a guess say). The
 * new rule goes first: a client's rules are tried in order, and the
 * reviewer's latest word on a layer should win over a broader pattern
 * written by hand further down.
 * @param {{description?: string, rules: object[]}} profile
 */
export function setLayerRole(profile, layer, role) {
  const rules = (profile?.rules ?? []).filter((r) => !pageRule(r, layer));
  if (role) rules.unshift({ role, layer: escapeGlob(layer) });
  return { ...profile, rules };
}

// A first guess at the client's profile name, from the plan's name.
export function suggestName(title = '') {
  const slug = title.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 40);
  return PROFILE_NAME.test(slug) && !BUILTIN_PROFILES.includes(slug) ? slug : 'client';
}

// Why a name can't be a client profile's, or '' when it can.
export function nameProblem(name) {
  if (!PROFILE_NAME.test(name)) return 'Use lower-case letters, digits, - and _ (up to 64).';
  if (BUILTIN_PROFILES.includes(name)) return `“${name}” is a built-in profile; choose another name.`;
  return '';
}
