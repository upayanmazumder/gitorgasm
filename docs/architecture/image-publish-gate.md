# Image publish gate

Finding: a same-repository pull request can request package write access and replace the mutable `latest` image. Production now checks a signed publication attestation before admitting Gitorgasm core workloads. Registry write access alone no longer permits unsigned images to start. This does not protect against compromise of the trusted main-branch publisher or cluster administration.

## What the files do

- `.github/workflows/ci.yml` job `image` builds the image through `image.yml` with `permissions: contents: read`. `image.yml` has no registry login and `push: false`.
- `.github/workflows/ci.yml` job `image-publish` calls `image-publish.yml`. It is the only job with `packages: write`. Its `if` requires `github.event_name == 'push'` and `github.ref == 'refs/heads/main'`. `image-publish.yml` repeats the same condition on its job, so a different caller cannot push from another ref or event.
- The publish job sets `environment: image-publish`. GitHub documents that the job that calls a reusable workflow cannot use `environment`; the job inside the reusable workflow can ([Reuse workflows](https://docs.github.com/en/actions/how-tos/reuse-automations/reuse-workflows)). That is why the environment is in `image-publish.yml`.
- The `github` context inside a called workflow is the caller's, so the condition sees the real event and ref (same page). The called workflow can only keep or reduce the caller's `GITHUB_TOKEN` permissions.

## The limit of file-level controls

For a `pull_request` run, GitHub uses workflow files from the PR head ([workflow syntax](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax)). A same-repository PR author can edit workflow conditions and request `packages: write`. File-level publishing guards do not prevent that. The cluster admission policy is the independent deployment boundary.

## Settings applied in GitHub (2026-11-05)

These settings live in GitHub, not in files. Re-check them if the repository is recreated or transferred.

1. **Environment `image-publish`**: deployment branches are limited to `main`, and administrators cannot bypass its rules. **Limit:** `GITHUB_TOKEN` and its `packages: write` scope are not tied to an environment. A PR author can delete the `environment:` line and still receive `packages: write` if a job requests it. The environment protects only secrets and variables stored in it.
2. **GHCR package `gitorgasm/core`**: inherited access from the source repository is off, and no person has direct access except the owner. The repository `gitorgasm` keeps Actions access with the Admin role, because the publish job needs it. GitHub does not offer a way to limit that access to the `main` ref.
3. **Actions settings**: the default workflow `GITHUB_TOKEN` permission is read-only.
4. **Ruleset `main protection`**: it requires review from code owners. CODEOWNERS lists `/.github/workflows/` and `/deploy/`. Repository administrators can bypass the ruleset. This protects `main`, not a PR run.

## Enforced publication check

The publish job signs the digest and attaches a signed publication attestation of type `https://gitorgasm.upayan.dev/attestations/image-publish/v1`. Its predicate records the repository, commit, ref, and image digest. This records publication, not SLSA build provenance or SBOM verification.

The cluster policy verifies the signature, certificate identity, subject digest, and transparency proof. It requires issuer `https://token.actions.githubusercontent.com` and subject `https://github.com/upayanmazumder/gitorgasm/.github/workflows/image-publish.yml@refs/heads/main`. Predicate constraints require repository `upayanmazumder/gitorgasm` and ref `refs/heads/main`. Another workflow or PR ref does not qualify.

Cluster [ADR 0010](https://github.com/upayanmazumder/cluster/blob/main/docs/adr/0010-gitorgasm-image-signatures.md) records the controller and policy. Enforcement applies only to the exact core image repository in namespace `gitorgasm`. Workload webhooks fail closed. Postgres and other namespaces remain outside this image policy.

Live server-side dry-runs admitted attested index `781672f7fdd17a459770e7a4937b06823fffb467318bdb68f5fa7be2acbfb6cd`. They rejected unsigned index `7410b54ac65caa9dc0ce7298fecf2d8b2dc58975c29f18c97c7ffb21630e18c9`. These checks created no pods. Independent cosign verification also rejected an incorrect workflow identity. That identity check was not a separate live admission test.

## Publishing credential

Publishing still uses `GITHUB_TOKEN`. No separate personal access token or environment secret was created. The environment rule does not restrict that token. Deployment security therefore depends on cluster verification, not on registry write restrictions alone.

## Image Updater and tags: left unchanged

`deploy/kustomize/base/kustomization.yaml` keeps `newTag: latest`. The Image Updater configuration is not in this repository (ADR 0012 keeps the Application and ImageUpdater entry in `upayanmazumder/cluster`).

The Image Updater docs describe `allowTags` regular expressions with the `newest-build` strategy ([update strategies](https://argocd-image-updater.readthedocs.io/en/stable/basics/update-strategies/)). That strategy chooses by image build date, not by push date, and the build date is set by whoever builds the image. A writer who can push `latest` can also push a `sha-` tag. Restricting to `sha-` tags therefore does not close this path, and it would change the rollout mechanism without a provenance check. ADR 0010 and 0012 also record that pinning `sha-<commit>` in git needs a pull request per image, which was rejected. If the owner wants immutable tags, that is a decision for an ADR together with the cluster repository change.
