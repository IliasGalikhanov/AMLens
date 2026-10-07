import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { stripTypeScriptTypes } from 'node:module';

// Vite resolves the production import without an extension; Node's TS loader requires one.
const source = readFileSync(new URL('./graphLayoutTask.ts', import.meta.url), 'utf8')
  .replace("from './graphPhysics'", `from '${new URL('./graphPhysics.ts', import.meta.url).href}'`);
const { createLayoutTask } = await import('data:text/javascript;base64,' + Buffer.from(stripTypeScriptTypes(source)).toString('base64'));
const input = (count, motion = true) => ({
  nodes: Array.from({ length: count }, (_, i) => ({ id: String(i), radius: 6, cluster: Math.floor(i / 50) })),
  edges: Array.from({ length: Math.max(0, count - 1) }, (_, i) => ({ source: String(i), target: String(i + 1) })),
  previous: [], forces: { repulsion: 1, center: 1, distance: 1 }, motion,
});
function scheduler() {
  const queue = new Set();
  return {
    schedule(callback) { queue.add(callback); return () => queue.delete(callback); },
    next() { const callback = queue.values().next().value; if (callback) { queue.delete(callback); callback(); } },
    get size() { return queue.size; },
  };
}

test('initial layout yields before taking physics steps and cancellation prevents stale frames', () => {
  const clock = scheduler(); const frames = [];
  const task = createLayoutTask(input(3000), (frame) => frames.push(frame), clock.schedule);
  assert.equal(frames.length, 1);
  assert.equal(frames[0].ids.length, 3000);
  assert.equal(frames[0].positions.length, 6000);
  assert.equal(clock.size, 1);
  task.dispose(); clock.next();
  assert.equal(clock.size, 0);
  assert.equal(frames.length, 1);
  task.configure({ repulsion: 2, center: 2, distance: 2 }, true);
  task.pin('1', { x: 20, y: 20 });
  assert.equal(clock.size, 0);
});

test('reduced motion still computes a full final layout over yielding batches without intermediate animation', () => {
  const clock = scheduler(); const frames = [];
  const task = createLayoutTask(input(1200, false), (frame) => frames.push(frame), clock.schedule);
  let batches = 0;
  while (clock.size && batches < 500) { clock.next(); batches += 1; }
  assert.ok(batches > 1 && batches < 500);
  assert.equal(frames.length, 2);
  assert.equal(frames[1].settled, true);
  assert.equal(frames[1].positions.length, 2400);
  assert.ok(frames[1].positions.every(Number.isFinite));
  task.dispose();
});
