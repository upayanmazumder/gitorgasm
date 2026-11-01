const rustCrates = ['scoring', 'tests/performance/lang-bench/rust'];
const markdown = file => /\.mdx?$/i.test(file);
const shared = file => {
  const name = file.split('/').at(-1);
  return file === 'Makefile' || file.startsWith('.husky/') || file.startsWith('.github/') ||
    /^(package\.json|go\.(mod|sum|work|work\.sum)|Cargo\.(toml|lock)|pnpm-lock\.yaml|npm-shrinkwrap\.json)$/.test(name) ||
    /(?:\.lockb?|(?:^|-)lock\.json)$/.test(name);
};
const known = file => ['web/', 'core/', 'contracts/', 'deploy/',
  'tests/performance/lang-bench/go/', ...rustCrates.map(crate => `${crate}/`),
].some(prefix => file.startsWith(prefix));

export function planChecks({ files, full = false }) {
  const commands = [];
  const add = (cwd, command, args, env) => commands.push({ cwd, command, args, ...(env && { env }) });
  if (full || files.some(file => shared(file) || (!markdown(file) && !known(file)))) {
    add('.', 'make', ['lint', 'test']);
    return commands;
  }
  const code = files.filter(file => !markdown(file));
  const changed = (...prefixes) => code.some(file => prefixes.some(prefix => file.startsWith(prefix)));
  const lintAll = changed('core/', 'contracts/');
  if (lintAll) add('.', 'make', ['lint', 'test-go']);
  if (changed('web/')) {
    if (!lintAll) {
      for (const task of ['lint', 'typecheck', 'format:check']) add('web', 'pnpm', [task]);
    }
    add('web', 'pnpm', ['test'], { SKIP_ENV_VALIDATION: 'true' });
  } else if (!lintAll && files.some(file => file.startsWith('web/'))) {
    add('web', 'pnpm', ['format:check']);
  }
  for (const crate of rustCrates) {
    if (!changed(`${crate}/`)) continue;
    if (!lintAll) {
      add(crate, 'cargo', ['fmt', '--check']);
      add(crate, 'cargo', ['clippy', '--all-targets', '--locked', '--', '-D', 'warnings']);
    }
    add(crate, 'cargo', ['test', '--locked']);
  }
  if (!lintAll && changed('tests/performance/lang-bench/go/')) {
    add('tests/performance/lang-bench/go', 'go', ['vet', './...']);
  }
  if (changed('deploy/')) add('.', 'make', ['manifests']);
  return commands;
}
