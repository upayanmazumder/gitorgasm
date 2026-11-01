import { execFileSync, spawnSync } from 'node:child_process';
import { readFileSync } from 'node:fs';

const git = (...args) => execFileSync('git', args, { encoding: 'utf8' }).trim();
const root = git('rev-parse', '--show-toplevel');
process.chdir(root);
const zero = /^0+$/;
const files = new Set();
const updates = readFileSync(0, 'utf8').trim();
for (const update of updates ? updates.split('\n') : []) {
  const [, local, , remote] = update.trim().split(/\s+/);
  if (!local || !remote) throw new Error('Invalid pre-push ref update');
  if (zero.test(local)) continue;
  const base = zero.test(remote) ? git('merge-base', local, 'refs/remotes/origin/main') : remote;
  const changed = execFileSync('git', ['diff', '--name-only', '-z', base, local], { encoding: 'utf8' });
  for (const file of changed.split('\0').filter(Boolean)) files.add(file);
}
const changed = (...prefixes) => [...files].some(file => prefixes.some(prefix => file.startsWith(prefix)));
const run = (cwd, command, args, env = {}) => {
  console.log(`[pre-push] ${cwd}: ${command} ${args.join(' ')}`);
  const result = spawnSync(command, args, { cwd, stdio: 'inherit', env: { ...process.env, ...env } });
  if (result.error) throw result.error;
  if (result.status !== 0) process.exit(result.status ?? 1);
};
const web = [...files].some(file => file.startsWith('web/') && !file.endsWith('.md'));
if (web) {
  run('web', 'pnpm', ['lint']);
  run('web', 'pnpm', ['typecheck']);
  run('web', 'pnpm', ['format:check']);
  run('web', 'pnpm', ['test'], { SKIP_ENV_VALIDATION: 'true' });
}
if (changed('core/', 'contracts/')) run('core', 'go', ['vet', './...']);
for (const crate of ['scoring', 'tests/performance/lang-bench/rust']) {
  if (!changed(`${crate}/`)) continue;
  run(crate, 'cargo', ['fmt', '--check']);
  run(crate, 'cargo', ['clippy', '--all-targets', '--locked', '--', '-D', 'warnings']);
  run(crate, 'cargo', ['test', '--locked']);
}
if (changed('tests/performance/lang-bench/go/')) {
  run('tests/performance/lang-bench/go', 'go', ['vet', './...']);
}
if (changed('deploy/')) run(root, 'make', ['manifests']);
if (changed('.github/workflows/')) {
  run(root, 'go', ['run', 'github.com/rhysd/actionlint/cmd/actionlint@v1.7.12']);
}
if (changed('.husky/', '.lintstagedrc.mjs', 'commitlint.config.mjs')) {
  run(root, 'node', ['--check', '.husky/check-push.mjs']);
  run(root, 'node', ['--check', '.lintstagedrc.mjs']);
  run(root, 'node', ['--check', 'commitlint.config.mjs']);
}
console.log('[pre-push] Changed-area checks passed. Full integration checks remain in CI.');
