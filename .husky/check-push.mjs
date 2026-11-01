import { execFileSync, spawnSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { discoverPushChanges } from './push-refs.mjs';
import { planChecks } from './push-scope.mjs';

const root = execFileSync('git', ['rev-parse', '--show-toplevel'], { encoding: 'utf8' }).trim();
let input = '';
try {
  input = readFileSync(0, 'utf8');
} catch {
  // Unreadable hook input is uncertain, just like an unresolved comparison.
}
const changes = discoverPushChanges(input, root);
const commands = planChecks(changes);
if (changes.full) console.log('[pre-push] Uncertain ref comparison; running full lint and tests.');
for (const { cwd, command, args, env } of commands) {
  console.log(`[pre-push] ${cwd}: ${command} ${args.join(' ')}`);
  const result = spawnSync(command, args, {
    cwd: cwd === '.' ? root : `${root}/${cwd}`,
    stdio: 'inherit',
    env: { ...process.env, ...env },
  });
  if (result.error) throw result.error;
  if (result.status !== 0) process.exit(result.status ?? 1);
}
console.log(`[pre-push] ${commands.length ? 'Required checks passed.' : 'Documentation/deletion-only push; no code checks needed.'}`);
