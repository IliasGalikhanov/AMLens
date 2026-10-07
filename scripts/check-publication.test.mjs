import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, rmSync, writeFileSync, readFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';

const project = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const checker = join(project, 'scripts/check-publication.mjs');
const fakeToken = ['sk', 'proj', 'A'.repeat(50)].join('-');
function fixture(t) {
  const dir = mkdtempSync(join(tmpdir(), 'finance-publication-'));
  t.after(() => rmSync(dir, { recursive: true, force: true }));
  writeFileSync(join(dir, '.gitignore'), readFileSync(join(project, '.gitignore')));
  return dir;
}
function scan(dir) { return spawnSync(process.execPath, [checker, '--root', dir], { encoding: 'utf8' }); }
function git(dir, ...args) {
  const r = spawnSync('git', args, { cwd: dir, encoding: 'utf8' });
  assert.equal(r.status, 0, r.stderr);
}
test('detects a token in source without printing it', (t) => {
  const dir = fixture(t); writeFileSync(join(dir, 'source.txt'), fakeToken);
  const r = scan(dir); assert.equal(r.status, 1);
  assert.match(r.stderr, /source.txt:1/); assert.ok(!r.stderr.includes(fakeToken));
});
test('local env and data remain ignored, example secrets must stay empty', (t) => {
  const dir = fixture(t); writeFileSync(join(dir, '.env'), 'OPENAI_API_KEY=' + fakeToken);
  writeFileSync(join(dir, 'data.zip'), 'not a real archive');
  writeFileSync(join(dir, '.env.example'), 'OPENAI_API_KEY=\n');
  assert.equal(scan(dir).status, 0);
  writeFileSync(join(dir, '.env.example'), 'OPENAI_API_KEY=placeholder\n');
  assert.equal(scan(dir).status, 1);
});
test('a force-added env is blocked even after its working copy is cleaned', (t) => {
  const dir = fixture(t); git(dir, 'init', '-q');
  writeFileSync(join(dir, '.env'), 'OPENAI_API_KEY=' + fakeToken); git(dir, 'add', '-f', '.env');
  writeFileSync(join(dir, '.env'), '');
  const r = scan(dir); assert.equal(r.status, 1); assert.match(r.stderr, /\.env/); assert.ok(!r.stderr.includes(fakeToken));
});
test('checks staged content, not only the current working file', (t) => {
  const dir = fixture(t); git(dir, 'init', '-q');
  writeFileSync(join(dir, 'source.txt'), fakeToken); git(dir, 'add', 'source.txt');
  writeFileSync(join(dir, 'source.txt'), 'clean');
  const r = scan(dir); assert.equal(r.status, 1); assert.match(r.stderr, /индекс Git/); assert.ok(!r.stderr.includes(fakeToken));
});
test('the actual ignore rules exclude archive copies, inputs, outputs and env', (t) => {
  const dir = fixture(t); git(dir, 'init', '-q');
  for (const name of ['data.zip', 'data (1).zip', 'starter.zip', 'starter (1).zip', 'nodes.parquet', 'backend/.env', '.env.local', 'out/result.json']) {
    const r = spawnSync('git', ['check-ignore', '--no-index', name], { cwd: dir, encoding: 'utf8' });
    assert.equal(r.status, 0, name);
  }
  const example = spawnSync('git', ['check-ignore', '--no-index', '.env.example'], { cwd: dir, encoding: 'utf8' });
  assert.equal(example.status, 1);
});
