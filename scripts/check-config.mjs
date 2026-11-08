import fs from 'node:fs';
import assert from 'node:assert/strict';

const compose = fs.readFileSync('compose.yaml', 'utf8');
const panel = fs.readFileSync('deploy/compose.1panel.yaml', 'utf8');
const env = fs.readFileSync('.env.example', 'utf8');
const docker = fs.readFileSync('Dockerfile', 'utf8');

// The deployment target is one SQLite file: no database server anywhere.
for (const [name, file] of [['compose.yaml', compose], ['deploy/compose.1panel.yaml', panel]]) {
  assert(!/image:\s*postgres/i.test(file), name + ' must not run a PostgreSQL container');
  assert(!file.includes('DATABASE_URL'), name + ' must not configure DATABASE_URL');
  assert(file.includes('SQLITE_PATH'), name + ' must point at the SQLite file');
  assert(file.includes('read_only: true'), name + ' application root must be read-only');
  assert(file.includes('APP_ENCRYPTION_KEY'), name + ' must require the encryption key');
  assert(file.includes('PUBLIC_ORIGIN'), name + ' must pin the public origin');
}
assert(!compose.includes('5432:5432'), 'do not publish a database port');
assert(env.includes('APP_ENCRYPTION_KEY='), 'encryption key documented');
assert(!env.includes('DATABASE_URL'), 'no database URL in the environment example');
assert(!docker.includes('poppler') && !docker.includes('tesseract'), 'no local OCR/PDF runtime');

const pkg = JSON.parse(fs.readFileSync('web/package.json', 'utf8'));
assert(pkg.packageManager.startsWith('pnpm@'), 'lock package manager version');

console.log('Deployment guards passed: single-file SQLite, no database server, read-only root.');
