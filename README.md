# helm-on-obs-poc

Go hello-world web server (`app/`) served by a Helm chart, published to both GHCR and OBS.

Pipeline:

1. Tag push (`v*`) → `release.yaml`:
   - **Image**: builds multi-arch (`amd64`/`arm64`) container image, pushes to `ghcr.io/e-minguez/hello-world-go`. Then `obs-sync-image.yaml` commits `Dockerfile` + `app/` into OBS package `home:eminguez:containers/hello-world-go`; OBS builds and publishes the image.
   - **Chart**: lints, packages, and pushes to `oci://ghcr.io/e-minguez/charts/hello-world-helm-chart`. Then `obs-sync.yaml` mirrors into OBS package `home:eminguez:charts/hello-world-chart`; OBS builds and publishes the chart.
2. `sync-to-obs.yaml` re-runs the chart sync every 6h as a safety net.

## Install

From GHCR (deploys the GHCR image `ghcr.io/e-minguez/hello-world-go`):

```sh
helm install hello oci://ghcr.io/e-minguez/charts/hello-world-helm-chart --version 0.2.1
```

From the openSUSE registry (built by OBS; this chart variant deploys the OBS image `registry.opensuse.org/home/eminguez/containers/images/hello-world-go`):

```sh
helm install hello oci://registry.opensuse.org/home/eminguez/charts/images/hello-world-chart --version 0.2.1
```

The image tag defaults to the chart's `appVersion` (the hello-world-go release). The only difference between the two chart variants is `image.repository`: the OBS sync rewrites it to the openSUSE registry path. Override either with `--set image.repository=...` / `--set image.tag=...`.

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

The `home:eminguez:containers` project needs container build support.

**Project config** (`osc meta -e prjconf home:eminguez:containers`):

```
%if "%_repository" == "images"
Type: docker
Repotype: none
Patterntype: none
BuildEngine: podman
%endif

# devel:BCI:Tumbleweed pulls in container-build-checks-strict via the
# system-packages:podman substitute, which treats all warnings (tag namespace,
# release uniqueness, inherited base-image labels) as fatal. Redefining the
# substitute without it (and ignoring it) makes those warnings non-fatal so a
# custom image in a home: project builds. Ignore alone is not enough — the
# package arrives through the substitute, so the substitute must be redefined.
Ignore: container-build-checks-strict
Substitute: system-packages:podman podman buildah createrepo_c release-compare container-build-checks-vendor-openSUSE skopeo umoci post-build-checks
```

**Repository** named `images` — base images resolved from `devel:BCI:Tumbleweed` (single current version of each → always latest, no `Prefer:` pinning needed). Both BCI repos are needed: `containerfile` builds the Dockerfile/BCI images (`golang`, `bci-busybox`), while `images` builds the KIWI base (`opensuse/tumbleweed`). `openSUSE:Tumbleweed/standard` provides RPMs. Architectures: `x86_64`, `aarch64`. Project meta:

```xml
<repository name="images">
  <path project="devel:BCI:Tumbleweed" repository="containerfile"/>
  <path project="devel:BCI:Tumbleweed" repository="images"/>
  <path project="openSUSE:Tumbleweed" repository="standard"/>
  <arch>x86_64</arch>
  <arch>aarch64</arch>
</repository>
```

Published image: `registry.opensuse.org/home/eminguez/containers/images/hello-world-go`. Package: `hello-world-go` (already created).

The OBS build base images differ from GHCR because OBS resolves them from `devel:BCI:Tumbleweed` by short name: the sync workflow rewrites `opensuse/bci/golang:latest` and `opensuse/bci/bci-busybox:latest` into the generated `Dockerfile`, and flattens the Go sources (OBS packages have no subdirectories).

## Releasing

The git tag and the chart version are independent:

1. Bump `hello-world-helm-chart/Chart.yaml`:
   - `version:` — the chart version. **Must change**, or `obs-sync.yaml` sees the same version on GHCR and OBS and skips the sync.
   - `appVersion:` — the hello-world-go image tag the chart deploys.
2. Push a tag: `git tag v0.2.7 && git push origin v0.2.7`.

The tag drives the image build (`hello-world-go:<tag-without-v>`); `appVersion` selects which image tag the chart references. Keep them aligned if you want the chart to deploy the image built by the same release.

## Required GitHub secrets

| Secret | Purpose |
|---|---|
| `OBS_USER` | OBS account username (HTTP Basic auth — the scoped token types don't work for `osc commit`) |
| `OBS_PASSWORD` | OBS account password. Recommended: dedicated bot account added as maintainer on both OBS projects. |

`GITHUB_TOKEN` is auto-provided.
