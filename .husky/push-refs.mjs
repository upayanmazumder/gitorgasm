import { spawnSync } from 'node:child_process';

const zero = sha => /^0+$/.test(sha);
const objectId = /^(?:[a-f\d]{40}|[a-f\d]{64})$/i;
const validRemoteRef = ref => ref.startsWith('refs/') && !ref.endsWith('.') &&
  !/[\x00-\x20\x7f~^:?*[\\]/.test(ref) && !ref.includes('..') && !ref.includes('@{') &&
  ref.split('/').every(part => part && !part.startsWith('.') && !part.endsWith('.lock'));

export function parseUpdates(input) {
  if (!input.trim()) return null;
  const updates = [];
  for (const line of input.trim().split(/\r?\n/)) {
    const fields = line.trim().split(/\s+/);
    if (fields.length !== 4) return null;
    const [localRef, local, remoteRef, remote] = fields;
    if (!objectId.test(local) || !objectId.test(remote) || local.length !== remote.length ||
      !validRemoteRef(remoteRef) || (localRef === '(delete)' && !zero(local))) return null;
    updates.push({ localRef, local, remoteRef, remote });
  }
  return updates;
}

// Discovery reads only local Git objects; uncertainty always requests the full suite.
export function discoverPushChanges(input, cwd = process.cwd()) {
  const fallback = { files: [], full: true };
  const updates = parseUpdates(input);
  if (!updates) return fallback;
  const git = (...args) => {
    const result = spawnSync('git', args, { cwd, encoding: 'utf8' });
    return result.error || result.status !== 0 ? null : result.stdout;
  };
  const files = new Set();
  for (const { local, remote } of updates) {
    if (zero(local)) continue;
    const remoteCommit = zero(remote) ? null : git('rev-parse', '--verify', `${remote}^{commit}`);
    const base = remoteCommit ?? git('merge-base', 'refs/remotes/origin/main', local);
    if (!base?.trim()) return fallback;
    const diff = git('diff', '--name-only', '--no-renames', '-z', base.trim(), local, '--');
    if (diff === null || !diff.endsWith('\0')) return fallback;
    const changed = diff.slice(0, -1).split('\0');
    if (changed.some(file => !file)) return fallback;
    for (const file of changed) files.add(file);
  }
  return { files: [...files], full: false };
}
