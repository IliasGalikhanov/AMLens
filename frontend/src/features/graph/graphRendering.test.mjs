import { test } from 'node:test';
import assert from 'node:assert/strict';
import cytoscape from 'cytoscape';
import { createGraphFocus, supportsGraphWebGL } from './graphRendering.ts';

test('WebGL capability failure retains Canvas and releases successful probe', () => {
  assert.equal(supportsGraphWebGL(() => ({ getContext: () => null })), false);
  assert.equal(supportsGraphWebGL(() => { throw new Error('GPU unavailable'); }), false);
  let released = false;
  const gl = { isContextLost: () => false, MAX_TEXTURE_SIZE: 123, getParameter: () => 4096,
    getExtension: () => ({ loseContext: () => { released = true; } }) };
  assert.equal(supportsGraphWebGL(() => ({ getContext: () => gl })), true);
  assert.equal(released, true);
  assert.equal(supportsGraphWebGL(() => ({ getContext: () => ({ ...gl, isContextLost: () => true }) })), false);
});

test('incremental focus matches full recomputation through hover, selection and missing IDs', () => {
  const cy = cytoscape({ headless: true, elements: [
    ...['a', 'b', 'c', 'd'].map(id => ({ data: { id } })),
    { data: { id: 'ab', source: 'a', target: 'b' } },
    { data: { id: 'bc', source: 'b', target: 'c' } },
  ] });
  const focus = createGraphFocus(cy);
  for (const [selected, hovered] of [[null,null], ['a',null], ['a','c'], ['a','b'], ['d','c'], ['d',null], ['missing',null], [null,'a'], [null,null]]) {
    focus(selected, hovered);
    const actualFocus = cy.getElementById(hovered ?? selected ?? '');
    const neighborhood = actualFocus.neighborhood();
    cy.elements().forEach(ele => {
      assert.equal(ele.hasClass('is-selected'), ele.id() === selected);
      assert.equal(ele.hasClass('is-hovered'), ele.id() === hovered);
      assert.equal(ele.hasClass('is-neighbor'), ele.isNode() && neighborhood.contains(ele));
      assert.equal(ele.hasClass('is-path'), ele.isEdge() && neighborhood.contains(ele));
      const lit = ele.same(actualFocus) || neighborhood.contains(ele) || ele.id() === selected || ele.id() === hovered;
      assert.equal(ele.hasClass('is-dimmed'), actualFocus.length > 0 && !lit);
    });
  }
  assert.equal(cy.nodes().length, 4);
  assert.equal(cy.edges().length, 2);
  cy.destroy();
});
