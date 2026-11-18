import { test } from 'node:test';
import assert from 'node:assert/strict';
import { orderSiblings } from '../js/render.js';

function layer(names) {
  const parent = {
    children: [], moves: 0,
    insertBefore(node, next) {
      this.children.splice(this.children.indexOf(node), 1);
      this.children.splice(next === null ? this.children.length : this.children.indexOf(next), 0, node);
      this.moves++;
    },
  };
  parent.children = names.map(name => ({
    name,
    get nextSibling() { return parent.children[parent.children.indexOf(this) + 1] ?? null; },
    compareDocumentPosition() { throw Error('must not query document-wide order'); },
  }));
  return parent;
}

function permutations(values) {
  if (values.length === 0) return [[]];
  return values.flatMap((value, i) => permutations(values.filter((_, j) => i !== j)).map(rest => [value, ...rest]));
}

test('all path permutations reach exact paint order using sibling checks', () => {
  for (const names of permutations(['wall', 'desk', 'chair', 'glass'])) {
    const parent = layer(['wall', 'desk', 'chair', 'glass']);
    const nodes = names.map(name => parent.children.find(node => node.name === name));
    orderSiblings(parent, nodes);
    assert.deepEqual(parent.children.map(node => node.name), names);
    const moved = parent.moves;
    orderSiblings(parent, nodes);
    assert.equal(parent.moves, moved, 'an already ordered layer must not move any nodes');
  }
});

test('empty and singleton path layers need no DOM moves', () => {
  for (const names of [[], ['desk']]) {
    const parent = layer(names);
    orderSiblings(parent, [...parent.children]);
    assert.equal(parent.moves, 0);
  }
});
