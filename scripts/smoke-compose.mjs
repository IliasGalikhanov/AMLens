// Run from the repository root: node scripts/smoke-compose.mjs
// Only generated project names and their volumes are removed in finally.
import assert from 'node:assert/strict';
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const workspace = 'amlens-check-' + process.pid;
const demo = workspace + '-demo';
const temporary = mkdtempSync(join(tmpdir(), 'amlens-check-'));
const appPort = process.env.SMOKE_APP_PORT || '18080';
const demoPort = process.env.SMOKE_DEMO_PORT || '18081';
const env = { ...process.env, APP_PORT: appPort, DEMO_PORT: demoPort, OPENAI_API_KEY: '', OPENAI_MODEL: '', DOMAIN: 'demo.example.org' };
const base = 'http://127.0.0.1:' + appPort;
function docker(args, capture = false, allowFailure = false) {
  const result = spawnSync('docker', args, { cwd: root, env, encoding: 'utf8', stdio: capture ? 'pipe' : 'inherit', timeout: 900_000 });
  if (!allowFailure && (result.error || result.status !== 0)) throw new Error('Docker command failed: ' + args.join(' ') + '\n' + (result.stderr ?? '') + (result.error ?? ''));
  return result.stdout?.trim();
}
function compose(project, file, ...args) { return docker(['compose', '-p', project, '-f', file, ...args]); }
async function request(path, init = {}, expected = 200, url = base) {
  const response = await fetch(url + path, { ...init, signal: AbortSignal.timeout(180_000) });
  assert.equal(response.status, expected, path + ' HTTP status');
  return response;
}
async function json(path, init = {}, expected = 200, url = base) { return (await request(path, init, expected, url)).json(); }
async function upload(bad = false) {
  const form = new FormData();
  for (const name of ['nodes', 'edges', 'transactions']) {
    const data = bad && name === 'nodes' ? Buffer.from('invalid') : readFileSync(join(temporary, name + '.parquet'));
    form.append(name, new Blob([data]), name + '.parquet');
  }
  return json('/api/analyze', { method: 'POST', body: form }, bad ? 422 : 200);
}
const started = performance.now();
try {
  compose(workspace, 'compose.yaml', 'up', '--build', '-d', '--wait', '--wait-timeout', '180');
  assert.equal((await json('/api/health')).analysis_ready, false);
  await request('/api/analysis', {}, 404);
  assert.match(await (await request('/')).text(), /<div id="root">/);
  compose(workspace, 'compose.yaml', 'exec', '-T', 'backend', 'amlens', 'demo', '--output-dir', '/tmp/smoke-input');
  for (const name of ['nodes', 'edges', 'transactions']) {
    // docker cp cannot read a tmpfs mount on all Docker engines.
    const result = spawnSync('docker', ['compose', '-p', workspace, '-f', 'compose.yaml', 'exec', '-T', 'backend', 'cat', '/tmp/smoke-input/' + name + '.parquet'], { cwd: root, env });
    assert.equal(result.status, 0, 'read generated Parquet');
    writeFileSync(join(temporary, name + '.parquet'), result.stdout);
  }
  const analysisStart = performance.now();
  const first = await upload();
  console.log('Synthetic import: ' + Math.round(performance.now() - analysisStart) + ' ms');
  const analysis = await json('/api/analysis');
  assert.equal(analysis.analysis_id, first.analysis_id);
  assert.equal(analysis.nodes.length, 60);
  assert.equal(analysis.edges.length, 81);
  assert.equal(analysis.summary.n_transactions, 162);
  assert(analysis.nodes.some((node) => node.gid === '9007199254741001'));
  const card = await json('/api/nodes/9007199254741000');
  assert.equal(card.outgoing.length, 9);
  const exports = {};
  for (const name of ['nodes_roles.csv', 'clusters.csv', 'top_nodes.csv']) {
    const response = await request('/api/exports/' + name);
    assert.equal(response.headers.get('x-analysis-id'), first.analysis_id);
    exports[name] = await response.text();
    assert(exports[name].split('\n').length > 2);
  }
  const second = await upload();
  assert.notEqual(first.analysis_id, second.analysis_id);
  assert.equal((await json('/api/analyses')).length, 2);
  await json('/api/analyses/' + first.analysis_id + '/activate', { method: 'POST' });
  await upload(true);
  compose(workspace, 'compose.yaml', 'restart', 'backend');
  compose(workspace, 'compose.yaml', 'up', '-d', '--wait', '--wait-timeout', '120');
  assert.deepEqual(await json('/api/analysis'), analysis);
  assert.deepEqual(await json('/api/nodes/9007199254741000'), card);
  for (const [name, expected] of Object.entries(exports)) assert.equal(await (await request('/api/exports/' + name)).text(), expected);
  assert.equal((await json('/api/analyses')).length, 2);

  // Exercise the documented stopped-database backup/restore procedure.
  compose(workspace, 'compose.yaml', 'stop', 'backend');
  compose(workspace, 'compose.yaml', 'cp', 'backend:/var/lib/amlens/analyses.db', join(temporary, 'backup.db'));
  compose(workspace, 'compose.yaml', 'cp', join(temporary, 'backup.db'), 'backend:/var/lib/amlens/analyses.db');
  compose(workspace, 'compose.yaml', 'run', '--rm', '--no-deps', '--user', '0', '--cap-add', 'CHOWN', '--entrypoint', 'chown', 'backend', '10001:10001', '/var/lib/amlens/analyses.db');
  compose(workspace, 'compose.yaml', 'up', '-d', '--wait', '--wait-timeout', '120');
  assert.deepEqual(await json('/api/analysis'), analysis);
  assert.deepEqual(await json('/api/nodes/9007199254741000'), card);
  docker(['compose', '-p', demo, '-f', 'compose.demo.yaml', '-f', 'compose.public-demo.yaml', 'config', '--quiet']);
  compose(demo, 'compose.demo.yaml', 'up', '--build', '-d', '--wait', '--wait-timeout', '180');
  const demoURL = 'http://127.0.0.1:' + demoPort;
  docker(['run', '--rm', '--env', 'DOMAIN=demo.example.org', '--entrypoint', 'caddy',
    '--mount', 'type=bind,source=' + join(root, 'deploy', 'Caddyfile.public-demo') + ',target=/etc/caddy/Caddyfile,readonly',
    demo + '-frontend', 'validate', '--config', '/etc/caddy/Caddyfile', '--adapter', 'caddyfile']);
  const health = await json('/api/health', {}, 200, demoURL);
  assert.equal(health.demo_mode, true);
  assert.equal(health.ai_configured, false);
  const demoAnalysis = await json('/api/analysis', {}, 200, demoURL);
  assert.equal(demoAnalysis.nodes.length, 60);
  assert.notEqual(demoAnalysis.analysis_id, first.analysis_id);
  for (const path of ['/api/analyze', '/api/ask', '/api/analyses/' + demoAnalysis.analysis_id + '/activate']) {
    await request(path, { method: 'POST' }, 403, demoURL);
  }
  console.log('PASS: clean install, real Parquet import, exact IDs, cards, CSV, history, failed import, restart recovery, backup/restore, isolated read-only demo.');
  console.log('Total: ' + Math.round((performance.now() - started) / 1000) + ' s');
} catch (error) {
  docker(['compose', '-p', workspace, '-f', 'compose.yaml', 'logs', '--tail', '80'], false, true);
  docker(['compose', '-p', demo, '-f', 'compose.demo.yaml', 'logs', '--tail', '80'], false, true);
  throw error;
} finally {
  docker(['compose', '-p', demo, '-f', 'compose.demo.yaml', 'down', '--volumes', '--remove-orphans'], false, true);
  docker(['compose', '-p', workspace, '-f', 'compose.yaml', 'down', '--volumes', '--remove-orphans'], false, true);
  assert.equal(dirname(resolve(temporary)), resolve(tmpdir()));
  rmSync(temporary, { recursive: true, force: true });
}
