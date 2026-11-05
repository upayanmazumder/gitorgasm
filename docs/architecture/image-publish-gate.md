# Image publish gate

Finding: a same-repository pull request could push the mutable `latest` tag of `ghcr.io/upayanmazumder/gitorgasm/core`, which Argo CD Image Updater deploys to production (ADR 0012). This page records what the repository files do now and what only the owner can set in GitHub. **The finding is not closed by the files alone.**

## What the files do

- `.github/workflows/ci.yml` job `image` builds the image through `image.yml` with `permissions: contents: read`. `image.yml` has no registry login and `push: false`.
- `.github/workflows/ci.yml` job `image-publish` calls `image-publish.yml`. It is the only job with `packages: write`. Its `if` requires `github.event_name == 'push'` and `github.ref == 'refs/heads/main'`. `image-publish.yml` repeats the same condition on its job, so a different caller cannot push from another ref or event.
- The publish job sets `environment: image-publish`. GitHub documents that the job that calls a reusable workflow cannot use `environment`; the job inside the reusable workflow can ([Reuse workflows](https://docs.github.com/en/actions/how-tos/reuse-automations/reuse-workflows)). That is why the environment is in `image-publish.yml`.
- The `github` context inside a called workflow is the caller's, so the condition sees the real event and ref (same page). The called workflow can only keep or reduce the caller's `GITHUB_TOKEN` permissions.

## The limit of file-level controls

For a `pull_request` run, GitHub uses the workflow files of the PR head ([workflow syntax](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax)). A same-repository PR author can edit `ci.yml` and `image-publish.yml`, remove the `if` conditions, and ask for `packages: write`. No change to these files can stop that. The controls below, which live in GitHub settings, are what stop it.

## Settings applied in GitHub (2026-11-05)

These settings live in GitHub, not in files. Re-check them if the repository is recreated or transferred.

1. **Environment `image-publish`**: deployment branches are limited to `main`, and administrators cannot bypass its rules. **Limit:** `GITHUB_TOKEN` and its `packages: write` scope are not tied to an environment. A PR author can delete the `environment:` line and still receive `packages: write` if a job requests it. The environment protects only secrets and variables stored in it.
2. **GHCR package `gitorgasm/core`**: inherited access from the source repository is off, and no person has direct access except the owner. The repository `gitorgasm` keeps Actions access with the Admin role, because the publish job needs it. GitHub does not offer a way to limit that access to the `main` ref.
3. **Actions settings**: the default workflow `GITHUB_TOKEN` permission is read-only.
4. **Ruleset `main protection`**: it requires review from code owners. CODEOWNERS lists `/.github/workflows/` and `/deploy/`. Repository administrators can bypass the ruleset. This protects `main`, not a PR run.

## Still open

1. **Provenance check at deploy time**: the build attaches provenance (`mode=max`) and an SBOM. Nothing in the cluster verifies them. The Image Updater entry and the Argo CD Application live in `upayanmazumder/cluster`, not here. The cluster has no admission policy controller today. A check needs one, plus signed attestations from the publish job. That is a new platform component and needs an ADR in the cluster repository. This repository cannot enforce it.
2. **Credential that only the environment can read**: a push credential stored as an `image-publish` environment secret would make the environment a real gate. GHCR accepts only `GITHUB_TOKEN` and personal access tokens. A classic token with `write:packages` also forces the `repo` scope in the token form. That scope gives full control of the owner's private repositories, which is a worse exposure than the one it would close. No such credential was created. Revisit this if GHCR accepts a narrower credential.

## Image Updater and tags: left unchanged

`deploy/kustomize/base/kustomization.yaml` keeps `newTag: latest`. The Image Updater configuration is not in this repository (ADR 0012 keeps the Application and ImageUpdater entry in `upayanmazumder/cluster`).

The Image Updater docs describe `allowTags` regular expressions with the `newest-build` strategy ([update strategies](https://argocd-image-updater.readthedocs.io/en/stable/basics/update-strategies/)). That strategy chooses by image build date, not by push date, and the build date is set by whoever builds the image. A writer who can push `latest` can also push a `sha-` tag. Restricting to `sha-` tags therefore does not close this path, and it would change the rollout mechanism without a provenance check. ADR 0010 and 0012 also record that pinning `sha-<commit>` in git needs a pull request per image, which was rejected. If the owner wants immutable tags, that is a decision for an ADR together with the cluster repository change.
