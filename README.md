# Ceasing kustomization controller

[![.github/workflows/release.yml](https://github.com/raffis/ceasing-kustomize-controller/workflows/.github/workflows/release.yml/badge.svg)](https://github.com/raffis/ceasing-kustomize-controller/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/raffis/ceasing-kustomize-controller)](https://goreportcard.com/report/github.com/raffis/ceasing-kustomize-controller)
[![Coverage Status](https://coveralls.io/repos/github/raffis/ceasing-kustomize-controller/badge.svg?branch=master)](https://coveralls.io/github/raffis/ceasing-kustomize-controller?branch=master)
[![Docker Pulls](https://img.shields.io/docker/pulls/raffis/ceasing-kustomize-controller.svg?maxAge=604800)](https://hub.docker.com/r/raffis/ceasing-kustomize-controller)
[![Artifact HUB](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/prometheus-ceasing-kustomize-controller)](https://artifacthub.io/packages/search?repo=prometheus-ceasing-kustomize-controller)

A ceasing kustomization is a wrapper around flux2 kustomization CRD.
It introduces a kustomization with an expiration time.
After the expiration is reached the managed kustomization is removed from the cluster and all resources created by the kustomization.
The CeasingKustomization continues to gate the managed kustomization and will not recreate it.
That way you can also distribute CeasingKustomizations via GitOps as you might already do with Kustomizations.

**Note**: A CeasingKustomization always deploys the underlying kustomization as prunable no matter if it is set to true or false.

## Use cases
The use case is primarily to create temporary resources so you don't need to remember it needs to be removed again,
for example:

* Grant temporary RBAC rules, for example give a user temporary exec permission
* Create temporary user accounts
* Temporary preview tenants
* ...

## Example CeasingKustomization

```yaml
apiVersion: kustomize.raffis.github.io/v1beta2
kind: CeasingKustomization
metadata:
  name: admin-access
spec:
  ttl: 1d
  kustomizationTemplate:
    spec:
      interval: 5m
      path: "./base/admin-access"
      prune: false
      sourceRef:
        kind: GitRepository
        name: gitops
      timeout: 80s
```

## Configure the controller

You may change base settings for the controller using env variables (or alternatively command line arguments).

Available env variables:

| Name  | Description | Default |
|-------|-------------| --------|
| `METRICS_ADDR` | The address of the metric endpoint binds to. | `:9556` |
| `PROBE_ADDR` | The address of the probe endpoints binds to. | `:9557` |
| `ENABLE_LEADER_ELECTION` | Enable leader election for controller manager. | `true` |
| `LEADER_ELECTION_NAMESPACE` | Change the leader election namespace. This is by default the same where the controller is deployed. | `` |
| `NAMESPACES` | The controller listens by default for all namespaces. This may be limited to a comma delimited list of dedicated namespaces. | `` |
| `CONCURRENT` | The number of concurrent reconcile workers.  | `4` |
