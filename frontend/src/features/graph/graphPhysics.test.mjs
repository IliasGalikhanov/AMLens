import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createGraphSimulation } from './graphPhysics.ts';

const nodes = Array.from({ length: 30 }, (_, i) => ({ id: String(i), radius: 6, cluster: Math.floor(i / 10) }));
const links = nodes.slice(1).map((node) => ({ source: '0', target: node.id }));
const settle = (simulation) => { for (let i = 0; i < 1500; i++) if (!simulation.step()) return i; assert.fail('simulation did not sleep'); };

test('the graph settles with finite positions, including isolates and self links', () => {
  const simulation = createGraphSimulation(nodes, [...links, { source: '0', target: '0' }, { source: 'missing', target: '0' }]);
  settle(simulation);
  assert.equal(simulation.nodes.length, nodes.length);
  for (const node of simulation.nodes) assert.ok(Number.isFinite(node.x) && Number.isFinite(node.y));
  const isolated = createGraphSimulation([{ id: 'alone', radius: 9, cluster: 1 }], []);
  settle(isolated);
  assert.equal(createGraphSimulation([], []).step(), false);
});
test('dragging keeps the grabbed node under the pointer while moving neighbors', () => {
  const simulation = createGraphSimulation(nodes, links);
  settle(simulation);
  const neighbor = simulation.nodes.find((node) => node.id === '1');
  const old = { x: neighbor.x, y: neighbor.y };
  simulation.pin('0', { x: 400, y: 100 });
  for (let i = 0; i < 50; i++) simulation.step();
  const grabbed = simulation.nodes.find((node) => node.id === '0');
  assert.equal(grabbed.x, 400); assert.equal(grabbed.y, 100);
  assert.ok(Math.hypot(neighbor.x - old.x, neighbor.y - old.y) > 1);
  simulation.pin('0', null);
  settle(simulation);
  assert.notEqual(grabbed.x, 400);
});
test('repulsion and link distance change spacing, without changing IDs or links', () => {
  const pair = nodes.slice(0, 2);
  function distance(repulsion, length) {
    const simulation = createGraphSimulation(pair, [{ source: '0', target: '1' }]);
    simulation.configure({ repulsion, distance: length, center: 1 });
    settle(simulation);
    return Math.hypot(simulation.nodes[0].x - simulation.nodes[1].x, simulation.nodes[0].y - simulation.nodes[1].y);
  }
  assert.ok(distance(2, 1) > distance(0.5, 1));
  assert.ok(distance(1, 2) > distance(1, 0.5));
});
test('coincident nodes separate and existing positions survive graph replacement', () => {
  const previous = new Map(nodes.map((node) => [node.id, { x: 0, y: 0 }]));
  const simulation = createGraphSimulation(nodes, links, previous);
  assert.ok(simulation.nodes.every((node) => node.x === 0 && node.y === 0));
  settle(simulation);
  assert.ok(new Set(simulation.nodes.map((node) => node.x + ',' + node.y)).size === nodes.length);
});

test('a complete 8000-client network uses subquadratic repulsion and preserves every client and spring', () => {
  const count = 8000;
  const input = Array.from({ length: count }, (_, i) => ({ id: `92233720368547${String(i).padStart(5, '0')}`, radius: 6, cluster: i % 40 }));
  const edges = input.slice(1, 7600).map((node, i) => ({ source: input[i].id, target: node.id }));
  edges.push({ source: input[0].id, target: input[0].id });
  const started = performance.now();
  const simulation = createGraphSimulation(input, edges);
  for (let step = 0; step < 8; step += 1) {
    simulation.step();
    assert.ok(simulation.statistics().repulsionVisits < count * 250,
      `Barnes–Hut visits ${simulation.statistics().repulsionVisits} must be far below ${count * (count - 1) / 2} pairs`);
  }
  assert.equal(simulation.statistics().springs, 7599);
  assert.deepEqual(new Set(simulation.nodes.map((node) => node.id)), new Set(input.map((node) => node.id)));
  assert.ok(simulation.nodes.every((node) => Number.isFinite(node.x) && Number.isFinite(node.y)));
  // A generous guard catches accidental all-pairs regressions on slow CI, while visit counts verify complexity.
  assert.ok(performance.now() - started < 10000);
});

test('large coincident graphs avoid a degenerate quadratic leaf and still separate nodes', () => {
  const input = Array.from({ length: 2500 }, (_, i) => ({ id: String(i), radius: 6, cluster: 0 }));
  const previous = new Map(input.map((node) => [node.id, { x: 0, y: 0 }]));
  const simulation = createGraphSimulation(input, [], previous);
  simulation.step();
  assert.ok(simulation.statistics().repulsionVisits <= input.length * 2);
  assert.equal(new Set(simulation.nodes.map((node) => `${node.x},${node.y}`)).size, input.length);
  simulation.pin('1000', { x: 700, y: -600 });
  for (let i = 0; i < 10; i += 1) simulation.step();
  assert.equal(simulation.nodes.find((node) => node.id === '1000').x, 700);
  assert.equal(simulation.nodes.find((node) => node.id === '1000').y, -600);
});
