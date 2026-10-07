// Called by the Go integration test against a local server with synthetic inputs.
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { createApiClient } from '../frontend/src/shared/api/client.ts';

const [baseUrl, inputDirectory] = process.argv.slice(2);
assert.ok(baseUrl && inputDirectory, 'Usage: node scripts/test-api-contract.mjs URL INPUT_DIRECTORY');
const api = createApiClient({ baseUrl });
assert.equal((await api.getHealth()).analysis_ready, false);
assert.equal(await api.getAnalysis(), null);
const files = Object.fromEntries(['nodes', 'edges', 'transactions'].map((name) => [
  name, new File([readFileSync(join(inputDirectory, name + '.parquet'))], name + '.parquet'),
]));
const upload = await api.uploadAnalysis(files);
const analysis = await api.getAnalysis();
assert.equal(analysis.analysis_id, upload.analysis_id);
assert.equal(analysis.nodes.length, 4);
assert.ok(analysis.nodes.some((node) => node.gid === '100000000000000001'));
assert.ok(analysis.nodes.some((node) => node.gid === '100000000000000002'));
const card = await api.getNodeCard('100000000000000001');
assert.equal(card.outgoing.length, 2);
assert.equal(card.incoming.length, 1);
for (const name of ['nodes_roles.csv', 'clusters.csv', 'top_nodes.csv']) {
  const result = await api.downloadExport(name, analysis.analysis_id);
  assert.ok(result.blob.size > 0);
}
await assert.rejects(api.askQuestion({
  analysis_id: analysis.analysis_id, question: 'Почему этот узел?',
  context_gids: ['100000000000000001'],
}), { code: 'AI_UNAVAILABLE', status: 503 });
await assert.rejects(api.uploadAnalysis({ ...files, nodes: new File(['invalid'], 'nodes.parquet') }), { code: 'INVALID_SCHEMA' });
assert.equal((await api.getAnalysis()).analysis_id, analysis.analysis_id);
console.log('Go HTTP server → real TypeScript API client: passed');
