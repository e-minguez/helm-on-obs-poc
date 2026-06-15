# helm-on-obs-poc

Basic Helm chart deploying [openSUSE nginx](https://registry.opensuse.org/) (`registry.opensuse.org/opensuse/nginx`). Published to GHCR as an OCI artifact on tag push (`v*`).

## Install

```sh
helm install hello oci://ghcr.io/e-minguez/charts/hello-world-helm-chart --version 0.1.0
```

## Local dev

```sh
helm lint hello-world-helm-chart
helm template test hello-world-helm-chart
```
