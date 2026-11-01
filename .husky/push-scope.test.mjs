import assert from 'node:assert/strict';
import test from 'node:test';
import { planChecks } from './push-scope.mjs';

const commands = files => planChecks({ files }).map(({ cwd, command, args }) => [cwd, command, ...args]);
const full = [['.', 'make', 'lint', 'test']];

test('core and contract changes run lint and database-backed Go tests', () => {
  for (const file of ['core/internal/store/query.go', 'contracts/openapi/v1.yaml']) {
    assert.deepEqual(commands([file]), [['.', 'make', 'lint', 'test-go']]);
  }
});

test('web markdown gets formatting without code checks', () => {
  assert.deepEqual(commands(['web/README.md', 'web/docs/guide.mdx']), [['web', 'pnpm', 'format:check']]);
  assert.deepEqual(commands(['README.md', 'docs/adr/0001.md']), []);
  assert.deepEqual(commands(['web/src/page.tsx']).map(command => command.slice(0, 3)), [
    ['web', 'pnpm', 'lint'], ['web', 'pnpm', 'typecheck'],
    ['web', 'pnpm', 'format:check'], ['web', 'pnpm', 'test'],
  ]);
});

test('shared configuration and dependencies override every scoped check', () => {
  for (const file of [
    'Makefile', 'package.json', 'pnpm-lock.yaml', 'web/package.json', 'web/pnpm-lock.yaml',
    'core/go.mod', 'core/go.sum', 'scoring/Cargo.toml', 'scoring/Cargo.lock',
    'nested/package-lock.json', 'nested/yarn.lock', '.husky/check-push.mjs',
    '.github/workflows/ci.yml', '.github/ISSUE_TEMPLATE/bug.md',
  ]) assert.deepEqual(commands([file, 'web/src/page.tsx']), full, file);
});

test('unknown non-doc paths and comparison uncertainty use full checks', () => {
  assert.deepEqual(commands(['scripts/new-check.sh']), full);
  assert.deepEqual(commands(['tests/performance/lang-bench/run.sh']), full);
  assert.deepEqual(planChecks({ files: [], full: true }).map(({ cwd, command, args }) => [cwd, command, ...args]), full);
});

test('known Rust, benchmark Go, and deployment paths retain scoped checks', () => {
  assert.deepEqual(commands(['scoring/src/lib.rs']), [
    ['scoring', 'cargo', 'fmt', '--check'],
    ['scoring', 'cargo', 'clippy', '--all-targets', '--locked', '--', '-D', 'warnings'],
    ['scoring', 'cargo', 'test', '--locked'],
  ]);
  assert.deepEqual(commands(['tests/performance/lang-bench/go/main.go', 'deploy/kustomize/base/api.yaml']), [
    ['tests/performance/lang-bench/go', 'go', 'vet', './...'], ['.', 'make', 'manifests'],
  ]);
});
