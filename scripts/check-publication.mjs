import { existsSync, lstatSync, readdirSync, readFileSync } from 'node:fs';
import { dirname, join, relative, resolve, sep } from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';

const root = process.argv[2] === '--root' && process.argv[3]
  ? resolve(process.argv[3]) : resolve(dirname(fileURLToPath(import.meta.url)), '..');
const ignoredDirectories = new Set(['.git', 'node_modules', 'dist', 'build', 'bin', '.tmp', '.cache', '.venv', '__pycache__']);
const privateDirectories = /^(?:data.*|starter.*|datasets|uploads|out|exports)$/i;
const privateFile = /(?:^\.env(?:\..+)?$|\.(?:zip|7z|rar|tar|gz|parquet|arrow|feather|csv|xlsx?|db(?:-\w+)?|sqlite3?(?:-\w+)?|sql|pem|key|p12|pfx)$|^(?:credentials|secrets)\.json$)/i;
const buildFile = /(?:\.(?:exe|test|pyc|tsbuildinfo|log)$|^coverage|^\.DS_Store$|^Thumbs\.db$)/;
const ignoredPrivateDirectory = new RegExp(privateDirectories.source);
const ignoredPrivateFile = new RegExp(privateFile.source);
const normalize = (path) => path.split(sep).join('/');
function sensitivePath(path) {
  const parts = path.split('/');
  return parts.slice(0, -1).some((part) => privateDirectories.test(part))
    || (parts.at(-1) !== '.env.example' && privateFile.test(parts.at(-1)));
}
let skipped = 0;
function walk(dir) {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const full = join(dir, entry.name);
    if (entry.isSymbolicLink()) return [normalize(relative(root, full))];
    if (entry.isDirectory()) {
      if (ignoredDirectories.has(entry.name) || ignoredPrivateDirectory.test(entry.name) || entry.name.endsWith('.egg-info')) {
        skipped++; return [];
      }
      return walk(full);
    }
    const path = normalize(relative(root, full));
    if ((entry.name !== '.env.example' && ignoredPrivateFile.test(entry.name)) || buildFile.test(entry.name)) { skipped++; return []; }
    return [path];
  });
}
let files;
const hasGit = existsSync(join(root, '.git'));
if (hasGit) {
  const result = spawnSync('git', ['ls-files', '--cached', '--others', '--exclude-standard', '-z'], { cwd: root, encoding: 'utf8' });
  if (result.status !== 0) { console.error('Не удалось получить список файлов Git.'); process.exit(1); }
  files = [...new Set(result.stdout.split('\0').filter(Boolean))];
} else {
  files = walk(root);
}
const patterns = [
  /sk-(?:proj-|svcacct-)?[A-Za-z0-9_-]{20,}/g,
  /gh[pousr]_[A-Za-z0-9]{20,}/g,
  /github_pat_[A-Za-z0-9_]{30,}/g,
  /AKIA[0-9A-Z]{16}/g,
  /-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----/g,
  /(?:OPENAI_API_KEY|API_TOKEN|ACCESS_TOKEN|CLIENT_SECRET)\s*[:=]\s*["'][A-Za-z0-9_\-/.+]{20,}["']/gi,
];
const findings = [];
function inspect(content, path, label = path) {
  for (const pattern of patterns) {
    pattern.lastIndex = 0;
    for (const match of content.matchAll(pattern)) {
      findings.push(label + ':' + (content.slice(0, match.index).split('\n').length) + ': возможный секрет (значение скрыто)');
    }
  }
  if (path.endsWith('.env.example')) {
    for (const [index, line] of content.split(/\r?\n/).entries()) {
      const match = line.match(/^\s*([A-Z0-9_]*(?:KEY|TOKEN|PASSWORD|SECRET))\s*=\s*(.*?)\s*$/);
      if (match && !['', '""', "''"].includes(match[2])) findings.push(label + ':' + (index + 1) + ': секрет в примере окружения должен быть пустым');
    }
  }
}
for (const path of files.sort()) {
  if (sensitivePath(path)) { findings.push(path + ': запрещённый файл среди кандидатов публикации'); continue; }
  const full = join(root, path);
  let content;
  if (existsSync(full)) {
    // Do not follow symlinks outside the publication tree.
    if (lstatSync(full).isSymbolicLink()) { findings.push(path + ': проверьте символическую ссылку'); continue; }
    content = readFileSync(full, 'utf8');
    inspect(content, path);
  }
  if (hasGit) {
    // A cleaned working file may still have a secret in its staged version.
    const staged = spawnSync('git', ['show', ':' + path], { cwd: root, encoding: 'utf8', maxBuffer: 32 * 1024 * 1024 });
    if (staged.error) findings.push(path + ': не удалось проверить индекс Git');
    else if (staged.status === 0 && staged.stdout !== content) inspect(staged.stdout, path, path + ' [индекс Git]');
  }
}
if (findings.length) {
  console.error([...new Set(findings)].join('\n'));
  process.exitCode = 1;
} else {
  console.log('Проверено файлов: ' + files.length + '. Признаков секретов и запрещённых файлов среди кандидатов публикации не найдено.');
  if (!hasGit) console.log('Git ещё не создан. Локальные данные, окружения и сборки исключены по правилам проекта; пропущено элементов: ' + skipped + '.');
}
