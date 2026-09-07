# Examples

Copied from the composed resource ordering prototype's end-to-end fixtures, so
the layout is theirs: `setup/` holds the pieces you apply first, `delete/`
holds variants that drive ordered teardown.

Each Composition is standalone. Apply one, then apply `xr.yaml`. Applying a
different Composition over the top switches scenario without recreating the XR.

## Prerequisites

Crossplane from the [ordering branch][branch] with
`--enable-composed-resource-ordering`, plus:

```shell
kubectl apply -f setup/functions.yaml     # this function
kubectl apply -f setup/definition.yaml    # the XRD
```

`provider-nop` is needed by anything setting `readyAfter`, which is currently
just `setup/composition-nested.yaml`:

```shell
kubectl apply -f - <<'YAML'
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-nop
spec:
  package: xpkg.crossplane.io/crossplane-contrib/provider-nop:v0.5.0
YAML
```

## Scenarios

* `setup/composition.yaml` — a linear chain of three resources.
* `setup/composition-nested.yaml` — four levels deep, with a diamond, fan-out
  and two independent branches. The most informative one, and the only one
  whose waves are slow enough to watch.
* `setup/composition-required.yaml` — a resource waiting on an
  EnvironmentConfig the XR requires but doesn't compose. Nothing is created
  until you `kubectl apply -f environmentconfig.yaml`.
* `setup/composition-cbd-before.yaml` then `setup/composition-cbd-after.yaml` —
  a create-before-destroy replacement. The replacement appears while its
  predecessor still exists, then the predecessor goes.
* `setup/composition-contradiction-before.yaml` then
  `setup/composition-contradiction.yaml` — the pipeline drops a resource while
  keeping something that depends on it. Neither can move, and Crossplane says
  so rather than stalling silently.
* `delete/composition.yaml`, `delete/composition-nested.yaml` — the same
  graphs with everything removed from desired state, which tears them down in
  reverse order.

## Watching it work

For the nested scenario, creation timestamps show the waves:

```shell
kubectl -n default get nopresources.nop.crossplane.io \
  --sort-by=.metadata.creationTimestamp \
  -o custom-columns='CREATED:.metadata.creationTimestamp,\
NAME:.metadata.annotations.crossplane\.io/composition-resource-name'
```

The others compose ConfigMaps, so use `get configmaps -l
crossplane.io/composite=ordered` and read `.data.name` instead.

Why something is waiting shows up on the XR:

```shell
kubectl -n default describe xordering ordered | grep -A2 "holding back"
```

Note that events are best-effort — Kubernetes drops rapid duplicates, and
teardown waves often finish too quickly to leave any. Crossplane's own logs
always have the reasons.

[branch]: https://github.com/stevendborrelli/crossplane/tree/composed-resource-ordering
