# Image publish gate

Finding: a same-repository pull request could push the mutable `latest` tag of `ghcr.io/upayanmazumder/gitorgasm/core`, which Argo CD Image Updater deploys to production (ADR 0012). This page records what the repository files do now and what only the owner can set in GitHub. **The finding is not closed by the files alone.**

## What the files do

- `.github/workflows/ci.yml` job `image` builds the image through `image.yml` with `permissions: contents: read`. `image.yml` has no registry login and `push: false`.
- `.github/workflows/ci.yml` job `image-publish` calls `image-publish.yml`. It is the only job with `packages: write`. Its `if` requires `github.event_name == 'push'` and `github.ref == 'refs/heads/main'`. `image-publish.yml` repeats the same condition on its job, so a different caller cannot push from another ref or event.
- The publish job sets `environment: image-publish`. GitHub documents that the job that calls a reusable workflow cannot use `environment`; the job inside the reusable workflow can ([Reuse workflows](https://docs.github.com/en/actions/how-tos/reuse-automations/reuse-workflows)). That is why the environment is in `image-publish.yml`.
- The `github` context inside a called workflow is the caller's, so the condition sees the real event and ref (same page). The called workflow can only keep or reduce the caller's `GITHUB_TOKEN` permissions.

## The limit of file-level controls

For a `pull_request` run, GitHub uses the workflow files of the PR head ([workflow syntax](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax)). A same-repository PR author can edit `ci.yml` and `image-publish.yml`, remove the `if` conditions, and ask for `packages: write`. No change to these files can stop that. The controls below, which live in GitHub settings, are what stop it.

## Settings only the owner can change

1. **Environment `image-publish`** (Settings, Environments): create it, choose "Selected branches and tags" under Deployment branches, and add a branch rule for `main`. Consider deselecting "Allow administrators to bypass configured protection rules" ([Managing environments](https://docs.github.com/en/actions/how-tos/deploy/configure-and-manage-deployments/manage-environments)). A job that references an environment waits for its protection rules. **Limit:** `GITHUB_TOKEN` and its `packages: write` scope are not tied to an environment. A PR author can delete the `environment:` line and still receive `packages: write` if a job requests it. The environment protects only secrets and variables stored in it. Treat it as a label until item 2 or the option below is in place.
2. **GHCR package access** (package `gitorgasm/core`, Package settings): remove or restrict "Manage Actions access" so that only this repository's workflows can write, and review who has direct write or admin on the package. Check whether the package settings let you limit write access to the `main` ref; if they do not, rely on item 1.
3. **Actions settings** (Settings, Actions, General): set the default workflow `GITHUB_TOKEN` permission to read-only, require approval for workflow runs from outside collaborators, and review who has write access to the repository. Anyone with write access can open a same-repository PR.
4. **Review of workflow changes**: add a ruleset or branch protection on `main` that requires review from code owners (CODEOWNERS already lists `/.github/workflows/` and `/deploy/`). `CONTRIBUTING.md` currently sets required approvals to 0 while there is one maintainer, so a workflow change merges without review. This protects `main`, not a PR run; it keeps the files above intact after merge.
5. **Provenance check at deploy time**: the build attaches provenance (`mode=max`) and an SBOM. Nothing in the cluster verifies them. The Image Updater entry and the Argo CD Application live in `upayanmazumder/cluster`, not here. Add an admission or verification step there that accepts only images whose attestation shows this repository and `refs/heads/main`. This repository cannot enforce it.

## Option that needs an owner decision: environment-scoped push credential

To make the environment a real gate, push to GHCR with a credential stored as an `image-publish` environment secret instead of `GITHUB_TOKEN`, and set the repository's workflow token to read-only. Environment secrets reach only jobs that reference the environment and pass its branch rule. This changes how publishing authenticates and needs a credential the owner creates (for example a fine-grained credential limited to the package), so it is not implemented in this change. Until then, the first two settings above carry the enforcement.

## Image Updater and tags: left unchanged

`deploy/kustomize/base/kustomization.yaml` keeps `newTag: latest`. The Image Updater configuration is not in this repository (ADR 0012 keeps the Application and ImageUpdater entry in `upayanmazumder/cluster`).

The Image Updater docs describe `allowTags` regular expressions with the `newest-build` strategy ([update strategies](https://argocd-image-updater.readthedocs.io/en/stable/basics/update-strategies/)). That strategy chooses by image build date, not by push date, and the build date is set by whoever builds the image. A writer who can push `latest` can also push a `sha-` tag. Restricting to `sha-` tags therefore does not close this path, and it would change the rollout mechanism without a provenance check. ADR 0010 and 0012 also record that pinning `sha-<commit>` in git needs a pull request per image, which was rejected. If the owner wants immutable tags, that is a decision for an ADR together with the cluster repository change.
