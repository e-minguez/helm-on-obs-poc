# helm-on-obs-poc

Go hello-world web server (`app/`) served by a Helm chart, published to both GHCR and OBS.

Pipeline:

1. Tag push (`v*`) → `release.yaml`:
   - **Image**: builds multi-arch (`amd64`/`arm64`) container image, pushes to `ghcr.io/e-minguez/hello-world-go`. Then `obs-sync-image.yaml` commits `Dockerfile` + `app/` into OBS package `home:eminguez:containers/hello-world-go`; OBS builds and publishes the image.
   - **Chart**: lints, packages, and pushes to `oci://ghcr.io/e-minguez/charts/hello-world-helm-chart`. Then `obs-sync.yaml` mirrors into OBS package `home:eminguez:charts/hello-world-chart`; OBS builds and publishes the chart.
2. `sync-to-obs.yaml` re-runs the chart sync every 6h as a safety net.

## Install

From GHCR:

```sh
helm install hello oci://ghcr.io/e-minguez/charts/hello-world-helm-chart --version 0.2.0
```

From the openSUSE registry (built by OBS):

```sh
helm install hello oci://registry.opensuse.org/home/eminguez/charts/images/hello-world-chart --version 0.2.0
```

The chart deploys the Go hello-world image. Default image: `ghcr.io/e-minguez/hello-world-go:latest`. To use the OBS-built image:

```sh
helm install hello oci://ghcr.io/e-minguez/charts/hello-world-helm-chart --version 0.2.0 \
  --set image.repository=registry.opensuse.org/home/eminguez/containers/images/hello-world-go
```

## Local dev

```sh
# Run the Go app directly
cd app && go run .
curl localhost:8080

# Build and run the container
podman build -t hello-world-go:test -f Containerfile .
podman run --rm -p 8080:8080 hello-world-go:test

# Lint and render the chart
helm lint hello-world-helm-chart
helm template test hello-world-helm-chart
```

## OBS chart project setup

The `home:eminguez:charts` project needs these tweaks for Helm chart builds:

**Project config** (`osc meta -e prjconf home:eminguez:charts`):

```
Type: helm
Repotype: helm
Patterntype: none
Required: perl-YAML-LibYAML
```

**Repository** — Tumbleweed/standard, renamed to `images` so the published OCI path is `oci://registry.opensuse.org/home/eminguez/charts/images/<package>`. Architectures: `x86_64`, `aarch64`.

## OBS container image project setup

The `home:eminguez:containers` project needs container build support:

**Project config** (`osc meta -e prjconf home:eminguez:containers`):

```
Type: docker
```

**Repository** named `images` — Tumbleweed/standard with a container build target (`BuildEngine: podman`). Published image: `registry.opensuse.org/home/eminguez/containers/images/hello-world-go`. Package: `hello-world-go` (already created).

## Required GitHub secrets

| Secret | Purpose |
|---|---|
| `OBS_USER` | OBS account username (HTTP Basic auth — the scoped token types don't work for `osc commit`) |
| `OBS_PASSWORD` | OBS account password. Recommended: dedicated bot account added as maintainer on both OBS projects. |

`GITHUB_TOKEN` is auto-provided.
