import { createGraphSimulation } from '../frontend/src/features/graph/graphPhysics.ts';
import { cpus } from 'node:os';
console.log(JSON.stringify({ node: process.version, platform: process.platform, cpu: cpus()[0]?.model }));
for (const size of [1000, 5000, 10000]) {
  const nodes = Array.from({ length: size }, (_, i) => ({ id: String(9007199254741000n + BigInt(i)), radius: 6, cluster: Math.floor(i / 100) }));
  const links = nodes.slice(1).map((node, i) => ({ source: nodes[i].id, target: node.id }));
  const sim = createGraphSimulation(nodes, links);
  for (let i = 0; i < 5; i++) sim.step();
  const samples = [];
  for (let i = 0; i < 30; i++) {
    const start = performance.now(); sim.step(); samples.push(performance.now() - start);
  }
  samples.sort((a, b) => a - b);
  console.log(JSON.stringify({ nodes: size, edges: links.length, median_step_ms: +samples[15].toFixed(2), p95_step_ms: +samples[28].toFixed(2) }));
}
console.log('Measures layout CPU time only; excludes browser rendering, labels, input and GPU. This is not FPS.');
