import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';
import { discoverPushChanges, parseUpdates } from './push-refs.mjs';

const zero = '0'.repeat(40);
const missing = 'f'.repeat(40);
const record = (local, remote) => `refs/heads/topic ${local} refs/heads/topic ${remote}`;

function repository(t) {
  const cwd = mkdtempSync(join(tmpdir(), 'gitorgasm-push-'));
  t.after(() => rmSync(cwd, { recursive: true, force: true }));
  const git = (...args) => execFileSync('git', args, { cwd, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] }).trim();
  git('init', '--initial-branch=main');
  git('config', 'user.name', 'Hook test');
  git('config', 'user.email', 'hook@example.invalid');
  git('config', 'core.hooksPath', '/dev/null');
  git('config', 'commit.gpgsign', 'false');
  const commit = (file, value) => {
    writeFileSync(join(cwd, file), value);
    git('add', '--', file);
    git('commit', '-m', 'test: change fixture');
    return git('rev-parse', 'HEAD');
  };
  const base = commit('README.md', 'base\n');
  git('update-ref', 'refs/remotes/origin/main', base);
  const next = commit('unknown.sh', 'exit 0\n');
  return { cwd, git, commit, base, next };
}

test('malformed and absent stdin are uncertain, not skipped', () => {
  const malformed = [
    '',
    '\n',
    'not a ref record',
    record('HEAD', zero),
    `${record(missing, zero)} extra`,
    `HEAD ${missing} refs/ ${zero}`,
    `HEAD ${missing} refs/heads/bad..name ${zero}`,
  ];
  for (const input of malformed) {
    assert.equal(parseUpdates(input), null, input);
    assert.equal(discoverPushChanges(input).full, true, input);
  }
  assert.equal(parseUpdates(record(missing, zero)).length, 1);
});

test('new branches and missing remote objects compare to origin/main merge base', t => {
  const { cwd, next } = repository(t);
  for (const remote of [zero, missing]) {
    assert.deepEqual(discoverPushChanges(record(next, remote), cwd), { files: ['unknown.sh'], full: false });
  }
});

test('multiple refs aggregate paths and deletion records are ignored', t => {
  const { cwd, base, next, commit } = repository(t);
  const last = commit('README.md', 'changed docs\n');
  const deletion = `(delete) ${zero} refs/heads/old ${next}`;
  const result = discoverPushChanges([record(next, base), record(last, next), deletion].join('\n'), cwd);
  assert.deepEqual(result, { files: ['unknown.sh', 'README.md'], full: false });
  assert.deepEqual(discoverPushChanges(deletion, cwd), { files: [], full: false });
});

test('empty non-deletion diffs and unresolved objects use full checks', t => {
  const { cwd, base, next, git } = repository(t);
  assert.equal(discoverPushChanges(record(next, next), cwd).full, true);
  assert.equal(discoverPushChanges(record(missing, base), cwd).full, true);
  git('update-ref', '-d', 'refs/remotes/origin/main');
  assert.equal(discoverPushChanges(record(next, zero), cwd).full, true);
  assert.equal(discoverPushChanges(record(next, missing), cwd).full, true);
});
