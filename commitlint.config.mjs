// Conventional Commits: feat, fix, docs, refactor, test, chore, ... (see CONTRIBUTING.md).
export default {
  extends: ['@commitlint/config-conventional'],
  rules: {
    'body-max-line-length': [1, 'always', 100],
  },
};
