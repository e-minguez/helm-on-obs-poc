# helm-on-obs-poc

Basic Helm chart deploying [openSUSE nginx](https://registry.opensuse.org/) (`registry.opensuse.org/opensuse/nginx`).

Pipeline:

1. Tag push (`v*`) → `release.yaml` lints, packages, and publishes to **GHCR** at `oci://ghcr.io/e-minguez/charts/hello-world-helm-chart`.
2. Same workflow then chains into `obs-sync.yaml`, which mirrors the chart into the OBS package [`home:eminguez:charts/hello-world-chart`](https://build.opensuse.org/package/show/home:eminguez:charts/hello-world-chart) (Chart.yaml with `#!BuildTag:` header + `chart-assets.tar.gz`). OBS rebuilds and publishes to its own registry.
3. `sync-to-obs.yaml` re-runs the same sync every 6h as a safety net.

## Install

From GHCR:

```sh
helm install hello oci://ghcr.io/e-minguez/charts/hello-world-helm-chart --version 0.1.0
```

From the openSUSE registry (built by OBS):

```sh
helm install hello oci://registry.opensuse.org/home/eminguez/charts/images/hello-world-chart --version 0.1.0
```

## Local dev

```sh
helm lint hello-world-helm-chart
helm template test hello-world-helm-chart
```

## OBS project setup

The `home:eminguez:charts` project needs these tweaks for Helm chart builds:

**Project config** (`osc meta -e prjconf home:eminguez:charts`):

```
Type: helm
Repotype: helm
Patterntype: none
Required: perl-YAML-LibYAML
```

**Repository** — Tumbleweed/standard, renamed to `images` so the published OCI path is `oci://registry.opensuse.org/home/eminguez/charts/images/<package>`. Architectures: `x86_64`, `aarch64`.

## Required GitHub secrets

| Secret | Purpose |
|---|---|
| `OBS_USER` | OBS account username (HTTP Basic auth — the scoped token types don't work for `osc commit`) |
| `OBS_PASSWORD` | OBS account password. Recommended: dedicated bot account added as maintainer on `home:eminguez:charts` only. |

`GITHUB_TOKEN` is auto-provided.
