# Optimizations & smoothing for the GHCR → OBS pipeline

## Context
Pipeline currently: tag push → `release.yaml` (lint, helm package, push to GHCR) → calls `obs-sync.yaml` reusable workflow → spins up `opensuse/tumbleweed:latest` container, `zypper -n in helm osc git grep gawk tar` every run (~45–90s of pure tool install), `osc co`, version check, optionally re-tar + `osc commit`. A 6-hourly `sync-to-obs.yaml` cron does the same as a safety net.

Pain points:
- ~70 % of every sync-job runtime is **zypper install** that produces identical bytes every time.
- Most cron runs find **no version change** but still pay the full setup cost.
- Tool versions drift (`tumbleweed:latest`, unpinned helm/osc) — silent breakage waiting to happen.
- No concurrency control — two overlapping runs could `osc commit` simultaneously.
- One chart today; if it grows to N charts each will re-pay the setup cost.

Menu of optimizations, ordered roughly by **payoff / effort** ratio. Pick what's worth doing; ignore the rest.

---

## Tier 1 — Quick wins (small change, real payoff)

### 1.1 Cheap early-exit job (skip the heavy container when nothing changed)
**Effort:** ~20 min. **Impact:** kills ~80 % of cron-run minutes once 0.1.0 is in OBS.

Add a `check` job that runs on `ubuntu-latest` (no container), uses `helm show chart` against GHCR and `curl -u $OBS_USER:$OBS_PASSWORD https://api.opensuse.org/source/home:eminguez:charts/hello-world-chart/Chart.yaml` to read the current OBS version. Emits `outputs.changed`. The `sync` job adds `if: needs.check.outputs.changed == 'true'`.

```yaml
jobs:
  check:
    runs-on: ubuntu-latest
    outputs:
      changed: ${{ steps.diff.outputs.changed }}
    steps:
      - id: diff
        env: { OBS_USER: ..., OBS_PASSWORD: ..., GH_TOKEN: ... }
        run: |
          # lightweight: 2 curls + jq, no zypper, ~5 sec total
          ...
  sync:
    needs: check
    if: needs.check.outputs.changed == 'true'
    runs-on: ubuntu-latest
    container: opensuse/tumbleweed:latest
    ...
```

### 1.2 Concurrency control
**Effort:** 2 lines. **Impact:** prevents the cron run racing the release-triggered run on `osc commit`.

Add to both `release.yaml` and `sync-to-obs.yaml`:
```yaml
concurrency:
  group: obs-sync-${{ github.workflow }}
  cancel-in-progress: false   # queue, don't drop
```

### 1.3 Pin base image + tools
**Effort:** 5 min + future Renovate bumps. **Impact:** removes "worked yesterday, broken today" class of failures.

- `container: opensuse/tumbleweed:latest` → pin to a digest (`tumbleweed@sha256:...`) and let Renovate bump it.
- Optionally `zypper -n in helm-3.16.4 osc-1.9.2 ...` with explicit versions (or accept rolling Tumbleweed and pin only the base image).

### 1.4 `timeout-minutes` per job
**Effort:** 1 line. **Impact:** prevents a wedged osc/helm from burning the full 6-hour quota.

```yaml
jobs:
  sync:
    timeout-minutes: 10
```

### 1.5 Quiet, deterministic zypper
**Effort:** flag changes. **Impact:** cleaner logs, marginally faster.

```
zypper -n --quiet in --no-recommends helm osc git tar
```
`--no-recommends` skips suggested-but-not-required packages → fewer downloads.

---

## Tier 2 — Custom CI image (biggest win, more upfront work)

### 2.1 Build & publish a pre-baked tooling image
**Effort:** ~1 hour. **Impact:** sync job goes from ~90s tool install → <5s pull, every run.

New file: `ci/Dockerfile`
```Dockerfile
FROM opensuse/tumbleweed:latest
RUN zypper -n --quiet in --no-recommends \
      helm osc git tar gawk grep ca-certificates && \
    zypper clean -a
```

New workflow: `.github/workflows/ci-image.yaml`
- Triggers: `push` paths-filter on `ci/Dockerfile`, weekly cron, `workflow_dispatch`.
- Builds with `docker/build-push-action`, pushes to `ghcr.io/e-minguez/charts-ci:latest` and `:YYYYMMDD-<sha>`.

`obs-sync.yaml` switches:
```yaml
container: ghcr.io/e-minguez/charts-ci:20260601-abc1234
# (or :latest during early development, then pin once stable)
```
…and the entire "Install tooling" step disappears.

**Bonus:** the image also covers any *future* helm workflows in the repo — composes naturally.

### 2.2 Cache zypper instead of (or alongside 2.1)
**Effort:** ~10 min. **Impact:** ~30 % install-time reduction, but custom image is strictly better.

```yaml
- uses: actions/cache@<sha>
  with:
    path: /var/cache/zypp
    key: zypper-${{ hashFiles('.github/workflows/obs-sync.yaml') }}
```
Fallback if 2.1 feels heavy — but if you do 2.1 you don't need this.

---

## Tier 3 — Architectural alternatives (rethink, not optimize)

### 3.1 OBS-side `_service` file: pull-based sync, eliminate GitHub Actions
**Effort:** 1–2 hours. **Impact:** removes `obs-sync.yaml` + `sync-to-obs.yaml` entirely, plus the OBS_USER/OBS_PASSWORD secret risk.

Put a `_service` file in the OBS package that uses `download_url` (or a custom one-shot script via `obs_scm`) to fetch the latest chart tgz from public GHCR, unpack, inject `#!BuildTag:`, repackage. OBS runs services on its own scheduler.

Trade-offs:
- Requires GHCR package to be **public** (OBS can't easily auth to private GHCR).
- All knowledge moves into OBS (`_service` XML) → less Git-visible, harder to reason about from the repo.
- Loses the "release tag → immediate OBS push" fast path; OBS service polling cadence is the only trigger.

Worth doing only if maintaining GitHub Actions OBS auth becomes painful.

### 3.2 Hybrid: keep release.yaml's `mirror-to-obs`, drop the cron
**Effort:** delete 1 file. **Impact:** removes redundant safety net once you trust the release-chained path.

The cron sync was justified as a safety net, but `release.yaml`'s `mirror-to-obs` job already retries via GH's standard re-run UI. If GHCR ↔ OBS drift becomes a concern, a *lightweight* cron (just the check job from 1.1, ping if drift detected, don't auto-fix) is more useful than a full re-sync.

### 3.3 OBS scoped token for rebuild trigger
**Effort:** 30 min. **Impact:** lets the workflow trigger an OBS rebuild without `osc commit` (when only config changed, not files).

Create a `rebuild`-type OBS token scoped to `home:eminguez:charts/hello-world-chart`. Workflow does `curl -H "Authorization: Token <T>" -X POST https://api.opensuse.org/trigger/rebuild?project=...&package=...`. Useful for "rebuild against a new BCI base" type events. Doesn't replace the file-sync path, complements it.

---

## Tier 4 — Quality-of-life

### 4.1 Composite action for tool setup
If the repo grows past two workflows needing the same tools, extract `Install tooling` + `Configure osc credentials` + `Log in to GHCR` into `.github/actions/setup-helm-osc/action.yaml`. Until then, YAGNI.

### 4.2 Renovate or Dependabot
`.github/renovate.json` to auto-bump:
- `actions/*` and `azure/setup-helm` SHA pins (Dependabot can do this too).
- The custom CI image tag once 2.1 lands.
- Base image digest.

### 4.3 Local `Justfile` / `Makefile`
```
just lint        # helm lint
just template    # helm template test ...
just package     # helm package
just bump 0.1.1  # sed Chart.yaml + git tag
```
Friction-reducer for the manual steps you do today by hand.

### 4.4 Conventional commits → auto version bump + tag
Use `release-please` or similar to read commit messages, open a PR that bumps `Chart.yaml.version`, and tag on merge. Removes the manual `Chart.yaml` edit + `git tag` ceremony. Probably overkill until release cadence is > monthly.

### 4.5 Chart signing (cosign)
Sign the chart on GHCR push, verify on OBS sync. Defense-in-depth for the OCI artifact.

### 4.6 Smoke test before publish
New job in `release.yaml` between `publish` and `mirror-to-obs`: spin up `kind`, `helm install`, curl the pod, `helm uninstall`. Catches "shipped a broken chart" before it lands in OBS.

---

## Recommended phased rollout

1. **Now:** 1.1 (early-exit), 1.2 (concurrency), 1.4 (timeout). 10 minutes total, large operational win.
2. **Next:** 2.1 (custom CI image). One afternoon, ~70s saved per sync run forever.
3. **Then:** 1.3 (pin base image) + 4.2 (Renovate) together. Keeps 2.1's pins from rotting.
4. **Later, if it hurts:** 3.1 (OBS `_service`) or 4.6 (smoke test) depending on which problem bites first.

Skip 2.2, 4.1, 4.3, 4.4 until a second chart or a second pain point appears — premature otherwise.

---

## Verification (per change)

Each Tier-1 item: re-run `sync-to-obs.yaml` manually, compare runtime in Actions UI before/after.

Tier 2: after the CI image lands, the sync job's "Install tooling" step should be replaced by a "Pull container" line in the runner log, and the job should complete in ~30 % of current wall-clock.

Tier 3 (`_service`): trigger via OBS UI (Source Services → Run), confirm the package files refresh and a build queues without any GitHub Actions involvement.
